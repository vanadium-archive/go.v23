package vdl

// TODO(toddw): Merge with vdl/{opconst,valconv}/testutil_test.go

import (
	"fmt"
	"strings"
	"testing"
)

// CallAndRecover calls the function f and returns the result of recover().
// This minimizes the scope of the deferred recover, to ensure f is actually the
// function that paniced.
func CallAndRecover(f func()) (result interface{}) {
	defer func() {
		result = recover()
	}()
	f()
	return
}

func expectErr(t *testing.T, err error, wantstr string, format string, args ...interface{}) bool {
	gotstr := fmt.Sprint(err)
	msg := fmt.Sprintf(format, args...)
	if wantstr != "" && !strings.Contains(gotstr, wantstr) {
		t.Errorf(`%s got error %q, want substr %q`, msg, gotstr, wantstr)
		return false
	}
	if wantstr == "" && err != nil {
		t.Errorf(`%s got error %q, want nil`, msg, gotstr)
		return false
	}
	return true
}

func expectPanic(t *testing.T, f func(), wantstr string, format string, args ...interface{}) {
	got := CallAndRecover(f)
	gotstr := fmt.Sprint(got)
	msg := fmt.Sprintf(format, args...)
	if wantstr != "" && !strings.Contains(gotstr, wantstr) {
		t.Errorf(`%s got panic %q, want substr %q`, msg, gotstr, wantstr)
	}
	if wantstr == "" && got != nil {
		t.Errorf(`%s got panic %q, want nil`, msg, gotstr)
	}
}

func expectMismatchedKind(t *testing.T, f func()) {
	expectPanic(t, f, "mismatched kind", "")
}

