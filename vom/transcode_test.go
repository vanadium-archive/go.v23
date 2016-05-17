// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"bytes"
	"flag"
	"regexp"
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vom/testdata/data81"
)

var filter *string = flag.String("testdata-filter", "", "regex for allowed testdata tests")

func init() {
	flag.Parse()
}

func filterRegex(t *testing.T) *regexp.Regexp {
	pattern := ".*" + *filter + ".*"
	regex, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("invalid regex filter %q", *filter)
	}
	return regex
}

func TestTranscodeXDecoderToXEncoder(t *testing.T) {
	regex := filterRegex(t)
	for _, test := range data81.Tests {
		if !regex.MatchString(test.Name) {
			continue
		}

		inBytes := hex2Bin(t, test.Hex)
		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		dec := NewDecoder(bytes.NewReader(inBytes))
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
func TestTranscodeVDLValueToXEncoder(t *testing.T) {
	regex := filterRegex(t)
	for _, test := range data81.Tests {
		if !regex.MatchString(test.Name) {
			continue
		}

		var buf bytes.Buffer
		enc := NewEncoder(&buf)
		if err := test.Value.VDLWrite(enc.Encoder()); err != nil {
			t.Errorf("%s: error in transcode: %v", test.Name, err)
			continue
		}
		if got, want := buf.Bytes(), hex2Bin(t, test.Hex); !bytes.Equal(got, want) {
			t.Errorf("%s: got %x, want %x", test.Name, got, want)
		}
	}
}
