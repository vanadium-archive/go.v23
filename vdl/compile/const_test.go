package compile_test

import (
	"testing"

	"veyron2/val"
	"veyron2/vdl/build"
	"veyron2/vdl/compile"
	"veyron2/vdl/vdltest"
)

func TestConst(t *testing.T) {
	for _, test := range constTests {
		env := compile.NewEnv(-1)
		for _, tpkg := range test.Pkgs {
			// Compile the package with a single file, and adding the "package foo"
			// prefix to the source data automatically.
			files := map[string]string{
				tpkg.Name + ".vdl": "package " + tpkg.Name + "\n" + tpkg.Data,
			}
			buildPkg := vdltest.FakeBuildPackage(tpkg.Name, tpkg.Name, files)
			pkg := build.CompilePackage(buildPkg, env)
			vdltest.ExpectResult(t, env.Errors, test.Name, tpkg.ErrRE)
			if pkg == nil || tpkg.ErrRE != "" {
				continue
			}
			matchConstRes(t, test.Name, tpkg, pkg.Files[0].ConstDefs)
		}
	}
}

func matchConstRes(t *testing.T, tname string, cpkg constPkg, cdefs []*compile.ConstDef) {
	if cpkg.ExpectRes == nil {
		return
	}
	// Look for a ConstDef called "Res" to compare our expected results.
	for _, cdef := range cdefs {
		if cdef.Name == "Res" {
			if got, want := cdef.Value, cpkg.ExpectRes; !val.Equal(got, want) {
				t.Errorf("%s value got %s(%s), want %s(%s)", tname, got.Type(), got, want.Type(), want)
			}
			return
		}
	}
	t.Errorf("%s couldn't find Res in package %s", tname, cpkg.Name)
}

func namedZero(name string, base *val.Type) *val.Value {
	return val.Zero(val.NamedType(name, base))
}

func makeIntList(vals ...int64) *val.Value {
	listv := val.Zero(val.ListType(val.Int64Type)).AssignLen(len(vals))
	for index, v := range vals {
		listv.Index(index).AssignInt(v)
	}
	return listv
}

func makeStringIntMap(m map[string]int64) *val.Value {
	mapv := val.Zero(val.MapType(val.StringType, val.Int64Type))
	for k, v := range m {
		mapv.AssignMapIndex(val.StringValue(k), val.Int64Value(v))
	}
	return mapv
}

func makeStruct(name string, x int64, y string, z bool) *val.Value {
	t := val.StructType(name, []val.StructField{
		{"X", val.Int64Type}, {"Y", val.StringType}, {"Z", val.BoolType},
	})
	structv := val.Zero(t)
	structv.Field(0).AssignInt(x)
	structv.Field(1).AssignString(y)
	structv.Field(2).AssignBool(z)
	return structv
}

func makeABStruct() *val.Value {
	tA := val.StructType("a.A", []val.StructField{
		{"X", val.Int64Type}, {"Y", val.StringType},
	})
	tB := val.StructType("a.B", []val.StructField{{"Z", val.ListType(tA)}})
	res := val.Zero(tB)
	listv := res.Field(0).AssignLen(2)
	listv.Index(0).Field(0).AssignInt(1)
	listv.Index(0).Field(1).AssignString("a")
	listv.Index(1).Field(0).AssignInt(2)
	listv.Index(1).Field(1).AssignString("b")
	return res
}

type constPkg struct {
	Name      string
	Data      string
	ExpectRes *val.Value
	ErrRE     string
}

type cp []constPkg

