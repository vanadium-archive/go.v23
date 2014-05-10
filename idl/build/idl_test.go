package build

import (
	"testing"
)

var (
	primBool  = &NamedType{TypeName: "bool", TypeDef: globalBool}
	primInt32 = &NamedType{TypeName: "int32", TypeDef: globalInt32}
	primError = &NamedType{TypeName: "error", TypeDef: globalError}
)

var fieldTests = []struct {
	Name               string
	field              *Field
	dontNeedNameResult string
	mustHaveNameResult string
}{
	{
		"No name",
		&Field{Name: "", Type: primBool},
		"",
		"has no name",
	},
	{
		"Non-exported name",
		&Field{Name: "foo", Type: primBool},
		"has a non-exported field name",
		"has a non-exported field name",
	},
	{
		"Unresolved type",
		&Field{Name: "Foo", Type: &NamedType{PackageName: "foo", TypeName: "bar"}},
		"internal error: type foo.bar isn't resolved",
		"internal error: type foo.bar isn't resolved",
	},
	{
		"Good field",
		&Field{Name: "Foo", Type: primBool},
		"",
		"",
	},
}

func TestFieldVerify(t *testing.T) {
	env := NewEnv(100)
	for _, test := range fieldTests {
		test.field.Verify(dontNeedName, "f", "m", false, env)
		expectResult(t, env, test.Name, test.dontNeedNameResult)
		test.field.Verify(mustHaveName, "f", "m", false, env)
		expectResult(t, env, test.Name, test.mustHaveNameResult)
	}
}

var methodTests = []struct {
	Name   string
	method *Method
	Result []string
}{
	{
		"No name",
		&Method{Name: "",
			OutArgs: []*Field{&Field{Name: "E", Type: primError}}},
		[]string{"interface Iface method has no name"},
	},
	{
		"Non-exported name",
		&Method{Name: "meth",
			OutArgs: []*Field{&Field{Name: "E", Type: primError}}},
		[]string{"interface Iface method meth isn't exported"},
	},
	{
		"No in args name",
		&Method{Name: "Meth",
			InArgs:  []*Field{&Field{Name: "", Type: primBool}},
			OutArgs: []*Field{&Field{Name: "E", Type: primError}}},
		[]string{"interface Iface method Meth input arg #0 has no name"},
	},
	{
		"No in & out args names",
		&Method{Name: "Meth",
			InArgs: []*Field{&Field{Name: "", Type: primBool}},
			OutArgs: []*Field{
				&Field{Name: "", Type: primBool},
				&Field{Name: "", Type: primInt32},
				&Field{Name: "", Type: primError}}},
		[]string{"interface Iface method Meth input arg #0 has no name",
			"interface Iface method Meth output arg #[012] has no name"},
	},
	{
		"No out args",
		&Method{Name: "Meth",
			InArgs:  []*Field{&Field{Name: "E", Type: primBool}},
			OutArgs: nil},
		[]string{"interface Iface method Meth last output arg isn't type \"error\""},
	},
	{
		"No error out arg in the last position",
		&Method{Name: "Meth",
			InArgs: []*Field{&Field{Name: "E", Type: primBool}},
			OutArgs: []*Field{
				&Field{Name: "", Type: primError},
				&Field{Name: "", Type: primInt32},
			}},
		[]string{"interface Iface method Meth last output arg isn't type \"error\""},
	},
	{
		"Invalid tag",
		&Method{Name: "Meth",
			OutArgs: []*Field{&Field{Type: primError}},
			Tags:    []Const{Const{}}},
		[]string{"interface Iface method Meth tag #0 isn't valid"},
	},
	{
		"Good method with unnamed output args",
		&Method{Name: "Meth",
			InArgs: []*Field{&Field{Name: "B", Type: primBool}},
			OutArgs: []*Field{
				&Field{Name: "", Type: primInt32},
				&Field{Name: "", Type: primError}}},
		nil,
	},
	{
		"Good method with named output args",
		&Method{Name: "Meth",
			InArgs: []*Field{&Field{Name: "B", Type: primBool}},
			OutArgs: []*Field{
				&Field{Name: "I", Type: primInt32},
				&Field{Name: "E", Type: primError}}},
		nil,
	},
	{
		"Good method with valid tag",
		&Method{Name: "Meth",
			OutArgs: []*Field{&Field{Type: primError}},
			Tags:    []Const{Const{true, nil}}},
		nil,
	},
}

