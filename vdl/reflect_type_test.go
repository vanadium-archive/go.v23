package vdl

import (
	"fmt"
	"reflect"
	"testing"
	"unsafe"
)

// Tests of TypeFromReflect success.
type rtTest struct {
	rt reflect.Type
	t  *Type
}

// rtKeyTests contains types that may be used as map keys.
var rtKeyTests = []rtTest{
	// Unnamed scalars
	{reflect.TypeOf((*interface{})(nil)).Elem(), AnyType},
	{reflect.TypeOf((*Type)(nil)), TypeValType},
	{reflect.TypeOf(bool(false)), BoolType},
	{reflect.TypeOf(uint8(0)), ByteType},
	{reflect.TypeOf(uint16(0)), Uint16Type},
	{reflect.TypeOf(uint32(0)), Uint32Type},
	{reflect.TypeOf(uint64(0)), Uint64Type},
	{reflect.TypeOf(uint(0)), testUintType()},
	{reflect.TypeOf(uintptr(0)), testUintptrType()},
	{reflect.TypeOf(int8(0)), Int16Type}, // there is no Int8Type
	{reflect.TypeOf(int16(0)), Int16Type},
	{reflect.TypeOf(int32(0)), Int32Type},
	{reflect.TypeOf(int64(0)), Int64Type},
	{reflect.TypeOf(int(0)), testIntType()},
	{reflect.TypeOf(float32(0)), Float32Type},
	{reflect.TypeOf(float64(0)), Float64Type},
	{reflect.TypeOf(complex64(0)), Complex64Type},
	{reflect.TypeOf(complex128(0)), Complex128Type},
	{reflect.TypeOf(string("")), StringType},
	// Named scalars
	{reflect.TypeOf((*nInterface)(nil)).Elem(), AnyType},
	{reflect.TypeOf(nType(nil)), TypeValType},
	{reflect.TypeOf(nBool(false)), rtN("Bool", BoolType)},
	{reflect.TypeOf(nUint8(0)), rtN("Uint8", ByteType)},
	{reflect.TypeOf(nUint16(0)), rtN("Uint16", Uint16Type)},
	{reflect.TypeOf(nUint32(0)), rtN("Uint32", Uint32Type)},
	{reflect.TypeOf(nUint64(0)), rtN("Uint64", Uint64Type)},
	{reflect.TypeOf(nUint(0)), rtN("Uint", testUintType())},
	{reflect.TypeOf(nUintptr(0)), rtN("Uintptr", testUintptrType())},
	{reflect.TypeOf(nInt8(0)), rtN("Int8", Int16Type)},
	{reflect.TypeOf(nInt16(0)), rtN("Int16", Int16Type)},
	{reflect.TypeOf(nInt32(0)), rtN("Int32", Int32Type)},
	{reflect.TypeOf(nInt64(0)), rtN("Int64", Int64Type)},
	{reflect.TypeOf(nInt(0)), rtN("Int", testIntType())},
	{reflect.TypeOf(nFloat32(0)), rtN("Float32", Float32Type)},
	{reflect.TypeOf(nFloat64(0)), rtN("Float64", Float64Type)},
	{reflect.TypeOf(nComplex64(0)), rtN("Complex64", Complex64Type)},
	{reflect.TypeOf(nComplex128(0)), rtN("Complex128", Complex128Type)},
	{reflect.TypeOf(nString("")), rtN("String", StringType)},
	// Unnamed arrays
	{reflect.TypeOf([3]interface{}{}), ArrayType(3, AnyType)},
	{reflect.TypeOf([3]*Type{}), ArrayType(3, TypeValType)},
	{reflect.TypeOf([3]bool{}), ArrayType(3, BoolType)},
	{reflect.TypeOf([3]uint8{}), ArrayType(3, ByteType)},
	{reflect.TypeOf([3]uint16{}), ArrayType(3, Uint16Type)},
	{reflect.TypeOf([3]uint32{}), ArrayType(3, Uint32Type)},
	{reflect.TypeOf([3]uint64{}), ArrayType(3, Uint64Type)},
	{reflect.TypeOf([3]uint{}), ArrayType(3, testUintType())},
	{reflect.TypeOf([3]uintptr{}), ArrayType(3, testUintptrType())},
	{reflect.TypeOf([3]int8{}), ArrayType(3, Int16Type)},
	{reflect.TypeOf([3]int16{}), ArrayType(3, Int16Type)},
	{reflect.TypeOf([3]int32{}), ArrayType(3, Int32Type)},
	{reflect.TypeOf([3]int64{}), ArrayType(3, Int64Type)},
	{reflect.TypeOf([3]int{}), ArrayType(3, testIntType())},
	{reflect.TypeOf([3]float32{}), ArrayType(3, Float32Type)},
	{reflect.TypeOf([3]float64{}), ArrayType(3, Float64Type)},
	{reflect.TypeOf([3]complex64{}), ArrayType(3, Complex64Type)},
	{reflect.TypeOf([3]complex128{}), ArrayType(3, Complex128Type)},
	{reflect.TypeOf([3]string{}), ArrayType(3, StringType)},
	// Named arrays
	{reflect.TypeOf(nArray3Interface{}), rtNArray("Interface", AnyType)},
	{reflect.TypeOf(nArray3TypeVal{}), rtNArray("TypeVal", TypeValType)},
	{reflect.TypeOf(nArray3Bool{}), rtNArray("Bool", BoolType)},
	{reflect.TypeOf(nArray3Uint8{}), rtNArray("Uint8", ByteType)},
	{reflect.TypeOf(nArray3Uint16{}), rtNArray("Uint16", Uint16Type)},
	{reflect.TypeOf(nArray3Uint32{}), rtNArray("Uint32", Uint32Type)},
	{reflect.TypeOf(nArray3Uint64{}), rtNArray("Uint64", Uint64Type)},
	{reflect.TypeOf(nArray3Uint{}), rtNArray("Uint", testUintType())},
	{reflect.TypeOf(nArray3Uintptr{}), rtNArray("Uintptr", testUintptrType())},
	{reflect.TypeOf(nArray3Int8{}), rtNArray("Int8", Int16Type)},
	{reflect.TypeOf(nArray3Int16{}), rtNArray("Int16", Int16Type)},
	{reflect.TypeOf(nArray3Int32{}), rtNArray("Int32", Int32Type)},
	{reflect.TypeOf(nArray3Int64{}), rtNArray("Int64", Int64Type)},
	{reflect.TypeOf(nArray3Int{}), rtNArray("Int", testIntType())},
	{reflect.TypeOf(nArray3Float32{}), rtNArray("Float32", Float32Type)},
	{reflect.TypeOf(nArray3Float64{}), rtNArray("Float64", Float64Type)},
	{reflect.TypeOf(nArray3Complex64{}), rtNArray("Complex64", Complex64Type)},
	{reflect.TypeOf(nArray3Complex128{}), rtNArray("Complex128", Complex128Type)},
	{reflect.TypeOf(nArray3String{}), rtNArray("String", StringType)},
	// Unnamed structs
	{reflect.TypeOf(struct{ X interface{} }{}), StructType(StructField{"X", AnyType})},
	{reflect.TypeOf(struct{ X *Type }{}), StructType(StructField{"X", TypeValType})},
	{reflect.TypeOf(struct{ X bool }{}), StructType(StructField{"X", BoolType})},
	{reflect.TypeOf(struct{ X uint8 }{}), StructType(StructField{"X", ByteType})},
	{reflect.TypeOf(struct{ X uint16 }{}), StructType(StructField{"X", Uint16Type})},
	{reflect.TypeOf(struct{ X uint32 }{}), StructType(StructField{"X", Uint32Type})},
	{reflect.TypeOf(struct{ X uint64 }{}), StructType(StructField{"X", Uint64Type})},
	{reflect.TypeOf(struct{ X uint }{}), StructType(StructField{"X", testUintType()})},
	{reflect.TypeOf(struct{ X uintptr }{}), StructType(StructField{"X", testUintptrType()})},
	{reflect.TypeOf(struct{ X int8 }{}), StructType(StructField{"X", Int16Type})},
	{reflect.TypeOf(struct{ X int16 }{}), StructType(StructField{"X", Int16Type})},
	{reflect.TypeOf(struct{ X int32 }{}), StructType(StructField{"X", Int32Type})},
	{reflect.TypeOf(struct{ X int64 }{}), StructType(StructField{"X", Int64Type})},
	{reflect.TypeOf(struct{ X int }{}), StructType(StructField{"X", testIntType()})},
	{reflect.TypeOf(struct{ X float32 }{}), StructType(StructField{"X", Float32Type})},
	{reflect.TypeOf(struct{ X float64 }{}), StructType(StructField{"X", Float64Type})},
	{reflect.TypeOf(struct{ X complex64 }{}), StructType(StructField{"X", Complex64Type})},
	{reflect.TypeOf(struct{ X complex128 }{}), StructType(StructField{"X", Complex128Type})},
	{reflect.TypeOf(struct{ X string }{}), StructType(StructField{"X", StringType})},
	// Named structs
	{reflect.TypeOf(nStructInterface{}), rtNStruct("Interface", AnyType)},
	{reflect.TypeOf(nStructTypeVal{}), rtNStruct("TypeVal", TypeValType)},
	{reflect.TypeOf(nStructBool{}), rtNStruct("Bool", BoolType)},
	{reflect.TypeOf(nStructUint8{}), rtNStruct("Uint8", ByteType)},
	{reflect.TypeOf(nStructUint16{}), rtNStruct("Uint16", Uint16Type)},
	{reflect.TypeOf(nStructUint32{}), rtNStruct("Uint32", Uint32Type)},
	{reflect.TypeOf(nStructUint64{}), rtNStruct("Uint64", Uint64Type)},
	{reflect.TypeOf(nStructUint{}), rtNStruct("Uint", testUintType())},
	{reflect.TypeOf(nStructUintptr{}), rtNStruct("Uintptr", testUintptrType())},
	{reflect.TypeOf(nStructInt8{}), rtNStruct("Int8", Int16Type)},
	{reflect.TypeOf(nStructInt16{}), rtNStruct("Int16", Int16Type)},
	{reflect.TypeOf(nStructInt32{}), rtNStruct("Int32", Int32Type)},
	{reflect.TypeOf(nStructInt64{}), rtNStruct("Int64", Int64Type)},
	{reflect.TypeOf(nStructInt{}), rtNStruct("Int", testIntType())},
	{reflect.TypeOf(nStructFloat32{}), rtNStruct("Float32", Float32Type)},
	{reflect.TypeOf(nStructFloat64{}), rtNStruct("Float64", Float64Type)},
	{reflect.TypeOf(nStructComplex64{}), rtNStruct("Complex64", Complex64Type)},
	{reflect.TypeOf(nStructComplex128{}), rtNStruct("Complex128", Complex128Type)},
	{reflect.TypeOf(nStructString{}), rtNStruct("String", StringType)},
}

