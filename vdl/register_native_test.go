// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"fmt"
	"reflect"
	"testing"
)

type (
	wireA   struct{}
	nativeA struct{}

	nativeError struct{}
)

func equalNativeInfo(a, b nativeInfo) bool {
	// We don't care about comparing the stack traces.
	a.stack = nil
	b.stack = nil
	return reflect.DeepEqual(a, b)
}

func TestRegisterNative(t *testing.T) {
	tests := []nativeInfo{
		{
			reflect.TypeOf(wireA{}),
			reflect.TypeOf(nativeA{}),
			reflect.ValueOf(func(wireA, *nativeA) error { return nil }),
			reflect.ValueOf(func(*wireA, nativeA) error { return nil }),
			nil,
		},
		// TODO(toddw): Add tests where the wire type is a VDL union.

		// We can only register the error conversion once, and it's registered in
		// convert_test via the verror package, so we can't check registration.
	}
	for _, test := range tests {
		name := fmt.Sprint("[%v %v]", test.WireType, test.NativeType)
		RegisterNative(test.toNativeFunc.Interface(), test.fromNativeFunc.Interface())
		if got, want := nativeInfoFromWire(test.WireType), test; !equalNativeInfo(*got, want) {
			t.Errorf("%s nativeInfoFromWire got %#v, want %#v", name, got, want)
		}
		if got, want := nativeInfoFromNative(test.NativeType), test; !equalNativeInfo(*got, want) {
			t.Errorf("%s nativeInfoFromNative got %#v, want %#v", name, got, want)
		}
	}
}

func TestDeriveNativeInfo(t *testing.T) {
	tests := []nativeInfo{
		{
			reflect.TypeOf(wireA{}),
			reflect.TypeOf(nativeA{}),
			reflect.ValueOf(func(wireA, *nativeA) error { return nil }),
			reflect.ValueOf(func(*wireA, nativeA) error { return nil }),
			nil,
		},
		{
			// Check our special-casing for the conversion functions for errors.
			reflect.TypeOf(WireError{}),
			reflect.TypeOf(nativeError{}),
			reflect.ValueOf(func(WireError, *nativeError) error { return nil }),
			reflect.ValueOf(func(*WireError, error) error { return nil }),
			nil,
		},
	}
	for _, test := range tests {
		name := fmt.Sprint("[%v %v]", test.WireType, test.NativeType)
		ni, err := deriveNativeInfo(test.toNativeFunc.Interface(), test.fromNativeFunc.Interface())
		if err != nil {
			t.Errorf("%s got error: %v", err)
		}
		if got, want := ni, test; !equalNativeInfo(*got, want) {
			t.Errorf("%s got %#v, want %#v", name, got, want)
		}
	}
}

func TestDeriveNativeInfoError(t *testing.T) {
	const (
		errTo     = "toFn must have signature ToNative(wire W, native *N) error"
		errFrom   = "fromFn must have signature FromNative(wire *W, native N) error"
		errMis    = "mismatched wire/native types"
		errMisErr = "mismatched error conversion"
	)
	var (
		goodTo   = func(wireA, *nativeA) error { return nil }
		goodFrom = func(*wireA, nativeA) error { return nil }
	)

	tests := []struct {
		Name                 string
		ToNative, FromNative interface{}
		ErrStr               string
	}{
		{"NilFuncs", nil, nil, "nil arguments"},
		{"NotFuncs", "abc", "abc", "arguments must be functions"},
		{"BadTo1", func() {}, goodFrom, errTo},
		{"BadTo2", func(wireA) {}, goodFrom, errTo},
		{"BadTo3", func(wireA, nativeA) {}, goodFrom, errTo},
		{"BadTo3", func(wireA, *nativeA) {}, goodFrom, errTo},
		{"BadTo3", func(wireA, nativeA) error { return nil }, goodFrom, errTo},
		{"BadFrom1", goodTo, func() {}, errFrom},
		{"BadFrom2", goodTo, func(wireA) {}, errFrom},
		{"BadFrom3", goodTo, func(wireA, nativeA) {}, errFrom},
		{"BadFrom3", goodTo, func(*wireA, nativeA) {}, errFrom},
		{"BadFrom3", goodTo, func(wireA, nativeA) error { return nil }, errFrom},
		{
			"Mismatch1",
			func(string, *nativeA) error { return nil },
			func(*wireA, nativeA) error { return nil },
			errMis,
		},
		{
			"Mismatch2",
			func(wireA, *string) error { return nil },
			func(*wireA, nativeA) error { return nil },
			errMis,
		},
		{
			"Mismatch3",
			func(wireA, *nativeA) error { return nil },
			func(*string, nativeA) error { return nil },
			errMis,
		},
		{
			"Mismatch4",
			func(wireA, *nativeA) error { return nil },
			func(*wireA, string) error { return nil },
			errMis,
		},
		{
			"MismatchPtr1",
			func(*wireA, *nativeA) error { return nil },
			func(*wireA, nativeA) error { return nil },
			errMis,
		},
		{
			"MismatchPtr2",
			func(wireA, *nativeA) error { return nil },
			func(*wireA, *nativeA) error { return nil },
			errMis,
		},
		{
			"MismatchError1",
			func(WireError, *nativeA) error { return nil },
			func(*WireError, nativeA) error { return nil },
			errMisErr,
		},
		{
			"MismatchError2",
			func(WireError, *error) error { return nil },
			func(*WireError, error) error { return nil },
			errMisErr,
		},
		{
			"MismatchError3",
			func(WireError, *error) error { return nil },
			func(*WireError, nativeA) error { return nil },
			errMisErr,
		},
		{
			"SameType",
			func(wireA, *wireA) error { return nil },
			func(*wireA, wireA) error { return nil },
			"wire type == native type",
		},
	}
	for _, test := range tests {
		ni, err := deriveNativeInfo(test.ToNative, test.FromNative)
		if ni != nil {
			t.Errorf("%s got %#v, want nil", test.Name, ni)
		}
		ExpectErr(t, err, test.ErrStr, test.Name)
	}
}
