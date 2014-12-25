package vom

import (
	"fmt"
	"io"

	"v.io/veyron/veyron2/wiretype"
)

// encoderBinary handles encoding type and value messages in binary format.
type encoderBinary struct {
	writer io.Writer
	te     *binaryTypeEncoder
	stateV *encState

	// For managing type definitions.
	sentTypes  map[*rtInfo]wiretype.TypeID
	nextTypeID wiretype.TypeID
}

func newEncoderBinary(w io.Writer, stT, stV *encState) *encoderBinary {
	e := &encoderBinary{
		writer:     w,
		te:         newBinaryTypeEncoder(w),
		stateV:     stV,
		sentTypes:  make(map[*rtInfo]wiretype.TypeID),
		nextTypeID: wiretype.TypeIDFirst,
	}
	return e
}

func (e *encoderBinary) Format() Format { return FormatBinary }

// EncodeReflectValue encodes rv as a sequence of binary type and value messages.
func (e *encoderBinary) EncodeReflectValue(rv Value) error {
	rti, err := lookupRTInfo(rv.Type())
	if err != nil {
		return err
	}
	id, err := e.te.encodeUnsentTypes(rti)
	if err != nil {
		return err
	}
	msg, err := e.encodeValueMsg(id, rti, rv)
	if err != nil {
		return err
	}
	_, err = e.writer.Write(msg)
	return err
}

// paddingLen must be large enough to hold the header info in encodeBinaryMsg.
const paddingLen = maxEncodedUintBytes * 2

func initBinaryMsg(st *encState) {
	st.reset()
	st.buf.Reset()
	st.buf.Grow(paddingLen)
}

func encodeBinaryMsg(buf *encbuf, id int64) []byte {
	// Every binary message starts out with a signed TypeID, sometimes followed by
	// the byte length of the rest of the message.  We don't know the msg len
	// until we've actually encoded the data, so we initialize our buffer with
	// enough padding to hold the header.  After we've produced the actual message
	// we go back and fill in the header.
	//
	// The rawEncode*End methods fill the trailing bytes in the buffer and return
	// the start index of the encoded data.  That means the order we encode here
	// is the opposite of how things appear in the encoded message.
	msg := buf.Bytes()
	header := msg[:paddingLen]
	if id < 0 || !primitiveTypeIDs[wiretype.TypeID(id)] {
		// All typedefs and non-primitive values encode the byte length of the rest
		// of the message, to let the decoder skip over values of unknown types.
		start := rawEncodeUintEnd(header, uint64(len(msg)-paddingLen))
		header = header[:start]
	}
	// Every message starts with a signed TypeID, where +id means a value follows,
	// while -id means a type definition follows.
	start := rawEncodeIntEnd(header, id)
	return msg[start:]
}

// encodeValueMsg encodes a value message and returns the encoded bytes.
func (e *encoderBinary) encodeValueMsg(id wiretype.TypeID, rti *rtInfo, rv Value) ([]byte, error) {
	initBinaryMsg(e.stateV)
	if err := encodeBinaryValue(e.stateV, e.te, rti, rv); err != nil {
		return nil, err
	}
	return encodeBinaryMsg(e.stateV.buf, +int64(id)), nil // +id means a value follows
}

// encodeBinaryValue encodes a given value.
// lookupOrEncodeType should be a function that provides the type id for a given rtInfo object (or
// creates a type id if necessary).
func encodeBinaryValue(st *encState, te *binaryTypeEncoder, rti *rtInfo, rv Value) error {
	var err error
	if rti, rv, err = rti.callCustomEncode(rv, FormatBinary); err != nil {
		return err
	}
	switch rti.kind {
	case typeKindBool:
		rawEncodeBool(st.buf, rv.Bool())
	case typeKindString:
		rawEncodeString(st.buf, rv.String())
	case typeKindByteSlice:
		rawEncodeByteSlice(st.buf, rv.Bytes())
	case typeKindByte:
		// Bytes are written as a single byte; if we wrote them as unsigned integers
		// we'd use two bytes for values from [128, 255].
		st.buf.WriteByte(byte(rv.Uint()))
	case typeKindUint:
		rawEncodeUint(st.buf, rv.Uint())
	case typeKindInt:
		rawEncodeInt(st.buf, rv.Int())
	case typeKindFloat:
		rawEncodeFloat(st.buf, rv.Float())
	case typeKindSlice:
		return encodeBinarySlice(st, te, rti, rv)
	case typeKindArray:
		return encodeBinaryArray(st, te, rti, rv)
	case typeKindMap:
		return encodeBinaryMap(st, te, rti, rv)
	case typeKindStruct:
		return encodeBinaryStruct(st, te, rti, rv)
	case typeKindPtr:
		return encodeBinaryPtr(st, te, rti, rv)
	case typeKindInterface:
		return encodeBinaryInterface(st, te, rv)
	default:
		panic(fmt.Errorf("vom: encodeValue unhandled rti %+v", rti))
	}
	return nil
}

