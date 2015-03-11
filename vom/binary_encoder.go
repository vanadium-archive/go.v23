package vom

import (
	"errors"
	"fmt"
	"io"
	"reflect"

	"v.io/v23/vdl"
)

var (
	errEncodeNilType      = errors.New("no value encoded (encoder finished with nil type)")
	errEncodeZeroTypeID   = errors.New("encoder finished with type ID 0")
	errEncodeBadTypeStack = errors.New("encoder has bad type stack")

	// Make sure binaryEncoder implements the vdl *Target interfaces.
	_ vdl.Target       = (*binaryEncoder)(nil)
	_ vdl.ListTarget   = (*binaryEncoder)(nil)
	_ vdl.SetTarget    = (*binaryEncoder)(nil)
	_ vdl.MapTarget    = (*binaryEncoder)(nil)
	_ vdl.FieldsTarget = (*binaryEncoder)(nil)
)

type binaryEncoder struct {
	writer io.Writer
	// The binary encoder uses a 2-buffer strategy for writing.  We use bufT to
	// encode type definitions, and write them out to the writer directly.  We use
	// buf to buffer up the encoded value.  The buf buffering is necessary so
	// that we can compute the total message length, and also to ensure that all
	// dynamic types are written before the value.
	buf, bufT *encbuf
	// We maintain a typeStack, where typeStack[0] holds the type of the top-level
	// value being encoded, and subsequent layers of the stack holds type information
	// for composites and subtypes.  Each entry also holds the start position of the
	// encoding buffer, which will be used to ignore zero value fields in structs.
	// typeStackT is used for encoding type definitions.
	typeStack, typeStackT []typeStackEntry
	// The binary encoder maintains all types it has sent in sentTypes.
	sentTypes *encoderTypes
}

type typeStackEntry struct {
	tt  *vdl.Type
	pos int
}

func newBinaryEncoder(w io.Writer, et *encoderTypes) *binaryEncoder {
	return &binaryEncoder{
		writer:     w,
		buf:        newEncbuf(),
		bufT:       newEncbuf(),
		typeStack:  make([]typeStackEntry, 0, 10),
		typeStackT: make([]typeStackEntry, 0, 10),
		sentTypes:  et,
	}
}

// paddingLen must be large enough to hold the header in writeMsg.
const paddingLen = maxEncodedUintBytes * 2

func (e *binaryEncoder) StartEncode() error {
	e.buf.Reset()
	e.buf.Grow(paddingLen)
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
	encType := e.typeStack[0].tt
	id := e.sentTypes.LookupID(encType)
	if id == 0 {
		return errEncodeZeroTypeID
	}
	return e.writeMsg(e.buf, +int64(id), hasBinaryMsgLen(encType))
}

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

