// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdlconv_test

import (
	"reflect"

	"v.io/v23/vdl/vdlconv"
)

var functions = []reflect.Value{
	reflect.ValueOf(vdlconv.Uint64ToUint8),
	reflect.ValueOf(vdlconv.Uint64ToUint16),
	reflect.ValueOf(vdlconv.Uint64ToUint32),
	reflect.ValueOf(vdlconv.Uint64ToInt8),
	reflect.ValueOf(vdlconv.Uint64ToInt16),
	reflect.ValueOf(vdlconv.Uint64ToInt32),
	reflect.ValueOf(vdlconv.Uint64ToInt64),
	reflect.ValueOf(vdlconv.Uint64ToFloat32),
	reflect.ValueOf(vdlconv.Uint64ToFloat64),
	reflect.ValueOf(vdlconv.Uint64ToComplex64),
	reflect.ValueOf(vdlconv.Uint64ToComplex128),
	reflect.ValueOf(vdlconv.Int64ToUint8),
	reflect.ValueOf(vdlconv.Int64ToUint16),
	reflect.ValueOf(vdlconv.Int64ToUint32),
	reflect.ValueOf(vdlconv.Int64ToUint64),
	reflect.ValueOf(vdlconv.Int64ToInt8),
	reflect.ValueOf(vdlconv.Int64ToInt16),
	reflect.ValueOf(vdlconv.Int64ToInt32),
	reflect.ValueOf(vdlconv.Int64ToFloat32),
	reflect.ValueOf(vdlconv.Int64ToFloat64),
	reflect.ValueOf(vdlconv.Int64ToComplex64),
	reflect.ValueOf(vdlconv.Int64ToComplex128),
	reflect.ValueOf(vdlconv.Float64ToUint8),
	reflect.ValueOf(vdlconv.Float64ToUint16),
	reflect.ValueOf(vdlconv.Float64ToUint32),
	reflect.ValueOf(vdlconv.Float64ToUint64),
	reflect.ValueOf(vdlconv.Float64ToInt8),
	reflect.ValueOf(vdlconv.Float64ToInt16),
	reflect.ValueOf(vdlconv.Float64ToInt32),
	reflect.ValueOf(vdlconv.Float64ToInt64),
	reflect.ValueOf(vdlconv.Float64ToFloat32),
	reflect.ValueOf(vdlconv.Float64ToComplex64),
	reflect.ValueOf(vdlconv.Float64ToComplex128),
	reflect.ValueOf(vdlconv.Complex128ToUint8),
	reflect.ValueOf(vdlconv.Complex128ToUint16),
	reflect.ValueOf(vdlconv.Complex128ToUint32),
	reflect.ValueOf(vdlconv.Complex128ToUint64),
	reflect.ValueOf(vdlconv.Complex128ToInt8),
	reflect.ValueOf(vdlconv.Complex128ToInt16),
	reflect.ValueOf(vdlconv.Complex128ToInt32),
	reflect.ValueOf(vdlconv.Complex128ToInt64),
	reflect.ValueOf(vdlconv.Complex128ToFloat32),
	reflect.ValueOf(vdlconv.Complex128ToFloat64),
	reflect.ValueOf(vdlconv.Complex128ToComplex64),
}
