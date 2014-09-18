package vom2

import (
	"fmt"
	"io"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vdl/valconv"
	"veyron.io/veyron/veyron2/verror"
)

var (
	errEncodeNilType      = verror.BadProtocolf("no value encoded (encoder finished with nil type)")
	errEncodeZeroTypeID   = verror.Internalf("encoder finished with type ID 0")
	errEncodeBadTypeStack = verror.Internalf("encoder has bad type stack")

	// Make sure binaryEncoder implements the valconv *Target interfaces.
	_ valconv.Target       = (*binaryEncoder)(nil)
	_ valconv.ListTarget   = (*binaryEncoder)(nil)
	_ valconv.SetTarget    = (*binaryEncoder)(nil)
	_ valconv.MapTarget    = (*binaryEncoder)(nil)
	_ valconv.StructTarget = (*binaryEncoder)(nil)
)

type binaryEncoder struct {
	writer io.Writer
	// The binary encoder uses a 2-buffer strategy for writing.  We use bufT to
	// encode type definitions, and write them out to the writer directly.  We use
	// bufV to buffer up the encoded value.  The bufV buffering is necessary so
	// that we can compute the total message length, and also to ensure that all
	// dynamic types are written before the value.
	bufT, bufV *encbuf
	// The binary encoder maintains all types it has sent in sentTypes.  In
	// addition it maintains a typeStack, where typeStack[0] holds the type of the
	// top-level value being encoded, and subsequent layers of the stack holds
	// type information for composites and subtypes.
	sentTypes *encoderTypes
	typeStack []*vdl.Type
}

func newBinaryEncoder(w io.Writer, et *encoderTypes) *binaryEncoder {
	return &binaryEncoder{
		writer:    w,
		bufT:      newEncbuf(),
		bufV:      newEncbuf(),
		sentTypes: et,
		typeStack: make([]*vdl.Type, 0, 10),
	}
}

// paddingLen must be large enough to hold the header in writeMsg.
const paddingLen = maxEncodedUintBytes * 2

func (e *binaryEncoder) writeMsg(buf *encbuf, id int64, encodeLen bool) error {
	// Binary messages always start with a signed id, sometimes followed by the
	// byte length of the rest of the message.  We only know the byte length after
	// we've encoded the rest of the message.  To make this reasonably efficient,
	// the buffer is initialized with enough padding to hold the id and length,
	// and we go back and fill them in here.
	//
	// The binaryEncode*End methods fill in the trailing bytes of the buffer and
	// return the start index of the encoded data.  Thus the binaryEncode*End
	// calls here are in the opposite order they appear in the encoded message.
	msg := buf.Bytes()
	header := msg[:paddingLen]
	if encodeLen {
		start := binaryEncodeUintEnd(header, uint64(len(msg)-paddingLen))
		header = header[:start]
	}
	start := binaryEncodeIntEnd(header, id)
	_, err := e.writer.Write(msg[start:])
	return err
}

func (e *binaryEncoder) StartEncode() error {
	e.bufV.Reset()
	e.bufV.Grow(paddingLen)
	e.typeStack = e.typeStack[:0]
	return nil
}

func (e *binaryEncoder) FinishEncode() error {
	switch {
	case len(e.typeStack) > 1:
		return errEncodeBadTypeStack
	case len(e.typeStack) == 0:
		return errEncodeNilType
	}
	encType := e.typeStack[0]
	id := e.sentTypes.LookupID(encType)
	if id == 0 {
		return errEncodeZeroTypeID
	}
	return e.writeMsg(e.bufV, +int64(id), hasBinaryMsgLen(encType))
}

// encodeUnsentTypes writes type definitions for t and all subtypes that haven't
// been sent yet.  Returns the type ID assigned to t.
func (e *binaryEncoder) encodeUnsentTypes(t *vdl.Type) (TypeID, error) {
	id, isNew := e.sentTypes.LookupOrAssignID(t)
	if isNew {
		if err := e.encodeType(t, id); err != nil {
			return 0, err
		}
	}
	return id, nil
}

// encodeType performs recursive depth-first pre-order traversal of the type
// graph, encoding unexplored types along the way.  Any types that have already
// been explored (even if they're still pending encoding) terminate that branch
// of the traversal, to ensure termination for recursive types.
func (e *binaryEncoder) encodeType(t *vdl.Type, id TypeID) error {
	// Encode the wire type.  We may assign new type ids, but we never recursively
	// encode type messages.  WireType is represented as a struct, which always
	// encodes the message length.
	const encodeMsgLen = true
	e.bufT.Reset()
	e.bufT.Grow(paddingLen)
	children := e.encodeWireType(e.bufT, t)
	if err := e.writeMsg(e.bufT, -int64(id), encodeMsgLen); err != nil {
		return err
	}

	// Now continue the recursive pre-order traversal of the unexplored children.
	for _, child := range children {
		if err := e.encodeType(child.t, child.id); err != nil {
			return err
		}
	}
	return nil
}

