// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl_test

import (
	"reflect"
	"testing"

	"v.io/v23/vdl"
)

type NamedType struct {
	X int16
}

// BenchmarkRepeatedVdlTypeOf determines the time for a repeated
// vdl.TypeOf, i.e. skipping the initial vdl.Type of that builds
// the type.
func BenchmarkRepeatedVdlTypeOf(b *testing.B) {
	val := &NamedType{}
	vdl.TypeOf(val)

	for i := 0; i < b.N; i++ {
		vdl.TypeOf(val)
	}
}

// BenchmarkRepeatedReflectTypeOf is the reflect analog of
// BenchmarkRepeatedVdlTypeOf, for comparison purposes.
func BenchmarkRepeatedReflectTypeOf(b *testing.B) {
	reflect.TypeOf(&NamedType{})

	for i := 0; i < b.N; i++ {
		reflect.TypeOf(&NamedType{})
	}
}
