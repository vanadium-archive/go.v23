package compile_test

import (
	"strings"
	"testing"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vdl/build"
	"veyron.io/veyron/veyron2/vdl/compile"
	"veyron.io/veyron/veyron2/vdl/vdltest"
)

func testConstPackage(t *testing.T, name string, tpkg constPkg, env *compile.Env) *compile.Package {
	// Compile the package with a single file, and adding the "package foo"
	// prefix to the source data automatically.
	files := map[string]string{
		tpkg.Name + ".vdl": "package " + tpkg.Name + "\n" + tpkg.Data,
	}
	buildPkg := vdltest.FakeBuildPackage(tpkg.Name, tpkg.Name, files)
	pkg := build.CompilePackage(buildPkg, env)
	vdltest.ExpectResult(t, env.Errors, name, tpkg.ErrRE)
	if pkg == nil || tpkg.ErrRE != "" {
		return nil
	}
	matchConstRes(t, name, tpkg, pkg.Files[0].ConstDefs)
	return pkg
}

func matchConstRes(t *testing.T, tname string, tpkg constPkg, cdefs []*compile.ConstDef) {
	if tpkg.ExpectRes == nil {
		return
	}
	// Look for a ConstDef called "Res" to compare our expected results.
	for _, cdef := range cdefs {
		if cdef.Name == "Res" {
			if got, want := cdef.Value, tpkg.ExpectRes; !vdl.EqualValue(got, want) {
				t.Errorf("%s value got %s(%s), want %s(%s)", tname, got.Type(), got, want.Type(), want)
			}
			return
		}
	}
	t.Errorf("%s couldn't find Res in package %s", tname, tpkg.Name)
}

func testConfigFile(t *testing.T, name string, tpkg constPkg, env *compile.Env) {
	// Take advantage of the fact that vdl files and config files have very
	// similar syntax.  Just prefix the data with "config Res\n" rather than
	// "package a\n" and we have a valid config file.
	fname := tpkg.Name + ".config"
	data := "config = Res\n" + tpkg.Data
	config := build.CompileConfig(fname, strings.NewReader(data), nil, env)
	vdltest.ExpectResult(t, env.Errors, name, tpkg.ErrRE)
	if config == nil || tpkg.ErrRE != "" {
		return
	}
	if got, want := config, tpkg.ExpectRes; !vdl.EqualValue(got, want) {
		t.Errorf("%s value got %s(%s), want %s(%s)", name, got.Type(), got, want.Type(), want)
	}
}

func TestConst(t *testing.T) {
	for _, test := range constTests {
		env := compile.NewEnv(-1)
		env.EnableExperimental()
		for _, tpkg := range test.Pkgs {
			testConstPackage(t, test.Name, tpkg, env)
		}
	}
}

func TestConfig(t *testing.T) {
	for _, test := range constTests {
		env := compile.NewEnv(-1)
		env.EnableExperimental()
		// Compile all but the last tpkg as regular packages.
		for _, tpkg := range test.Pkgs[:len(test.Pkgs)-1] {
			testConstPackage(t, test.Name, tpkg, env)
		}
		// Compile the last tpkg as a regular package to see if it defines anything
		// other than consts.
		last := test.Pkgs[len(test.Pkgs)-1]
		pkg := testConstPackage(t, test.Name, last, env)
		if pkg == nil ||
			len(pkg.Files[0].ErrorIDs) > 0 ||
			len(pkg.Files[0].TypeDefs) > 0 ||
			len(pkg.Files[0].Interfaces) > 0 {
			continue // has non-const stuff, can't be a valid config file
		}
		// Finally compile the config file.
		testConfigFile(t, test.Name, last, env)
	}
}

func namedZero(name string, base *vdl.Type) *vdl.Value {
	return vdl.ZeroValue(vdl.NamedType(name, base))
}

func makeIntList(vals ...int64) *vdl.Value {
	listv := vdl.ZeroValue(vdl.ListType(vdl.Int64Type)).AssignLen(len(vals))
	for index, v := range vals {
		listv.Index(index).AssignInt(v)
	}
	return listv
}

func makeStringSet(keys ...string) *vdl.Value {
	setv := vdl.ZeroValue(vdl.SetType(vdl.StringType))
	for _, k := range keys {
		setv.AssignSetKey(vdl.StringValue(k))
	}
	return setv
}

