package vom

import (
	"fmt"
	"math"
	"strconv"

	"v.io/core/veyron2/wiretype"
)

// Data used to test VOM types and values
type vomTestData struct {
	data          interface{}
	canCreateObj  bool
	canCallEquals bool
}

var vomTestVals []vomTestData

func init() {
	type testType struct {
		x int
		y *testType
		Z string
	}

	y := 4

	recurse := Recurse{U: 4}
	recurse.R = &recurse

	recurseA := RecurseA{Ua: 3}
	recurseB := RecurseB{Ub: 7, A: &recurseA}
	recurseA.B = &recurseB

	vomTestVals = []vomTestData{
		vomTestData{uint16(3), true, true},
		vomTestData{true, true, true},
		vomTestData{-1, true, true},
		vomTestData{4.5, true, true},
		vomTestData{"test", true, true},
		vomTestData{[]int{3, 4, 2}, true, false},
		vomTestData{[][1]string{{"A"}}, false, false},
		vomTestData{[]interface{}{3}, true, false},
		vomTestData{[3]int{3, 2, 1}, false, true},
		vomTestData{[1]interface{}{8}, false, true},
		vomTestData{map[string]int{"A": 3, "B": 8}, true, false},
		vomTestData{map[interface{}]int{"A": 3, 1: 8}, true, false},
		vomTestData{map[string]interface{}{"A": 3, "B": "X"}, true, false},
		vomTestData{testType{x: 4, y: &testType{x: 2, Z: "B"}, Z: "A"}, false, true},
		vomTestData{struct{ x, y int }{x: 4, y: 7}, false, true},
		vomTestData{&y, true, true},
		vomTestData{complex(3, 2), true, true},
		vomTestData{recurse, true, true},
		vomTestData{recurseA, true, true},
		vomTestData{recurseB, true, true},
		vomTestData{nil, true, true},
	}
}

// Data types used to test vom encoding and decoding.
type (
	Bool      bool
	String    string
	ByteSlice []byte
	Uint      uint
	Uint8     uint8
	Uint16    uint16
	Uint32    uint32
	Uint64    uint64
	Uintptr   uintptr
	Int       int
	Int8      int8
	Int16     int16
	Int32     int32
	Int64     int64
	Float32   float32
	Float64   float64

	BoolSlice     []bool
	BoolArray2    [2]bool
	UintStringMap map[uint]string
	StructA       struct{ A uint }
	BoolPtr       *bool

	Complex64  complex64
	Complex128 complex128

	Slice      []uint
	Array      [2]uint
	Map        map[uint]string
	OuterSlice []Slice
	OuterArray [1]Slice
	OuterMap   map[uint]Slice

	Struct struct {
		A uint
	}
	OuterStruct struct {
		S Slice
	}
	PtrStruct struct {
		A *uint
	}
	PartialStruct struct {
		// Unexported fields are ignored when encoding and decoding.
		x bool
		A uint
		y bool
		B uint
	}

	Recurse struct {
		U uint
		R *Recurse
	}

	RecurseA struct {
		Ua uint
		B  *RecurseB
	}
	RecurseB struct {
		Ub uint
		A  *RecurseA
	}

	NestedInterface struct {
		I interface{}
	}

	Byte           byte
	NamedByteSlice []Byte
	ByteArray      [3]byte
	NamedByteArray [3]Byte
)

// Test VomEncode / VomDecode using a non-pointer VomEncode receiver.  This is
// the canonical style.
type UserCoder struct {
	i int
}

func (uc UserCoder) VomEncode() (string, error) {
	return strconv.Itoa(uc.i), nil
}

func (uc *UserCoder) VomDecode(s string) error {
	var err error
	uc.i, err = strconv.Atoi(s)
	return err
}

// Test VomEncode / VomDecode using a pointer VomEncode receiver.
type UserCoderPtrEncode struct {
	i int
}

func (uc *UserCoderPtrEncode) VomEncode() (string, error) {
	return strconv.Itoa(uc.i), nil
}

func (uc *UserCoderPtrEncode) VomDecode(s string) error {
	var err error
	uc.i, err = strconv.Atoi(s)
	return err
}

// Test VomEncode / VomDecode that returns a pointer.
type UserCoderPtrType struct {
	i int
}

func (uc UserCoderPtrType) VomEncode() (*string, error) {
	str := strconv.Itoa(uc.i)
	return &str, nil
}

func (uc *UserCoderPtrType) VomDecode(s *string) error {
	var err error
	uc.i, err = strconv.Atoi(*s)
	return err
}

// Test GobEncode / GobDecode and MarshalJSON / UnmarshalJSON.
type UserCoderGobJSON struct {
	i int
}

func (uc *UserCoderGobJSON) GobEncode() ([]byte, error) {
	return []byte(strconv.Itoa(uc.i)), nil
}

func (uc *UserCoderGobJSON) GobDecode(buf []byte) error {
	var err error
	uc.i, err = strconv.Atoi(string(buf))
	return err
}

func (uc *UserCoderGobJSON) MarshalJSON() ([]byte, error) {
	// MarshalJSON must produce a valid JSON value, thus the quotes.
	return []byte(`"` + strconv.Itoa(uc.i) + `"`), nil
}

func (uc *UserCoderGobJSON) UnmarshalJSON(buf []byte) error {
	_, err := fmt.Sscanf(string(buf), `"%d"`, &uc.i)
	return err
}

// Test non-empty interfaces.
type Fooer interface {
	Foo() int
}

type ConcreteFoo int

func (f ConcreteFoo) Foo() int {
	return int(f)
}

// Test maps with multiple string keys.
type MultiStringKeyMap map[[2]string]string

func init() {
	// Register all test types so they can be decoded generically.
	Register(Uint(0))
	Register(Slice{})
	Register(Array{})
	Register(Map{})
	Register(OuterSlice{})
	Register(OuterArray{})
	Register(OuterMap{})
	Register(Struct{})
	Register(OuterStruct{})
	Register(PtrStruct{})
	Register(PartialStruct{})
	Register(Recurse{})
	Register(RecurseA{})
	Register(RecurseB{})
	Register(NestedInterface{})
	Register(UserCoder{})
	Register(UserCoderPtrEncode{})
	Register(UserCoderPtrType{})
	Register(UserCoderGobJSON{})
	Register(Complex64(0))
	Register(Complex128(0))
	Register(ConcreteFoo(0))
	Register(Byte(0))
	Register(ByteSlice{})
	Register(ByteArray{})
	Register(NamedByteSlice{})
	Register(NamedByteArray{})
	Register(MultiStringKeyMap{})
	Register(wrapComplex64{})
	Register(wrapComplex128{})
	// We also register the Fooer interface so that generic decoding faithfully
	// reproduces the correct interface.
	var fooer Fooer
	Register(&fooer)

	// For now we must explicitly register unnamed array and struct types in order
	// for generic decoding to faithfully reproduce the type.
	Register([2]uint{})
	Register([2]Uint{})
	Register(struct{ A uint }{})
	Register(struct{ A Uint }{})
	Register(struct{}{})

	Register(wiretype.NamedPrimitiveType{})
	Register(wiretype.SliceType{})
	Register(wiretype.ArrayType{})
	Register(wiretype.MapType{})
	Register(wiretype.FieldType{})
	Register(wiretype.StructType{})
	Register(wiretype.PtrType{})
}

// We can't create certain values as composite literals (e.g. pointers to
// interfaces, cyclic values), so we create vars manually here first.  Since
// these variables will be used in the coderTests composite literal, each
// pointer must be allocated immediately, but may be filled in later in the init
// function, which runs after coderTests is initialized.
var (
	one              uint        = 1
	ifaceUint        interface{} = uint(1)
	ifaceStruct      interface{} = Struct{1}
	ifaceRecurseAB   interface{} = RecurseA{1, &RecurseB{2, nil}}
	ifaceNestedIface interface{} = NestedInterface{ifaceStruct}
	recurseCycle     *Recurse    = &Recurse{}
	recurseABCycle   *RecurseA   = &RecurseA{}
	fooer            Fooer       = ConcreteFoo(3)
)

func init() {
	recurseCycle.U = 5
	recurseCycle.R = recurseCycle

	recurseABCycle.Ua = 5
	recurseABCycle.B = &RecurseB{6, recurseABCycle}
}

type v []interface{}
type j []string