// rtNonKeyTests contains types that may not be used as map keys.
var rtNonKeyTests = []rtTest{
	// Unnamed slices
	{reflect.TypeOf([]interface{}{}), ListType(AnyType)},
	{reflect.TypeOf([]*Type{}), ListType(TypeValType)},
	{reflect.TypeOf([]bool{}), ListType(BoolType)},
	{reflect.TypeOf([]uint8{}), ListType(ByteType)},
	{reflect.TypeOf([]uint16{}), ListType(Uint16Type)},
	{reflect.TypeOf([]uint32{}), ListType(Uint32Type)},
	{reflect.TypeOf([]uint64{}), ListType(Uint64Type)},
	{reflect.TypeOf([]uint{}), ListType(testUintType())},
	{reflect.TypeOf([]uintptr{}), ListType(testUintptrType())},
	{reflect.TypeOf([]int8{}), ListType(Int16Type)},
	{reflect.TypeOf([]int16{}), ListType(Int16Type)},
	{reflect.TypeOf([]int32{}), ListType(Int32Type)},
	{reflect.TypeOf([]int64{}), ListType(Int64Type)},
	{reflect.TypeOf([]int{}), ListType(testIntType())},
	{reflect.TypeOf([]float32{}), ListType(Float32Type)},
	{reflect.TypeOf([]float64{}), ListType(Float64Type)},
	{reflect.TypeOf([]complex64{}), ListType(Complex64Type)},
	{reflect.TypeOf([]complex128{}), ListType(Complex128Type)},
	{reflect.TypeOf([]string{}), ListType(StringType)},
	// Named slices
	{reflect.TypeOf(nSliceInterface{}), rtNSlice("Interface", AnyType)},
	{reflect.TypeOf(nSliceTypeVal{}), rtNSlice("TypeVal", TypeValType)},
	{reflect.TypeOf(nSliceBool{}), rtNSlice("Bool", BoolType)},
	{reflect.TypeOf(nSliceUint8{}), rtNSlice("Uint8", ByteType)},
	{reflect.TypeOf(nSliceUint16{}), rtNSlice("Uint16", Uint16Type)},
	{reflect.TypeOf(nSliceUint32{}), rtNSlice("Uint32", Uint32Type)},
	{reflect.TypeOf(nSliceUint64{}), rtNSlice("Uint64", Uint64Type)},
	{reflect.TypeOf(nSliceUint{}), rtNSlice("Uint", testUintType())},
	{reflect.TypeOf(nSliceUintptr{}), rtNSlice("Uintptr", testUintptrType())},
	{reflect.TypeOf(nSliceInt8{}), rtNSlice("Int8", Int16Type)},
	{reflect.TypeOf(nSliceInt16{}), rtNSlice("Int16", Int16Type)},
	{reflect.TypeOf(nSliceInt32{}), rtNSlice("Int32", Int32Type)},
	{reflect.TypeOf(nSliceInt64{}), rtNSlice("Int64", Int64Type)},
	{reflect.TypeOf(nSliceInt{}), rtNSlice("Int", testIntType())},
	{reflect.TypeOf(nSliceFloat32{}), rtNSlice("Float32", Float32Type)},
	{reflect.TypeOf(nSliceFloat64{}), rtNSlice("Float64", Float64Type)},
	{reflect.TypeOf(nSliceComplex64{}), rtNSlice("Complex64", Complex64Type)},
	{reflect.TypeOf(nSliceComplex128{}), rtNSlice("Complex128", Complex128Type)},
	{reflect.TypeOf(nSliceString{}), rtNSlice("String", StringType)},
	// Unnamed sets
	{reflect.TypeOf(map[interface{}]struct{}{}), rtSet(AnyType)},
	{reflect.TypeOf(map[*Type]struct{}{}), rtSet(TypeValType)},
	{reflect.TypeOf(map[bool]struct{}{}), rtSet(BoolType)},
	{reflect.TypeOf(map[uint8]struct{}{}), rtSet(ByteType)},
	{reflect.TypeOf(map[uint16]struct{}{}), rtSet(Uint16Type)},
	{reflect.TypeOf(map[uint32]struct{}{}), rtSet(Uint32Type)},
	{reflect.TypeOf(map[uint64]struct{}{}), rtSet(Uint64Type)},
	{reflect.TypeOf(map[uint]struct{}{}), rtSet(testUintType())},
	{reflect.TypeOf(map[uintptr]struct{}{}), rtSet(testUintptrType())},
	{reflect.TypeOf(map[int8]struct{}{}), rtSet(Int16Type)},
	{reflect.TypeOf(map[int16]struct{}{}), rtSet(Int16Type)},
	{reflect.TypeOf(map[int32]struct{}{}), rtSet(Int32Type)},
	{reflect.TypeOf(map[int64]struct{}{}), rtSet(Int64Type)},
	{reflect.TypeOf(map[int]struct{}{}), rtSet(testIntType())},
	{reflect.TypeOf(map[float32]struct{}{}), rtSet(Float32Type)},
	{reflect.TypeOf(map[float64]struct{}{}), rtSet(Float64Type)},
	{reflect.TypeOf(map[complex64]struct{}{}), rtSet(Complex64Type)},
	{reflect.TypeOf(map[complex128]struct{}{}), rtSet(Complex128Type)},
	{reflect.TypeOf(map[string]struct{}{}), rtSet(StringType)},
	// Named sets
	{reflect.TypeOf(nSetInterface{}), rtNSet("Interface", AnyType)},
	{reflect.TypeOf(nSetTypeVal{}), rtNSet("TypeVal", TypeValType)},
	{reflect.TypeOf(nSetBool{}), rtNSet("Bool", BoolType)},
	{reflect.TypeOf(nSetUint8{}), rtNSet("Uint8", ByteType)},
	{reflect.TypeOf(nSetUint16{}), rtNSet("Uint16", Uint16Type)},
	{reflect.TypeOf(nSetUint32{}), rtNSet("Uint32", Uint32Type)},
	{reflect.TypeOf(nSetUint64{}), rtNSet("Uint64", Uint64Type)},
	{reflect.TypeOf(nSetUint{}), rtNSet("Uint", testUintType())},
	{reflect.TypeOf(nSetUintptr{}), rtNSet("Uintptr", testUintptrType())},
	{reflect.TypeOf(nSetInt8{}), rtNSet("Int8", Int16Type)},
	{reflect.TypeOf(nSetInt16{}), rtNSet("Int16", Int16Type)},
	{reflect.TypeOf(nSetInt32{}), rtNSet("Int32", Int32Type)},
	{reflect.TypeOf(nSetInt64{}), rtNSet("Int64", Int64Type)},
	{reflect.TypeOf(nSetInt{}), rtNSet("Int", testIntType())},
	{reflect.TypeOf(nSetFloat32{}), rtNSet("Float32", Float32Type)},
	{reflect.TypeOf(nSetFloat64{}), rtNSet("Float64", Float64Type)},
	{reflect.TypeOf(nSetComplex64{}), rtNSet("Complex64", Complex64Type)},
	{reflect.TypeOf(nSetComplex128{}), rtNSet("Complex128", Complex128Type)},
	{reflect.TypeOf(nSetString{}), rtNSet("String", StringType)},
	// Unnamed maps
	{reflect.TypeOf(map[interface{}]interface{}{}), rtMap(AnyType)},
	{reflect.TypeOf(map[*Type]*Type{}), rtMap(TypeValType)},
	{reflect.TypeOf(map[bool]bool{}), rtMap(BoolType)},
	{reflect.TypeOf(map[uint8]uint8{}), rtMap(ByteType)},
	{reflect.TypeOf(map[uint16]uint16{}), rtMap(Uint16Type)},
	{reflect.TypeOf(map[uint32]uint32{}), rtMap(Uint32Type)},
	{reflect.TypeOf(map[uint64]uint64{}), rtMap(Uint64Type)},
	{reflect.TypeOf(map[uint]uint{}), rtMap(testUintType())},
	{reflect.TypeOf(map[uintptr]uintptr{}), rtMap(testUintptrType())},
	{reflect.TypeOf(map[int8]int8{}), rtMap(Int16Type)},
	{reflect.TypeOf(map[int16]int16{}), rtMap(Int16Type)},
	{reflect.TypeOf(map[int32]int32{}), rtMap(Int32Type)},
	{reflect.TypeOf(map[int64]int64{}), rtMap(Int64Type)},
	{reflect.TypeOf(map[int]int{}), rtMap(testIntType())},
	{reflect.TypeOf(map[float32]float32{}), rtMap(Float32Type)},
	{reflect.TypeOf(map[float64]float64{}), rtMap(Float64Type)},
	{reflect.TypeOf(map[complex64]complex64{}), rtMap(Complex64Type)},
	{reflect.TypeOf(map[complex128]complex128{}), rtMap(Complex128Type)},
	{reflect.TypeOf(map[string]string{}), rtMap(StringType)},
	// Named maps
	{reflect.TypeOf(nMapInterface{}), rtNMap("Interface", AnyType)},
	{reflect.TypeOf(nMapTypeVal{}), rtNMap("TypeVal", TypeValType)},
	{reflect.TypeOf(nMapBool{}), rtNMap("Bool", BoolType)},
	{reflect.TypeOf(nMapUint8{}), rtNMap("Uint8", ByteType)},
	{reflect.TypeOf(nMapUint16{}), rtNMap("Uint16", Uint16Type)},
	{reflect.TypeOf(nMapUint32{}), rtNMap("Uint32", Uint32Type)},
	{reflect.TypeOf(nMapUint64{}), rtNMap("Uint64", Uint64Type)},
	{reflect.TypeOf(nMapUint{}), rtNMap("Uint", testUintType())},
	{reflect.TypeOf(nMapUintptr{}), rtNMap("Uintptr", testUintptrType())},
	{reflect.TypeOf(nMapInt8{}), rtNMap("Int8", Int16Type)},
	{reflect.TypeOf(nMapInt16{}), rtNMap("Int16", Int16Type)},
	{reflect.TypeOf(nMapInt32{}), rtNMap("Int32", Int32Type)},
	{reflect.TypeOf(nMapInt64{}), rtNMap("Int64", Int64Type)},
	{reflect.TypeOf(nMapInt{}), rtNMap("Int", testIntType())},
	{reflect.TypeOf(nMapFloat32{}), rtNMap("Float32", Float32Type)},
	{reflect.TypeOf(nMapFloat64{}), rtNMap("Float64", Float64Type)},
	{reflect.TypeOf(nMapComplex64{}), rtNMap("Complex64", Complex64Type)},
	{reflect.TypeOf(nMapComplex128{}), rtNMap("Complex128", Complex128Type)},
	{reflect.TypeOf(nMapString{}), rtNMap("String", StringType)},
	// Recursive types
	{reflect.TypeOf(nRecurseSelf{}), recurseSelfType()},
	{reflect.TypeOf(nRecurseA{}), recurseAType()},
	{reflect.TypeOf(nRecurseB{}), recurseBType()},
}

