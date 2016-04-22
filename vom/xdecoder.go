// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"errors"
	"fmt"
	"io"

	"v.io/v23/vdl"
	"v.io/v23/verror"
)

const (
	// IEEE 754 represents float64 using 52 bits to represent the mantissa, with
	// an extra implied leading bit.  That gives us 53 bits to store integers
	// without overflow - i.e. [0, (2^53)-1].  And since 2^53 is a small power of
	// two, it can also be stored without loss via mantissa=1 exponent=53.  Thus
	// we have our max and min values.  Ditto for float32, which uses 23 bits with
	// an extra implied leading bit.
	float64MaxInt = (1 << 53)
	float64MinInt = -(1 << 53)
	float32MaxInt = (1 << 24)
	float32MinInt = -(1 << 24)
)

var (
	errEmptyDecoderStack          = errors.New("vom: empty decoder stack")
	errReadRawBytesAlreadyStarted = errors.New("vom: read into vom.RawBytes after StartValue called")
	errReadRawBytesFromNonAny     = errors.New("vom: read into vom.RawBytes only supported on any values")
)

type XDecoder struct {
	dec xDecoder
}

type xDecoder struct {
	old                  *Decoder
	stack                []decoderStackEntry
	ignoreNextStartValue bool
}

type decoderStackEntry struct {
	Type       *vdl.Type
	Index      int
	LenHint    int
	NumStarted int
	IsAny      bool
	IsOptional bool
}

func NewXDecoder(r io.Reader) *XDecoder {
	return &XDecoder{xDecoder{
		old: NewDecoder(r),
	}}
}

func NewXDecoderWithTypeDecoder(r io.Reader, typeDec *TypeDecoder) *XDecoder {
	return &XDecoder{xDecoder{
		old: NewDecoderWithTypeDecoder(r, typeDec),
	}}
}

func (d *XDecoder) Decoder() vdl.Decoder {
	return &d.dec
}

func (d *XDecoder) Decode(v interface{}) error {
	return vdl.Read(&d.dec, v)
}

func (d *XDecoder) Ignore() error {
	return d.dec.old.Ignore()
}

func (d *xDecoder) IgnoreNextStartValue() {
	d.ignoreNextStartValue = true
}

func (d *xDecoder) StackDepth() int {
	return len(d.stack)
}

func (d *xDecoder) decodeWireType(wt *wireType) (TypeId, error) {
	// TODO(toddw): Flip useOldDecoder=false to enable XDecoder.
	const useOldDecoder = true
	if useOldDecoder {
		return d.old.decodeWireType(wt)
	}
	// Type messages are just a regularly encoded wireType, which is a union.  To
	// decode we pre-populate the stack with an entry for the wire type, and run
	// the code-generated __VDLRead_wireType method.
	tid, err := d.old.nextMessage()
	if err != nil {
		return 0, err
	}
	d.stack = append(d.stack, decoderStackEntry{
		Type:    wireTypeType,
		Index:   -1,
		LenHint: 1, // wireType is a union
	})
	d.ignoreNextStartValue = true
	if err := __VDLRead_wireType(d, wt); err != nil {
		return 0, err
	}
	return tid, nil
}

// readRawBytes fills in raw with the next value.  It can be called for both
// top-level and internal values.
func (d *xDecoder) readRawBytes(raw *RawBytes) error {
	if d.ignoreNextStartValue {
		// If the user has already called StartValue on the decoder, it's harder to
		// capture all the raw bytes, since the optional flag and length hints have
		// already been decoded.  So we simply disallow this from happening.
		return errReadRawBytesAlreadyStarted
	}
	tt, err := d.dfsNextType()
	if err != nil {
		return err
	}
	// Handle top-level values.  All types of values are supported, since we can
	// simply copy the message bytes.
	if len(d.stack) == 0 {
		anyLen, err := d.old.peekValueByteLen(tt)
		if err != nil {
			return err
		}
		if err := d.old.decodeRaw(tt, anyLen, raw); err != nil {
			return err
		}
		return d.old.endMessage()
	}
	// Handle internal values.  Only any values are supported at the moment, since
	// they come with a header that tells us the exact length to read.
	//
	// TODO(toddw): Handle other types, either by reading and skipping bytes based
	// on the type, or by falling back to a decode / re-encode slowpath.
	if tt.Kind() != vdl.Any {
		return errReadRawBytesFromNonAny
	}
	ttElem, anyLen, err := d.old.readAnyHeader()
	if err != nil {
		return err
	}
	if ttElem == nil {
		// This is a nil any value, which has already been read by readAnyHeader.
		// We simply fill in RawBytes with the single WireCtrlNil byte.
		raw.Version = d.old.buf.version
		raw.Type = vdl.AnyType
		raw.RefTypes = nil
		raw.AnyLengths = nil
		raw.Data = []byte{WireCtrlNil}
		return nil
	}
	return d.old.decodeRaw(ttElem, anyLen, raw)
}

