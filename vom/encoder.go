// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"io"
	"reflect"

	"fmt"

	"v.io/v23/vdl"
	"v.io/v23/verror"
)

type Version byte

const (
	Version80 = Version(0x80)
	Version81 = Version(0x81)
)

var (
	errEncodeBadTypeStack      = verror.Register(pkgPath+".errEncodeBadTypeStack", verror.NoRetry, "{1:}{2:} vom: encoder has bad type stack{:_}")
	errEncodeNilType           = verror.Register(pkgPath+".errEncodeNilType", verror.NoRetry, "{1:}{2:} vom: encoder finished with nil type{:_}")
	errEncoderTypeMismatch     = verror.Register(pkgPath+".errEncoderTypeMismatch", verror.NoRetry, "{1:}{2:} encoder type mismatch, got {3}, want {4}{:_}")
	errEncoderWantBytesType    = verror.Register(pkgPath+".errEncoderWantBytesType", verror.NoRetry, "{1:}{2:} encoder type mismatch, got {3}, want bytes{:_}")
	errLabelNotInType          = verror.Register(pkgPath+".errLabelNotInType", verror.NoRetry, "{1:}{2:} enum label {3} doesn't exist in type {4}{:_}")
	errFieldNotInTopType       = verror.Register(pkgPath+".errFieldNotInTopType", verror.NoRetry, "{1:}{2:} field name {3} doesn't exist in top type {4}{:_}")
	errUnsupportedInVOMVersion = verror.Register(pkgPath+".errUnsupportedInVOMVersion", verror.NoRetry, "{1:}{2:} {3} unsupported in vom version {4}{:_}")
)

var (
	rtPtrToValue = reflect.TypeOf((*vdl.Value)(nil))
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
	buf *switchedEncbuf
	// We maintain a typeStack, where typeStack[0] holds the type of the top-level
	// value being encoded, and subsequent layers of the stack holds type information
	// for composites and subtypes. Each entry also holds the start position of the
	// encoding buffer, which will be used to ignore zero value fields in structs.
	typeStack []typeStackEntry
	// All types are sent through typeEnc.
	typeEnc         *TypeEncoder
	sentVersionByte bool
	version         Version
}

type typeStackEntry struct {
	tt         *vdl.Type
	fieldIndex int // -1 for if it is not in a struct
}

// NewEncoder returns a new Encoder that writes to the given writer in the
// binary format. The binary format is compact and fast.
func NewEncoder(w io.Writer) *Encoder {
	return NewVersionedEncoder(Version80, w)
}

// NewVersionedEncoder returns a new Encoder that writes to the given writer with
// the specified VOM version.
func NewVersionedEncoder(version Version, w io.Writer) *Encoder {
	if !isAllowedVersion(version) {
		panic(fmt.Sprintf("unsupported VOM version: %x", version))
	}
	return &Encoder{encoder{
		writer:          w,
		buf:             newSwitchedEncbuf(w, version),
		typeStack:       make([]typeStackEntry, 0, 10),
		typeEnc:         newTypeEncoderWithoutVersionByte(version, w),
		sentVersionByte: false,
		version:         version,
	}}
}

// NewEncoderWithTypeEncoder returns a new Encoder that writes to the given
// writer in the binary format. Types will be encoded separately through the
// given typeEncoder.
func NewEncoderWithTypeEncoder(w io.Writer, typeEnc *TypeEncoder) *Encoder {
	return NewVersionedEncoderWithTypeEncoder(Version80, w, typeEnc)
}

// NewVersionedEncoderWithTypeEncoder returns a new Encoder that writes to the given
// writer in the binary format. Types will be encoded separately through the
// given typeEncoder.
func NewVersionedEncoderWithTypeEncoder(version Version, w io.Writer, typeEnc *TypeEncoder) *Encoder {
	if !isAllowedVersion(version) {
		panic(fmt.Sprintf("unsupported VOM version: %x", version))
	}
	return &Encoder{encoder{
		writer:          w,
		buf:             newSwitchedEncbuf(w, version),
		typeStack:       make([]typeStackEntry, 0, 10),
		typeEnc:         typeEnc,
		sentVersionByte: false,
		version:         version,
	}}
}

