package vom2

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vom2/testdata"
)

func TestBinaryDecoder(t *testing.T) {
	for _, test := range testdata.Tests {
		want, rtWant := test.Value, reflect.TypeOf(test.Value)
		data, err := binFromHexPat(test.Hex)
		if err != nil {
			t.Errorf("%s: couldn't convert to binary from hexpat: %v", test.Name, test.Hex)
			continue
		}
		testDecode(t, test.Name, data, rtWant, want)
		// Check that we can derive reflect info for all our values.
		ri, err := vdl.DeriveReflectInfo(rtWant)
		if err != nil {
			t.Fatalf("%s: DeriveReflectInfo(%v) failed: %v", test.Name, rtWant, err)
		}
		if len(ri.OneOfFields) > 0 {
			// Special case for oneof types, to decode into the oneof interface.
			testDecode(t, test.Name+" [oneof iface]", data, ri.WireType, want)
		}
	}
}

func testDecode(t *testing.T, name, data string, rt reflect.Type, want interface{}) {
	for _, mode := range allReadModes {
		head := fmt.Sprintf("%s (%s)", name, mode)
		// Decode into a new pointer of the value type.
		decoder, err := NewDecoder(mode.testReader(strings.NewReader(data)))
		if err != nil {
			t.Errorf("%s: NewDecoder failed: %v", head, err)
			return
		}
		rvGot := reflect.New(rt)
		if err := decoder.Decode(rvGot.Interface()); err != nil {
			t.Errorf("%s: Decode(ptr) failed: %v", head, err)
			return
		}
		if got := rvGot.Elem().Interface(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: Decode(ptr)\nGOT  %T %#v\nWANT %T %#v", head, got, got, want, want)
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