// Define a bunch of regular Go types used in tests.
type (
	// Scalars
	nInterface  interface{}
	nType       *Type
	nBool       bool
	nUint8      uint8
	nUint16     uint16
	nUint32     uint32
	nUint64     uint64
	nUint       uint
	nUintptr    uintptr
	nInt8       int8
	nInt16      int16
	nInt32      int32
	nInt64      int64
	nInt        int
	nFloat32    float32
	nFloat64    float64
	nComplex64  complex64
	nComplex128 complex128
	nString     string
	// Arrays
	nArray3Interface  [3]nInterface
	nArray3TypeObject [3]*Type
	nArray3Bool       [3]bool
	nArray3Uint8      [3]uint8
	nArray3Uint16     [3]uint16
	nArray3Uint32     [3]uint32
	nArray3Uint64     [3]uint64
	nArray3Uint       [3]uint
	nArray3Uintptr    [3]uintptr
	nArray3Int8       [3]int8
	nArray3Int16      [3]int16
	nArray3Int32      [3]int32
	nArray3Int64      [3]int64
	nArray3Int        [3]int
	nArray3Float32    [3]float32
	nArray3Float64    [3]float64
	nArray3Complex64  [3]complex64
	nArray3Complex128 [3]complex128
	nArray3String     [3]string
	// Structs
	nStructInterface  struct{ X nInterface }
	nStructTypeObject struct{ X *Type }
	nStructBool       struct{ X bool }
	nStructUint8      struct{ X uint8 }
	nStructUint16     struct{ X uint16 }
	nStructUint32     struct{ X uint32 }
	nStructUint64     struct{ X uint64 }
	nStructUint       struct{ X uint }
	nStructUintptr    struct{ X uintptr }
	nStructInt8       struct{ X int8 }
	nStructInt16      struct{ X int16 }
	nStructInt32      struct{ X int32 }
	nStructInt64      struct{ X int64 }
	nStructInt        struct{ X int }
	nStructFloat32    struct{ X float32 }
	nStructFloat64    struct{ X float64 }
	nStructComplex64  struct{ X complex64 }
	nStructComplex128 struct{ X complex128 }
	nStructString     struct{ X string }
	// Slices
	nSliceInterface  []nInterface
	nSliceTypeObject []*Type
	nSliceBool       []bool
	nSliceUint8      []uint8
	nSliceUint16     []uint16
	nSliceUint32     []uint32
	nSliceUint64     []uint64
	nSliceUint       []uint
	nSliceUintptr    []uintptr
	nSliceInt8       []int8
	nSliceInt16      []int16
	nSliceInt32      []int32
	nSliceInt64      []int64
	nSliceInt        []int
	nSliceFloat32    []float32
	nSliceFloat64    []float64
	nSliceComplex64  []complex64
	nSliceComplex128 []complex128
	nSliceString     []string
	// Sets
	nSetInterface  map[nInterface]struct{}
	nSetTypeObject map[*Type]struct{}
	nSetBool       map[bool]struct{}
	nSetUint8      map[uint8]struct{}
	nSetUint16     map[uint16]struct{}
	nSetUint32     map[uint32]struct{}
	nSetUint64     map[uint64]struct{}
	nSetUint       map[uint]struct{}
	nSetUintptr    map[uintptr]struct{}
	nSetInt8       map[int8]struct{}
	nSetInt16      map[int16]struct{}
	nSetInt32      map[int32]struct{}
	nSetInt64      map[int64]struct{}
	nSetInt        map[int]struct{}
	nSetFloat32    map[float32]struct{}
	nSetFloat64    map[float64]struct{}
	nSetComplex64  map[complex64]struct{}
	nSetComplex128 map[complex128]struct{}
	nSetString     map[string]struct{}
	// Maps
	nMapInterface  map[nInterface]nInterface
	nMapTypeObject map[*Type]*Type
	nMapBool       map[bool]bool
	nMapUint8      map[uint8]uint8
	nMapUint16     map[uint16]uint16
	nMapUint32     map[uint32]uint32
	nMapUint64     map[uint64]uint64
	nMapUint       map[uint]uint
	nMapUintptr    map[uintptr]uintptr
	nMapInt8       map[int8]int8
	nMapInt16      map[int16]int16
	nMapInt32      map[int32]int32
	nMapInt64      map[int64]int64
	nMapInt        map[int]int
	nMapFloat32    map[float32]float32
	nMapFloat64    map[float64]float64
	nMapComplex64  map[complex64]complex64
	nMapComplex128 map[complex128]complex128
	nMapString     map[string]string
	// Recursive types
	nRecurseSelf struct{ X []nRecurseSelf }
	nRecurseA    struct{ B []nRecurseB }
	nRecurseB    struct{ A []nRecurseA }

	// Composite types representing sets of numbers.
	nMapUint64Empty    map[nUint64]struct{}
	nMapInt64Empty     map[nUint64]struct{}
	nMapFloat64Empty   map[nUint64]struct{}
	nMapComplex64Empty map[nUint64]struct{}
	nMapUint64Bool     map[nUint64]nBool
	nMapInt64Bool      map[nInt64]nBool
	nMapFloat64Bool    map[nFloat64]nBool
	nMapComplex64Bool  map[nComplex64]nBool
	// Composite types representing sets of strings.
	nMapStringEmpty map[nString]struct{}
	nMapStringBool  map[nString]nBool
	nStructXYZBool  struct{ X, Y, Z nBool }
	nStructWXBool   struct{ W, X nBool }
	// Composite types representing maps of strings to numbers.
	nMapStringUint64    map[nString]nUint64
	nMapStringInt64     map[nString]nInt64
	nMapStringFloat64   map[nString]nFloat64
	nMapStringComplex64 map[nString]nComplex64
	nStructVWXUint64    struct{ V, W, X nUint64 }
	nStructVWXInt64     struct{ V, W, X nInt64 }
	nStructVWXFloat64   struct{ V, W, X nFloat64 }
	nStructVWXComplex64 struct{ V, W, X nComplex64 }
	nStructUVUint64     struct{ U, V nUint64 }
	nStructUVInt64      struct{ U, V nInt64 }
	nStructUVFloat64    struct{ U, V nFloat64 }
	nStructUVComplex64  struct{ U, V nComplex64 }
	// Types that cannot be converted to sets.  We represent sets as
	// map[key]struct{} on the Go side, but don't allow map[key]nEmpty.
	nEmpty           struct{}
	nMapStringnEmpty map[nString]nEmpty
	nStructXYZEmpty  struct{ X, Y, Z struct{} }
	nStructXYZnEmpty struct{ X, Y, Z nEmpty }
)