func (d *xDecoder) StartValue() error {
	//defer func() { fmt.Printf("HACK: StartValue  %+v\n", d.stack) }()
	if d.ignoreNextStartValue {
		d.ignoreNextStartValue = false
		return nil
	}
	tt, err := d.dfsNextType()
	if err != nil {
		return err
	}
	return d.setupValue(tt)
}

func (d *xDecoder) setupValue(tt *vdl.Type) error {
	// Handle any, which may be nil.  We "dereference" non-nil any to the inner
	// type.  If that happens to be an optional, it's handled below.
	isAny := false
	if tt.Kind() == vdl.Any {
		isAny = true
		var err error
		switch tt, _, err = d.old.readAnyHeader(); {
		case err != nil:
			return err
		case tt == nil:
			tt = vdl.AnyType // nil any
		}
	}
	// Handle optional, which may be nil.  Similar to any, we "dereference"
	// non-nil optional to the inner type, which is never allowed to be another
	// optional or any type.
	isOptional := false
	if tt.Kind() == vdl.Optional {
		isOptional = true
		// Read the WireCtrlNil code, but if it's not WireCtrlNil we need to keep
		// the buffer as-is, since it's the first byte of the value, which may
		// itself be another control code.
		switch ctrl, err := binaryPeekControl(d.old.buf); {
		case err != nil:
			return err
		case ctrl == WireCtrlNil:
			d.old.buf.Skip(1) // nil optional
		default:
			tt = tt.Elem() // non-nil optional
		}
	}
	// Initialize LenHint for composite types.
	entry := decoderStackEntry{
		Type:       tt,
		Index:      -1,
		LenHint:    -1,
		NumStarted: 0,
		IsAny:      isAny,
		IsOptional: isOptional,
	}
	switch tt.Kind() {
	case vdl.Array, vdl.List, vdl.Set, vdl.Map:
		// TODO(toddw): Handle sentry-terminated collections without a length hint.
		len, err := binaryDecodeLenOrArrayLen(d.old.buf, tt)
		if err != nil {
			return err
		}
		entry.LenHint = len
	case vdl.Union:
		// Union shouldn't have a LenHint, but we abuse it in NextField as a
		// convenience for detecting when fields are done, so we initialize it here.
		// It has to be at least 1, since 0 will cause NextField to think that the
		// union field has already been decoded.
		entry.LenHint = 1
	case vdl.Struct:
		// Struct shouldn't have a LenHint, but we abuse it in NextField as a
		// convenience for detecting when fields are done, so we initialize it here.
		entry.LenHint = tt.NumField()
	}
	// Finally push the entry onto our stack.
	d.stack = append(d.stack, entry)
	return nil
}

func (d *xDecoder) FinishValue() error {
	//defer func() { fmt.Printf("HACK: FinishValue %+v\n", d.stack) }()
	d.ignoreNextStartValue = false
	stackTop := len(d.stack) - 1
	if stackTop == -1 {
		return errEmptyDecoderStack
	}
	d.stack = d.stack[:stackTop]
	if stackTop == 0 {
		return d.old.endMessage()
	}
	return nil
}

func (d *xDecoder) top() *decoderStackEntry {
	if stackTop := len(d.stack) - 1; stackTop >= 0 {
		return &d.stack[stackTop]
	}
	return nil
}

