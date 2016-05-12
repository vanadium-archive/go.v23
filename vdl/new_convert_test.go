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

func TestConvertNew(t *testing.T) {
	for _, entry := range vdltest.AllPass() {
		rvTargetPtr := reflect.New(entry.Target.Type())
		if err := vdl.Convert(rvTargetPtr.Interface(), entry.Source.Interface()); err != nil {
			t.Errorf("%s: Convert failed: %v", entry.Name(), err)
		}
		if got, want := rvTargetPtr.Elem(), entry.Target; !vdl.DeepEqualReflect(got, want) {
			t.Errorf("%s\nGOT  %v\nWANT %v", entry.Name(), got, want)
		}
	}
}

func TestConvertFailNew(t *testing.T) {
	for _, entry := range vdltest.AllFail() {
		rvTargetPtr := reflect.New(entry.Target.Type())
		if err := vdl.Convert(rvTargetPtr.Interface(), entry.Source.Interface()); err == nil {
			t.Errorf("%s: Convert passed, wanted failure", entry.Name())
		}
	}
}
