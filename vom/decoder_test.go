package vom

import (
	"bytes"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
	"unsafe"

	"v.io/core/veyron/runtimes/google/lib/reflectutil"
)

// We run all of our tests in two different modes - one them gets reads as
// usual, and the other trickles reads one byte at a time.  This ensures the
// decoding logic correctly handles short successful reads.
type readerType int

const (
	readerRegular readerType = iota
	readerOneByte
)

func newDecoder(r io.Reader, rtype readerType) *Decoder {
	if rtype == readerOneByte {
		r = iotest.OneByteReader(r)
	}
	return NewDecoder(r)
}

func testDecoder(t *testing.T, format Format, rtype readerType) {
	for _, test := range coderTests {
		var data string
		switch format {
		case FormatBinary:
			var err error
			if data, err = binFromHexPat(test.HexPat); err != nil {
				t.Errorf("%s: couldn't convert to binary from hex pat: %v", test.Name, test.HexPat)
				continue
			}
		case FormatJSON:
			data = test.VomJSON
		}
		// Try decoding into nil, which ignores the actual value.
		decoder := newDecoder(strings.NewReader(data), rtype)
		for _, expect := range test.Values {
			if err := decoder.Decode(nil); err != nil {
				t.Errorf("%s: %v Decode(nil) of %#v failed: %v", test.Name, format, expect, err)
				continue
			}
		}
		// Try decoding each value using an rv of the pointer type.  The underlying
		// value has been allocated.
		decoder = newDecoder(strings.NewReader(data), rtype)
		for _, expect := range test.Values {
			rvActual := New(TypeOf(expect))
			if err := decoder.DecodeValue(rvActual); err != nil {
				t.Errorf("%s: %v Decode(ptr) of %#v failed: %v", test.Name, format, expect, err)
				continue
			}
			var actual interface{}
			if rvActual.Elem().IsValid() {
				actual = rvActual.Elem().Interface()
			}
			if !reflectutil.SharingDeepEqual(actual, expect) {
				t.Errorf("%s: %v Decode(ptr) mismatch\ngot  %#v %T\nwant %#v %T", test.Name, format, actual, actual, expect, expect)
			}
		}
		// Try decoding each value using an rv of the pointer-pointer type.  Only
		// the outer pointer has been allocated, but points to nil.
		decoder = newDecoder(strings.NewReader(data), rtype)
		for _, expect := range test.Values {
			rvActual := New(PtrTo(TypeOf(expect)))
			if err := decoder.DecodeValue(rvActual); err != nil {
				t.Errorf("%s: %v Decode(ptrptr) of %#v failed: %v", test.Name, format, expect, err)
				continue
			}
			var actual interface{}
			if rvActual.Elem().Elem().IsValid() {
				actual = rvActual.Elem().Elem().Interface()
			}
			if !reflectutil.SharingDeepEqual(actual, expect) {
				t.Errorf("%s: %v Decode(ptrptr) mismatch\ngot  %#v %T\nwant %#v %T", test.Name, format, actual, actual, expect, expect)
			}
		}
		// Try decoding each value using a pointer to a nil interface, which
		// constructs the object as long as it's registered.
		decoder = newDecoder(strings.NewReader(data), rtype)
		for _, expect := range test.Values {
			var actual interface{}
			if err := decoder.DecodeValue(ValueOf(&actual)); err != nil {
				t.Errorf("%s: %v Decode(iface) of %#v failed: %v", test.Name, format, expect, err)
				continue
			}
			if !reflectutil.SharingDeepEqual(actual, expect) {
				t.Errorf("%s: %v Decode(iface) mismatch\ngot  %#v %T\nwant %#v %T", test.Name, format, actual, actual, expect, expect)
			}
		}
		// Try decoding each value using a pointer to an interface containing an
		// allocated object of the concrete encoded type.  This doesn't rely on
		// registration, but does rely on the concrete object to be compatible with
		// the encoded object, which is trivially true here since we're using
		// exactly the same type.
		decoder = newDecoder(strings.NewReader(data), rtype)
		for _, expect := range test.Values {
			rvConcrete := New(TypeOf(expect)).Elem()
			var actual interface{} = rvConcrete.Interface()
			if err := decoder.DecodeValue(ValueOf(&actual)); err != nil {
				t.Errorf("%s: %v Decode(ptriface) of %#v failed: %v", test.Name, format, expect, err)
				continue
			}
			if !reflectutil.SharingDeepEqual(actual, expect) {
				t.Errorf("%s: %v Decode(ptriface) mismatch\ngot  %#v %T\nwant %#v %T", test.Name, format, actual, actual, expect, expect)
			}
		}
		// Try decoding each value, with leading whitespace.
		decoder = newDecoder(strings.NewReader("\t\n\r "+data), rtype)
		for _, expect := range test.Values {
			rvActual := New(TypeOf(expect))
			if err := decoder.DecodeValue(rvActual); err != nil {
				t.Errorf("%s: %v Decode(ptr) of %#v failed: %v", test.Name, format, expect, err)
				continue
			}
			var actual interface{}
			if rvActual.Elem().IsValid() {
				actual = rvActual.Elem().Interface()
			}
			if !reflectutil.SharingDeepEqual(actual, expect) {
				t.Errorf("%s: %v Decode(whitespace) mismatch\ngot  %#v %T\nwant %#v %T", test.Name, format, actual, actual, expect, expect)
			}
		}
	}
}
func DISABLEDTestDecoderBinary(t *testing.T) {
	testDecoder(t, FormatBinary, readerRegular)
}
func DISABLEDTestDecoderBinaryOneByte(t *testing.T) {
	testDecoder(t, FormatBinary, readerOneByte)
}
func DISABLEDTestDecoderJSON(t *testing.T) {
	testDecoder(t, FormatJSON, readerRegular)
}
func DISABLEDTestDecoderJSONOneByte(t *testing.T) {
	testDecoder(t, FormatJSON, readerOneByte)
}

