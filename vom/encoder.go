// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"fmt"
	"io"
	"reflect"

	"v.io/v23/vdl"
	"v.io/v23/verror"
)

func (v Version) String() string {
	return fmt.Sprintf("Version%x", byte(v))
}

var (
	errEncodeBadTypeStack      = verror.Register(pkgPath+".errEncodeBadTypeStack", verror.NoRetry, "{1:}{2:} vom: encoder has bad type stack{:_}")
	errEncodeNilType           = verror.Register(pkgPath+".errEncodeNilType", verror.NoRetry, "{1:}{2:} vom: encoder finished with nil type{:_}")
	errEncoderTypeMismatch     = verror.Register(pkgPath+".errEncoderTypeMismatch", verror.NoRetry, "{1:}{2:} encoder type mismatch, got {3}, want {4}{:_}")
	errEncoderWantBytesType    = verror.Register(pkgPath+".errEncoderWantBytesType", verror.NoRetry, "{1:}{2:} encoder type mismatch, got {3}, want bytes{:_}")
	errLabelNotInType          = verror.Register(pkgPath+".errLabelNotInType", verror.NoRetry, "{1:}{2:} enum label {3} doesn't exist in type {4}{:_}")
	errFieldNotInTopType       = verror.Register(pkgPath+".errFieldNotInTopType", verror.NoRetry, "{1:}{2:} field name {3} doesn't exist in top type {4}{:_}")
	errUnsupportedInVOMVersion = verror.Register(pkgPath+".errUnsupportedInVOMVersion", verror.NoRetry, "{1:}{2:} {3} unsupported in vom version {4}{:_}")
	errUnusedTypeIds           = verror.Register(pkgPath+".errUnusedTypeIds", verror.NoRetry, "{1:}{2:} vom: some type ids unused during encode {:_}")
	errUnusedAnys              = verror.Register(pkgPath+".errUnusedAnys", verror.NoRetry, "{1:}{2:} vom: some anys unused during encode {:_}")
)

const (
	typeIDListInitialSize = 16
	anyLenListInitialSize = 16
)

// paddingLen must be large enough to hold the header in writeMsg.
const paddingLen = maxEncodedUintBytes * 2

var (
	rtPtrToValue    = reflect.TypeOf((*vdl.Value)(nil))
	rtRawBytes      = reflect.TypeOf(RawBytes{})
	rtPtrToRawBytes = reflect.TypeOf((*RawBytes)(nil))
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
	version         Version

	tids    *typeIDList
	anyLens *anyLenList

	hasLen, hasAny, hasTypeObject bool
	typeIncomplete                bool
	mid                           int64 // message id
}

type typeStackEntry struct {
	tt          *vdl.Type
	fieldIndex  int          // -1 for if it is not in a struct
	anyStartRef *anyStartRef // only non-nil for any
}

// NewEncoder returns a new Encoder that writes to the given writer in the
// binary format. The binary format is compact and fast.
func NewEncoder(w io.Writer) *Encoder {
	return NewVersionedEncoder(DefaultVersion, w)
}

