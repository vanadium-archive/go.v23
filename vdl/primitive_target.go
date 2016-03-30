// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"fmt"

	"v.io/v23/vdl/vdlconv"
)

type BoolTarget struct {
	Value *bool
	TargetBase
}

func (t *BoolTarget) FromBool(src bool, tt *Type) error {
	if !Compatible(tt, BoolType) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, BoolType)
	}
	*t.Value = src
	return nil
}
func (t *BoolTarget) FromZero(tt *Type) error {
	if !Compatible(tt, BoolType) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, BoolType)
	}
	*t.Value = false
	return nil
}

type ByteTarget struct {
	Value *byte
	TargetBase
}

func (t *ByteTarget) FromUint(src uint64, tt *Type) (err error) {
	if !Compatible(tt, ByteType) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, ByteType)
	}
	*t.Value, err = vdlconv.Uint64ToUint8(src)
	return
}
func (t *ByteTarget) FromInt(src int64, tt *Type) (err error) {
	if !Compatible(tt, ByteType) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, ByteType)
	}
	*t.Value, err = vdlconv.Int64ToUint8(src)
	return
}
func (t *ByteTarget) FromFloat(src float64, tt *Type) (err error) {
	if !Compatible(tt, ByteType) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, ByteType)
	}
	*t.Value, err = vdlconv.Float64ToUint8(src)
	return
}
func (t *ByteTarget) FromZero(tt *Type) error {
	if !Compatible(tt, ByteType) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, ByteType)
	}
	*t.Value = 0
	return nil
}

type Uint16Target struct {
	Value *uint16
	TargetBase
}

func (t *Uint16Target) FromUint(src uint64, tt *Type) (err error) {
	if !Compatible(tt, Uint16Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Uint16Type)
	}
	*t.Value, err = vdlconv.Uint64ToUint16(src)
	return
}
func (t *Uint16Target) FromInt(src int64, tt *Type) (err error) {
	if !Compatible(tt, Uint16Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Uint16Type)
	}
	*t.Value, err = vdlconv.Int64ToUint16(src)
	return
}
func (t *Uint16Target) FromFloat(src float64, tt *Type) (err error) {
	if !Compatible(tt, Uint16Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Uint16Type)
	}
	*t.Value, err = vdlconv.Float64ToUint16(src)
	return
}
func (t *Uint16Target) FromZero(tt *Type) error {
	if !Compatible(tt, Uint16Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Uint16Type)
	}
	*t.Value = 0
	return nil
}

type Uint32Target struct {
	Value *uint32
	TargetBase
}

func (t *Uint32Target) FromUint(src uint64, tt *Type) (err error) {
	if !Compatible(tt, Uint32Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Uint32Type)
	}
	*t.Value, err = vdlconv.Uint64ToUint32(src)
	return
}
func (t *Uint32Target) FromInt(src int64, tt *Type) (err error) {
	if !Compatible(tt, Uint32Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Uint32Type)
	}
	*t.Value, err = vdlconv.Int64ToUint32(src)
	return
}
func (t *Uint32Target) FromFloat(src float64, tt *Type) (err error) {
	if !Compatible(tt, Uint32Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Uint32Type)
	}
	*t.Value, err = vdlconv.Float64ToUint32(src)
	return
}
func (t *Uint32Target) FromZero(tt *Type) error {
	if !Compatible(tt, Uint32Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Uint32Type)
	}
	*t.Value = 0
	return nil
}

type Uint64Target struct {
	Value *uint64
	TargetBase
}

func (t *Uint64Target) FromUint(src uint64, tt *Type) (err error) {
	if !Compatible(tt, Uint64Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Uint64Type)
	}
	*t.Value = src
	return
}
func (t *Uint64Target) FromInt(src int64, tt *Type) (err error) {
	if !Compatible(tt, Uint64Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Uint64Type)
	}
	*t.Value, err = vdlconv.Int64ToUint64(src)
	return
}
func (t *Uint64Target) FromFloat(src float64, tt *Type) (err error) {
	if !Compatible(tt, Uint64Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Uint64Type)
	}
	*t.Value, err = vdlconv.Float64ToUint64(src)
	return
}
func (t *Uint64Target) FromZero(tt *Type) error {
	if !Compatible(tt, Uint64Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Uint64Type)
	}
	*t.Value = 0
	return nil
}

