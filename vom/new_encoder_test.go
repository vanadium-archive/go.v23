// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build newvdltests

package vom

import (
	"fmt"
	"testing"
	"v.io/v23/vdl"
	"v.io/v23/vom/vomtest"
)

func TestEncoderNew(t *testing.T) {
	for _, test := range vomtest.Data() {
		version := Version(test.Version)
		hexVersion := fmt.Sprintf("%x", test.Version)
		vdlValue := vdl.ValueOf(test.Value)
		name := test.Name + " [vdl.Value]"
		testEncode(t, version, name, vdlValue, hexVersion+test.HexType+test.HexValue)
		name = test.Name + " [vdl.Value] (with TypeEncoder)"
		testEncodeWithTypeEncoder(t, version, name, vdlValue, hexVersion, test.HexType, test.HexValue)

		name = test.Name + " [go value]"
		testEncode(t, version, name, test.Value, hexVersion+test.HexType+test.HexValue)
		name = test.Name + " [go value] (with TypeEncoder)"
		testEncodeWithTypeEncoder(t, version, name, test.Value, hexVersion, test.HexType, test.HexValue)
	}
}
