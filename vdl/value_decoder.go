// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"errors"
	"fmt"
	"math"
)

var (
	errEmptyDecoderStack = errors.New("vdl: empty decoder stack")
)

// Decoder returns a decoder that traverses vv.
func (vv *Value) Decoder() Decoder {
	return &valueDecoder{initial: vv}
}

// ValueDecoder is an implementation of Decoder for vdl Value.
type valueDecoder struct {
	initial    *Value
	ignoreNext bool
	stack      []vdStackEntry
}

type vdStackEntry struct {
	Value      *Value   // the vdl Value
	Index      int      // next index or field (index into Keys for map/set)
	NumStarted int      // hold state for multiple StartValue() calls
	IsAny      bool     // true iff this value is within an any value
	IsOptional bool     // true iff this value is within an optional value
	Keys       []*Value // keys for set/map
}

func (d *valueDecoder) StartValue() error {
	if d.ignoreNext {
		d.ignoreNext = false
		return nil
	}
	var vv *Value
	if top := d.top(); top == nil {
		vv = d.initial
	} else {
		switch top.Value.Kind() {
		case Array, List:
			vv = top.Value.Index(top.Index)
		case Set:
			vv = top.Keys[top.Index]
		case Map:
			switch top.NumStarted % 2 {
			case 0:
				vv = top.Keys[top.Index]
			case 1:
				vv = top.Value.MapIndex(top.Keys[top.Index])
			}
		case Struct:
			vv = top.Value.StructField(top.Index)
		case Union:
			_, vv = top.Value.UnionField()
		default:
			return fmt.Errorf("unknown composite kind: %v", top.Value.Kind())
		}
		top.NumStarted++
	}
	var isAny, isOptional bool
	if vv.Kind() == Any {
		isAny = true
		if !vv.IsNil() {
			vv = vv.Elem()
		}
	}
	if vv.Kind() == Optional {
		isOptional = true
		if !vv.IsNil() {
			vv = vv.Elem()
		}
	}
	entry := vdStackEntry{
		Value:      vv,
		IsAny:      isAny,
		IsOptional: isOptional,
		Index:      -1,
	}
	if vv.Kind() == Map || vv.Kind() == Set {
		entry.Keys = vv.Keys()
	}
	d.stack = append(d.stack, entry)
	return nil
}

func (d *valueDecoder) StackDepth() int {
	return len(d.stack)
}

func (d *valueDecoder) IgnoreNextStartValue() {
	d.ignoreNext = true
}

func (d *valueDecoder) FinishValue() error {
	if len(d.stack) == 0 {
		return errEmptyDecoderStack
	}
	d.stack = d.stack[:len(d.stack)-1]
	d.ignoreNext = false
	return nil
}

func (d *valueDecoder) SkipValue() error {
	d.ignoreNext = false
	return nil
}

func (d *valueDecoder) NextEntry() (bool, error) {
	top := d.top()
	if top == nil {
		return false, errEmptyDecoderStack
	}
	top.Index++
	switch top.Value.Kind() {
	case Array, List, Set, Map:
		switch {
		case top.Index == top.Value.Len():
			return true, nil
		case top.Index > top.Value.Len():
			return false, fmt.Errorf("vdl: NextEntry called after done, stack: %+v", d.stack)
		}
	}
	return false, nil
}

func (d *valueDecoder) NextField() (string, error) {
	top := d.top()
	if top == nil {
		return "", errEmptyDecoderStack
	}
	top.Index++
	index, max := top.Index, top.Value.Type().NumField()
	if top.Value.Kind() == Union {
		max = 1
		index, _ = top.Value.UnionField()
	}
	if top.Index == max {
		return "", nil
	} else if top.Index > max {
		return "", fmt.Errorf("vdl: NextField called after done, stack: %+v", d.stack)
	}
	return top.Value.Type().Field(index).Name, nil
}

func (d *valueDecoder) Type() *Type {
	if top := d.top(); top != nil {
		return top.Value.Type()
	}
	return nil
}

func (d *valueDecoder) IsAny() bool {
	if top := d.top(); top != nil {
		return top.IsAny
	}
	return false
}

func (d *valueDecoder) IsOptional() bool {
	if top := d.top(); top != nil {
		return top.IsOptional
	}
	return false
}

func (d *valueDecoder) IsNil() bool {
	if top := d.top(); top != nil {
		return top.Value.IsNil()
	}
	return false
}

func (d *valueDecoder) Index() int {
	if top := d.top(); top != nil {
		return top.Index
	}
	return -1
}

func (d *valueDecoder) LenHint() int {
	if top := d.top(); top != nil {
		switch top.Value.Kind() {
		case List, Map, Set, Array:
			return top.Value.Len()
		}
	}
	return -1
}

func (d *valueDecoder) DecodeBool() (bool, error) {
	top := d.top()
	if top == nil {
		return false, errEmptyDecoderStack
	}
	if top.Value.Kind() != Bool {
		return false, fmt.Errorf("vdl: type mismatch, got %v, want bool", top.Value.Type())
	}
	return top.Value.Bool(), nil
}