// dfsNextType determines the type of the next value that we will decode, by
// walking the static type in DFS order.  To bootstrap we retrieve the top-level
// type from the VOM value message.
func (d *xDecoder) dfsNextType() (*vdl.Type, error) {
	top := d.top()
	if top == nil {
		// Bootstrap: start decoding a new top-level value.
		if err := d.old.decodeTypeDefs(); err != nil {
			return nil, err
		}
		tid, err := d.old.nextMessage()
		if err != nil {
			return nil, err
		}
		return d.old.typeDec.lookupType(tid)
	}
	// Check invariants now, right before we actually walk to the next type, and
	// before we've incremented NumStarted.
	//
	// TODO(toddw): In theory we could check the invariants in more places
	// (e.g. in NextEntry and NextField after incrementing Index), but that may
	// get expensive.
	if err := d.checkInvariants(top); err != nil {
		return nil, err
	}
	top.NumStarted++
	// Return the next type from our composite types.
	switch top.Type.Kind() {
	case vdl.Array, vdl.List:
		return top.Type.Elem(), nil
	case vdl.Set:
		return top.Type.Key(), nil
	case vdl.Map:
		if top.NumStarted%2 == 1 {
			return top.Type.Key(), nil
		} else {
			return top.Type.Elem(), nil
		}
	case vdl.Union, vdl.Struct:
		return top.Type.Field(top.Index).Type, nil
	}
	return nil, fmt.Errorf("vom: invalid DFS walk, scalar type, stack %+v", d.stack)
}

func (d *xDecoder) checkInvariants(top *decoderStackEntry) error {
	switch top.Type.Kind() {
	case vdl.Array, vdl.List, vdl.Set, vdl.Map, vdl.Union, vdl.Struct:
		if top.Index < 0 || (top.Index >= top.LenHint && top.LenHint >= 0) {
			return fmt.Errorf("vom: invalid DFS walk, bad index, check NextEntry, NextField and FinishValue, stack %+v", d.stack)
		}
	default:
		if top.Index != -1 || top.LenHint != -1 {
			return fmt.Errorf("vom: invalid DFS walk, internal error, stack %+v", d.stack)
		}
	}
	var bad bool
	switch top.Type.Kind() {
	case vdl.Array, vdl.List, vdl.Set:
		bad = top.NumStarted != top.Index
	case vdl.Map:
		bad = top.NumStarted/2 != top.Index
	case vdl.Union:
		bad = top.NumStarted != 0
	case vdl.Struct:
		bad = top.NumStarted >= top.Type.NumField()
	}
	if bad {
		return fmt.Errorf("vom: invalid DFS walk, mismatched NextEntry, NextField and StartValue, stack %+v", d.stack)
	}
	return nil
}

func (d *xDecoder) NextEntry() (bool, error) {
	// Our strategy is to increment top.Index until it reaches top.LenHint.
	// Currently the LenHint is always set, so it's stronger than a hint.
	//
	// TODO(toddw): Handle sentry-terminated collections without a LenHint.
	top := d.top()
	if top == nil {
		return false, errEmptyDecoderStack
	}
	// Increment index and check errors.
	top.Index++
	switch top.Type.Kind() {
	case vdl.Array, vdl.List, vdl.Set, vdl.Map:
		if top.Index > top.LenHint && top.LenHint >= 0 {
			return false, fmt.Errorf("vom: NextEntry called after done, stack: %+v", d.stack)
		}
	default:
		return false, fmt.Errorf("vom: NextEntry called on invalid type, stack: %+v", d.stack)
	}
	return top.Index == top.LenHint, nil
}

func (d *xDecoder) NextField() (string, error) {
	top := d.top()
	if top == nil {
		return "", errEmptyDecoderStack
	}
	// Increment index and check errors.  Note that the actual top.Index is
	// decoded from the buf data stream; we use top.LenHint to help detect when
	// the fields are done, and to detect invalid calls after we're done.
	top.Index++
	switch top.Type.Kind() {
	case vdl.Union, vdl.Struct:
		if top.Index > top.LenHint {
			return "", fmt.Errorf("vom: NextField called after done, stack: %+v", d.stack)
		}
	default:
		return "", fmt.Errorf("vom: NextField called on invalid type, stack: %+v", d.stack)
	}
	var field int
	switch top.Type.Kind() {
	case vdl.Union:
		if top.Index == top.LenHint {
			// We know we're done since we set LenHint=Index+1 the first time around,
			// and we incremented the index above.
			return "", nil
		}
		// Decode the union field index.
		switch index, err := binaryDecodeUint(d.old.buf); {
		case err != nil:
			return "", err
		case index >= uint64(top.Type.NumField()):
			return "", verror.New(errIndexOutOfRange, nil)
		default:
			// Set LenHint=Index+1 so that we'll know we're done next time around.
			field = int(index)
			top.Index = field
			top.LenHint = field + 1
		}
	case vdl.Struct:
		// Decode the struct field index.
		switch index, ctrl, err := binaryDecodeUintWithControl(d.old.buf); {
		case err != nil:
			return "", err
		case ctrl == WireCtrlEnd:
			// Set Index=LenHint to ensure repeated calls will fail.
			top.Index = top.LenHint
			return "", nil
		case ctrl != 0:
			return "", verror.New(errUnexpectedControlByte, nil, ctrl)
		case index >= uint64(top.Type.NumField()):
			return "", verror.New(errIndexOutOfRange, nil)
		default:
			field = int(index)
			top.Index = field
		}
	}
	return top.Type.Field(field).Name, nil
}