func makeStringIntMap(m map[string]int64) *vdl.Value {
	mapv := vdl.ZeroValue(vdl.MapType(vdl.StringType, vdl.Int64Type))
	for k, v := range m {
		mapv.AssignMapIndex(vdl.StringValue(k), vdl.Int64Value(v))
	}
	return mapv
}

func makeStruct(name string, x int64, y string, z bool) *vdl.Value {
	t := vdl.NamedType(name, vdl.StructType([]vdl.StructField{
		{"X", vdl.Int64Type}, {"Y", vdl.StringType}, {"Z", vdl.BoolType},
	}...))
	structv := vdl.ZeroValue(t)
	structv.Field(0).AssignInt(x)
	structv.Field(1).AssignString(y)
	structv.Field(2).AssignBool(z)
	return structv
}

func makeABStruct() *vdl.Value {
	tA := vdl.NamedType("a.A", vdl.StructType([]vdl.StructField{
		{"X", vdl.Int64Type}, {"Y", vdl.StringType},
	}...))
	tB := vdl.NamedType("a.B", vdl.StructType(vdl.StructField{"Z", vdl.ListType(tA)}))
	res := vdl.ZeroValue(tB)
	listv := res.Field(0).AssignLen(2)
	listv.Index(0).Field(0).AssignInt(1)
	listv.Index(0).Field(1).AssignString("a")
	listv.Index(1).Field(0).AssignInt(2)
	listv.Index(1).Field(1).AssignString("b")
	return res
}

func makeEnumXYZ(name, label string) *vdl.Value {
	t := vdl.NamedType(name, vdl.EnumType("X", "Y", "Z"))
	return vdl.ZeroValue(t).AssignEnumLabel(label)
}

