// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vom/testdata"
)

func TestDecoder(t *testing.T) {
	for _, test := range testdata.Tests {
		// Decode hex patterns into binary data.
		binmagic, err := binFromHexPat(test.HexMagic)
		if err != nil {
			t.Errorf("%s: couldn't convert to binary from hexmagic: %q", test.Name, test.HexMagic)
			continue
		}
		bintype, err := binFromHexPat(test.HexType)
		if err != nil {
			t.Errorf("%s: couldn't convert to binary from hextype: %q", test.Name, test.HexType)
			continue
		}
		binvalue, err := binFromHexPat(test.HexValue)
		if err != nil {
			t.Errorf("%s: couldn't convert to binary from hexvalue: %q", test.Name, test.HexValue)
			continue
		}

		name := test.Name + " [vdl.Value]"
		testDecodeVDL(t, name, binmagic+bintype+binvalue, test.Value)
		name = test.Name + " [vdl.Value] (with TypeDecoder)"
		testDecodeVDLWithTypeDecoder(t, name, binmagic, bintype, binvalue, test.Value)

		name = test.Name + " [vdl.Any]"
		testDecodeVDL(t, name, binmagic+bintype+binvalue, vdl.AnyValue(test.Value))
		name = test.Name + " [vdl.Any] (with TypeDecoder)"
		testDecodeVDLWithTypeDecoder(t, name, binmagic, bintype, binvalue, vdl.AnyValue(test.Value))

		// Convert into Go value for the rest of our tests.
		goValue, err := toGoValue(test.Value)
		if err != nil {
			t.Errorf("%s: %v", test.Name, err)
			continue
		}

		name = test.Name + " [go value]"
		testDecodeGo(t, name, binmagic+bintype+binvalue, reflect.TypeOf(goValue), goValue)
		name = test.Name + " [go value] (with TypeDecoder)"
		testDecodeGoWithTypeDecoder(t, name, binmagic, bintype, binvalue, reflect.TypeOf(goValue), goValue)

		name = test.Name + " [go interface]"
		testDecodeGo(t, name, binmagic+bintype+binvalue, reflect.TypeOf((*interface{})(nil)).Elem(), goValue)
		name = test.Name + " [go interface] (with TypeDecoder)"
		testDecodeGoWithTypeDecoder(t, name, binmagic, bintype, binvalue, reflect.TypeOf((*interface{})(nil)).Elem(), goValue)
	}
}

func testDecodeVDL(t *testing.T, name, bin string, value *vdl.Value) {
	for _, mode := range allReadModes {
		head := fmt.Sprintf("%s (%s)", name, mode)
		decoder, err := NewDecoder(mode.testReader(strings.NewReader(bin)))
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
			t.Errorf("%s: Decode mismatch\nGOT %v\nWANT %v", head, got, want)
			return
		}
	}
}

func testDecodeVDLWithTypeDecoder(t *testing.T, name, binmagic, bintype, binvalue string, value *vdl.Value) {
	for _, mode := range allReadModes {
		head := fmt.Sprintf("%s (%s)", name, mode)
		typedec, err := NewTypeDecoder(mode.testReader(strings.NewReader(binmagic + bintype)))
		if err != nil {
			t.Errorf("%s: NewTypeDecoder failed: %v", head, err)
			return
		}
		decoder, err := NewDecoderWithTypeDecoder(mode.testReader(strings.NewReader(binmagic+binvalue)), typedec)
		if err != nil {
			t.Errorf("%s: NewDecoderWithTypeDecoder failed: %v", head, err)
			return
		}
		got := vdl.ZeroValue(value.Type())
		if err := decoder.Decode(got); err != nil {
			t.Errorf("%s: Decode failed: %v", head, err)
			return
		}
		if want := value; !vdl.EqualValue(got, want) {
			t.Errorf("%s: Decode mismatch\nGOT %v\nWANT %v", head, got, want)
			return
		}
	}
}

func testDecodeGo(t *testing.T, name, bin string, rt reflect.Type, want interface{}) {
	for _, mode := range allReadModes {
		head := fmt.Sprintf("%s (%s)", name, mode)
		decoder, err := NewDecoder(mode.testReader(strings.NewReader(bin)))
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
			t.Errorf("%s: Decode mismatch\nGOT %T %#v\nWANT %T %#v", head, got, got, want, want)
			return
		}
	}
}

func testDecodeGoWithTypeDecoder(t *testing.T, name, binmagic, bintype, binvalue string, rt reflect.Type, want interface{}) {
	for _, mode := range allReadModes {
		head := fmt.Sprintf("%s (%s)", name, mode)
		typedec, err := NewTypeDecoder(mode.testReader(strings.NewReader(binmagic + bintype)))
		if err != nil {
			t.Errorf("%s: NewTypeDecoder failed: %v", head, err)
			return
		}
		decoder, err := NewDecoderWithTypeDecoder(mode.testReader(strings.NewReader(binmagic+binvalue)), typedec)
		if err != nil {
			t.Errorf("%s: NewDecoderWithTypeDecoder failed: %v", head, err)
			return
		}
		rv := reflect.New(rt)
		if err := decoder.Decode(rv.Interface()); err != nil {
			t.Errorf("%s: Decode failed: %v", head, err)
			return
		}
		if got := rv.Elem().Interface(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: Decode mismatch\nGOT %T %#v\nWANT %T %#v", head, got, got, want, want)
			return
		}
	}
}

