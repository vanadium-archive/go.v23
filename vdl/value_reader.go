// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"fmt"
)

// VDLRead reads from a decoder into this vdl Value.
func (vv *Value) VDLRead(dec Decoder) error {
	if vv == nil || vv.t == nil {
		return fmt.Errorf("cannot decode into nil vdl value")
	}
	if err := dec.StartValue(); err != nil {
		return err
	}
	if dec.IsNil() {
		return vv.readHandleNil(dec)
	}
	fillvvAny := vv
	if vv.Kind() == Any {
		innerType := dec.Type()
		if dec.IsOptional() {
			innerType = OptionalType(innerType)
		}
		fillvvAny = ZeroValue(innerType)
	}
	fillvv := fillvvAny
	if fillvvAny.Kind() == Optional {
		fillvv = ZeroValue(fillvvAny.Type().Elem())
	}

	if err := fillvv.readFillValue(dec); err != nil {
		return err
	}

	if fillvvAny.Kind() == Optional {
		fillvvAny.Assign(OptionalValue(fillvv))
	}
	if vv.Kind() == Any {
		vv.Assign(fillvvAny)
	}
	return dec.FinishValue()
}

// readHandleNil handles the case that dec.IsNil() is true
func (vv *Value) readHandleNil(dec Decoder) error {
	switch {
	case dec.IsOptional():
		// handles optional inside-any and optional on-its-own cases
		if dec.Type().Kind() != Optional {
			return fmt.Errorf("invalid optional value returned from decoder of type %v", dec.Type())
		}
		vv.Assign(ZeroValue(dec.Type()))
	case dec.IsAny():
		vv.Assign(nil)
	default:
		return fmt.Errorf("invalid non-any, non-optional nil value of type %v", dec.Type())
	}
	return dec.FinishValue()
}

func (vv *Value) readFillValue(dec Decoder) error {
	if vv.Type().IsBytes() {
		fixedLength := -1
		if vv.Kind() == Array {
			fixedLength = vv.Type().Len()
		}
		var val []byte
		if err := dec.DecodeBytes(fixedLength, &val); err != nil {
			return err
		}
		vv.AssignBytes(val)
		return nil
	}
	switch vv.Kind() {
	case Bool:
		val, err := dec.DecodeBool()
		if err != nil {
			return err
		}
		vv.AssignBool(val)
	case Byte, Uint16, Uint32, Uint64:
		val, err := dec.DecodeUint(uint(bitlenV(vv.Kind())))
		if err != nil {
			return err
		}
		vv.AssignUint(val)
	case Int8, Int16, Int32, Int64:
		val, err := dec.DecodeInt(uint(bitlenV(vv.Kind())))
		if err != nil {
			return err
		}
		vv.AssignInt(val)
	case Float32, Float64:
		val, err := dec.DecodeFloat(uint(bitlenV(vv.Kind())))
		if err != nil {
			return err
		}
		vv.AssignFloat(val)
	case String:
		val, err := dec.DecodeString()
		if err != nil {
			return err
		}
		vv.AssignString(val)
	case TypeObject:
		val, err := dec.DecodeTypeObject()
		if err != nil {
			return err
		}
		vv.AssignTypeObject(val)
	case Enum:
		val, err := dec.DecodeString()
		if err != nil {
			return err
		}
		index := vv.Type().EnumIndex(val)
		if index == -1 {
			return fmt.Errorf("vdl: %v invalid enum label %q", vv.Type(), val)
		}
		vv.AssignEnumIndex(index)
	case Array:
		if err := vv.readArray(dec); err != nil {
			return err
		}
	case List:
		if err := vv.readList(dec); err != nil {
			return err
		}
	case Set:
		if err := vv.readSet(dec); err != nil {
			return err
		}
	case Map:
		if err := vv.readMap(dec); err != nil {
			return err
		}
	case Struct:
		if err := vv.readStruct(dec); err != nil {
			return err
		}
	case Union:
		if err := vv.readUnion(dec); err != nil {
			return err
		}
	default:
		panic(fmt.Sprintf("unhandled type: %v", vv.Type()))
	}
	return nil
}

