// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"bytes"
	"io"
	"testing"

	"v.io/v23/vom/testdata/data81"
)

func TestRawValueDecodeEncode(t *testing.T) {
	for _, test := range data81.Tests {
		// Interleaved
		rv := RawValue{}
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
		enc := NewVersionedEncoder(Version81, &out)
		if err := enc.Encode(&rv); err != nil {
			t.Errorf("unexpected error encoding raw value %v in test %s: %v", rv, test.Name, err)
			continue
		}
		if !bytes.Equal(out.Bytes(), hex2Bin(t, test.Hex)) {
			t.Errorf("got bytes: %x but expected %s", out.Bytes(), test.Hex)
		}

		// Split type and value stream.
		rv = RawValue{}
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
		typeEnc := NewVersionedTypeEncoder(Version81, &typeOut)
		enc = NewVersionedEncoderWithTypeEncoder(Version81, &out, typeEnc)
		if err := enc.Encode(&rv); err != nil {
			t.Errorf("unexpected error encoding raw value %v in test %s: %v", rv, test.Name, err)
			continue
		}
		expectedType := test.HexVersion + test.HexType
		if expectedType == "81" {
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
