package vom2

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vom2/testdata"
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

// TestNilEncoding tests that go nil and empty composite type values are both encoded as
// corresponding vdl zero values.
func TestNilEncoding(t *testing.T) {
	tests := []struct {
		input interface{}
		want  *vdl.Value
	}{
		{
			[]int64(nil),
			vdl.ZeroValue(vdl.TypeOf([]int64{})),
		},
		{
			[]int64{},
			vdl.ZeroValue(vdl.TypeOf([]int64{})),
		},
		{
			map[string]int64(nil),
			vdl.ZeroValue(vdl.TypeOf(map[string]int64{})),
		},
		{
			map[string]int64{},
			vdl.ZeroValue(vdl.TypeOf(map[string]int64{})),
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
