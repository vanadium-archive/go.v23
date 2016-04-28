// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build reflectdecoder

// This code is expected to be deleted. It is here temporarily until
// we know whether it will actually be needed.

package vdl

import (
	"fmt"
	"reflect"
)

func NewReflectDecoder(rv reflect.Value) *reflectDecoder {
	return &reflectDecoder{initialRv: rv}
}

type reflectDecoder struct {
	initialRv         reflect.Value
	ignoreNext        bool
	stack             []rdStackEntry
	InnerDecoderDepth int
	InnerDecoder      Decoder
}

type rdStackEntry struct {
	Type             *Type
	ReflectValue     reflect.Value
	OrigReflectValue reflect.Value // if the type is native / error, hold the original rv to fill in FinishValue()
	Index            int
	NumStarted       int
	IsAny            bool
	IsOptional       bool
	IsNil            bool
	Keys             []reflect.Value
}

func (d *reflectDecoder) StartValue() error {
	if d.ignoreNext {
		d.ignoreNext = false
		return nil
	}
	if d.InnerDecoderDepth > 0 {
		d.InnerDecoderDepth++
		return d.InnerDecoder.StartValue()
	}
	var rv reflect.Value
	var tt *Type
	if top := d.top(); top == nil {
		rv = d.initialRv
		var err error
		if rv.IsValid() {
			tt, err = TypeFromReflect(rv.Type())
			if err != nil {
				return err
			}
		}
	} else {
		switch top.Type.Kind() {
		case Array, List:
			rv = top.ReflectValue.Index(top.Index)
			tt = top.Type.Elem()
		case Map:
			switch top.NumStarted % 2 {
			case 0:
				rv = top.Keys[top.Index]
				tt = top.Type.Key()
			case 1:
				rv = top.ReflectValue.MapIndex(top.Keys[top.Index])
				tt = top.Type.Elem()
			}
			top.NumStarted++
		case Set:
			rv = top.Keys[top.Index]
			tt = top.Type.Key()
		case Struct:
			fld := top.Type.Field(top.Index)
			rv = top.ReflectValue.FieldByName(fld.Name)
			tt = fld.Type
		case Union:
			unionRv := top.ReflectValue
			if top.ReflectValue.Kind() == reflect.Interface && !top.ReflectValue.IsNil() {
				unionRv = unionRv.Elem()
			}
			ri, _, err := deriveReflectInfo(unionRv.Type())
			if err != nil {
				return err
			}
			if unionRv.Type() == ri.Type {
				rv = reflect.Zero(ri.UnionFields[0].Type).FieldByName("Value")
				tt = tt.Field(0).Type
			} else {
				rv = unionRv.FieldByName("Value")
				unionIndex := -1
				for index, field := range ri.UnionFields {
					if field.RepType == unionRv.Type() {
						unionIndex = index
					}
				}
				if unionIndex < 0 {
					return fmt.Errorf("vdl: union field not found")
				}
				tt = top.Type.Field(unionIndex).Type
			}
		default:
			return fmt.Errorf("unknown composite kind: %v", top.Type.Kind())
		}
	}

	// ptrRv will be set to a single level of ptr around the inner value.
	// rv will be dereferenced entirely.
	var ptrRv reflect.Value
	for rv.IsValid() && (rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface) {
		if rv.Kind() == reflect.Ptr {
			ptrRv = rv
		} else {
			ptrRv = reflect.Value{}
		}
		rv = rv.Elem()
	}

	if ptrRv.IsValid() {
		if dec, ok := ptrRv.Interface().(HasDecoder); ok {
			d.InnerDecoderDepth++
			d.InnerDecoder = dec.Decoder()
			return d.InnerDecoder.StartValue()
		}

		if tt == ErrorType {
			ni, err := nativeInfoForError()
			if err != nil {
				return err
			}

			ptrRv = reflect.New(reflect.TypeOf(WireError{}))
			if err := ni.FromNative(ptrRv, rv); err != nil {
				return err
			}
			rv = ptrRv.Elem()
		}
	}

	if !rv.IsValid() || (tt.Kind() == Any && rv.IsNil()) {
		d.stack = append(d.stack, rdStackEntry{
			ReflectValue: rv,
			Type:         tt,
			IsAny:        true,
			IsNil:        true,
			Index:        -1,
		})
		return nil
	}

	if dec, ok := rv.Interface().(HasDecoder); ok {
		d.InnerDecoderDepth++
		d.InnerDecoder = dec.Decoder()
		return d.InnerDecoder.StartValue()
	}

	if ni := nativeInfoFromNative(rv.Type()); ni != nil {
		ptrRv = reflect.New(ni.WireType)
		if err := ni.FromNative(ptrRv, rv); err != nil {
			return err
		}
		rv = ptrRv.Elem()
	}

	var isAny, isOptional, isNil bool
	if tt.Kind() == Any {
		isAny = true
		var err error
		tt, err = TypeFromReflect(rv.Type())
		if err != nil {
			return err
		}
	}
	if tt.Kind() == Optional {
		switch {
		case !ptrRv.IsValid() || ptrRv.IsNil():
			isNil = true
		default:
			isOptional = true
			tt = tt.Elem()
		}
	}
	if tt.Kind() == TypeObject {
		rv = ptrRv
	}
	entry := rdStackEntry{
		ReflectValue: rv,
		Type:         tt,
		IsAny:        isAny,
		IsOptional:   isOptional,
		IsNil:        isNil,
		Index:        -1,
	}
	if tt.Kind() == Map || tt.Kind() == Set {
		entry.Keys = rv.MapKeys()
	}
	d.stack = append(d.stack, entry)
	return nil
}

