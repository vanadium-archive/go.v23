package compile_test

import (
	"reflect"
	"testing"

	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vdl/build"
	"v.io/core/veyron2/vdl/compile"
	"v.io/core/veyron2/vdl/parse"
	"v.io/core/veyron2/vdl/vdltest"
)

func TestInterface(t *testing.T) {
	for _, test := range ifaceTests {
		env := compile.NewEnv(-1)
		for _, tpkg := range test.Pkgs {
			// Compile the package with a single file, adding the "package a" prefix
			// to the source data automatically.
			files := map[string]string{
				tpkg.Name + ".vdl": "package " + tpkg.Name + "\n" + tpkg.Data,
			}
			pkgPath := "p.kg/" + tpkg.Name // use dots in pkgpath to test tricky cases
			buildPkg := vdltest.FakeBuildPackage(tpkg.Name, pkgPath, files)
			pkg := build.BuildPackage(buildPkg, env)
			vdltest.ExpectResult(t, env.Errors, test.Name, tpkg.ErrRE)
			if pkg == nil || tpkg.ErrRE != "" {
				continue
			}
			matchIfaceRes(t, test.Name, tpkg, pkg.Files[0].Interfaces)
		}
	}
}

func matchIfaceRes(t *testing.T, tname string, tpkg ifacePkg, ifaces []*compile.Interface) {
	if tpkg.Iface == nil {
		return
	}
	// Look for an interface called "Res" to compare our expected results.
	for _, iface := range ifaces {
		if iface.Name == "Res" {
			if got, want := normalizeIface(*iface), normalizeIface(*tpkg.Iface); !reflect.DeepEqual(got, want) {
				t.Errorf("%s got %v, want %v", tname, got, want)
			}
			return
		}
	}
	t.Errorf("%s couldn't find Res in package %s", tname, tpkg.Name)
}

func normalizeIface(x compile.Interface) compile.Interface {
	// Don't compare uninteresting portions, to make tests more succinct.
	x.Pos = parse.Pos{}
	x.Exported = false
	x.File = nil
	embeds := x.Embeds
	x.Embeds = nil
	for _, embed := range embeds {
		norm := normalizeIface(*embed)
		x.Embeds = append(x.Embeds, &norm)
	}
	methods := x.Methods
	x.Methods = nil
	for _, method := range methods {
		norm := normalizeMethod(*method)
		x.Methods = append(x.Methods, &norm)
	}
	return x
}

func normalizeMethod(x compile.Method) compile.Method {
	x.Pos = parse.Pos{}
	x.InArgs = normalizeArgs(x.InArgs)
	x.OutArgs = normalizeArgs(x.OutArgs)
	return x
}

func normalizeArgs(x []*compile.Arg) (ret []*compile.Arg) {
	for _, arg := range x {
		norm := normalizeArg(*arg)
		ret = append(ret, &norm)
	}
	return
}

func normalizeArg(x compile.Arg) compile.Arg {
	x.Pos = parse.Pos{}
	return x
}

func np(name string) compile.NamePos {
	return compile.NamePos{Name: name}
}

type ifaceTest struct {
	Name string
	Pkgs ip
}

type ip []ifacePkg

type ifacePkg struct {
	Name  string
	Data  string
	Iface *compile.Interface
	ErrRE string
}

var ifaceTests = []ifaceTest{
	{"Empty", ip{{"a", `type Res interface{}`, &compile.Interface{NamePos: np("Res")}, ""}}},
	{"NoArgs", ip{{"a", `type Res interface{NoArgs() error}`,
		&compile.Interface{
			NamePos: np("Res"),
			Methods: []*compile.Method{
				{NamePos: np("NoArgs"), OutArgs: []*compile.Arg{{Type: vdl.ErrorType}}},
			},
		},
		"",
	}}},
	{"HasArgs", ip{{"a", `type Res interface{HasArgs(x bool) (string | error)}`,
		&compile.Interface{
			NamePos: np("Res"),
			Methods: []*compile.Method{{
				NamePos: np("HasArgs"),
				InArgs:  []*compile.Arg{{NamePos: np("x"), Type: vdl.BoolType}},
				OutArgs: []*compile.Arg{{Type: vdl.StringType}, {Type: vdl.ErrorType}},
			}},
		},
		"",
	}}},
	{"Embed", ip{{"a", `type A interface{};type Res interface{A}`,
		&compile.Interface{
			NamePos: np("Res"),
			Embeds:  []*compile.Interface{{NamePos: np("A")}},
		},
		"",
	}}},
	{"MultiEmbed", ip{{"a", `type A interface{};type B interface{};type Res interface{A;B}`,
		&compile.Interface{
			NamePos: np("Res"),
			Embeds:  []*compile.Interface{{NamePos: np("A")}, {NamePos: np("B")}},
		},
		"",
	}}},
	{"MultiPkgEmbed", ip{
		{"a", `type Res interface{}`, &compile.Interface{NamePos: np("Res")}, ""},
		{"b", `import "p.kg/a";type Res interface{a.Res}`,
			&compile.Interface{
				NamePos: np("Res"),
				Embeds:  []*compile.Interface{{NamePos: np("Res")}},
			},
			"",
		},
	}},
	{"MultiPkgEmbedQualifiedPath", ip{
		{"a", `type Res interface{}`, &compile.Interface{NamePos: np("Res")}, ""},
		{"b", `import "p.kg/a";type Res interface{"p.kg/a".Res}`,
			&compile.Interface{
				NamePos: np("Res"),
				Embeds:  []*compile.Interface{{NamePos: np("Res")}},
			},
			"",
		},
	}},
	{"UnmatchedEmbed", ip{{"a", `type A interface{};type Res interface{A.foobar}`, nil,
		`\(\.foobar unmatched\)`,
	}}},
}