func testEncodeDecode(t *testing.T, format Format, rtype readerType) {
	for _, test := range encodeDecodeTests {
		var buf bytes.Buffer
		encoder := NewEncoder(&buf).SetFormat(format)
		skipDecode := false
		for _, encVal := range test.EncValues {
			if err := encoder.Encode(encVal); err != nil {
				t.Errorf("%s: Encode(%#v) failed: %v", test.Name, encVal, err)
				skipDecode = true
			}
		}
		if skipDecode {
			continue // Move on to the next test
		}
		encoded := buf.Bytes()
		// Try decoding each value using an rv of the pointer type.
		decoder := newDecoder(bytes.NewReader(encoded), rtype)
		for ix, expect := range test.DecValues {
			rvActual := ToVomValue(reflect.New(reflect.TypeOf(expect)))
			derr := decoder.DecodeValue(rvActual)
			hasRE, err := matchIndexedErrorRE(derr, test.DecRE, ix)
			if err != nil {
				t.Errorf("%s: Decode(ptr) of %#v %v", test.Name, expect, err)
				continue
			}
			if hasRE {
				continue
			}
			var actual interface{}
			if rvActual.Elem().IsValid() {
				actual = rvActual.Elem().Interface()
			}
			if !reflectutil.SharingDeepEqual(actual, expect) {
				t.Errorf("%s: Decode(ptr) mismatch\ngot  %#v %T\nwant %#v %T", test.Name, actual, actual, expect, expect)
			}
		}
	}
}
func TestEncodeDecodeBinary(t *testing.T) {
	testEncodeDecode(t, FormatBinary, readerRegular)
}
func TestEncodeDecodeBinaryOneByte(t *testing.T) {
	testEncodeDecode(t, FormatBinary, readerOneByte)
}
func TestEncodeDecodeJSON(t *testing.T) {
	testEncodeDecode(t, FormatJSON, readerRegular)
}
func TestEncodeDecodeJSONOneByte(t *testing.T) {
	testEncodeDecode(t, FormatJSON, readerOneByte)
}

