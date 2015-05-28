// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"io"
	"reflect"

	"v.io/v23/vdl"
	"v.io/v23/verror"
)

const pkgPath = "v.io/v23/vom"

var (
	errEncodeBadTypeStack   = verror.Register(pkgPath+".errEncodeBadTypeStack", verror.NoRetry, "{1:}{2:} vom: encoder has bad type stack{:_}")
	errEncodeNilType        = verror.Register(pkgPath+".errEncodeNilType", verror.NoRetry, "{1:}{2:} vom: encoder finished with nil type{:_}")
	errEncodeZeroTypeId     = verror.Register(pkgPath+".errEncodeZeroTypeId", verror.NoRetry, "{1:}{2:} vom: encoder finished with type Id 0{:_}")
	errEncoderTypeMismatch  = verror.Register(pkgPath+".errEncoderTypeMismatch", verror.NoRetry, "{1:}{2:} encoder type mismatch, got {3}, want {4}{:_}")
	errEncoderWantBytesType = verror.Register(pkgPath+".errEncoderWantBytesType", verror.NoRetry, "{1:}{2:} encoder type mismatch, got {3}, want bytes{:_}")
	errLabelNotInType       = verror.Register(pkgPath+".errLabelNotInType", verror.NoRetry, "{1:}{2:} enum label {3} doesn't exist in type {4}{:_}")
	errFieldNotInTopType    = verror.Register(pkgPath+".errFieldNotInTopType", verror.NoRetry, "{1:}{2:} field name {3} doesn't exist in top type {4}{:_}")
)

var (
	// Make sure encoder implements the vdl *Target interfaces.
	_ vdl.Target       = (*encoder)(nil)
	_ vdl.ListTarget   = (*encoder)(nil)
	_ vdl.SetTarget    = (*encoder)(nil)
	_ vdl.MapTarget    = (*encoder)(nil)
	_ vdl.FieldsTarget = (*encoder)(nil)
)

// Encoder manages the transmission and marshaling of typed values to the other
// side of a connection.
type Encoder struct {
	// The underlying implementation is hidden to avoid exposing the Target
	// interface methods.
	enc encoder
}

type encoder struct {
	writer io.Writer
	// We use buf to buffer up the encoded value. The buffering is necessary so
	// that we can compute the total message length.
	buf *encbuf
	// We maintain a typeStack, where typeStack[0] holds the type of the top-level
	// value being encoded, and subsequent layers of the stack holds type information
	// for composites and subtypes. Each entry also holds the start position of the
	// encoding buffer, which will be used to ignore zero value fields in structs.
	typeStack []typeStackEntry
	// All types are sent through typeEnc.
	typeEnc         *TypeEncoder
	sentVersionByte bool
}

type typeStackEntry struct {
	tt  *vdl.Type
	pos int
}

// NewEncoder returns a new Encoder that writes to the given writer in the
// binary format. The binary format is compact and fast.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{encoder{
		writer:          w,
		buf:             newEncbuf(),
		typeStack:       make([]typeStackEntry, 0, 10),
		typeEnc:         newTypeEncoderWithoutVersionByte(w),
		sentVersionByte: false,
	}}
}

// NewEncoderWithTypeEncoder returns a new Encoder that writes to the given
// writer in the binary format. Types will be encoded separately through the
// given typeEncoder.
func NewEncoderWithTypeEncoder(w io.Writer, typeEnc *TypeEncoder) *Encoder {
	return &Encoder{encoder{
		writer:          w,
		buf:             newEncbuf(),
		typeStack:       make([]typeStackEntry, 0, 10),
		typeEnc:         typeEnc,
		sentVersionByte: false,
	}}
}

func newEncoderWithoutVersionByte(w io.Writer, typeEnc *TypeEncoder) *encoder {
	return &encoder{
		writer:          w,
		buf:             newEncbuf(),
		typeStack:       make([]typeStackEntry, 0, 10),
		typeEnc:         typeEnc,
		sentVersionByte: true,
	}
}

