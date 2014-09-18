package vom

import (
	"reflect"
	"testing"

	"veyron.io/veyron/veyron/runtimes/google/lib/reflectutil"
	"veyron.io/veyron/veyron2/wiretype"
)

func compareRTInfo(a, b *rtInfo) bool {
	acopy := copyAndClearRTInfo(a)
	bcopy := copyAndClearRTInfo(b)
	return reflectutil.SharingDeepEqual(acopy, bcopy)
}

// As a convenience for testing we clear out fields that are annoying to specify
// in composite literals before doing the comparison.
func copyAndClearRTInfo(rti *rtInfo) *rtInfo {
	if rti == nil {
		return nil
	}
	res := *rti
	res.rt = nil
	res.zero = nil
	res.elem = copyAndClearRTInfo(rti.elem)
	res.key = copyAndClearRTInfo(rti.key)
	res.numMethods = 0 // TODO(bprosnitz) Remove this line and handle methods correctly in all cases
	if len(rti.fields) == 0 {
		res.fields = nil
	} else {
		res.fields = make([]*rtInfoField, len(rti.fields))
		for fx, field := range rti.fields {
			fcopy := *field
			fcopy.info = copyAndClearRTInfo(field.info)
			res.fields[fx] = &fcopy
		}
	}
	res.fieldsByName = nil
	res.customBinary = copyAndClearCustomCoder(res.customBinary)
	res.customJSON = copyAndClearCustomCoder(res.customJSON)
	return &res
}

func copyAndClearCustomCoder(coder *customCoder) *customCoder {
	if coder == nil {
		return nil
	}
	res := *coder
	res.rti = copyAndClearRTInfo(res.rti)
	res.encFunc = reflect.Value{}
	res.decFunc = reflect.Value{}
	res.rcvrConv = nil
	return &res
}

func TestRTInfo(t *testing.T) {
	for _, test := range rtInfoTests {
		rti, rterr := lookupRTInfo(TypeOf(test.value))
		hasRE, err := matchErrorRE(rterr, test.errRE)
		if err != nil {
			t.Errorf("lookupRTInfo(%T) %v", test.value, err)
			continue
		}
		if hasRE {
			continue
		}

		if !compareRTInfo(rti, test.rti) {
			t.Errorf("lookupRTInfo(%T) mismatch\ngot  %+v\nwant %+v", test.value, rti, test.rti)
		}
	}
}

type VomCoder int

func (vc VomCoder) VomEncode() (bool, error) {
	return false, nil
}
func (vc *VomCoder) VomDecode(s bool) error {
	return nil
}

type VomCoderPtr int

func (vc VomCoderPtr) VomEncode() (*bool, error) {
	return nil, nil
}
func (vc *VomCoderPtr) VomDecode(b *bool) error {
	return nil
}

type GobCoder int

func (gc GobCoder) GobEncode() ([]byte, error) {
	return nil, nil
}
func (gc *GobCoder) GobDecode(buf []byte) error {
	return nil
}

type TwoMethods interface {
	One()
	Two()
}

type StructWithInterfaces struct {
	A interface{}
	B TwoMethods
}