func recurseSelfType() *Type {
	var builder TypeBuilder
	n := builder.Named("veyron.io/veyron/veyron2/vdl.nRecurseSelf")
	n.AssignBase(builder.Struct().AppendField("X", builder.List().AssignElem(n)))
	builder.Build()
	t, err := n.Built()
	if err != nil {
		panic(err)
	}
	return t
}

func recurseABTypes() [2]*Type {
	var builder TypeBuilder
	a := builder.Named("veyron.io/veyron/veyron2/vdl.nRecurseA")
	b := builder.Named("veyron.io/veyron/veyron2/vdl.nRecurseB")
	a.AssignBase(builder.Struct().AppendField("B", builder.List().AssignElem(b)))
	b.AssignBase(builder.Struct().AppendField("A", builder.List().AssignElem(a)))
	builder.Build()
	aT, err := a.Built()
	if err != nil {
		panic(err)
	}
	bT, err := b.Built()
	if err != nil {
		panic(err)
	}
	return [2]*Type{aT, bT}
}

func recurseAType() *Type { return recurseABTypes()[0] }
func recurseBType() *Type { return recurseABTypes()[1] }

// Special case enum isn't regularly expressible in Go.
type nEnum int

const (
	nEnumA nEnum = iota
	nEnumB
	nEnumC
)

func (x *nEnum) Set(label string) error {
	switch label {
	case "A":
		*x = nEnumA
		return nil
	case "B":
		*x = nEnumB
		return nil
	case "C":
		*x = nEnumC
		return nil
	}
	*x = -1
	return fmt.Errorf("unknown label %q in nEnum", label)
}

func (x nEnum) String() string {
	switch x {
	case nEnumA:
		return "A"
	case nEnumB:
		return "B"
	case nEnumC:
		return "C"
	}
	return ""
}

func (nEnum) __VDLReflect(struct{ Enum struct{ A, B, C string } }) {}

// Special case union isn't regularly expressible in Go.
//   type nUnion union{A bool;B string;C int32}
type (
	nUnion interface {
		Index() int
		Name() string
		__VDLReflect(__nUnionReflect)
	}
	nUnionA struct{ Value bool }
	nUnionB struct{ Value string }
	nUnionC struct{ Value int32 }

	__nUnionReflect struct {
		Type  nUnion
		Union struct {
			A nUnionA
			B nUnionB
			C nUnionC
		}
	}
)

func (nUnionA) Name() string                 { return "A" }
func (nUnionA) Index() int                   { return 0 }
func (nUnionA) __VDLReflect(__nUnionReflect) {}
func (nUnionB) Name() string                 { return "B" }
func (nUnionB) Index() int                   { return 1 }
func (nUnionB) __VDLReflect(__nUnionReflect) {}
func (nUnionC) Name() string                 { return "C" }
func (nUnionC) Index() int                   { return 2 }
func (nUnionC) __VDLReflect(__nUnionReflect) {}