type typeAndID struct {
	t  *vdl.Type
	id TypeID
}

// encodeWireType encodes the wire type corresponding to t into buf.  Returns
// the unexplored children subtypes of t, which also need to be encoded.
//
// The wire type is manually encoded via direct calls to binaryEncode*.  An
// alternative would be to convert t into its WireType representation, but
// that's slower and more complicated.
//
// TODO(toddw): Consider converting to WireType after oneof is implemented.
func (e *binaryEncoder) encodeWireType(buf *encbuf, t *vdl.Type) (children []typeAndID) {
	switch kind := t.Kind(); kind {
	case vdl.Bool, vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64, vdl.Complex64, vdl.Complex128, vdl.String:
		binaryEncodeUint(buf, uint64(WireNamedID))
		binaryEncodeUint(buf, 1)
		binaryEncodeString(buf, t.Name())
		binaryEncodeUint(buf, 2)
		binaryEncodeUint(buf, uint64(bootstrapKindToID[kind]))
		binaryEncodeUint(buf, 0)
		return
	case vdl.Enum:
		binaryEncodeUint(buf, uint64(WireEnumID))
		binaryEncodeUint(buf, 1)
		binaryEncodeString(buf, t.Name())
		binaryEncodeUint(buf, 2)
		binaryEncodeUint(buf, uint64(t.NumEnumLabel()))
		for x := 0; x < t.NumEnumLabel(); x++ {
			binaryEncodeString(buf, t.EnumLabel(x))
		}
		binaryEncodeUint(buf, 0)
		return
	case vdl.Array:
		elemID, isNew := e.sentTypes.LookupOrAssignID(t.Elem())
		if isNew {
			children = append(children, typeAndID{t.Elem(), elemID})
		}
		binaryEncodeUint(buf, uint64(WireArrayID))
		if t.Name() != "" {
			binaryEncodeUint(buf, 1)
			binaryEncodeString(buf, t.Name())
		}
		binaryEncodeUint(buf, 2)
		binaryEncodeUint(buf, uint64(elemID))
		binaryEncodeUint(buf, 3)
		binaryEncodeUint(buf, uint64(t.Len()))
		binaryEncodeUint(buf, 0)
		return
	case vdl.List:
		elemID, isNew := e.sentTypes.LookupOrAssignID(t.Elem())
		if isNew {
			children = append(children, typeAndID{t.Elem(), elemID})
		}
		binaryEncodeUint(buf, uint64(WireListID))
		if t.Name() != "" {
			binaryEncodeUint(buf, 1)
			binaryEncodeString(buf, t.Name())
		}
		binaryEncodeUint(buf, 2)
		binaryEncodeUint(buf, uint64(elemID))
		binaryEncodeUint(buf, 0)
		return
	case vdl.Set:
		keyID, isNew := e.sentTypes.LookupOrAssignID(t.Key())
		if isNew {
			children = append(children, typeAndID{t.Key(), keyID})
		}
		binaryEncodeUint(buf, uint64(WireSetID))
		if t.Name() != "" {
			binaryEncodeUint(buf, 1)
			binaryEncodeString(buf, t.Name())
		}
		binaryEncodeUint(buf, 2)
		binaryEncodeUint(buf, uint64(keyID))
		binaryEncodeUint(buf, 0)
		return
	case vdl.Map:
		keyID, isNew := e.sentTypes.LookupOrAssignID(t.Key())
		if isNew {
			children = append(children, typeAndID{t.Key(), keyID})
		}
		elemID, isNew := e.sentTypes.LookupOrAssignID(t.Elem())
		if isNew {
			children = append(children, typeAndID{t.Elem(), elemID})
		}
		binaryEncodeUint(buf, uint64(WireMapID))
		if t.Name() != "" {
			binaryEncodeUint(buf, 1)
			binaryEncodeString(buf, t.Name())
		}
		binaryEncodeUint(buf, 2)
		binaryEncodeUint(buf, uint64(keyID))
		binaryEncodeUint(buf, 3)
		binaryEncodeUint(buf, uint64(elemID))
		binaryEncodeUint(buf, 0)
		return
	case vdl.Struct:
		binaryEncodeUint(buf, uint64(WireStructID))
		binaryEncodeUint(buf, 1)
		binaryEncodeString(buf, t.Name())
		binaryEncodeUint(buf, 2)
		binaryEncodeUint(buf, uint64(t.NumField()))
		for x := 0; x < t.NumField(); x++ {
			field := t.Field(x)
			fieldID, isNew := e.sentTypes.LookupOrAssignID(field.Type)
			if isNew {
				children = append(children, typeAndID{field.Type, fieldID})
			}
			binaryEncodeUint(buf, 1)
			binaryEncodeString(buf, field.Name)
			binaryEncodeUint(buf, 2)
			binaryEncodeUint(buf, uint64(fieldID))
			binaryEncodeUint(buf, 0)
		}
		binaryEncodeUint(buf, 0)
		return
	case vdl.OneOf:
		binaryEncodeUint(buf, uint64(WireOneOfID))
		binaryEncodeUint(buf, 1)
		binaryEncodeString(buf, t.Name())
		binaryEncodeUint(buf, 2)
		binaryEncodeUint(buf, uint64(t.NumOneOfType()))
		for x := 0; x < t.NumOneOfType(); x++ {
			one := t.OneOfType(x)
			oneID, isNew := e.sentTypes.LookupOrAssignID(one)
			if isNew {
				children = append(children, typeAndID{one, oneID})
			}
			binaryEncodeUint(buf, uint64(oneID))
		}
		binaryEncodeUint(buf, 0)
		return
	default:
		panic(fmt.Errorf("vom2: encodeWireType unhandled type %v", t))
	}
}

