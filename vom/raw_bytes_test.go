// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"bytes"
	"io"
	"reflect"
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vom/testdata/data81"
	"v.io/v23/vom/testdata/types"
)

type testUint64 uint64

type structTypeObject struct {
	T *vdl.Type
}

type structAny struct {
	X *RawBytes
}

type structAnyAndTypes struct {
	A *vdl.Type
	B *RawBytes
	C *vdl.Type
	D *RawBytes
}

// Test various combinations of having/not having any and type object.
var rawBytesTestCases = []struct {
	name     string
	goValue  interface{}
	rawBytes RawBytes
}{
	{
		name:    "testUint64(99)",
		goValue: testUint64(99),
		rawBytes: RawBytes{
			Version: DefaultVersion,
			Type:    vdl.TypeOf(testUint64(0)),
			Data:    []byte{0x63},
		},
	},
	{
		name:    "typeobject(int32)",
		goValue: vdl.Int32Type,
		rawBytes: RawBytes{
			Version:  DefaultVersion,
			Type:     vdl.TypeOf(vdl.Int32Type),
			RefTypes: []*vdl.Type{vdl.Int32Type},
			Data:     []byte{0x00},
		},
	},
	{
		name:    "structTypeObject{typeobject(int32)}",
		goValue: structTypeObject{vdl.Int32Type},
		rawBytes: RawBytes{
			Version:  DefaultVersion,
			Type:     vdl.TypeOf(structTypeObject{}),
			RefTypes: []*vdl.Type{vdl.Int32Type},
			Data:     []byte{0x00, 0x00, WireCtrlEnd},
		},
	},
	{
		name: `structAnyAndTypes{typeobject(int32), true, typeobject(bool), "abc"}`,
		goValue: structAnyAndTypes{
			vdl.Int32Type,
			&RawBytes{
				Version: DefaultVersion,
				Type:    vdl.BoolType,
				Data:    []byte{0x01},
			},
			vdl.BoolType,
			&RawBytes{
				Version: DefaultVersion,
				Type:    vdl.TypeOf(""),
				Data:    []byte{0x03, 0x61, 0x62, 0x63},
			},
		},
		rawBytes: RawBytes{
			Version:    DefaultVersion,
			Type:       vdl.TypeOf(structAnyAndTypes{}),
			RefTypes:   []*vdl.Type{vdl.Int32Type, vdl.BoolType, vdl.StringType},
			AnyLengths: []uint64{1, 4},
			Data: []byte{
				0x00, 0x00, // A
				0x01, 0x01, 0x00, 0x01, // B
				0x02, 0x01, // C
				0x03, 0x02, 0x01, 0x03, 0x61, 0x62, 0x63, // D
				WireCtrlEnd,
			},
		},
	},
	{
		name:    "large message", // to test that multibyte length is encoded properly
		goValue: makeLargeBytes(1000),
		rawBytes: RawBytes{
			Version: DefaultVersion,
			Type:    vdl.ListType(vdl.ByteType),
			Data:    append([]byte{0xfe, 0x03, 0xe8}, makeLargeBytes(1000)...),
		},
	},
	{
		name:    "*vdl.Value",
		goValue: vdl.ValueOf(uint16(5)),
		rawBytes: RawBytes{
			Version: DefaultVersion,
			Type:    vdl.Uint16Type,
			Data:    []byte{0x05},
		},
	},
	{
		name:    "*vdl.Value - top level any",
		goValue: vdl.ValueOf([]interface{}{uint16(5)}).Index(0),
		rawBytes: RawBytes{
			Version: DefaultVersion,
			Type:    vdl.Uint16Type,
			Data:    []byte{0x05},
		},
	},
	{
		name: "any(nil)",
		goValue: &RawBytes{
			Version: DefaultVersion,
			Type:    vdl.AnyType,
			Data:    []byte{WireCtrlNil},
		},
		rawBytes: RawBytes{
			Version: DefaultVersion,
			Type:    vdl.AnyType,
			Data:    []byte{WireCtrlNil},
		},
	},
}

func makeLargeBytes(size int) []byte {
	b := make([]byte, size)
	for i := range b {
		b[i] = byte(i % 10)
	}
	return b
}