// Encode transmits the value v. Values of type T are encodable as long as the
// type of T is representable as val.Type, or T is special-cased below;
// otherwise an error is returned.
//
//   Types that are special-cased, only for v:
//     *RawValue     - Transcode v into the appropriate output format.
//
//   Types that are special-cased, recursively throughout v:
//     *vdl.Value    - Encode the semantic value represented by v.
//     reflect.Value - Encode the semantic value represented by v.
//
// Encode(nil) is a special case that encodes the zero value of the any type.
// See the discussion of zero values in the Value documentation.
func (e *Encoder) Encode(v interface{}) error {
	if raw, ok := v.(*RawValue); ok && raw != nil {
		// TODO(toddw): Decode from RawValue, encoding into e.enc.
		_ = raw
		panic("Encode(RawValue) NOT IMPLEMENTED")
	}
	if !e.enc.sentVersionByte {
		if err := writeVersionByte(e.enc.writer); err != nil {
			return err
		}
		e.enc.sentVersionByte = true
	}
	if err := e.enc.startEncode(); err != nil {
		return err
	}
	if err := vdl.FromReflect(&e.enc, reflect.ValueOf(v)); err != nil {
		return err
	}
	return e.enc.finishEncode(0)
}

func (e *encoder) encodeWireType(tid typeId, wt wireType) error {
	if err := e.startEncode(); err != nil {
		return err
	}
	if err := vdl.FromReflect(e, reflect.ValueOf(wt)); err != nil {
		return err
	}
	// We encode the negative id for type definitions.
	return e.finishEncode(-tid)
}

// paddingLen must be large enough to hold the header in writeMsg.
const paddingLen = maxEncodedUintBytes * 2

func (e *encoder) startEncode() error {
	e.buf.Reset()
	e.buf.Grow(paddingLen)
	e.typeStack = e.typeStack[:0]
	return nil
}

func (e *encoder) finishEncode(tid typeId) error {
	switch {
	case len(e.typeStack) > 1:
		return verror.New(errEncodeBadTypeStack, nil)
	case len(e.typeStack) == 0:
		return verror.New(errEncodeNilType, nil)
	}
	encType := e.typeStack[0].tt
	if tid == 0 {
		if tid = e.typeEnc.lookupTypeId(encType); tid == 0 {
			return verror.New(errEncodeZeroTypeId, nil)
		}
	}
	// Binary messages always start with a typeId, sometimes followed by the byte
	// length of the rest of the message. We only know the byte length after
	// we've encoded the rest of the message. To make this reasonably efficient,
	// the buffer is initialized with enough padding to hold the id and length,
	// and we go back and fill them in here.
	//
	// The binaryEncode*End methods fill in the trailing bytes of the buffer and
	// return the start index of the encoded data. Thus the binaryEncode*End
	// calls here are in the opposite order they appear in the encoded message.
	msg := e.buf.Bytes()
	header := msg[:paddingLen]
	if hasBinaryMsgLen(encType) {
		start := binaryEncodeUintEnd(header, uint64(len(msg)-paddingLen))
		header = header[:start]
	}
	// Note that we encode the negative id for type definitions.
	start := binaryEncodeIntEnd(header, int64(tid))
	_, err := e.writer.Write(msg[start:])
	return err
}

func errTypeMismatch(t *vdl.Type, kinds ...vdl.Kind) error {
	return verror.New(errEncoderTypeMismatch, nil, t, kinds)
}

// prepareType prepares to encode a non-nil value of type tt, checking to make
// sure it has one of the specified kinds, and encoding any unsent types.
func (e *encoder) prepareType(tt *vdl.Type, kinds ...vdl.Kind) error {
	for _, k := range kinds {
		if tt.Kind() == k || tt.Kind() == vdl.Optional && tt.Elem().Kind() == k {
			return e.prepareTypeHelper(tt, false)
		}
	}
	return errTypeMismatch(tt, kinds...)
}

// prepareTypeHelper encodes any unsent types, and manages the type stack. If
// fromNil is true, we skip encoding the typeid for any type, since we'll be
// encoding a nil instead.
func (e *encoder) prepareTypeHelper(tt *vdl.Type, fromNil bool) error {
	var tid typeId
	// Check the bootstrap wire types first to avoid recursive calls to the type
	// encoder for wire types.
	if _, exists := bootstrapWireTypes[tt]; !exists {
		var err error
		tid, err = e.typeEnc.encode(tt)
		if err != nil {
			return err
		}
	}
	// Handle the type id for Any values.
	switch {
	case len(e.typeStack) == 0:
		// Encoding the top-level. We postpone encoding of the tid until writeMsg
		// is called, to handle positive and negative ids, and the message length.
		e.pushType(tt)
	case !fromNil && e.topType().Kind() == vdl.Any:
		binaryEncodeUint(e.buf, uint64(tid))
	}
	return nil
}

func (e *encoder) pushType(tt *vdl.Type) {
	e.typeStack = append(e.typeStack, typeStackEntry{tt, e.buf.Len()})
}