var (
	rtibool      = &rtInfo{name: "bool", kind: typeKindBool, id: wiretype.TypeIDBool}
	rtistring    = &rtInfo{name: "string", kind: typeKindString, id: wiretype.TypeIDString}
	rtibyteslice = &rtInfo{name: "[]byte", kind: typeKindByteSlice, id: wiretype.TypeIDByteSlice, elem: rtiuint8}

	rtiuint    = &rtInfo{name: "uint", kind: typeKindUint, id: wiretype.TypeIDUint}
	rtiuint8   = &rtInfo{name: "byte", kind: typeKindByte, id: wiretype.TypeIDUint8, suffix: "8"}
	rtiuint16  = &rtInfo{name: "uint16", kind: typeKindUint, id: wiretype.TypeIDUint16, suffix: "16"}
	rtiuint32  = &rtInfo{name: "uint32", kind: typeKindUint, id: wiretype.TypeIDUint32, suffix: "32"}
	rtiuint64  = &rtInfo{name: "uint64", kind: typeKindUint, id: wiretype.TypeIDUint64, suffix: "64"}
	rtiuintptr = &rtInfo{name: "uintptr", kind: typeKindUint, id: wiretype.TypeIDUintptr, suffix: "ptr"}

	rtiint   = &rtInfo{name: "int", kind: typeKindInt, id: wiretype.TypeIDInt}
	rtiint8  = &rtInfo{name: "int8", kind: typeKindInt, id: wiretype.TypeIDInt8, suffix: "8"}
	rtiint16 = &rtInfo{name: "int16", kind: typeKindInt, id: wiretype.TypeIDInt16, suffix: "16"}
	rtiint32 = &rtInfo{name: "int32", kind: typeKindInt, id: wiretype.TypeIDInt32, suffix: "32"}
	rtiint64 = &rtInfo{name: "int64", kind: typeKindInt, id: wiretype.TypeIDInt64, suffix: "64"}

	rtifloat32 = &rtInfo{name: "float32", kind: typeKindFloat, id: wiretype.TypeIDFloat32, suffix: "32"}
	rtifloat64 = &rtInfo{name: "float64", kind: typeKindFloat, id: wiretype.TypeIDFloat64, suffix: "64"}

	rtifieldauint = &rtInfoField{"A", 0, rtiuint}

	complex64Fields = []*rtInfoField{
		&rtInfoField{"R", 0, rtifloat32}, &rtInfoField{"I", 1, rtifloat32},
	}
	complex128Fields = []*rtInfoField{
		&rtInfoField{"R", 0, rtifloat64}, &rtInfoField{"I", 1, rtifloat64},
	}

	rtiboolptr  = &rtInfo{name: "*bool", kind: typeKindPtr, elem: rtibool, numStars: 1}
	rtiboolptr2 = &rtInfo{name: "**bool", kind: typeKindPtr, elem: rtiboolptr, numStars: 2}
	rtiboolptr3 = &rtInfo{name: "***bool", kind: typeKindPtr, elem: rtiboolptr2, numStars: 3}

	rtivomcoder    = &rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.VomCoder", snames: s{"VomCoder", "vom.VomCoder", "veyron.io/veyron/veyron2/vom.VomCoder"}, kind: typeKindCustom, customBinary: &customCoder{rti: rtibool}, customJSON: &customCoder{rti: rtibool}, numMethods: 1}
	rtivomcoderptr = &rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.VomCoderPtr", snames: s{"VomCoderPtr", "vom.VomCoderPtr", "veyron.io/veyron/veyron2/vom.VomCoderPtr"}, kind: typeKindCustom, customBinary: &customCoder{rti: rtiboolptr}, customJSON: &customCoder{rti: rtiboolptr}, numStarsBinary: 1, numStarsJSON: 1, numMethods: 1}
	rtigobcoder    = &rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.GobCoder", snames: s{"GobCoder", "vom.GobCoder", "veyron.io/veyron/veyron2/vom.GobCoder"}, kind: typeKindInt, id: wiretype.TypeIDInt, customBinary: &customCoder{rti: rtibyteslice}, numMethods: 1}

	rtiiface    = &rtInfo{name: "interface", kind: typeKindInterface, id: wiretype.TypeIDInterface}
	rti2methods = &rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.TwoMethods", snames: s{"TwoMethods", "vom.TwoMethods", "veyron.io/veyron/veyron2/vom.TwoMethods"}, kind: typeKindInterface, id: wiretype.TypeIDInterface, numMethods: 2}
)

type s []string