func (d *reflectDecoder) StackDepth() int {
	return len(d.stack) + d.InnerDecoderDepth
}

func (d *reflectDecoder) IgnoreNextStartValue() {
	d.ignoreNext = true
}

func (d *reflectDecoder) FinishValue() error {
	if d.InnerDecoderDepth > 0 {
		d.InnerDecoderDepth--
		return d.InnerDecoder.FinishValue()
	}
	top := d.top()
	if top == nil {
		return errEmptyDecoderStack
	}
	d.stack = d.stack[:len(d.stack)-1]
	return nil
}

func (d *reflectDecoder) SkipValue() error {
	if d.InnerDecoderDepth > 0 {
		return d.InnerDecoder.SkipValue()
	}
	return nil
}

func (d *reflectDecoder) NextEntry() (bool, error) {
	if d.InnerDecoderDepth > 0 {
		return d.InnerDecoder.NextEntry()
	}
	top := d.top()
	if top == nil {
		return false, errEmptyDecoderStack
	}
	top.Index++
	switch top.Type.Kind() {
	case Array, List, Set, Map:
		switch {
		case top.Index == top.ReflectValue.Len():
			return true, nil
		case top.Index > top.ReflectValue.Len():
			return false, fmt.Errorf("vdl: NextEntry called after done, stack: %+v", d.stack)
		}
	}
	top.NumStarted = 0
	return false, nil
}

func (d *reflectDecoder) NextField() (string, error) {
	if d.InnerDecoderDepth > 0 {
		return d.InnerDecoder.NextField()
	}
	top := d.top()
	if top == nil {
		return "", errEmptyDecoderStack
	}
	max := top.Type.NumField()
	if top.Type.Kind() == Union {
		max = 1
	}
	top.Index++
	if top.Index == max {
		return "", nil
	} else if top.Index > max {
		return "", fmt.Errorf("vdl: NextField called after done, stack: %+v", d.stack)
	}

	if top.Type.Kind() == Union {
		unionRv := top.ReflectValue
		if top.ReflectValue.Kind() == reflect.Interface && !top.ReflectValue.IsNil() {
			unionRv = unionRv.Elem()
		}
		ri, _, err := deriveReflectInfo(unionRv.Type())
		if err != nil {
			return "", err
		}
		if unionRv.Type() == ri.Type {
			// Uninitialized union.
			return ri.UnionFields[0].Name, nil
		}
		for _, field := range ri.UnionFields {
			if field.RepType == unionRv.Type() {
				return field.Name, nil
			}
		}
		return "", fmt.Errorf("invalid union field type")
	}
	return top.Type.Field(top.Index).Name, nil
}

func (d *reflectDecoder) Type() *Type {
	if d.InnerDecoderDepth > 0 {
		return d.InnerDecoder.Type()
	}
	if top := d.top(); top != nil {
		return top.Type
	}
	return nil
}