type constPkg struct {
	Name      string
	Data      string
	ExpectRes *vdl.Value
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
		cp{{"a", `const Res = true`, vdl.BoolValue(true), ""}}},
	{
		"UntypedString",
		cp{{"a", `const Res = "abc"`, vdl.StringValue("abc"), ""}}},
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

	// Test list literals.
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
		cp{{"a", `const Res = []int64{2:2, 1:1, 0}`, nil, "duplicate index 2"}}},
	{
		"IntListInvalidIndex",
		cp{{"a", `const Res = []int64{"a":2, 1:1, 2:2}`, nil, "invalid index"}}},
	{
		"IntListInvalidValue",
		cp{{"a", `const Res = []int64{0,1,"c"}`, nil, "invalid list value"}}},
	{
		"IndexingNamedList",
		cp{{"a", `const A = []int64{3,4,2}; const Res=A[1]`, vdl.Int64Value(4), ""}}},
	{
		"IndexingUnnamedList",
		cp{{"a", `const Res = []int64{3,4,2}[1]`, nil, "cannot apply index operator to unnamed constant"}}},
	{
		"TypedListIndexing",
		cp{{"a", `const A = []int64{3,4,2};  const Res = A[int16(1)]`, vdl.Int64Value(4), ""}}},
	{
		"NegativeListIndexing",
		cp{{"a", `const A = []int64{3,4,2}; const Res = A[-1]`, nil, "error converting index [(]const -1 overflows uint64[)]"}}},
	{
		"OutOfBoundsListIndexing",
		cp{{"a", `const A = []int64{3,4,2}; const Res = A[10]`, nil, "index out of bounds"}}},
	{
		"InvalidIndexType",
		cp{{"a", `const A = []int64{3,4,2}; const Res = A["ok"]`, nil, "error converting"}}},
	{
		"InvalidIndexBaseType",
		cp{{"a", `type A struct{}; const B = A{}; const Res = B["ok"]`, nil, "illegal use of index operator with unsupported type"}}},

	// Test set literals.
	{
		"StringSet",
		cp{{"a", `const Res = set[string]{"a","b","c"}`, makeStringSet("a", "b", "c"), ""}}},
	{
		"StringSetInvalidIndex",
		cp{{"a", `const Res = set[string]{"a","b","c":3}`, nil, "invalid index"}}},
	{
		"StringSetDupKey",
		cp{{"a", `const Res = set[string]{"a","b","b"}`, nil, "duplicate key"}}},
	{
		"StringSetInvalidKey",
		cp{{"a", `const Res = set[string]{"a","b",3}`, nil, "invalid set key"}}},

	// Test map literals.
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
		"StringIntMapInvalidKey",
		cp{{"a", `const Res = map[string]int64{"a":1, "b":2, 3:3}`, nil, "invalid map key"}}},
	{
		"StringIntMapInvalidValue",
		cp{{"a", `const Res = map[string]int64{"a":1, "b":2, "c":"c"}`, nil, "invalid map value"}}},
	{
		"MapIndexing",
		cp{{"a", `const A = map[int64]int64{1:4}; const Res=A[1]`, vdl.Int64Value(4), ""}}},
	{
		"MapUnnamedIndexing",
		cp{{"a", `const Res = map[int64]int64{1:4}[1]`, nil, "cannot apply index operator to unnamed constant"}}},
	{
		"MapTypedIndexing",
		cp{{"a", `const A = map[int64]int64{1:4}; const Res = A[int64(1)]`, vdl.Int64Value(4), ""}}},
	{
		"MapIncorrectlyTypedIndexing",
		cp{{"a", `const A = map[int64]int64{1:4};const Res = A[int16(1)]`, nil, "invalid map index [(]int64 not assignable from int16[(]1[)][)]"}}},
	{
		"MapIndexingMissingValue",
		cp{{"a", `const A = map[int64]int64{1:4}; const Res = A[0]`, nil, "map key not in map"}}},

	// Test struct literals.
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
		cp{{"a", `type A struct{X int64;Y string;Z bool}; const Res = A{X:1,"b",Z:true}`, nil, "mixed key:value and value"}}},
	{
		"StructInvalidFieldName",
		cp{{"a", `type A struct{X int64;Y string;Z bool}; const Res = A{1+1:1}`, nil, `invalid field name`}}},
	{
		"StructUnknownFieldName",
		cp{{"a", `type A struct{X int64;Y string;Z bool}; const Res = A{ZZZ:1}`, nil, `unknown field "ZZZ"`}}},
	{
		"StructDupFieldName",
		cp{{"a", `type A struct{X int64;Y string;Z bool}; const Res = A{X:1,X:2}`, nil, `duplicate field "X"`}}},
	{
		"StructTooManyFields",
		cp{{"a", `type A struct{X int64;Y string;Z bool}; const Res = A{1,"b",true,4}`, nil, `too many fields`}}},
	{
		"StructTooFewFields",
		cp{{"a", `type A struct{X int64;Y string;Z bool}; const Res = A{1,"b"}`, nil, `too few fields`}}},
	{
		"StructInvalidField",
		cp{{"a", `type A struct{X int64;Y string;Z bool}; const Res = A{Y:1}`, nil, "invalid struct field"}}},
	{
		"ImplicitSubTypes",
		cp{{"a", `type A struct{X int64;Y string}; type B struct{Z []A}; const Res = B{{{1, "a"}, A{X:2,Y:"b"}}}`, makeABStruct(), ""}}},
	{
		"StructSelector",
		cp{{"a", `type A struct{X int64;Y string}; const x = A{2,"b"}; const Res = x.Y`, vdl.StringValue("b"), ""}}},
	{
		"StructMultipleSelector",
		cp{{"a", `type A struct{X int64;Y B}; type B struct{Z bool}; const x = A{2,B{true}}; const Res = x.Y.Z`, vdl.BoolValue(true), ""}}},

	{
		"InvalidStructSelectorName",
		cp{{"a", `type A struct{X int64;Y string}; const x = A{2,"b"}; const Res = x.Z`, nil, "invalid field name"}}},
	{
		"StructSelectorOnNonStructType",
		cp{{"a", `type A []int32; const x = A{2}; const Res = x.Z`, nil, "invalid selector on const of kind: list"}}},
	{
		"SelectorOnUnnamedStruct",
		cp{{"a", `type A struct{X int64;Y string}; const Res = A{2,"b"}.Y`, nil, "cannot apply selector operator to unnamed constant"}}},

	// Test enums.
	{
		"Enum",
		cp{{"a", `type A enum{X;Y;Z}; const Res = A.X`, makeEnumXYZ("a.A", "X"), ""}}},
	{
		"EnumNoLabel",
		cp{{"a", `type A enum{X;Y;Z}; const Res = A`, nil, "A is a type"}}},

	// Test explicit primitive type conversions.
	{
		"TypedBool",
		cp{{"a", `const Res = bool(false)`, vdl.BoolValue(false), ""}}},
	{
		"TypedString",
		cp{{"a", `const Res = string("abc")`, vdl.StringValue("abc"), ""}}},
	{
		"TypedInt32",
		cp{{"a", `const Res = int32(123)`, vdl.Int32Value(123), ""}}},
	{
		"TypedFloat32",
		cp{{"a", `const Res = float32(1.5)`, vdl.Float32Value(1.5), ""}}},
	{
		"TypedComplex64",
		cp{{"a", `const Res = complex64(2+1.5i)`, vdl.Complex64Value(2 + 1.5i), ""}}},
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
			`types "int32" and "bool" aren't compatible`}}},
	{
		"TypedFloat32Mismatch",
		cp{{"a", `const Res = float32(true)`, nil,
			`types "float32" and "bool" aren't compatible`}}},

	// Test explicit user type conversions.
	{
		"TypedUserBool",
		cp{{"a", `type TypedBool bool;const Res = TypedBool(true)`, namedZero("a.TypedBool", vdl.BoolType).AssignBool(true), ""}}},
	{
		"TypedUserString",
		cp{{"a", `type TypedStr string;const Res = TypedStr("abc")`, namedZero("a.TypedStr", vdl.StringType).AssignString("abc"), ""}}},
	{
		"TypedUserInt32",
		cp{{"a", `type TypedInt int32;const Res = TypedInt(123)`, namedZero("a.TypedInt", vdl.Int32Type).AssignInt(123), ""}}},
	{
		"TypedUserFloat32",
		cp{{"a", `type TypedFlt float32;const Res = TypedFlt(1.5)`, namedZero("a.TypedFlt", vdl.Float32Type).AssignFloat(1.5), ""}}},
	{
		"TypedUserComplex64",
		cp{{"a", `type TypedCpx complex64;const Res = TypedCpx(1.5+2i)`, namedZero("a.TypedCpx", vdl.Complex64Type).AssignComplex(1.5 + 2i), ""}}},
	{
		"TypedUserBoolMismatch",
		cp{{"a", `type TypedBool bool;const Res = TypedBool(1)`, nil,
			`invalid type conversion \(can't convert 1 to a.TypedBool bool\)`}}},
	{
		"TypedUserStringMismatch",
		cp{{"a", `type TypedStr string;const Res = TypedStr(1)`, nil,
			`invalid type conversion \(can't convert 1 to a.TypedStr string\)`}}},
	{
		"TypedUserInt32Mismatch",
		cp{{"a", `type TypedInt int32;const Res = TypedInt(true)`, nil,
			`types "a.TypedInt int32" and "bool" aren't compatible`}}},
	{
		"TypedUserFloat32Mismatch",
		cp{{"a", `type TypedFlt float32;const Res = TypedFlt(true)`, nil,
			`types "a.TypedFlt float32" and "bool" aren't compatible`}}},

	// Test named consts.
	{
		"NamedBool",
		cp{{"a", `const foo = true;const Res = foo`, vdl.BoolValue(true), ""}}},
	{
		"NamedString",
		cp{{"a", `const foo = "abc";const Res = foo`, vdl.StringValue("abc"), ""}}},
	{
		"NamedInt32",
		cp{{"a", `const foo = int32(123);const Res = foo`, vdl.Int32Value(123), ""}}},
	{
		"NamedFloat32",
		cp{{"a", `const foo = float32(1.5);const Res = foo`, vdl.Float32Value(1.5), ""}}},
	{
		"NamedComplex64",
		cp{{"a", `const foo = complex64(3+2i);const Res = foo`, vdl.Complex64Value(3 + 2i), ""}}},
	{
		"NamedUserBool",
		cp{{"a", `type TypedBool bool;const foo = TypedBool(true);const Res = foo`,
			namedZero("a.TypedBool", vdl.BoolType).AssignBool(true), ""}}},
	{
		"NamedUserString",
		cp{{"a", `type TypedStr string;const foo = TypedStr("abc");const Res = foo`,
			namedZero("a.TypedStr", vdl.StringType).AssignString("abc"), ""}}},
	{
		"NamedUserInt32",
		cp{{"a", `type TypedInt int32;const foo = TypedInt(123);const Res = foo`,
			namedZero("a.TypedInt", vdl.Int32Type).AssignInt(123), ""}}},
	{
		"NamedUserFloat32",
		cp{{"a", `type TypedFlt float32;const foo = TypedFlt(1.5);const Res = foo`,
			namedZero("a.TypedFlt", vdl.Float32Type).AssignFloat(1.5), ""}}},
	{
		"ConstNamedI",
		cp{{"a", `const I = true;const Res = I`, vdl.BoolValue(true), ""}}},

	// Test unary ops.
	{
		"Not",
		cp{{"a", `const Res = !true`, vdl.BoolValue(false), ""}}},
	{
		"Pos",
		cp{{"a", `const Res = int32(+123)`, vdl.Int32Value(123), ""}}},
	{
		"Neg",
		cp{{"a", `const Res = int32(-123)`, vdl.Int32Value(-123), ""}}},
	{
		"Complement",
		cp{{"a", `const Res = int32(^1)`, vdl.Int32Value(-2), ""}}},
	{
		"TypedNot",
		cp{{"a", `type TypedBool bool;const Res = !TypedBool(true)`, namedZero("a.TypedBool", vdl.BoolType), ""}}},
	{
		"TypedPos",
		cp{{"a", `type TypedInt int32;const Res = TypedInt(+123)`, namedZero("a.TypedInt", vdl.Int32Type).AssignInt(123), ""}}},
	{
		"TypedNeg",
		cp{{"a", `type TypedInt int32;const Res = TypedInt(-123)`, namedZero("a.TypedInt", vdl.Int32Type).AssignInt(-123), ""}}},
	{
		"TypedComplement",
		cp{{"a", `type TypedInt int32;const Res = TypedInt(^1)`, namedZero("a.TypedInt", vdl.Int32Type).AssignInt(-2), ""}}},
	{
		"NamedNot",
		cp{{"a", `const foo = bool(true);const Res = !foo`, vdl.BoolValue(false), ""}}},
	{
		"NamedPos",
		cp{{"a", `const foo = int32(123);const Res = +foo`, vdl.Int32Value(123), ""}}},
	{
		"NamedNeg",
		cp{{"a", `const foo = int32(123);const Res = -foo`, vdl.Int32Value(-123), ""}}},
	{
		"NamedComplement",
		cp{{"a", `const foo = int32(1);const Res = ^foo`, vdl.Int32Value(-2), ""}}},
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
		cp{{"a", `const Res = true || false`, vdl.BoolValue(true), ""}}},
	{
		"And",
		cp{{"a", `const Res = true && false`, vdl.BoolValue(false), ""}}},
	{
		"Lt11",
		cp{{"a", `const Res = 1 < 1`, vdl.BoolValue(false), ""}}},
	{
		"Lt12",
		cp{{"a", `const Res = 1 < 2`, vdl.BoolValue(true), ""}}},
	{
		"Lt21",
		cp{{"a", `const Res = 2 < 1`, vdl.BoolValue(false), ""}}},
	{
		"Gt11",
		cp{{"a", `const Res = 1 > 1`, vdl.BoolValue(false), ""}}},
	{
		"Gt12",
		cp{{"a", `const Res = 1 > 2`, vdl.BoolValue(false), ""}}},
	{
		"Gt21",
		cp{{"a", `const Res = 2 > 1`, vdl.BoolValue(true), ""}}},
	{
		"Le11",
		cp{{"a", `const Res = 1 <= 1`, vdl.BoolValue(true), ""}}},
	{
		"Le12",
		cp{{"a", `const Res = 1 <= 2`, vdl.BoolValue(true), ""}}},
	{
		"Le21",
		cp{{"a", `const Res = 2 <= 1`, vdl.BoolValue(false), ""}}},
	{
		"Ge11",
		cp{{"a", `const Res = 1 >= 1`, vdl.BoolValue(true), ""}}},
	{
		"Ge12",
		cp{{"a", `const Res = 1 >= 2`, vdl.BoolValue(false), ""}}},
	{
		"Ge21",
		cp{{"a", `const Res = 2 >= 1`, vdl.BoolValue(true), ""}}},
	{
		"Ne11",
		cp{{"a", `const Res = 1 != 1`, vdl.BoolValue(false), ""}}},
	{
		"Ne12",
		cp{{"a", `const Res = 1 != 2`, vdl.BoolValue(true), ""}}},
	{
		"Ne21",
		cp{{"a", `const Res = 2 != 1`, vdl.BoolValue(true), ""}}},
	{
		"Eq11",
		cp{{"a", `const Res = 1 == 1`, vdl.BoolValue(true), ""}}},
	{
		"Eq12",
		cp{{"a", `const Res = 1 == 2`, vdl.BoolValue(false), ""}}},
	{
		"Eq21",
		cp{{"a", `const Res = 2 == 1`, vdl.BoolValue(false), ""}}},

	// Test arithmetic ops.
	{
		"IntPlus",
		cp{{"a", `const Res = int32(1) + 1`, vdl.Int32Value(2), ""}}},
	{
		"IntMinus",
		cp{{"a", `const Res = int32(2) - 1`, vdl.Int32Value(1), ""}}},
	{
		"IntTimes",
		cp{{"a", `const Res = int32(3) * 2`, vdl.Int32Value(6), ""}}},
	{
		"IntDivide",
		cp{{"a", `const Res = int32(5) / 2`, vdl.Int32Value(2), ""}}},
	{
		"FloatPlus",
		cp{{"a", `const Res = float32(1) + 1`, vdl.Float32Value(2), ""}}},
	{
		"FloatMinus",
		cp{{"a", `const Res = float32(2) - 1`, vdl.Float32Value(1), ""}}},
	{
		"FloatTimes",
		cp{{"a", `const Res = float32(3) * 2`, vdl.Float32Value(6), ""}}},
	{
		"FloatDivide",
		cp{{"a", `const Res = float32(5) / 2`, vdl.Float32Value(2.5), ""}}},
	{
		"ComplexPlus",
		cp{{"a", `const Res = 3i + complex64(1+2i) + 1`, vdl.Complex64Value(2 + 5i), ""}}},
	{
		"ComplexMinus",
		cp{{"a", `const Res = complex64(1+2i) -4 -1i`, vdl.Complex64Value(-3 + 1i), ""}}},
	{
		"ComplexTimes",
		cp{{"a", `const Res = complex64(1+3i) * (5+1i)`, vdl.Complex64Value(2 + 16i), ""}}},
	{
		"ComplexDivide",
		cp{{"a", `const Res = complex64(2+16i) / (5+1i)`, vdl.Complex64Value(1 + 3i), ""}}},

	// Test integer arithmetic ops.
	{
		"Mod",
		cp{{"a", `const Res = int32(8) % 3`, vdl.Int32Value(2), ""}}},
	{
		"BitOr",
		cp{{"a", `const Res = int32(8) | 7`, vdl.Int32Value(15), ""}}},
	{
		"BitAnd",
		cp{{"a", `const Res = int32(8) & 15`, vdl.Int32Value(8), ""}}},
	{
		"BitXor",
		cp{{"a", `const Res = int32(8) ^ 5`, vdl.Int32Value(13), ""}}},
	{
		"UntypedFloatMod",
		cp{{"a", `const Res = int32(8.0 % 3.0)`, vdl.Int32Value(2), ""}}},
	{
		"UntypedFloatBitOr",
		cp{{"a", `const Res = int32(8.0 | 7.0)`, vdl.Int32Value(15), ""}}},
	{
		"UntypedFloatBitAnd",
		cp{{"a", `const Res = int32(8.0 & 15.0)`, vdl.Int32Value(8), ""}}},
	{
		"UntypedFloatBitXor",
		cp{{"a", `const Res = int32(8.0 ^ 5.0)`, vdl.Int32Value(13), ""}}},
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
		cp{{"a", `const Res = int32(8) << 2`, vdl.Int32Value(32), ""}}},
	{
		"Rsh",
		cp{{"a", `const Res = int32(8) >> 2`, vdl.Int32Value(2), ""}}},
	{
		"UntypedFloatLsh",
		cp{{"a", `const Res = int32(8.0 << 2.0)`, vdl.Int32Value(32), ""}}},
	{
		"UntypedFloatRsh",
		cp{{"a", `const Res = int32(8.0 >> 2.0)`, vdl.Int32Value(2), ""}}},

	// Test mixed ops.
	{
		"Mixed",
		cp{{"a", `const F = "f";const Res = "f" == F && (1+2) == 3`, vdl.BoolValue(true), ""}}},
	{
		"MixedPrecedence",
		cp{{"a", `const Res = int32(1+2*3-4)`, vdl.Int32Value(3), ""}}},

	// Test uint conversion.
	{
		"MaxUint32",
		cp{{"a", `const Res = uint32(4294967295)`, vdl.Uint32Value(4294967295), ""}}},
	{
		"MaxUint64",
		cp{{"a", `const Res = uint64(18446744073709551615)`,
			vdl.Uint64Value(18446744073709551615), ""}}},
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
		cp{{"a", `const Res = uint32(0)`, vdl.Uint32Value(0), ""}}},

	// Test int conversion.
	{
		"MinInt32",
		cp{{"a", `const Res = int32(-2147483648)`, vdl.Int32Value(-2147483648), ""}}},
	{
		"MinInt64",
		cp{{"a", `const Res = int64(-9223372036854775808)`,
			vdl.Int64Value(-9223372036854775808), ""}}},
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
			vdl.Int32Value(2147483647), ""}}},
	{
		"MaxInt64",
		cp{{"a", `const Res = int64(9223372036854775807)`,
			vdl.Int64Value(9223372036854775807), ""}}},
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
		cp{{"a", `const Res = int32(0)`, vdl.Int32Value(0), ""}}},

	// Test float conversion.
	{
		"SmallestFloat32",
		cp{{"a", `const Res = float32(1.401298464324817070923729583289916131281e-45)`,
			vdl.Float32Value(1.401298464324817070923729583289916131281e-45), ""}}},
	{
		"SmallestFloat64",
		cp{{"a", `const Res = float64(4.940656458412465441765687928682213723651e-324)`,
			vdl.Float64Value(4.940656458412465441765687928682213723651e-324), ""}}},
	{
		"MaxFloat32",
		cp{{"a", `const Res = float32(3.40282346638528859811704183484516925440e+38)`,
			vdl.Float32Value(3.40282346638528859811704183484516925440e+38), ""}}},
	{
		"MaxFloat64",
		cp{{"a", `const Res = float64(1.797693134862315708145274237317043567980e+308)`,
			vdl.Float64Value(1.797693134862315708145274237317043567980e+308), ""}}},
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
		cp{{"a", `const Res = float32(0)`, vdl.Float32Value(0), ""}}},

	// Test complex conversion.
	{
		"RealComplexToFloat",
		cp{{"a", `const Res = float64(1+0i)`, vdl.Float64Value(1), ""}}},
	{
		"RealComplexToInt",
		cp{{"a", `const Res = int32(1+0i)`, vdl.Int32Value(1), ""}}},
	{
		"FloatToRealComplex",
		cp{{"a", `const Res = complex64(1.5)`, vdl.Complex64Value(1.5), ""}}},
	{
		"IntToRealComplex",
		cp{{"a", `const Res = complex64(2)`, vdl.Complex64Value(2), ""}}},

	// Test float rounding - note that 1.1 incurs loss of precision.
	{
		"RoundedCompareFloat32",
		cp{{"a", `const Res = float32(1.1) == 1.1`, vdl.BoolValue(true), ""}}},
	{
		"RoundedCompareFloat64",
		cp{{"a", `const Res = float64(1.1) == 1.1`, vdl.BoolValue(true), ""}}},
	{
		"RoundedTruncation",
		cp{{"a", `const Res = float64(float32(1.1)) != 1.1`, vdl.BoolValue(true), ""}}},

	// Test multi-package consts
	{"MultiPkgSameConstName", cp{
		{"a", `const Res = true`, vdl.BoolValue(true), ""},
		{"b", `const Res = true`, vdl.BoolValue(true), ""}}},
	{"MultiPkgDep", cp{
		{"a", `const Res = x;const x = true`, vdl.BoolValue(true), ""},
		{"b", `import "a";const Res = a.Res && false`, vdl.BoolValue(false), ""}}},
	{"MultiPkgUnexportedConst", cp{
		{"a", `const Res = x;const x = true`, vdl.BoolValue(true), ""},
		{"b", `import "a";const Res = a.x && false`, nil, "a.x undefined"}}},
	{"MultiPkgSamePkgName", cp{
		{"a", `const Res = true`, vdl.BoolValue(true), ""},
		{"a", `const Res = true`, nil, "invalid recompile"}}},
	{"MultiPkgUnimportedPkg", cp{
		{"a", `const Res = true`, vdl.BoolValue(true), ""},
		{"b", `const Res = a.Res && false`, nil, "a.Res undefined"}}},
	{"RedefinitionOfImportedName", cp{
		{"a", `const Res = true`, vdl.BoolValue(true), ""},
		{"b", `import "a"; const a = "test"; const Res = a`, nil, "const a name conflict"}}},
}
