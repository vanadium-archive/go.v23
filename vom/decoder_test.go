package vom

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vom/testdata"
)

func TestBinaryDecoder(t *testing.T) {
	for _, test := range testdata.Tests {
		// Decode hex pattern into binary data.
		data, err := binFromHexPat(test.Hex)
		if err != nil {
			t.Errorf("%s: couldn't convert to binary from hexpat: %v", test.Name, test.Hex)
			continue
		}

		name := test.Name + " [vdl.Value]"
		testDecodeVDL(t, name, data, test.Value)

		name = test.Name + " [vdl.Any]"
		testDecodeVDL(t, name, data, vdl.AnyValue(test.Value))

		// Convert into Go value for the rest of our tests.
		goValue, err := toGoValue(test.Value)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}

		name = test.Name + " [go value]"
		testDecodeGo(t, name, data, reflect.TypeOf(goValue), goValue)

		name = test.Name + " [go interface]"
		testDecodeGo(t, name, data, reflect.TypeOf((*interface{})(nil)).Elem(), goValue)
	}
}

func testDecodeVDL(t *testing.T, name, data string, value *vdl.Value) {
	for _, mode := range allReadModes {
		head := fmt.Sprintf("%s (%s)", name, mode)
		decoder, err := NewDecoder(mode.testReader(strings.NewReader(data)))
		if err != nil {
			t.Errorf("%s: NewDecoder failed: %v", head, err)
			return
		}
		got := vdl.ZeroValue(value.Type())
		if err := decoder.Decode(got); err != nil {
			t.Errorf("%s: Decode failed: %v", head, err)
			return
		}
		if want := value; !vdl.EqualValue(got, want) {
			t.Errorf("%s: Decode mismatch\nGOT  %v\nWANT %v", head, got, want)
			return
		}
	}
}

func testDecodeGo(t *testing.T, name, data string, rt reflect.Type, want interface{}) {
	for _, mode := range allReadModes {
		head := fmt.Sprintf("%s (%s)", name, mode)
		decoder, err := NewDecoder(mode.testReader(strings.NewReader(data)))
		if err != nil {
			t.Errorf("%s: NewDecoder failed: %v", head, err)
			return
		}
		rvGot := reflect.New(rt)
		if err := decoder.Decode(rvGot.Interface()); err != nil {
			t.Errorf("%s: Decode failed: %v", head, err)
			return
		}
		if got := rvGot.Elem().Interface(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: Decode mismatch\nGOT  %T %#v\nWANT %T %#v", head, got, got, want, want)
			return
		}
	}
}

// TestRoundtrip tests encoding Input and then decoding results in Want.
func TestRoundtrip(t *testing.T) {
	tests := []struct {
		In, Want interface{}
	}{
		// Test that encoding nil/empty composites leads to nil.
		{[]byte(nil), []byte(nil)},
		{[]byte{}, []byte(nil)},
		{[]int64(nil), []int64(nil)},
		{[]int64{}, []int64(nil)},
		{map[string]int64(nil), map[string]int64(nil)},
		{map[string]int64{}, map[string]int64(nil)},
		{struct{}{}, struct{}{}},
		{struct{ A []byte }{nil}, struct{ A []byte }{}},
		{struct{ A []byte }{[]byte{}}, struct{ A []byte }{}},
		{struct{ A []int64 }{nil}, struct{ A []int64 }{}},
		{struct{ A []int64 }{[]int64{}}, struct{ A []int64 }{}},
		// Test that encoding nil typeobject leads to AnyType.
		{(*vdl.Type)(nil), vdl.AnyType},
		// Test that both encoding and decoding ignore unexported fields.
		{struct{ a, X, b string }{"a", "XYZ", "b"}, struct{ d, X, e string }{X: "XYZ"}},
		{
			struct {
				a bool
				X string
				b int64
			}{true, "XYZ", 123},
			struct {
				a complex64
				X string
				b []byte
			}{X: "XYZ"},
		},
		// Test for array encoding/decoding.
		{[3]byte{1, 2, 3}, [3]byte{1, 2, 3}},
		{[3]int64{1, 2, 3}, [3]int64{1, 2, 3}},
		// Test for zero value struct/union field encoding/decoding.
		{struct{ A int64 }{0}, struct{ A int64 }{}},
		{struct{ T *vdl.Type }{nil}, struct{ T *vdl.Type }{vdl.AnyType}},
		{struct{ M map[uint64]struct{} }{make(map[uint64]struct{})}, struct{ M map[uint64]struct{} }{}},
		{struct{ M map[uint64]string }{make(map[uint64]string)}, struct{ M map[uint64]string }{}},
		{struct{ N struct{ A int64 } }{struct{ A int64 }{0}}, struct{ N struct{ A int64 } }{}},
		{struct{ N *testdata.NStruct }{&testdata.NStruct{false, "", 0}}, struct{ N *testdata.NStruct }{&testdata.NStruct{}}},
		{struct{ N *testdata.NStruct }{nil}, struct{ N *testdata.NStruct }{}},
		{testdata.NUnion(testdata.NUnionA{false}), testdata.NUnion(testdata.NUnionA{})},
	}
	for _, test := range tests {
		name := fmt.Sprintf("(%#v,%#v)", test.In, test.Want)
		var buf bytes.Buffer
		encoder, err := NewEncoder(&buf)
		if err != nil {
			t.Errorf("%s: NewEncoder failed: %v", name, err)
			continue
		}
		if err := encoder.Encode(test.In); err != nil {
			t.Errorf("%s: binary Encode(%#v) failed: %v", name, test.In, err)
			continue
		}
		testDecodeGo(t, name, buf.String(), reflect.TypeOf(test.Want), test.Want)
	}
}