func encodeBinarySlice(st *encState, te *binaryTypeEncoder, rti *rtInfo, rv Value) error {
	// Slices are encoded with their length followed by that many elems.
	rawEncodeUint(st.buf, uint64(rv.Len()))
	for ix := 0; ix < rv.Len(); ix++ {
		if err := encodeBinaryValue(st, te, rti.elem, rv.Index(ix)); err != nil {
			return err
		}
	}
	return nil
}

func encodeBinaryArray(st *encState, te *binaryTypeEncoder, rti *rtInfo, rv Value) error {
	// Arrays are encoded with a fixed number of elems.
	for ix := 0; ix < rti.len; ix++ {
		if err := encodeBinaryValue(st, te, rti.elem, rv.Index(ix)); err != nil {
			return err
		}
	}
	return nil
}

func encodeBinaryMap(st *encState, te *binaryTypeEncoder, rti *rtInfo, rv Value) error {
	// Maps are encoded as a length followed by that many key / elem pairs.
	rawEncodeUint(st.buf, uint64(rv.Len()))
	for _, key := range rv.MapKeys() {
		if err := encodeBinaryValue(st, te, rti.key, key); err != nil {
			return err
		}
		if err := encodeBinaryValue(st, te, rti.elem, rv.MapIndex(key)); err != nil {
			return err
		}
	}
	return nil
}

func encodeBinaryStruct(st *encState, te *binaryTypeEncoder, rti *rtInfo, rv Value) error {
	// Structs are encoded as a sequence of non-zero fields, where the first item
	// is the field index delta, followed by the value of the field.  The index is
	// initialized at -1 so that field index 0 gets a delta of 1.
	prevFx := -1
	for fx, rtf := range rti.fields {
		field := rv.Field(rtf.index)
		if isZeroValue(field) {
			continue
		}
		rawEncodeUint(st.buf, uint64(fx-prevFx))
		if err := encodeBinaryValue(st, te, rtf.info, field); err != nil {
			return err
		}
		prevFx = fx
	}
	// Encode a final 0 delta sentry.
	rawEncodeUint(st.buf, 0)
	return nil
}

func encodeBinaryPtr(st *encState, te *binaryTypeEncoder, rti *rtInfo, rv Value) error {
	// Nil pointers are encoded as 0.
	if rv.IsNil() {
		rawEncodeInt(st.buf, 0)
		return nil
	}
	ptr := rv.Pointer()
	refID, lookup := st.lookupOrAssignRefID(ptr)
	if lookup {
		// Already seen, just encode +refID.
		rawEncodeInt(st.buf, +refID)
		return nil
	}
	// Otherwise we just assigned a new refID, encode -refID and the value.
	rawEncodeInt(st.buf, -refID)
	return encodeBinaryValue(st, te, rti.elem, rv.Elem())
}

func encodeBinaryInterface(st *encState, te *binaryTypeEncoder, rv Value) error {
	// Nil interfaces are encoded as 0.
	if rv.IsNil() {
		rawEncodeInt(st.buf, 0)
		return nil
	}
	// Non-nil interfaces are encoded with both the typeid and value of the
	// concrete underlying value.  Any types we haven't seen yet will be encoded
	// and sent from the type buffer, to ensure all types are written out before
	// any part of the value is written.
	rv = rv.Elem()
	rti, err := lookupRTInfo(rv.Type())
	if err != nil {
		return err
	}
	var id wiretype.TypeID
	if id, err = te.encodeUnsentTypes(rti); err != nil {
		return err
	}

	// First encode +typeid to let the decoder know the type.
	rawEncodeInt(st.buf, +int64(id))
	return encodeBinaryValue(st, te, rti, rv)
}
