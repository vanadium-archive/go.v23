package vom

import (
	"fmt"
	"io"

	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vdl/valconv"
	"v.io/core/veyron2/verror"
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
	_ valconv.FieldsTarget = (*binaryEncoder)(nil)
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

// encodeUnsentTypes encodes the wire type corresponding to t into the type
// buffer e.bufT. It does this recursively in depth-first order, encoding any
// children of the type before  the type itself. Type ids are allocated in the
// order that we recurse and consequentially may be sent out of sequential
// order if type information for children is sent (before the parent type).
//
// The wire type is manually encoded via direct calls to binaryEncode*.  An
// alternative would be to convert t into its WireType representation, but
// that's slower and more complicated.
//
// TODO(toddw): Consider converting to WireType after union is implemented.
func (e *binaryEncoder) encodeUnsentTypes(t *vdl.Type) (TypeID, error) {
	// Lookup a type ID for t or assign a new one.
	id, isNew := e.sentTypes.LookupOrAssignID(t)
	if !isNew {
		return id, nil
	}

	// Encode child types and collect their IDs.
	var elemID TypeID
	var keyID TypeID
	var fieldIDs []TypeID
	var err error
	switch t.Kind() {
	case vdl.Array, vdl.List, vdl.Optional:
		if elemID, err = e.encodeUnsentTypes(t.Elem()); err != nil {
			return 0, err
		}
	case vdl.Set:
		if keyID, err = e.encodeUnsentTypes(t.Key()); err != nil {
			return 0, err
		}
	case vdl.Map:
		if keyID, err = e.encodeUnsentTypes(t.Key()); err != nil {
			return 0, err
		}
		if elemID, err = e.encodeUnsentTypes(t.Elem()); err != nil {
			return 0, err
		}
	case vdl.Struct, vdl.Union:
		fieldIDs = make([]TypeID, t.NumField())
		for x := 0; x < t.NumField(); x++ {
			if fieldIDs[x], err = e.encodeUnsentTypes(t.Field(x).Type); err != nil {
				return 0, err
			}
		}
	}

	// Setup the buffer for writing a new type.
	buf := e.bufT
	buf.Reset()
	buf.Grow(paddingLen)

	// Construct the type
	switch kind := t.Kind(); kind {
	case vdl.Bool, vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64, vdl.Complex64, vdl.Complex128, vdl.String:
		binaryEncodeUint(buf, uint64(WireNamedID))
		binaryEncodeUint(buf, 1)
		binaryEncodeString(buf, t.Name())
		binaryEncodeUint(buf, 2)
		binaryEncodeUint(buf, uint64(bootstrapKindToID[kind]))
		binaryEncodeUint(buf, 0)
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
	case vdl.Array, vdl.List, vdl.Optional:
		id := WireArrayID
		switch kind {
		case vdl.List:
			id = WireListID
		case vdl.Optional:
			id = WireOptionalID
		}
		binaryEncodeUint(buf, uint64(id))
		binaryEncodeUint(buf, 1)
		binaryEncodeString(buf, t.Name())
		binaryEncodeUint(buf, 2)
		binaryEncodeUint(buf, uint64(elemID))
		if kind == vdl.Array {
			binaryEncodeUint(buf, 3)
			binaryEncodeUint(buf, uint64(t.Len()))
		}
		binaryEncodeUint(buf, 0)
	case vdl.Set:
		binaryEncodeUint(buf, uint64(WireSetID))
		binaryEncodeUint(buf, 1)
		binaryEncodeString(buf, t.Name())
		binaryEncodeUint(buf, 2)
		binaryEncodeUint(buf, uint64(keyID))
		binaryEncodeUint(buf, 0)
	case vdl.Map:
		binaryEncodeUint(buf, uint64(WireMapID))
		binaryEncodeUint(buf, 1)
		binaryEncodeString(buf, t.Name())
		binaryEncodeUint(buf, 2)
		binaryEncodeUint(buf, uint64(keyID))
		binaryEncodeUint(buf, 3)
		binaryEncodeUint(buf, uint64(elemID))
		binaryEncodeUint(buf, 0)
	case vdl.Struct, vdl.Union:
		id := WireStructID
		if kind == vdl.Union {
			id = WireUnionID
		}
		binaryEncodeUint(buf, uint64(id))
		binaryEncodeUint(buf, 1)
		binaryEncodeString(buf, t.Name())
		binaryEncodeUint(buf, 2)
		binaryEncodeUint(buf, uint64(t.NumField()))
		for x := 0; x < len(fieldIDs); x++ {
			fieldID := fieldIDs[x]
			binaryEncodeUint(buf, 1)
			binaryEncodeString(buf, t.Field(x).Name)
			binaryEncodeUint(buf, 2)
			binaryEncodeUint(buf, uint64(fieldID))
			binaryEncodeUint(buf, 0)
		}
		binaryEncodeUint(buf, 0)
	default:
		panic(fmt.Errorf("vom: encodeUnsentTypes unhandled type %v", t))
	}

	// Write the type definition message.
	const encodeMsgLen = true
	err = e.writeMsg(buf, -int64(id), encodeMsgLen)
	return id, err
}

func errTypeMismatch(t *vdl.Type, kinds ...vdl.Kind) error {
	return verror.BadArgf("encoder type mismatch, got %q, want %v", t, kinds)
}

// prepareType prepares to encode a non-nil value of type tt, checking to make
// sure it has one of the specified kinds, and encoding any unsent types.
func (e *binaryEncoder) prepareType(t *vdl.Type, kinds ...vdl.Kind) error {
	for _, k := range kinds {
		if t.Kind() == k || t.Kind() == vdl.Optional && t.Elem().Kind() == k {
			return e.prepareTypeHelper(t, false)
		}
	}
	return errTypeMismatch(t, kinds...)
}

// prepareTypeHelper encodes any unsent types, and manages the type stack.  If
// fromNil is true, we skip encoding the typeid or exists byte for any and
// optional types, since we'll be encoding a nil 0 instead.
func (e *binaryEncoder) prepareTypeHelper(tt *vdl.Type, fromNil bool) error {
	id, err := e.encodeUnsentTypes(tt)
	if err != nil {
		return err
	}
	top := e.topType()
	// Handle the type id for Any values.
	switch {
	case top == nil:
		// Encoding the top-level.  We postpone encoding of the id until writeMsg is
		// called, to handle positive and negative ids, and the message length.
		top = tt
		e.pushType(top)
	case top.Kind() == vdl.Any:
		if !fromNil {
			binaryEncodeUint(e.bufV, uint64(id))
		}
	}
	// Handle the exists byte for Optional values.  Note that if we have an
	// any(?foo), we must encode the type id for ?foo first, before we encode the
	// exists byte.
	if !fromNil &&
		((top.Kind() == vdl.Optional) ||
			(top.Kind() == vdl.Any && tt.Kind() == vdl.Optional)) {
		binaryEncodeUint(e.bufV, 1)
	}
	return nil
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
	if err := e.prepareTypeHelper(tt, false); err != nil {
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

func (e *binaryEncoder) FromTypeObject(src *vdl.Type) error {
	if err := e.prepareType(vdl.TypeObjectType, vdl.TypeObject); err != nil {
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
	// TODO(toddw): Implement optional values with the new flags mechanism.
	if !tt.CanBeNil() {
		return errTypeMismatch(tt, vdl.Any, vdl.Optional)
	}
	if err := e.prepareTypeHelper(tt, true); err != nil {
		return err
	}
	binaryEncodeUint(e.bufV, 0)
	return nil
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

func (e *binaryEncoder) StartFields(tt *vdl.Type) (valconv.FieldsTarget, error) {
	if err := e.prepareType(tt, vdl.Struct, vdl.Union); err != nil {
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

func (e *binaryEncoder) FinishFields(valconv.FieldsTarget) error {
	if top := e.topType(); top != nil && top.Kind() == vdl.Struct || top.Kind() == vdl.Optional && top.Elem().Kind() == vdl.Struct {
		// Write the struct terminator; don't write for union.
		binaryEncodeUint(e.bufV, 0)
	}
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
	// TODO(toddw): Change the encoding to the new scheme described in doc.go.
	top := e.topType()
	if top == nil {
		return nil, nil, errEncodeBadTypeStack
	}
	if top.Kind() == vdl.Optional {
		top = top.Elem()
	}
	if k := top.Kind(); k != vdl.Struct && k != vdl.Union {
		return nil, nil, errTypeMismatch(top, vdl.Struct, vdl.Union)
	}
	// Struct and Union are encoded as a sequence of fields, in any order.  Each
	// field starts with its absolute 1-based index, followed by the value.  Union
	// always consists of a single field, while structs use a 0 terminator.
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
