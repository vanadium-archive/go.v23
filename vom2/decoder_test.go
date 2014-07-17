package vom2

import (
	"reflect"
	"strings"
	"testing"
)

func TestBinaryDecoder(t *testing.T) {
	for _, mode := range allReadModes {
		for _, test := range coderTests {
			data, err := binFromHexPat(test.Binary)
			if err != nil {
				t.Errorf("%s %s: couldn't convert to binary from hexpat: %v", mode, test.Name, test.Binary)
				continue
			}
			// Decode into a new pointer of the value type.
			decoder, err := NewDecoder(mode.testReader(strings.NewReader(data)))
			if err != nil {
				t.Errorf("%s %s: NewDecoder failed: %v", mode, test.Name, err)
				continue
			}
			for _, want := range test.Values {
				rvGot := reflect.New(reflect.TypeOf(want))
				if err := decoder.Decode(rvGot.Interface()); err != nil {
					t.Errorf("%s %s: Decode(ptr) [%#v] failed: %v", mode, test.Name, want, err)
					continue
				}
				if got := rvGot.Elem().Interface(); !reflect.DeepEqual(got, want) {
					t.Errorf("%s %s: Decode(ptr)\nGOT  %T %#v\nWANT %T %#v", mode, test.Name, got, got, want, want)
				}
			}
		}
	}
}