// Define a bunch of *Type types used in tests.
var (
	// Named scalar types
	boolTypeN       = NamedType("nBool", BoolType)
	nByteType       = NamedType("nByte", ByteType)
	uint16TypeN     = NamedType("nUint16", Uint16Type)
	uint32TypeN     = NamedType("nUint32", Uint32Type)
	uint64TypeN     = NamedType("nUint64", Uint64Type)
	int16TypeN      = NamedType("nInt16", Int16Type)
	int32TypeN      = NamedType("nInt32", Int32Type)
	int64TypeN      = NamedType("nInt64", Int64Type)
	float32TypeN    = NamedType("nFloat32", Float32Type)
	float64TypeN    = NamedType("nFloat64", Float64Type)
	complex64TypeN  = NamedType("nComplex64", Complex64Type)
	complex128TypeN = NamedType("nComplex128", Complex128Type)
	stringTypeN     = NamedType("nString", StringType)

	// Composite types representing strings and bytes.
	bytesType   = ListType(ByteType)
	bytesTypeN  = NamedType("nBytes", bytesType)
	bytes3Type  = ArrayType(3, ByteType)
	bytes3TypeN = NamedType("nBytes3", bytes3Type)
	// Composite types representing sequences of numbers.
	array3Uint64Type     = ArrayType(3, Uint64Type)
	array3Uint64TypeN    = NamedType("nArray3Uint64", ArrayType(3, uint64TypeN))
	array3Int64Type      = ArrayType(3, Int64Type)
	array3Int64TypeN     = NamedType("nArray3Int64", ArrayType(3, int64TypeN))
	array3Float64Type    = ArrayType(3, Float64Type)
	array3Float64TypeN   = NamedType("nArray3Float64", ArrayType(3, float64TypeN))
	array3Complex64Type  = ArrayType(3, Complex64Type)
	array3Complex64TypeN = NamedType("nArray3Complex64", ArrayType(3, complex64TypeN))
	listUint64Type       = ListType(Uint64Type)
	listUint64TypeN      = NamedType("nListUint64", ListType(uint64TypeN))
	listInt64Type        = ListType(Int64Type)
	listInt64TypeN       = NamedType("nListInt64", ListType(int64TypeN))
	listFloat64Type      = ListType(Float64Type)
	listFloat64TypeN     = NamedType("nListFloat64", ListType(float64TypeN))
	listComplex64Type    = ListType(Complex64Type)
	listComplex64TypeN   = NamedType("nListComplex64", ListType(complex64TypeN))
	// Composite types representing sets of numbers.
	setUint64Type         = SetType(Uint64Type)
	setUint64TypeN        = NamedType("nSetUint64", SetType(uint64TypeN))
	setInt64Type          = SetType(Int64Type)
	setInt64TypeN         = NamedType("nSetInt64", SetType(int64TypeN))
	setFloat64Type        = SetType(Float64Type)
	setFloat64TypeN       = NamedType("nSetFloat64", SetType(float64TypeN))
	setComplex64Type      = SetType(Complex64Type)
	setComplex64TypeN     = NamedType("nSetComplex64", SetType(complex64TypeN))
	mapUint64BoolType     = MapType(Uint64Type, BoolType)
	mapUint64BoolTypeN    = NamedType("nMapUint64Bool", MapType(uint64TypeN, boolTypeN))
	mapInt64BoolType      = MapType(Int64Type, BoolType)
	mapInt64BoolTypeN     = NamedType("nMapInt64Bool", MapType(int64TypeN, boolTypeN))
	mapFloat64BoolType    = MapType(Float64Type, BoolType)
	mapFloat64BoolTypeN   = NamedType("nMapFloat64Bool", MapType(float64TypeN, boolTypeN))
	mapComplex64BoolType  = MapType(Complex64Type, BoolType)
	mapComplex64BoolTypeN = NamedType("nMapComplex64Bool", MapType(complex64TypeN, boolTypeN))
	// Composite types representing sets of strings.
	setStringType      = SetType(StringType)
	setStringTypeN     = NamedType("nSetString", SetType(stringTypeN))
	mapStringBoolType  = MapType(StringType, BoolType)
	mapStringBoolTypeN = NamedType("nMapStringBool", MapType(stringTypeN, boolTypeN))
	structXYZBoolType  = StructType(Field{"X", BoolType}, Field{"Y", BoolType}, Field{"Z", BoolType})
	structXYZBoolTypeN = NamedType("nStructXYZBool", StructType(Field{"X", boolTypeN}, Field{"Y", boolTypeN}, Field{"Z", boolTypeN}))
	structWXBoolType   = StructType(Field{"W", BoolType}, Field{"X", BoolType})
	structWXBoolTypeN  = NamedType("nStructWXBool", StructType(Field{"W", boolTypeN}, Field{"X", boolTypeN}))
	// Composite types representing maps of strings to numbers.
	mapStringUint64Type     = MapType(StringType, Uint64Type)
	mapStringUint64TypeN    = NamedType("nMapStringUint64", MapType(stringTypeN, uint64TypeN))
	mapStringInt64Type      = MapType(StringType, Int64Type)
	mapStringInt64TypeN     = NamedType("nMapStringInt64", MapType(stringTypeN, int64TypeN))
	mapStringFloat64Type    = MapType(StringType, Float64Type)
	mapStringFloat64TypeN   = NamedType("nMapStringFloat64", MapType(stringTypeN, float64TypeN))
	mapStringComplex64Type  = MapType(StringType, Complex64Type)
	mapStringComplex64TypeN = NamedType("nMapStringComplex64", MapType(stringTypeN, complex64TypeN))
	structVWXUint64Type     = StructType(Field{"V", Uint64Type}, Field{"W", Uint64Type}, Field{"X", Uint64Type})
	structVWXUint64TypeN    = NamedType("nStructVWXUint64", StructType(Field{"V", uint64TypeN}, Field{"W", uint64TypeN}, Field{"X", uint64TypeN}))
	structVWXInt64Type      = StructType(Field{"V", Int64Type}, Field{"W", Int64Type}, Field{"X", Int64Type})
	structVWXInt64TypeN     = NamedType("nStructVWXInt64", StructType(Field{"V", int64TypeN}, Field{"W", int64TypeN}, Field{"X", int64TypeN}))
	structVWXFloat64Type    = StructType(Field{"V", Float64Type}, Field{"W", Float64Type}, Field{"X", Float64Type})
	structVWXFloat64TypeN   = NamedType("nStructVWXFloat64", StructType(Field{"V", float64TypeN}, Field{"W", float64TypeN}, Field{"X", float64TypeN}))
	structVWXComplex64Type  = StructType(Field{"V", Complex64Type}, Field{"W", Complex64Type}, Field{"X", Complex64Type})
	structVWXComplex64TypeN = NamedType("nStructVWXComplex64", StructType(Field{"V", complex64TypeN}, Field{"W", complex64TypeN}, Field{"X", complex64TypeN}))
	structUVUint64Type      = StructType(Field{"U", Uint64Type}, Field{"V", Uint64Type})
	structUVUint64TypeN     = NamedType("nStructUVUint64", StructType(Field{"U", uint64TypeN}, Field{"V", uint64TypeN}))
	structUVInt64Type       = StructType(Field{"U", Int64Type}, Field{"V", Int64Type})
	structUVInt64TypeN      = NamedType("nStructUVInt64", StructType(Field{"U", int64TypeN}, Field{"V", int64TypeN}))
	structUVFloat64Type     = StructType(Field{"U", Float64Type}, Field{"V", Float64Type})
	structUVFloat64TypeN    = NamedType("nStructUVFloat64", StructType(Field{"U", float64TypeN}, Field{"V", float64TypeN}))
	structUVComplex64Type   = StructType(Field{"U", Complex64Type}, Field{"V", Complex64Type})
	structUVComplex64TypeN  = NamedType("nStructUVComplex64", StructType(Field{"U", complex64TypeN}, Field{"V", complex64TypeN}))

	structAIntType  = StructType(Field{"A", Int64Type})
	structAIntTypeN = NamedType("nStructA", structAIntType)

	unionTypeN = rtN("Union", UnionType([]Field{{"A", BoolType}, {"B", StringType}, {"C", Int32Type}}...))

	// Types that cannot be converted to sets.  Although we represent sets as
	// map[key]struct{} on the Go side, we don't allow these as general
	// conversions for val.Value.
	emptyType           = StructType()
	emptyTypeN          = NamedType("nEmpty", StructType())
	mapStringEmptyType  = MapType(StringType, emptyType)
	mapStringEmptyTypeN = NamedType("nMapStringEmpty", MapType(stringTypeN, emptyTypeN))
	structXYZEmptyType  = StructType(Field{"X", emptyType}, Field{"Y", emptyType}, Field{"Z", emptyType})
	structXYZEmptyTypeN = NamedType("nStructXYZEmpty", StructType(Field{"X", emptyTypeN}, Field{"Y", emptyTypeN}, Field{"Z", emptyTypeN}))
)