var rtInfoTests = []struct {
	value interface{}
	rti   *rtInfo
	errRE string
}{
	// Primitives
	{bool(false), rtibool, ""},
	{string(""), rtistring, ""},
	{[]byte(""), rtibyteslice, ""},

	{uint(0), rtiuint, ""},
	{uint8(0), rtiuint8, ""},
	{uint16(0), rtiuint16, ""},
	{uint32(0), rtiuint32, ""},
	{uint64(0), rtiuint64, ""},
	{uintptr(0), rtiuintptr, ""},

	{int(0), rtiint, ""},
	{int8(0), rtiint8, ""},
	{int16(0), rtiint16, ""},
	{int32(0), rtiint32, ""},
	{int64(0), rtiint64, ""},

	{float32(0), rtifloat32, ""},
	{float64(0), rtifloat64, ""},

	// Named primitives
	{Bool(false),
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.Bool", snames: s{"Bool", "vom.Bool", "veyron.io/veyron/veyron2/vom.Bool"}, kind: typeKindBool, id: wiretype.TypeIDBool},
		""},
	{String(""),
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.String", snames: s{"String", "vom.String", "veyron.io/veyron/veyron2/vom.String"}, kind: typeKindString, id: wiretype.TypeIDString},
		""},
	{ByteSlice(""),
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.ByteSlice", snames: s{"ByteSlice", "vom.ByteSlice", "veyron.io/veyron/veyron2/vom.ByteSlice"}, kind: typeKindByteSlice, id: wiretype.TypeIDByteSlice, elem: rtiuint8},
		""},

	{Uint(0),
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.Uint", snames: s{"Uint", "vom.Uint", "veyron.io/veyron/veyron2/vom.Uint"}, kind: typeKindUint, id: wiretype.TypeIDUint},
		""},
	{Uint8(0),
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.Uint8", snames: s{"Uint8", "vom.Uint8", "veyron.io/veyron/veyron2/vom.Uint8"}, kind: typeKindByte, id: wiretype.TypeIDUint8, suffix: "8"},
		""},
	{Uint16(0),
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.Uint16", snames: s{"Uint16", "vom.Uint16", "veyron.io/veyron/veyron2/vom.Uint16"}, kind: typeKindUint, id: wiretype.TypeIDUint16, suffix: "16"},
		""},
	{Uint32(0),
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.Uint32", snames: s{"Uint32", "vom.Uint32", "veyron.io/veyron/veyron2/vom.Uint32"}, kind: typeKindUint, id: wiretype.TypeIDUint32, suffix: "32"},
		""},
	{Uint64(0),
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.Uint64", snames: s{"Uint64", "vom.Uint64", "veyron.io/veyron/veyron2/vom.Uint64"}, kind: typeKindUint, id: wiretype.TypeIDUint64, suffix: "64"},
		""},
	{Uintptr(0),
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.Uintptr", snames: s{"Uintptr", "vom.Uintptr", "veyron.io/veyron/veyron2/vom.Uintptr"}, kind: typeKindUint, id: wiretype.TypeIDUintptr, suffix: "ptr"},
		""},

	{Int(0),
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.Int", snames: s{"Int", "vom.Int", "veyron.io/veyron/veyron2/vom.Int"}, kind: typeKindInt, id: wiretype.TypeIDInt},
		""},
	{Int8(0),
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.Int8", snames: s{"Int8", "vom.Int8", "veyron.io/veyron/veyron2/vom.Int8"}, kind: typeKindInt, id: wiretype.TypeIDInt8, suffix: "8"},
		""},
	{Int16(0),
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.Int16", snames: s{"Int16", "vom.Int16", "veyron.io/veyron/veyron2/vom.Int16"}, kind: typeKindInt, id: wiretype.TypeIDInt16, suffix: "16"},
		""},
	{Int32(0),
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.Int32", snames: s{"Int32", "vom.Int32", "veyron.io/veyron/veyron2/vom.Int32"}, kind: typeKindInt, id: wiretype.TypeIDInt32, suffix: "32"},
		""},
	{Int64(0),
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.Int64", snames: s{"Int64", "vom.Int64", "veyron.io/veyron/veyron2/vom.Int64"}, kind: typeKindInt, id: wiretype.TypeIDInt64, suffix: "64"},
		""},

	{Float32(0),
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.Float32", snames: s{"Float32", "vom.Float32", "veyron.io/veyron/veyron2/vom.Float32"}, kind: typeKindFloat, id: wiretype.TypeIDFloat32, suffix: "32"},
		""},
	{Float64(0),
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.Float64", snames: s{"Float64", "vom.Float64", "veyron.io/veyron/veyron2/vom.Float64"}, kind: typeKindFloat, id: wiretype.TypeIDFloat64, suffix: "64"},
		""},

	// Unnnamed composites
	{[]bool{},
		&rtInfo{name: "[]bool", kind: typeKindSlice, elem: rtibool},
		""},
	{[2]bool{},
		&rtInfo{name: "[2]bool", kind: typeKindArray, elem: rtibool, len: 2},
		""},
	{map[uint]string{},
		&rtInfo{name: "map[uint]string", kind: typeKindMap, key: rtiuint, elem: rtistring},
		""},
	{struct{ A uint }{},
		&rtInfo{name: "struct{A uint}", kind: typeKindStruct, fields: []*rtInfoField{rtifieldauint}},
		""},
	{(*bool)(nil),
		&rtInfo{name: "*bool", kind: typeKindPtr, elem: rtibool, numStars: 1},
		""},

	// Named composites
	{BoolSlice{},
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.BoolSlice", snames: s{"BoolSlice", "vom.BoolSlice", "veyron.io/veyron/veyron2/vom.BoolSlice"}, kind: typeKindSlice, elem: rtibool},
		""},
	{BoolArray2{},
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.BoolArray2", snames: s{"BoolArray2", "vom.BoolArray2", "veyron.io/veyron/veyron2/vom.BoolArray2"}, kind: typeKindArray, elem: rtibool, len: 2},
		""},
	{UintStringMap{},
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.UintStringMap", snames: s{"UintStringMap", "vom.UintStringMap", "veyron.io/veyron/veyron2/vom.UintStringMap"}, kind: typeKindMap, key: rtiuint, elem: rtistring},
		""},
	{StructA{},
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.StructA", snames: s{"StructA", "vom.StructA", "veyron.io/veyron/veyron2/vom.StructA"}, kind: typeKindStruct, fields: []*rtInfoField{rtifieldauint}},
		""},
	{BoolPtr(nil),
		&rtInfo{isNamed: true, name: "veyron.io/veyron/veyron2/vom.BoolPtr", snames: s{"BoolPtr", "vom.BoolPtr", "veyron.io/veyron/veyron2/vom.BoolPtr"}, kind: typeKindPtr, elem: rtibool, numStars: 1},
		""},

	// Complex
	{complex64(0),
		&rtInfo{
			isNamed: true, name: "complex64", snames: s{"complex64"}, tags: s{"complex64"}, kind: typeKindCustom,
			customBinary: &customCoder{
				rti: &rtInfo{isNamed: true, name: "complex64", tags: s{"complex64"}, kind: typeKindStruct, fields: complex64Fields},
			},
			customJSON: &customCoder{
				rti: &rtInfo{isNamed: true, name: "complex64", tags: s{"complex64"}, kind: typeKindStruct, fields: complex64Fields},
			},
		},
		""},
	{Complex64(0),
		&rtInfo{
			isNamed: true, name: "veyron.io/veyron/veyron2/vom.Complex64", snames: s{"Complex64", "vom.Complex64", "veyron.io/veyron/veyron2/vom.Complex64"}, tags: s{"complex64"}, kind: typeKindCustom,
			customBinary: &customCoder{
				rti: &rtInfo{isNamed: true, name: "complex64", tags: s{"complex64"}, kind: typeKindStruct, fields: complex64Fields},
			},
			customJSON: &customCoder{
				rti: &rtInfo{isNamed: true, name: "complex64", tags: s{"complex64"}, kind: typeKindStruct, fields: complex64Fields},
			},
		},
		""},
	{complex128(0),
		&rtInfo{
			isNamed: true, name: "complex128", snames: s{"complex128"}, tags: s{"complex128"}, kind: typeKindCustom,
			customBinary: &customCoder{
				rti: &rtInfo{isNamed: true, name: "complex128", tags: s{"complex128"}, kind: typeKindStruct, fields: complex128Fields},
			},
			customJSON: &customCoder{
				rti: &rtInfo{isNamed: true, name: "complex128", tags: s{"complex128"}, kind: typeKindStruct, fields: complex128Fields},
			},
		},
		""},
	{Complex128(0),
		&rtInfo{
			isNamed: true, name: "veyron.io/veyron/veyron2/vom.Complex128", snames: s{"Complex128", "vom.Complex128", "veyron.io/veyron/veyron2/vom.Complex128"}, tags: s{"complex128"}, kind: typeKindCustom,
			customBinary: &customCoder{
				rti: &rtInfo{isNamed: true, name: "complex128", tags: s{"complex128"}, kind: typeKindStruct, fields: complex128Fields},
			},
			customJSON: &customCoder{
				rti: &rtInfo{isNamed: true, name: "complex128", tags: s{"complex128"}, kind: typeKindStruct, fields: complex128Fields},
			},
		},
		""},

	// Pointers
	{(*bool)(nil), rtiboolptr, ""},
	{(**bool)(nil), rtiboolptr2, ""},
	{(***bool)(nil), rtiboolptr3, ""},

	// Custom VomEncode / VomDecode methods.
	{VomCoder(0), rtivomcoder, ""},
	{(*VomCoder)(nil),
		&rtInfo{name: "*veyron.io/veyron/veyron2/vom.VomCoder", kind: typeKindPtr, elem: rtivomcoder, numStars: 1, numMethods: 2},
		""},
	{VomCoderPtr(0), rtivomcoderptr, ""},
	{(*VomCoderPtr)(nil),
		&rtInfo{name: "*veyron.io/veyron/veyron2/vom.VomCoderPtr", kind: typeKindPtr, elem: rtivomcoderptr, numStars: 1, numStarsBinary: 1, numStarsJSON: 1, numMethods: 2},
		""},
	{(**VomCoderPtr)(nil),
		&rtInfo{
			name: "**veyron.io/veyron/veyron2/vom.VomCoderPtr", kind: typeKindPtr, numStars: 2, numStarsBinary: 1, numStarsJSON: 1,
			elem: &rtInfo{name: "*veyron.io/veyron/veyron2/vom.VomCoderPtr", kind: typeKindPtr, numStars: 1, numStarsBinary: 1, numStarsJSON: 1, numMethods: 2, elem: rtivomcoderptr}},
		""},
	{GobCoder(0), rtigobcoder, ""},

	// Interfaces
	{(*interface{})(nil),
		&rtInfo{name: "*interface", kind: typeKindPtr, numStars: 1, elem: rtiiface},
		""},
	{(*TwoMethods)(nil),
		&rtInfo{name: "*veyron.io/veyron/veyron2/vom.TwoMethods", kind: typeKindPtr, numStars: 1, elem: rti2methods},
		""},
	{StructWithInterfaces{},
		&rtInfo{
			isNamed: true,
			name:    "veyron.io/veyron/veyron2/vom.StructWithInterfaces",
			snames:  s{"StructWithInterfaces", "vom.StructWithInterfaces", "veyron.io/veyron/veyron2/vom.StructWithInterfaces"},
			kind:    typeKindStruct,
			fields: []*rtInfoField{
				{name: "A", index: 0, info: rtiiface},
				{name: "B", index: 1, info: rti2methods},
			},
		},
		""},
	{struct {
		A interface{}
		B TwoMethods
	}{},
		&rtInfo{
			name: "struct{A interface;B veyron.io/veyron/veyron2/vom.TwoMethods}",
			kind: typeKindStruct,
			fields: []*rtInfoField{
				{name: "A", index: 0, info: rtiiface},
				{name: "B", index: 1, info: rti2methods},
			},
		},
		""},
}