// encodeUnsentTypes encodes the wire type corresponding to tt into the type
// buffer e.bufT. It does this recursively in depth-first order, encoding any
// children of the type before  the type itself. Type ids are allocated in the
// order that we recurse and consequentially may be sent out of sequential
// order if type information for children is sent (before the parent type).
func (e *binaryEncoder) encodeUnsentTypes(tt *vdl.Type) (typeId, error) {
	if isWireTypeType(tt) {
		// Ignore types for wire type definition.
		return 0, nil
	}
	// Lookup a type ID for tt or assign a new one.
	tid, isNew := e.sentTypes.LookupOrAssignID(tt)
	if !isNew {
		return tid, nil
	}

	// Construct the wireType.
	var wt wireType
	switch kind := tt.Kind(); kind {
	case vdl.Bool, vdl.Byte, vdl.String, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64, vdl.Complex64, vdl.Complex128:
		wt = wireTypeNamedT{wireNamed{tt.Name(), bootstrapKindToID[kind]}}
	case vdl.Enum:
		wireEnum := wireEnum{tt.Name(), make([]string, tt.NumEnumLabel())}
		for ix := 0; ix < tt.NumEnumLabel(); ix++ {
			wireEnum.Labels[ix] = tt.EnumLabel(ix)
		}
		wt = wireTypeEnumT{wireEnum}
	case vdl.Array:
		elm, err := e.encodeUnsentTypes(tt.Elem())
		if err != nil {
			return 0, err
		}
		wt = wireTypeArrayT{wireArray{tt.Name(), elm, uint64(tt.Len())}}
	case vdl.List:
		elm, err := e.encodeUnsentTypes(tt.Elem())
		if err != nil {
			return 0, err
		}
		wt = wireTypeListT{wireList{tt.Name(), elm}}
	case vdl.Set:
		key, err := e.encodeUnsentTypes(tt.Key())
		if err != nil {
			return 0, err
		}
		wt = wireTypeSetT{wireSet{tt.Name(), key}}
	case vdl.Map:
		key, err := e.encodeUnsentTypes(tt.Key())
		if err != nil {
			return 0, err
		}
		elm, err := e.encodeUnsentTypes(tt.Elem())
		if err != nil {
			return 0, err
		}
		wt = wireTypeMapT{wireMap{tt.Name(), key, elm}}
	case vdl.Struct:
		wireStruct := wireStruct{tt.Name(), make([]wireField, tt.NumField())}
		for ix := 0; ix < tt.NumField(); ix++ {
			field, err := e.encodeUnsentTypes(tt.Field(ix).Type)
			if err != nil {
				return 0, err
			}
			wireStruct.Fields[ix] = wireField{tt.Field(ix).Name, field}
		}
		wt = wireTypeStructT{wireStruct}
	case vdl.Union:
		wireUnion := wireUnion{tt.Name(), make([]wireField, tt.NumField())}
		for ix := 0; ix < tt.NumField(); ix++ {
			field, err := e.encodeUnsentTypes(tt.Field(ix).Type)
			if err != nil {
				return 0, err
			}
			wireUnion.Fields[ix] = wireField{tt.Field(ix).Name, field}
		}
		wt = wireTypeUnionT{wireUnion}
	case vdl.Optional:
		elm, err := e.encodeUnsentTypes(tt.Elem())
		if err != nil {
			return 0, err
		}
		wt = wireTypeOptionalT{wireOptional{tt.Name(), elm}}
	default:
		panic(fmt.Errorf("vom: encodeUnsentTypes unhandled type %v", tt))
	}

	// Encode and write the wire type definition.
	//
	// We use the same encoding routines as values for wire types. Since the type
	// encoding can happen any time in the middle of value encoding, we save the
	// status for value encoding such as buf and type stack. This assumes wireType
	// encoding below will never call this recursively. This is true for now since
	// wireType type and its all child types are internal built-in types and we do
	// not send those types.
	//
	// TODO(jhahn): clean this up to be simpler to understand, easier to ensure that
	// vdl.FromReflect doesn't end up calling us recursively.
	e.buf, e.bufT = e.bufT, e.buf
	e.typeStack, e.typeStackT = e.typeStackT, e.typeStack

	e.buf.Reset()
	e.buf.Grow(paddingLen)
	e.typeStack = e.typeStack[:0]

	if err := vdl.FromReflect(e, reflect.ValueOf(wt)); err != nil {
		return 0, err
	}

	switch {
	case len(e.typeStack) > 1:
		return 0, errEncodeBadTypeStack
	case len(e.typeStack) == 0:
		return 0, errEncodeNilType
	}
	encType := e.typeStack[0].tt
	if err := e.writeMsg(e.buf, -int64(tid), hasBinaryMsgLen(encType)); err != nil {
		return 0, err
	}

	e.typeStack, e.typeStackT = e.typeStackT, e.typeStack
	e.buf, e.bufT = e.bufT, e.buf
	return tid, nil
}

