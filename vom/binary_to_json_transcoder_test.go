package vom

import (
	"bytes"
	"strings"
	"testing"
)

// TestVOMBinaryToJSON tests converting from VOM Binary to JSON for a number of inputs.
func TestVOMBinaryToJSON(t *testing.T) {
	for _, test := range coderTests {
		if test.JSON == nil {
			continue
		}

		b, err := binFromHexPat(test.HexPat)
		if err != nil {
			t.Errorf("error decoding hex string in test %s: %v ", test.Name, test.HexPat)
			continue
		}
		r := strings.NewReader(string(b))
		var buf bytes.Buffer
		tr := NewBinaryToJSONTranscoder(&buf, r)

		for _, json := range test.JSON {
			buf.Reset()
			if err := tr.Transcode(); err != nil {
				t.Errorf("failure in test %v: %v", test.Name, err)
				continue
			}

			if err := equivalentJSON(json, buf.String()); err != nil {
				t.Errorf("result didn't match expectation in test %v: got %v. expected %v", test.Name, buf.String(), json)
			}
		}
	}

}
