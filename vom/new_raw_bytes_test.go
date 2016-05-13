// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build newvdltests

package vom_test

import (
	"bytes"
	"io"
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vom"
	"v.io/v23/vom/vomtest"
)

// TODO(toddw): Clean up these tests.

func TestXRawBytesDecodeEncode(t *testing.T) {
	for _, test := range vomtest.AllPass() {
		// Interleaved
		rb := vom.RawBytes{}
		interleavedReader := bytes.NewReader(test.Bytes())
		if err := vom.NewXDecoder(interleavedReader).Decode(&rb); err != nil {
			t.Errorf("%s: decode failed: %v", test.Name(), err)
			continue
		}
		if _, err := interleavedReader.ReadByte(); err != io.EOF {
			t.Errorf("%s: expected EOF, but got %v", test.Name(), err)
			continue
		}

		var out bytes.Buffer
		enc := vom.NewVersionedXEncoder(test.Version, &out)
		if err := enc.Encode(&rb); err != nil {
			t.Errorf("%s: encode failed: %v\nRawBytes: %v", test.Name(), err, rb)
			continue
		}
		if got, want := out.Bytes(), test.Bytes(); !bytes.Equal(got, want) {
			t.Errorf("%s\nGOT  %x\nWANT %x", test.Name(), got, want)
		}

		// Split type and value stream.
		rb = vom.RawBytes{}
		typeReader := bytes.NewReader(test.TypeBytes())
		typeDec := vom.NewTypeDecoder(typeReader)
		typeDec.Start()
		defer typeDec.Stop()
		valueReader := bytes.NewReader(test.ValueBytes())
		if err := vom.NewXDecoderWithTypeDecoder(valueReader, typeDec).Decode(&rb); err != nil {
			t.Errorf("%s: decode failed: %v", test.Name(), err)
			continue
		}
		if _, err := typeReader.ReadByte(); err != io.EOF {
			t.Errorf("%s: type reader got %v, want EOF", test.Name(), err)
			continue
		}
		if _, err := valueReader.ReadByte(); err != io.EOF {
			t.Errorf("%s: value reader got %v, want EOF", test.Name(), err)
			continue
		}

		out.Reset()
		var typeOut bytes.Buffer
		typeEnc := vom.NewVersionedTypeEncoder(test.Version, &typeOut)
		enc = vom.NewVersionedXEncoderWithTypeEncoder(test.Version, &out, typeEnc)
		if err := enc.Encode(&rb); err != nil {
			t.Errorf("%s: encode failed: %v\nRawBytes: %v", test.Name(), err, rb)
			continue
		}
		if got, want := typeOut.Bytes(), test.TypeBytes(); !bytes.Equal(got, want) {
			t.Errorf("%s: type bytes\nGOT  %x\nWANT %x", test.Name(), got, want)
		}
		if got, want := out.Bytes(), test.ValueBytes(); !bytes.Equal(got, want) {
			t.Errorf("%s: value bytes\nGOT  %x\nWANT %x", test.Name(), got, want)
		}
	}
}

func TestXRawBytesToFromValue(t *testing.T) {
	for _, test := range vomtest.AllPass() {
		rb, err := vom.XRawBytesFromValue(test.Value.Interface())
		if err != nil {
			t.Errorf("%v %s: XRawBytesFromValue failed: %v", test.Version, test.Name(), err)
			continue
		}
		var vv *vdl.Value
		if err := rb.ToValue(&vv); err != nil {
			t.Errorf("%v %s: rb.ToValue failed: %v", test.Version, test.Name(), err)
			continue
		}
		if got, want := vv, vdl.ValueOf(test.Value.Interface()); !vdl.EqualValue(got, want) {
			t.Errorf("%v %s\nGOT  %v\nWANT %v", test.Version, test.Name(), got, want)
		}
	}
}

func TestXRawBytesDecoder(t *testing.T) {
	for _, test := range vomtest.AllPass() {
		tt, err := vdl.TypeFromReflect(test.Value.Type())
		if err != nil {
			t.Errorf("%s: TypeFromReflect failed: %v", test.Name(), err)
			continue
		}
		in, err := vom.XRawBytesFromValue(test.Value.Interface())
		if err != nil {
			t.Errorf("%s: XRawBytesFromValue failed: %v", test.Name(), err)
			continue
		}
		out := vdl.ZeroValue(tt)
		if err := out.VDLRead(in.Decoder()); err != nil {
			t.Errorf("%s: VDLRead failed: %v", test.Name(), err)
			continue
		}
		if got, want := out, vdl.ValueOf(test.Value.Interface()); !vdl.EqualValue(got, want) {
			t.Errorf("%s\nGOT  %v\nWANT %v", test.Name(), got, want)
		}
	}
}

func TestXRawBytesWriter(t *testing.T) {
	for _, test := range vomtest.AllPass() {
		var buf bytes.Buffer
		enc := vom.NewXEncoder(&buf)
		rb, err := vom.XRawBytesFromValue(test.Value.Interface())
		if err != nil {
			t.Errorf("%s: XRawBytesFromValue failed: %v", test.Name(), err)
			continue
		}
		if err := rb.VDLWrite(enc.Encoder()); err != nil {
			t.Errorf("%s: VDLWrite failed: %v", test.Name(), err)
			continue
		}
		if got, want := buf.Bytes(), test.Bytes(); !bytes.Equal(got, want) {
			t.Errorf("%s\nGOT  %x\nWANT %x", test.Name(), got, want)
		}
	}
}
