// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build newvdltests

package vdl_test

import (
	"reflect"
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vdl/vdltest"
)

func zeroTargetPtr(e vdltest.Entry) interface{} {
	if e.Target != nil {
		return reflect.New(reflect.TypeOf(e.Target)).Interface()
	}
	var x interface{}
	return &x
}

func TestConvertNew(t *testing.T) {
	for _, entry := range vdltest.AllPass() {
		targetPtr := zeroTargetPtr(entry)
		if err := vdl.Convert(targetPtr, entry.Source); err != nil {
			t.Errorf("%s: error %v", entry.Name(), err)
		}
		target := reflect.ValueOf(targetPtr).Elem().Interface()
		if !vdl.DeepEqual(target, entry.Target) {
			t.Errorf("%s: got %T %#v, want %T %#v", entry.Name(), target, target, entry.Target, entry.Target)
		}
	}
}

func TestConvertFailNew(t *testing.T) {
	for _, entry := range vdltest.AllFail() {
		targetPtr := zeroTargetPtr(entry)
		if err := vdl.Convert(targetPtr, entry.Source); err == nil {
			t.Errorf("%s: expected failure", entry.Name())
		}
	}
}