func TestDecodeToRawBytes(t *testing.T) {
	for _, test := range rawBytesTestCases {
		bytes, err := Encode(test.goValue)
		if err != nil {
			t.Fatalf("%s: error in encode %v", test.name, err)
		}
		var rb RawBytes
		if err := Decode(bytes, &rb); err != nil {
			t.Fatalf("%s: error decoding into raw bytes: %v", test.name, err)
		}
		if !reflect.DeepEqual(rb, test.rawBytes) {
			t.Errorf("%s: got %#v, want %#v", test.name, rb, test.rawBytes)
		}
	}
}

func TestEncodeFromRawBytes(t *testing.T) {
	for _, test := range rawBytesTestCases {
		fullBytes, err := Encode(test.goValue)
		if err != nil {
			t.Fatalf("%s: error in encode %v", test.name, err)
		}
		fullBytesFromRaw, err := Encode(&test.rawBytes)
		if err != nil {
			t.Fatalf("%s: error in encode %v", test.name, err)
		}
		if !bytes.Equal(fullBytes, fullBytesFromRaw) {
			t.Errorf("%s: got %x, want %x", test.name, fullBytesFromRaw, fullBytes)
		}
	}
}

// Same as rawBytesTestCases, but wrapped within structAny
var rawBytesWrappedTestCases = []struct {
	name     string
	goValue  interface{}
	rawBytes RawBytes
}{
	{
		name:    "testUint64(99)",
		goValue: testUint64(99),
		rawBytes: RawBytes{
			Version:    DefaultVersion,
			Type:       vdl.TypeOf(testUint64(0)),
			RefTypes:   []*vdl.Type{vdl.TypeOf(testUint64(0))},
			AnyLengths: []uint64{1},
			Data:       []byte{0x63},
		},
	},
	{
		name:    "typeobject(int32)",
		goValue: vdl.Int32Type,
		rawBytes: RawBytes{
			Version:    DefaultVersion,
			Type:       vdl.TypeOf(vdl.Int32Type),
			RefTypes:   []*vdl.Type{vdl.TypeObjectType, vdl.Int32Type},
			AnyLengths: []uint64{1},
			Data:       []byte{0x01},
		},
	},
	{
		name:    "structTypeObject{typeobject(int32)}",
		goValue: structTypeObject{vdl.Int32Type},
		rawBytes: RawBytes{
			Version:    DefaultVersion,
			Type:       vdl.TypeOf(structTypeObject{}),
			RefTypes:   []*vdl.Type{vdl.TypeOf(structTypeObject{}), vdl.Int32Type},
			AnyLengths: []uint64{3},
			Data:       []byte{0x00, 0x01, 0xe1},
		},
	},
	{
		name: `structAnyAndTypes{typeobject(int32), true, typeobject(bool), "abc"}`,
		goValue: structAnyAndTypes{
			vdl.Int32Type,
			&RawBytes{
				Version: DefaultVersion,
				Type:    vdl.BoolType,
				Data:    []byte{0x01},
			},
			vdl.BoolType,
			&RawBytes{
				Version: DefaultVersion,
				Type:    vdl.TypeOf(""),
				Data:    []byte{0x03, 0x61, 0x62, 0x63},
			},
		},
		rawBytes: RawBytes{
			Version:    DefaultVersion,
			Type:       vdl.TypeOf(structAnyAndTypes{}),
			RefTypes:   []*vdl.Type{vdl.TypeOf(structAnyAndTypes{}), vdl.Int32Type, vdl.BoolType, vdl.StringType},
			AnyLengths: []uint64{16, 1, 4},
			Data: []byte{
				0x00, 0x01, // A
				0x01, 0x02, 0x01, 0x01, // B
				0x02, 0x02, // C
				0x03, 0x03, 0x02, 0x03, 0x61, 0x62, 0x63, // D
				0xe1,
			},
		},
	},
	{
		name:    "large message", // to test that multibyte length is encoded properly
		goValue: makeLargeBytes(1000),
		rawBytes: RawBytes{
			Version:    DefaultVersion,
			Type:       vdl.ListType(vdl.ByteType),
			RefTypes:   []*vdl.Type{vdl.ListType(vdl.ByteType)},
			AnyLengths: []uint64{0x3eb},
			Data:       append([]byte{0xfe, 0x03, 0xe8}, makeLargeBytes(1000)...),
		},
	},
	{
		name:    "*vdl.Value",
		goValue: vdl.ValueOf(uint16(5)),
		rawBytes: RawBytes{
			Version:    DefaultVersion,
			Type:       vdl.Uint16Type,
			RefTypes:   []*vdl.Type{vdl.Uint16Type},
			AnyLengths: []uint64{1},
			Data:       []byte{0x05},
		},
	},
	{
		name:    "*vdl.Value - top level any",
		goValue: vdl.ValueOf([]interface{}{uint16(5)}).Index(0),
		rawBytes: RawBytes{
			Version:    DefaultVersion,
			Type:       vdl.Uint16Type,
			RefTypes:   []*vdl.Type{vdl.Uint16Type},
			AnyLengths: []uint64{1},
			Data:       []byte{0x05},
		},
	},
}