func (d *xDecoder) Type() *vdl.Type {
	if top := d.top(); top != nil {
		return top.Type
	}
	return nil
}

func (d *xDecoder) IsAny() bool {
	if top := d.top(); top != nil {
		return top.IsAny
	}
	return false
}

func (d *xDecoder) IsOptional() bool {
	if top := d.top(); top != nil {
		return top.IsOptional
	}
	return false
}

func (d *xDecoder) IsNil() bool {
	if top := d.top(); top != nil {
		// Becuase of the "dereferencing" we do, the only time the type is any or
		// optional is when it's nil.
		return top.Type == vdl.AnyType || top.Type.Kind() == vdl.Optional
	}
	return false
}

func (d *xDecoder) Index() int {
	if top := d.top(); top != nil {
		return top.Index
	}
	return -1
}

func (d *xDecoder) LenHint() int {
	if top := d.top(); top != nil {
		// Note that union and struct shouldn't have a LenHint, but we abuse it in
		// NextField as a convenience for detecting when fields are done, so an
		// "arbitrary" value is returned here.  Users shouldn't be looking at it for
		// union and struct anyways.
		return top.LenHint
	}
	return -1
}

func (d *xDecoder) DecodeBool() (bool, error) {
	tt := d.Type()
	if tt == nil {
		return false, errEmptyDecoderStack
	}
	if tt.Kind() == vdl.Bool {
		return binaryDecodeBool(d.old.buf)
	}
	return false, fmt.Errorf("vom: type mismatch, got %v, want bool", tt)
}

func (d *xDecoder) DecodeUint(bitlen uint) (uint64, error) {
	const errFmt = "vom: %v conversion to uint%d loses precision: %v"
	tt := d.Type()
	if tt == nil {
		return 0, errEmptyDecoderStack
	}
	switch tt.Kind() {
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64:
		x, err := binaryDecodeUint(d.old.buf)
		if err != nil {
			return 0, err
		}
		if shift := 64 - bitlen; x != (x<<shift)>>shift {
			return 0, fmt.Errorf(errFmt, tt, bitlen, x)
		}
		return x, nil
	case vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
		x, err := binaryDecodeInt(d.old.buf)
		if err != nil {
			return 0, err
		}
		ux := uint64(x)
		if shift := 64 - bitlen; x < 0 || ux != (ux<<shift)>>shift {
			return 0, fmt.Errorf(errFmt, tt, bitlen, x)
		}
		return ux, nil
	case vdl.Float32, vdl.Float64:
		x, err := binaryDecodeFloat(d.old.buf)
		if err != nil {
			return 0, err
		}
		ux := uint64(x)
		if shift := 64 - bitlen; x != float64(ux) || ux != (ux<<shift)>>shift {
			return 0, fmt.Errorf(errFmt, tt, bitlen, x)
		}
		return ux, nil
	}
	return 0, fmt.Errorf("vom: type mismatch, got %v, want uint%d", tt, bitlen)
}

func (d *xDecoder) DecodeInt(bitlen uint) (int64, error) {
	const errFmt = "vom: %v conversion to int%d loses precision: %v"
	tt := d.Type()
	if tt == nil {
		return 0, errEmptyDecoderStack
	}
	switch tt.Kind() {
	case vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
		x, err := binaryDecodeInt(d.old.buf)
		if err != nil {
			return 0, err
		}
		if shift := 64 - bitlen; x != (x<<shift)>>shift {
			return 0, fmt.Errorf(errFmt, tt, bitlen, x)
		}
		return x, nil
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64:
		x, err := binaryDecodeUint(d.old.buf)
		if err != nil {
			return 0, err
		}
		ix := int64(x)
		if shift := 64 - bitlen; ix < 0 || x != (x<<shift)>>shift {
			return 0, fmt.Errorf(errFmt, tt, bitlen, x)
		}
		return ix, nil
	case vdl.Float32, vdl.Float64:
		x, err := binaryDecodeFloat(d.old.buf)
		if err != nil {
			return 0, err
		}
		ix := int64(x)
		if shift := 64 - bitlen; x != float64(ix) || ix != (ix<<shift)>>shift {
			return 0, fmt.Errorf(errFmt, tt, bitlen, x)
		}
		return ix, nil
	}
	return 0, fmt.Errorf("vom: type mismatch, got %v, want int%d", tt, bitlen)
}

