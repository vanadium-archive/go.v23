package vom2

import (
	"reflect"
	"strings"
	"testing"

	"veyron.io/veyron/veyron2/vom2/testdata"
)

func TestBinaryDecoder(t *testing.T) {
	for _, mode := range allReadModes {
		for _, test := range testdata.Tests {
			data, err := binFromHexPat(test.Hex)
			if err != nil {
				t.Errorf("%s %s: couldn't convert to binary from hexpat: %v", mode, test.Name, test.Hex)
				continue
			}
			// Decode into a new pointer of the value type.
			decoder, err := NewDecoder(mode.testReader(strings.NewReader(data)))
			if err != nil {
				t.Errorf("%s %s: NewDecoder failed: %v", mode, test.Name, err)
				continue
			}
			want, rvGot := test.Value, reflect.New(reflect.TypeOf(test.Value))
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
