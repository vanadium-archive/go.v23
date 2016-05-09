// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build newvdltests

package vom

import (
	"bytes"
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vom/vomtest"
)

func TestTranscodeXDecoderToXEncoderNew(t *testing.T) {
	for _, test := range vomtest.Data() {
		inBytes := hex2Bin(t, test.Hex)
		var buf bytes.Buffer
		enc := NewVersionedXEncoder(Version(test.Version), &buf)
		dec := NewXDecoder(bytes.NewReader(inBytes))
		if err := vdl.Transcode(enc.Encoder(), dec.Decoder()); err != nil {
			t.Errorf("%s: error in transcode: %v", test.Name, err)
			continue
		}
		if got, want := buf.Bytes(), inBytes; !bytes.Equal(got, want) {
			t.Errorf("%s: got %x, want %x", test.Name, got, want)
		}
	}
}

// TODO(bprosnitz) This is probably not the right place for this test.
func TestTranscodeVDLValueToXEncoderNew(t *testing.T) {
	for _, test := range vomtest.Data() {
		var buf bytes.Buffer
		enc := NewXEncoder(&buf)
		if err := vdl.Write(enc.Encoder(), test.Value); err != nil {
			t.Errorf("%s: error in transcode: %v", test.Name, err)
			continue
		}
		if got, want := buf.Bytes(), hex2Bin(t, test.Hex); !bytes.Equal(got, want) {
			t.Errorf("%s: got %x, want %x", test.Name, got, want)
		}
	}
}