func TestMethodVerify(t *testing.T) {
	env := NewEnv(100)
	for _, test := range methodTests {
		test.method.Verify("f", "Iface", env)
		expectResult(t, env, test.Name, test.Result...)
	}
}

var ifaceTests = []struct {
	Name   string
	iface  *Interface
	Result string
}{
	{
		"No name",
		&Interface{
			Name: "",
			Components: []InterfaceComponent{
				&Method{Name: "Meth", OutArgs: []*Field{
					&Field{Name: "E", Type: primError}}}}},
		"interface #0 has no name",
	},
	{
		"Non-exported name",
		&Interface{
			Name: "iface",
			Components: []InterfaceComponent{
				&Method{Name: "Meth", OutArgs: []*Field{
					&Field{Name: "E", Type: primError}}}}},
		"interface iface isn't exported",
	},
	{
		"No methods",
		&Interface{
			Name:       "Iface",
			Components: []InterfaceComponent{}},
		"interface Iface has no methods or embedded interfaces",
	},
	{
		"Good interface with method",
		&Interface{
			Name: "Iface",
			Components: []InterfaceComponent{
				&Method{Name: "Meth", OutArgs: []*Field{
					&Field{Name: "E", Type: primError}}}}},
		"",
	},
	{
		"Good interface with embedded interface",
		&Interface{
			Name: "Iface",
			Components: []InterfaceComponent{
				&EmbeddedInterface{Type: primError}}},
		"",
	},
}

func TestInterfaceVerify(t *testing.T) {
	env := NewEnv(100)
	for _, test := range ifaceTests {
		test.iface.Verify("f", 0, env)
		expectResult(t, env, test.Name, test.Result)
	}
}

var fileTests = []struct {
	Name   string
	file   *File
	Result string
}{
	{
		"No name",
		&File{BaseName: "", PackageName: "pkg"},
		"empty file name in package pkg",
	},
	{
		"Mismatched pkg name",
		&File{BaseName: "file", PackageName: "pkg2"},
		"file expected package pkg, actual package pkg2",
	},
	{
		"No import path",
		&File{BaseName: "file", PackageName: "pkg", Imports: []*Import{
			&Import{Path: ""}}},
		"empty import path",
	},
	{
		"Dup import path",
		&File{BaseName: "file", PackageName: "pkg", Imports: []*Import{
			&Import{LocalName: "a", Path: "foo"},
			&Import{LocalName: "a", Path: "bar"}}},
		"duplicate import package name a",
	},
	{
		"Unresolved type def",
		&File{BaseName: "file", PackageName: "pkg",
			TypeDefs: []*TypeDef{&TypeDef{Name: "Foo", Base: &NamedType{}}}},
		"package pkg type Foo isn't resolved",
	},
	{
		"Invalid const",
		&File{BaseName: "file", PackageName: "pkg",
			ConstDefs: []*ConstDef{&ConstDef{Name: "Foo"}}},
		"package pkg const Foo isn't valid",
	},
	{
		"No interface name",
		&File{BaseName: "file", PackageName: "pkg", Interfaces: []*Interface{
			&Interface{Name: "", Components: []InterfaceComponent{
				&Method{Name: "Meth", OutArgs: []*Field{
					&Field{Name: "E", Type: primError}}}}}}},
		"interface #0 has no name",
	},
	{
		"Good file",
		&File{BaseName: "file", PackageName: "pkg",
			TypeDefs:  []*TypeDef{&TypeDef{Name: "Foo", Base: primBool}},
			ConstDefs: []*ConstDef{&ConstDef{Name: "Bar", Const: Const{true, nil}}},
			Interfaces: []*Interface{
				&Interface{Name: "Iface", Components: []InterfaceComponent{
					&Method{Name: "Meth", OutArgs: []*Field{
						&Field{Name: "E", Type: primError}}}}}}},
		"",
	},
	{
		"Bad file",
		&File{BaseName: "file", PackageName: "pkg",
			TypeDefs:  []*TypeDef{&TypeDef{Name: "Baz", Kind: KindStruct, Base: &StructType{Fields: []*Field{&Field{Name: "o", Type: primInt32}}, TypeDef: globalPrimitive(KindStruct)}}},
			ConstDefs: []*ConstDef{&ConstDef{Name: "Bar", Const: Const{true, nil}}},
			Interfaces: []*Interface{
				&Interface{Name: "Iface", Components: []InterfaceComponent{
					&Method{Name: "Meth", OutArgs: []*Field{
						&Field{Name: "E", Type: primError}}}}}}},
		"type pkg.Baz has a non-exported field name \"o\" - all struct fields must be exported if they're named.",
	},
	{
		"Another good file",
		&File{BaseName: "file", PackageName: "pkg",
			TypeDefs:  []*TypeDef{&TypeDef{Name: "Baz", Kind: KindStruct, Base: &StructType{Fields: []*Field{&Field{Name: "O", Type: primInt32}}, TypeDef: globalPrimitive(KindStruct)}}},
			ConstDefs: []*ConstDef{&ConstDef{Name: "Bar", Const: Const{true, nil}}},
			Interfaces: []*Interface{
				&Interface{Name: "Iface", Components: []InterfaceComponent{
					&Method{Name: "Meth",
						InArgs: []*Field{
							&Field{Name: "a", Type: primBool}},
						OutArgs: []*Field{
							&Field{Name: "r", Type: primBool},
							&Field{Name: "E", Type: primError}}}}}}},
		"",
	},
}

