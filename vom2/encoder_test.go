package vom2

import (
	"bytes"
	"fmt"
	"testing"
)

// TODO(toddw): Re-enable this test based on the new vomtest generated data.
func DISABLEDTestBinaryEncoder(t *testing.T) {
	for _, test := range coderTests {
		var buf bytes.Buffer
		encoder, err := NewBinaryEncoder(&buf)
		if err != nil {
			t.Errorf("%s: NewBinaryEncoder failed: %v", test.Name, err)
			continue
		}
		skipMatch := false
		for _, value := range test.Values {
			if err := encoder.Encode(value); err != nil {
				t.Errorf("%s: binary Encode(%#v) failed: %v", test.Name, value, err)
				skipMatch = true
			}
		}
		if skipMatch {
			continue
		}
		got, want := fmt.Sprintf("%x", buf.String()), test.Binary
		match, err := matchHexPat(got, want)
		if err != nil {
			t.Error(err)
		}
		if !match {
			t.Errorf("%s: binary Encode(%#v)\nGOT  %s\nWANT %s", test.Name, test.Values, got, want)
		}
	}
}
