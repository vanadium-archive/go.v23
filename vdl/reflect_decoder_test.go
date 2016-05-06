// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build reflectdecoder

package vdl_test

import (
	"testing"

	"fmt"
	"reflect"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	"v.io/v23/vom/testdata/data81"
)

// Tests ReflectDecoder using ValueRead.
func TestReflectDecoder(t *testing.T) {
	for _, test := range data81.Tests {
		out := vdl.ZeroValue(test.Value.Type())
		goValue, err := toGoValue(test.Value)
		if err != nil {
			t.Errorf("%s: error in toGoValue: %v", test.Name, err)
			continue
		}
		dec := vdl.NewReflectDecoder(reflect.ValueOf(goValue))
		if err := out.VDLRead(dec); err != nil {
			t.Errorf("%s: error in ValueRead: %v", test.Name, err)
			continue
		}
		if !vdl.EqualValue(test.Value, out) {
			t.Errorf("%s: got %v, want %v", test.Name, out, test.Value)
		}
	}
}

func toGoValue(value *vdl.Value) (interface{}, error) {
	if value == nil {
		return nil, nil
	}
	if value.Kind() == vdl.Any {
		return nil, nil
	}
	rt := vdl.TypeToReflect(value.Type())
	if rt == nil {
		return reflect.Value{}, fmt.Errorf("TypeToReflect(%v) failed", value.Type())
	}
	rv := reflect.New(rt)
	if err := vdl.Convert(rv.Interface(), value); err != nil {
		return reflect.Value{}, fmt.Errorf("vdl.Convert(%T, %v) failed: %v", rt, value, err)
	}
	return rv.Elem().Interface(), nil
}

type interfaceStructAny struct {
	Any interface{}
}

// Tests ReflectDecoder with a non-empty any inside of an interface.
func TestReflectDecoderInterfaceAny(t *testing.T) {
	dec := vdl.NewReflectDecoder(reflect.ValueOf(interfaceStructAny{true}))
	if err := dec.StartValue(); err != nil {
		t.Fatalf("error in StartValue: %v", err)
	}
	switch name, err := dec.NextField(); {
	case err != nil:
		t.Fatalf("error in NextField: %v", err)
	case name != "Any":
		t.Errorf("incorrect field name. got %q, want %q", name, "Any")
	}
	if err := dec.StartValue(); err != nil {
		t.Fatalf("error in StartValue: %v", err)
	}
	if got, want := dec.Type(), vdl.BoolType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := dec.IsAny(), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := dec.IsNil(), false; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	switch val, err := dec.DecodeBool(); {
	case err != nil:
		t.Fatalf("error in DecodeBool: %v", err)
	case val != true:
		t.Errorf("got %v, want %v", val, true)
	}
	if err := dec.FinishValue(); err != nil {
		t.Fatalf("error in FinishValue: %v", err)
	}
	if err := dec.FinishValue(); err != nil {
		t.Fatalf("error in FinishValue: %v", err)
	}
}