// In concurrent modes, one goroutine may try to read vom types before they are
// actually sent by other goroutine. We use a simple buffered pipe to provide
// blocking read since bytes.Buffer will return EOF in this case.
type pipe struct {
	b bytes.Buffer
	m sync.Mutex
	c sync.Cond
}

func newPipe() (io.Reader, io.Writer) {
	p := &pipe{}
	p.c.L = &p.m
	return p, p
}

func (r *pipe) Read(p []byte) (n int, err error) {
	r.m.Lock()
	defer r.m.Unlock()
	for r.b.Len() == 0 {
		r.c.Wait()
	}
	return r.b.Read(p)
}

func (w *pipe) Write(p []byte) (n int, err error) {
	w.m.Lock()
	defer w.m.Unlock()
	defer w.c.Signal()
	return w.b.Write(p)
}

// TestRoundtrip* tests test encoding and then decoding results in various modes.
func TestRoundtrip(t *testing.T)                   { testRoundtrip(t, false, 1) }
func TestRoundtripWithTypeDecoder_1(t *testing.T)  { testRoundtrip(t, true, 1) }
func TestRoundtripWithTypeDecoder_5(t *testing.T)  { testRoundtrip(t, true, 5) }
func TestRoundtripWithTypeDecoder_10(t *testing.T) { testRoundtrip(t, true, 10) }
func TestRoundtripWithTypeDecoder_20(t *testing.T) { testRoundtrip(t, true, 20) }

func testRoundtrip(t *testing.T, withTypeEncoderDecoder bool, concurrency int) {
	mp := runtime.GOMAXPROCS(runtime.NumCPU())
	defer runtime.GOMAXPROCS(mp)

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
		{testdata.RecA{}, testdata.RecA(nil)},
		{testdata.RecStruct{}, testdata.RecStruct{}},
		// Test for verifying correctness when encoding/decoding shared types concurrently.
		{testdata.MStruct{}, testdata.MStruct{E: vdl.AnyType, F: vdl.AnyValue(nil)}},
		{testdata.NStruct{}, testdata.NStruct{}},
		{testdata.XyzStruct{}, testdata.XyzStruct{}},
		{testdata.YzStruct{}, testdata.YzStruct{}},
		{testdata.MBool(false), testdata.MBool(false)},
		{testdata.NString(""), testdata.NString("")},
	}

	var (
		typeenc *TypeEncoder
		typedec *TypeDecoder
	)
	if withTypeEncoderDecoder {
		var err error
		r, w := newPipe()
		typeenc, err = NewTypeEncoder(w)
		if err != nil {
			t.Fatal("NewTypeEncoder failed: %v", err)
		}
		typedec, err = NewTypeDecoder(r)
		if err != nil {
			t.Fatal("NewTypeDecoder failed: %v", err)
		}
	}

	var wg sync.WaitGroup
	for n := 0; n < concurrency; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for _, n := range rand.Perm(len(tests) * 10) {
				test := tests[n%len(tests)]
				name := fmt.Sprintf("[%d]:%#v,%#v", n, test.In, test.Want)

				var (
					encoder *Encoder
					decoder *Decoder
					buf     bytes.Buffer
					err     error
				)
				if withTypeEncoderDecoder {
					encoder, err = NewEncoderWithTypeEncoder(&buf, typeenc)
					if err != nil {
						t.Errorf("%s: NewEncoderWithTypeEncoder failed: %v", name, err)
						return
					}
					decoder, err = NewDecoderWithTypeDecoder(&buf, typedec)
					if err != nil {
						t.Errorf("%s: NewDecoderWithTypeDecoder failed: %v", name, err)
						return
					}
				} else {
					encoder, err = NewEncoder(&buf)
					if err != nil {
						t.Errorf("%s: NewEncoder failed: %v", name, err)
						return
					}
					decoder, err = NewDecoder(&buf)
					if err != nil {
						t.Errorf("%s: NewDecoder failed: %v", name, err)
						return
					}
				}

				if err := encoder.Encode(test.In); err != nil {
					t.Errorf("%s: Encode(%#v) failed: %v", name, test.In, err)
					return
				}
				rv := reflect.New(reflect.TypeOf(test.Want))
				if err := decoder.Decode(rv.Interface()); err != nil {
					t.Errorf("%s: Decode failed: %v", name, err)
					return
				}
				if got := rv.Elem().Interface(); !reflect.DeepEqual(got, test.Want) {
					t.Errorf("%s: Decode mismatch\nGOT %T %#v\nWANT %T %#v", name, got, got, test.Want, test.Want)
					return
				}
			}
		}(n)
	}
	wg.Wait()
}