func newEncoderWithoutVersionByte(version Version, w io.Writer, typeEnc *TypeEncoder) *encoder {
	if !isAllowedVersion(version) {
		panic(fmt.Sprintf("unsupported VOM version: %x", version))
	}
	return &encoder{
		writer:          w,
		buf:             newSwitchedEncbuf(w, version),
		typeStack:       make([]typeStackEntry, 0, 10),
		typeEnc:         typeEnc,
		sentVersionByte: true,
		version:         version,
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
	if !e.enc.sentVersionByte {
		if _, err := e.enc.writer.Write([]byte{byte(e.enc.version)}); err != nil {
			return err
		}
		e.enc.sentVersionByte = true
	}
	if raw, ok := v.(*RawValue); ok && raw != nil {
		return e.encodeRaw(raw)
	}
	vdlType := extractType(v)
	tid, err := e.enc.typeEnc.encode(vdlType)
	if err != nil {
		return err
	}
	if err := e.enc.startEncode(containsAnyOrTypeObject(vdlType), hasChunkLen(vdlType), false, int64(tid)); err != nil {
		return err
	}
	if err := vdl.FromReflect(&e.enc, reflect.ValueOf(v)); err != nil {
		return err
	}
	return e.enc.finishEncode()
}

func (e *Encoder) encodeRaw(raw *RawValue) error {
	if e.enc.version == Version80 {
		return verror.New(errUnsupportedInVOMVersion, nil, e.enc.version, "RawValue")
	}
	tid, err := e.enc.typeEnc.encode(raw.t)
	if err != nil {
		return err
	}
	if err := e.enc.buf.StartMessage(containsAnyOrTypeObject(raw.t), hasChunkLen(raw.t), false, int64(tid)); err != nil {
		return err
	}
	for i, refType := range raw.refTypes {
		mid, err := e.enc.typeEnc.encode(refType)
		if err != nil {
			return err
		}
		if index := e.enc.buf.ReferenceTypeID(mid); index != uint64(i) {
			return verror.New(verror.ErrInternal, nil, "index unexpectedly out of order")
		}
	}
	if err := e.enc.buf.Write(raw.value); err != nil {
		return err
	}
	return e.enc.buf.FinishMessage()
}

// TODO(bprosnitz) Remove -- this is copied from vdl
func extractType(v interface{}) *vdl.Type {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr && !rv.IsNil() {
		if rv.Type().ConvertibleTo(rtPtrToValue) {
			return rv.Convert(rtPtrToValue).Interface().(*vdl.Value).Type()
		}
		rv = rv.Elem()
	}
	return vdl.TypeOf(v)
}

func (e *encoder) encodeWireType(tid typeId, wt wireType, typeIncomplete bool) error {
	if err := e.startEncode(false, true, typeIncomplete, int64(-tid)); err != nil {
		return err
	}
	if err := vdl.FromReflect(e, reflect.ValueOf(wt)); err != nil {
		return err
	}
	// We encode the negative id for type definitions.
	return e.finishEncode()
}

func (e *encoder) startEncode(hasAny, hasLen, typeIncomplete bool, mid int64) error {
	if err := e.buf.StartMessage(hasAny, hasLen, typeIncomplete, int64(mid)); err != nil {
		return err
	}
	e.typeStack = e.typeStack[:0]
	return nil
}

func (e *encoder) finishEncode() error {
	switch {
	case len(e.typeStack) > 1:
		return verror.New(errEncodeBadTypeStack, nil)
	case len(e.typeStack) == 0:
		return verror.New(errEncodeNilType, nil)
	}
	return e.buf.FinishMessage()
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
		if e.isStructFieldValue() {
			binaryEncodeUint(e.buf, uint64(e.topTypeFieldIndex()))
		}
		if e.version == Version80 {
			binaryEncodeUint(e.buf, uint64(tid))
		} else {
			binaryEncodeUint(e.buf, e.buf.ReferenceTypeID(tid))
		}
	}
	return nil
}