func (e *encoder) popType() error {
	if len(e.typeStack) == 0 {
		return verror.New(errEncodeBadTypeStack, nil)
	}
	e.typeStack = e.typeStack[:len(e.typeStack)-1]
	return nil
}

func (e *encoder) topType() *vdl.Type {
	return e.typeStack[len(e.typeStack)-1].tt
}

func (e *encoder) topTypeAndPos() (*vdl.Type, int) {
	entry := e.typeStack[len(e.typeStack)-1]
	return entry.tt, entry.pos
}

// canIgnoreField returns true iff a zero-value field can be ignored.
func (e *encoder) canIgnoreField(fromNil bool) bool {
	if len(e.typeStack) < 2 {
		return false
	}
	if !fromNil && e.typeStack[len(e.typeStack)-1].tt.Kind() == vdl.Any {
		// Struct fields of type any can only be ignored if the value is nil.
		// E.g. type X{ A any } can ignore X{nil}, but can't ignore X{false}.
		return false
	}
	top2 := e.typeStack[len(e.typeStack)-2].tt
	return top2.Kind() == vdl.Struct || (top2.Kind() == vdl.Optional && top2.Elem().Kind() == vdl.Struct)
}

// ignoreField ignores the encoding output for the current field.
func (e *encoder) ignoreField() {
	e.buf.Truncate(e.typeStack[len(e.typeStack)-1].pos)
}

// Implementation of vdl.Target interface.

func (e *encoder) FromBool(src bool, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.Bool); err != nil {
		return err
	}
	if src == false && e.canIgnoreField(false) {
		e.ignoreField()
		return nil
	}
	binaryEncodeBool(e.buf, src)
	return nil
}

func (e *encoder) FromUint(src uint64, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64); err != nil {
		return err
	}
	if src == 0 && e.canIgnoreField(false) {
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

func (e *encoder) FromInt(src int64, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.Int16, vdl.Int32, vdl.Int64); err != nil {
		return err
	}
	if src == 0 && e.canIgnoreField(false) {
		e.ignoreField()
		return nil
	}
	binaryEncodeInt(e.buf, src)
	return nil
}

func (e *encoder) FromFloat(src float64, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.Float32, vdl.Float64); err != nil {
		return err
	}
	if src == 0 && e.canIgnoreField(false) {
		e.ignoreField()
		return nil
	}
	binaryEncodeFloat(e.buf, src)
	return nil
}

func (e *encoder) FromComplex(src complex128, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.Complex64, vdl.Complex128); err != nil {
		return err
	}
	if src == 0 && e.canIgnoreField(false) {
		e.ignoreField()
		return nil
	}
	binaryEncodeFloat(e.buf, real(src))
	binaryEncodeFloat(e.buf, imag(src))
	return nil
}