func errTypeMismatch(t *vdl.Type, kinds ...vdl.Kind) error {
	return fmt.Errorf("encoder type mismatch, got %q, want %v", t, kinds)
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
// fromNil is true, we skip encoding the typeid for any type, since we'll be
// encoding a nil instead.
func (e *binaryEncoder) prepareTypeHelper(tt *vdl.Type, fromNil bool) error {
	tid, err := e.encodeUnsentTypes(tt)
	if err != nil {
		return err
	}
	top := e.topType()
	// Handle the type id for Any values.
	switch {
	case top == nil:
		// Encoding the top-level.  We postpone encoding of the tid until writeMsg
		// is called, to handle positive and negative ids, and the message length.
		top = tt
		e.pushType(top)
	case top.Kind() == vdl.Any:
		if !fromNil {
			binaryEncodeUint(e.buf, uint64(tid))
		}
	}
	return nil
}

func (e *binaryEncoder) pushType(tt *vdl.Type) {
	e.typeStack = append(e.typeStack, typeStackEntry{tt, e.buf.Len()})
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
	return e.typeStack[len(e.typeStack)-1].tt
}

func (e *binaryEncoder) topTypeAndPos() (*vdl.Type, int) {
	if len(e.typeStack) == 0 {
		return nil, -1
	}
	entry := e.typeStack[len(e.typeStack)-1]
	return entry.tt, entry.pos
}

// canIgnoreField returns true if a zero-value field can be ignored.
func (e *binaryEncoder) canIgnoreField() bool {
	if len(e.typeStack) < 2 {
		return false
	}
	secondTop := e.typeStack[len(e.typeStack)-2].tt
	return secondTop.Kind() == vdl.Struct || (secondTop.Kind() == vdl.Optional && secondTop.Elem().Kind() == vdl.Struct)
}

// ignoreField ignores the encoding output for the current field.
func (e *binaryEncoder) ignoreField() {
	e.buf.Truncate(e.typeStack[len(e.typeStack)-1].pos)
}

func (e *binaryEncoder) FromBool(src bool, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.Bool); err != nil {
		return err
	}
	if src == false && e.canIgnoreField() {
		e.ignoreField()
		return nil
	}
	binaryEncodeBool(e.buf, src)
	return nil
}

func (e *binaryEncoder) FromUint(src uint64, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64); err != nil {
		return err
	}
	if src == 0 && e.canIgnoreField() {
		e.ignoreField()
		return nil
	}
	if tt.Kind() == vdl.Byte {
		e.buf.WriteByte(byte(src))
	} else {
		binaryEncodeUint(e.buf, src)
	}
	return nil
}

func (e *binaryEncoder) FromInt(src int64, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.Int16, vdl.Int32, vdl.Int64); err != nil {
		return err
	}
	if src == 0 && e.canIgnoreField() {
		e.ignoreField()
		return nil
	}
	binaryEncodeInt(e.buf, src)
	return nil
}

func (e *binaryEncoder) FromFloat(src float64, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.Float32, vdl.Float64); err != nil {
		return err
	}
	if src == 0 && e.canIgnoreField() {
		e.ignoreField()
		return nil
	}
	binaryEncodeFloat(e.buf, src)
	return nil
}

func (e *binaryEncoder) FromComplex(src complex128, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.Complex64, vdl.Complex128); err != nil {
		return err
	}
	if src == 0 && e.canIgnoreField() {
		e.ignoreField()
		return nil
	}
	binaryEncodeFloat(e.buf, real(src))
	binaryEncodeFloat(e.buf, imag(src))
	return nil
}

func (e *binaryEncoder) FromBytes(src []byte, tt *vdl.Type) error {
	if !tt.IsBytes() {
		return fmt.Errorf("encoder type mismatch, got %q, want bytes", tt)
	}
	if err := e.prepareTypeHelper(tt, false); err != nil {
		return err
	}
	if tt.Kind() == vdl.List {
		if len(src) == 0 && e.canIgnoreField() {
			e.ignoreField()
			return nil
		}
		binaryEncodeUint(e.buf, uint64(len(src)))
	} else {
		// We always encode array length to 0.
		binaryEncodeUint(e.buf, 0)
	}

	e.buf.Write(src)
	return nil
}

func (e *binaryEncoder) FromString(src string, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.String); err != nil {
		return err
	}
	if len(src) == 0 && e.canIgnoreField() {
		e.ignoreField()
		return nil
	}
	binaryEncodeString(e.buf, src)
	return nil
}

func (e *binaryEncoder) FromEnumLabel(src string, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.Enum); err != nil {
		return err
	}
	index := tt.EnumIndex(src)
	if index < 0 {
		return fmt.Errorf("enum label %q doesn't exist in type %q", src, tt)
	}
	if index == 0 && e.canIgnoreField() {
		e.ignoreField()
		return nil
	}
	binaryEncodeUint(e.buf, uint64(index))
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
	if src == vdl.AnyType && e.canIgnoreField() {
		e.ignoreField()
		return nil
	}
	binaryEncodeUint(e.buf, uint64(id))
	return nil
}