func anyValue(x *Value) *Value                  { return ZeroValue(AnyType).Assign(x) }
func boolValue(t *Type, x bool) *Value          { return ZeroValue(t).AssignBool(x) }
func byteValue(t *Type, x byte) *Value          { return ZeroValue(t).AssignByte(x) }
func uintValue(t *Type, x uint64) *Value        { return ZeroValue(t).AssignUint(x) }
func intValue(t *Type, x int64) *Value          { return ZeroValue(t).AssignInt(x) }
func floatValue(t *Type, x float64) *Value      { return ZeroValue(t).AssignFloat(x) }
func complexValue(t *Type, x complex128) *Value { return ZeroValue(t).AssignComplex(x) }
func stringValue(t *Type, x string) *Value      { return ZeroValue(t).AssignString(x) }
func bytesValue(t *Type, x string) *Value       { return ZeroValue(t).AssignBytes([]byte(x)) }
func bytes3Value(t *Type, x string) *Value      { return ZeroValue(t).CopyBytes([]byte(x)) }

func setStringValue(t *Type, x ...string) *Value {
	res := ZeroValue(t)
	for _, vx := range x {
		key := ZeroValue(t.Key()).AssignString(vx)
		res.AssignSetKey(key)
	}
	return res
}

type sb struct {
	s string
	b bool
}

