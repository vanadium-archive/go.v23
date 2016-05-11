// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build newvdltests

package vom

import (
	"bytes"
	"fmt"
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vom/vomtest"
)

func TestXEncoder(t *testing.T) {
	for _, test := range vomtest.Data() {
		version := Version(test.Version)
		hexVersion := fmt.Sprintf("%x", test.Version)
		vdlValue := vdl.ValueOf(test.Value)
		name := test.Name + " [vdl.Value]"
		testXEncode(t, version, name, vdlValue, hexVersion+test.HexType+test.HexValue)
		name = test.Name + " [vdl.Value] (with TypeEncoder)"
		testXEncodeWithTypeEncoder(t, version, name, vdlValue, hexVersion, test.HexType, test.HexValue)

		name = test.Name + " [go value]"
		testXEncode(t, version, name, test.Value, hexVersion+test.HexType+test.HexValue)
		name = test.Name + " [go value] (with TypeEncoder)"
		testXEncodeWithTypeEncoder(t, version, name, test.Value, hexVersion, test.HexType, test.HexValue)
	}
}

func testXEncode(t *testing.T, version Version, name string, value interface{}, hex string) {
	for _, singleShot := range []bool{false, true} {
		var bin []byte
		if !singleShot {
			var buf bytes.Buffer
			encoder := NewVersionedXEncoder(version, &buf)
			if err := encoder.Encode(value); err != nil {
				t.Errorf("%s: Encode(%#v) failed: %v", name, value, err)
				return
			}
			bin = buf.Bytes()
		} else {
			name += " (single-shot)"
			var err error
			bin, err = VersionedXEncode(version, value)
			if err != nil {
				t.Errorf("%s: Encode(%#v) failed: %v", name, value, err)
				return
			}
		}
		got, want := fmt.Sprintf("%x", bin), hex
		match, err := matchHexPat(got, want)
		if err != nil {
			t.Error(err)
		}
		if !match {
			t.Errorf("%s: Encode(%#v)\n GOT %s\nWANT %s", name, value, got, want)
		}
	}
}

func testXEncodeWithTypeEncoder(t *testing.T, version Version, name string, value interface{}, hexversion, hextype, hexvalue string) {
	var buf, typebuf bytes.Buffer
	typeenc := NewVersionedTypeEncoder(version, &typebuf)
	encoder := NewVersionedXEncoderWithTypeEncoder(version, &buf, typeenc)
	if err := encoder.Encode(value); err != nil {
		t.Errorf("%s: Encode(%#v) failed: %v", name, value, err)
		return
	}
	got, want := fmt.Sprintf("%x", typebuf.Bytes()), hexversion+hextype
	match, err := matchHexPat(got, want)
	if err != nil {
		t.Error(err)
	}
	if !match && len(hextype) > 0 {
		t.Errorf("%s: EncodeType(%#v)\nGOT %s\nWANT %s", name, value, got, want)
	}
	got, want = fmt.Sprintf("%x", buf.Bytes()), hexversion+hexvalue
	match, err = matchHexPat(got, want)
	if err != nil {
		t.Error(err)
	}
	if !match {
		t.Errorf("%s: Encode(%#v)\nGOT %s\nWANT %s", name, value, got, want)
	}
}
