// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build newvdltests

package vom

import (
	"bytes"
	"io"
	"testing"

	"fmt"
	"v.io/v23/vdl"
	"v.io/v23/vom/vomtest"
)

func TestRawBytesDecodeEncodeNew(t *testing.T) {
	for _, test := range vomtest.Data() {
		version := Version(test.Version)
		hexVersion := fmt.Sprintf("%x", test.Version)
		// Interleaved
		rb := RawBytes{}
		interleavedReader := bytes.NewReader(hex2Bin(t, test.Hex))
		if err := NewDecoder(interleavedReader).Decode(&rb); err != nil {
			t.Errorf("unexpected error decoding %s: %v", test.Name, err)
			continue
		}
		if _, err := interleavedReader.ReadByte(); err != io.EOF {
			t.Errorf("expected EOF, but got %v", err)
			continue
		}

		var out bytes.Buffer
		enc := NewVersionedEncoder(version, &out)
		if err := enc.Encode(&rb); err != nil {
			t.Errorf("unexpected error encoding raw bytes %v in test %s: %v", rb, test.Name, err)
			continue
		}
		if !bytes.Equal(out.Bytes(), hex2Bin(t, test.Hex)) {
			t.Errorf("got bytes: %x but expected %s", out.Bytes(), test.Hex)
		}

		// Split type and value stream.
		rb = RawBytes{}
		typeReader := bytes.NewReader(hex2Bin(t, hexVersion+test.HexType))
		typeDec := NewTypeDecoder(typeReader)
		typeDec.Start()
		defer typeDec.Stop()
		valueReader := bytes.NewReader(hex2Bin(t, hexVersion+test.HexValue))
		if err := NewDecoderWithTypeDecoder(valueReader, typeDec).Decode(&rb); err != nil {
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
		typeEnc := NewVersionedTypeEncoder(version, &typeOut)
		enc = NewVersionedEncoderWithTypeEncoder(version, &out, typeEnc)
		if err := enc.Encode(&rb); err != nil {
			t.Errorf("unexpected error encoding raw value %v in test %s: %v", rb, test.Name, err)
			continue
		}
		expectedType := hexVersion + test.HexType
		if expectedType == "81" || expectedType == "82" {
			expectedType = ""
		}
		if !bytes.Equal(typeOut.Bytes(), hex2Bin(t, expectedType)) {
			t.Errorf("got type bytes: %x but expected %s", typeOut.Bytes(), expectedType)
		}
		if !bytes.Equal(out.Bytes(), hex2Bin(t, hexVersion+test.HexValue)) {
			t.Errorf("got value bytes: %x but expected %s", out.Bytes(), hexVersion+test.HexValue)
		}
	}
}

func TestRawBytesToFromValueNew(t *testing.T) {
	for _, test := range vomtest.Data() {
		version := Version(test.Version)
		rb, err := RawBytesFromValue(test.Value)
		if err != nil {
			t.Fatalf("%v %s: error in RawBytesFromValue %v", version, test.Name, err)
		}
		var vv *vdl.Value
		if err := rb.ToValue(&vv); err != nil {
			t.Fatalf("%v %s: error in rb.ToValue %v", version, test.Name, err)
		}
		if test.Name == "any(nil)" {
			// Skip any(nil)
			// TODO(bprosnitz) any(nil) results in two different nil representations. This shouldn't be the case.
			continue
		}
		if got, want := vv, vdl.ValueOf(test.Value); !vdl.EqualValue(got, want) {
			t.Errorf("%v %s: error in converting to and from raw value. got %v, but want %v", version,
				test.Name, got, want)
		}
	}
}

func TestRawBytesDecoderNew(t *testing.T) {
	for _, test := range vomtest.Data() {
		in := RawBytesOf(test.Value)
		out := vdl.ZeroValue(vdl.TypeOf(test.Value))
		if err := out.VDLRead(in.Decoder()); err != nil {
			t.Errorf("%s: error in ValueRead: %v", test.Name, err)
			continue
		}
		if !vdl.EqualValue(vdl.ValueOf(test.Value), out) {
			t.Errorf("%s: got %v, want %v", test.Name, out, test.Value)
		}
	}
}

func TestRawBytesWriterNew(t *testing.T) {
	for _, test := range vomtest.Data() {
		var buf bytes.Buffer
		enc := NewXEncoder(&buf)
		if err := RawBytesOf(test.Value).VDLWrite(enc.Encoder()); err != nil {
			t.Errorf("%s: error in transcode: %v", test.Name, err)
			continue
		}
		if got, want := buf.Bytes(), hex2Bin(t, test.Hex); !bytes.Equal(got, want) {
			t.Errorf("%s: got %x, want %x", test.Name, got, want)
		}
	}
}