func (vv *Value) checkCompatible(dec Decoder) error {
	if (dec.StackDepth() == 1 || dec.IsAny()) && !Compatible(vv.Type(), dec.Type()) {
		return fmt.Errorf("incompatible %v %v, from %v", vv.Kind(), vv.Type(), dec.Type())
	}
	return nil
}

func (vv *Value) readArray(dec Decoder) error {
	if err := vv.checkCompatible(dec); err != nil {
		return err
	}
	index := 0
	for {
		switch done, err := dec.NextEntry(); {
		case err != nil:
			return err
		case done != (index >= vv.Type().Len()):
			return fmt.Errorf("array len mismatch, got %d, want %v", index, vv.Type())
		case done:
			return nil
		}
		if err := vv.Index(index).VDLRead(dec); err != nil {
			return err
		}
		index++
	}
}

func (vv *Value) readList(dec Decoder) error {
	if err := vv.checkCompatible(dec); err != nil {
		return err
	}
	switch len := dec.LenHint(); {
	case len >= 0:
		vv.AssignLen(len)
	default:
		// Assign 0 length when we don't have a hint.
		vv.AssignLen(0)
	}
	index := 0
	for {
		switch done, err := dec.NextEntry(); {
		case err != nil:
			return err
		case done:
			return nil
		}
		if needLen := index + 1; needLen > vv.Len() {
			var cap int
			if needLen <= 1024 {
				cap = needLen * 2
			} else {
				cap = needLen + needLen/4
			}
			// Grow the underlying buffer.  The first AssignLen grows the buffer to
			// the capacity, while the second AssignLen sets the actual length.
			//
			// TODO(toddw): Consider changing the Value API to either add an Append
			// method, or to allow the user to explicitly manage the capacity.
			vv.AssignLen(cap)
			vv.AssignLen(needLen)
		}
		if err := vv.Index(index).VDLRead(dec); err != nil {
			return err
		}
		index++
	}
}

func (vv *Value) readSet(dec Decoder) error {
	if err := vv.checkCompatible(dec); err != nil {
		return err
	}
	for {
		switch done, err := dec.NextEntry(); {
		case err != nil:
			return err
		case done:
			return nil
		}
		key := ZeroValue(vv.Type().Key())
		if err := key.VDLRead(dec); err != nil {
			return err
		}
		vv.AssignSetKey(key)
	}
}

func (vv *Value) readMap(dec Decoder) error {
	if err := vv.checkCompatible(dec); err != nil {
		return err
	}
	for {
		switch done, err := dec.NextEntry(); {
		case err != nil:
			return err
		case done:
			return nil
		}
		key := ZeroValue(vv.Type().Key())
		if err := key.VDLRead(dec); err != nil {
			return err
		}
		elem := ZeroValue(vv.Type().Elem())
		if err := elem.VDLRead(dec); err != nil {
			return err
		}
		vv.AssignMapIndex(key, elem)
	}
}

func (vv *Value) readStruct(dec Decoder) error {
	if err := vv.checkCompatible(dec); err != nil {
		return err
	}
	// Reset to zero struct, since fields may be missing.
	vv.Assign(nil)
	for {
		name, err := dec.NextField()
		switch {
		case err != nil:
			return err
		case name == "":
			return nil
		}
		switch field := vv.StructFieldByName(name); {
		case field != nil:
			if err := field.VDLRead(dec); err != nil {
				return err
			}
		default:
			if err := dec.SkipValue(); err != nil {
				return err
			}
		}
	}
}

func (vv *Value) readUnion(dec Decoder) error {
	if err := vv.checkCompatible(dec); err != nil {
		return err
	}
	name, err := dec.NextField()
	switch {
	case err != nil:
		return err
	case name == "":
		return fmt.Errorf("missing field in union %v, from %v", vv.Type(), dec.Type())
	}
	field, index := vv.Type().FieldByName(name)
	if index < 0 {
		return fmt.Errorf("field %q not in union %v, from %v", name, vv.Type(), dec.Type())
	}
	elem := ZeroValue(field.Type)
	if err := elem.VDLRead(dec); err != nil {
		return err
	}
	vv.AssignUnionField(index, elem)
	switch name, err := dec.NextField(); {
	case err != nil:
		return err
	case name != "":
		return fmt.Errorf("extra field %q in union %v, from %v", name, vv.Type(), dec.Type())
	}
	return nil
}