// coderTests contains the tests we use for basic encoding and decoding tests.
// We check that encoding the values in a single encoder results in the expected
// hexpat, and that decoding the values from the hexpat in our various modes
// results in the expected values.  These are implicitly round-trip tests.
//
// We must use explicit sizes for the primitives that correspond to the vom
// defaults; e.g. uint64 rather than uint.  This is because the generic decoder
// doesn't know the size of type types we're encoding, and will default to the
// largest size for a given type.
var coderTests = []struct {
	Name    string
	Values  v
	HexPat  string // Hex-encoded protocol pattern; see matchHexPat for details.
	VomJSON string // VOM JSON-encoded expected result.
	JSON    j      // JSON-encoded expected results
}{
	// Primitives.
	{
		//            TypeID        Value
		// 0401       [04]bool      [01]true
		// 0603616263 [06]string    [03616263]"abc"
		// 0803646566 [08]byteslice [03646566]"def"
		"Basic",
		v{true, string("abc"), []byte("def")},
		"040106036162630803646566",
		`["bool",true]
				["string","abc"]
				["[]byte","ZGVm"]
				`,
		j{`true`, `"abc"`, `[100, 101, 102]`},
	},
	{
		//            TypeID      Value
		// 6001       [60]uintptr [01]1
		// 6202       [62]uint    [02]2
		// 6403       [64]uint8   [03]3
		// 6604       [66]uint16  [04]4
		// 6805       [68]uint32  [05]5
		// 6a06       [6a]uint64  [06]6
		"Uints",
		v{uintptr(1), uint(2), uint8(3), uint16(4), uint32(5), uint64(6)},
		"600162026403660468056a06",
		`["uintptr",1]
					["uint",2]
					["byte",3]
					["uint16",4]
					["uint32",5]
					["uint64",6]
					`,
		j{`1`, `2`, `3`, `4`, `5`, `6`},
	},
	{
		//            TypeID        Value
		// 4201       [42]int      [01]-1
		// 4403       [44]int8     [03]-2
		// 4605       [46]int16    [05]-3
		// 4807       [48]int32    [07]-4
		// 4a09       [4a]int64    [09]-5
		"Ints",
		v{int(-1), int8(-2), int16(-3), int32(-4), int64(-5)},
		"42014403460548074a09",
		`["int",-1]
					["int8",-2]
					["int16",-3]
					["int32",-4]
					["int64",-5]
					`,
		j{`-1`, `-2`, `-3`, `-4`, `-5`},
	},
	{
		//            TypeID        Value
		// 32fe3140   [32]float32   [fe3140]17.0
		// 34fe3240   [34]float64   [fe3240]18.0
		"Floats",
		v{float32(17.0), float64(18.0)},
		"32fe314034fe3240",
		`["float32",17]
					["float64",18]
					`,
		j{`17.0`, `18.0`},
	},
	{
		//            TypeID      Value
		// 647f       [64]uint8   [7f]127
		// 6480       [64]uint8   [80]128
		// 64ff       [64]uint8   [ff]255
		// 627f       [62]uint    [7f]127
		// 62ff80     [62]uint    [ff80]128
		// 62ffff     [62]uint    [ffff]255
		// Bytes only use one byte for [0, 255] while uint may take 2 bytes.
		"ByteVsUint",
		v{byte(127), byte(128), byte(255), uint(127), uint(128), uint(255)},
		"647f648064ff627f62ff8062ffff",
		`["byte",127]
			["byte",128]
			["byte",255]
			["uint",127]
			["uint",128]
			["uint",255]
			`,
		j{`127`, `128`, `255`, `127`, `128`, `255`},
	},
	{
		//                      TypeID      Value
		// 6af920000000000000   [6a]uint64  (2^53)
		// 6af920000000000001   [6a]uint64  (2^53)+1
		// 6af8ffffffffffffffff [6a]uint64  (2^64)-1
		// For JSON we quote numbers larger than 2^53.
		"BigUints",
		v{uint64(1 << 53), uint64((1 << 53) + 1), uint64(math.MaxUint64)},
		"6af9200000000000006af9200000000000016af8ffffffffffffffff",
		`["uint64",9007199254740992]
			["uint64","9007199254740993"]
			["uint64","18446744073709551615"]
			`,
		j{`9007199254740992`, `9007199254740993`, `18446744073709551615`},
	},
	{
		//                      TypeID      Value
		// 4af940000000000000   [4a]int64  (2^53)
		// 4af940000000000002   [4a]int64  (2^53)+1
		// 4af8fffffffffffffffe [4a]int64  (2^63)-1
		// 4af93fffffffffffff   [4a]int64  -(2^53)
		// 4af940000000000001   [4a]int64  -((2^53)+1)
		// 4af8ffffffffffffffff [4a]int64  -(2^63)
		// For JSON we quote numbers whose absoute value is larger than 2^53.
		"BigInts",
		v{int64(1 << 53), int64((1 << 53) + 1), int64(math.MaxInt64),
			-int64(1 << 53), -int64((1 << 53) + 1), int64(math.MinInt64)},
		"4af940000000000000" +
			"4af940000000000002" +
			"4af8fffffffffffffffe" +
			"4af93fffffffffffff" +
			"4af940000000000001" +
			"4af8ffffffffffffffff",
		`["int64",9007199254740992]
			["int64","9007199254740993"]
			["int64","9223372036854775807"]
			["int64",-9007199254740992]
			["int64","-9007199254740993"]
			["int64","-9223372036854775808"]
			`,
		j{`9007199254740992`, `9007199254740993`, `9223372036854775807`,
			`-9007199254740992`, `-9007199254740993`, `-9223372036854775808`},
	},

	// Simple named types.
	{
		// ff81 16           typedef 65[ff81] len[16]
		// 10                (NamedPrimitiveType)
		//   01 31             (NamedPrimitiveType.Type) uint
		//   01 10 766579726f6e322f766f6d2e55696e74
		//                     (NamedPrimitiveType.Name) "v.io/core/veyron2/vom.Uint"
		// 00                (NamedPrimitiveType end)
		//
		// ff82 01 01        type 65[ff82] len[01] value 1[01]
		"Uint",
		v{Uint(1)},
		"ff81161001310110766579726f6e322f766f6d2e55696e7400" +
			"ff820101",
		`["type","v.io/core/veyron2/vom.Uint uint"]
			["Uint",1]
			`,
		j{`1`},
	},

	// Slices are always encoded with a typedef to identify the type.  The value
	// is encoded starting with the length, followed by that many values.
	{
		// ff81 04           typedef 65[ff81] len[04]
		// 12                (SliceType)
		//   01 31             (SliceType.Type) uint
		// 00                (SliceType end)
		//
		// ff82 03 020102    type 65[ff82] len[03] values 1,2[02 0102]
		"UnnamedSlice",
		v{[]uint{1, 2}},
		"ff810412013100" +
			"ff8203020102",
		`["[]uint",[1,2]]
		`,
		j{`[1, 2]`},
	},
	{
		// ff81 04           typedef 65[ff81] len[04]
		// 12                (SliceType)
		//   01 42             (SliceType.Elem) typeid 66
		// 00                (SliceType end)
		//
		// ff83 16           typedef 66[ff83] len[16]
		// 10                (NamedPrimitiveType)
		//   01 31             (NamedPrimitiveType.Type) uint
		//   01 10 766579726f6e322f766f6d2e55696e74
		//                     (NamedPrimitiveType.Name) "v.io/core/veyron2/vom.Uint"
		// 00                (NamedPrimitiveType end)
		//
		// ff82 03 020102    type 65[ff82] len[03] values 1,2[02 0102]
		"UnnamedSliceOfUint",
		v{[]Uint{1, 2}},
		"ff810412014200" +
			"ff83161001310110766579726f6e322f766f6d2e55696e7400" +
			"ff8203020102",
		`["type","v.io/core/veyron2/vom.Uint uint"]
			["[]Uint",[1,2]]
			`,
		j{`[1, 2]`},
	},
	{
		// ff81 17           typedef 65[ff81] len[17]
		// 12                (SliceType)
		//   01 31             (SliceType.Elem) uint
		//   01 11 766579726f6e322f766f6d2e536c696365
		//                     (SliceType.Name) "v.io/core/veyron2/vom.Slice"
		// 00                (SliceType end)
		//
		// ff82 03 020102    type 65[ff82] len[03] values 1,2[02 0102]
		"Slice",
		v{Slice{1, 2}},
		"ff81171201310111766579726f6e322f766f6d2e536c69636500" +
			"ff8203020102",
		`["type","v.io/core/veyron2/vom.Slice []uint"]
			["Slice",[1,2]]
			`,
		j{`[1, 2]`},
	},
	{
		// ff81 1c           typedef 65[ff81] len[1c]
		// 12                (SliceType)
		//   01 42             (SliceType.Elem) typeid 66
		//   01 16 766579726f6e322f766f6d2e4f75746572536c696365
		//                     (SliceType.Name) "v.io/core/veyron2/vom.OuterSlice"
		// 00                (SliceType end)
		//
		// ff83 17           typedef 66[ff83] len[17]
		// 12                (SliceType)
		//   01 31             (SliceType.Elem) uint
		//   01 11 766579726f6e322f766f6d2e536c696365
		//                     (SliceType.Name) "v.io/core/veyron2/vom.Slice"
		// 00                (SliceType end)
		//
		// ff82 04 01 020102 type 65[ff82] len[04] values 1,2[01 02 0102]
		"OuterSlice",
		v{OuterSlice{Slice{1, 2}}},
		"ff811c1201420116766579726f6e322f766f6d2e4f75746572536c69636500" +
			"ff83171201310111766579726f6e322f766f6d2e536c69636500" +
			"ff820401020102",
		`["type","v.io/core/veyron2/vom.Slice []uint"]
			["type","v.io/core/veyron2/vom.OuterSlice []Slice"]
			["OuterSlice",[[1,2]]]
			`,
		j{`[[1, 2]]`},
	},
	{
		// ff81 04           typedef 65[ff81] len[04]
		// 12                (SliceType)
		//   01 31             (SliceType.Type) uint
		// 00                (SliceType end)
		//
		// ff82 01 00        type 65[ff82] len[01] nil[00]
		//
		// Empty slice []uint{} is distinct from nil slice in Go, but both are
		// represented the same and are translated to the nil slice.
		//
		// TODO(toddw): We can choose to handle the nil/empty distinction via a
		// special case, or we can choose to faithfully reproduce the sharing
		// concept in slices, or we can choose to do nothing.
		"NilSlice",
		v{[]uint(nil)},
		"ff810412013100ff820100",
		`["[]uint",[]]
			`,
		j{`[]`},
	},

	// Arrays are always encoded with a typedef to identify the type.  The value
	// is encoded starting as N values.  The Go reflect package doesn't let us
	// dynamically create an array type given its elem type, so we rely on
	// explicit registration of the type for generic decoding.
	{
		// ff81 06           typedef 65[ff81] len[06]
		// 14                (ArrayType)
		//   01 31             (ArrayType.Elem) uint
		//   01 02             (ArrayType.Len) 2
		// 00                (ArrayType end)
		//
		// ff82 02 0102    type 65[ff82] len[02] values 1,2[0102]
		"UnnamedArray",
		v{[2]uint{1, 2}},
		"ff8106140131010200" +
			"ff82020102",
		`["[2]uint",[1,2]]
			`,
		j{`[1, 2]`},
	},
	{
		// ff81 06           typedef 65[ff81] len[06]
		// 14                (ArrayType)
		//   01 42             (ArrayType.Elem) typeid 66
		//   01 02             (ArrayType.Len) 2
		// 00                (ArrayType end)
		//
		// ff83 16           typedef 66[ff83] len[16]
		// 10                (NamedPrimitiveType)
		//   01 31             (NamedPrimitiveType.Type) uint
		//   01 10 766579726f6e322f766f6d2e55696e74
		//                     (NamedPrimitiveType.Name) "v.io/core/veyron2/vom.Uint"
		// 00                (NamedPrimitiveType end)
		//
		// ff82 02 0102      type 65[ff82] len[02] values 1,2[0102]
		"UnnamedArrayOfUint",
		v{[2]Uint{1, 2}},
		"ff8106140142010200" +
			"ff83161001310110766579726f6e322f766f6d2e55696e7400" +
			"ff82020102",
		`["type","v.io/core/veyron2/vom.Uint uint"]
			["[2]Uint",[1,2]]
			`,
		j{`[1, 2]`},
	},
	{
		// ff81 19           typedef 65[ff81] len[19]
		// 14                (ArrayType)
		//   01 31             (ArrayType.Elem) uint
		//   01 02             (ArrayType.Len) 2
		//   01 11 766579726f6e322f766f6d2e4172726179
		//                     (ArrayType.Name) "v.io/core/veyron2/vom.Array"
		// 00                (ArrayType end)
		//
		// ff82 02 0102    type 65[ff82] len[02] values 1,2[0102]
		"Array",
		v{Array{1, 2}},
		"ff811914013101020111766579726f6e322f766f6d2e417272617900" +
			"ff82020102",
		`["type","v.io/core/veyron2/vom.Array [2]uint"]
			["Array",[1,2]]
			`,
		j{`[1, 2]`},
	},
	{
		// ff81 1e           typedef 65[ff81] len[1e]
		// 14                (ArrayType)
		//   01 42             (ArrayType.Elem) typeid 66
		//   01 01             (ArrayType.Len) 1
		//   01 16 766579726f6e322f766f6d2e4f757465724172726179
		//                     (ArrayType.Name) "v.io/core/veyron2/vom.OuterArray"
		// 00                (ArrayType end)
		//
		// ff83 17           typedef 66[ff83] len[17]
		// 12                (SliceType)
		//   01 31             (SliceType.Elem) uint
		//   01 11 766579726f6e322f766f6d2e536c696365
		//                     (SliceType.Name) "v.io/core/veyron2/vom.Slice"
		// 00                (SliceType end)
		//
		// ff82 03 020102    type 65[ff82] len[03] values 1,2[02 0102]
		"OuterArray",
		v{OuterArray{Slice{1, 2}}},
		"ff811e14014201010116766579726f6e322f766f6d2e4f75746572417272617900" +
			"ff83171201310111766579726f6e322f766f6d2e536c69636500" +
			"ff8203020102",
		`["type","v.io/core/veyron2/vom.Slice []uint"]
			["type","v.io/core/veyron2/vom.OuterArray [1]Slice"]
			["OuterArray",[[1,2]]]
			`,
		j{`[[1, 2]]`},
	},

	// Maps are always encoded with a typedef to identify the type.  The value is
	// encoded starting with the length, followed by that many key/elem values.
	{
		// ff81 06           typedef 65[ff81] len[06]
		// 16                (MapType)
		//   01 31             (MapType.Key) uint
		//   01 03             (MapType.Elem) string
		// 00                (MapType end)
		//
		// ff82 0b 02        type 65[ff82] len[0b] len[02]
		//   01 03 616263      key 1[01] value "abc"[03 616263]
		//   02 03 646566      key 2[02] value "def"[03 646566]
		"UnnamedMap",
		v{map[uint]string{1: "abc", 2: "def"}},
		"ff8106160131010300ff820b02[0103616263,0203646566]",
		`["map[uint]string",{"1":"abc","2":"def"}]
		`,
		j{`{"1": "abc", "2": "def"}`},
	},
	{
		// ff81 06           typedef 65[ff81] len[06]
		// 16                (MapType)
		//   01 42             (MapType.Key) typeid 66
		//   01 03             (MapType.Elem) string
		// 00                (MapType end)
		//
		// ff83 16           typedef 66[ff83] len[16]
		// 10                (NamedPrimitiveType)
		//   01 31             (NamedPrimitiveType.Type) uint
		//   01 10 766579726f6e322f766f6d2e55696e74
		//                     (NamedPrimitiveType.Name) "v.io/core/veyron2/vom.Uint"
		// 00                (NamedPrimitiveType end)
		//
		// ff82 0b 02        type 65[ff82] len[0b] len[02]
		//   01 03 616263      key 1[01] value "abc"[03 616263]
		//   02 03 646566      key 2[02] value "def"[03 646566]
		"UnnamedMapKeyUint",
		v{map[Uint]string{1: "abc", 2: "def"}},
		"ff8106160142010300" +
			"ff83161001310110766579726f6e322f766f6d2e55696e7400" +
			"ff820b02[0103616263,0203646566]",
		`["type","v.io/core/veyron2/vom.Uint uint"]
			["map[Uint]string",{"1":"abc","2":"def"}]
			`,
		j{`{"1": "abc", "2": "def"}`},
	},
	{
		// ff81 17           typedef 65[ff81] len[17]
		// 16                (MapType)
		//   01 31             (MapType.Key) uint
		//   01 03             (MapType.Elem) string
		//   01 0f 766579726f6e322f766f6d2e4d6170
		//                     (MapType.Name) "v.io/core/veyron2/vom.Map"
		// 00                (MapType end)
		//
		// ff82 0b 02        type 65[ff82] len[0b] len[02]
		//   01 03 616263      key 1[01] value "abc"[03 616263]
		//   02 03 646566      key 2[02] value "def"[03 646566]
		"Map",
		v{Map{1: "abc", 2: "def"}},
		"ff81171601310103010f766579726f6e322f766f6d2e4d617000" +
			"ff820b02[0103616263,0203646566]",
		`["type","v.io/core/veyron2/vom.Map map[uint]string"]
			["Map",{"1":"abc","2":"def"}]
			`,
		j{`{"1": "abc", "2": "def"}`},
	},
	{
		// ff81 06           typedef 65[ff81] len[06]
		// 16                (MapType)
		//   01 31             (MapType.Key) uint
		//   01 03             (MapType.Elem) string
		// 00                (MapType end)
		//
		// ff82 01 00        type 65[ff82] len[01] nil[00]
		"NilMap",
		v{map[uint]string(nil)},
		"ff8106160131010300ff820100",
		`["map[uint]string",{}]
			`,
		j{`{}`},
	},
	{
		// ff81 1c           typedef 65[ff81] len[1c]
		// 16                (MapType)
		//   01 31             (MapType.Key) uint
		//   01 42             (MapType.Elem) typeid 66
		//   01 14 766579726f6e322f766f6d2e4f757465724d6170
		//                     (MapType.Name) "v.io/core/veyron2/vom.OuterMap"
		// 00                (MapType end)
		//
		// ff83 17           typedef 66[ff83] len[17]
		// 12                (SliceType)
		//   01 31             (SliceType.Elem) uint
		//   01 11 766579726f6e322f766f6d2e536c696365
		//                     (SliceType.Name) "v.io/core/veyron2/vom.Slice"
		// 00                (SliceType end)
		//
		// ff82 09 02        type 65[ff82] len[09] len[02]
		//   03 020405         key 3[03] value []{4, 5}[02 0405]
		//   06 020708         key 6[06] value []{7, 8}[02 0708]
		"OuterMap",
		v{OuterMap{3: Slice{4, 5}, 6: Slice{7, 8}}},
		"ff811c1601310142" +
			"0114766579726f6e322f766f6d2e4f757465724d617000" +
			"ff83171201310111766579726f6e322f766f6d2e536c69636500" +
			"ff820902[03020405,06020708]",
		`["type","v.io/core/veyron2/vom.Slice []uint"]
			["type","v.io/core/veyron2/vom.OuterMap map[uint]Slice"]
			["OuterMap",{"3":[4,5],"6":[7,8]}]
			`,
		j{`{"3": [4, 5], "6": [7, 8]}`},
	},

	// Structs are always encoded with a typedef to identify the type.  The value
	// is encoded as a sequence of (field delta, value) pairs.  The field count
	// starts at -1, so field index 0 has a delta of 1.  Delta 0 is a sentry that
	// represents the end of the struct.
	{
		// ff81 0a           typedef 65[ff81] len[0a]
		// 18                (StructType)
		//   01 01             (StructType.Fields) 1 field
		//     01 31             (FieldType.Type 0) uint
		//     01 01 41          (FieldType.Name 0) "A"
		//     00                (FieldType end  0)
		// 00                (StructType end)
		//
		// ff82 03           type 65[ff82] len[03]
		//   01 01             (struct.A) 1[01]
		// 00                (end)
		"UnnamedStruct",
		v{struct{ A uint }{1}},
		"ff810a18010101310101410000" +
			"ff8203010100",
		`["struct{A uint}",{"A":1}]
			`,
		j{`{"a": 1}`},
	},
	// Test converting between capitalization styles on a longer struct field name.
	{
		// ff81 14           typedef 65[ff81] len[14]
		// 18                (StructType)
		//   01 01             (StructType.Fields) 1 field
		//     01 31             (FieldType.Type 0) uint
		//     01 0b 52 65 73 75 6C 74 43 6F 75 6E 74 (FieldType.Name 0) "ResultCount"
		//     00                (FieldType end  0)
		// 00                (StructType end)
		//
		// ff82 03           type 65[ff82] len[03]
		//   01 01             (struct.A) 1[01]
		// 00                (end)
		"StructWithLongFieldName",
		v{struct{ ResultCount uint }{1}},
		"ff81141801010131010b526573756c74436f756e740000" +
			"ff8203010100",
		`["struct{ResultCount uint}",{"ResultCount":1}]
			`,
		j{`{"resultCount": 1}`},
	},
	{
		// ff81 02           typedef 65[ff81] len[02]
		// 18                (StructType)
		// 00                (StructType end)
		//
		// ff82 01           type 65[ff82] len[01]
		// 00                (end)
		"EmptyStruct",
		v{struct{}{}},
		"ff81021800ff820100",
		`["struct{}",{}]
			`,
		j{`{}`},
	},
	{
		// ff81 0a           typedef 65[ff81] len[0a]
		// 18                (StructType)
		//   01 01             (StructType.Fields) 1 field
		//     01 42             (FieldType.Type 0) typeid 66
		//     01 01 41          (FieldType.Name 0) "A"
		//     00                (FieldType end  0)
		// 00                (StructType end)
		//
		// ff83 16           typedef 66[ff83] len[16]
		// 10                (NamedPrimitiveType)
		//   01 31             (NamedPrimitiveType.Type) uint
		//   01 10 766579726f6e322f766f6d2e55696e74
		//                     (NamedPrimitiveType.Name) "v.io/core/veyron2/vom.Uint"
		// 00                (NamedPrimitiveType end)
		//
		// ff82 03           type 65[ff82] len[03]
		//   01 01             (struct.A) 1[01]
		// 00                (end)
		"UnnamedStructOfUint",
		v{struct{ A Uint }{1}},
		"ff810a18010101420101410000" +
			"ff83161001310110766579726f6e322f766f6d2e55696e7400" +
			"ff8203010100",
		`["type","v.io/core/veyron2/vom.Uint uint"]
			["struct{A Uint}",{"A":1}]
			`,
		j{`{"a": 1}`},
	},
	{
		// ff81 1e           typedef 65[ff81] len[1e]
		// 18                (StructType)
		//   01 01             (StructType.Fields) 1 field
		//     01 31             (FieldType.Type 0) uint
		//     01 01 41          (FieldType.Name 0) "A"
		//     00                (FieldType end  0)
		//   01 12 766579726f6e322f766f6d2e537472756374
		//                     (StructType.Name) "v.io/core/veyron2/vom.Struct"
		// 00                (StructType end)
		//
		// ff82 03           type 65[ff82] len[03]
		//   01 01             (Struct.A) 1[01]
		// 00                (end)
		"Struct",
		v{Struct{1}},
		"ff811e180101013101014100" +
			"0112766579726f6e322f766f6d2e53747275637400" +
			"ff8203010100",
		`["type","v.io/core/veyron2/vom.Struct struct{A uint}"]
			["Struct",{"A":1}]
			`,
		j{`{"a": 1}`},
	},
	{
		// ff81 1e           typedef 65[ff81] len[1e]
		// 18                (StructType)
		//   01 01             (StructType.Fields) 1 field
		//     01 31             (FieldType.Type 0) uint
		//     01 01 41          (FieldType.Name 0) "A"
		//     00                (FieldType end  0)
		//   01 12 766579726f6e322f766f6d2e537472756374
		//                     (StructType.Name) "v.io/core/veyron2/vom.Struct"
		// 00                (StructType end)
		//
		// ff82 01           type 65[ff82] len[05]
		// 00                (end)
		"ZeroStruct",
		v{Struct{}},
		"ff811e180101013101014100" +
			"0112766579726f6e322f766f6d2e53747275637400" +
			"ff820100",
		`["type","v.io/core/veyron2/vom.Struct struct{A uint}"]
			["Struct",{}]
			`,
		j{`{}`},
	},
	{
		// ff81 23           typedef 65[ff81] len[23]
		// 18                (StructType)
		//   01 01             (StructType.Fields) 1 field
		//     01 42             (FieldType.Type 0) typeid 66
		//     01 01 53          (FieldType.Name 0) "S"
		//     00                (FieldType end  0)
		//   01 17 766579726f6e322f766f6d2e4f75746572537472756374
		//                     (StructType.Name) "v.io/core/veyron2/vom.OuterStruct"
		// 00                (StructType end)
		//
		// ff83 17           typedef 66[ff83] len[17]
		// 12                (SliceType)
		//   01 31             (SliceType.Elem) uint
		//   01 11 766579726f6e322f766f6d2e536c696365
		//                     (SliceType.Name) "v.io/core/veyron2/vom.Slice"
		// 00                (SliceType end)
		//
		// ff82 05           type 65[ff82] len[05]
		//   01 02 0506        (OuterStruct.S) values 5, 6[02 05 06]
		// 00                (end)
		"OuterStruct",
		v{OuterStruct{Slice{5, 6}}},
		"ff8123180101014201015300" +
			"0117766579726f6e322f766f6d2e4f7574657253747275637400" +
			"ff83171201310111766579726f6e322f766f6d2e536c69636500" +
			"ff82050102050600",
		`["type","v.io/core/veyron2/vom.Slice []uint"]
			["type","v.io/core/veyron2/vom.OuterStruct struct{S Slice}"]
			["OuterStruct",{"S":[5,6]}]
			`,
		j{`{"s":[5,6]}`},
	},
	{
		// ff81 21           typedef 65[ff81] len[21]
		// 18                (StructType)
		//   01 01             (StructType.Fields) 1 field
		//     01 42             (FieldType.Type 0) typeid 66
		//     01 01 41          (FieldType.Name 0) "A"
		//     00                (FieldType end  0)
		//   01 15 766579726f6e322f766f6d2e507472537472756374
		//                     (StructType.Name) "v.io/core/veyron2/vom.PtrStruct"
		// 00                (StructType end)
		//
		// ff83 04           typedef 66[ff83] len[04]
		// 1a                (PtrType)
		//   01 31             (PtrType.Elem) uint
		// 00                (PtrType end)
		//
		// ff82 04           type 65[ff82] len[04]
		//   01                refdef 1[01]
		//   01 01             (PtrStruct.A) value 1[01]
		// 00                (end)
		"PtrStructOne",
		v{PtrStruct{&one}},
		"ff8121180101014201014100" +
			"0115766579726f6e322f766f6d2e50747253747275637400" +
			"ff83041a013100" +
			"ff820401010100",
		`["type","v.io/core/veyron2/vom.PtrStruct struct{A *uint}"]
			["PtrStruct",{"A":[1,1]}]
			`,
		nil,
	},
	{
		// ff81 21           typedef 65[ff81] len[21]
		// 18                (StructType)
		//   01 01             (StructType.Fields) 1 field
		//     01 42             (FieldType.Type 0) typeid 66
		//     01 01 41          (FieldType.Name 0) "A"
		//     00                (FieldType end  0)
		//   01 15 766579726f6e322f766f6d2e507472537472756374
		//                     (StructType.Name) "v.io/core/veyron2/vom.PtrStruct"
		// 00                (StructType end)
		//
		// ff83 04           typedef 66[ff83] len[04]
		// 1a                (PtrType)
		//   01 31             (PtrType.Elem) uint
		// 00                (PtrType end)
		//
		// ff82 04           type 65[ff82] len[04]
		//   01                refdef 1[01]
		//   01 00             (PtrStruct.A) value 0[00]
		// 00                (end)
		"PtrStructZero",
		v{PtrStruct{new(uint)}},
		"ff8121180101014201014100" +
			"0115766579726f6e322f766f6d2e50747253747275637400" +
			"ff83041a013100" +
			"ff820401010000",
		`["type","v.io/core/veyron2/vom.PtrStruct struct{A *uint}"]
			["PtrStruct",{"A":[1,0]}]
			`,
		nil,
	},

	{
		// ff81 21           typedef 65[ff81] len[21]
		// 18                (StructType)
		//   01 01             (StructType.Fields) 1 field
		//     01 42             (FieldType.Type 0) typeid 66
		//     01 01 41          (FieldType.Name 0) "A"
		//     00                (FieldType end  0)
		//   01 15 766579726f6e322f766f6d2e507472537472756374
		//                     (StructType.Name) "v.io/core/veyron2/vom.PtrStruct"
		// 00                (StructType end)
		//
		// ff83 04           typedef 66[ff83] len[04]
		// 1a                (PtrType)
		//   01 31             (PtrType.Elem) uint
		// 00                (PtrType end)
		//
		// ff82 01           type 65[ff82] len[01]
		//   00                nil[00]
		"PtrStructNil",
		v{PtrStruct{}},
		"ff8121180101014201014100" +
			"0115766579726f6e322f766f6d2e50747253747275637400" +
			"ff83041a013100" +
			"ff820100",
		`["type","v.io/core/veyron2/vom.PtrStruct struct{A *uint}"]
			["PtrStruct",{}]
			`,
		nil,
	},

	// Only exported fields of structs are encoded in the type definition, and
	// zero valued fields are skipped in the value.
	{
		// ff81 2b           typedef 65[ff81] len[2b]
		// 18                (StructType)
		//   01 02             (StructType.Fields) 2 fields
		//     01 31             (FieldType.Type 0) uint
		//     01 01 41          (FieldType.Name 0) "A"
		//     00                (FieldType end  0)
		//     01 31             (FieldType.Type 0) uint
		//     01 01 42          (FieldType.Name 0) "B"
		//     00                (FieldType end  0)
		//   01 19 766579726f6e322f766f6d2e5061727469616c537472756374
		//                     (StructType.Name) "v.io/core/veyron2/vom.PartialStruct"
		// 00                (StructType end)
		//
		// ff82 03           type 65[ff82] len[03]
		//   02 05             (PartialStruct.A) 5[05]
		// 00                (end)
		"PartialStruct",
		v{PartialStruct{B: 5}},
		"ff812b180102013101014100013101014200" +
			"0119766579726f6e322f766f6d2e5061727469616c53747275637400" +
			"ff8203020500",
		`["type","v.io/core/veyron2/vom.PartialStruct struct{A uint;B uint}"]
			["PartialStruct",{"B":5}]
			`,
		j{`{"b": 5}`},
	},
	// Recursive types also work.
	{
		// ff81 25           typedef 65[ff81] len[25]
		// 18                (StructType)
		//   01 02             (StructType.Fields) 2 fields
		//     01 31             (FieldType.Type 0) uint
		//     01 01 55          (FieldType.Name 0) "U"
		//     00                (FieldType end  0)
		//     01 42             (FieldType.Type 0) typeid 66
		//     01 01 52          (FieldType.Name 0) "R"
		//     00                (FieldType end  0)
		//   01 13 766579726f6e322f766f6d2e52656375727365
		//                     (StructType.Name) "v.io/core/veyron2/vom.Recurse"
		// 00                (StructType end)
		//
		// ff83 04           typedef 66[ff83] len[04]
		// 1a                (PtrType)
		//   01 41             (PtrType.Elem) typeid 65
		// 00                (PtrType end)
		//
		// ff82 08           type 65[ff82] len[08]
		//   01 01             (Recurse.U) value 1[01]
		//   01 01             (Recurse.R) refdef 1[01]
		//     01 02             (Recurse.U) value 2[02]
		//     00                (end)
		//   00                (end)
		"Recurse",
		v{Recurse{1, &Recurse{2, nil}}},
		"ff8125180102013101015500014201015200" +
			"0113766579726f6e322f766f6d2e5265637572736500" +
			"ff83041a014100" +
			"ff82080101010101020000",
		`["type","v.io/core/veyron2/vom.Recurse struct{U uint;R *Recurse}"]
			["Recurse",{"U":1,"R":[1,{"U":2}]}]
			`,
		nil},
	{
		// ff81 04           typedef 65[ff81] len[04]
		// 1a                (PtrType)
		//   01 42             (PtrType.Elem) typeid 66
		// 00                (PtrType end)
		//
		// ff83 25           typedef 66[ff83] len[25]
		// 18                (StructType)
		//   01 02             (StructType.Fields) 2 fields
		//     01 31             (FieldType.Type 0) uint
		//     01 01 55          (FieldType.Name 0) "U"
		//     00                (FieldType end  0)
		//     01 41             (FieldType.Type 0) typeid 65
		//     01 01 52          (FieldType.Name 0) "R"
		//     00                (FieldType end  0)
		//   01 13 766579726f6e322f766f6d2e52656375727365
		//                     (StructType.Name) "v.io/core/veyron2/vom.Recurse"
		// 00                (StructType end)
		//
		// ff82 06           type 65[ff82] len[06]
		// 01                refdef 1[01]
		//   01 05             (Recurse.U) value 5[05]
		//   01 02             (Recurse.R) ref 1[02]
		//   00                (end)
		// Not only is the type recursive, the value is as well.
		"RecurseCycle",
		v{recurseCycle},
		"ff81041a014200" +
			"ff8325180102013101015500014101015200" +
			"0113766579726f6e322f766f6d2e5265637572736500" +
			"ff8206010105010200",
		`["type","v.io/core/veyron2/vom.Recurse struct{U uint;R *Recurse}"]
			["*Recurse",[1,{"U":5,"R":[1]}]]
			`,
		nil},
	{
		// ff81 27           typedef 65[ff81] len[27]
		// 18                (StructType)
		//   01 02             (StructType.Fields) 2 fields
		//     01 31             (FieldType.Type 0) uint
		//     01 02 5561        (FieldType.Name 0) "Ua"
		//     00                (FieldType end  0)
		//     01 42             (FieldType.Type 0) typeid 66
		//     01 01 42          (FieldType.Name 0) "B"
		//     00                (FieldType end  0)
		//   01 14 766579726f6e322f766f6d2e5265637572736541
		//                     (StructType.Name) "v.io/core/veyron2/vom.RecurseA"
		// 00                (StructType end)
		//
		// ff83 04           typedef 66[ff83] len[04]
		// 1a                (PtrType)
		//   01 43             (PtrType.Elem) typeid 67
		// 00                (PtrType end)
		//
		// ff85 27           typedef 67[ff85] len[27]
		// 18                (StructType)
		//   01 02             (StructType.Fields) 2 fields
		//     01 31             (FieldType.Type 0) uint
		//     01 02 5562        (FieldType.Name 0) "Ub"
		//     00                (FieldType end  0)
		//     01 44             (FieldType.Type 0) typeid 68
		//     01 01 41          (FieldType.Name 0) "A"
		//     00                (FieldType end  0)
		//   01 14 766579726f6e322f766f6d2e5265637572736542
		//                     (StructType.Name) "v.io/core/veyron2/vom.RecurseB"
		// 00                (StructType end)
		//
		// ff87 04           typedef 68[ff87] len[04]
		// 1a                (PtrType)
		//   01 41             (PtrType.Elem) typeid 65
		// 00                (PtrType end)
		//
		// ff82 08           type 65[ff82] len[08]
		//   01 01             (RecurseA.Ua) value 1[01]
		//   01 01             (RecurseA.B) refdef 1[01]
		//     01 02             (RecurseB.Ub) value 2[02]
		//     00                (end)
		//   00                (end)
		"RecurseAB",
		v{RecurseA{1, &RecurseB{2, nil}}},
		"ff812718010201310102556100014201014200" +
			"0114766579726f6e322f766f6d2e526563757273654100" +
			"ff83041a014300" +
			"ff852718010201310102556200014401014100" +
			"0114766579726f6e322f766f6d2e526563757273654200" +
			"ff87041a014100" +
			"ff82080101010101020000",
		`["type","v.io/core/veyron2/vom.RecurseB struct{Ub uint;A *veyron2/vom.RecurseA}"]
			["type","v.io/core/veyron2/vom.RecurseA struct{Ua uint;B *RecurseB}"]
			["RecurseA",{"Ua":1,"B":[1,{"Ub":2}]}]
			`,
		nil},
	{
		// ff81 04           typedef 65[ff81] len[04]
		// 1a                (PtrType)
		//   01 42             (PtrType.Elem) typeid 66
		// 00                (PtrType end)
		//
		// ff83 27           typedef 66[ff83] len[27]
		// 18                (StructType)
		//   01 02             (StructType.Fields) 2 fields
		//     01 31             (FieldType.Type 0) uint
		//     01 02 5561        (FieldType.Name 0) "Ua"
		//     00                (FieldType end  0)
		//     01 43             (FieldType.Type 0) typeid 67
		//     01 01 42          (FieldType.Name 0) "B"
		//     00                (FieldType end  0)
		//   01 14 766579726f6e322f766f6d2e5265637572736541
		//                     (StructType.Name) "v.io/core/veyron2/vom.RecurseA"
		// 00                (StructType end)
		//
		// ff85 04           typedef 67[ff85] len[04]
		// 1a                (PtrType)
		//   01 44             (PtrType.Elem) typeid 68
		// 00                (PtrType end)
		//
		// ff87 27           typedef 68[ff87] len[27]
		// 18                (StructType)
		//   01 02             (StructType.Fields) 2 fields
		//     01 31             (FieldType.Type 0) uint
		//     01 02 5562        (FieldType.Name 0) "Ub"
		//     00                (FieldType end  0)
		//     01 41             (FieldType.Type 0) typeid 65
		//     01 01 41          (FieldType.Name 0) "A"
		//     00                (FieldType end  0)
		//   01 14 766579726f6e322f766f6d2e5265637572736542
		//                     (StructType.Name) "v.io/core/veyron2/vom.RecurseB"
		// 00                (StructType end)
		//
		// ff82 0b           type 65[ff82] len[0b]
		// 01                refdef 1[01]
		//   01 05             (RecurseA.Ua) value 5[05]
		//   01 03             (RecurseA.B) refdef 2[03]
		//     01 06             (RecurseB.Ub) value 6[06]
		//     01 02             (RecurseB.A) ref 1[02]
		//     00                (end)
		//   00                (end)
		// Not only is the type recursive, the value is as well.
		"RecurseABCycle",
		v{recurseABCycle},
		"ff81041a014200" +
			"ff832718010201310102556100014301014200" +
			"0114766579726f6e322f766f6d2e526563757273654100" +
			"ff85041a014400" +
			"ff872718010201310102556200014101014100" +
			"0114766579726f6e322f766f6d2e526563757273654200" +
			"ff820b0101050103010601020000",
		`["type","v.io/core/veyron2/vom.RecurseB struct{Ub uint;A *veyron2/vom.RecurseA}"]
			["type","v.io/core/veyron2/vom.RecurseA struct{Ua uint;B *RecurseB}"]
			["*RecurseA",[1,{"Ua":5,"B":[2,{"Ub":6,"A":[1]}]}]]
			`,
		nil},

	// Tests of user-defined VomEncode and VomDecode methods.
	{
		// ff81 1b           typedef 65[ff81] len[1b]
		// 10                (NamedPrimitiveType)
		//   01 03             (NamedPrimitiveType.Type) string
		//   01 15 766579726f6e322f766f6d2e55736572436f646572
		//                     (NamedPrimitiveType.Name) "v.io/core/veyron2/vom.UserCoder"
		// 00                (NamedPrimitiveType end)
		//
		// ff82 02 01 39     type 65[ff82] len[02] value "9"[0139]
		"UserCoder",
		v{UserCoder{9}},
		"ff811b100103" +
			"0115766579726f6e322f766f6d2e55736572436f64657200" +
			"ff82020139",
		`["type","v.io/core/veyron2/vom.UserCoder string"]
			["UserCoder","9"]
			`,
		nil},
	{
		// ff81 04           typedef 65[ff81] len[04]
		// 1a                (PtrType)
		//   01 42             (PtrType.Elem) typeid 66
		// 00                (PtrType end)
		//
		// ff83 1b           typedef 66[ff83] len[1b]
		// 10                (NamedPrimitiveType)
		//   01 03             (NamedPrimitiveType.Type) string
		//   01 15 766579726f6e322f766f6d2e55736572436f646572
		//                     (NamedPrimitiveType.Name) "v.io/core/veyron2/vom.UserCoder"
		// 00                (NamedPrimitiveType end)
		//
		// ff82 03 01 01 39  type 65[ff82] len[03] refdef 1[01] value "9"[0139]
		"PtrUserCoder",
		v{&UserCoder{9}},
		"ff81041a014200" +
			"ff831b100103" +
			"0115766579726f6e322f766f6d2e55736572436f64657200" +
			"ff8203010139",
		`["type","v.io/core/veyron2/vom.UserCoder string"]
			["*UserCoder",[1,"9"]]
			`,
		nil},
	{
		// ff81 04           typedef 65[ff81] len[04]
		// 1a                (PtrType)
		//   01 42             (PtrType.Elem) typeid 66
		// 00                (PtrType end)
		//
		// ff83 1b           typedef 66[ff83] len[1b]
		// 10                (NamedPrimitiveType)
		//   01 03             (NamedPrimitiveType.Type) string
		//   01 15 766579726f6e322f766f6d2e55736572436f646572
		//                     (NamedPrimitiveType.Name) "v.io/core/veyron2/vom.UserCoder"
		// 00                (NamedPrimitiveType end)
		//
		// ff82 01 00        type 65[ff82] len[01] nil[00]
		"PtrUserCoderNil",
		v{(*UserCoder)(nil)},
		"ff81041a014200" +
			"ff831b100103" +
			"0115766579726f6e322f766f6d2e55736572436f64657200" +
			"ff820100",
		`["type","v.io/core/veyron2/vom.UserCoder string"]
			["*UserCoder",null]
			`,
		nil},
	{
		// ff81 04           typedef 65[ff81] len[04]
		// 1a                (PtrType)
		//   01 42             (PtrType.Elem) typeid 66
		// 00                (PtrType end)
		//
		// ff83 24           typedef 66[ff83] len[24]
		// 10                (NamedPrimitiveType)
		//   01 03             (NamedPrimitiveType.Type) string
		//   01 1e 766579726f6e322f766f6d2e55736572436f646572507472456e636f6465
		//                     (NamedPrimitiveType.Name) "v.io/core/veyron2/vom.UserCoderPtrEncode"
		// 00                (NamedPrimitiveType end)
		//
		// ff82 03 01 01 39  type 65[ff82] len[03] refdef 1[01] value "9"[0139]
		"UserCoderPtrEncode",
		v{&UserCoderPtrEncode{9}},
		"ff81041a014200" +
			"ff8324100103" +
			"011e766579726f6e322f766f6d2e55736572436f646572507472456e636f646500" +
			"ff8203010139",
		`["type","v.io/core/veyron2/vom.UserCoderPtrEncode string"]
			["*UserCoderPtrEncode",[1,"9"]]
			`,
		nil},
	{
		// ff81 04           typedef 65[ff81] len[04]
		// 1a                (PtrType)
		//   01 42             (PtrType.Elem) typeid 66
		// 00                (PtrType end)
		//
		// ff83 24           typedef 66[ff83] len[24]
		// 10                (NamedPrimitiveType)
		//   01 03             (NamedPrimitiveType.Type) string
		//   01 1e 766579726f6e322f766f6d2e55736572436f646572507472456e636f6465
		//                     (NamedPrimitiveType.Name) "v.io/core/veyron2/vom.UserCoderPtrEncode"
		// 00                (NamedPrimitiveType end)
		//
		// ff82 01 00        type 65[ff82] len[01] nil[00]
		"UserCoderPtrEncodeNil",
		v{(*UserCoderPtrEncode)(nil)},
		"ff81041a014200" +
			"ff8324100103" +
			"011e766579726f6e322f766f6d2e55736572436f646572507472456e636f646500" +
			"ff820100",
		`["type","v.io/core/veyron2/vom.UserCoderPtrEncode string"]
			["*UserCoderPtrEncode",null]
			`,
		nil},
	{
		// ff81 22           typedef 65[ff81] len[22]
		// 1a                (PtrType)
		//   01 03             (Ptr.Type) string
		//   01 1c 766579726f6e322f766f6d2e55736572436f64657250747254797065
		//                     (PtrType.Name) "v.io/core/veyron2/vom.UserCoderPtrType"
		// 00                (PtrType end)
		//
		// ff82 03 01 01 39  type 65[ff82] len[03] refdef 1[01] value "9"[0139]
		"UserCoderPtrType",
		v{UserCoderPtrType{9}},
		"ff81221a0103" +
			"011c766579726f6e322f766f6d2e55736572436f6465725074725479706500" +
			"ff8203010139",
		`["type","v.io/core/veyron2/vom.UserCoderPtrType *string"]
			["UserCoderPtrType",[1,"9"]]
			`,
		nil},
	{
		// ff81 04           typedef 65[ff81] len[04]
		// 1a                (PtrType)
		//   01 42             (PtrType.Elem) typeid 66
		// 00                (PtrType end)
		//
		// ff83 22           typedef 66[ff83] len[22]
		// 1a                (PtrType)
		//   01 03             (Ptr.Type) string
		//   01 1c 766579726f6e322f766f6d2e55736572436f64657250747254797065
		//                     (PtrType.Name) "v.io/core/veyron2/vom.UserCoderPtrType"
		// 00                (PtrType end)
		//
		// ff82 04           type 65[ff82] len[04]
		//  01 03 01 39        refdef 1[01] refdef 2[03] value "9"[0139]
		"PtrUserCoderPtrType",
		v{&UserCoderPtrType{9}},
		"ff81041a014200" +
			"ff83221a0103" +
			"011c766579726f6e322f766f6d2e55736572436f6465725074725479706500" +
			"ff820401030139",
		`["type","v.io/core/veyron2/vom.UserCoderPtrType *string"]
			["*UserCoderPtrType",[1,[2,"9"]]]
			`,
		nil},
	{
		// ff81 04           typedef 65[ff81] len[04]
		// 1a                (PtrType)
		//   01 42             (PtrType.Elem) typeid 66
		// 00                (PtrType end)
		//
		// ff83 22           typedef 66[ff83] len[22]
		// 1a                (PtrType)
		//   01 03             (Ptr.Type) string
		//   01 1c 766579726f6e322f766f6d2e55736572436f64657250747254797065
		//                     (PtrType.Name) "v.io/core/veyron2/vom.UserCoderPtrType"
		// 00                (PtrType end)
		//
		// ff82 01 00        type 65[ff82] len[01] nil[00]
		"PtrUserCoderPtrTypeNil",
		v{(*UserCoderPtrType)(nil)},
		"ff81041a014200" +
			"ff83221a0103" +
			"011c766579726f6e322f766f6d2e55736572436f6465725074725479706500" +
			"ff820100",
		`["type","v.io/core/veyron2/vom.UserCoderPtrType *string"]
			["*UserCoderPtrType",null]
			`,
		nil},

	// Tests of user-defined Gob / JSON coder methods.
	{
		// ff81 04           typedef 65[ff81] len[04]
		// 1a                (PtrType)
		//   01 42             (PtrType.Elem) typeid 66
		// 00                (PtrType end)
		//
		// ff83 22           typedef 66[ff83] len[22]
		// 10                (NamedPrimitiveType)
		//   01 04             (NamedPrimitiveType.Type) []byte
		//   01 1c 766579726f6e322f766f6d2e55736572436f646572476f624a534f4e
		//                     (NamedPrimitiveType.Name) "v.io/core/veyron2/vom.UserCoderGobJSON"
		// 00                (NamedPrimitiveType end)
		//
		// ff82 03 01 01 39  type 65[ff82] len[03] refdef 1[01] value "9"[0139]
		"UserCoderGobJSON",
		v{&UserCoderGobJSON{9}},
		"ff81041a014200" +
			"ff8322100104" +
			"011c766579726f6e322f766f6d2e55736572436f646572476f624a534f4e00" +
			"ff8203010139",
		`["type","v.io/core/veyron2/vom.UserCoderGobJSON string"]
			["*UserCoderGobJSON",[1,"9"]]
			`,
		nil},
	// TODO(bprosnitz) Make unnamed complex decoding work in JSON.
	/*
	   	// Complex values are special-cased and always encoded with a typedef to
	   	// identify the type.  The value is encoded as a struct{r float64;i float64}
	   	// with a special tag "complex" to identify the type for the decoder.
	   	{
	   		// ff81 27           typedef 65[ff81] len[27]
	   		// 18                (StructType)
	   		//   01 02             (StructType.Fields) 2 fields
	   		//     01 19             (FieldType.Type 0) float32
	   		//     01 01 52          (FieldType.Name 0) "R"
	   		//     00                (FieldType end  0)
	   		//     01 19             (FieldType.Type 1) float32
	   		//     01 01 49          (FieldType.Name 1) "I"
	   		//     00                (FieldType end  1)
	   		//   01 09 636f6d706c65783634
	   		//                     (StructType.Name) "complex64"
	   		//   01 01 09 636f6d706c65783634
	   		//                     (StructType.Tags) []{"complex64"}
	   		// 00                (StructType end)
	   		//
	   		// ff82 07           type 65[ff82] len[07]
	   		//   01 fef03f         (complex.R) 1[fef03f]
	   		//   01 40             (complex.I) 2[40]
	   		// 00                (end)
	   		"UnnamedComplex64",
	   		v{complex64(1 + 2i)},
	   		"ff81271801020119010152000119010149000109636f6d706c65783634" +
	   			"010109636f6d706c6578363400" +
	   			"ff820701fef03f014000",
	   		`["type","complex64 struct{R float32;I float32}",["complex64"]]
	   ["complex64",{"R":1,"I":2}]
	   `},
	*/
	{
		// ff81 33           typedef 65[ff81] len[33]
		// 18                (StructType)
		//   01 02             (StructType.Fields) 2 fields
		//     01 19             (FieldType.Type 0) float32
		//     01 01 52          (FieldType.Name 0) "R"
		//     00                (FieldType end  0)
		//     01 19             (FieldType.Type 1) float32
		//     01 01 49          (FieldType.Name 1) "I"
		//     00                (FieldType end  1)
		//   01 15 766579726f6e322f766f6d2e436f6d706c65783634
		//                     (StructType.Name) "v.io/core/veyron2/vom.Complex64"
		//   01 01 09 636f6d706c65783634
		//                     (StructType.Tags) []{"complex64"}
		// 00                (StructType end)
		//
		// ff82 07           type 65[ff82] len[07]
		//   01 fef03f         (Complex.R) 1[fef03f]
		//   01 40             (Complex.I) 2[40]
		// 00                (end)
		"Complex64",
		v{Complex64(1 + 2i)},
		"ff8133180102011901015200011901014900" +
			"0115766579726f6e322f766f6d2e436f6d706c65783634" +
			"010109636f6d706c6578363400" +
			"ff820701fef03f014000",
		`["type","v.io/core/veyron2/vom.Complex64 struct{R float32;I float32}",["complex64"]]
	["Complex64",{"R":1,"I":2}]
	`,
		nil},

	// TODO(bprosnitz) Make unnamed complex decoding work in JSON.
	/*
	   	{
	   		// ff81 29           typedef 65[ff81] len[29]
	   		// 18                (StructType)
	   		//   01 02             (StructType.Fields) 2 fields
	   		//     01 1a             (FieldType.Type 0) float64
	   		//     01 01 52          (FieldType.Name 0) "R"
	   		//     00                (FieldType end  0)
	   		//     01 1a             (FieldType.Type 1) float64
	   		//     01 01 49          (FieldType.Name 1) "I"
	   		//     00                (FieldType end  1)
	   		//   01 0a 636f6d706c6578313238
	   		//                     (StructType.Name) "complex128"
	   		//   01 01 0a 636f6d706c6578313238
	   		//                     (StructType.Tags) []{"complex128"}
	   		// 00                (StructType end)
	   		//
	   		// ff82 07           type 65[ff82] len[07]
	   		//   01 fef03f         (complex.R) 1[fef03f]
	   		//   01 40             (complex.I) 2[40]
	   		// 00                (end)
	   		"UnnamedComplex128",
	   		v{complex128(1 + 2i)},
	   		"ff8129180102011a01015200011a01014900010a636f6d706c6578313238" +
	   			"01010a636f6d706c657831323800" +
	   			"ff820701fef03f014000",
	   		`["type","complex128 struct{R float64;I float64}",["complex128"]]
	   ["complex128",{"R":1,"I":2}]
	   `},
	*/
	{
		// ff81 35           typedef 65[ff81] len[35]
		// 18                (StructType)
		//   01 02             (StructType.Fields) 2 fields
		//     01 1a             (FieldType.Type 0) float64
		//     01 01 52          (FieldType.Name 0) "R"
		//     00                (FieldType end  0)
		//     01 1a             (FieldType.Type 1) float64
		//     01 01 49          (FieldType.Name 1) "I"
		//     00                (FieldType end  1)
		//   01 16 766579726f6e322f766f6d2e436f6d706c6578313238
		//                     (StructType.Name) "v.io/core/veyron2/vom.Complex128"
		//   01 01 0a 636f6d706c6578313238
		//                     (StructType.Tags) []{"complex128"}
		// 00                (StructType end)
		//
		// ff82 07           type 65[ff82] len[07]
		//   01 fef03f         (Complex.R) 1[fef03f]
		//   01 40             (Complex.I) 2[40]
		// 00                (end)
		"Complex128",
		v{Complex128(1 + 2i)},
		"ff8135180102011a01015200011a01014900" +
			"0116766579726f6e322f766f6d2e436f6d706c6578313238" +
			"01010a636f6d706c657831323800" +
			"ff820701fef03f014000",
		`["type","v.io/core/veyron2/vom.Complex128 struct{R float64;I float64}",["complex128"]]
		["Complex128",{"R":1,"I":2}]
		`,
		nil},

	// Interfaces are encoded as regular values, with the concrete type id in the
	// value to identify the concrete value.
	{
		// ff81 04           typedef 65[ff81] len[04]
		// 1a                (PtrType)
		//   01 01             (PtrType.Elem) interface[01]
		// 00                (PtrType end)
		//
		// ff82 03           type 65[ff82] len[03]
		//   01 62 01          refdef 1[01] type uint[62] value 1[01]
		"InterfaceUint",
		v{&ifaceUint},
		"ff81041a010100" +
			"ff8203016201",
		`["*interface",[1,["uint",1]]]
		`,
		nil,
	},
	{
		// ff81 04           typedef 65[ff81] len[04]
		// 1a                (PtrType)
		//   01 01             (PtrType.Elem) interface[01]
		// 00                (PtrType end)
		//
		// ff82 03           type 65[ff82] len[03]
		//   01 62 01          refdef 1[01] type uint[62] value 1[01]
		//
		// ff82 03           type 65[ff82] len[03]
		//   01 62 01          refdef 1[01] type uint[62] value 1[01]
		"InterfaceUintRepeated",
		v{&ifaceUint, &ifaceUint},
		"ff81041a010100" +
			"ff8203016201" +
			"ff8203016201",
		`["*interface",[1,["uint",1]]]
		["*interface",[1,["uint",1]]]
		`,
		nil,
	},
	{
		// ff81 04           typedef 65[ff81] len[04]
		// 1a                (PtrType)
		//   01 01             (PtrType.Elem) interface[01]
		// 00                (PtrType end)
		//
		// ff83 1e           typedef 66[ff83] len[1e]
		// 18                (StructType)
		//   01 01             (StructType.Fields) 1 field
		//     01 31             (FieldType.Type 0) uint
		//     01 01 41          (FieldType.Name 0) "A"
		//     00                (FieldType end  0)
		//   01 12 766579726f6e322f766f6d2e537472756374
		//                     (StructType.Name) "v.io/core/veyron2/vom.Struct"
		// 00                (StructType end)
		//
		// ff82 06           type 65[ff82] len[06]
		//   01 ff84           refdef 1[01] type 66[ff84]
		//   01 01             (Struct.A) value 1[01]
		//   00                (Struct end)
		"InterfaceStruct",
		v{&ifaceStruct},
		"ff81041a010100" +
			"ff831e180101013101014100" +
			"0112766579726f6e322f766f6d2e53747275637400" +
			"ff820601ff84010100",
		`["type","v.io/core/veyron2/vom.Struct struct{A uint}"]
		["*interface",[1,["Struct",{"A":1}]]]
		`,
		nil,
	},
	{
		// ff81 04           typedef 65[ff81] len[04]
		// 1a                (PtrType)
		//   01 01             (PtrType.Elem) interface[01]
		// 00                (PtrType end)
		//
		// ff83 27           typedef 66[ff83] len[27]
		// 18                (StructType)
		//   01 02             (StructType.Fields) 2 fields
		//     01 31             (FieldType.Type 0) uint
		//     01 02 5561        (FieldType.Name 0) "Ua"
		//     00                (FieldType end  0)
		//     01 43             (FieldType.Type 0) typeid 67
		//     01 01 42          (FieldType.Name 0) "B"
		//     00                (FieldType end  0)
		//   01 14 766579726f6e322f766f6d2e5265637572736541
		//                     (StructType.Name) "v.io/core/veyron2/vom.RecurseA"
		// 00                (StructType end)
		//
		// ff85 04           typedef 67[ff85] len[04]
		// 1a                (PtrType)
		//   01 44             (PtrType.Elem) typeid 68
		// 00                (PtrType end)
		//
		// ff87 27           typedef 68[ff87] len[27]
		// 18                (StructType)
		//   01 02             (StructType.Fields) 2 fields
		//     01 31             (FieldType.Type 0) uint
		//     01 02 5562        (FieldType.Name 0) "Ub"
		//     00                (FieldType end  0)
		//     01 45             (FieldType.Type 0) typeid 69
		//     01 01 41          (FieldType.Name 0) "A"
		//     00                (FieldType end  0)
		//   01 14 766579726f6e322f766f6d2e5265637572736542
		//                     (StructType.Name) "v.io/core/veyron2/vom.RecurseB"
		// 00                (StructType end)
		//
		// ff89 04           typedef 69[ff89] len[04]
		// 1a                (PtrType)
		//   01 42             (PtrType.Elem) typeid 66
		// 00                (PtrType end)
		//
		// ff82 0b           type 65[ff82] len[0b]
		//   01 ff84           refdef 1[01] type 66[ff84]
		//   01 01             (RecurseA.Ua) value 1[01]
		//   01 03             (RecurseA.B) refdef 2[03]
		//     01 02             (RecurseB.Ub) value 2[02]
		//     00                (RecurseB end)
		//   00                (RecurseA end)
		"InterfaceRecurseAB",
		v{&ifaceRecurseAB},
		"ff81041a010100" +
			"ff832718010201310102556100014301014200" +
			"0114766579726f6e322f766f6d2e526563757273654100" +
			"ff85041a014400" +
			"ff872718010201310102556200014501014100" +
			"0114766579726f6e322f766f6d2e526563757273654200" +
			"ff89041a014200" +
			"ff820b01ff840101010301020000",
		`["type","v.io/core/veyron2/vom.RecurseB struct{Ub uint;A *veyron2/vom.RecurseA}"]
		["type","v.io/core/veyron2/vom.RecurseA struct{Ua uint;B *RecurseB}"]
		["*interface",[1,["RecurseA",{"Ua":1,"B":[2,{"Ub":2}]}]]]
		`, nil},
	{
		// ff81 27           typedef 65[ff81] len[27]
		// 18                (StructType)
		//   01 01             (StructType.Fields) 1 field
		//     01 01             (FieldType.Type 0) interface
		//     01 01 49          (FieldType.Name 0) "I"
		//     00                (FieldType end  0)
		//   01 1b 766579726f6e322f766f6d2e4e6573746564496e74657266616365
		//                     (StructType.Name) "v.io/core/veyron2/vom.NestedInterface"
		// 00                (StructType end)
		//
		// ff83 1e           typedef 66[ff83] len[1e]
		// 18                (StructType)
		//   01 01             (StructType.Fields) 1 field
		//     01 31             (FieldType.Type 0) uint
		//     01 01 41          (FieldType.Name 0) "A"
		//     00                (FieldType end  0)
		//   01 12 766579726f6e322f766f6d2e537472756374
		//                     (StructType.Name) "v.io/core/veyron2/vom.Struct"
		// 00                (StructType end)
		//
		// ff82 07           type 65[ff82] len[07]
		//   01 ff84           (NestedInterface.I) type 66[ff84]
		//     01 01             (Struct.A) value 1[01]
		//     00                (Struct end)
		//   00                (NestedInterface end)
		"NestedInterface",
		v{NestedInterface{ifaceStruct}},
		"ff8127180101010101014900" +
			"011b766579726f6e322f766f6d2e4e6573746564496e7465726661636500" +
			"ff831e180101013101014100" +
			"0112766579726f6e322f766f6d2e53747275637400" +
			"ff820701ff8401010000",
		`["type","v.io/core/veyron2/vom.NestedInterface struct{I interface}"]
		["type","v.io/core/veyron2/vom.Struct struct{A uint}"]
		["NestedInterface",{"I":["Struct",{"A":1}]}]
		`,
		j{`{"i":{"a":1}}`},
	},
	{
		// ff81 04           typedef 65[ff81] len[04]
		// 1a                (PtrType)
		//   01 01             (PtrType.Elem) interface[01]
		// 00                (PtrType end)
		//
		// ff83 27           typedef 66[ff83] len[27]
		// 18                (StructType)
		//   01 01             (StructType.Fields) 1 field
		//     01 01             (FieldType.Type 0) interface
		//     01 01 49          (FieldType.Name 0) "I"
		//     00                (FieldType end  0)
		//   01 1b 766579726f6e322f766f6d2e4e6573746564496e74657266616365
		//                     (StructType.Name) "v.io/core/veyron2/vom.NestedInterface"
		// 00                (StructType end)
		//
		// ff85 1e           typedef 67[ff85] len[1e]
		// 18                (StructType)
		//   01 01             (StructType.Fields) 1 field
		//     01 31             (FieldType.Type 0) uint
		//     01 01 41          (FieldType.Name 0) "A"
		//     00                (FieldType end  0)
		//   01 12 766579726f6e322f766f6d2e537472756374
		//                     (StructType.Name) "v.io/core/veyron2/vom.Struct"
		// 00                (StructType end)
		//
		// ff82 0a           type 65[ff82] len[0a]
		//   01 ff84           refdef 1[01] type 66[ff84]
		//     01 ff86           (NestedInterface.I) type 67[ff86]
		//       01 01             (Struct.A) value 1[01]
		//       00                (Struct end)
		//     00                (NestedInterface end)
		"InterfaceNestedInterface",
		v{&ifaceNestedIface},
		"ff81041a010100" +
			"ff8327180101010101014900" +
			"011b766579726f6e322f766f6d2e4e6573746564496e7465726661636500" +
			"ff851e180101013101014100" +
			"0112766579726f6e322f766f6d2e53747275637400" +
			"ff820a01ff8401ff8601010000",
		`["type","v.io/core/veyron2/vom.NestedInterface struct{I interface}"]
		["type","v.io/core/veyron2/vom.Struct struct{A uint}"]
		["*interface",[1,["NestedInterface",{"I":["Struct",{"A":1}]}]]]
		`,
		nil,
	},
	{
		// ff81 27           typedef 65[ff81] len[27]
		// 18                (StructType)
		//   01 01             (StructType.Fields) 1 field
		//     01 01             (FieldType.Type 0) interface
		//     01 01 49          (FieldType.Name 0) "I"
		//     00                (FieldType end  0)
		//   01 1b 766579726f6e322f766f6d2e4e6573746564496e74657266616365
		//                     (StructType.Name) "v.io/core/veyron2/vom.NestedInterface"
		// 00                (StructType end)
		//
		// ff83 1e           typedef 66[ff83] len[1e]
		// 18                (StructType)
		//   01 01             (StructType.Fields) 1 field
		//     01 31             (FieldType.Type 0) uint
		//     01 01 41          (FieldType.Name 0) "A"
		//     00                (FieldType end  0)
		//   01 12 766579726f6e322f766f6d2e537472756374
		//                     (StructType.Name) "v.io/core/veyron2/vom.Struct"
		// 00                (StructType end)
		//
		// ff82 0b           type 65[ff82] len[0b]
		//   01 ff82           (NestedInterface.I) type 65[ff82]
		//     01 ff84           (NestedInterface.I) type 66[ff84]
		//       01 01             (Struct.A) value 1[01]
		//       00                (Struct end)
		//     00                (NestedInterface end)
		//   00                (NestedInterface end)
		"NestedInterfaceNestedInterface",
		v{NestedInterface{ifaceNestedIface}},
		"ff8127180101010101014900" +
			"011b766579726f6e322f766f6d2e4e6573746564496e7465726661636500" +
			"ff831e180101013101014100" +
			"0112766579726f6e322f766f6d2e53747275637400" +
			"ff820b01ff8201ff840101000000",
		`["type","v.io/core/veyron2/vom.NestedInterface struct{I interface}"]
		["type","v.io/core/veyron2/vom.Struct struct{A uint}"]
		["NestedInterface",{"I":["NestedInterface",{"I":["Struct",{"A":1}]}]}]
		`,
		j{`{"i":{"i":{"a":1}}}`},
	},
	{
		// ff81 27           typedef 65[ff81] len[27]
		// 18                (StructType)
		//   01 01             (StructType.Fields) 1 field
		//     01 01             (FieldType.Type 0) interface
		//     01 01 49          (FieldType.Name 0) "I"
		//     00                (FieldType end  0)
		//   01 1b 766579726f6e322f766f6d2e4e6573746564496e74657266616365
		//                     (StructType.Name) "v.io/core/veyron2/vom.NestedInterface"
		// 00                (StructType end)
		//
		// ff83 1e           typedef 66[ff83] len[1e]
		// 18                (StructType)
		//   01 01             (StructType.Fields) 1 field
		//     01 31             (FieldType.Type 0) uint
		//     01 01 41          (FieldType.Name 0) "A"
		//     00                (FieldType end  0)
		//   01 12 766579726f6e322f766f6d2e537472756374
		//                     (StructType.Name) "v.io/core/veyron2/vom.Struct"
		// 00                (StructType end)
		//
		// ff82 0b           type 65[ff82] len[0b]
		//   01 ff82           (NestedInterface.I) type 65[ff82]
		//     01 ff84           (NestedInterface.I) type 66[ff84]
		//       01 01             (Struct.A) value 1[01]
		//       00                (Struct end)
		//     00                (NestedInterface end)
		//   00                (NestedInterface end)
		//
		// ff82 0b           type 65[ff82] len[0b]
		//   01 ff82           (NestedInterface.I) type 65[ff82]
		//     01 ff84           (NestedInterface.I) type 66[ff84]
		//       01 01             (Struct.A) value 1[01]
		//       00                (Struct end)
		//     00                (NestedInterface end)
		//   00                (NestedInterface end)
		"NestedInterfaceNestedInterfaceRepeated",
		v{NestedInterface{ifaceNestedIface}, NestedInterface{ifaceNestedIface}},
		"ff8127180101010101014900" +
			"011b766579726f6e322f766f6d2e4e6573746564496e7465726661636500" +
			"ff831e180101013101014100" +
			"0112766579726f6e322f766f6d2e53747275637400" +
			"ff820b01ff8201ff840101000000" +
			"ff820b01ff8201ff840101000000",
		`["type","v.io/core/veyron2/vom.NestedInterface struct{I interface}"]
		["type","v.io/core/veyron2/vom.Struct struct{A uint}"]
		["NestedInterface",{"I":["NestedInterface",{"I":["Struct",{"A":1}]}]}]
		["NestedInterface",{"I":["NestedInterface",{"I":["Struct",{"A":1}]}]}]
		`,
		j{`{"i":{"i":{"a":1}}}`, `{"i":{"i":{"a":1}}}`},
	},
	{
		// ff81 04           typedef 65[ff81] len[04]
		// 1a                (PtrType)
		//   01 42             (PtrType.Elem) typeid 66
		// 00                (PtrType end)
		//
		// ff83 17           typedef 66[ff83] len[17]
		// 10                (NamedPrimitiveType)
		//   01 01             (NamedPrimitiveType.Type) interface
		//   01 11 766579726f6e322f766f6d2e466f6f6572
		//                     (NamedPrimitiveType.Name) "v.io/core/veyron2/vom.Fooer"
		// 00                (NamedPrimitiveType end)
		//
		// ff83 1d           typedef 67[ff85] len[1d]
		// 10                (NamedPrimitiveType)
		//   01 21             (NamedPrimitiveType.Type) int
		//   01 17 766579726f6e322f766f6d2e436f6e6372657465466f6f00
		//                     (NamedPrimitiveType.Name) "v.io/core/veyron2/vom.ConcreteFoo"
		// 00                (NamedPrimitiveType end)
		//
		// ff82 04           type 65[ff82] len[04]
		//   01 ff86           refdef 1[01] type 67[ff86]
		//   06                value 3[06]
		"NonEmptyInterface",
		v{&fooer},
		"ff81041a014200" +
			"ff83171001010111766579726f6e322f766f6d2e466f6f657200" +
			"ff851d1001210117766579726f6e322f766f6d2e436f6e6372657465466f6f00" +
			"ff820401ff8606",
		`["type","v.io/core/veyron2/vom.Fooer interface"]
		["type","v.io/core/veyron2/vom.ConcreteFoo int"]
		["*Fooer",[1,["ConcreteFoo",3]]]
		`,
		nil,
	},

	// Byte slices and arrays.
	{
		// ff81 1b           typedef 65[ff81] len[1b]
		// 10                (NamedPrimitiveType)
		//   01 04             (NamedPrimitiveType.Type) []byte
		//   01 15 766579726f6e322f766f6d2e42797465536c696365
		//                     (NamedPrimitiveType.Name) "v.io/core/veyron2/vom.ByteSlice"
		// 00                (NamedPrimitiveType end)
		//
		// ff82 04 03616263  type 65[ff82] len[04] value "abc"[03616263]
		"ByteSlice",
		v{ByteSlice("abc")},
		"ff811b1001040115766579726f6e322f766f6d2e42797465536c69636500" +
			"ff820403616263",
		`["type","v.io/core/veyron2/vom.ByteSlice []byte"]
		["ByteSlice","YWJj"]
		`,
		j{`[97,98,99]`},
	},
	{
		// ff81 20           typedef 65[ff81] len[20]
		// 12                (SliceType)
		//   01 42             (SliceType.Elem) typeid 66
		//   01 1a 766579726f6e322f766f6d2e4e616d656442797465536c696365
		//                     (SliceType.Name) "v.io/core/veyron2/vom.NamedByteSlice"
		// 00                (SliceType end)
		//
		// ff83 16           typedef 66[ff82] len[16]
		// 10                (NamedPrimitiveType)
		//   01 32             (NamedPrimitiveType.Type) uint8
		//   01 10 766579726f6e322f766f6d2e42797465
		//                     (NamedPrimitiveType.Name) "v.io/core/veyron2/vom.Byte"
		// 00                (NamedPrimitiveType end)
		//
		// ff82 04 03616263  type 65[ff82] len[04] value "abc"[03616263]
		"NamedByteSlice",
		v{NamedByteSlice("abc")},
		"ff8120120142011a766579726f6e322f766f6d2e4e616d656442797465536c69636500" +
			"ff83161001320110766579726f6e322f766f6d2e4279746500" +
			"ff820403616263",
		`["type","v.io/core/veyron2/vom.Byte byte"]
		["type","v.io/core/veyron2/vom.NamedByteSlice []Byte"]
		["NamedByteSlice","YWJj"]
		`,
		j{`[97,98,99]`},
	},
	{
		// ff81 1d           typedef 65[ff81] len[1d]
		// 14                (ArrayType)
		//   01 32             (ArrayType.Type) uint8
		//   01 03             (ArrayType.Len) 3
		//   01 15 766579726f6e322f766f6d2e427974654172726179
		//                     (ArrayType.Name) "v.io/core/veyron2/vom.ByteArray"
		// 00                (ArrayType end)
		//
		// ff82 03 616263    type 65[ff82] len[03] value "abc"[616263]
		"ByteArray",
		v{ByteArray{'a', 'b', 'c'}},
		"ff811d14013201030115766579726f6e322f766f6d2e42797465417272617900" +
			"ff8203616263",
		`["type","v.io/core/veyron2/vom.ByteArray [3]byte"]
		["ByteArray","YWJj"]
		`,
		j{`[97,98,99]`},
	},
	{
		// ff81 22           typedef 65[ff81] len[22]
		// 14                (ArrayType)
		//   01 42             (ArrayType.Type) typeid 66
		//   01 03             (ArrayType.Len) 3
		//   01 1a 766579726f6e322f766f6d2e4e616d6564427974654172726179
		//                     (ArrayType.Name) "v.io/core/veyron2/vom.NamedByteArray"
		// 00                (ArrayType end)
		//
		// ff83 16           typedef 66[ff82] len[16]
		// 10                (NamedPrimitiveType)
		//   01 32             (NamedPrimitiveType.Type) uint8
		//   01 10 766579726f6e322f766f6d2e42797465
		//                     (NamedPrimitiveType.Name) "v.io/core/veyron2/vom.Byte"
		// 00                (NamedPrimitiveType end)
		//
		// ff82 03 616263    type 65[ff82] len[03] value "abc"[616263]
		"NamedByteArray",
		v{NamedByteArray{'a', 'b', 'c'}},
		"ff81221401420103" +
			"011a766579726f6e322f766f6d2e4e616d656442797465417272617900" +
			"ff83161001320110766579726f6e322f766f6d2e4279746500" +
			"ff8203616263",
		`["type","v.io/core/veyron2/vom.Byte byte"]
		["type","v.io/core/veyron2/vom.NamedByteArray [3]Byte"]
		["NamedByteArray","YWJj"]
		`,
		j{`[97,98,99]`},
	},

	// Map with [2]string keys.  This exercises the JSON string escaping /
	// unescaping logic.
	{
		// ff81 25           typedef 65[ff81] len[25]
		// 16                (MapType)
		//   01 42             (MapType.Key) typeid 66
		//   01 03             (MapType.Elem) string
		//   01 1d 766579726f6e322f766f6d2e4d756c7469537472696e674b65794d6170
		//                     (MapType.Name) "v.io/core/veyron2/vom.MultiStringKeyMap"
		// 00                (MapType end)
		//
		// ff83 06           typedef 66[ff83] len[06]
		// 14                (ArrayType)
		//   01 03             (ArrayType.Type) string
		//   01 02             (ArrayType.Len) 2
		// 00                (ArrayType end)
		//
		// ff82 0d 01        type 65[ff82] len[0d] len[01]
		//   03616263          key   "abc"
		//   03646566                "def"
		//   03676869          value "ghi"
		"MultiStringKeyMap",
		v{MultiStringKeyMap{[2]string{"abc", "def"}: "ghi"}},
		"ff81251601420103011d766579726f6e322f766f6d2e4d756c7469537472696e674b65794d617000ff8306140103010200ff820d01036162630364656603676869",
		`["type","v.io/core/veyron2/vom.MultiStringKeyMap map[[2]string]string"]
		["MultiStringKeyMap",{"[\"abc\",\"def\"]":"ghi"}]
		`,
		j{`{"[\"abc\",\"def\"]":"ghi"}`},
	},
	{
		// ff81 25           typedef 65[ff81] len[25]
		// 16                (MapType)
		//   01 42             (MapType.Key) typeid 66
		//   01 03             (MapType.Elem) string
		//   01 1d 766579726f6e322f766f6d2e4d756c7469537472696e674b65794d6170
		//                     (MapType.Name) "v.io/core/veyron2/vom.MultiStringKeyMap"
		// 00                (MapType end)
		//
		// ff83 06           typedef 66[ff83] len[06]
		// 14                (ArrayType)
		//   01 03             (ArrayType.Type) string
		//   01 02             (ArrayType.Len) 2
		// 00                (ArrayType end)
		//
		// ff82 13 01        type 65[ff82] len[13] len[01]
		//   056122622263      key   `a"b"c`
		//   056422652266            `d"e"f`
		//   056722682269      value `g"h"i`
		"MultiStringKeyMap",
		v{MultiStringKeyMap{[2]string{`a"b"c`, `d"e"f`}: `g"h"i`}},
		"ff81251601420103011d766579726f6e322f766f6d2e4d756c7469537472696e674b65794d617000ff8306140103010200ff821301056122622263056422652266056722682269",
		`["type","v.io/core/veyron2/vom.MultiStringKeyMap map[[2]string]string"]
		["MultiStringKeyMap",{"[\"a\\\"b\\\"c\",\"d\\\"e\\\"f\"]":"g\"h\"i"}]
		`,
		j{`{"[\"a\\\"b\\\"c\",\"d\\\"e\\\"f\"]":"g\"h\"i"}`},
	},
	{
		// ff81 25           typedef 65[ff81] len[25]
		// 16                (MapType)
		//   01 42             (MapType.Key) typeid 66
		//   01 03             (MapType.Elem) string
		//   01 1d 766579726f6e322f766f6d2e4d756c7469537472696e674b65794d6170
		//                     (MapType.Name) "v.io/core/veyron2/vom.MultiStringKeyMap"
		// 00                (MapType end)
		//
		// ff83 06           typedef 66[ff83] len[06]
		// 14                (ArrayType)
		//   01 03             (ArrayType.Type) string
		//   01 02             (ArrayType.Len) 2
		// 00                (ArrayType end)
		//
		// ff82 15 01        type 65[ff82] len[15] len[01]
		//   0641080c0a0d09    key   "A\b\f\n\r\t"
		//   06080c0a0d0942          "\b\f\n\r\tB"
		//   05080c0a0d09      value "\b\f\n\r\t"
		"MultiStringKeyMap",
		v{MultiStringKeyMap{[2]string{"A\b\f\n\r\t", "\b\f\n\r\tB"}: "\b\f\n\r\t"}},
		"ff81251601420103011d766579726f6e322f766f6d2e4d756c7469537472696e674b65794d617000ff8306140103010200ff8215010641080c0a0d0906080c0a0d094205080c0a0d09",
		`["type","v.io/core/veyron2/vom.MultiStringKeyMap map[[2]string]string"]
		["MultiStringKeyMap",{"[\"A\\b\\f\\n\\r\\t\",\"\\b\\f\\n\\r\\tB\"]":"\b\f\n\r\t"}]
		`,
		j{`{"[\"A\\b\\f\\n\\r\\t\",\"\\b\\f\\n\\r\\tB\"]":"\b\f\n\r\t"}`},
	},
}