// Take all values from coderTests and encode into a single stream, alternating
// between formats.
func testEncodeDecodeMixedFormat(t *testing.T, rtype readerType) {
	var buf bytes.Buffer
	encoder := NewEncoder(&buf)
	skipDecode := false
	format := FormatBinary
	for _, test := range coderTests {
		for _, encVal := range test.Values {
			encoder.SetFormat(format)
			if err := encoder.Encode(encVal); err != nil {
				t.Errorf("%s: Encode(%#v) failed: %v", test.Name, encVal, err)
				skipDecode = true
			}
			if format == FormatBinary {
				format = FormatJSON
			} else {
				format = FormatBinary
			}
		}
	}
	if skipDecode {
		return // Don't bother trying to decode
	}
	encoded := buf.Bytes()
	// Try decoding each value using an rv of the pointer type.
	decoder := newDecoder(bytes.NewReader(encoded), rtype)
	for _, test := range coderTests {
		for _, expect := range test.Values {
			rvActual := ToVomValue(reflect.New(reflect.TypeOf(expect)))
			if err := decoder.DecodeValue(rvActual); err != nil {
				t.Errorf("%s: Decode(ptr) of %#v failed: %v", test.Name, expect, err)
				continue
			}
			var actual interface{}
			if rvActual.Elem().IsValid() {
				actual = rvActual.Elem().Interface()
			}
			if !reflectutil.SharingDeepEqual(actual, expect) {
				t.Errorf("%s: Decode(ptr) mismatch\ngot  %#v %T\nwant %#v %T", test.Name, actual, actual, expect, expect)
			}
		}
	}
}
func TestEncodeDecodeMixedFormat(t *testing.T) {
	testEncodeDecodeMixedFormat(t, readerRegular)
}
func TestEncodeDecodeMixedFormatOneByte(t *testing.T) {
	testEncodeDecodeMixedFormat(t, readerOneByte)
}

func testDecodeErrorBinary(t *testing.T, rtype readerType) {
	for _, test := range decodeErrorBinaryTests {
		data, err := binFromHexPat(test.HexPat)
		if err != nil {
			t.Errorf("%s: couldn't convert to binary from hex pat: %v", test.Name, test.HexPat)
			continue
		}
		decoder := newDecoder(strings.NewReader(data), rtype)
		for ix, decVal := range test.Values {
			rvActual := ToVomValue(reflect.New(reflect.TypeOf(decVal)))
			derr := decoder.DecodeValue(rvActual)
			if _, err := matchIndexedErrorRE(derr, test.DecRE, ix); err != nil {
				t.Errorf("%s: decoding %v", test.Name, err)
			}
		}
	}
}
func TestDecodeErrorBinary(t *testing.T) {
	testDecodeErrorBinary(t, readerRegular)
}
func TestDecodeErrorBinaryOneByte(t *testing.T) {
	testDecodeErrorBinary(t, readerOneByte)
}

func testDecodeErrorJSON(t *testing.T, rtype readerType) {
	for _, test := range decodeErrorJSONTests {
		decoder := newDecoder(strings.NewReader(test.JSON), rtype)
		for ix, decVal := range test.Values {
			rvActual := ToVomValue(reflect.New(reflect.TypeOf(decVal)))
			derr := decoder.DecodeValue(rvActual)
			if _, err := matchIndexedErrorRE(derr, test.DecRE, ix); err != nil {
				t.Errorf("%s: decoding %v", test.Name, err)
			}
		}
	}
}
func TestDecodeErrorJSON(t *testing.T) {
	testDecodeErrorJSON(t, readerRegular)
}
func TestDecodeErrorJSONOneByte(t *testing.T) {
	testDecodeErrorJSON(t, readerOneByte)
}