func TestWrappedRawBytes(t *testing.T) {
	for i, test := range rawBytesWrappedTestCases {
		unwrapped := rawBytesTestCases[i]
		wrappedBytes, err := Encode(structAny{&unwrapped.rawBytes})
		if err != nil {
			t.Fatalf("%s: error in encode %v", test.name, err)
		}
		var any structAny
		if err := Decode(wrappedBytes, &any); err != nil {
			t.Fatalf("%s: error in decode %v", test.name, err)
		}
		if !reflect.DeepEqual(any.X, &test.rawBytes) {
			t.Errorf("%s: got %#v, want %#v", test.name, any.X, &test.rawBytes)
		}
	}
}

func TestEncodeNilRawBytes(t *testing.T) {
	// Top-level
	expectedBytes, err := Encode(vdl.ZeroValue(vdl.AnyType))
	if err != nil {
		t.Fatal(err)
	}
	encodedBytes, err := Encode((*RawBytes)(nil))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encodedBytes, expectedBytes) {
		t.Errorf("encoding nil RawBytes: got %x, expected %x", encodedBytes, expectedBytes)
	}

	// Within an object.
	expectedBytes, err = Encode([]*vdl.Value{vdl.ZeroValue(vdl.AnyType)})
	if err != nil {
		t.Fatal(err)
	}
	encodedBytes, err = Encode([]*RawBytes{nil})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRawBytesDecodeEncode(t *testing.T) {
	versions := []struct {
		Version Version
		Tests   []types.TestCase
	}{
		{Version81, data81.Tests},
	}
	for _, testVersion := range versions {
		for _, test := range testVersion.Tests {
			// Interleaved
			rb := RawBytes{}
			interleavedReader := bytes.NewReader(hex2Bin(t, test.Hex))
			if err := NewDecoder(interleavedReader).Decode(&rb); err != nil {
				t.Errorf("unexpected error decoding %s: %v", test.Name, err)
				continue
			}
			if _, err := interleavedReader.ReadByte(); err != io.EOF {
				t.Errorf("expected EOF, but got %v", err)
				continue
			}

			var out bytes.Buffer
			enc := NewVersionedEncoder(testVersion.Version, &out)
			if err := enc.Encode(&rb); err != nil {
				t.Errorf("unexpected error encoding raw bytes %v in test %s: %v", rb, test.Name, err)
				continue
			}
			if !bytes.Equal(out.Bytes(), hex2Bin(t, test.Hex)) {
				t.Errorf("got bytes: %x but expected %s", out.Bytes(), test.Hex)
			}

			// Split type and value stream.
			rb = RawBytes{}
			typeReader := bytes.NewReader(hex2Bin(t, test.HexVersion+test.HexType))
			typeDec := NewTypeDecoder(typeReader)
			typeDec.Start()
			defer typeDec.Stop()
			valueReader := bytes.NewReader(hex2Bin(t, test.HexVersion+test.HexValue))
			if err := NewDecoderWithTypeDecoder(valueReader, typeDec).Decode(&rb); err != nil {
				t.Errorf("unexpected error decoding %s: %v", test.Name, err)
				continue
			}
			if test.HexType != "" {
				// If HexType is empty, then the type stream will just have the version byte that won't be read, so ignore
				// that case.
				if _, err := typeReader.ReadByte(); err != io.EOF {
					t.Errorf("in type reader expected EOF, but got %v", err)
					continue
				}
			}
			if _, err := valueReader.ReadByte(); err != io.EOF {
				t.Errorf("in value reader expected EOF, but got %v", err)
				continue
			}

			out.Reset()
			var typeOut bytes.Buffer
			typeEnc := NewVersionedTypeEncoder(testVersion.Version, &typeOut)
			enc = NewVersionedEncoderWithTypeEncoder(testVersion.Version, &out, typeEnc)
			if err := enc.Encode(&rb); err != nil {
				t.Errorf("unexpected error encoding raw value %v in test %s: %v", rb, test.Name, err)
				continue
			}
			expectedType := test.HexVersion + test.HexType
			if expectedType == "81" || expectedType == "82" {
				expectedType = ""
			}
			if !bytes.Equal(typeOut.Bytes(), hex2Bin(t, expectedType)) {
				t.Errorf("got type bytes: %x but expected %s", typeOut.Bytes(), expectedType)
			}
			if !bytes.Equal(out.Bytes(), hex2Bin(t, test.HexVersion+test.HexValue)) {
				t.Errorf("got value bytes: %x but expected %s", out.Bytes(), test.HexVersion+test.HexValue)
			}
		}
	}
}

