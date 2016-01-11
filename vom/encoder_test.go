// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"bytes"
	"fmt"
	"testing"

	"v.io/v23/vom/testdata/data80"
	"v.io/v23/vom/testdata/data81"
	"v.io/v23/vom/testdata/data82"
	"v.io/v23/vom/testdata/types"
)

func TestEncoder80(t *testing.T) {
	testEncoder(t, Version80, data80.Tests)
}

func TestEncoder81(t *testing.T) {
	testEncoder(t, Version81, data81.Tests)
}

func TestEncoder82(t *testing.T) {
	testEncoder(t, Version82, data82.Tests)
}

func testEncoder(t *testing.T, version Version, tests []types.TestCase) {
	for _, test := range tests {
		name := test.Name + " [vdl.Value]"
		testEncode(t, version, name, test.Value, test.HexVersion+test.HexType+test.HexValue)
		name = test.Name + " [vdl.Value] (with TypeEncoder)"
		testEncodeWithTypeEncoder(t, version, name, test.Value, test.HexVersion, test.HexType, test.HexValue)

		// Convert into Go value for the rest of our tests.
		goValue, err := toGoValue(test.Value)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}

		name = test.Name + " [go value]"
		testEncode(t, version, name, goValue, test.HexVersion+test.HexType+test.HexValue)
		name = test.Name + " [go value] (with TypeEncoder)"
		testEncodeWithTypeEncoder(t, version, name, goValue, test.HexVersion, test.HexType, test.HexValue)
	}
}

func testEncode(t *testing.T, version Version, name string, value interface{}, hex string) {
	for _, singleShot := range []bool{false, true} {
		var bin []byte
		if !singleShot {
			var buf bytes.Buffer
			encoder := NewVersionedEncoder(version, &buf)
			if err := encoder.Encode(value); err != nil {
				t.Errorf("%s: Encode(%#v) failed: %v", name, value, err)
				return
			}
			bin = buf.Bytes()
		} else {
			name += " (single-shot)"
			var err error
			bin, err = VersionedEncode(version, value)
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

func testEncodeWithTypeEncoder(t *testing.T, version Version, name string, value interface{}, hexversion, hextype, hexvalue string) {
	var buf, typebuf bytes.Buffer
	typeenc := NewVersionedTypeEncoder(version, &typebuf)
	encoder := NewVersionedEncoderWithTypeEncoder(version, &buf, typeenc)
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
