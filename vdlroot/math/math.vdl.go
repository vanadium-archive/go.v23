// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: math

package math

import (
	"fmt"
	"v.io/v23/vdl"
)

var _ = __VDLInit() // Must be first; see __VDLInit comments for details.

//////////////////////////////////////////////////
// Type definitions

type Complex64 struct {
	Real float32
	Imag float32
}

func (Complex64) __VDLReflect(struct {
	Name string `vdl:"math.Complex64"`
}) {
}

func (m *Complex64) FillVDLTarget(t vdl.Target, tt *vdl.Type) error {
	fieldsTarget1, err := t.StartFields(tt)
	if err != nil {
		return err
	}

	keyTarget2, fieldTarget3, err := fieldsTarget1.StartField("Real")
	if err != vdl.ErrFieldNoExist && err != nil {
		return err
	}
	if err != vdl.ErrFieldNoExist {
		if err := fieldTarget3.FromFloat(float64(m.Real), tt.NonOptional().Field(0).Type); err != nil {
			return err
		}
		if err := fieldsTarget1.FinishField(keyTarget2, fieldTarget3); err != nil {
			return err
		}
	}
	keyTarget4, fieldTarget5, err := fieldsTarget1.StartField("Imag")
	if err != vdl.ErrFieldNoExist && err != nil {
		return err
	}
	if err != vdl.ErrFieldNoExist {
		if err := fieldTarget5.FromFloat(float64(m.Imag), tt.NonOptional().Field(1).Type); err != nil {
			return err
		}
		if err := fieldsTarget1.FinishField(keyTarget4, fieldTarget5); err != nil {
			return err
		}
	}
	if err := t.FinishFields(fieldsTarget1); err != nil {
		return err
	}
	return nil
}

func (m *Complex64) MakeVDLTarget() vdl.Target {
	return nil
}

type Complex64Target struct {
	Value      *complex64
	wireValue  Complex64
	realTarget vdl.Float32Target
	imagTarget vdl.Float32Target
	vdl.TargetBase
	vdl.FieldsTargetBase
}

func (t *Complex64Target) StartFields(tt *vdl.Type) (vdl.FieldsTarget, error) {
	t.wireValue = Complex64{}
	if ttWant := vdl.TypeOf((*Complex64)(nil)).Elem(); !vdl.Compatible(tt, ttWant) {
		return nil, fmt.Errorf("type %v incompatible with %v", tt, ttWant)
	}
	return t, nil
}
func (t *Complex64Target) StartField(name string) (key, field vdl.Target, _ error) {
	switch name {
	case "Real":
		t.realTarget.Value = &t.wireValue.Real
		target, err := &t.realTarget, error(nil)
		return nil, target, err
	case "Imag":
		t.imagTarget.Value = &t.wireValue.Imag
		target, err := &t.imagTarget, error(nil)
		return nil, target, err
	default:
		return nil, nil, fmt.Errorf("field %s not in struct math.Complex64", name)
	}
}
func (t *Complex64Target) FinishField(_, _ vdl.Target) error {
	return nil
}
func (t *Complex64Target) FinishFields(_ vdl.FieldsTarget) error {

	if err := Complex64ToNative(t.wireValue, t.Value); err != nil {
		return err
	}
	return nil
}

type Complex128 struct {
	Real float64
	Imag float64
}

func (Complex128) __VDLReflect(struct {
	Name string `vdl:"math.Complex128"`
}) {
}