var constTests = []struct {
	Name string
	Pkgs cp
}{
	// Test literals.
	{
		"UntypedBool",
		cp{{"a", `const Res = true`, val.BoolValue(true), ""}}},
	{
		"UntypedString",
		cp{{"a", `const Res = "abc"`, val.StringValue("abc"), ""}}},
	{
		"UntypedInteger",
		cp{{"a", `const Res = 123`, nil,
			`final const invalid \(123 must be assigned a type\)`}}},
	{
		"UntypedFloat",
		cp{{"a", `const Res = 1.5`, nil,
			`final const invalid \(1\.5 must be assigned a type\)`}}},
	{
		"UntypedComplex",
		cp{{"a", `const Res = 3.4+9.8i`, nil,
			`final const invalid \(3\.4\+9\.8i must be assigned a type\)`}}},

	// Test composite literals.
	{
		"IntList",
		cp{{"a", `const Res = []int64{0,1,2}`, makeIntList(0, 1, 2), ""}}},
	{
		"IntListKeys",
		cp{{"a", `const Res = []int64{1:1, 2:2, 0:0}`, makeIntList(0, 1, 2), ""}}},
	{
		"IntListMixedKey",
		cp{{"a", `const Res = []int64{1:1, 2, 0:0}`, makeIntList(0, 1, 2), ""}}},
	{
		"IntListDupKey",
		cp{{"a", `const Res = []int64{2:2, 1:1, 0}`, nil, "duplicate index 2 in list literal"}}},
	{
		"StringIntMap",
		cp{{"a", `const Res = map[string]int64{"a":1, "b":2, "c":3}`, makeStringIntMap(map[string]int64{"a": 1, "b": 2, "c": 3}), ""}}},
	{
		"StringIntMapNoKey",
		cp{{"a", `const Res = map[string]int64{"a":1, "b":2, 3}`, nil, "missing key"}}},
	{
		"StringIntMapDupKey",
		cp{{"a", `const Res = map[string]int64{"a":1, "b":2, "a":3}`, nil, "duplicate key"}}},
	{
		"StructNoKeys",
		cp{{"a", `type A struct{X int64;Y string;Z bool}; const Res = A{1,"b",true}`, makeStruct("a.A", 1, "b", true), ""}}},
	{
		"StructKeys",
		cp{{"a", `type A struct{X int64;Y string;Z bool}; const Res = A{X:1,Y:"b",Z:true}`, makeStruct("a.A", 1, "b", true), ""}}},
	{
		"StructKeysShort",
		cp{{"a", `type A struct{X int64;Y string;Z bool}; const Res = A{Y:"b"}`, makeStruct("a.A", 0, "b", false), ""}}},
	{
		"StructMixedKeys",
		cp{{"a", `type A struct{X int64;Y string;Z bool}; const Res = A{X:1,"b",Z:true}`, nil, "mixed key:value and value in a.A struct literal"}}},
	{
		"StructInvalidFieldName",
		cp{{"a", `type A struct{X int64;Y string;Z bool}; const Res = A{1+1:1}`, nil, `invalid field name`}}},
	{
		"StructUnknownFieldName",
		cp{{"a", `type A struct{X int64;Y string;Z bool}; const Res = A{ZZZ:1}`, nil, `unknown field "ZZZ" in a.A struct literal`}}},
	{
		"StructDupFieldName",
		cp{{"a", `type A struct{X int64;Y string;Z bool}; const Res = A{X:1,X:2}`, nil, `duplicate field "X" in a.A struct literal`}}},
	{
		"StructTooManyFields",
		cp{{"a", `type A struct{X int64;Y string;Z bool}; const Res = A{1,"b",true,4}`, nil, `too many fields in a.A struct literal`}}},
	{
		"StructTooFewFields",
		cp{{"a", `type A struct{X int64;Y string;Z bool}; const Res = A{1,"b"}`, nil, `too few fields in a.A struct literal`}}},
	{
		"ImplicitSubTypes",
		cp{{"a", `type A struct{X int64;Y string}; type B struct{Z []A}; const Res = B{{{1, "a"}, A{X:2,Y:"b"}}}`, makeABStruct(), ""}}},

	// Test explicit primitive type conversions.
	{
		"TypedBool",
		cp{{"a", `const Res = bool(false)`, val.BoolValue(false), ""}}},
	{
		"TypedString",
		cp{{"a", `const Res = string("abc")`, val.StringValue("abc"), ""}}},
	{
		"TypedInt32",
		cp{{"a", `const Res = int32(123)`, val.Int32Value(123), ""}}},
	{
		"TypedFloat32",
		cp{{"a", `const Res = float32(1.5)`, val.Float32Value(1.5), ""}}},
	{
		"TypedComplex64",
		cp{{"a", `const Res = complex64(2+1.5i)`, val.Complex64Value(2 + 1.5i), ""}}},
	{
		"TypedBoolMismatch",
		cp{{"a", `const Res = bool(1)`, nil,
			"can't convert 1 to bool"}}},
	{
		"TypedStringMismatch",
		cp{{"a", `const Res = string(1)`, nil,
			"can't convert 1 to string"}}},
	{
		"TypedInt32Mismatch",
		cp{{"a", `const Res = int32(true)`, nil,
			"can't convert true to int32"}}},
	{
		"TypedFloat32Mismatch",
		cp{{"a", `const Res = float32(true)`, nil,
			"can't convert true to float32"}}},

	// Test explicit user type conversions.
	{
		"TypedUserBool",
		cp{{"a", `type Bool bool;const Res = Bool(true)`, namedZero("a.Bool", val.BoolType).AssignBool(true), ""}}},
	{
		"TypedUserString",
		cp{{"a", `type Str string;const Res = Str("abc")`, namedZero("a.Str", val.StringType).AssignString("abc"), ""}}},
	{
		"TypedUserInt32",
		cp{{"a", `type Int int32;const Res = Int(123)`, namedZero("a.Int", val.Int32Type).AssignInt(123), ""}}},
	{
		"TypedUserFloat32",
		cp{{"a", `type Flt float32;const Res = Flt(1.5)`, namedZero("a.Flt", val.Float32Type).AssignFloat(1.5), ""}}},
	{
		"TypedUserComplex64",
		cp{{"a", `type Cpx complex64;const Res = Cpx(1.5+2i)`, namedZero("a.Cpx", val.Complex64Type).AssignComplex(1.5 + 2i), ""}}},
	{
		"TypedUserBoolMismatch",
		cp{{"a", `type Bool bool;const Res = Bool(1)`, nil,
			`invalid type conversion \(can't convert 1 to a.Bool bool\)`}}},
	{
		"TypedUserStringMismatch",
		cp{{"a", `type Str string;const Res = Str(1)`, nil,
			`invalid type conversion \(can't convert 1 to a.Str string\)`}}},
	{
		"TypedUserInt32Mismatch",
		cp{{"a", `type Int int32;const Res = Int(true)`, nil,
			`invalid type conversion \(can't convert true to a.Int int32\)`}}},
	{
		"TypedUserFloat32Mismatch",
		cp{{"a", `type Flt float32;const Res = Flt(true)`, nil,
			`invalid type conversion \(can't convert true to a.Flt float32\)`}}},

	// Test named consts.
	{
		"NamedBool",
		cp{{"a", `const Foo = true;const Res = Foo`, val.BoolValue(true), ""}}},
	{
		"NamedString",
		cp{{"a", `const Foo = "abc";const Res = Foo`, val.StringValue("abc"), ""}}},
	{
		"NamedInt32",
		cp{{"a", `const Foo = int32(123);const Res = Foo`, val.Int32Value(123), ""}}},
	{
		"NamedFloat32",
		cp{{"a", `const Foo = float32(1.5);const Res = Foo`, val.Float32Value(1.5), ""}}},
	{
		"NamedComplex64",
		cp{{"a", `const Foo = complex64(3+2i);const Res = Foo`, val.Complex64Value(3 + 2i), ""}}},
	{
		"NamedUserBool",
		cp{{"a", `type Bool bool;const Foo = Bool(true);const Res = Foo`,
			namedZero("a.Bool", val.BoolType).AssignBool(true), ""}}},
	{
		"NamedUserString",
		cp{{"a", `type Str string;const Foo = Str("abc");const Res = Foo`,
			namedZero("a.Str", val.StringType).AssignString("abc"), ""}}},
	{
		"NamedUserInt32",
		cp{{"a", `type Int int32;const Foo = Int(123);const Res = Foo`,
			namedZero("a.Int", val.Int32Type).AssignInt(123), ""}}},
	{
		"NamedUserFloat32",
		cp{{"a", `type Flt float32;const Foo = Flt(1.5);const Res = Foo`,
			namedZero("a.Flt", val.Float32Type).AssignFloat(1.5), ""}}},
	{
		"ConstNamedI",
		cp{{"a", `const I = true;const Res = I`, val.BoolValue(true), ""}}},

	// Test unary ops.
	{
		"Not",
		cp{{"a", `const Res = !true`, val.BoolValue(false), ""}}},
	{
		"Pos",
		cp{{"a", `const Res = int32(+123)`, val.Int32Value(123), ""}}},
	{
		"Neg",
		cp{{"a", `const Res = int32(-123)`, val.Int32Value(-123), ""}}},
	{
		"Complement",
		cp{{"a", `const Res = int32(^1)`, val.Int32Value(-2), ""}}},
	{
		"TypedNot",
		cp{{"a", `type Bool bool;const Res = !Bool(true)`, namedZero("a.Bool", val.BoolType), ""}}},
	{
		"TypedPos",
		cp{{"a", `type Int int32;const Res = Int(+123)`, namedZero("a.Int", val.Int32Type).AssignInt(123), ""}}},
	{
		"TypedNeg",
		cp{{"a", `type Int int32;const Res = Int(-123)`, namedZero("a.Int", val.Int32Type).AssignInt(-123), ""}}},
	{
		"TypedComplement",
		cp{{"a", `type Int int32;const Res = Int(^1)`, namedZero("a.Int", val.Int32Type).AssignInt(-2), ""}}},
	{
		"NamedNot",
		cp{{"a", `const Foo = bool(true);const Res = !Foo`, val.BoolValue(false), ""}}},
	{
		"NamedPos",
		cp{{"a", `const Foo = int32(123);const Res = +Foo`, val.Int32Value(123), ""}}},
	{
		"NamedNeg",
		cp{{"a", `const Foo = int32(123);const Res = -Foo`, val.Int32Value(-123), ""}}},
	{
		"NamedComplement",
		cp{{"a", `const Foo = int32(1);const Res = ^Foo`, val.Int32Value(-2), ""}}},
	{
		"ErrNot",
		cp{{"a", `const Res = !1`, nil, `unary \! invalid \(untyped integer not supported\)`}}},
	{
		"ErrPos",
		cp{{"a", `const Res = +"abc"`, nil, `unary \+ invalid \(untyped string not supported\)`}}},
	{
		"ErrNeg",
		cp{{"a", `const Res = -false`, nil, `unary \- invalid \(untyped boolean not supported\)`}}},
	{
		"ErrComplement",
		cp{{"a", `const Res = ^1.5`, nil, `unary \^ invalid \(converting untyped rational 1.5 to integer loses precision\)`}}},

	// Test logical and comparison ops.
	{
		"Or",
		cp{{"a", `const Res = true || false`, val.BoolValue(true), ""}}},
	{
		"And",
		cp{{"a", `const Res = true && false`, val.BoolValue(false), ""}}},
	{
		"Lt11",
		cp{{"a", `const Res = 1 < 1`, val.BoolValue(false), ""}}},
	{
		"Lt12",
		cp{{"a", `const Res = 1 < 2`, val.BoolValue(true), ""}}},
	{
		"Lt21",
		cp{{"a", `const Res = 2 < 1`, val.BoolValue(false), ""}}},
	{
		"Gt11",
		cp{{"a", `const Res = 1 > 1`, val.BoolValue(false), ""}}},
	{
		"Gt12",
		cp{{"a", `const Res = 1 > 2`, val.BoolValue(false), ""}}},
	{
		"Gt21",
		cp{{"a", `const Res = 2 > 1`, val.BoolValue(true), ""}}},
	{
		"Le11",
		cp{{"a", `const Res = 1 <= 1`, val.BoolValue(true), ""}}},
	{
		"Le12",
		cp{{"a", `const Res = 1 <= 2`, val.BoolValue(true), ""}}},
	{
		"Le21",
		cp{{"a", `const Res = 2 <= 1`, val.BoolValue(false), ""}}},
	{
		"Ge11",
		cp{{"a", `const Res = 1 >= 1`, val.BoolValue(true), ""}}},
	{
		"Ge12",
		cp{{"a", `const Res = 1 >= 2`, val.BoolValue(false), ""}}},
	{
		"Ge21",
		cp{{"a", `const Res = 2 >= 1`, val.BoolValue(true), ""}}},
	{
		"Ne11",
		cp{{"a", `const Res = 1 != 1`, val.BoolValue(false), ""}}},
	{
		"Ne12",
		cp{{"a", `const Res = 1 != 2`, val.BoolValue(true), ""}}},
	{
		"Ne21",
		cp{{"a", `const Res = 2 != 1`, val.BoolValue(true), ""}}},
	{
		"Eq11",
		cp{{"a", `const Res = 1 == 1`, val.BoolValue(true), ""}}},
	{
		"Eq12",
		cp{{"a", `const Res = 1 == 2`, val.BoolValue(false), ""}}},
	{
		"Eq21",
		cp{{"a", `const Res = 2 == 1`, val.BoolValue(false), ""}}},

	// Test arithmetic ops.
	{
		"IntPlus",
		cp{{"a", `const Res = int32(1) + 1`, val.Int32Value(2), ""}}},
	{
		"IntMinus",
		cp{{"a", `const Res = int32(2) - 1`, val.Int32Value(1), ""}}},
	{
		"IntTimes",
		cp{{"a", `const Res = int32(3) * 2`, val.Int32Value(6), ""}}},
	{
		"IntDivide",
		cp{{"a", `const Res = int32(5) / 2`, val.Int32Value(2), ""}}},
	{
		"FloatPlus",
		cp{{"a", `const Res = float32(1) + 1`, val.Float32Value(2), ""}}},
	{
		"FloatMinus",
		cp{{"a", `const Res = float32(2) - 1`, val.Float32Value(1), ""}}},
	{
		"FloatTimes",
		cp{{"a", `const Res = float32(3) * 2`, val.Float32Value(6), ""}}},
	{
		"FloatDivide",
		cp{{"a", `const Res = float32(5) / 2`, val.Float32Value(2.5), ""}}},
	{
		"ComplexPlus",
		cp{{"a", `const Res = 3i + complex64(1+2i) + 1`, val.Complex64Value(2 + 5i), ""}}},
	{
		"ComplexMinus",
		cp{{"a", `const Res = complex64(1+2i) -4 -1i`, val.Complex64Value(-3 + 1i), ""}}},
	{
		"ComplexTimes",
		cp{{"a", `const Res = complex64(1+3i) * (5+1i)`, val.Complex64Value(2 + 16i), ""}}},
	{
		"ComplexDivide",
		cp{{"a", `const Res = complex64(2+16i) / (5+1i)`, val.Complex64Value(1 + 3i), ""}}},

	// Test integer arithmetic ops.
	{
		"Mod",
		cp{{"a", `const Res = int32(8) % 3`, val.Int32Value(2), ""}}},
	{
		"BitOr",
		cp{{"a", `const Res = int32(8) | 7`, val.Int32Value(15), ""}}},
	{
		"BitAnd",
		cp{{"a", `const Res = int32(8) & 15`, val.Int32Value(8), ""}}},
	{
		"BitXor",
		cp{{"a", `const Res = int32(8) ^ 5`, val.Int32Value(13), ""}}},
	{
		"UntypedFloatMod",
		cp{{"a", `const Res = int32(8.0 % 3.0)`, val.Int32Value(2), ""}}},
	{
		"UntypedFloatBitOr",
		cp{{"a", `const Res = int32(8.0 | 7.0)`, val.Int32Value(15), ""}}},
	{
		"UntypedFloatBitAnd",
		cp{{"a", `const Res = int32(8.0 & 15.0)`, val.Int32Value(8), ""}}},
	{
		"UntypedFloatBitXor",
		cp{{"a", `const Res = int32(8.0 ^ 5.0)`, val.Int32Value(13), ""}}},
	{
		"TypedFloatMod",
		cp{{"a", `const Res = int32(float32(8.0) % 3.0)`, nil,
			`binary % invalid \(can't convert typed float32 to integer\)`}}},
	{
		"TypedFloatBitOr",
		cp{{"a", `const Res = int32(float32(8.0) | 7.0)`, nil,
			`binary | invalid \(can't convert typed float32 to integer\)`}}},
	{
		"TypedFloatBitAnd",
		cp{{"a", `const Res = int32(float32(8.0) & 15.0)`, nil,
			`binary & invalid \(can't convert typed float32 to integer\)`}}},
	{
		"TypedFloatBitXor",
		cp{{"a", `const Res = int32(float32(8.0) ^ 5.0)`, nil,
			`binary \^ invalid \(can't convert typed float32 to integer\)`}}},

	// Test shift ops.
	{
		"Lsh",
		cp{{"a", `const Res = int32(8) << 2`, val.Int32Value(32), ""}}},
	{
		"Rsh",
		cp{{"a", `const Res = int32(8) >> 2`, val.Int32Value(2), ""}}},
	{
		"UntypedFloatLsh",
		cp{{"a", `const Res = int32(8.0 << 2.0)`, val.Int32Value(32), ""}}},
	{
		"UntypedFloatRsh",
		cp{{"a", `const Res = int32(8.0 >> 2.0)`, val.Int32Value(2), ""}}},

	// Test mixed ops.
	{
		"Mixed",
		cp{{"a", `const F = "f";const Res = "f" == F && (1+2) == 3`, val.BoolValue(true), ""}}},
	{
		"MixedPrecedence",
		cp{{"a", `const Res = int32(1+2*3-4)`, val.Int32Value(3), ""}}},

	// Test uint conversion.
	{
		"MaxUint32",
		cp{{"a", `const Res = uint32(4294967295)`, val.Uint32Value(4294967295), ""}}},
	{
		"MaxUint64",
		cp{{"a", `const Res = uint64(18446744073709551615)`,
			val.Uint64Value(18446744073709551615), ""}}},
	{
		"OverflowUint32",
		cp{{"a", `const Res = uint32(4294967296)`, nil,
			"const 4294967296 overflows uint32"}}},
	{
		"OverflowUint64",
		cp{{"a", `const Res = uint64(18446744073709551616)`, nil,
			"const 18446744073709551616 overflows uint64"}}},
	{
		"NegUint32",
		cp{{"a", `const Res = uint32(-3)`, nil,
			"const -3 overflows uint32"}}},
	{
		"NegUint64",
		cp{{"a", `const Res = uint64(-4)`, nil,
			"const -4 overflows uint64"}}},
	{
		"ZeroUint32",
		cp{{"a", `const Res = uint32(0)`, val.Uint32Value(0), ""}}},

	// Test int conversion.
	{
		"MinInt32",
		cp{{"a", `const Res = int32(-2147483648)`, val.Int32Value(-2147483648), ""}}},
	{
		"MinInt64",
		cp{{"a", `const Res = int64(-9223372036854775808)`,
			val.Int64Value(-9223372036854775808), ""}}},
	{
		"MinOverflowInt32",
		cp{{"a", `const Res = int32(-2147483649)`, nil,
			"const -2147483649 overflows int32"}}},
	{
		"MinOverflowInt64",
		cp{{"a", `const Res = int64(-9223372036854775809)`, nil,
			"const -9223372036854775809 overflows int64"}}},
	{
		"MaxInt32",
		cp{{"a", `const Res = int32(2147483647)`,
			val.Int32Value(2147483647), ""}}},
	{
		"MaxInt64",
		cp{{"a", `const Res = int64(9223372036854775807)`,
			val.Int64Value(9223372036854775807), ""}}},
	{
		"MaxOverflowInt32",
		cp{{"a", `const Res = int32(2147483648)`, nil,
			"const 2147483648 overflows int32"}}},
	{
		"MaxOverflowInt64",
		cp{{"a", `const Res = int64(9223372036854775808)`, nil,
			"const 9223372036854775808 overflows int64"}}},
	{
		"ZeroInt32",
		cp{{"a", `const Res = int32(0)`, val.Int32Value(0), ""}}},

	// Test float conversion.
	{
		"SmallestFloat32",
		cp{{"a", `const Res = float32(1.401298464324817070923729583289916131281e-45)`,
			val.Float32Value(1.401298464324817070923729583289916131281e-45), ""}}},
	{
		"SmallestFloat64",
		cp{{"a", `const Res = float64(4.940656458412465441765687928682213723651e-324)`,
			val.Float64Value(4.940656458412465441765687928682213723651e-324), ""}}},
	{
		"MaxFloat32",
		cp{{"a", `const Res = float32(3.40282346638528859811704183484516925440e+38)`,
			val.Float32Value(3.40282346638528859811704183484516925440e+38), ""}}},
	{
		"MaxFloat64",
		cp{{"a", `const Res = float64(1.797693134862315708145274237317043567980e+308)`,
			val.Float64Value(1.797693134862315708145274237317043567980e+308), ""}}},
	{
		"UnderflowFloat32",
		cp{{"a", `const Res = float32(1.401298464324817070923729583289916131280e-45)`,
			nil, "underflows float32"}}},
	{
		"UnderflowFloat64",
		cp{{"a", `const Res = float64(4.940656458412465441765687928682213723650e-324)`,
			nil, "underflows float64"}}},
	{
		"OverflowFloat32",
		cp{{"a", `const Res = float32(3.40282346638528859811704183484516925441e+38)`,
			nil, "overflows float32"}}},
	{
		"OverflowFloat64",
		cp{{"a", `const Res = float64(1.797693134862315708145274237317043567981e+308)`,
			nil, "overflows float64"}}},
	{
		"ZeroFloat32",
		cp{{"a", `const Res = float32(0)`, val.Float32Value(0), ""}}},

	// Test complex conversion.
	{
		"RealComplexToFloat",
		cp{{"a", `const Res = float64(1+0i)`, val.Float64Value(1), ""}}},
	{
		"RealComplexToInt",
		cp{{"a", `const Res = int32(1+0i)`, val.Int32Value(1), ""}}},
	{
		"FloatToRealComplex",
		cp{{"a", `const Res = complex64(1.5)`, val.Complex64Value(1.5), ""}}},
	{
		"IntToRealComplex",
		cp{{"a", `const Res = complex64(2)`, val.Complex64Value(2), ""}}},

	// Test float rounding - note that 1.1 incurs loss of precision.
	{
		"RoundedCompareFloat32",
		cp{{"a", `const Res = float32(1.1) == 1.1`, val.BoolValue(true), ""}}},
	{
		"RoundedCompareFloat64",
		cp{{"a", `const Res = float64(1.1) == 1.1`, val.BoolValue(true), ""}}},
	{
		"RoundedTruncation",
		cp{{"a", `const Res = float64(float32(1.1)) != 1.1`, val.BoolValue(true), ""}}},

	// Test multi-package consts
	{"MultiPkgSameConstName", cp{
		{"a", `const Res = true`, val.BoolValue(true), ""},
		{"b", `const Res = true`, val.BoolValue(true), ""}}},
	{"MultiPkgDep", cp{
		{"a", `const Res = true`, val.BoolValue(true), ""},
		{"b", `import "a";const Res = a.Res && false`, val.BoolValue(false), ""}}},
	{"MultiPkgSamePkgName", cp{
		{"a", `const Res = true`, val.BoolValue(true), ""},
		{"a", `const Res = true`, nil, "invalid recompile"}}},
	{"MultiPkgUnimportedPkg", cp{
		{"a", `const Res = true`, val.BoolValue(true), ""},
		{"b", `const Res = a.Res && false`, nil, "const a.Res undefined"}}},
}