func testUintType() *Type {
	switch bitlen := 8 * unsafe.Sizeof(uint(0)); bitlen {
	case 32:
		return Uint32Type
	case 64:
		return Uint64Type
	default:
		panic(fmt.Errorf("testUintType unhandled bitlen %d", bitlen))
	}
}

func testUintptrType() *Type {
	switch bitlen := 8 * unsafe.Sizeof(uintptr(0)); bitlen {
	case 32:
		return Uint32Type
	case 64:
		return Uint64Type
	default:
		panic(fmt.Errorf("testUintptrType unhandled bitlen %d", bitlen))
	}
}

func testIntType() *Type {
	switch bitlen := 8 * unsafe.Sizeof(int(0)); bitlen {
	case 32:
		return Int32Type
	case 64:
		return Int64Type
	default:
		panic(fmt.Errorf("testIntType unhandled bitlen %d", bitlen))
	}
}

func rtN(suffix string, base *Type) *Type {
	return NamedType("veyron2/vdl.n"+suffix, base)
}

func rtNArray(suffix string, base *Type) *Type {
	return NamedType("veyron2/vdl.nArray3"+suffix, ArrayType(3, base))
}

func rtNStruct(suffix string, base *Type) *Type {
	return NamedType("veyron2/vdl.nStruct"+suffix, StructType(StructField{"X", base}))
}