// prepareType prepares to encode a value of type tt, checking to make sure it
// has one of the specified kinds, and encoding any unsent types.
func (e *binaryEncoder) prepareType(t *vdl.Type, kinds ...vdl.Kind) error {
	for _, k := range kinds {
		if t.Kind() == k {
			return e.unvalidatedPrepareType(t)
		}
	}
	return verror.BadArgf("encoder type mismatch, got %q, want %v", t, kinds)
}

// unvalidatedPrepareType encodes any unsent types, and handles encoding type
// ids for variant types.
func (e *binaryEncoder) unvalidatedPrepareType(tt *vdl.Type) error {
	switch top := e.topType(); {
	case top == nil:
		// Encoding the top-level.  We postpone encoding of the id until writeMsg is
		// called, to handle positive and negative ids, and the message length.
		_, err := e.encodeUnsentTypes(tt)
		if err != nil {
			return err
		}
		e.pushType(tt)
		return nil
	case top == tt:
		// Encoding a concrete sub-type, nothing more to do.
		return nil
	case top.Kind() == vdl.Any || (top.Kind() == vdl.OneOf && top.OneOfIndex(tt) >= 0):
		// Encoding a variant sub-type, encode unsent types and id.
		id, err := e.encodeUnsentTypes(tt)
		if err != nil {
			return err
		}
		binaryEncodeUint(e.bufV, uint64(id))
		return nil
	default:
		return verror.BadArgf("encoder type stack mismatch, got %q, top type %q", tt, top)
	}
}

func (e *binaryEncoder) pushType(tt *vdl.Type) {
	e.typeStack = append(e.typeStack, tt)
}

func (e *binaryEncoder) popType() error {
	if len(e.typeStack) == 0 {
		return errEncodeBadTypeStack
	}
	e.typeStack = e.typeStack[:len(e.typeStack)-1]
	return nil
}

func (e *binaryEncoder) topType() *vdl.Type {
	if len(e.typeStack) == 0 {
		return nil
	}
	return e.typeStack[len(e.typeStack)-1]
}

func (e *binaryEncoder) FromBool(src bool, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.Bool); err != nil {
		return err
	}
	binaryEncodeBool(e.bufV, src)
	return nil
}

func (e *binaryEncoder) FromUint(src uint64, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64); err != nil {
		return err
	}
	if tt.Kind() == vdl.Byte {
		e.bufV.WriteByte(byte(src))
	} else {
		binaryEncodeUint(e.bufV, src)
	}
	return nil
}

func (e *binaryEncoder) FromInt(src int64, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.Int16, vdl.Int32, vdl.Int64); err != nil {
		return err
	}
	binaryEncodeInt(e.bufV, src)
	return nil
}

func (e *binaryEncoder) FromFloat(src float64, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.Float32, vdl.Float64); err != nil {
		return err
	}
	binaryEncodeFloat(e.bufV, src)
	return nil
}

func (e *binaryEncoder) FromComplex(src complex128, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.Complex64, vdl.Complex128); err != nil {
		return err
	}
	binaryEncodeFloat(e.bufV, real(src))
	binaryEncodeFloat(e.bufV, imag(src))
	return nil
}

func (e *binaryEncoder) FromBytes(src []byte, tt *vdl.Type) error {
	if !tt.IsBytes() {
		return verror.BadArgf("encoder type mismatch, got %q, want bytes", tt)
	}
	if err := e.unvalidatedPrepareType(tt); err != nil {
		return err
	}
	if tt.Kind() == vdl.List {
		binaryEncodeUint(e.bufV, uint64(len(src)))
	}
	e.bufV.Write(src)
	return nil
}

