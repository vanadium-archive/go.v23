package vom2

type v []interface{}
type j []string

var coderTests = []struct {
	Name   string
	Values v
	Binary string // Binary-encoded result as a hex pattern (see matchHexPat)
	JSON   j      // JSON-encoded result.
}{
	// Primitives.
	{
		//            TypeID        Value
		// 0601       [06]bool      [01]true
		// 0803616263 [08]string    [03616263]"abc"
		// 3403646566 [32]byteslice [03646566]"def"
		"Basic",
		v{true, string("abc"), []byte("def")},
		"80060108036162633403646566",
		j{`true`, `"abc"`, `"ZGVm"`},
	},
	{
		//            TypeID      Value
		// 0a01       [0a]byte    [01]1
		// 0c02       [0c]uint16  [02]2
		// 0e03       [0e]uint32  [03]3
		// 1004       [10]uint64  [04]4
		"Uint",
		v{byte(1), uint16(2), uint32(3), uint64(4)},
		"800a010c020e031004",
		j{`1`, `2`, `3`, `4`},
	},
	{
		//            TypeID       Value
		// 1201       [12]int16    [01]-1
		// 1403       [14]int32    [03]-2
		// 1605       [16]int64    [05]-3
		"Int",
		v{int16(-1), int32(-2), int64(-3)},
		"80120114031605",
		j{`-1`, `-2`, `-3`},
	},
	{
		//            TypeID        Value
		// 18fe0840   [18]float32   [fe0840]3.0
		// 1afe1040   [1a]float64   [fe1040]4.0
		"Float",
		v{float32(3), float64(4)},
		"8018fe08401afe1040",
		j{`17.0`, `18.0`},
	},
	{
		//                  TypeID         MsgLen  Value
		// 1c06fe0840fe1040 [1c]complex64  [06]    [fe0840fe1040]3+4i
		// 1e06fe1440fe1840 [1e]complex128 [06]    [fe1440fe1840]5+6i
		"Complex",
		v{complex(float32(3), float32(4)), complex(float64(5), float64(6))},
		"801c06fe0840fe10401e06fe1440fe1840",
		j{}, // TODO
	},
	// TODO: byte vs uint

	// Named primitives.
	{
		// ff81 17           typedef 65[ff81] msglen[17]
		// 10                (WireNamed)
		//   01 11 766579726f6e322f766f6d322e426f6f6c
		//                     (WireNamed.Name) "veyron.io/veyron/veyron2/vom2.Bool"
		//   02 03             (WireNamed.Base) bool[03]
		//   00              (WireNamed end)
		//
		// ff82 01           type 65[ff82] value true[01]
		"NamedBool",
		v{Bool(true), Bool(true)},
		"80" +
			"ff8117100111766579726f6e322f766f6d322e426f6f6c020300" +
			"ff8201ff8201",
		j{}, // TODO
	},
	{
		// ff81 19           typedef 65[ff81] msglen[19]
		// 10                (WireNamed)
		//   01 13 766579726f6e322f766f6d322e537472696e67
		//                     (WireNamed.Name) "veyron.io/veyron/veyron2/vom2.String"
		//   02 04             (WireNamed.Base) string[04]
		//   00              (WireNamed end)
		//
		// ff82 03 616263    type 65[ff82] value abc[03 616263]
		"NamedString",
		v{String("abc"), String("abc")},
		"80" +
			"ff8119100113766579726f6e322f766f6d322e537472696e67020400" +
			"ff8203616263ff8203616263",
		j{}, // TODO
	},
	// TODO: more named primitives

	// Arrays
	{
		// ff81 08           typedef 65[ff81] msglen[06]
		// 12                (WireArray)
		//   02 06             (WireArray.Elem) uint16[06]
		//   03 02             (WireArray.Len)  2[02]
		//   00              (WireArray end)
		//
		// ff82 02 0102      type 65[ff82] msglen[02] value 1,2[0102]
		"UnnamedArray",
		v{[2]uint16{1, 2}, [2]uint16{1, 2}},
		"80" +
			"ff8106120206030200" +
			"ff82020102ff82020102",
		j{}, // TODO
	},
	{
		// ff81 21           typedef 65[ff81] msglen[21]
		// 12                (WireArray)
		//   01 19 766579726f6e322f766f6d322e41727261793255696e743136
		//                     (WireArray.Name) "veyron.io/veyron/veyron2/vom2.Array2Uint16"
		//   02 06             (WireArray.Elem) uint16[06]
		//   03 02             (WireArray.Len)  2[02]
		//   00              (WireArray end)
		//
		// ff82 02 0102      type 65[ff82] msglen[02] value 1,2[0102]
		"Array2Uint16",
		v{Array2Uint16{1, 2}, Array2Uint16{1, 2}},
		"80" +
			"ff8121120119766579726f6e322f766f6d322e41727261793255696e7431360206030200" +
			"ff82020102ff82020102",
		j{}, // TODO
	},

	// Lists
	{
		// ff81 04           typedef 65[ff81] msglen[04]
		// 13                (WireList)
		//   02 06             (WireList.Elem) uint16[06]
		//   00              (WireList end)
		//
		// ff82 03 02 0102   type 65[ff82] msglen[03] value 1,2[02 0102]
		"UnnamedList",
		v{[]uint16{1, 2}, []uint16{1, 2}},
		"80" +
			"ff810413020600" +
			"ff8203020102ff8203020102",
		j{}, // TODO
	},
	{
		// ff81 1d           typedef 65[ff81] msglen[1d]
		// 13                (WireList)
		//   01 17 766579726f6e322f766f6d322e4c69737455696e743136
		//                     (WireList.Name) "veyron.io/veyron/veyron2/vom2.ListUint16"
		//   02 06             (WireList.Elem) uint16[06]
		//   00              (WireList end)
		//
		// ff82 03 02 0102   type 65[ff82] msglen[03] value 1,2[02 0102]
		"ListUint16",
		v{ListUint16{1, 2}, ListUint16{1, 2}},
		"80" +
			"ff811d130117766579726f6e322f766f6d322e4c69737455696e743136020600" +
			"ff8203020102ff8203020102",
		j{}, // TODO
	},

	// Sets
	{
		// ff81 04           typedef 65[ff81] msglen[04]
		// 14                (WireSet)
		//   02 06             (WireSet.Key) uint16[06]
		//   00              (WireSet end)
		//
		// ff82 03 02 0102   type 65[ff82] msglen[03] value 1,2[02 0102]
		"UnnamedSet",
		v{map[uint16]struct{}{1: struct{}{}, 2: struct{}{}}, map[uint16]struct{}{1: struct{}{}, 2: struct{}{}}},
		"80" +
			"ff810414020600" +
			"ff820302[01,02]ff820302[01,02]",
		j{}, // TODO
	},
	{
		// ff81 1c           typedef 65[ff81] msglen[1c]
		// 14                (WireSet)
		//   01 16 766579726f6e322f766f6d322e53657455696e743136
		//                     (WireSet.Name) "veyron.io/veyron/veyron2/vom2.SetUint16"
		//   02 06             (WireSet.Key)  uint16[06]
		//   00              (WireSet end)
		//
		// ff82 03 02 0102   type 65[ff82] msglen[03] value 1,2[02 0102]
		"SetUint16",
		v{SetUint16{1: struct{}{}, 2: struct{}{}}, SetUint16{1: struct{}{}, 2: struct{}{}}},
		"80" +
			"ff811c140116766579726f6e322f766f6d322e53657455696e743136020600" +
			"ff820302[01,02]ff820302[01,02]",
		j{}, // TODO
	},

	// Maps
	{
		// ff81 06           typedef 65[ff81] msglen[06]
		// 15                (WireMap)
		//   02 06             (WireMap.Key)  uint16[06]
		//   03 04             (WireMap.Elem) string[04]
		//   00              (WireMap end)
		//
		// ff82 0b 02        type 65[ff82] msglen[0b] len[02]
		//   01 03 616263      1: "abc"
		//   02 03 646566      2: "def"
		"UnnamedMap",
		v{map[uint16]string{1: "abc", 2: "def"}, map[uint16]string{1: "abc", 2: "def"}},
		"80" +
			"ff8106150206030400" +
			"ff820b02[0103616263,0203646566]" +
			"ff820b02[0103616263,0203646566]",
		j{}, // TODO
	},
	{
		// ff81 24           typedef 65[ff81] msglen[24]
		// 15                (WireMap)
		//   01 1c 766579726f6e322f766f6d322e4d617055696e743136537472696e67
		//                     (WireMap.Name) "veyron.io/veyron/veyron2/vom2.MapUint16String"
		//   02 06             (WireMap.Key)  uint16[06]
		//   03 04             (WireMap.Elem) string[04]
		//   00              (WireMap end)
		//
		// ff82 0b 02        type 65[ff82] msglen[0b] len[02]
		//   01 03 616263      1: "abc"
		//   02 03 646566      2: "def"
		"MapUint16String",
		v{MapUint16String{1: "abc", 2: "def"}, MapUint16String{1: "abc", 2: "def"}},
		"80" +
			"ff812415011c766579726f6e322f766f6d322e4d617055696e743136537472696e670206030400" +
			"ff820b02[0103616263,0203646566]" +
			"ff820b02[0103616263,0203646566]",
		j{}, // TODO
	},

	// TODO(toddw): Add tests of struct, any, oneof, etc.
}

type (
	Bool       bool
	String     string
	ByteSlice  []byte
	Byte       byte
	Uint16     uint16
	Uint32     uint32
	Uint64     uint64
	Int16      int16
	Int32      int32
	Int64      int64
	Float32    float32
	Float64    float64
	Complex64  complex64
	Complex128 complex128

	Array2Uint16    [2]uint16
	ListUint16      []uint16
	SetUint16       map[uint16]struct{}
	MapUint16String map[uint16]string
)