func (m *Complex128) FillVDLTarget(t vdl.Target, tt *vdl.Type) error {
	fieldsTarget1, err := t.StartFields(tt)
	if err != nil {
		return err
	}

	keyTarget2, fieldTarget3, err := fieldsTarget1.StartField("Real")
	if err != vdl.ErrFieldNoExist && err != nil {
		return err
	}
	if err != vdl.ErrFieldNoExist {
		if err := fieldTarget3.FromFloat(float64(m.Real), tt.NonOptional().Field(0).Type); err != nil {
			return err
		}
		if err := fieldsTarget1.FinishField(keyTarget2, fieldTarget3); err != nil {
			return err
		}
	}
	keyTarget4, fieldTarget5, err := fieldsTarget1.StartField("Imag")
	if err != vdl.ErrFieldNoExist && err != nil {
		return err
	}
	if err != vdl.ErrFieldNoExist {
		if err := fieldTarget5.FromFloat(float64(m.Imag), tt.NonOptional().Field(1).Type); err != nil {
			return err
		}
		if err := fieldsTarget1.FinishField(keyTarget4, fieldTarget5); err != nil {
			return err
		}
	}
	if err := t.FinishFields(fieldsTarget1); err != nil {
		return err
	}
	return nil
}

func (m *Complex128) MakeVDLTarget() vdl.Target {
	return nil
}

type Complex128Target struct {
	Value      *complex128
	wireValue  Complex128
	realTarget vdl.Float64Target
	imagTarget vdl.Float64Target
	vdl.TargetBase
	vdl.FieldsTargetBase
}

func (t *Complex128Target) StartFields(tt *vdl.Type) (vdl.FieldsTarget, error) {
	t.wireValue = Complex128{}
	if ttWant := vdl.TypeOf((*Complex128)(nil)).Elem(); !vdl.Compatible(tt, ttWant) {
		return nil, fmt.Errorf("type %v incompatible with %v", tt, ttWant)
	}
	return t, nil
}
func (t *Complex128Target) StartField(name string) (key, field vdl.Target, _ error) {
	switch name {
	case "Real":
		t.realTarget.Value = &t.wireValue.Real
		target, err := &t.realTarget, error(nil)
		return nil, target, err
	case "Imag":
		t.imagTarget.Value = &t.wireValue.Imag
		target, err := &t.imagTarget, error(nil)
		return nil, target, err
	default:
		return nil, nil, fmt.Errorf("field %s not in struct math.Complex128", name)
	}
}
func (t *Complex128Target) FinishField(_, _ vdl.Target) error {
	return nil
}
func (t *Complex128Target) FinishFields(_ vdl.FieldsTarget) error {

	if err := Complex128ToNative(t.wireValue, t.Value); err != nil {
		return err
	}
	return nil
}

// Create zero values for each type.
var (
	__VDLZeroComplex64 = func() complex64 {
		var native complex64
		if err := vdl.Convert(&native, Complex64{}); err != nil {
			panic(err)
		}
		return native
	}()
	__VDLZeroComplex128 = func() complex128 {
		var native complex128
		if err := vdl.Convert(&native, Complex128{}); err != nil {
			panic(err)
		}
		return native
	}()
)

// Type-check native conversion functions.
var (
	_ func(Complex128, *complex128) error = Complex128ToNative
	_ func(*Complex128, complex128) error = Complex128FromNative
	_ func(Complex64, *complex64) error   = Complex64ToNative
	_ func(*Complex64, complex64) error   = Complex64FromNative
)

var __VDLInitCalled bool

// __VDLInit performs vdl initialization.  It is safe to call multiple times.
// If you have an init ordering issue, just insert the following line verbatim
// into your source files in this package, right after the "package foo" clause:
//
//    var _ = __VDLInit()
//
// The purpose of this function is to ensure that vdl initialization occurs in
// the right order, and very early in the init sequence.  In particular, vdl
// registration and package variable initialization needs to occur before
// functions like vdl.TypeOf will work properly.
//
// This function returns a dummy value, so that it can be used to initialize the
// first var in the file, to take advantage of Go's defined init order.
func __VDLInit() struct{} {
	if __VDLInitCalled {
		return struct{}{}
	}

	// Register native type conversions first, so that vdl.TypeOf works.
	vdl.RegisterNative(Complex128ToNative, Complex128FromNative)
	vdl.RegisterNative(Complex64ToNative, Complex64FromNative)

	// Register types.
	vdl.Register((*Complex64)(nil))
	vdl.Register((*Complex128)(nil))

	return struct{}{}
}