func (e *encoder) FromBytes(src []byte, tt *vdl.Type) error {
	if !tt.IsBytes() {
		return verror.New(errEncoderWantBytesType, nil, tt)
	}
	if err := e.prepareTypeHelper(tt, false); err != nil {
		return err
	}
	if tt.Kind() == vdl.List {
		if len(src) == 0 && e.canIgnoreField(false) {
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

func (e *encoder) FromString(src string, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.String); err != nil {
		return err
	}
	if len(src) == 0 && e.canIgnoreField(false) {
		e.ignoreField()
		return nil
	}
	binaryEncodeString(e.buf, src)
	return nil
}

func (e *encoder) FromEnumLabel(src string, tt *vdl.Type) error {
	if err := e.prepareType(tt, vdl.Enum); err != nil {
		return err
	}
	index := tt.EnumIndex(src)
	if index < 0 {
		return verror.New(errLabelNotInType, nil, src, tt)
	}
	if index == 0 && e.canIgnoreField(false) {
		e.ignoreField()
		return nil
	}
	binaryEncodeUint(e.buf, uint64(index))
	return nil
}

func (e *encoder) FromTypeObject(src *vdl.Type) error {
	if err := e.prepareType(vdl.TypeObjectType, vdl.TypeObject); err != nil {
		return err
	}
	// Note that this function should never be called for wire types.
	tid, err := e.typeEnc.encode(src)
	if err != nil {
		return err
	}
	if src.Kind() == vdl.Any && e.canIgnoreField(false) {
		e.ignoreField()
		return nil
	}
	binaryEncodeUint(e.buf, uint64(tid))
	return nil
}

func (e *encoder) FromNil(tt *vdl.Type) error {
	if !tt.CanBeNil() {
		return errTypeMismatch(tt, vdl.Any, vdl.Optional)
	}
	if err := e.prepareTypeHelper(tt, true); err != nil {
		return err
	}
	if e.canIgnoreField(true) {
		e.ignoreField()
		return nil
	}
	binaryEncodeControl(e.buf, WireCtrlNil)
	return nil
}

func (e *encoder) StartList(tt *vdl.Type, len int) (vdl.ListTarget, error) {
	if err := e.prepareType(tt, vdl.Array, vdl.List); err != nil {
		return nil, err
	}
	if tt.Kind() == vdl.List {
		if len == 0 && e.canIgnoreField(false) {
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

func (e *encoder) StartSet(tt *vdl.Type, len int) (vdl.SetTarget, error) {
	if err := e.prepareType(tt, vdl.Set); err != nil {
		return nil, err
	}
	if len == 0 && e.canIgnoreField(false) {
		e.ignoreField()
	} else {
		binaryEncodeUint(e.buf, uint64(len))
	}
	e.pushType(tt)
	return e, nil
}

func (e *encoder) StartMap(tt *vdl.Type, len int) (vdl.MapTarget, error) {
	if err := e.prepareType(tt, vdl.Map); err != nil {
		return nil, err
	}
	if len == 0 && e.canIgnoreField(false) {
		e.ignoreField()
	} else {
		binaryEncodeUint(e.buf, uint64(len))
	}
	e.pushType(tt)
	return e, nil
}

func (e *encoder) StartFields(tt *vdl.Type) (vdl.FieldsTarget, error) {
	if err := e.prepareType(tt, vdl.Struct, vdl.Union); err != nil {
		return nil, err
	}
	e.pushType(tt)
	return e, nil
}

func (e *encoder) FinishList(vdl.ListTarget) error {
	return e.popType()
}

func (e *encoder) FinishSet(vdl.SetTarget) error {
	return e.popType()
}

func (e *encoder) FinishMap(vdl.MapTarget) error {
	return e.popType()
}

func (e *encoder) FinishFields(vdl.FieldsTarget) error {
	top, pos := e.topTypeAndPos()
	// Pop the type stack first to let canIgnoreField() see the correct
	// parent type.
	if err := e.popType(); err != nil {
		return err
	}
	if top.Kind() == vdl.Struct || (top.Kind() == vdl.Optional && top.Elem().Kind() == vdl.Struct) {
		// Write the struct terminator; don't write for union.
		if pos == e.buf.Len() && top.Kind() != vdl.Optional && e.canIgnoreField(false) {
			// Ignore the zero value only if it is not optional since we should
			// distinguish between empty and non-existent value. If we arrive here,
			// it means the current struct is empty, but not non-existent.
			e.ignoreField()
		} else {
			binaryEncodeControl(e.buf, WireCtrlEnd)
		}
	}
	return nil
}

func (e *encoder) StartElem(index int) (vdl.Target, error) {
	e.pushType(e.topType().Elem())
	return e, nil
}

func (e *encoder) FinishElem(elem vdl.Target) error {
	return e.popType()
}

func (e *encoder) StartKey() (vdl.Target, error) {
	e.pushType(e.topType().Key())
	return e, nil
}

func (e *encoder) FinishKey(key vdl.Target) error {
	return e.popType()
}

func (e *encoder) FinishKeyStartField(key vdl.Target) (vdl.Target, error) {
	if err := e.popType(); err != nil {
		return nil, err
	}
	e.pushType(e.topType().Elem())
	return e, nil
}

func (e *encoder) StartField(name string) (_, _ vdl.Target, _ error) {
	top := e.topType()
	if top.Kind() == vdl.Optional {
		top = top.Elem()
	}
	if k := top.Kind(); k != vdl.Struct && k != vdl.Union {
		return nil, nil, errTypeMismatch(top, vdl.Struct, vdl.Union)
	}
	// Struct and Union are encoded as a sequence of fields, in any order.  Each
	// field starts with its absolute 0-based index, followed by the value.  Union
	// always consists of a single field, while structs use a CtrlEnd terminator.
	if vfield, index := top.FieldByName(name); index >= 0 {
		e.pushType(vfield.Type)
		binaryEncodeUint(e.buf, uint64(index))
		return nil, e, nil
	}
	return nil, nil, verror.New(errFieldNotInTopType, nil, name, top)
}

func (e *encoder) FinishField(key, field vdl.Target) error {
	return e.popType()
}
