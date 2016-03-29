// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"reflect"
	"unsafe"
)

var (
	bitlenReflect = [...]uintptr{
		reflect.Uint8:   8,
		reflect.Uint16:  16,
		reflect.Uint32:  32,
		reflect.Uint64:  64,
		reflect.Uint:    8 * unsafe.Sizeof(uint(0)),
		reflect.Uintptr: 8 * unsafe.Sizeof(uintptr(0)),
		reflect.Int8:    8,
		reflect.Int16:   16,
		reflect.Int32:   32,
		reflect.Int64:   64,
		reflect.Int:     8 * unsafe.Sizeof(int(0)),
		reflect.Float32: 32,
		reflect.Float64: 64,
	}

	bitlenVDL = [...]uintptr{
		Byte:    8,
		Uint16:  16,
		Uint32:  32,
		Uint64:  64,
		Int8:    8,
		Int16:   16,
		Int32:   32,
		Int64:   64,
		Float32: 32,
		Float64: 64,
	}
)

// bitlen{R,V} enforce static type safety on kind.
func bitlenR(kind reflect.Kind) uintptr { return bitlenReflect[kind] }
func bitlenV(kind Kind) uintptr         { return bitlenVDL[kind] }

// isRTBytes returns true iff rt is an array or slice of bytes.
func isRTBytes(rt reflect.Type) bool {
	return (rt.Kind() == reflect.Array || rt.Kind() == reflect.Slice) && rt.Elem().Kind() == reflect.Uint8
}

// rtBytes extracts []byte from rv.  Assumes isRTBytes(rv.Type()) == true.
func rtBytes(rv reflect.Value) []byte {
	// Fastpath if the underlying type is []byte
	if rv.Kind() == reflect.Slice && rv.Type().Elem() == rtByte {
		return rv.Bytes()
	}
	// Slowpath copying bytes one by one.
	ret := make([]byte, rv.Len())
	for ix := 0; ix < rv.Len(); ix++ {
		ret[ix] = rv.Index(ix).Convert(rtByte).Interface().(byte)
	}
	return ret
}
