// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate ./gen_funcs.sh

package vdlconv_test

import (
	"reflect"
	"testing"
)

var floatKinds = map[reflect.Kind]bool{reflect.Float32: true, reflect.Float64: true}
var floatOrComplexKinds = map[reflect.Kind]bool{reflect.Float32: true, reflect.Float64: true, reflect.Complex64: true, reflect.Complex128: true}
var sourceKinds = map[reflect.Kind]bool{reflect.Uint64: true, reflect.Int64: true, reflect.Float64: true, reflect.Complex128: true}
var targetKinds = []reflect.Kind{reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128}

// TestNumericConversions tests all possible combinations of conversions from the sourceKinds to the targetKinds using
// the supplied list of compatible value groups.
func TestNumericConversions(t *testing.T) {
	// Each list is a set of types that can be validly converted between one another. Converting to any other type
	// should trigger an error.
	validGroups := []struct {
		name  string
		items []interface{}
	}{
		{
			name:  "0",
			items: []interface{}{uint8(0), uint16(0), uint32(0), uint64(0), int8(0), int16(0), int32(0), int64(0), float32(0), float64(0), complex64(0), complex128(0)},
		},
		{
			name:  "0x7f: max usable by both uint8 and int8",
			items: []interface{}{uint8(0x7f), uint16(0x7f), uint32(0x7f), uint64(0x7f), int8(0x7f), int16(0x7f), int32(0x7f), int64(0x7f), float32(0x7f), float64(0x7f), complex64(0x7f), complex128(0x7f)},
		},
		{
			name:  "0xff: max usable by uint8",
			items: []interface{}{uint8(0xff), uint16(0xff), uint32(0xff), uint64(0xff), int16(0xff), int32(0xff), int64(0xff), float32(0xff), float64(0xff), complex64(0xff), complex128(0xff)},
		},
		{
			name:  "-0x80: min usable by int8",
			items: []interface{}{int8(-0x80), int16(-0x80), int32(-0x80), int64(-0x80), float32(-0x80), float64(-0x80), complex64(-0x80), complex128(-0x80)},
		},
		{
			name:  "0x7fff: max usable by both uint16 and int16",
			items: []interface{}{uint16(0x7fff), uint32(0x7fff), uint64(0x7fff), int16(0x7fff), int32(0x7fff), int64(0x7fff), float32(0x7fff), float64(0x7fff), complex64(0x7fff), complex128(0x7fff)},
		},
		{
			name:  "0xffff: max usable by uint16",
			items: []interface{}{uint16(0xffff), uint32(0xffff), uint64(0xffff), int32(0xffff), int64(0xffff), float32(0xffff), float64(0xffff), complex64(0xffff), complex128(0xffff)},
		},
		{
			name:  "-0x8000: min usable by int16",
			items: []interface{}{int16(-0x8000), int32(-0x8000), int64(-0x8000), float32(-0x8000), float64(-0x8000), complex64(-0x8000), complex128(-0x8000)},
		},
		{
			name:  "(1<<24)=16777216: max integer without loss in float32",
			items: []interface{}{uint32((1 << 24)), uint64((1 << 24)), int32((1 << 24)), int64((1 << 24)), float32((1 << 24)), float64((1 << 24)), complex64((1 << 24)), complex128((1 << 24))},
		},
		{
			name:  "(1<<24)+1=16777217: one more than max integer without loss in float32",
			items: []interface{}{uint32((1 << 24) + 1), uint64((1 << 24) + 1), int32((1 << 24) + 1), int64((1 << 24) + 1), float64((1 << 24) + 1), complex128((1 << 24) + 1)},
		},
		{
			name:  "-(1<<24)=-16777216: min integer without loss in float32",
			items: []interface{}{int32(-(1 << 24)), int64(-(1 << 24)), float32(-(1 << 24)), float64(-(1 << 24)), complex64(-(1 << 24)), complex128(-(1 << 24))},
		},
		{
			name:  "-(1<<24)-1=-16777217: one less than min integer without loss in float32",
			items: []interface{}{int32(-(1 << 24) - 1), int64(-(1 << 24) - 1), float64(-(1 << 24) - 1), complex128(-(1 << 24) - 1)},
		},
		{
			name:  "0x7fffffff: max usable by both uint32 and int32",
			items: []interface{}{uint32(0x7fffffff), uint64(0x7fffffff), int32(0x7fffffff), int64(0x7fffffff), float64(0x7fffffff), complex128(0x7fffffff)},
		},
		{
			name:  "0xffffffff: max usable by uint16",
			items: []interface{}{uint32(0xffffffff), uint64(0xffffffff), int64(0xffffffff), float64(0xffffffff), complex128(0xffffffff)},
		},
		{
			name:  "-0x80000000: min usable by int16",
			items: []interface{}{int32(-0x80000000), int64(-0x80000000), float64(-0x80000000), complex128(-0x80000000)},
		},
		{
			name:  "(1<<53)=9007199254740992: max integer without loss in float64",
			items: []interface{}{uint64((1 << 53)), int64((1 << 53)), float64((1 << 53)), complex128((1 << 53))},
		},
		{
			name:  "(1<<53)+1=9007199254740993: one more than max integer without loss in float64",
			items: []interface{}{uint64((1 << 53) + 1), int64((1 << 53) + 1)},
		},
		{
			name:  "-(1<<53)=-9007199254740992: min integer without loss in float64",
			items: []interface{}{int64(-(1 << 53)), float64(-(1 << 53)), complex128(-(1 << 53))},
		},
		{
			name:  "-(1<<53)-1=-9007199254740993: one less than min integer without loss in float64",
			items: []interface{}{int64(-(1 << 53) - 1)},
		},
		{
			name:  "0x7fffffffffffffff: max usable by both uint64 and int64",
			items: []interface{}{uint64(0x7fffffffffffffff), int64(0x7fffffffffffffff)},
		},
		{
			name:  "0xffffffffffffffff: max usable by uint64",
			items: []interface{}{uint64(0xffffffffffffffff)},
		},
		{
			name:  "-0x8000000000000000: min usable by int64",
			items: []interface{}{int64(-0x80000000000000)},
		},
		{
			name:  "1.1e37: large floating point out of the range of uint64/int64",
			items: []interface{}{float32(1.1e37), float64(1.1e37), complex64(1.1e37), complex128(1.1e37)},
		},
		{
			name:  "2+1i: complex number with non-zero imaginary part",
			items: []interface{}{complex64(2 + 1i), complex128(2 + 1i)},
		},
	}

	for _, testCase := range validGroups {
		expected := map[reflect.Kind]interface{}{}
		for _, item := range testCase.items {
			expected[reflect.TypeOf(item).Kind()] = item
		}
		for _, item := range testCase.items {
			itemKind := reflect.TypeOf(item).Kind()
			if !sourceKinds[itemKind] {
				continue
			}
			for _, targetKind := range targetKinds {
				// At least one of the values is not float / complex.
				// Look up the expected value and check for it.
				for _, function := range functions {
					inKind := function.Type().In(0).Kind()
					outKind := function.Type().Out(0).Kind()
					if inKind == itemKind && outKind == targetKind {
						if floatOrComplexKinds[itemKind] && floatOrComplexKinds[targetKind] {
							// There are special rules if both are float / complex.
							if nonZeroImag(item) && floatKinds[targetKind] {
								// Expect failure
								_, err := call(function, item)
								if err == nil {
									t.Errorf("%s: expected error when converting %v %v to %v, but got none", testCase.name, itemKind, item, targetKind)
								}
							} else {
								// Expect success
								out, err := call(function, item)
								if err != nil {
									t.Errorf("%s: unexpected error converting %v %v to %v: %v", testCase.name, itemKind, item, targetKind, err)
								} else {
									if err != nil {
										t.Errorf("%s: unexpected error converting %v %v to %v: %v", testCase.name, itemKind, item, targetKind, err)
									} else {
										expectedVal, isExpected := expected[targetKind]
										if isExpected {
											if got, want := out, expectedVal; !reflect.DeepEqual(got, want) {
												t.Errorf("%s: when converting %v %v to %v got %v, want %v", testCase.name, itemKind, item, targetKind, got, want)
											}
										} else {
											// no information on expected float value, ignore.
										}
									}
								}
							}
						} else {
							out, err := call(function, item)
							expectedVal, isExpected := expected[targetKind]
							if isExpected {
								if err != nil {
									t.Errorf("%s: unexpected error converting %v %v to %v: %v", testCase.name, itemKind, item, targetKind, err)
								} else {
									if got, want := out, expectedVal; !reflect.DeepEqual(got, want) {
										t.Errorf("%s: when converting %v %v to %v got %v, want %v", testCase.name, itemKind, item, targetKind, got, want)
									}
								}
							} else {
								if err == nil {
									t.Errorf("%s: expected error when converting %v %v to %v, but got none", testCase.name, itemKind, item, targetKind)
								}
							}
						}
					}
				}
			}
		}
	}
}

func nonZeroImag(x interface{}) bool {
	switch c := x.(type) {
	case complex64:
		return imag(c) != 0
	case complex128:
		return imag(c) != 0
	default:
		return false
	}
}

func call(function reflect.Value, item interface{}) (interface{}, error) {
	out := function.Call([]reflect.Value{reflect.ValueOf(item)})
	var err error
	if !out[1].IsNil() {
		err = out[1].Interface().(error)
	}
	return out[0].Interface(), err
}
