// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl_test

import (
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vom/testdata/data81"
)

func TestValueRead(t *testing.T) {
	for _, test := range data81.Tests {
		out := vdl.ZeroValue(test.Value.Type())
		if err := out.VDLRead(test.Value.Decoder()); err != nil {
			t.Errorf("%s: error in ValueRead: %v", test.Name, err)
			continue
		}
		if !vdl.EqualValue(test.Value, out) {
			t.Errorf("%s: got %v, want %v", test.Name, out, test.Value)
		}
	}
}
