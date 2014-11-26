package vom2

import (
	"bytes"
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

// TestNilDecoding tests that go nil and empty composite type values are both
// decoded as nil values.
func TestNilDecoding(t *testing.T) {
	tests := []struct {
		input interface{}
		want  interface{}
	}{
		{
			[]int64(nil),
			[]int64(nil),
		},
		{
			[]int64{},
			[]int64(nil),
		},
		{
			map[string]int64(nil),
			map[string]int64(nil),
		},
		{
			map[string]int64{},
			map[string]int64(nil),
		},
	}
	for _, test := range tests {
		var buf bytes.Buffer
		enc, err := NewBinaryEncoder(&buf)
		if err != nil {
			t.Fatalf("Error creating new encoder: %v", err)
		}
		if err := enc.Encode(test.input); err != nil {
			t.Errorf("Error encoding %v: %v", test.input, err)
			continue
		}
		dec, err := NewDecoder(bytes.NewBuffer(buf.Bytes()))
		if err != nil {
			t.Fatalf("Error creating new decoder: %v", err)
		}
		var got interface{}
		if err := dec.Decode(&got); err != nil {
			t.Errorf("Error decoding %v", err)
		}
		if want := test.want; !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}