func rtNSlice(suffix string, base *Type) *Type {
	return NamedType("veyron2/vdl.nSlice"+suffix, ListType(base))
}

func rtSet(base *Type) *Type {
	return SetType(base)
}

func rtNSet(suffix string, base *Type) *Type {
	return NamedType("veyron2/vdl.nSet"+suffix, rtSet(base))
}

func rtMap(base *Type) *Type {
	return MapType(base, base)
}

func rtNMap(suffix string, base *Type) *Type {
	return NamedType("veyron2/vdl.nMap"+suffix, rtMap(base))
}

func allTests() []rtTest {
	// Start with all keys and non keys
	tests := make([]rtTest, len(rtKeyTests)+len(rtNonKeyTests))
	n := copy(tests, rtKeyTests)
	copy(tests[n:], rtNonKeyTests)
	// Add all types we can generate via reflect; no arrays and structs.
	for _, test := range rtKeyTests {
		switch test.t.Kind() {
		case Any, TypeVal, Nilable:
			tests = append(tests, rtTest{reflect.PtrTo(test.rt), test.t})
		default:
			tests = append(tests, rtTest{reflect.PtrTo(test.rt), NilableType(test.t)})
		}
		tests = append(tests, rtTest{reflect.SliceOf(test.rt), ListType(test.t)})
		tests = append(tests, rtTest{reflect.MapOf(test.rt, test.rt), MapType(test.t, test.t)})
	}
	// Now generate types from everything we have so far, for more complicated subtypes.
	for _, test := range tests {
		switch test.t.Kind() {
		case Any, TypeVal, Nilable:
			tests = append(tests, rtTest{reflect.PtrTo(test.rt), test.t})
		default:
			tests = append(tests, rtTest{reflect.PtrTo(test.rt), NilableType(test.t)})
		}
		tests = append(tests, rtTest{reflect.SliceOf(test.rt), ListType(test.t)})
		for _, key := range rtKeyTests {
			// Only generate maps with valid keys.
			tests = append(tests, rtTest{reflect.MapOf(key.rt, reflect.SliceOf(test.rt)), MapType(key.t, ListType(test.t))})
		}
	}
	return tests
}

