package vom

import (
	"bytes"
	"fmt"
	"testing"

	"v.io/core/veyron2/vom/testdata"
)

func TestBinaryEncoder(t *testing.T) {
	for _, test := range testdata.Tests {
		var buf bytes.Buffer
		encoder, err := NewBinaryEncoder(&buf)
		if err != nil {
			t.Errorf("%s: NewBinaryEncoder failed: %v", test.Name, err)
			continue
		}
		if err := encoder.Encode(test.Value); err != nil {
			t.Errorf("%s: binary Encode(%#v) failed: %v", test.Name, test.Value, err)
			continue
		}
		got, want := fmt.Sprintf("%x", buf.String()), test.Hex
		match, err := matchHexPat(got, want)
		if err != nil {
			t.Error(err)
		}
		if !match {
			t.Errorf("%s: binary Encode(%#v)\nGOT  %s\nWANT %s", test.Name, test.Value, got, want)
		}
	}
}
