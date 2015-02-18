package vom

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vom/testdata"
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
		if len(ri.UnionFields) > 0 {
			// Special case for union types, to decode into the union interface.
			testDecode(t, test.Name+" [union iface]", data, ri.WireType, want)
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
		testDecode(t, name, buf.String(), reflect.TypeOf(test.Want), test.Want)
	}
}