type Int8Target struct {
	Value *int8
	TargetBase
}

func (t *Int8Target) FromUint(src uint64, tt *Type) (err error) {
	if !Compatible(tt, Int8Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Int8Type)
	}
	*t.Value, err = vdlconv.Uint64ToInt8(src)
	return
}
func (t *Int8Target) FromInt(src int64, tt *Type) (err error) {
	if !Compatible(tt, Int8Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Int8Type)
	}
	*t.Value, err = vdlconv.Int64ToInt8(src)
	return
}
func (t *Int8Target) FromFloat(src float64, tt *Type) (err error) {
	if !Compatible(tt, Int8Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Int8Type)
	}
	*t.Value, err = vdlconv.Float64ToInt8(src)
	return
}
func (t *Int8Target) FromZero(tt *Type) error {
	if !Compatible(tt, Int8Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Int8Type)
	}
	*t.Value = 0
	return nil
}

type Int16Target struct {
	Value *int16
	TargetBase
}

func (t *Int16Target) FromUint(src uint64, tt *Type) (err error) {
	if !Compatible(tt, Int16Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Int16Type)
	}
	*t.Value, err = vdlconv.Uint64ToInt16(src)
	return
}
func (t *Int16Target) FromInt(src int64, tt *Type) (err error) {
	if !Compatible(tt, Int16Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Int16Type)
	}
	*t.Value, err = vdlconv.Int64ToInt16(src)
	return
}
func (t *Int16Target) FromFloat(src float64, tt *Type) (err error) {
	if !Compatible(tt, Int16Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Int16Type)
	}
	*t.Value, err = vdlconv.Float64ToInt16(src)
	return
}
func (t *Int16Target) FromZero(tt *Type) error {
	if !Compatible(tt, Int16Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Int16Type)
	}
	*t.Value = 0
	return nil
}

type Int32Target struct {
	Value *int32
	TargetBase
}

func (t *Int32Target) FromUint(src uint64, tt *Type) (err error) {
	if !Compatible(tt, Int32Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Int32Type)
	}
	*t.Value, err = vdlconv.Uint64ToInt32(src)
	return
}
func (t *Int32Target) FromInt(src int64, tt *Type) (err error) {
	if !Compatible(tt, Int32Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Int32Type)
	}
	*t.Value, err = vdlconv.Int64ToInt32(src)
	return
}
func (t *Int32Target) FromFloat(src float64, tt *Type) (err error) {
	if !Compatible(tt, Int32Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Int32Type)
	}
	*t.Value, err = vdlconv.Float64ToInt32(src)
	return
}
func (t *Int32Target) FromZero(tt *Type) error {
	if !Compatible(tt, Int32Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Int32Type)
	}
	*t.Value = 0
	return nil
}

type Int64Target struct {
	Value *int64
	TargetBase
}

func (t *Int64Target) FromUint(src uint64, tt *Type) (err error) {
	if !Compatible(tt, Int64Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Int64Type)
	}
	*t.Value, err = vdlconv.Uint64ToInt64(src)
	return
}
func (t *Int64Target) FromInt(src int64, tt *Type) (err error) {
	if !Compatible(tt, Int64Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Int64Type)
	}
	*t.Value = src
	return
}
func (t *Int64Target) FromFloat(src float64, tt *Type) (err error) {
	if !Compatible(tt, Int64Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Int64Type)
	}
	*t.Value, err = vdlconv.Float64ToInt64(src)
	return
}
func (t *Int64Target) FromZero(tt *Type) error {
	if !Compatible(tt, Int64Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Int64Type)
	}
	*t.Value = 0
	return nil
}

type Float32Target struct {
	Value *float32
	TargetBase
}