func (d *reflectDecoder) IsAny() bool {
	if d.InnerDecoderDepth > 0 {
		return d.InnerDecoder.IsAny()
	}
	if top := d.top(); top != nil {
		return top.IsAny
	}
	return false
}

func (d *reflectDecoder) IsOptional() bool {
	if d.InnerDecoderDepth > 0 {
		return d.InnerDecoder.IsOptional()
	}
	if top := d.top(); top != nil {
		return top.IsOptional
	}
	return false
}

func (d *reflectDecoder) IsNil() bool {
	if d.InnerDecoderDepth > 0 {
		return d.InnerDecoder.IsNil()
	}
	if top := d.top(); top != nil {
		return top.IsNil
	}
	return false
}

func (d *reflectDecoder) Index() int {
	if d.InnerDecoderDepth > 0 {
		return d.InnerDecoder.Index()
	}
	if top := d.top(); top != nil {
		return top.Index
	}
	return -1
}

func (d *reflectDecoder) LenHint() int {
	if d.InnerDecoderDepth > 0 {
		return d.InnerDecoder.LenHint()
	}
	top := d.top()
	switch top.Type.Kind() {
	case List, Map, Set:
		return top.ReflectValue.Len()
	case Array:
		return top.Type.Len()
	}
	return -1
}

func (d *reflectDecoder) DecodeBool() (bool, error) {
	if d.InnerDecoderDepth > 0 {
		return d.InnerDecoder.DecodeBool()
	}
	top := d.top()
	if top == nil {
		return false, errEmptyDecoderStack
	}
	if top.Type.Kind() != Bool {
		return false, fmt.Errorf("vdl: type mismatch, got %v, want bool", top.Type)
	}
	return top.ReflectValue.Bool(), nil
}

func (d *reflectDecoder) DecodeUint(bitlen uint) (uint64, error) {
	if d.InnerDecoderDepth > 0 {
		return d.InnerDecoder.DecodeUint(bitlen)
	}
	const errFmt = "vom: %#v conversion to uint%d loses precision: %v"
	top := d.top()
	switch top.Type.Kind() {
	case Byte, Uint16, Uint32, Uint64:
		x := top.ReflectValue.Uint()
		if shift := 64 - bitlen; x != (x<<shift)>>shift {
			return 0, fmt.Errorf(errFmt, top.ReflectValue, bitlen, x)
		}
		return x, nil
	case Int8, Int16, Int32, Int64:
		x := top.ReflectValue.Int()
		ux := uint64(x)
		if shift := 64 - bitlen; x < 0 || ux != (ux<<shift)>>shift {
			return 0, fmt.Errorf(errFmt, top.ReflectValue, bitlen, x)
		}
		return ux, nil
	case Float32, Float64:
		x := top.ReflectValue.Float()
		ux := uint64(x)
		if shift := 64 - bitlen; x != float64(ux) || ux != (ux<<shift)>>shift {
			return 0, fmt.Errorf(errFmt, top.ReflectValue, bitlen, x)
		}
		return ux, nil
	default:
		return 0, fmt.Errorf("invalid kind for DecodeUint(): %v", top.Type.Kind())
	}
}

func (d *reflectDecoder) DecodeInt(bitlen uint) (int64, error) {
	if d.InnerDecoderDepth > 0 {
		return d.InnerDecoder.DecodeInt(bitlen)
	}
	const errFmt = "vom: %#v conversion to int%d loses precision: %v"
	top := d.top()
	switch top.Type.Kind() {
	case Byte, Uint16, Uint32, Uint64:
		x := top.ReflectValue.Uint()
		ix := int64(x)
		if shift := 64 - bitlen; ix < 0 || x != (x<<shift)>>shift {
			return 0, fmt.Errorf(errFmt, top.ReflectValue, bitlen, x)
		}
		return ix, nil
	case Int8, Int16, Int32, Int64:
		x := top.ReflectValue.Int()
		if shift := 64 - bitlen; x != (x<<shift)>>shift {
			return 0, fmt.Errorf(errFmt, top.ReflectValue, bitlen, x)
		}
		return x, nil
	case Float32, Float64:
		x := top.ReflectValue.Float()
		ix := int64(x)
		if shift := 64 - bitlen; x != float64(ix) || ix != (ix<<shift)>>shift {
			return 0, fmt.Errorf(errFmt, top.ReflectValue, bitlen, x)
		}
		return ix, nil
	default:
		return 0, fmt.Errorf("invalid kind for DecodeInt(): %v", top.Type.Kind())
	}
}

