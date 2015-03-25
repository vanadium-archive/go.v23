// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"errors"
	"fmt"
	"io"
	"reflect"

	"v.io/v23/vdl"
)

var (
	errEncodeBadTypeStack = errors.New("vom: encoder has bad type stack")
	errEncodeNilType      = errors.New("vom: encoder finished with nil type")
	errEncodeZeroTypeId   = errors.New("vom: encoder finished with type Id 0")

	// Make sure Encoder implements the vdl *Target interfaces.
	_ vdl.Target       = (*Encoder)(nil)
	_ vdl.ListTarget   = (*Encoder)(nil)
	_ vdl.SetTarget    = (*Encoder)(nil)
	_ vdl.MapTarget    = (*Encoder)(nil)
	_ vdl.FieldsTarget = (*Encoder)(nil)
)

// Encoder manages the transmission and marshaling of typed values to the other
// side of a connection.
type Encoder struct {
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
	typeEnc *TypeEncoder
}

type typeStackEntry struct {
	tt  *vdl.Type
	pos int
}

// NewEncoder returns a new Encoder that writes to the given writer in the
// binary format. The binary format is compact and fast.
func NewEncoder(w io.Writer) (*Encoder, error) {
	if err := writeMagicByte(w); err != nil {
		return nil, err
	}
	return newEncoder(w, newTypeEncoder(w)), nil
}

// NewEncoderWithTypeEncoder returns a new Encoder that writes to the given
// writer in the binary format. Types will be encoded separately through the
// given typeEncoder.
func NewEncoderWithTypeEncoder(w io.Writer, typeEnc *TypeEncoder) (*Encoder, error) {
	if err := writeMagicByte(w); err != nil {
		return nil, err
	}
	return newEncoder(w, typeEnc), nil
}