var (
	auint   uint   = math.MaxUint8
	auint8  uint8  = math.MaxUint8
	buint8  uint8  = math.MaxUint8
	buint16 uint16 = math.MaxUint8
	cuint16 uint16 = math.MaxUint16
	cuint32 uint32 = math.MaxUint16
	duint32 uint32 = math.MaxUint32
	duint64 uint64 = math.MaxUint32
	euint64 uint64 = math.MaxUint32
	euint   uint   = math.MaxUint32

	aint   int   = math.MinInt8
	aint8  int8  = math.MinInt8
	bint8  int8  = math.MinInt8
	bint16 int16 = math.MinInt8
	cint16 int16 = math.MinInt16
	cint32 int32 = math.MinInt16
	dint32 int32 = math.MinInt32
	dint64 int64 = math.MinInt32
	eint64 int64 = math.MinInt32
	eint   int   = math.MinInt32

	aflt32 float32 = math.MaxFloat32
	aflt64 float64 = math.MaxFloat32
	bflt32 float32 = math.SmallestNonzeroFloat32
	bflt64 float64 = math.SmallestNonzeroFloat32

	acx64  complex64  = complex(math.MaxFloat32, math.SmallestNonzeroFloat32)
	acx128 complex128 = complex(math.MaxFloat32, math.SmallestNonzeroFloat32)
	bcx64  complex64  = complex(math.SmallestNonzeroFloat32, math.MaxFloat32)
	bcx128 complex128 = complex(math.SmallestNonzeroFloat32, math.MaxFloat32)

	astr = string("abc")
	absl = []byte("abc")
	bstr = string("def")
	bbsl = []byte("def")

	abool bool = true

	intPtr1a   *int  = new(int)
	intPtr1b   *int  = new(int)
	intPaPtr1a **int = new(*int)
	intPbPtr1a **int = new(*int)
	intPaPtr1b **int = new(*int)
	intPbPtr1b **int = new(*int)
)

func init() {
	*intPtr1a = 1
	*intPtr1b = 1
	*intPaPtr1a = intPtr1a
	*intPbPtr1a = intPtr1a
	*intPaPtr1b = intPtr1b
	*intPbPtr1b = intPtr1b
}

type abUF struct {
	A uint8
	B float32
}

type abFI struct {
	A float64
	B int64
}

type bcIS struct {
	B int32
	C string
}

type bcSI struct {
	B string
	C int32
}

type abInt struct {
	A, B int
}

type abIntPtr struct {
	A, B *int
}

type abIntPPtr struct {
	A, B **int
}

type e []string