func TestRawBytesToFromValue(t *testing.T) {
	versions := []struct {
		Version Version
		Tests   []types.TestCase
	}{
		{Version81, data81.Tests},
	}
	for _, testVersion := range versions {
		for _, test := range testVersion.Tests {
			rb, err := RawBytesFromValue(test.Value)
			if err != nil {
				t.Fatalf("%v %s: error in RawBytesFromValue %v", testVersion.Version, test.Name, err)
			}
			var vv *vdl.Value
			if err := rb.ToValue(&vv); err != nil {
				t.Fatalf("%v %s: error in rb.ToValue %v", testVersion.Version, test.Name, err)
			}
			if test.Name == "any(nil)" {
				// Skip any(nil)
				// TODO(bprosnitz) any(nil) results in two different nil representations. This shouldn't be the case.
				continue
			}
			if got, want := vv, test.Value; !vdl.EqualValue(got, want) {
				t.Errorf("%v %s: error in converting to and from raw value. got %v, but want %v", testVersion.Version,
					test.Name, got, want)
			}
		}
	}
}

func TestVdlTypeOfRawBytes(t *testing.T) {
	if got, want := vdl.TypeOf(&RawBytes{}), vdl.AnyType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestVdlValueOfRawBytes(t *testing.T) {
	for _, test := range rawBytesTestCases {
		want := vdl.ValueOf(test.goValue)
		got := vdl.ValueOf(test.rawBytes)
		if !vdl.EqualValue(got, want) {
			t.Errorf("vdl.ValueOf(RawBytes) %s: got %v, want %v", test.name, got, want)
		}
	}
}

func TestConvertRawBytes(t *testing.T) {
	for _, test := range rawBytesTestCases {
		var rb *RawBytes
		if err := vdl.Convert(&rb, test.goValue); err != nil {
			t.Errorf("vdl.Convert %#v to RawBytes: %v", test.goValue, err)
		}
		if !reflect.DeepEqual(rb, &test.rawBytes) {
			t.Errorf("vdl.Convert to RawBytes %s: got %v, want %v", test.name, rb, test.rawBytes)
		}
	}
}

type structAnyInterface struct {
	X interface{}
}

func TestConvertRawBytesWrapped(t *testing.T) {
	for _, test := range rawBytesTestCases {
		var any structAny
		if err := vdl.Convert(&any, structAnyInterface{test.goValue}); err != nil {
			t.Errorf("vdl.Convert %#v to RawBytes: %v", structAnyInterface{test.goValue}, err)
		}
		got, want := any, structAny{&test.rawBytes}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("vdl.Convert to RawBytes %s: got %v, want %v", test.name, got, want)
		}
	}
}