func newEncoder(w io.Writer, typeEnc *TypeEncoder) *Encoder {
	return &Encoder{
		writer:    w,
		buf:       newEncbuf(),
		typeStack: make([]typeStackEntry, 0, 10),
		typeEnc:   typeEnc,
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
	if err := e.startEncode(); err != nil {
		return err
	}
	if err := vdl.FromReflect(e, reflect.ValueOf(v)); err != nil {
		return err
	}
	return e.finishEncode(0)
}

func (e *Encoder) encodeWireType(tid typeId, wt wireType) error {
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

func (e *Encoder) startEncode() error {
	e.buf.Reset()
	e.buf.Grow(paddingLen)
	e.typeStack = e.typeStack[:0]
	return nil
}

func (e *Encoder) finishEncode(tid typeId) error {
	switch {
	case len(e.typeStack) > 1:
		return errEncodeBadTypeStack
	case len(e.typeStack) == 0:
		return errEncodeNilType
	}
	encType := e.typeStack[0].tt
	if tid == 0 {
		if tid = e.typeEnc.lookupTypeId(encType); tid == 0 {
			return errEncodeZeroTypeId
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
	return fmt.Errorf("encoder type mismatch, got %q, want %v", t, kinds)
}

// prepareType prepares to encode a non-nil value of type tt, checking to make
// sure it has one of the specified kinds, and encoding any unsent types.
func (e *Encoder) prepareType(tt *vdl.Type, kinds ...vdl.Kind) error {
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
func (e *Encoder) prepareTypeHelper(tt *vdl.Type, fromNil bool) error {
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
	top := e.topType()
	// Handle the type id for Any values.
	switch {
	case top == nil:
		// Encoding the top-level. We postpone encoding of the tid until writeMsg
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

func (e *Encoder) pushType(tt *vdl.Type) {
	e.typeStack = append(e.typeStack, typeStackEntry{tt, e.buf.Len()})
}

func (e *Encoder) popType() error {
	if len(e.typeStack) == 0 {
		return errEncodeBadTypeStack
	}
	e.typeStack = e.typeStack[:len(e.typeStack)-1]
	return nil
}

func (e *Encoder) topType() *vdl.Type {
	if len(e.typeStack) == 0 {
		return nil
	}
	return e.typeStack[len(e.typeStack)-1].tt
}

func (e *Encoder) topTypeAndPos() (*vdl.Type, int) {
	if len(e.typeStack) == 0 {
		return nil, -1
	}
	entry := e.typeStack[len(e.typeStack)-1]
	return entry.tt, entry.pos
}

// canIgnoreField returns true if a zero-value field can be ignored.
func (e *Encoder) canIgnoreField() bool {
	if len(e.typeStack) < 2 {
		return false
	}
	secondTop := e.typeStack[len(e.typeStack)-2].tt
	return secondTop.Kind() == vdl.Struct || (secondTop.Kind() == vdl.Optional && secondTop.Elem().Kind() == vdl.Struct)
}

// ignoreField ignores the encoding output for the current field.
func (e *Encoder) ignoreField() {
	e.buf.Truncate(e.typeStack[len(e.typeStack)-1].pos)
}

// Implementation of vdl.Target interface.

func (e *Encoder) FromBool(src bool, tt *vdl.Type) error {
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

func (e *Encoder) FromUint(src uint64, tt *vdl.Type) error {
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

func (e *Encoder) FromInt(src int64, tt *vdl.Type) error {
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

func (e *Encoder) FromFloat(src float64, tt *vdl.Type) error {
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

func (e *Encoder) FromComplex(src complex128, tt *vdl.Type) error {
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

func (e *Encoder) FromBytes(src []byte, tt *vdl.Type) error {
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

func (e *Encoder) FromString(src string, tt *vdl.Type) error {
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

func (e *Encoder) FromEnumLabel(src string, tt *vdl.Type) error {
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

func (e *Encoder) FromTypeObject(src *vdl.Type) error {
	if err := e.prepareType(vdl.TypeObjectType, vdl.TypeObject); err != nil {
		return err
	}
	// Note that this function should never be called for wire types.
	tid, err := e.typeEnc.encode(src)
	if err != nil {
		return err
	}
	if src == vdl.AnyType && e.canIgnoreField() {
		e.ignoreField()
		return nil
	}
	binaryEncodeUint(e.buf, uint64(tid))
	return nil
}

func (e *Encoder) FromNil(tt *vdl.Type) error {
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

func (e *Encoder) StartList(tt *vdl.Type, len int) (vdl.ListTarget, error) {
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

func (e *Encoder) StartSet(tt *vdl.Type, len int) (vdl.SetTarget, error) {
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

func (e *Encoder) StartMap(tt *vdl.Type, len int) (vdl.MapTarget, error) {
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

func (e *Encoder) StartFields(tt *vdl.Type) (vdl.FieldsTarget, error) {
	if err := e.prepareType(tt, vdl.Struct, vdl.Union); err != nil {
		return nil, err
	}
	e.pushType(tt)
	return e, nil
}

func (e *Encoder) FinishList(vdl.ListTarget) error {
	return e.popType()
}

func (e *Encoder) FinishSet(vdl.SetTarget) error {
	return e.popType()
}

func (e *Encoder) FinishMap(vdl.MapTarget) error {
	return e.popType()
}

func (e *Encoder) FinishFields(vdl.FieldsTarget) error {
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
			binaryEncodeControl(e.buf, WireCtrlEnd)
		}
	}
	return nil
}

func (e *Encoder) StartElem(index int) (vdl.Target, error) {
	e.pushType(e.topType().Elem())
	return e, nil
}

func (e *Encoder) FinishElem(elem vdl.Target) error {
	return e.popType()
}

func (e *Encoder) StartKey() (vdl.Target, error) {
	e.pushType(e.topType().Key())
	return e, nil
}

func (e *Encoder) FinishKey(key vdl.Target) error {
	return e.popType()
}

func (e *Encoder) FinishKeyStartField(key vdl.Target) (vdl.Target, error) {
	if err := e.popType(); err != nil {
		return nil, err
	}
	e.pushType(e.topType().Elem())
	return e, nil
}

func (e *Encoder) StartField(name string) (_, _ vdl.Target, _ error) {
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

func (e *Encoder) FinishField(key, field vdl.Target) error {
	return e.popType()
}