// encodeDecodeTests tests encoding and decoding with different types of values.
var encodeDecodeTests = []struct {
	Name      string
	EncValues v
	DecValues v
	DecRE     e
}{
	// Test various valid combinations of our primitives, including different
	// types and combinations of pointers.
	{
		"Uint",
		v{auint8, buint16, cuint32, duint64, euint},
		v{auint, buint8, cuint16, duint32, euint64},
		nil},
	{
		"UintPtr",
		v{&auint8, &buint16, &cuint32, &duint64, &euint},
		v{&auint, &buint8, &cuint16, &duint32, &euint64},
		nil},
	{
		"UintToPtr",
		v{auint8, buint16, cuint32, duint64, euint},
		v{&auint, &buint8, &cuint16, &duint32, &euint64},
		nil},
	{
		"UintFromPtr",
		v{&auint8, &buint16, &cuint32, &duint64, &euint},
		v{auint, buint8, cuint16, duint32, euint64},
		nil},
	{
		"Int",
		v{aint8, bint16, cint32, dint64, eint},
		v{aint, bint8, cint16, dint32, eint64},
		nil},
	{
		"IntPtr",
		v{&aint8, &bint16, &cint32, &dint64, &eint},
		v{&aint, &bint8, &cint16, &dint32, &eint64},
		nil},
	{
		"IntToPtr",
		v{aint8, bint16, cint32, dint64, eint},
		v{&aint, &bint8, &cint16, &dint32, &eint64},
		nil},
	{
		"IntFromPtr",
		v{&aint8, &bint16, &cint32, &dint64, &eint},
		v{aint, bint8, cint16, dint32, eint64},
		nil},
	{
		"Float",
		v{aflt32, bflt64, &aflt32, &bflt64, aflt32, bflt64, &aflt32, &bflt64},
		v{aflt64, bflt32, &aflt64, &bflt32, &aflt64, &bflt32, aflt64, bflt32},
		nil},
	{
		"MixedNumbers",
		v{int64(255), float32(65535), uint64(math.MaxInt32), float64(-1 << 53), uint32(1 << 24), int64(1 << 53)},
		v{uint8(255), uint16(65535), int32(math.MaxInt32), int64(-1 << 53), float32(1 << 24), float64(1 << 53)},
		nil},
	{
		"Complex",
		v{acx64, bcx128, &acx64, &bcx128, acx64, bcx128, &acx64, &bcx128},
		v{acx128, bcx64, &acx128, &bcx64, &acx128, &bcx64, acx128, bcx64},
		nil},
	{
		"String",
		v{astr, bbsl, &astr, &bbsl, astr, bbsl, &astr, &bbsl},
		v{absl, bstr, &absl, &bstr, &absl, &bstr, absl, bstr},
		nil},
	{
		"Bool",
		v{abool, &abool, abool, &abool},
		v{abool, &abool, &abool, abool},
		nil},

	// Test decoding errors for primitives
	{
		"UintErrors",
		v{int(-1), uint16(256), float32(1.5), int(-2), float64(2.5)},
		v{uint(0), uint8(0), uint16(0), uint32(0), uint64(0)},
		e{`vom: type mismatch, can't fill int -1 into value of type "uint"`,
			`vom: type mismatch, can't fill uint 256 into value of type "uint8"`,
			`vom: type mismatch, can't fill float 1.5 into value of type "uint16"`,
			`vom: type mismatch, can't fill int -2 into value of type "uint32"`,
			`vom: type mismatch, can't fill float 2.5 into value of type "uint64"`}},
	{
		"IntErrors",
		v{float32(1.5), uint16(128), uint16(32768), float32(-2.5), float64(-3.5)},
		v{int(0), int8(0), int16(0), int32(0), int64(0)},
		e{`vom: type mismatch, can't fill float 1.5 into value of type "int"`,
			`vom: type mismatch, can't fill uint 128 into value of type "int8"`,
			`vom: type mismatch, can't fill uint 32768 into value of type "int16"`,
			`vom: type mismatch, can't fill float -2.5 into value of type "int32"`,
			`vom: type mismatch, can't fill float -3.5 into value of type "int64"`}},
	{
		"FloatErrors",
		v{uint32(1 << 25), uint64(1 << 54)},
		v{float32(0), float64(0)},
		e{`vom: type mismatch, can't fill uint \d+ into value of type "float32"`,
			`vom: type mismatch, can't fill uint \d+ into value of type "float64"`}},
	{
		"BoolErrors",
		v{true, true, true, true},
		v{uint(0), int(0), float32(0), string("")},
		e{`vom: type mismatch, can't fill bool true into value of type "uint"`,
			`vom: type mismatch, can't fill bool true into value of type "int"`,
			`vom: type mismatch, can't fill bool true into value of type "float32"`,
			`vom: type mismatch, can't fill bool true into value of type "string"`}},
	{
		"StringErrors",
		v{"a", "b", "c", "d"},
		v{uint(0), int(0), float32(0), bool(true)},
		e{`vom: type mismatch, can't fill string with len 1 into value of type "uint"`,
			`vom: type mismatch, can't fill string with len 1 into value of type "int"`,
			`vom: type mismatch, can't fill string with len 1 into value of type "float32"`,
			`vom: type mismatch, can't fill string with len 1 into value of type "bool"`}},

	// Test compatible structs.
	{
		"StructMixedTypes",
		v{abUF{1, -1}, abFI{2, -2}, &abUF{3, -3}, &abFI{4, -4}, abUF{5, -5}, abFI{6, -6}, &abUF{7, -7}, &abFI{8, -8}},
		v{abFI{1, -1}, abUF{2, -2}, &abFI{3, -3}, &abUF{4, -4}, &abFI{5, -5}, &abUF{6, -6}, abFI{7, -7}, abUF{8, -8}},
		nil},
	{
		"StructOverlapFields",
		v{abUF{1, -1}, bcIS{-2, "2"}, &abUF{3, -3}, &bcIS{-4, "4"}, abUF{5, -5}, bcIS{-6, "6"}, &abUF{7, -7}, &bcIS{-8, "8"}},
		v{bcIS{-1, ""}, abUF{0, -2}, &bcIS{-3, ""}, &abUF{0, -4}, &bcIS{-5, ""}, &abUF{0, -6}, &bcIS{-7, ""}, &abUF{0, -8}},
		nil},

	// Test decoding errors for structs
	{
		"StructIncompatibleFields",
		v{abUF{1, -1}, abFI{2, -2}, bcIS{3, "3"}},
		v{bcSI{"", 0}, bcSI{"", 0}, bcSI{"", 0}},
		e{`vom: type mismatch, can't fill float -1 into value of type "string"`,
			`vom: type mismatch, can't fill int -2 into value of type "string"`,
			`vom: type mismatch, can't fill int 3 into value of type "string"`}},

	// Test pointer sharing into different numbers of pointers
	{
		"PtrSharing",
		v{abIntPtr{intPtr1a, intPtr1b}, abIntPtr{intPtr1a, intPtr1a}},
		v{abIntPtr{intPtr1a, intPtr1b}, abIntPtr{intPtr1a, intPtr1a}},
		nil},
	{
		"PtrSharingLess",
		v{abIntPPtr{intPaPtr1a, intPbPtr1b}, abIntPPtr{intPaPtr1a, intPaPtr1a}},
		v{abIntPtr{intPtr1a, intPtr1b}, abIntPtr{intPtr1a, intPtr1a}},
		nil},
	{
		"PtrSharingMore",
		v{abIntPtr{intPtr1a, intPtr1b}, abIntPtr{intPtr1a, intPtr1a}},
		v{abIntPPtr{intPaPtr1a, intPbPtr1b}, abIntPPtr{intPaPtr1a, intPbPtr1a}},
		nil},
	{
		"PtrSharingFlatLess",
		v{abIntPPtr{intPaPtr1a, intPbPtr1b}, abIntPPtr{intPaPtr1a, intPaPtr1a}},
		v{abInt{1, 1}, abInt{1, 1}},
		nil},
	{
		"PtrSharingFlatMore",
		v{abInt{1, 1}, abInt{1, 1}},
		v{abIntPtr{intPtr1a, intPtr1b}, abIntPPtr{intPaPtr1a, intPbPtr1b}},
		nil},

	// TypeNotRegistered succeed with vom.Value because type registration is no longer needed in
	// this case.
	{
		"TypeNotRegistered",
		v{NestedInterface{abInt{}}},
		v{NestedInterface{abInt{}}},
		nil,
	},

	// Test string escaping.
	{
		"StringEscapingSimple",
		v{allCtrlStr, MultiStringKeyMap{[2]string{allCtrlStr, allCtrlStr}: allCtrlStr}},
		v{allCtrlStr, MultiStringKeyMap{[2]string{allCtrlStr, allCtrlStr}: allCtrlStr}},
		nil},
	{
		// 1,2,3,4 byte utf8, respectively:
		// $ (U+0024), Σ (U+03A3), ⾶ (U+2FB6), 𝄞 (U+1D11E)
		"StringEscapingUnicode",
		v{"$Σ⾶𝄞", MultiStringKeyMap{[2]string{"$Σ⾶𝄞", "$Σ⾶𝄞"}: "$Σ⾶𝄞"}},
		v{"$Σ⾶𝄞", MultiStringKeyMap{[2]string{"$Σ⾶𝄞", "$Σ⾶𝄞"}: "$Σ⾶𝄞"}},
		nil},

	// Test number corner cases.
	{
		"NumberCornerCases",
		v{uint64((1 << 64) - 1), uint64((1 << 64) - 2), int64(-1 << 63), int64((-1 << 63) + 1), int64((1 << 63) - 1), int64((1 << 63) - 2)},
		v{uint64((1 << 64) - 1), uint64((1 << 64) - 2), int64(-1 << 63), int64((-1 << 63) + 1), int64((1 << 63) - 1), int64((1 << 63) - 2)},
		nil},
}