func (t *Float32Target) FromUint(src uint64, tt *Type) (err error) {
	if !Compatible(tt, Float32Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Float32Type)
	}
	*t.Value, err = vdlconv.Uint64ToFloat32(src)
	return
}
func (t *Float32Target) FromInt(src int64, tt *Type) (err error) {
	if !Compatible(tt, Float32Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Float32Type)
	}
	*t.Value, err = vdlconv.Int64ToFloat32(src)
	return
}
func (t *Float32Target) FromFloat(src float64, tt *Type) (err error) {
	if !Compatible(tt, Float32Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Float32Type)
	}
	*t.Value, err = vdlconv.Float64ToFloat32(src)
	return
}
func (t *Float32Target) FromZero(tt *Type) error {
	if !Compatible(tt, Float32Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Float32Type)
	}
	*t.Value = 0
	return nil
}

type Float64Target struct {
	Value *float64
	TargetBase
}

func (t *Float64Target) FromUint(src uint64, tt *Type) (err error) {
	if !Compatible(tt, Float64Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Float64Type)
	}
	*t.Value, err = vdlconv.Uint64ToFloat64(src)
	return
}
func (t *Float64Target) FromInt(src int64, tt *Type) (err error) {
	if !Compatible(tt, Float64Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Float64Type)
	}
	*t.Value, err = vdlconv.Int64ToFloat64(src)
	return
}
func (t *Float64Target) FromFloat(src float64, tt *Type) (err error) {
	if !Compatible(tt, Float64Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Float64Type)
	}
	*t.Value = src
	return
}
func (t *Float64Target) FromZero(tt *Type) error {
	if !Compatible(tt, Float64Type) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, Float64Type)
	}
	*t.Value = 0
	return nil
}

type StringTarget struct {
	Value *string
	TargetBase
}

func (t *StringTarget) FromString(src string, tt *Type) error {
	if !Compatible(tt, StringType) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, StringType)
	}
	*t.Value = src
	return nil
}
func (t *StringTarget) FromZero(tt *Type) error {
	if !Compatible(tt, StringType) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, StringType)
	}
	*t.Value = ""
	return nil
}

type BytesTarget struct {
	Value *[]byte
	TargetBase
}

var bytesType = TypeOf([]byte{})

func (t *BytesTarget) FromBytes(src []byte, tt *Type) error {
	if !Compatible(tt, bytesType) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, StringType)
	}
	if len(src) == 0 {
		*t.Value = nil
	} else {
		*t.Value = make([]byte, len(src))
		copy(*t.Value, src)
	}
	return nil
}
func (t *BytesTarget) FromZero(tt *Type) error {
	if !Compatible(tt, bytesType) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, bytesType)
	}
	*t.Value = nil
	return nil
}

type TypeObjectTarget struct {
	Value **Type
	TargetBase
}

func (t *TypeObjectTarget) FromTypeObject(tt *Type) error {
	*t.Value = tt
	return nil
}
func (t *TypeObjectTarget) FromZero(tt *Type) error {
	if !Compatible(tt, TypeObjectType) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, TypeObjectType)
	}
	*t.Value = AnyType
	return nil
}

type StringSliceTarget struct {
	Value *[]string
	TargetBase
	ListTargetBase
}

var stringSliceType = TypeOf([]string{})

func (t *StringSliceTarget) StartList(tt *Type, len int) (ListTarget, error) {
	if !Compatible(tt, stringSliceType) {
		return nil, fmt.Errorf("Type %v incompatible with expected type %v", tt, stringSliceType)
	}
	if cap(*t.Value) < len {
		*t.Value = make([]string, len)
	} else {
		*t.Value = (*t.Value)[:len]
	}
	return t, nil
}
func (t *StringSliceTarget) StartElem(index int) (elem Target, _ error) {
	return &StringTarget{Value: &(*t.Value)[index]}, error(nil)
}
func (t *StringSliceTarget) FinishElem(elem Target) error {
	return nil
}
func (t *StringSliceTarget) FinishList(elem ListTarget) error {
	return nil
}
func (t *StringSliceTarget) FromZero(tt *Type) error {
	if !Compatible(tt, stringSliceType) {
		return fmt.Errorf("Type %v incompatible with expected type %v", tt, stringSliceType)
	}
	*t.Value = nil
	return nil
}