func TestTypeFromReflect(t *testing.T) {
	rtCacheEnabled = false
	testTypeFromReflect(t, "no cache")
	rtCacheEnabled = true
	testTypeFromReflect(t, "cache")
	testTypeFromReflect(t, "all cached")
}

func testTypeFromReflect(t *testing.T, prefix string) {
	for _, test := range allTests() {
		got, err := TypeFromReflect(test.rt)
		expectErr(t, err, "", "%s TypeFromReflect(%v)", prefix, test.rt)
		if want := test.t; got != want {
			t.Errorf("%s TypeFromReflect(%v) got type %v, want %v", prefix, test.rt, got, want)
		}
	}
}

// Tests of TypeFromReflect errors.
type rtErrorTest struct {
	rt     reflect.Type
	errstr string
}

var rtErrorTests = []rtErrorTest{
	{reflect.Type(nil), `invalid val.TypeOf(nil)`},
	{reflect.TypeOf(make(chan int64)), `type "chan int64" not supported`},
	{reflect.TypeOf(func() {}), `type "func()" not supported`},
	{reflect.TypeOf(unsafe.Pointer(uintptr(0))), `type "unsafe.Pointer" not supported`},
	{reflect.TypeOf(map[*int64]string{}), `invalid nilable key "?int64"`},
	{reflect.TypeOf(struct{ a int64 }{}), `type "struct { a int64 }" only has unexported fields`},
}

func allErrorTests() []rtErrorTest {
	// Start with base error tests
	tests := make([]rtErrorTest, len(rtErrorTests))
	copy(tests, rtErrorTests)
	// Add some types we can generate via reflect
	for _, test := range rtErrorTests {
		if test.rt != nil {
			tests = append(tests, rtErrorTest{reflect.PtrTo(test.rt), test.errstr})
			tests = append(tests, rtErrorTest{reflect.SliceOf(test.rt), test.errstr})
		}
	}
	// Now generate types from everything we have so far, for more complicated subtypes.
	for _, test := range tests {
		if test.rt != nil {
			tests = append(tests, rtErrorTest{reflect.PtrTo(reflect.PtrTo(test.rt)), test.errstr})
			tests = append(tests, rtErrorTest{reflect.SliceOf(reflect.SliceOf(test.rt)), test.errstr})
		}
	}
	return tests
}

func TestTypeFromReflectError(t *testing.T) {
	for _, test := range allErrorTests() {
		got, err := TypeFromReflect(test.rt)
		expectErr(t, err, test.errstr, "TypeFromReflect(%v)", test.rt)
		if got != nil {
			t.Errorf("TypeFromReflect(%v) got type %v, want nil", test.rt, got)
		}
	}
}
