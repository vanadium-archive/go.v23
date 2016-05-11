// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build newvdltests

package vom

import (
	"fmt"
	"reflect"
	"strings"
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
		testDecodeVDLNew(t, name, binversion+bintype+binvalue, vdlValue)
		name = test.Name + " [vdl.Value] (with TypeDecoder)"
		testDecodeVDLWithTypeDecoderNew(t, name, binversion, bintype, binvalue, vdlValue)
		name = test.Name + " [vdl.Any]"
		testDecodeVDLNew(t, name, binversion+bintype+binvalue, vdl.AnyValue(vdlValue))
		name = test.Name + " [vdl.Any] (with TypeDecoder)"
		testDecodeVDLWithTypeDecoderNew(t, name, binversion, bintype, binvalue, vdl.AnyValue(vdlValue))

		// Convert in
		name = test.Name + " [go value]"
		testDecodeGoNew(t, name, binversion+bintype+binvalue, reflect.TypeOf(test.Value), test.Value)
		name = test.Name + " [go value] (with TypeDecoder)"
		testDecodeGoWithTypeDecoderNew(t, name, binversion, bintype, binvalue, reflect.TypeOf(test.Value), test.Value)

		name = test.Name + " [go interface]"
		testDecodeGoNew(t, name, binversion+bintype+binvalue, reflect.TypeOf((*interface{})(nil)).Elem(), test.Value)
		name = test.Name + " [go interface] (with TypeDecoder)"
		testDecodeGoWithTypeDecoderNew(t, name, binversion, bintype, binvalue, reflect.TypeOf((*interface{})(nil)).Elem(), test.Value)
	}
}

func testDecodeVDLNew(t *testing.T, name, bin string, value *vdl.Value) {
	for _, mode := range allReadModes {
		head := fmt.Sprintf("%s (%s)", name, mode)
		decoder := NewXDecoder(mode.testReader(strings.NewReader(bin)))
		if value == nil {
			value = vdl.ZeroValue(vdl.AnyType)
		}
		got := vdl.ZeroValue(value.Type())
		if err := decoder.Decode(got); err != nil {
			t.Errorf("%s: Decode failed: %v", head, err)
			return
		}
		if want := value; !vdl.EqualValue(got, want) {
			t.Errorf("%s: Decode mismatch\nGOT  %v\nWANT %v", head, got, want)
			return
		}
	}
	// Test single-shot vom.Decode twice, to ensure we test the cache hit case.
	testDecodeVDLSingleShotNew(t, name, bin, value)
	testDecodeVDLSingleShotNew(t, name, bin, value)
}

func testDecodeVDLSingleShotNew(t *testing.T, name, bin string, value *vdl.Value) {
	// Test the single-shot vom.Decode.
	head := fmt.Sprintf("%s (single-shot)", name)
	got := vdl.ZeroValue(value.Type())
	if err := XDecode([]byte(bin), got); err != nil {
		t.Errorf("%s: Decode failed: %v", head, err)
		return
	}
	if want := value; !vdl.EqualValue(got, want) {
		t.Errorf("%s: Decode mismatch\nGOT  %v\nWANT %v", head, got, want)
		return
	}
}

func testDecodeVDLWithTypeDecoderNew(t *testing.T, name, binversion, bintype, binvalue string, value *vdl.Value) {
	for _, mode := range allReadModes {
		head := fmt.Sprintf("%s (%s)", name, mode)
		typedec := NewTypeDecoder(mode.testReader(strings.NewReader(binversion + bintype)))
		typedec.Start()
		decoder := NewXDecoderWithTypeDecoder(mode.testReader(strings.NewReader(binversion+binvalue)), typedec)
		if value == nil {
			value = vdl.ZeroValue(vdl.AnyType)
		}
		got := vdl.ZeroValue(value.Type())
		if err := decoder.Decode(got); err != nil {
			t.Errorf("%s: Decode failed: %v", head, err)
			return
		}
		if want := value; !vdl.EqualValue(got, want) {
			t.Errorf("%s: Decode mismatch\nGOT  %v\nWANT %v", head, got, want)
			return
		}
		typedec.Stop()
	}
}

func testDecodeGoNew(t *testing.T, name, bin string, rt reflect.Type, want interface{}) {
	for _, mode := range allReadModes {
		head := fmt.Sprintf("%s (%s)", name, mode)
		decoder := NewXDecoder(mode.testReader(strings.NewReader(bin)))
		var got interface{}
		if rt != nil {
			got = reflect.New(rt).Elem().Interface()
		}
		if err := decoder.Decode(&got); err != nil {
			t.Errorf("%s: Decode failed: %v", head, err)
			return
		}
		if !vdl.DeepEqual(got, want) {
			t.Errorf("%s: Decode mismatch\nGOT  %T %+v\nWANT %T %+v", head, got, got, want, want)
			return
		}
	}
	// Test single-shot vom.Decode twice, to ensure we test the cache hit case.
	testDecodeGoSingleShotNew(t, name, bin, rt, want)
	testDecodeGoSingleShotNew(t, name, bin, rt, want)
}

func testDecodeGoSingleShotNew(t *testing.T, name, bin string, rt reflect.Type, want interface{}) {
	head := fmt.Sprintf("%s (single-shot)", name)
	var got interface{}
	if rt != nil {
		got = reflect.New(rt).Elem().Interface()
	}
	if err := XDecode([]byte(bin), &got); err != nil {
		t.Errorf("%s: Decode failed: %v", head, err)
		return
	}
	if !vdl.DeepEqual(got, want) {
		t.Errorf("%s: Decode mismatch\nGOT  %T %+v\nWANT %T %+v", head, got, got, want, want)
		return
	}
}

func testDecodeGoWithTypeDecoderNew(t *testing.T, name, binversion, bintype, binvalue string, rt reflect.Type, want interface{}) {
	for _, mode := range allReadModes {
		head := fmt.Sprintf("%s (%s)", name, mode)
		typedec := NewTypeDecoder(mode.testReader(strings.NewReader(binversion + bintype)))
		typedec.Start()
		decoder := NewXDecoderWithTypeDecoder(mode.testReader(strings.NewReader(binversion+binvalue)), typedec)
		var got interface{}
		if rt != nil {
			got = reflect.New(rt).Elem().Interface()
		}
		if err := decoder.Decode(&got); err != nil {
			t.Errorf("%s: Decode failed: %v", head, err)
			return
		}
		if !vdl.DeepEqual(got, want) {
			t.Errorf("%s: Decode mismatch\nGOT  %T %+v\nWANT %T %+v", head, got, got, want, want)
			return
		}
		typedec.Stop()
	}
}