func (e *encoder) pushType(tt *vdl.Type) {
	e.typeStack = append(e.typeStack, typeStackEntry{tt, -1})
}

func (e *encoder) pushFieldType(tt *vdl.Type, index int) {
	e.typeStack = append(e.typeStack, typeStackEntry{tt, index})
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

func (e *encoder) isStructFieldValue() bool {
	return e.topTypeFieldIndex() >= 0
}

func (e *encoder) topTypeFieldIndex() int {
	if len(e.typeStack) == 0 {
		return -1
	}
	return e.typeStack[len(e.typeStack)-1].fieldIndex
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

// Implementation of vdl.Target interface.
var boolAllowed = []vdl.Kind{vdl.Bool}

func (e *encoder) FromBool(src bool, tt *vdl.Type) error {
	if err := e.prepareType(tt, boolAllowed...); err != nil {
		return err
	}
	if e.isStructFieldValue() && e.topType().Kind() != vdl.Any {
		if src == true || !e.canIgnoreField(false) {
			binaryEncodeUint(e.buf, uint64(e.topTypeFieldIndex()))
		} else {
			return nil
		}
	}
	binaryEncodeBool(e.buf, src)
	return nil
}

var uintAllowed = []vdl.Kind{vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64}

func (e *encoder) FromUint(src uint64, tt *vdl.Type) error {
	if err := e.prepareType(tt, uintAllowed...); err != nil {
		return err
	}
	if e.isStructFieldValue() && e.topType().Kind() != vdl.Any {
		if src != 0 || !e.canIgnoreField(false) {
			binaryEncodeUint(e.buf, uint64(e.topTypeFieldIndex()))
		} else {
			return nil
		}
	}
	if e.version == Version80 && tt.Kind() == vdl.Byte {
		e.buf.WriteOneByte(byte(src))
	} else {
		binaryEncodeUint(e.buf, src)
	}
	return nil
}

var intAllowed = []vdl.Kind{vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64}

func (e *encoder) FromInt(src int64, tt *vdl.Type) error {
	if err := e.prepareType(tt, intAllowed...); err != nil {
		return err
	}
	if e.version == Version80 && tt.Kind() == vdl.Int8 {
		return verror.New(errUnsupportedInVOMVersion, nil, "int8", e.version)
	}
	if e.isStructFieldValue() && e.topType().Kind() != vdl.Any {
		if src != 0 || !e.canIgnoreField(false) {
			binaryEncodeUint(e.buf, uint64(e.topTypeFieldIndex()))
		} else {
			return nil
		}
	}
	binaryEncodeInt(e.buf, src)
	return nil
}

var floatAllowed = []vdl.Kind{vdl.Float32, vdl.Float64}

func (e *encoder) FromFloat(src float64, tt *vdl.Type) error {
	if err := e.prepareType(tt, floatAllowed...); err != nil {
		return err
	}
	if e.isStructFieldValue() && e.topType().Kind() != vdl.Any {
		if src != 0 || !e.canIgnoreField(false) {
			binaryEncodeUint(e.buf, uint64(e.topTypeFieldIndex()))
		} else {
			return nil
		}
	}
	binaryEncodeFloat(e.buf, src)
	return nil
}

var complexAllowed = []vdl.Kind{vdl.Complex64, vdl.Complex128}

func (e *encoder) FromComplex(src complex128, tt *vdl.Type) error {
	if err := e.prepareType(tt, complexAllowed...); err != nil {
		return err
	}
	if e.isStructFieldValue() && e.topType().Kind() != vdl.Any {
		if src != 0 || !e.canIgnoreField(false) {
			binaryEncodeUint(e.buf, uint64(e.topTypeFieldIndex()))
		} else {
			return nil
		}
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
		if e.isStructFieldValue() && e.topType().Kind() != vdl.Any {
			if len(src) != 0 || !e.canIgnoreField(false) {
				binaryEncodeUint(e.buf, uint64(e.topTypeFieldIndex()))
			} else {
				return nil
			}
		}
		binaryEncodeUint(e.buf, uint64(len(src)))
	} else {
		// We always encode array length to 0.
		if e.isStructFieldValue() && e.topType().Kind() != vdl.Any {
			var orData byte
			for _, b := range src {
				orData |= b
			}
			if orData != 0 || !e.canIgnoreField(false) {
				binaryEncodeUint(e.buf, uint64(e.topTypeFieldIndex()))
			} else {
				return nil
			}
		}
		binaryEncodeUint(e.buf, uint64(0))
	}

	e.buf.Write(src)
	return nil
}

var stringAllowed = []vdl.Kind{vdl.String}

func (e *encoder) FromString(src string, tt *vdl.Type) error {
	if err := e.prepareType(tt, stringAllowed...); err != nil {
		return err
	}
	if e.isStructFieldValue() && e.topType().Kind() != vdl.Any {
		if len(src) != 0 || !e.canIgnoreField(false) {
			binaryEncodeUint(e.buf, uint64(e.topTypeFieldIndex()))
		} else {
			return nil
		}
	}
	binaryEncodeString(e.buf, src)
	return nil
}

var enumAllowed = []vdl.Kind{vdl.Enum}

func (e *encoder) FromEnumLabel(src string, tt *vdl.Type) error {
	if err := e.prepareType(tt, enumAllowed...); err != nil {
		return err
	}
	index := tt.EnumIndex(src)
	if index < 0 {
		return verror.New(errLabelNotInType, nil, src, tt)
	}
	if e.isStructFieldValue() && e.topType().Kind() != vdl.Any {
		if index != 0 || !e.canIgnoreField(false) {
			binaryEncodeUint(e.buf, uint64(e.topTypeFieldIndex()))
		} else {
			return nil
		}
	}
	binaryEncodeUint(e.buf, uint64(index))
	return nil
}

var typeObjectAllowed = []vdl.Kind{vdl.TypeObject}

func (e *encoder) FromTypeObject(src *vdl.Type) error {
	if err := e.prepareType(vdl.TypeObjectType, typeObjectAllowed...); err != nil {
		return err
	}
	// Note that this function should never be called for wire types.
	tid, err := e.typeEnc.encode(src)
	if err != nil {
		return err
	}
	if e.version == Version80 {
		if e.isStructFieldValue() && e.topType().Kind() != vdl.Any {
			if src.Kind() != vdl.Any || !e.canIgnoreField(true) {
				binaryEncodeUint(e.buf, uint64(e.topTypeFieldIndex()))
				binaryEncodeUint(e.buf, uint64(tid))
			}
		} else {
			binaryEncodeUint(e.buf, uint64(tid))
		}
	} else {
		if e.isStructFieldValue() && e.topType().Kind() != vdl.Any {
			if src.Kind() != vdl.Any || !e.canIgnoreField(true) {
				binaryEncodeUint(e.buf, uint64(e.topTypeFieldIndex()))
				binaryEncodeUint(e.buf, e.buf.ReferenceTypeID(tid))
			}
		} else {
			binaryEncodeUint(e.buf, e.buf.ReferenceTypeID(tid))
		}
	}
	return nil
}

var nilAllowed = []vdl.Kind{vdl.Any, vdl.Optional}

func (e *encoder) FromNil(tt *vdl.Type) error {
	if !tt.CanBeNil() {
		return errTypeMismatch(tt, nilAllowed...)
	}
	if err := e.prepareTypeHelper(tt, true); err != nil {
		return err
	}
	switch tt.Kind() {
	case vdl.Optional:
		if e.isStructFieldValue() {
			if !e.canIgnoreField(true) && e.topType().Kind() != vdl.Any {
				binaryEncodeUint(e.buf, uint64(e.topTypeFieldIndex()))
				e.buf.WriteOneByte(WireCtrlNil)
			}
		} else {
			e.buf.WriteOneByte(WireCtrlNil)
		}
	case vdl.Any:
		if e.isStructFieldValue() {
			if !e.canIgnoreField(true) && e.topType().Kind() != vdl.Any {
				e.buf.WriteOneByte(WireCtrlNil)
			}
		} else {
			e.buf.WriteOneByte(WireCtrlNil)
		}
	}
	return nil
}

var listAllowed = []vdl.Kind{vdl.Array, vdl.List}

func (e *encoder) StartList(tt *vdl.Type, len int) (vdl.ListTarget, error) {
	if err := e.prepareType(tt, listAllowed...); err != nil {
		return nil, err
	}
	if tt.Kind() == vdl.List {
		if e.isStructFieldValue() && e.topType().Kind() != vdl.Any {
			if len != 0 || !e.canIgnoreField(false) {
				binaryEncodeUint(e.buf, uint64(e.topTypeFieldIndex()))
				binaryEncodeUint(e.buf, uint64(len))
			}
		} else {
			binaryEncodeUint(e.buf, uint64(len))
		}
	} else {
		// We always encode array length to 0.
		if e.isStructFieldValue() && e.topType().Kind() != vdl.Any {
			if len != 0 || !e.canIgnoreField(false) {
				binaryEncodeUint(e.buf, uint64(e.topTypeFieldIndex()))
				binaryEncodeUint(e.buf, uint64(0))
			}
		} else {
			binaryEncodeUint(e.buf, uint64(0))
		}
	}
	e.pushType(tt)
	return e, nil
}

var setAllowed = []vdl.Kind{vdl.Set}

func (e *encoder) StartSet(tt *vdl.Type, len int) (vdl.SetTarget, error) {
	if err := e.prepareType(tt, setAllowed...); err != nil {
		return nil, err
	}
	if e.isStructFieldValue() && e.topType().Kind() != vdl.Any {
		if len != 0 || !e.canIgnoreField(false) {
			binaryEncodeUint(e.buf, uint64(e.topTypeFieldIndex()))
			binaryEncodeUint(e.buf, uint64(len))
		}
	} else {
		binaryEncodeUint(e.buf, uint64(len))
	}
	e.pushType(tt)
	return e, nil
}

var mapAllowed = []vdl.Kind{vdl.Map}

func (e *encoder) StartMap(tt *vdl.Type, len int) (vdl.MapTarget, error) {
	if err := e.prepareType(tt, mapAllowed...); err != nil {
		return nil, err
	}
	if e.isStructFieldValue() && e.topType().Kind() != vdl.Any {
		if len != 0 || !e.canIgnoreField(false) {
			binaryEncodeUint(e.buf, uint64(e.topTypeFieldIndex()))
			binaryEncodeUint(e.buf, uint64(len))
		}
	} else {
		binaryEncodeUint(e.buf, uint64(len))
	}
	e.pushType(tt)
	return e, nil
}

var fieldsAllowed = []vdl.Kind{vdl.Struct, vdl.Union}

func (e *encoder) StartFields(tt *vdl.Type) (vdl.FieldsTarget, error) {
	if e.isStructFieldValue() && e.topType().Kind() != vdl.Any {
		// TODO(bprosnitz) We shouldn't need to write the struct field index for fields that are empty structs/unions
		binaryEncodeUint(e.buf, uint64(e.topTypeFieldIndex()))
	}
	if err := e.prepareType(tt, fieldsAllowed...); err != nil {
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
	top := e.topType()
	// Pop the type stack first to let canIgnoreField() see the correct
	// parent type.
	if err := e.popType(); err != nil {
		return err
	}
	if top.Kind() == vdl.Struct || (top.Kind() == vdl.Optional && top.Elem().Kind() == vdl.Struct) {
		// Write the struct terminator; don't write for union.
		e.buf.WriteOneByte(WireCtrlEnd)
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
		e.pushFieldType(vfield.Type, index)
		return nil, e, nil
	}
	return nil, nil, verror.New(errFieldNotInTopType, nil, name, top)
}

func (e *encoder) FinishField(key, field vdl.Target) error {
	return e.popType()
}