// NewVersionedEncoder returns a new Encoder that writes to the given writer with
// the specified VOM version.
func NewVersionedEncoder(version Version, w io.Writer) *Encoder {
	if !isAllowedVersion(version) {
		panic(fmt.Sprintf("unsupported VOM version: %x", version))
	}
	return &Encoder{encoder{
		writer:          w,
		buf:             newEncbuf(),
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
	return NewVersionedEncoderWithTypeEncoder(DefaultVersion, w, typeEnc)
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
		buf:             newEncbuf(),
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
		buf:             newEncbuf(),
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
//     *RawBytes     - Transcode v into the appropriate output format.
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
	if rb, ok := v.(*RawBytes); ok {
		// This case exists to skip finishEncode when there is a top-level RawBytes and
		// cover a common special case.
		// TODO(bprosnitz) This doesn't handle cases with more indirection of RawBytes.
		return e.enc.encodeRaw(rb)
	}
	vdlType := extractType(v)
	tid, err := e.enc.typeEnc.encode(vdlType)
	if err != nil {
		return err
	}
	if err := e.enc.startEncode(containsAny(vdlType), containsTypeObject(vdlType), hasChunkLen(vdlType), false, int64(tid)); err != nil {
		return err
	}
	if err := vdl.FromReflect(&e.enc, reflect.ValueOf(v)); err != nil {
		return err
	}
	return e.enc.finishEncode()
}

// TODO(bprosnitz) Remove -- this is copied from vdl
func extractType(v interface{}) *vdl.Type {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr && !rv.IsNil() {
		if rv.Type().ConvertibleTo(rtPtrToValue) {
			vv := rv.Convert(rtPtrToValue).Interface().(*vdl.Value)
			if vv.Kind() == vdl.Any && vv.Elem().IsValid() {
				vv = vv.Elem()
			}
			return vv.Type()
		}
		if rv.Type().ConvertibleTo(rtPtrToRawBytes) {
			return rv.Convert(rtPtrToRawBytes).Interface().(*RawBytes).Type
		}
		rv = rv.Elem()
	}
	return vdl.TypeOf(v)
}

func (e *encoder) encodeRaw(raw *RawBytes) error {
	if e.version == Version80 {
		return verror.New(errUnsupportedInVOMVersion, nil, "RawBytes", e.version)
	}
	if !e.sentVersionByte {
		if _, err := e.writer.Write([]byte{byte(e.version)}); err != nil {
			return err
		}
	}
	var fromNil bool
	if raw == nil {
		raw = RawBytesOf(vdl.ZeroValue(vdl.AnyType))
		// TODO(bprosnitz) fromNil should be set based on whether the inner raw bytes value is nil
		fromNil = true
	}
	if err := e.prepareTypeHelper(raw.Type, fromNil); err != nil {
		return err
	}
	tid, err := e.typeEnc.encode(raw.Type)
	if err != nil {
		return err
	}
	if err := e.startMessage(containsAny(raw.Type), containsTypeObject(raw.Type), hasChunkLen(raw.Type), false, int64(tid)); err != nil {
		return err
	}
	// The RawBytes object may have RefTypes and AnyLengths even if
	if containsTypeObject(raw.Type) || containsAny(raw.Type) {
		for _, refType := range raw.RefTypes {
			mid, err := e.typeEnc.encode(refType)
			if err != nil {
				return err
			}
			e.tids.tids = append(e.tids.tids, mid)
		}
	}
	if containsAny(raw.Type) {
		e.anyLens.lens = raw.AnyLengths
	}
	e.buf.Write(raw.Data)
	if err := e.finishMessage(); err != nil {
		return err
	}
	if err := e.popType(); err != nil {
		return err
	}

	return nil
}

func (e *encoder) encodeWireType(tid TypeId, wt wireType, typeIncomplete bool) error {
	if err := e.startEncode(false, false, true, typeIncomplete, int64(-tid)); err != nil {
		return err
	}
	if err := vdl.FromReflect(e, reflect.ValueOf(wt)); err != nil {
		return err
	}
	// We encode the negative id for type definitions.
	return e.finishEncode()
}

func (e *encoder) startEncode(hasAny, hasTypeObject, hasLen, typeIncomplete bool, mid int64) error {
	if err := e.startMessage(hasAny, hasTypeObject, hasLen, typeIncomplete, int64(mid)); err != nil {
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
	return e.finishMessage()
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
func (e *encoder) prepareTypeHelper(tt *vdl.Type, preventAnyWrap bool) error {
	var tid TypeId
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
	case !preventAnyWrap && e.topType().Kind() == vdl.Any:
		if e.version == Version80 {
			binaryEncodeUint(e.buf, uint64(tid))
		} else {
			binaryEncodeUint(e.buf, e.tids.ReferenceTypeID(tid))
			anyStartRef := e.anyLens.StartAny(e.buf.Len())
			e.typeStack[len(e.typeStack)-1].anyStartRef = &anyStartRef
			binaryEncodeUint(e.buf, uint64(anyStartRef.index))
		}
	}
	return nil
}

func (e *encoder) pushType(tt *vdl.Type) {
	e.typeStack = append(e.typeStack, typeStackEntry{tt, -1, nil})
}

func (e *encoder) pushFieldType(tt *vdl.Type, index int) {
	e.typeStack = append(e.typeStack, typeStackEntry{tt, index, nil})
}

func (e *encoder) popType() error {
	if len(e.typeStack) == 0 {
		return verror.New(errEncodeBadTypeStack, nil)
	}
	topEntry := e.typeStack[len(e.typeStack)-1]
	if topEntry.anyStartRef != nil {
		e.anyLens.FinishAny(*topEntry.anyStartRef, e.buf.Len())
	}
	e.typeStack = e.typeStack[:len(e.typeStack)-1]
	return nil
}

func (e *encoder) topType() *vdl.Type {
	return e.typeStack[len(e.typeStack)-1].tt
}

func (e *encoder) topTypeFieldIndex() int {
	if len(e.typeStack) == 0 {
		return -1
	}
	return e.typeStack[len(e.typeStack)-1].fieldIndex
}

// Implementation of vdl.Target interface.
var boolAllowed = []vdl.Kind{vdl.Bool}

func (e *encoder) FromBool(src bool, tt *vdl.Type) error {
	if err := e.prepareType(tt, boolAllowed...); err != nil {
		return err
	}
	binaryEncodeBool(e.buf, src)
	return nil
}

var uintAllowed = []vdl.Kind{vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64}

func (e *encoder) FromUint(src uint64, tt *vdl.Type) error {
	if err := e.prepareType(tt, uintAllowed...); err != nil {
		return err
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
	binaryEncodeInt(e.buf, src)
	return nil
}

var floatAllowed = []vdl.Kind{vdl.Float32, vdl.Float64}

func (e *encoder) FromFloat(src float64, tt *vdl.Type) error {
	if err := e.prepareType(tt, floatAllowed...); err != nil {
		return err
	}
	binaryEncodeFloat(e.buf, src)
	return nil
}

func (e *encoder) FromBytes(src []byte, tt *vdl.Type) error {
	if !tt.IsBytes() {
		return verror.New(errEncoderWantBytesType, nil, tt)
	}
	if err := e.prepareTypeHelper(tt, false); err != nil {
		return err
	}
	switch tt.Kind() {
	case vdl.List:
		binaryEncodeUint(e.buf, uint64(len(src)))
	case vdl.Array:
		// Special-case the array length to always encode 0.  We
		// already have the length in the type, so technically
		// we don't need to send any length for arrays.  But if we
		// don't encode a length, we'll be encoding the first array
		// elem directly, so we won't be able to encode any flags.
		// E.g. we won't be able to encode ?[3]byte.  So we encode
		// a dummy 0 length.
		binaryEncodeUint(e.buf, 0)
	}
	e.buf.Write(src)
	return nil
}

var stringAllowed = []vdl.Kind{vdl.String}

func (e *encoder) FromString(src string, tt *vdl.Type) error {
	if err := e.prepareType(tt, stringAllowed...); err != nil {
		return err
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
	switch e.version {
	case Version80:
		binaryEncodeUint(e.buf, uint64(tid))
	default:
		binaryEncodeUint(e.buf, e.tids.ReferenceTypeID(tid))
	}
	return nil
}

func (e *encoder) fromZero(tt *vdl.Type) error {
	switch tt.Kind() {
	case vdl.Bool:
		return e.FromBool(false, tt)
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64:
		return e.FromUint(0, tt)
	case vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
		return e.FromInt(0, tt)
	case vdl.Float32, vdl.Float64:
		return e.FromFloat(0, tt)
	case vdl.String:
		return e.FromString("", tt)
	case vdl.Enum:
		return e.FromEnumLabel(tt.EnumLabel(0), tt)
	case vdl.TypeObject:
		return e.FromTypeObject(vdl.AnyType)
	case vdl.Any, vdl.Optional:
		return e.FromNil(tt)
	case vdl.List:
		lt, err := e.StartList(tt, 0)
		if err != nil {
			return nil
		}
		return e.FinishList(lt)
	case vdl.Array:
		lt, err := e.StartList(tt, tt.Len())
		if err != nil {
			return nil
		}
		for i := 0; i < tt.Len(); i++ {
			t, err := lt.StartElem(i)
			if err != nil {
				return err
			}
			if err := t.(*encoder).fromZero(tt.Elem()); err != nil {
				return err
			}
			if err := lt.FinishElem(t); err != nil {
				return err
			}
		}
		return e.FinishList(lt)
	case vdl.Map:
		mt, err := e.StartMap(tt, 0)
		if err != nil {
			return nil
		}
		return e.FinishMap(mt)
	case vdl.Set:
		st, err := e.StartSet(tt, 0)
		if err != nil {
			return nil
		}
		return e.FinishSet(st)
	case vdl.Struct:
		ft, err := e.StartFields(tt)
		if err != nil {
			return err
		}
		return e.FinishFields(ft)
	case vdl.Union:
		ft, err := e.StartFields(tt)
		if err != nil {
			return err
		}
		key, field, err := ft.StartField(tt.Field(0).Name)
		if err != nil {
			return err
		}
		if err := field.(*encoder).fromZero(tt.Field(0).Type); err != nil {
			return err
		}
		if err := ft.FinishField(key, field); err != nil {
			return err
		}
		return e.FinishFields(ft)
	default:
		return fmt.Errorf("unknown kind: %v", tt.Kind())
	}
}

var nilAllowed = []vdl.Kind{vdl.Any, vdl.Optional}

func (e *encoder) FromNil(tt *vdl.Type) error {
	if !tt.CanBeNil() {
		return errTypeMismatch(tt, nilAllowed...)
	}
	// Emit any wrapper iff this is a nil optional within an any.
	preventAnyWrap := len(e.typeStack) == 0 || e.topType() != vdl.AnyType || tt.Kind() != vdl.Optional
	if err := e.prepareTypeHelper(tt, preventAnyWrap); err != nil {
		return err
	}
	e.buf.WriteOneByte(WireCtrlNil)
	return nil
}

var listAllowed = []vdl.Kind{vdl.Array, vdl.List}

func (e *encoder) StartList(tt *vdl.Type, len int) (vdl.ListTarget, error) {
	if err := e.prepareType(tt, listAllowed...); err != nil {
		return nil, err
	}
	switch tt.Kind() {
	case vdl.List:
		binaryEncodeUint(e.buf, uint64(len))
	case vdl.Array:
		binaryEncodeUint(e.buf, uint64(0))
	}
	e.pushType(tt)
	return e, nil
}

var setAllowed = []vdl.Kind{vdl.Set}

func (e *encoder) StartSet(tt *vdl.Type, len int) (vdl.SetTarget, error) {
	if err := e.prepareType(tt, setAllowed...); err != nil {
		return nil, err
	}
	binaryEncodeUint(e.buf, uint64(len))
	e.pushType(tt)
	return e, nil
}

var mapAllowed = []vdl.Kind{vdl.Map}

func (e *encoder) StartMap(tt *vdl.Type, len int) (vdl.MapTarget, error) {
	if err := e.prepareType(tt, mapAllowed...); err != nil {
		return nil, err
	}
	binaryEncodeUint(e.buf, uint64(len))
	e.pushType(tt)
	return e, nil
}

var fieldsAllowed = []vdl.Kind{vdl.Struct, vdl.Union}

func (e *encoder) StartFields(tt *vdl.Type) (vdl.FieldsTarget, error) {
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
		binaryEncodeUint(e.buf, uint64(e.topTypeFieldIndex()))
		return nil, e, nil
	}
	return nil, nil, verror.New(errFieldNotInTopType, nil, name, top)
}

func (e *encoder) FinishField(key, field vdl.Target) error {
	return e.popType()
}

func (e *encoder) ZeroField(name string) error {
	top := e.topType()
	if top.Kind() == vdl.Optional {
		top = top.Elem()
	}
	switch top.Kind() {
	case vdl.Struct:
		return nil
	case vdl.Union:
		fld, index := top.FieldByName(name)
		if index < 0 {
			return vdl.ErrFieldNoExist
		}
		return vdl.FromValue(e, vdl.ZeroValue(fld.Type))
	default:
		return verror.New(errInvalid, nil)
	}
}

func (e *encoder) startMessage(hasAny, hasTypeObject, hasLen, typeIncomplete bool, mid int64) error {
	e.buf.Reset()
	e.buf.Grow(paddingLen)
	e.hasLen = hasLen
	e.hasAny = hasAny
	e.hasTypeObject = hasTypeObject
	e.typeIncomplete = typeIncomplete
	e.mid = mid
	if e.version >= Version81 && (e.hasAny || e.hasTypeObject) {
		e.tids = newTypeIDList()
	} else {
		e.tids = nil
	}
	if e.version >= Version81 && e.hasAny {
		e.anyLens = newAnyLenList()
	} else {
		e.anyLens = nil
	}
	return nil
}

func (e *encoder) finishMessage() error {
	if e.version >= Version81 {
		if e.typeIncomplete {
			if _, err := e.writer.Write([]byte{WireCtrlTypeIncomplete}); err != nil {
				return err
			}
		}
		if e.hasAny || e.hasTypeObject {
			ids := e.tids.NewIDs()
			var anyLens []int
			if e.hasAny {
				anyLens = e.anyLens.NewAnyLens()
			}
			headerBuf := newEncbuf()
			binaryEncodeInt(headerBuf, e.mid)
			binaryEncodeUint(headerBuf, uint64(len(ids)))
			for _, id := range ids {
				binaryEncodeUint(headerBuf, uint64(id))
			}
			if e.hasAny {
				binaryEncodeUint(headerBuf, uint64(len(anyLens)))
				for _, anyLen := range anyLens {
					binaryEncodeUint(headerBuf, uint64(anyLen))
				}
			}
			msg := e.buf.Bytes()
			if e.hasLen {
				binaryEncodeUint(headerBuf, uint64(len(msg)-paddingLen))
			}
			if _, err := e.writer.Write(headerBuf.Bytes()); err != nil {
				return err
			}
			_, err := e.writer.Write(msg[paddingLen:])
			return err
		}
	}
	msg := e.buf.Bytes()
	header := msg[:paddingLen]
	if e.hasLen {
		start := binaryEncodeUintEnd(header, uint64(len(msg)-paddingLen))
		header = header[:start]
	}
	start := binaryEncodeIntEnd(header, e.mid)
	_, err := e.writer.Write(msg[start:])
	return err
}

func newTypeIDList() *typeIDList {
	return &typeIDList{
		tids: make([]TypeId, 0, typeIDListInitialSize),
	}
}

type typeIDList struct {
	tids      []TypeId
	totalSent int
}

func (l *typeIDList) ReferenceTypeID(tid TypeId) uint64 {
	for index, existingTid := range l.tids {
		if existingTid == tid {
			return uint64(index)
		}
	}

	l.tids = append(l.tids, tid)
	return uint64(len(l.tids) - 1)
}

func (l *typeIDList) Reset() error {
	if l.totalSent != len(l.tids) {
		return verror.New(errUnusedTypeIds, nil)
	}
	l.tids = l.tids[:0]
	l.totalSent = 0
	return nil
}

func (l *typeIDList) NewIDs() []TypeId {
	var newIDs []TypeId
	if l.totalSent < len(l.tids) {
		newIDs = l.tids[l.totalSent:]
	}
	l.totalSent = len(l.tids)
	return newIDs
}

func newAnyLenList() *anyLenList {
	return &anyLenList{
		lens: make([]int, 0, anyLenListInitialSize),
	}
}

type anyStartRef struct {
	index  int // index into the anyLen list
	marker int // position marker for the start of the any
}

type anyLenList struct {
	lens      []int
	totalSent int
}

func (l *anyLenList) StartAny(startMarker int) anyStartRef {
	l.lens = append(l.lens, 0)
	index := len(l.lens) - 1
	return anyStartRef{
		index:  index,
		marker: startMarker + lenUint(uint64(index)),
	}
}

func (l *anyLenList) FinishAny(start anyStartRef, endMarker int) {
	l.lens[start.index] = endMarker - start.marker
}

func (l *anyLenList) Reset() error {
	if l.totalSent != len(l.lens) {
		return verror.New(errUnusedAnys, nil)
	}
	l.lens = l.lens[:0]
	l.totalSent = 0
	return nil
}

func (l *anyLenList) NewAnyLens() []int {
	var newAnyLens []int
	if l.totalSent < len(l.lens) {
		newAnyLens = l.lens[l.totalSent:]
	}
	l.totalSent = len(l.lens)
	return newAnyLens
}