const allCtrlStr = "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x20\"<>&\u2028\u2029"

type VomDecodeNonPtrReceiver uint

func (v VomDecodeNonPtrReceiver) VomDecode(u uint) error {
	return nil
}

type VomDecodeBadInArgs uint

func (v *VomDecodeBadInArgs) VomDecode() error {
	return nil
}

type VomDecodeBadOutArgs uint

func (v *VomDecodeBadOutArgs) VomDecode(u uint) {}

type VomDecodeNotErrorOutArg uint

func (v *VomDecodeNotErrorOutArg) VomDecode(u uint) string {
	return ""
}

type VomDecodeA uint

func (v *VomDecodeA) VomDecode(u uint) error {
	return nil
}

type VomDecodeB uint

func (v *VomDecodeB) VomDecode(a VomDecodeA) error {
	return nil
}

type VomDecodeCycle uint

func (v *VomDecodeCycle) VomDecode(c VomDecodeCycle) error {
	return nil
}

// decodeErrorBinaryTests tests binary decoding errors.
var decodeErrorBinaryTests = []struct {
	Name   string
	HexPat string
	Values v
	DecRE  e
}{
	{
		"LeftoverBytes1",
		"ff810510013100ff",
		v{true},
		e{`vom: decoding saw 1 leftover bytes`}},
	{
		"LeftoverBytes2",
		"ff810610013100ffff",
		v{true},
		e{`vom: decoding saw 2 leftover bytes`}},
	{
		"DuplicateWireDef",
		"ff810710013101014100ff810710013101014200ff820101",
		v{uint(0)},
		e{`vom: duplicate wiredef for id 65`}},
	{
		"UnknownBootstrapTypeID",
		"0000",
		v{uint(0)},
		e{`vom: unknown bootstrap type id 0`}},
	{
		"UnknownTypeID",
		"ff8200",
		v{uint(0)},
		e{`vom: unknown user type id 65`}},
	{
		"Chan",
		"0401",
		v{make(chan int)},
		e{`vom: unsupported kind "chan" type "chan int"`}},
	{
		"Func",
		"0401",
		v{func() {}},
		e{`vom: unsupported kind "func" type "func\(\)"`}},
	{
		"UnsafePointer",
		"0401",
		v{unsafe.Pointer(nil)},
		e{`vom: unsupported kind "unsafe.Pointer" type "unsafe.Pointer"`}},

	// Test decoding errors for VomDecode
	{
		"VomDecodeNonPtrReceiver",
		"0401",
		v{VomDecodeNonPtrReceiver(0)},
		e{`vom: VomDecode can't be defined on non-pointer receiver`}},
	{
		"VomDecodeBadInArgs",
		"0401",
		v{VomDecodeBadInArgs(0)},
		e{`vom: VomDecode must have one in-arg`}},
	{
		"VomDecodeBadOutArgs",
		"0401",
		v{VomDecodeBadOutArgs(0)},
		e{`vom: VomDecode must have one out-arg`}},
	{
		"VomDecodeNotErrorOutArg",
		"0401",
		v{VomDecodeNotErrorOutArg(0)},
		e{`vom: VomDecode out-arg must be "error"`}},
	{
		"VomDecodeMulti",
		"0401",
		v{VomDecodeB(0)},
		e{`vom: VomDecode arg can't itself have VomDecode method`}},
	{
		"VomDecodeCycle",
		"0401",
		v{VomDecodeCycle(0)},
		e{`vom: VomDecode arg can't itself have VomDecode method`}},
}

