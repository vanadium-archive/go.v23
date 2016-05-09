// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build newvdltests

package vom

import (
	"reflect"
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vom/vomtest"
)

func TestDecoderNew(t *testing.T) {
	for _, test := range vomtest.Data() {
		// Decode hex patterns into binary data.
		binversion := string([]byte{test.Version})
		bintype, err := binFromHexPat(test.HexType)
		if err != nil {
			t.Errorf("%s: couldn't convert to binary from hextype: %q", test.Name, test.HexType)
			continue
		}
		binvalue, err := binFromHexPat(test.HexValue)
		if err != nil {
			t.Errorf("%s: couldn't convert to binary from hexvalue: %q", test.Name, test.HexValue)
			continue
		}

		vdlValue := vdl.ValueOf(test.Value)
		name := test.Name + " [vdl.Value]"
		testDecodeVDL(t, name, binversion+bintype+binvalue, vdlValue)
		name = test.Name + " [vdl.Value] (with TypeDecoder)"
		testDecodeVDLWithTypeDecoder(t, name, binversion, bintype, binvalue, vdlValue)
		name = test.Name + " [vdl.Any]"
		testDecodeVDL(t, name, binversion+bintype+binvalue, vdl.AnyValue(vdlValue))
		name = test.Name + " [vdl.Any] (with TypeDecoder)"
		testDecodeVDLWithTypeDecoder(t, name, binversion, bintype, binvalue, vdl.AnyValue(vdlValue))

		// Convert in
		name = test.Name + " [go value]"
		testDecodeGo(t, name, binversion+bintype+binvalue, reflect.TypeOf(test.Value), test.Value)
		name = test.Name + " [go value] (with TypeDecoder)"
		testDecodeGoWithTypeDecoder(t, name, binversion, bintype, binvalue, reflect.TypeOf(test.Value), test.Value)

		name = test.Name + " [go interface]"
		testDecodeGo(t, name, binversion+bintype+binvalue, reflect.TypeOf((*interface{})(nil)).Elem(), test.Value)
		name = test.Name + " [go interface] (with TypeDecoder)"
		testDecodeGoWithTypeDecoder(t, name, binversion, bintype, binvalue, reflect.TypeOf((*interface{})(nil)).Elem(), test.Value)
	}
}