func (e *binaryEncoder) FromNil(tt *vdl.Type) error {
	if !tt.CanBeNil() {
		return errTypeMismatch(tt, vdl.Any, vdl.Optional)
	}
	if err := e.prepareTypeHelper(tt, true); err != nil {
		return err
	}
	if e.canIgnoreField() {
		e.ignoreField()
		return nil
	}
	binaryEncodeControl(e.buf, WireCtrlNil)
	return nil
}

func (e *binaryEncoder) StartList(tt *vdl.Type, len int) (vdl.ListTarget, error) {
	if err := e.prepareType(tt, vdl.Array, vdl.List); err != nil {
		return nil, err
	}
	if tt.Kind() == vdl.List {
		if len == 0 && e.canIgnoreField() {
			e.ignoreField()
		} else {
			binaryEncodeUint(e.buf, uint64(len))
		}
	} else {
		// We always encode array length to 0.
		binaryEncodeUint(e.buf, 0)
	}
	e.pushType(tt)
	return e, nil
}

func (e *binaryEncoder) StartSet(tt *vdl.Type, len int) (vdl.SetTarget, error) {
	if err := e.prepareType(tt, vdl.Set); err != nil {
		return nil, err
	}
	if len == 0 && e.canIgnoreField() {
		e.ignoreField()
	} else {
		binaryEncodeUint(e.buf, uint64(len))
	}
	e.pushType(tt)
	return e, nil
}

func (e *binaryEncoder) StartMap(tt *vdl.Type, len int) (vdl.MapTarget, error) {
	if err := e.prepareType(tt, vdl.Map); err != nil {
		return nil, err
	}
	if len == 0 && e.canIgnoreField() {
		e.ignoreField()
	} else {
		binaryEncodeUint(e.buf, uint64(len))
	}
	e.pushType(tt)
	return e, nil
}

func (e *binaryEncoder) StartFields(tt *vdl.Type) (vdl.FieldsTarget, error) {
	if err := e.prepareType(tt, vdl.Struct, vdl.Union); err != nil {
		return nil, err
	}
	e.pushType(tt)
	return e, nil
}

func (e *binaryEncoder) FinishList(vdl.ListTarget) error {
	return e.popType()
}

func (e *binaryEncoder) FinishSet(vdl.SetTarget) error {
	return e.popType()
}

func (e *binaryEncoder) FinishMap(vdl.MapTarget) error {
	return e.popType()
}

func (e *binaryEncoder) FinishFields(vdl.FieldsTarget) error {
	top, pos := e.topTypeAndPos()
	// Pop the type stack first to let canIgnoreField() see the correct
	// parent type.
	if err := e.popType(); err != nil {
		return err
	}
	if top.Kind() == vdl.Struct || (top.Kind() == vdl.Optional && top.Elem().Kind() == vdl.Struct) {
		// Write the struct terminator; don't write for union.
		if pos == e.buf.Len() && top.Kind() != vdl.Optional && e.canIgnoreField() {
			// Ignore the zero value only if it is not optional since we should
			// distinguish between empty and non-existent value. If we arrive here,
			// it means the current struct is empty, but not non-existent.
			e.ignoreField()
		} else {
			binaryEncodeControl(e.buf, WireCtrlEOF)
		}
	}
	return nil
}

func (e *binaryEncoder) StartElem(index int) (vdl.Target, error) {
	e.pushType(e.topType().Elem())
	return e, nil
}

func (e *binaryEncoder) FinishElem(elem vdl.Target) error {
	return e.popType()
}

func (e *binaryEncoder) StartKey() (vdl.Target, error) {
	e.pushType(e.topType().Key())
	return e, nil
}

func (e *binaryEncoder) FinishKey(key vdl.Target) error {
	return e.popType()
}

func (e *binaryEncoder) FinishKeyStartField(key vdl.Target) (vdl.Target, error) {
	if err := e.popType(); err != nil {
		return nil, err
	}
	e.pushType(e.topType().Elem())
	return e, nil
}

func (e *binaryEncoder) StartField(name string) (_, _ vdl.Target, _ error) {
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
		binaryEncodeUint(e.buf, uint64(index))
		return nil, e, nil
	}
	return nil, nil, fmt.Errorf("field name %q doesn't exist in top type %q", name, top)
}

func (e *binaryEncoder) FinishField(key, field vdl.Target) error {
	return e.popType()
}
