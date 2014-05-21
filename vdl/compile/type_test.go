package compile_test

import (
	"testing"

	"veyron2/val"
	"veyron2/vdl/build"
	"veyron2/vdl/compile"
	"veyron2/vdl/vdltest"
)

func TestType(t *testing.T) {
	for _, test := range typeTests {
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
			matchTypeRes(t, test.Name, tpkg, pkg.Files[0].TypeDefs)
		}
	}
}

func matchTypeRes(t *testing.T, tname string, tpkg typePkg, tdefs []*compile.TypeDef) {
	if tpkg.ExpectBase == nil {
		return
	}
	// Look for a TypeDef called "Res" to compare our expected results.
	for _, tdef := range tdefs {
		if tdef.Name == "Res" {
			base, res, resname := tpkg.ExpectBase, tpkg.ExpectBase, tpkg.Name+".Res"
			if res.Name() != resname {
				res = val.NamedType(resname, base)
			} else {
				base = nil
			}
			if got, want := tdef.Type, res; got != want {
				t.Errorf("%s type got %s, want %s", tname, got, want)
			}
			if got, want := tdef.BaseType, base; got != want {
				t.Errorf("%s base type got %s, want %s", tname, got, want)
			}
			return
		}
	}
	t.Errorf("%s couldn't find Res in package %s", tname, tpkg.Name)
}

type typePkg struct {
	Name       string
	Data       string
	ExpectBase *val.Type
	ErrRE      string
}

type tp []typePkg

func namedX(base *val.Type) *val.Type {
	return val.NamedType("a.X", base)
}

func namedRes(base *val.Type) *val.Type {
	return val.NamedType("a.Res", base)
}

var typeTests = []struct {
	Name string
	Pkgs tp
}{
	// Test named built-ins.
	{"Bool", tp{{"a", `type Res bool`, val.BoolType, ""}}},
	{"Int32", tp{{"a", `type Res int32`, val.Int32Type, ""}}},
	{"Int64", tp{{"a", `type Res int64`, val.Int64Type, ""}}},
	{"Uint32", tp{{"a", `type Res uint32`, val.Uint32Type, ""}}},
	{"Uint64", tp{{"a", `type Res uint64`, val.Uint64Type, ""}}},
	{"Float32", tp{{"a", `type Res float32`, val.Float32Type, ""}}},
	{"Float64", tp{{"a", `type Res float64`, val.Float64Type, ""}}},
	{"Complex64", tp{{"a", `type Res complex64`, val.Complex64Type, ""}}},
	{"Complex128", tp{{"a", `type Res complex128`, val.Complex128Type, ""}}},
	{"String", tp{{"a", `type Res string`, val.StringType, ""}}},
	{"Bytes", tp{{"a", `type Res bytes`, val.BytesType, ""}}},
	{"Typeval", tp{{"a", `type Res typeval`, nil, "any and typeval cannot be renamed"}}},
	{"Any", tp{{"a", `type Res any`, nil, "any and typeval cannot be renamed"}}},
	{"Error", tp{{"a", `type Res error`, nil, "error cannot be renamed"}}},

	// Test composite types.
	{"Array", tp{{"a", `type Res [2]bool`, nil, "arrays are not supported"}}},
	{"List", tp{{"a", `type Res []int32`, val.ListType(val.Int32Type), ""}}},
	{"Map", tp{{"a", `type Res map[int32]string`, val.MapType(val.Int32Type, val.StringType), ""}}},
	{"Struct", tp{{"a", `type Res struct{A int32;B string}`, val.StructType("a.Res", []val.StructField{{"A", val.Int32Type}, {"B", val.StringType}}), ""}}},

	// Test named types based on named types.
	{"Bool", tp{{"a", `type Res X;type X bool`, namedX(val.BoolType), ""}}},
	{"Int32", tp{{"a", `type Res X;type X int32`, namedX(val.Int32Type), ""}}},
	{"Int64", tp{{"a", `type Res X;type X int64`, namedX(val.Int64Type), ""}}},
	{"Uint32", tp{{"a", `type Res X;type X uint32`, namedX(val.Uint32Type), ""}}},
	{"Uint64", tp{{"a", `type Res X;type X uint64`, namedX(val.Uint64Type), ""}}},
	{"Float32", tp{{"a", `type Res X;type X float32`, namedX(val.Float32Type), ""}}},
	{"Float64", tp{{"a", `type Res X;type X float64`, namedX(val.Float64Type), ""}}},
	{"Complex64", tp{{"a", `type Res X;type X complex64`, namedX(val.Complex64Type), ""}}},
	{"Complex128", tp{{"a", `type Res X;type X complex128`, namedX(val.Complex128Type), ""}}},
	{"String", tp{{"a", `type Res X;type X string`, namedX(val.StringType), ""}}},
	{"Bytes", tp{{"a", `type Res X;type X bytes`, namedX(val.BytesType), ""}}},
	{"List", tp{{"a", `type Res X;type X []int32`, namedX(val.ListType(val.Int32Type)), ""}}},
	{"Map", tp{{"a", `type Res X;type X map[int32]string`, namedX(val.MapType(val.Int32Type, val.StringType)), ""}}},
	{"Struct", tp{{"a", `type Res X;type X struct{A int32;B string}`, namedX(val.StructType("a.Res", []val.StructField{{"A", val.Int32Type}, {"B", val.StringType}})), ""}}},

	// Test multi-package types
	{"MultiPkgSameTypeName", tp{
		{"a", `type Res bool`, val.BoolType, ""},
		{"b", `type Res bool`, val.BoolType, ""}}},
	{"MultiPkgDep", tp{
		{"a", `type Res bool`, val.BoolType, ""},
		{"b", `import "a";type Res []a.Res`, val.ListType(namedRes(val.BoolType)), ""}}},
	{"MultiPkgSamePkgName", tp{
		{"a", `type Res bool`, val.BoolType, ""},
		{"a", `type Res bool`, nil, "invalid recompile"}}},
	{"MultiPkgUnimportedPkg", tp{
		{"a", `type Res bool`, val.BoolType, ""},
		{"b", `type Res []a.Res`, nil, "type a.Res undefined"}}},

	// Test errors.
	{"DupSame", tp{{"a", `type Res bool; type Res bool`, nil, "type Res redefined"}}},
	{"DupDiff", tp{{"a", `type Res bool; type Res int32`, nil, "type Res redefined"}}},
	{"InvalidName", tp{{"a", `type _Res bool`, nil, "type _Res invalid name"}}},
	{"Undefined", tp{{"a", `type Res foo`, nil, "type foo undefined"}}},
	{"UnnamedStruct", tp{{"a", `type Res []struct{A int32}`, nil, "unnamed struct type invalid"}}},
}