func (d *reflectDecoder) DecodeFloat(bitlen uint) (float64, error) {
	if d.InnerDecoderDepth > 0 {
		return d.InnerDecoder.DecodeFloat(bitlen)
	}
	const errFmt = "vom: %#v conversion to float%d loses precision: %v"
	top := d.top()
	switch top.Type.Kind() {
	case Byte, Uint16, Uint32, Uint64:
		x := top.ReflectValue.Uint()
		var max uint64
		if bitlen > 32 {
			max = float64MaxInt
		} else {
			max = float32MaxInt
		}
		if x > max {
			return 0, fmt.Errorf(errFmt, top.ReflectValue, bitlen, x)
		}
		return float64(x), nil
	case Int8, Int16, Int32, Int64:
		x := top.ReflectValue.Int()
		var min, max int64
		if bitlen > 32 {
			min, max = float64MinInt, float64MaxInt
		} else {
			min, max = float32MinInt, float32MaxInt
		}
		if x < min || x > max {
			return 0, fmt.Errorf(errFmt, top.ReflectValue, bitlen, x)
		}
		return float64(x), nil
	case Float32, Float64:
		return top.ReflectValue.Float(), nil
	default:
		return 0, fmt.Errorf("invalid kind for DecodeFloat(): %v", top.Type.Kind())
	}
}

func (d *reflectDecoder) DecodeBytes(fixedlen int, v *[]byte) error {
	if d.InnerDecoderDepth > 0 {
		return d.InnerDecoder.DecodeBytes(fixedlen, v)
	}
	if (d.StackDepth() == 1 || d.IsAny()) && !Compatible(TypeOf(*v), d.Type()) {
		return fmt.Errorf("incompatible bytes %T, from %v", *v, d.Type())
	}
	top := d.top()
	if top == nil {
		return errEmptyDecoderStack
	}
	if fixedlen >= 0 && top.ReflectValue.Len() != fixedlen {
		return fmt.Errorf("vdl: %v got %v bytes, want fixed len %v", top.Type, top.ReflectValue.Len(), fixedlen)
	}
	if !top.Type.IsBytes() {
		return bytesVDLRead(fixedlen, v, d)
	}
	if cap(*v) < top.ReflectValue.Len() {
		*v = make([]byte, top.ReflectValue.Len())
	} else {
		*v = (*v)[:top.ReflectValue.Len()]
	}
	reflect.Copy(reflect.ValueOf(*v), top.ReflectValue)
	return nil
}

func (d *reflectDecoder) DecodeString() (string, error) {
	if d.InnerDecoderDepth > 0 {
		return d.InnerDecoder.DecodeString()
	}
	top := d.top()
	if top == nil {
		return "", errEmptyDecoderStack
	}
	switch top.Type.Kind() {
	case String:
		return top.ReflectValue.String(), nil
	case Enum:
		if top.ReflectValue.Int() < 0 || int(top.ReflectValue.Int()) >= top.Type.NumEnumLabel() {
			return "", fmt.Errorf("vdl: %v enum index %d out of range", top.Type, top.ReflectValue.Int())
		}
		return top.Type.EnumLabel(int(top.ReflectValue.Int())), nil
	default:
		return "", fmt.Errorf("vdl: type mismatch, got %v, want string", top.Type)
	}
}

func (d *reflectDecoder) DecodeTypeObject() (*Type, error) {
	if d.InnerDecoderDepth > 0 {
		return d.InnerDecoder.DecodeTypeObject()
	}
	top := d.top()
	if top == nil {
		return nil, errEmptyDecoderStack
	}
	if top.Type != TypeObjectType {
		return nil, fmt.Errorf("vdl: type mismatch, got %v, want typeobject", top.Type)
	}
	return top.ReflectValue.Interface().(*Type), nil
}

func (d *reflectDecoder) top() *rdStackEntry {
	if len(d.stack) > 0 {
		return &d.stack[len(d.stack)-1]
	}
	return nil
}