// decodeErrorJSONTests tests JSON decoding errors.
var decodeErrorJSONTests = []struct {
	Name   string
	JSON   string
	Values v
	DecRE  e
}{
	{
		"NoLabel",
		`[123]`,
		v{0},
		e{`vom: json msg doesn't start with label`}},
	{
		"NoTypeComma",
		`["type" "v.io/core/veyron/lib/vom.Foo int"]`,
		v{0},
		e{`vom: json type msg missing typedef comma`}},
	{
		"NoTypeString",
		`["type", 123]`,
		v{0},
		e{`vom: json type msg missing typedef string`}},
	{
		"BadTypeNameDef",
		`["type", "foo"]`,
		v{0},
		e{`vom: json missing whitespace between name and def: "foo"`}},
	{
		"BadTypeEmptyName",
		`["type", " foo"]`,
		v{0},
		e{`vom: json empty type name: " foo"`}},
	{
		"BadTypeBadName",
		`["type", "#foo# int"]`,
		v{0},
		e{`vom: json name has invalid runes: "#foo#"`}},
	{
		"BadTypeTags1",
		`["type", "v.io/core/veyron/lib/vom.Foo int", 123]`,
		v{0},
		e{`vom: json expected array; saw start number`}},
	{
		"BadTypeTags2",
		`["type", "v.io/core/veyron/lib/vom.Foo int", [123]]`,
		v{0},
		e{`vom: json expected string; saw start number`}},
	{
		"ExtraTypeItem",
		`["type", "v.io/core/veyron/lib/vom.Foo int", ["foo"], 123]`,
		v{0},
		e{`vom: json expected end type msg; saw comma`}},
	{
		"DuplicateWireDef",
		`["type", "v.io/core/veyron/lib/vom.Foo int"]
["type", "v.io/core/veyron/lib/vom.Foo int"]`,
		v{0},
		e{`vom: json duplicate typedef name "v.io/core/veyron/lib/vom.Foo"`}},
	{
		"NoValue",
		`["int"]`,
		v{0},
		e{`vom: json typed value "int" missing value; saw end array`}},
	{
		"ExtraValueItem",
		`["int",123,"extra"]`,
		v{0},
		e{`vom: json expected end value msg; saw comma`}},
	{
		"Chan",
		`["int",123]`,
		v{make(chan int)},
		e{`vom: unsupported kind "chan" type "chan int"`}},
	{
		"Func",
		`["int",123]`,
		v{func() {}},
		e{`vom: unsupported kind "func" type "func\(\)"`}},
	{
		"UnsafePointer",
		`["int",123]`,
		v{unsafe.Pointer(nil)},
		e{`vom: unsupported kind "unsafe.Pointer" type "unsafe.Pointer"`}},

	// Test decoding errors for VomDecode
	{
		"VomDecodeNonPtrReceiver",
		`["int",123]`,
		v{VomDecodeNonPtrReceiver(0)},
		e{`vom: VomDecode can't be defined on non-pointer receiver`}},
	{
		"VomDecodeBadInArgs",
		`["int",123]`,
		v{VomDecodeBadInArgs(0)},
		e{`vom: VomDecode must have one in-arg`}},
	{
		"VomDecodeBadOutArgs",
		`["int",123]`,
		v{VomDecodeBadOutArgs(0)},
		e{`vom: VomDecode must have one out-arg`}},
	{
		"VomDecodeNotErrorOutArg",
		`["int",123]`,
		v{VomDecodeNotErrorOutArg(0)},
		e{`vom: VomDecode out-arg must be "error"`}},
	{
		"VomDecodeMulti",
		`["int",123]`,
		v{VomDecodeB(0)},
		e{`vom: VomDecode arg can't itself have VomDecode method`}},
	{
		"VomDecodeCycle",
		`["int",123]`,
		v{VomDecodeCycle(0)},
		e{`vom: VomDecode arg can't itself have VomDecode method`}},

	// Test number decoding errors
	{
		"Uint64TooBig",
		`["uint64",18446744073709551616]`,
		v{uint64(0)},
		e{`vom: number overflow`}},
	{
		"Uint64LeadingZero",
		`["uint64",01]`,
		v{uint64(0)},
		e{`vom: invalid number syntax`}},
	{
		"Uint64Negative",
		`["uint64",-1]`,
		v{uint64(0)},
		e{`vom: invalid number syntax`}},
	{
		"Int64TooSmall",
		`["int64",-9223372036854775809]`,
		v{int64(0)},
		e{`vom: number overflow`}},
	{
		"Int64TooBig",
		`["int64",9223372036854775808]`,
		v{int64(0)},
		e{`vom: number overflow`}},
	{
		"Int64LeadingZero",
		`["int64",-01]`,
		v{int64(0)},
		e{`vom: invalid number syntax`}},
	{
		"Int64NoDigits",
		`["int64",-]`,
		v{int64(0)},
		e{`vom: invalid number syntax`}},

	// Test string decoding errors
	{
		"StringNoEnd",
		`["string","abc]`,
		v{string("")},
		e{`EOF`}},
	{
		"StringBadEscape",
		`["string","ab\z"]`,
		v{string("")},
		e{`vom: json unknown escape char 'z'`}},
	{
		"StringBadEscapedRune",
		`["string","ab\udfff"]`,
		v{string("")},
		e{`vom: invalid rune`}},
	{
		"StringBadRune",
		`["string","ab` + "\xc0" + `"]`,
		v{string("")},
		e{`vom: invalid rune`}},
	{
		"MultiStringMismatchedQuotes",
		`["map[[2]string]string",{"[\"abc\",\"def"]":"ghi"}]`,
		v{MultiStringKeyMap{}},
		e{`vom: mismatched quotes`}},
	{
		"MultiStringMismatchedQuotes",
		`["map[[2]string]string",{"[\"abc\",\"def\\\"]":"ghi"}]`,
		v{MultiStringKeyMap{}},
		e{`vom: mismatched quotes`}},
}
