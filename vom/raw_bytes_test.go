// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"bytes"
	"io"
	"testing"

	"v.io/v23/vom/testdata/data81"
	"v.io/v23/vom/testdata/data82"
	"v.io/v23/vom/testdata/types"
	"v.io/v23/vdl"
)

func TestRawBytesDecodeEncode(t *testing.T) {
	versions := []struct {
		Version Version
		Tests   []types.TestCase
	}{
		{Version81, data81.Tests},
		{Version82, data82.Tests},
	}
	for _, testVersion := range versions {
		for _, test := range testVersion.Tests {
			// Interleaved
			rv := RawBytes{}
			interleavedReader := bytes.NewReader(hex2Bin(t, test.Hex))
			if err := NewDecoder(interleavedReader).Decode(&rv); err != nil {
				t.Errorf("unexpected error decoding %s: %v", test.Name, err)
				continue
			}
			if _, err := interleavedReader.ReadByte(); err != io.EOF {
				t.Errorf("expected EOF, but got %v", err)
				continue
			}

			var out bytes.Buffer
			enc := NewVersionedEncoder(testVersion.Version, &out)
			if err := enc.Encode(&rv); err != nil {
				t.Errorf("unexpected error encoding raw value %v in test %s: %v", rv, test.Name, err)
				continue
			}
			if !bytes.Equal(out.Bytes(), hex2Bin(t, test.Hex)) {
				t.Errorf("got bytes: %x but expected %s", out.Bytes(), test.Hex)
			}

			// Split type and value stream.
			rv = RawBytes{}
			typeReader := bytes.NewReader(hex2Bin(t, test.HexVersion+test.HexType))
			typeDec := NewTypeDecoder(typeReader)
			typeDec.Start()
			defer typeDec.Stop()
			valueReader := bytes.NewReader(hex2Bin(t, test.HexVersion+test.HexValue))
			if err := NewDecoderWithTypeDecoder(valueReader, typeDec).Decode(&rv); err != nil {
				t.Errorf("unexpected error decoding %s: %v", test.Name, err)
				continue
			}
			if test.HexType != "" {
				// If HexType is empty, then the type stream will just have the version byte that won't be read, so ignore
				// that case.
				if _, err := typeReader.ReadByte(); err != io.EOF {
					t.Errorf("in type reader expected EOF, but got %v", err)
					continue
				}
			}
			if _, err := valueReader.ReadByte(); err != io.EOF {
				t.Errorf("in value reader expected EOF, but got %v", err)
				continue
			}

			out.Reset()
			var typeOut bytes.Buffer
			typeEnc := NewVersionedTypeEncoder(testVersion.Version, &typeOut)
			enc = NewVersionedEncoderWithTypeEncoder(testVersion.Version, &out, typeEnc)
			if err := enc.Encode(&rv); err != nil {
				t.Errorf("unexpected error encoding raw value %v in test %s: %v", rv, test.Name, err)
				continue
			}
			expectedType := test.HexVersion + test.HexType
			if expectedType == "81" || expectedType == "82" {
				expectedType = ""
			}
			if !bytes.Equal(typeOut.Bytes(), hex2Bin(t, expectedType)) {
				t.Errorf("got type bytes: %x but expected %s", typeOut.Bytes(), expectedType)
			}
			if !bytes.Equal(out.Bytes(), hex2Bin(t, test.HexVersion+test.HexValue)) {
				t.Errorf("got value bytes: %x but expected %s", out.Bytes(), test.HexVersion+test.HexValue)
			}
		}
	}
}

func TestRawBytesToFromValue(t *testing.T) {
	versions := []struct {
		Version Version
		Tests   []types.TestCase
	}{
		{Version81, data81.Tests},
		{Version82, data82.Tests},
	}
	for _, testVersion := range versions {
		for _, test := range testVersion.Tests {
			rb, err := RawBytesFromValue(test.Value)
			if err != nil {
				t.Fatalf("%v %s: error in RawBytesFromValue %v", testVersion.Version, test.Name, err)
			}
			var vv *vdl.Value
			if err := rb.ToValue(&vv); err != nil {
				t.Fatalf("%v %s: error in rb.ToValue %v", testVersion.Version, test.Name, err)
			}
			if (test.Name == "any(nil)") {
				// Skip any(nil)
				// TODO(bprosnitz) any(nil) results in two different nil representations. This shouldn't be the case.
				continue
			}
			if got, want := vv, test.Value; !vdl.EqualValue(got, want) {
				t.Errorf("%v %s: error in converting to and from raw value. got %v, but want %v", testVersion.Version,
					test.Name, got, want)
			}
		}
	}
}