func (d *valueDecoder) DecodeUint(bitlen int) (uint64, error) {
	const errFmt = "vdl: %v conversion to uint%d loses precision: %v"
	top, ubitlen := d.top(), uint(bitlen)
	if top == nil {
		return 0, errEmptyDecoderStack
	}
	switch top.Value.Kind() {
	case Byte, Uint16, Uint32, Uint64:
		x := top.Value.Uint()
		if shift := 64 - ubitlen; x != (x<<shift)>>shift {
			return 0, fmt.Errorf(errFmt, top.Value, bitlen, x)
		}
		return x, nil
	case Int8, Int16, Int32, Int64:
		x := top.Value.Int()
		ux := uint64(x)
		if shift := 64 - ubitlen; x < 0 || ux != (ux<<shift)>>shift {
			return 0, fmt.Errorf(errFmt, top.Value, bitlen, x)
		}
		return ux, nil
	case Float32, Float64:
		x := top.Value.Float()
		ux := uint64(x)
		if shift := 64 - ubitlen; x != float64(ux) || ux != (ux<<shift)>>shift {
			return 0, fmt.Errorf(errFmt, top.Value, bitlen, x)
		}
		return ux, nil
	default:
		return 0, fmt.Errorf("vdl: type mismatch, got %v, want uint%d", top.Value.Type(), bitlen)
	}
}

func (d *valueDecoder) DecodeInt(bitlen int) (int64, error) {
	const errFmt = "vdl: %v conversion to int%d loses precision: %v"
	top, ubitlen := d.top(), uint(bitlen)
	if top == nil {
		return 0, errEmptyDecoderStack
	}
	switch top.Value.Kind() {
	case Byte, Uint16, Uint32, Uint64:
		x := top.Value.Uint()
		ix := int64(x)
		// The shift uses 65 since the topmost bit is the sign bit.  I.e. 32 bit
		// numbers should be shifted by 33 rather than 32.
		if shift := 65 - ubitlen; ix < 0 || x != (x<<shift)>>shift {
			return 0, fmt.Errorf(errFmt, top.Value, bitlen, x)
		}
		return ix, nil
	case Int8, Int16, Int32, Int64:
		x := top.Value.Int()
		if shift := 64 - ubitlen; x != (x<<shift)>>shift {
			return 0, fmt.Errorf(errFmt, top.Value, bitlen, x)
		}
		return x, nil
	case Float32, Float64:
		x := top.Value.Float()
		ix := int64(x)
		if shift := 64 - ubitlen; x != float64(ix) || ix != (ix<<shift)>>shift {
			return 0, fmt.Errorf(errFmt, top.Value, bitlen, x)
		}
		return ix, nil
	default:
		return 0, fmt.Errorf("vdl: type mismatch, got %v, want int%d", top.Value.Type(), bitlen)
	}
}

func (d *valueDecoder) DecodeFloat(bitlen int) (float64, error) {
	const errFmt = "vdl: %v conversion to float%d loses precision: %v"
	top := d.top()
	if top == nil {
		return 0, errEmptyDecoderStack
	}
	switch top.Value.Kind() {
	case Byte, Uint16, Uint32, Uint64:
		x := top.Value.Uint()
		var max uint64
		if bitlen > 32 {
			max = float64MaxInt
		} else {
			max = float32MaxInt
		}
		if x > max {
			return 0, fmt.Errorf(errFmt, top.Value, bitlen, x)
		}
		return float64(x), nil
	case Int8, Int16, Int32, Int64:
		x := top.Value.Int()
		var min, max int64
		if bitlen > 32 {
			min, max = float64MinInt, float64MaxInt
		} else {
			min, max = float32MinInt, float32MaxInt
		}
		if x < min || x > max {
			return 0, fmt.Errorf(errFmt, top.Value, bitlen, x)
		}
		return float64(x), nil
	case Float32, Float64:
		x := top.Value.Float()
		if bitlen <= 32 && (x < -math.MaxFloat32 || x > math.MaxFloat32) {
			return 0, fmt.Errorf(errFmt, top.Value, bitlen, x)
		}
		return x, nil
	default:
		return 0, fmt.Errorf("vdl: type mismatch, got %v, want float%d", top.Value.Type(), bitlen)
	}
}

func (d *valueDecoder) DecodeBytes(fixedlen int, v *[]byte) error {
	top := d.top()
	if top == nil {
		return errEmptyDecoderStack
	}
	if !top.Value.Type().IsBytes() {
		return bytesVDLRead(fixedlen, v, d)
	}
	if fixedlen >= 0 && top.Value.Len() != fixedlen {
		return fmt.Errorf("vdl: %v got %v bytes, want fixed len %v", top.Value.Type(), top.Value.Len(), fixedlen)
	}
	if cap(*v) < top.Value.Len() {
		*v = make([]byte, top.Value.Len())
	} else {
		*v = (*v)[:top.Value.Len()]
	}
	copy(*v, top.Value.Bytes())
	return nil
}

func (d *valueDecoder) DecodeString() (string, error) {
	top := d.top()
	if top == nil {
		return "", errEmptyDecoderStack
	}
	switch top.Value.Kind() {
	case String:
		return top.Value.RawString(), nil
	case Enum:
		return top.Value.EnumLabel(), nil
	default:
		return "", fmt.Errorf("vdl: type mismatch, got %v, want string", top.Value.Type())
	}
}

func (d *valueDecoder) DecodeTypeObject() (*Type, error) {
	top := d.top()
	if top == nil {
		return nil, errEmptyDecoderStack
	}
	if top.Value.Type() != TypeObjectType {
		return nil, fmt.Errorf("vdl: type mismatch, got %v, want typeobject", top.Value.Type())
	}
	return top.Value.TypeObject(), nil
}

func (d *valueDecoder) top() *vdStackEntry {
	if len(d.stack) > 0 {
		return &d.stack[len(d.stack)-1]
	}
	return nil
}
