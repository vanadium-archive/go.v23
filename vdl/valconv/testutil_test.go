package valconv

// TODO(toddw): Move vdl/valconv back into vdl, but keep vdl/opconst separate.
// TODO(toddw): Merge with vdl/testutil_test.go and vdl/opconst/testutil_test.go

import (
	"fmt"
	"strings"
	"testing"

	"veyron.io/veyron/veyron2/vdl"
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
	nType       *vdl.Type
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
	nArray3TypeObject [3]*vdl.Type
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
	nStructTypeObject struct{ X *vdl.Type }
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
	nSliceTypeObject []*vdl.Type
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
	nSetTypeObject map[*vdl.Type]struct{}
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
	nMapTypeObject map[*vdl.Type]*vdl.Type
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

func recurseSelfType() *vdl.Type {
	var builder vdl.TypeBuilder
	n := builder.Named("veyron.io/veyron/veyron2/vdl.nRecurseSelf")
	n.AssignBase(builder.Struct().AppendField("X", builder.List().AssignElem(n)))
	builder.Build()
	t, err := n.Built()
	if err != nil {
		panic(err)
	}
	return t
}

func recurseABTypes() [2]*vdl.Type {
	var builder vdl.TypeBuilder
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
	return [2]*vdl.Type{aT, bT}
}

func recurseAType() *vdl.Type { return recurseABTypes()[0] }
func recurseBType() *vdl.Type { return recurseABTypes()[1] }

// Special case enum isn't regularly expressible in Go.
type nEnum int

const (
	nEnumA nEnum = iota
	nEnumB
	nEnumC
	nEnumABC
)

func (x *nEnum) Assign(label string) bool {
	switch label {
	case "A":
		*x = nEnumA
		return true
	case "B":
		*x = nEnumB
		return true
	case "C":
		*x = nEnumC
		return true
	case "ABC":
		*x = nEnumABC
		return true
	}
	*x = -1
	return false
}

func (x nEnum) String() string {
	switch x {
	case nEnumA:
		return "A"
	case nEnumB:
		return "B"
	case nEnumC:
		return "C"
	case nEnumABC:
		return "ABC"
	}
	return ""
}

func (nEnum) __DescribeEnum(struct{ A, B, C, ABC nEnum }) {}

var enumTypeN = vdl.NamedType("nEnum", vdl.EnumType("A", "B", "C", "ABC"))

// oneof{A bool;B string;C nStructInt64}
type (
	nOneOfABC interface {
		Index() int
		Name() string
		__DescribeOneOf(__nOneOfABCDesc)
	}
	nOneOfABCA struct{ Value bool }
	nOneOfABCB struct{ Value string }
	nOneOfABCC struct{ Value nStructInt64 }

	__nOneOfABCDesc struct {
		nOneOfABC
		A nOneOfABCA
		B nOneOfABCB
		C nOneOfABCC
	}
)

func (nOneOfABCA) Name() string                    { return "A" }
func (nOneOfABCA) Index() int                      { return 0 }
func (nOneOfABCA) __DescribeOneOf(__nOneOfABCDesc) {}
func (nOneOfABCB) Name() string                    { return "B" }
func (nOneOfABCB) Index() int                      { return 1 }
func (nOneOfABCB) __DescribeOneOf(__nOneOfABCDesc) {}
func (nOneOfABCC) Name() string                    { return "C" }
func (nOneOfABCC) Index() int                      { return 2 }
func (nOneOfABCC) __DescribeOneOf(__nOneOfABCDesc) {}

// oneof{B string;C nStructInt64;D int64}
type (
	nOneOfBCD interface {
		Index() int
		Name() string
		__DescribeOneOf(__nOneOfBCDDesc)
	}
	nOneOfBCDB struct{ Value string }
	nOneOfBCDC struct{ Value nStructInt64 }
	nOneOfBCDD struct{ Value int64 }

	__nOneOfBCDDesc struct {
		nOneOfBCD
		B nOneOfBCDB
		C nOneOfBCDC
		D nOneOfBCDD
	}
)

func (nOneOfBCDB) Name() string                    { return "B" }
func (nOneOfBCDB) Index() int                      { return 0 }
func (nOneOfBCDB) __DescribeOneOf(__nOneOfBCDDesc) {}
func (nOneOfBCDC) Name() string                    { return "C" }
func (nOneOfBCDC) Index() int                      { return 1 }
func (nOneOfBCDC) __DescribeOneOf(__nOneOfBCDDesc) {}
func (nOneOfBCDD) Name() string                    { return "D" }
func (nOneOfBCDD) Index() int                      { return 2 }
func (nOneOfBCDD) __DescribeOneOf(__nOneOfBCDDesc) {}

var (
	structInt64TypeN = vdl.NamedType("veyron.io/veyron/veyron2/vdl/valconv.nStructInt64", vdl.StructType(vdl.Field{"X", vdl.Int64Type}))
	oneOfABCTypeN    = vdl.NamedType("veyron.io/veyron/veyron2/vdl/valconv.nOneOfABC", vdl.OneOfType([]vdl.Field{{"A", vdl.BoolType}, {"B", vdl.StringType}, {"C", structInt64TypeN}}...))
	oneOfBCDTypeN    = vdl.NamedType("veyron.io/veyron/veyron2/vdl/valconv.nOneOfBCD", vdl.OneOfType([]vdl.Field{{"B", vdl.StringType}, {"C", structInt64TypeN}, {"D", vdl.Int64Type}}...))
)

// Define a bunch of *Type types used in tests.
var (
	// Named scalar types
	boolTypeN       = vdl.NamedType("nBool", vdl.BoolType)
	nByteType       = vdl.NamedType("nByte", vdl.ByteType)
	uint16TypeN     = vdl.NamedType("nUint16", vdl.Uint16Type)
	uint32TypeN     = vdl.NamedType("nUint32", vdl.Uint32Type)
	uint64TypeN     = vdl.NamedType("nUint64", vdl.Uint64Type)
	int16TypeN      = vdl.NamedType("nInt16", vdl.Int16Type)
	int32TypeN      = vdl.NamedType("nInt32", vdl.Int32Type)
	int64TypeN      = vdl.NamedType("nInt64", vdl.Int64Type)
	float32TypeN    = vdl.NamedType("nFloat32", vdl.Float32Type)
	float64TypeN    = vdl.NamedType("nFloat64", vdl.Float64Type)
	complex64TypeN  = vdl.NamedType("nComplex64", vdl.Complex64Type)
	complex128TypeN = vdl.NamedType("nComplex128", vdl.Complex128Type)
	stringTypeN     = vdl.NamedType("nString", vdl.StringType)

	// Composite types representing strings and bytes.
	bytesType   = vdl.ListType(vdl.ByteType)
	bytesTypeN  = vdl.NamedType("nBytes", bytesType)
	bytes3Type  = vdl.ArrayType(3, vdl.ByteType)
	bytes3TypeN = vdl.NamedType("nBytes3", bytes3Type)
	// Composite types representing sequences of numbers.
	array3Uint64Type     = vdl.ArrayType(3, vdl.Uint64Type)
	array3Uint64TypeN    = vdl.NamedType("nArray3Uint64", vdl.ArrayType(3, uint64TypeN))
	array3Int64Type      = vdl.ArrayType(3, vdl.Int64Type)
	array3Int64TypeN     = vdl.NamedType("nArray3Int64", vdl.ArrayType(3, int64TypeN))
	array3Float64Type    = vdl.ArrayType(3, vdl.Float64Type)
	array3Float64TypeN   = vdl.NamedType("nArray3Float64", vdl.ArrayType(3, float64TypeN))
	array3Complex64Type  = vdl.ArrayType(3, vdl.Complex64Type)
	array3Complex64TypeN = vdl.NamedType("nArray3Complex64", vdl.ArrayType(3, complex64TypeN))
	listUint64Type       = vdl.ListType(vdl.Uint64Type)
	listUint64TypeN      = vdl.NamedType("nListUint64", vdl.ListType(uint64TypeN))
	listInt64Type        = vdl.ListType(vdl.Int64Type)
	listInt64TypeN       = vdl.NamedType("nListInt64", vdl.ListType(int64TypeN))
	listFloat64Type      = vdl.ListType(vdl.Float64Type)
	listFloat64TypeN     = vdl.NamedType("nListFloat64", vdl.ListType(float64TypeN))
	listComplex64Type    = vdl.ListType(vdl.Complex64Type)
	listComplex64TypeN   = vdl.NamedType("nListComplex64", vdl.ListType(complex64TypeN))
	// Composite types representing sets of numbers.
	setUint64Type         = vdl.SetType(vdl.Uint64Type)
	setUint64TypeN        = vdl.NamedType("nSetUint64", vdl.SetType(uint64TypeN))
	setInt64Type          = vdl.SetType(vdl.Int64Type)
	setInt64TypeN         = vdl.NamedType("nSetInt64", vdl.SetType(int64TypeN))
	setFloat64Type        = vdl.SetType(vdl.Float64Type)
	setFloat64TypeN       = vdl.NamedType("nSetFloat64", vdl.SetType(float64TypeN))
	setComplex64Type      = vdl.SetType(vdl.Complex64Type)
	setComplex64TypeN     = vdl.NamedType("nSetComplex64", vdl.SetType(complex64TypeN))
	mapUint64BoolType     = vdl.MapType(vdl.Uint64Type, vdl.BoolType)
	mapUint64BoolTypeN    = vdl.NamedType("nMapUint64Bool", vdl.MapType(uint64TypeN, boolTypeN))
	mapInt64BoolType      = vdl.MapType(vdl.Int64Type, vdl.BoolType)
	mapInt64BoolTypeN     = vdl.NamedType("nMapInt64Bool", vdl.MapType(int64TypeN, boolTypeN))
	mapFloat64BoolType    = vdl.MapType(vdl.Float64Type, vdl.BoolType)
	mapFloat64BoolTypeN   = vdl.NamedType("nMapFloat64Bool", vdl.MapType(float64TypeN, boolTypeN))
	mapComplex64BoolType  = vdl.MapType(vdl.Complex64Type, vdl.BoolType)
	mapComplex64BoolTypeN = vdl.NamedType("nMapComplex64Bool", vdl.MapType(complex64TypeN, boolTypeN))
	// Composite types representing sets of strings.
	setStringType      = vdl.SetType(vdl.StringType)
	setStringTypeN     = vdl.NamedType("nSetString", vdl.SetType(stringTypeN))
	mapStringBoolType  = vdl.MapType(vdl.StringType, vdl.BoolType)
	mapStringBoolTypeN = vdl.NamedType("nMapStringBool", vdl.MapType(stringTypeN, boolTypeN))
	structXYZBoolType  = vdl.StructType(vdl.Field{"X", vdl.BoolType}, vdl.Field{"Y", vdl.BoolType}, vdl.Field{"Z", vdl.BoolType})
	structXYZBoolTypeN = vdl.NamedType("nStructXYZBool", vdl.StructType(vdl.Field{"X", boolTypeN}, vdl.Field{"Y", boolTypeN}, vdl.Field{"Z", boolTypeN}))
	structWXBoolType   = vdl.StructType(vdl.Field{"W", vdl.BoolType}, vdl.Field{"X", vdl.BoolType})
	structWXBoolTypeN  = vdl.NamedType("nStructWXBool", vdl.StructType(vdl.Field{"W", boolTypeN}, vdl.Field{"X", boolTypeN}))
	// Composite types representing maps of strings to numbers.
	mapStringUint64Type     = vdl.MapType(vdl.StringType, vdl.Uint64Type)
	mapStringUint64TypeN    = vdl.NamedType("nMapStringUint64", vdl.MapType(stringTypeN, uint64TypeN))
	mapStringInt64Type      = vdl.MapType(vdl.StringType, vdl.Int64Type)
	mapStringInt64TypeN     = vdl.NamedType("nMapStringInt64", vdl.MapType(stringTypeN, int64TypeN))
	mapStringFloat64Type    = vdl.MapType(vdl.StringType, vdl.Float64Type)
	mapStringFloat64TypeN   = vdl.NamedType("nMapStringFloat64", vdl.MapType(stringTypeN, float64TypeN))
	mapStringComplex64Type  = vdl.MapType(vdl.StringType, vdl.Complex64Type)
	mapStringComplex64TypeN = vdl.NamedType("nMapStringComplex64", vdl.MapType(stringTypeN, complex64TypeN))
	structVWXUint64Type     = vdl.StructType(vdl.Field{"V", vdl.Uint64Type}, vdl.Field{"W", vdl.Uint64Type}, vdl.Field{"X", vdl.Uint64Type})
	structVWXUint64TypeN    = vdl.NamedType("nStructVWXUint64", vdl.StructType(vdl.Field{"V", uint64TypeN}, vdl.Field{"W", uint64TypeN}, vdl.Field{"X", uint64TypeN}))
	structVWXInt64Type      = vdl.StructType(vdl.Field{"V", vdl.Int64Type}, vdl.Field{"W", vdl.Int64Type}, vdl.Field{"X", vdl.Int64Type})
	structVWXInt64TypeN     = vdl.NamedType("nStructVWXInt64", vdl.StructType(vdl.Field{"V", int64TypeN}, vdl.Field{"W", int64TypeN}, vdl.Field{"X", int64TypeN}))
	structVWXFloat64Type    = vdl.StructType(vdl.Field{"V", vdl.Float64Type}, vdl.Field{"W", vdl.Float64Type}, vdl.Field{"X", vdl.Float64Type})
	structVWXFloat64TypeN   = vdl.NamedType("nStructVWXFloat64", vdl.StructType(vdl.Field{"V", float64TypeN}, vdl.Field{"W", float64TypeN}, vdl.Field{"X", float64TypeN}))
	structVWXComplex64Type  = vdl.StructType(vdl.Field{"V", vdl.Complex64Type}, vdl.Field{"W", vdl.Complex64Type}, vdl.Field{"X", vdl.Complex64Type})
	structVWXComplex64TypeN = vdl.NamedType("nStructVWXComplex64", vdl.StructType(vdl.Field{"V", complex64TypeN}, vdl.Field{"W", complex64TypeN}, vdl.Field{"X", complex64TypeN}))
	structUVUint64Type      = vdl.StructType(vdl.Field{"U", vdl.Uint64Type}, vdl.Field{"V", vdl.Uint64Type})
	structUVUint64TypeN     = vdl.NamedType("nStructUVUint64", vdl.StructType(vdl.Field{"U", uint64TypeN}, vdl.Field{"V", uint64TypeN}))
	structUVInt64Type       = vdl.StructType(vdl.Field{"U", vdl.Int64Type}, vdl.Field{"V", vdl.Int64Type})
	structUVInt64TypeN      = vdl.NamedType("nStructUVInt64", vdl.StructType(vdl.Field{"U", int64TypeN}, vdl.Field{"V", int64TypeN}))
	structUVFloat64Type     = vdl.StructType(vdl.Field{"U", vdl.Float64Type}, vdl.Field{"V", vdl.Float64Type})
	structUVFloat64TypeN    = vdl.NamedType("nStructUVFloat64", vdl.StructType(vdl.Field{"U", float64TypeN}, vdl.Field{"V", float64TypeN}))
	structUVComplex64Type   = vdl.StructType(vdl.Field{"U", vdl.Complex64Type}, vdl.Field{"V", vdl.Complex64Type})
	structUVComplex64TypeN  = vdl.NamedType("nStructUVComplex64", vdl.StructType(vdl.Field{"U", complex64TypeN}, vdl.Field{"V", complex64TypeN}))

	structAIntType  = vdl.StructType(vdl.Field{"A", vdl.Int64Type})
	structAIntTypeN = vdl.NamedType("nStructA", structAIntType)

	// Types that cannot be converted to sets.  Although we represent sets as
	// map[key]struct{} on the Go side, we don't allow these as general
	// conversions for val.Value.
	emptyType           = vdl.StructType()
	emptyTypeN          = vdl.NamedType("nEmpty", vdl.StructType())
	mapStringEmptyType  = vdl.MapType(vdl.StringType, emptyType)
	mapStringEmptyTypeN = vdl.NamedType("nMapStringEmpty", vdl.MapType(stringTypeN, emptyTypeN))
	structXYZEmptyType  = vdl.StructType(vdl.Field{"X", emptyType}, vdl.Field{"Y", emptyType}, vdl.Field{"Z", emptyType})
	structXYZEmptyTypeN = vdl.NamedType("nStructXYZEmpty", vdl.StructType(vdl.Field{"X", emptyTypeN}, vdl.Field{"Y", emptyTypeN}, vdl.Field{"Z", emptyTypeN}))
)

func anyValue(x *vdl.Value) *vdl.Value                  { return vdl.ZeroValue(vdl.AnyType).Assign(x) }
func boolValue(t *vdl.Type, x bool) *vdl.Value          { return vdl.ZeroValue(t).AssignBool(x) }
func byteValue(t *vdl.Type, x byte) *vdl.Value          { return vdl.ZeroValue(t).AssignByte(x) }
func uintValue(t *vdl.Type, x uint64) *vdl.Value        { return vdl.ZeroValue(t).AssignUint(x) }
func intValue(t *vdl.Type, x int64) *vdl.Value          { return vdl.ZeroValue(t).AssignInt(x) }
func floatValue(t *vdl.Type, x float64) *vdl.Value      { return vdl.ZeroValue(t).AssignFloat(x) }
func complexValue(t *vdl.Type, x complex128) *vdl.Value { return vdl.ZeroValue(t).AssignComplex(x) }
func stringValue(t *vdl.Type, x string) *vdl.Value      { return vdl.ZeroValue(t).AssignString(x) }
func bytesValue(t *vdl.Type, x string) *vdl.Value       { return vdl.ZeroValue(t).AssignBytes([]byte(x)) }
func bytes3Value(t *vdl.Type, x string) *vdl.Value      { return vdl.ZeroValue(t).CopyBytes([]byte(x)) }

func setStringValue(t *vdl.Type, x ...string) *vdl.Value {
	res := vdl.ZeroValue(t)
	for _, vx := range x {
		key := vdl.ZeroValue(t.Key()).AssignString(vx)
		res.AssignSetKey(key)
	}
	return res
}

type sb struct {
	s string
	b bool
}

func mapStringBoolValue(t *vdl.Type, x ...sb) *vdl.Value {
	res := vdl.ZeroValue(t)
	for _, sb := range x {
		key := vdl.ZeroValue(t.Key()).AssignString(sb.s)
		val := vdl.ZeroValue(t.Elem()).AssignBool(sb.b)
		res.AssignMapIndex(key, val)
	}
	return res
}

func mapStringEmptyValue(t *vdl.Type, x ...string) *vdl.Value {
	res := vdl.ZeroValue(t)
	for _, vx := range x {
		key := vdl.ZeroValue(t.Key()).AssignString(vx)
		val := vdl.ZeroValue(t.Elem())
		res.AssignMapIndex(key, val)
	}
	return res
}

func structBoolValue(t *vdl.Type, x ...sb) *vdl.Value {
	res := vdl.ZeroValue(t)
	for _, sb := range x {
		_, index := t.FieldByName(sb.s)
		res.Field(index).AssignBool(sb.b)
	}
	return res
}

func assignNum(v *vdl.Value, num float64) *vdl.Value {
	switch v.Kind() {
	case vdl.Byte:
		v.AssignByte(byte(num))
	case vdl.Uint16, vdl.Uint32, vdl.Uint64:
		v.AssignUint(uint64(num))
	case vdl.Int16, vdl.Int32, vdl.Int64:
		v.AssignInt(int64(num))
	case vdl.Float32, vdl.Float64:
		v.AssignFloat(num)
	case vdl.Complex64, vdl.Complex128:
		v.AssignComplex(complex(num, 0))
	default:
		panic(fmt.Errorf("val: assignNum unhandled %v", v.Type()))
	}
	return v
}

func seqNumValue(t *vdl.Type, x ...float64) *vdl.Value {
	res := vdl.ZeroValue(t)
	if t.Kind() == vdl.List {
		res.AssignLen(len(x))
	}
	for index, n := range x {
		assignNum(res.Index(index), n)
	}
	return res
}

func setNumValue(t *vdl.Type, x ...float64) *vdl.Value {
	res := vdl.ZeroValue(t)
	for _, n := range x {
		res.AssignSetKey(assignNum(vdl.ZeroValue(t.Key()), n))
	}
	return res
}

type nb struct {
	n float64
	b bool
}

func mapNumBoolValue(t *vdl.Type, x ...nb) *vdl.Value {
	res := vdl.ZeroValue(t)
	for _, nb := range x {
		key := assignNum(vdl.ZeroValue(t.Key()), nb.n)
		val := vdl.ZeroValue(t.Elem()).AssignBool(nb.b)
		res.AssignMapIndex(key, val)
	}
	return res
}

type sn struct {
	s string
	n float64
}

func mapStringNumValue(t *vdl.Type, x ...sn) *vdl.Value {
	res := vdl.ZeroValue(t)
	for _, sn := range x {
		key := vdl.ZeroValue(t.Key()).AssignString(sn.s)
		val := assignNum(vdl.ZeroValue(t.Elem()), sn.n)
		res.AssignMapIndex(key, val)
	}
	return res
}

func structNumValue(t *vdl.Type, x ...sn) *vdl.Value {
	res := vdl.ZeroValue(t)
	for _, sn := range x {
		_, index := t.FieldByName(sn.s)
		assignNum(res.Field(index), sn.n)
	}
	return res
}