func (e *binaryEncoder) FromString(src string, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.String); err != nil {
		return err
	}
	binaryEncodeString(e.bufV, src)
	return nil
}

func (e *binaryEncoder) FromEnumLabel(src string, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.Enum); err != nil {
		return err
	}
	index := tt.EnumIndex(src)
	if index < 0 {
		return verror.BadArgf("enum label %q doesn't exist in type %q", src, tt)
	}
	binaryEncodeUint(e.bufV, uint64(index))
	return nil
}

func (e *binaryEncoder) FromTypeVal(src *vdl.Type) error {
	if err := e.prepareType(vdl.TypeValType, vdl.TypeVal); err != nil {
		return err
	}
	id, err := e.encodeUnsentTypes(src)
	if err != nil {
		return err
	}
	binaryEncodeUint(e.bufV, uint64(id))
	return nil
}

func (e *binaryEncoder) FromNil(tt *vdl.Type) error {
	// We need figure out whether FromNil applies to pointers or interfaces, and
	// fix target.go to do the right thing.
	panic("TODO")
}

func (e *binaryEncoder) StartList(tt *vdl.Type, len int) (valconv.ListTarget, error) {
	if err := e.prepareType(tt, vdl.Array, vdl.List); err != nil {
		return nil, err
	}
	e.pushType(tt)
	if tt.Kind() == vdl.List {
		binaryEncodeUint(e.bufV, uint64(len))
	}
	return e, nil
}

func (e *binaryEncoder) StartSet(tt *vdl.Type, len int) (valconv.SetTarget, error) {
	if err := e.prepareType(tt, vdl.Set); err != nil {
		return nil, err
	}
	e.pushType(tt)
	binaryEncodeUint(e.bufV, uint64(len))
	return e, nil
}

func (e *binaryEncoder) StartMap(tt *vdl.Type, len int) (valconv.MapTarget, error) {
	if err := e.prepareType(tt, vdl.Map); err != nil {
		return nil, err
	}
	e.pushType(tt)
	binaryEncodeUint(e.bufV, uint64(len))
	return e, nil
}

func (e *binaryEncoder) StartStruct(tt *vdl.Type) (valconv.StructTarget, error) {
	if err := e.prepareType(tt, vdl.Struct); err != nil {
		return nil, err
	}
	e.pushType(tt)
	return e, nil
}

func (e *binaryEncoder) StartOneOf(tt *vdl.Type) (valconv.Target, error) {
	if err := e.prepareType(tt, vdl.OneOf); err != nil {
		return nil, err
	}
	e.pushType(tt)
	return e, nil
}

func (e *binaryEncoder) FinishList(valconv.ListTarget) error {
	return e.popType()
}

func (e *binaryEncoder) FinishSet(valconv.SetTarget) error {
	return e.popType()
}

func (e *binaryEncoder) FinishMap(valconv.MapTarget) error {
	return e.popType()
}

func (e *binaryEncoder) FinishStruct(valconv.StructTarget) error {
	binaryEncodeUint(e.bufV, 0)
	return e.popType()
}

func (e *binaryEncoder) FinishOneOf(valconv.Target) error {
	return e.popType()
}

func (e *binaryEncoder) StartElem(index int) (valconv.Target, error) {
	e.pushType(e.topType().Elem())
	return e, nil
}

func (e *binaryEncoder) FinishElem(elem valconv.Target) error {
	return e.popType()
}

func (e *binaryEncoder) StartKey() (valconv.Target, error) {
	e.pushType(e.topType().Key())
	return e, nil
}

func (e *binaryEncoder) FinishKeyStartField(key valconv.Target) (valconv.Target, error) {
	if err := e.popType(); err != nil {
		return nil, err
	}
	e.pushType(e.topType().Elem())
	return e, nil
}

func (e *binaryEncoder) FinishField(key, field valconv.Target) error {
	return e.popType()
}

func (e *binaryEncoder) StartField(name string) (_, _ valconv.Target, _ error) {
	// Structs are encoded as a sequence of fields, in any order.  Each field
	// starts with its absolute 1-based index, followed by the value.  The 0 index
	// indicates the end of the struct.
	top := e.topType()
	if top == nil {
		return nil, nil, errEncodeBadTypeStack
	}
	if vfield, index := top.FieldByName(name); index >= 0 {
		e.pushType(vfield.Type)
		binaryEncodeUint(e.bufV, uint64(index)+1)
		return nil, e, nil
	}
	return nil, nil, verror.BadArgf("field name %q doesn't exist in top type %q", name, top)
}

func (e *binaryEncoder) FinishKey(key valconv.Target) error {
	return e.popType()
}