func (d *xDecoder) DecodeFloat(bitlen uint) (float64, error) {
	const errFmt = "vom: %v conversion to float%d loses precision: %v"
	tt := d.Type()
	if tt == nil {
		return 0, errEmptyDecoderStack
	}
	switch tt.Kind() {
	case vdl.Float32, vdl.Float64:
		return binaryDecodeFloat(d.old.buf)
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64:
		x, err := binaryDecodeUint(d.old.buf)
		if err != nil {
			return 0, err
		}
		var max uint64
		if bitlen > 32 {
			max = float64MaxInt
		} else {
			max = float32MaxInt
		}
		if x > max {
			return 0, fmt.Errorf(errFmt, tt, bitlen, x)
		}
		return float64(x), nil
	case vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
		x, err := binaryDecodeInt(d.old.buf)
		if err != nil {
			return 0, err
		}
		var min, max int64
		if bitlen > 32 {
			min, max = float64MinInt, float64MaxInt
		} else {
			min, max = float32MinInt, float32MaxInt
		}
		if x < min || x > max {
			return 0, fmt.Errorf(errFmt, tt, bitlen, x)
		}
		return float64(x), nil
	}
	return 0, fmt.Errorf("vom: type mismatch, got %v, want float%d", tt, bitlen)
}

func (d *xDecoder) DecodeBytes(fixedlen int, v *[]byte) error {
	top := d.top()
	if top == nil {
		return errEmptyDecoderStack
	}
	tt, len := top.Type, top.LenHint
	if tt.IsBytes() {
		switch {
		case len == -1:
			return fmt.Errorf("vom: %v LenHint is currently required", tt)
		case fixedlen >= 0 && fixedlen != len:
			return fmt.Errorf("vom: %v got %v bytes, want fixed len %v", tt, len, fixedlen)
		case len == 0:
			*v = nil
			return nil
		}
		if cap(*v) >= len {
			*v = (*v)[:len]
		} else {
			*v = make([]byte, len)
		}
		return d.old.buf.ReadIntoBuf(*v)
	}
	// TODO(toddw): Deal with conversions from []number.
	return fmt.Errorf("vom: type mismatch, got %v, want bytes", tt)
}

func (d *xDecoder) DecodeString() (string, error) {
	tt := d.Type()
	if tt == nil {
		return "", errEmptyDecoderStack
	}
	switch tt.Kind() {
	case vdl.String:
		return binaryDecodeString(d.old.buf)
	case vdl.Enum:
		index, err := binaryDecodeUint(d.old.buf)
		switch {
		case err != nil:
			return "", err
		case index >= uint64(tt.NumEnumLabel()):
			return "", fmt.Errorf("vom: %v enum index %d out of range", tt, index)
		}
		return tt.EnumLabel(int(index)), nil
	}
	return "", fmt.Errorf("vom: type mismatch, got %v, want string", tt)
}

func (d *xDecoder) DecodeTypeObject() (*vdl.Type, error) {
	tt := d.Type()
	if tt == nil {
		return nil, errEmptyDecoderStack
	}
	switch tt.Kind() {
	case vdl.TypeObject:
		typeIndex, err := binaryDecodeUint(d.old.buf)
		if err != nil {
			return nil, err
		}
		tid, err := d.old.refTypes.ReferencedTypeId(typeIndex)
		if err != nil {
			return nil, err
		}
		return d.old.typeDec.lookupType(tid)
	}
	return nil, fmt.Errorf("vom: type mismatch, got %v, want typeobject", tt)
}

func (d *xDecoder) SkipValue() error {
	if err := d.StartValue(); err != nil {
		return err
	}
	// Nil values have already been read in StartValue, so we only need to
	// explicitly ignore non-nil values.
	if !d.IsNil() {
		if err := d.old.ignoreValue(d.Type()); err != nil {
			return err
		}
	}
	return d.FinishValue()
}
