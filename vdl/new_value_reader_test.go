// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build newvdltests

package vdl_test

import (
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vdl/vdltest"
)

func TestValueReadNew(t *testing.T) {
	for _, entry := range vdltest.ToEntryValues(vdltest.AllPass()) {
		out := vdl.ZeroValue(entry.Target.Type())
		if err := out.VDLRead(entry.Source.Decoder()); err != nil {
			t.Errorf("%s: error in ValueRead: %v", entry.Name(), err)
			continue
		}
		if !vdl.EqualValue(entry.Target, out) {
			t.Errorf("%s: got %v, want %v", entry.Name(), out, entry.Target)
		}
	}
}