func mapStringBoolValue(t *Type, x ...sb) *Value {
	res := ZeroValue(t)
	for _, sb := range x {
		key := ZeroValue(t.Key()).AssignString(sb.s)
		val := ZeroValue(t.Elem()).AssignBool(sb.b)
		res.AssignMapIndex(key, val)
	}
	return res
}

func mapStringEmptyValue(t *Type, x ...string) *Value {
	res := ZeroValue(t)
	for _, vx := range x {
		key := ZeroValue(t.Key()).AssignString(vx)
		val := ZeroValue(t.Elem())
		res.AssignMapIndex(key, val)
	}
	return res
}

func structBoolValue(t *Type, x ...sb) *Value {
	res := ZeroValue(t)
	for _, sb := range x {
		_, index := t.FieldByName(sb.s)
		res.Field(index).AssignBool(sb.b)
	}
	return res
}

func assignNum(v *Value, num float64) *Value {
	switch v.Kind() {
	case Byte:
		v.AssignByte(byte(num))
	case Uint16, Uint32, Uint64:
		v.AssignUint(uint64(num))
	case Int16, Int32, Int64:
		v.AssignInt(int64(num))
	case Float32, Float64:
		v.AssignFloat(num)
	case Complex64, Complex128:
		v.AssignComplex(complex(num, 0))
	default:
		panic(fmt.Errorf("val: assignNum unhandled %v", v.Type()))
	}
	return v
}

func seqNumValue(t *Type, x ...float64) *Value {
	res := ZeroValue(t)
	if t.Kind() == List {
		res.AssignLen(len(x))
	}
	for index, n := range x {
		assignNum(res.Index(index), n)
	}
	return res
}

func setNumValue(t *Type, x ...float64) *Value {
	res := ZeroValue(t)
	for _, n := range x {
		res.AssignSetKey(assignNum(ZeroValue(t.Key()), n))
	}
	return res
}

type nb struct {
	n float64
	b bool
}

func mapNumBoolValue(t *Type, x ...nb) *Value {
	res := ZeroValue(t)
	for _, nb := range x {
		key := assignNum(ZeroValue(t.Key()), nb.n)
		val := ZeroValue(t.Elem()).AssignBool(nb.b)
		res.AssignMapIndex(key, val)
	}
	return res
}

type sn struct {
	s string
	n float64
}

func mapStringNumValue(t *Type, x ...sn) *Value {
	res := ZeroValue(t)
	for _, sn := range x {
		key := ZeroValue(t.Key()).AssignString(sn.s)
		val := assignNum(ZeroValue(t.Elem()), sn.n)
		res.AssignMapIndex(key, val)
	}
	return res
}

func structNumValue(t *Type, x ...sn) *Value {
	res := ZeroValue(t)
	for _, sn := range x {
		_, index := t.FieldByName(sn.s)
		assignNum(res.Field(index), sn.n)
	}
	return res
}