func TestFileVerify(t *testing.T) {
	env := NewEnv(100)
	for _, test := range fileTests {
		test.file.Verify("pkg", env)
		expectResult(t, env, test.Name, test.Result)
	}
}

var pkgTests = []struct {
	Name   string
	pkg    *Package
	Result string
}{
	{
		"No name",
		&Package{Name: "", Dir: "/home/dir", Files: []*File{
			&File{BaseName: "file", PackageName: "pkg", Interfaces: []*Interface{
				&Interface{Name: "Iface", Components: []InterfaceComponent{
					&Method{Name: "Meth", OutArgs: []*Field{
						&Field{Name: "E", Type: primError}}}}}}}}},
		"empty package name",
	},
	{
		"No dir name",
		&Package{Name: "pkg", Dir: "", Files: []*File{
			&File{BaseName: "file", PackageName: "pkg", Interfaces: []*Interface{
				&Interface{Name: "Iface", Components: []InterfaceComponent{
					&Method{Name: "Meth", OutArgs: []*Field{
						&Field{Name: "E", Type: primError}}}}}}}}},
		"empty package dir for package pkg",
	},
	{
		"No files",
		&Package{Name: "pkg", Dir: "/home/dir}"},
		"no files in package pkg",
	},
	{
		"No file name",
		&Package{Name: "pkg", Dir: "/home/dir", Files: []*File{
			&File{BaseName: "", PackageName: "pkg", Interfaces: []*Interface{
				&Interface{Name: "Iface", Components: []InterfaceComponent{
					&Method{Name: "Meth", OutArgs: []*Field{
						&Field{Name: "E", Type: primError}}}}}}}}},
		"empty file name in package pkg",
	},
	{
		"Good package",
		&Package{Name: "pkg", Dir: "/home/dir", Files: []*File{
			&File{BaseName: "file", PackageName: "pkg", Interfaces: []*Interface{
				&Interface{Name: "Iface", Components: []InterfaceComponent{
					&Method{Name: "Meth", OutArgs: []*Field{
						&Field{Name: "E", Type: primError}}}}}}}}},
		"",
	},
}

func TestPackageVerify(t *testing.T) {
	env := NewEnv(100)
	for _, test := range pkgTests {
		test.pkg.Verify(env)
		expectResult(t, env, test.Name, test.Result)
	}
}
