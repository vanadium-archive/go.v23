package build

import (
	"math/big"
	"reflect"
	"strings"
	"testing"
)

type parseTest struct {
	name   string
	src    string
	expect *File
	errors []string
}

// importTests contains tests of stuff up to and including the imports.
var importTests = []parseTest{
	// Empty file isn't allowed (need at least a package).
	{
		"FAILEmptyFile",
		"",
		nil,
		[]string{"File must start with package statement"}},

	// Comment tests.
	{
		"PackageDocOneLiner",
		`// One liner
// Another line
package testpkg`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{3, 1}, PackageDoc: `// One liner
// Another line
`},
		nil},
	{
		"PackageDocMultiLiner",
		`/* Multi liner
Another line
*/
package testpkg`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{4, 1}, PackageDoc: `/* Multi liner
Another line
*/
`},
		nil},
	{
		"NotPackageDoc",
		`// Extra newline, not package doc

package testpkg`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{3, 1}, PackageDoc: ""},
		nil},
	{
		"FAILUnterminatedComment",
		`/* Unterminated
Another line
package testpkg`,
		nil,
		[]string{"comment not terminated"}},

	// Package tests.
	{
		"Package",
		"package testpkg;",
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1}},
		nil},
	{
		"PackageNoSemi",
		"package testpkg",
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1}},
		nil},
	{
		"FAILBadPackageName",
		"package foo.bar",
		nil,
		[]string{"testfile:1:12 syntax error"}},

	// Import tests.
	{
		"EmptyImport",
		`package testpkg;
import (
)`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1}},
		nil},
	{
		"OneImport",
		`package testpkg;
import "foo/bar";`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			Imports: []*Import{{Path: "foo/bar", Pos: Pos{Line: 2, Col: 8}}}},
		nil},
	{
		"OneImportLocalNameNoSemi",
		`package testpkg
import baz "foo/bar"`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			Imports: []*Import{{LocalName: "baz", Path: "foo/bar", Pos: Pos{Line: 2, Col: 8}}}},
		nil},
	{
		"OneImportParens",
		`package testpkg
import (
  "foo/bar";
)`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			Imports: []*Import{{Path: "foo/bar", Pos: Pos{Line: 3, Col: 3}}}},
		nil},
	{
		"OneImportParensNoSemi",
		`package testpkg
import (
  "foo/bar"
)`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			Imports: []*Import{{Path: "foo/bar", Pos: Pos{Line: 3, Col: 3}}}},
		nil},
	{
		"MixedImports",
		`package testpkg
import "foo/bar"
import (
  "baz";"a/b"
  "c/d"
)
import "z"`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			Imports: []*Import{
				{Path: "foo/bar", Pos: Pos{Line: 2, Col: 8}},
				{Path: "baz", Pos: Pos{Line: 4, Col: 3}},
				{Path: "a/b", Pos: Pos{Line: 4, Col: 9}},
				{Path: "c/d", Pos: Pos{Line: 5, Col: 3}},
				{Path: "z", Pos: Pos{Line: 7, Col: 8}}}},
		nil},
	{
		"FAILImportParensNotClosed",
		`package testpkg
import (
  "foo/bar"`,
		nil,
		[]string{"testfile:3:12 syntax error"}},
}

// fullTests contains tests of stuff after the imports.
var fullTests = []parseTest{
	// Data type tests.
	{
		"NamedType",
		`package testpkg
type foo bar`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			TypeDefs: []*TypeDef{
				{Name: "foo", Pos: Pos{2, 6}, Base: &NamedType{
					TypeName: "bar", Pos: Pos{2, 10}}}}},
		nil},
	{
		"QualifiedNamedType",
		`package testpkg
type foo bar.baz`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			TypeDefs: []*TypeDef{
				{Name: "foo", Pos: Pos{2, 6}, Base: &NamedType{
					PackageName: "bar", TypeName: "baz", Pos: Pos{2, 10}}}}},
		nil},
	{
		"ArrayType",
		`package testpkg
type foo [2]bar`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			TypeDefs: []*TypeDef{
				{Name: "foo", Pos: Pos{2, 6}, Base: &ArrayType{
					Len: 2, Elem: &NamedType{TypeName: "bar", Pos: Pos{2, 13}}}}}},
		nil},
	{
		"SliceType",
		`package testpkg
type foo []bar`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			TypeDefs: []*TypeDef{
				{Name: "foo", Pos: Pos{2, 6}, Base: &SliceType{
					Elem: &NamedType{TypeName: "bar", Pos: Pos{2, 12}}}}}},
		nil},
	{
		"MapType",
		`package testpkg
type foo map[bar]baz`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			TypeDefs: []*TypeDef{
				{Name: "foo", Pos: Pos{2, 6}, Base: &MapType{
					Key:  &NamedType{TypeName: "bar", Pos: Pos{2, 14}},
					Elem: &NamedType{TypeName: "baz", Pos: Pos{2, 18}}}}}},
		nil},
	{
		"StructTypeOneField",
		`package testpkg
type foo struct{a b;}`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			TypeDefs: []*TypeDef{
				{Name: "foo", Pos: Pos{2, 6}, Base: &StructType{Fields: []*Field{
					{Name: "a", Pos: Pos{2, 17},
						Type: &NamedType{TypeName: "b", Pos: Pos{2, 19}}}}}}}},
		nil},
	{
		"StructTypeOneFieldNoSemi",
		`package testpkg
type foo struct{a b}`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			TypeDefs: []*TypeDef{
				{Name: "foo", Pos: Pos{2, 6}, Base: &StructType{Fields: []*Field{
					{Name: "a", Pos: Pos{2, 17},
						Type: &NamedType{TypeName: "b", Pos: Pos{2, 19}}}}}}}},
		nil},
	{
		"StructTypeOneFieldNewline",
		`package testpkg
type foo struct{
  a b;
}`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			TypeDefs: []*TypeDef{
				{Name: "foo", Pos: Pos{2, 6}, Base: &StructType{Fields: []*Field{
					{Name: "a", Pos: Pos{3, 3},
						Type: &NamedType{TypeName: "b", Pos: Pos{3, 5}}}}}}}},
		nil},
	{
		"StructTypeOneFieldNewlineNoSemi",
		`package testpkg
type foo struct{
  a b
}`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			TypeDefs: []*TypeDef{
				{Name: "foo", Pos: Pos{2, 6}, Base: &StructType{Fields: []*Field{
					{Name: "a", Pos: Pos{3, 3},
						Type: &NamedType{TypeName: "b", Pos: Pos{3, 5}}}}}}}},
		nil},
	{
		"StructTypeOneFieldList",
		`package testpkg
type foo struct{a,b,c d}`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			TypeDefs: []*TypeDef{
				{Name: "foo", Pos: Pos{2, 6}, Base: &StructType{Fields: []*Field{
					{Name: "a", Pos: Pos{2, 17},
						Type: &NamedType{TypeName: "d", Pos: Pos{2, 23}}},
					{Name: "b", Pos: Pos{2, 19},
						Type: &NamedType{TypeName: "d", Pos: Pos{2, 23}}},
					{Name: "c", Pos: Pos{2, 21},
						Type: &NamedType{TypeName: "d", Pos: Pos{2, 23}}}}}}}},
		nil},
	{
		"StructTypeMixed",
		`package testpkg
type foo struct{
  a b;c,d e
  f,g h
}`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			TypeDefs: []*TypeDef{
				{Name: "foo", Pos: Pos{2, 6}, Base: &StructType{Fields: []*Field{
					{Name: "a", Pos: Pos{3, 3},
						Type: &NamedType{TypeName: "b", Pos: Pos{3, 5}}},
					{Name: "c", Pos: Pos{3, 7},
						Type: &NamedType{TypeName: "e", Pos: Pos{3, 11}}},
					{Name: "d", Pos: Pos{3, 9},
						Type: &NamedType{TypeName: "e", Pos: Pos{3, 11}}},
					{Name: "f", Pos: Pos{4, 3},
						Type: &NamedType{TypeName: "h", Pos: Pos{4, 7}}},
					{Name: "g", Pos: Pos{4, 5},
						Type: &NamedType{TypeName: "h", Pos: Pos{4, 7}}}}}}}},
		nil},
	{
		"FAILStructTypeNotClosed",
		`package testpkg
type foo struct{
  a b`,
		nil,
		[]string{"testfile:3:6 syntax error"}},
	{
		"FAILStructTypeUnnamedField",
		`package testpkg
type foo struct{a}`,
		nil,
		[]string{"testfile:2:18 syntax error"}},
	{
		"FAILStructTypeUnnamedFieldList",
		`package testpkg
type foo struct{a, b}`,
		nil,
		[]string{"testfile:2:21 syntax error"}},

	// Const definition tests.
	{
		"BoolConst",
		`package testpkg
const foo = true
const bar = false`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			ConstDefs: []*ConstDef{
				{Name: "foo", Pos: Pos{2, 7}, expr: &literalConst{true, Pos{2, 13}}},
				{Name: "bar", Pos: Pos{3, 7}, expr: &literalConst{false, Pos{3, 13}}}}},
		nil},
	{
		"StringConst",
		"package testpkg\nconst foo = \"abc\"\nconst bar = `def`",
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			ConstDefs: []*ConstDef{
				{Name: "foo", Pos: Pos{2, 7},
					expr: &literalConst{"abc", Pos{2, 13}}},
				{Name: "bar", Pos: Pos{3, 7},
					expr: &literalConst{"def", Pos{3, 13}}}}},
		nil},
	{
		"IntegerConst",
		`package testpkg
const foo = 123`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			ConstDefs: []*ConstDef{
				{Name: "foo", Pos: Pos{2, 7},
					expr: &literalConst{big.NewInt(123), Pos{2, 13}}}}},
		nil},
	{
		"FloatConst",
		`package testpkg
const foo = 1.5`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			ConstDefs: []*ConstDef{
				{Name: "foo", Pos: Pos{2, 7},
					expr: &literalConst{big.NewRat(3, 2), Pos{2, 13}}}}},
		nil},
	{
		"NamedConst",
		`package testpkg
const foo = baz
const bar = pkg.box`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			ConstDefs: []*ConstDef{
				{Name: "foo", Pos: Pos{2, 7},
					expr: &namedConst{name: "baz", p: Pos{2, 13}}},
				{Name: "bar", Pos: Pos{3, 7},
					expr: &namedConst{packageName: "pkg", name: "box", p: Pos{3, 13}}}}},
		nil},
	{
		"UnaryOpConst",
		`package testpkg
const foo = !false
const bar = +1
const baz = -2
const box = ^3`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			ConstDefs: []*ConstDef{
				{Name: "foo", Pos: Pos{2, 7},
					expr: &unaryOpConst{"!",
						&literalConst{false, Pos{2, 14}}, Pos{2, 13}}},
				{Name: "bar", Pos: Pos{3, 7},
					expr: &unaryOpConst{"+",
						&literalConst{big.NewInt(1), Pos{3, 14}}, Pos{3, 13}}},
				{Name: "baz", Pos: Pos{4, 7},
					expr: &unaryOpConst{"-",
						&literalConst{big.NewInt(2), Pos{4, 14}}, Pos{4, 13}}},
				{Name: "box", Pos: Pos{5, 7},
					expr: &unaryOpConst{"^",
						&literalConst{big.NewInt(3), Pos{5, 14}}, Pos{5, 13}}}}},
		nil},
	{
		"TypeConvConst",
		`package testpkg
const foo = baz(true)
const bar = pkg.box(false)`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			ConstDefs: []*ConstDef{
				{Name: "foo", Pos: Pos{2, 7},
					expr: &typeConvConst{&NamedType{"", "baz", Pos{2, 13}, nil, false},
						&literalConst{true, Pos{2, 17}}, Pos{2, 13}}},
				{Name: "bar", Pos: Pos{3, 7},
					expr: &typeConvConst{&NamedType{"pkg", "box", Pos{3, 13}, nil, false},
						&literalConst{false, Pos{3, 21}}, Pos{3, 13}}}}},
		nil},
	{
		"BinaryOpConst",
		`package testpkg
const a = true || false
const b = true && false
const c = 1 < 2
const d = 3 > 4
const e = 5 <= 6
const f = 7 >= 8
const g = 9 != 8
const h = 7 == 6
const i = 5 + 4
const j = 3 - 2
const k = 1 * 2
const l = 3 / 4
const m = 5 % 6
const n = 7 | 8
const o = 9 & 8
const p = 7 ^ 6
const q = 5 << 4
const r = 3 >> 2`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			ConstDefs: []*ConstDef{
				{Name: "a", Pos: Pos{2, 7},
					expr: &binaryOpConst{"||", &literalConst{true, Pos{2, 11}},
						&literalConst{false, Pos{2, 19}}, Pos{2, 16}}},
				{Name: "b", Pos: Pos{3, 7},
					expr: &binaryOpConst{"&&", &literalConst{true, Pos{3, 11}},
						&literalConst{false, Pos{3, 19}}, Pos{3, 16}}},
				{Name: "c", Pos: Pos{4, 7},
					expr: &binaryOpConst{"<", &literalConst{big.NewInt(1), Pos{4, 11}},
						&literalConst{big.NewInt(2), Pos{4, 15}}, Pos{4, 13}}},
				{Name: "d", Pos: Pos{5, 7},
					expr: &binaryOpConst{">", &literalConst{big.NewInt(3), Pos{5, 11}},
						&literalConst{big.NewInt(4), Pos{5, 15}}, Pos{5, 13}}},
				{Name: "e", Pos: Pos{6, 7},
					expr: &binaryOpConst{"<=", &literalConst{big.NewInt(5), Pos{6, 11}},
						&literalConst{big.NewInt(6), Pos{6, 16}}, Pos{6, 13}}},
				{Name: "f", Pos: Pos{7, 7},
					expr: &binaryOpConst{">=", &literalConst{big.NewInt(7), Pos{7, 11}},
						&literalConst{big.NewInt(8), Pos{7, 16}}, Pos{7, 13}}},
				{Name: "g", Pos: Pos{8, 7},
					expr: &binaryOpConst{"!=", &literalConst{big.NewInt(9), Pos{8, 11}},
						&literalConst{big.NewInt(8), Pos{8, 16}}, Pos{8, 13}}},
				{Name: "h", Pos: Pos{9, 7},
					expr: &binaryOpConst{"==", &literalConst{big.NewInt(7), Pos{9, 11}},
						&literalConst{big.NewInt(6), Pos{9, 16}}, Pos{9, 13}}},
				{Name: "i", Pos: Pos{10, 7},
					expr: &binaryOpConst{"+", &literalConst{big.NewInt(5), Pos{10, 11}},
						&literalConst{big.NewInt(4), Pos{10, 15}}, Pos{10, 13}}},
				{Name: "j", Pos: Pos{11, 7},
					expr: &binaryOpConst{"-", &literalConst{big.NewInt(3), Pos{11, 11}},
						&literalConst{big.NewInt(2), Pos{11, 15}}, Pos{11, 13}}},
				{Name: "k", Pos: Pos{12, 7},
					expr: &binaryOpConst{"*", &literalConst{big.NewInt(1), Pos{12, 11}},
						&literalConst{big.NewInt(2), Pos{12, 15}}, Pos{12, 13}}},
				{Name: "l", Pos: Pos{13, 7},
					expr: &binaryOpConst{"/", &literalConst{big.NewInt(3), Pos{13, 11}},
						&literalConst{big.NewInt(4), Pos{13, 15}}, Pos{13, 13}}},
				{Name: "m", Pos: Pos{14, 7},
					expr: &binaryOpConst{"%", &literalConst{big.NewInt(5), Pos{14, 11}},
						&literalConst{big.NewInt(6), Pos{14, 15}}, Pos{14, 13}}},
				{Name: "n", Pos: Pos{15, 7},
					expr: &binaryOpConst{"|", &literalConst{big.NewInt(7), Pos{15, 11}},
						&literalConst{big.NewInt(8), Pos{15, 15}}, Pos{15, 13}}},
				{Name: "o", Pos: Pos{16, 7},
					expr: &binaryOpConst{"&", &literalConst{big.NewInt(9), Pos{16, 11}},
						&literalConst{big.NewInt(8), Pos{16, 15}}, Pos{16, 13}}},
				{Name: "p", Pos: Pos{17, 7},
					expr: &binaryOpConst{"^", &literalConst{big.NewInt(7), Pos{17, 11}},
						&literalConst{big.NewInt(6), Pos{17, 15}}, Pos{17, 13}}},
				{Name: "q", Pos: Pos{18, 7},
					expr: &binaryOpConst{"<<", &literalConst{big.NewInt(5), Pos{18, 11}},
						&literalConst{big.NewInt(4), Pos{18, 16}}, Pos{18, 13}}},
				{Name: "r", Pos: Pos{19, 7},
					expr: &binaryOpConst{">>", &literalConst{big.NewInt(3), Pos{19, 11}},
						&literalConst{big.NewInt(2), Pos{19, 16}}, Pos{19, 13}}}}},
		nil},
	{
		"FAILConstOnlyName",
		`package testpkg
const foo`,
		nil,
		[]string{"testfile:2:10 syntax error"}},
	{
		"FAILConstNoEquals",
		`package testpkg
const foo bar`,
		nil,
		[]string{"testfile:2:11 syntax error"}},
	{
		"FAILConstNoValue",
		`package testpkg
const foo =`,
		nil,
		[]string{"testfile:2:12 syntax error"}},

	// Error definition tests.
	{
		"ErrorEmpty",
		`package testpkg
errorid()`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1}},
		nil},
	{
		"ErrorID",
		`package testpkg
errorid ErrIDFoo`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			ErrorIDs: []*ErrorID{
				{Name: "ErrIDFoo", ID: "", Pos: Pos{2, 9}}}},
		nil},
	{
		"ErrorIDOverwrite",
		`package testpkg
errorid ErrIDFoo = "pkg/path.ErrName"`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			ErrorIDs: []*ErrorID{
				{Name: "ErrIDFoo", ID: "pkg/path.ErrName", Pos: Pos{2, 9}}}},
		nil},
	{
		"ErrorMixed",
		`package testpkg
errorid ErrIDFoo
errorid (
  ErrIDBar
  ErrIDBaz = "pkg/path.ErrIDBaz"
)`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			ErrorIDs: []*ErrorID{
				{Name: "ErrIDFoo", ID: "", Pos: Pos{2, 9}},
				{Name: "ErrIDBar", ID: "", Pos: Pos{4, 3}},
				{Name: "ErrIDBaz", ID: "pkg/path.ErrIDBaz", Pos: Pos{5, 3}}}},
		nil},
	{
		"FAILErrorEmptyID",
		`package testpkg
errorid ErrIDFoo = ""`,
		nil,
		[]string{"testfile:2:20 Error id must be non-empty if specified"}},

	// Interface tests.
	{
		"InterfaceEmpty",
		`package testpkg
type foo interface{}`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			Interfaces: []*Interface{{Name: "foo", Pos: Pos{2, 6}, Def: &TypeDef{Name: "foo", Base: &InterfaceType{}, Pos: Pos{2, 6}}}}},
		nil},
	{
		"InterfaceOneMethodNoArgs",
		`package testpkg
type foo interface{meth1()}`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			Interfaces: []*Interface{
				{Name: "foo", Pos: Pos{2, 6}, Def: &TypeDef{Name: "foo", Base: &InterfaceType{}, Pos: Pos{2, 6}},
					Components: []InterfaceComponent{&Method{Name: "meth1", Pos: Pos{2, 20}}}}}},
		nil},
	{
		"InterfaceOneMethodOneInUnnamedOut",
		`package testpkg
type foo interface{meth1(a b) c}`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			Interfaces: []*Interface{
				{Name: "foo", Pos: Pos{2, 6}, Def: &TypeDef{Name: "foo", Base: &InterfaceType{}, Pos: Pos{2, 6}},
					Components: []InterfaceComponent{&Method{Name: "meth1", Pos: Pos{2, 20},
						InArgs: []*Field{
							{Name: "a", Pos: Pos{2, 26},
								Type: &NamedType{TypeName: "b", Pos: Pos{2, 28}}}},
						OutArgs: []*Field{
							{Name: "", Pos: Pos{2, 31},
								Type: &NamedType{TypeName: "c", Pos: Pos{2, 31}}}}}}}}},
		nil},
	{
		"InterfaceOneMethodOneInNamedOut",
		`package testpkg
type foo interface{meth1(a b) (c d)}`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			Interfaces: []*Interface{
				{Name: "foo", Pos: Pos{2, 6}, Def: &TypeDef{Name: "foo", Base: &InterfaceType{}, Pos: Pos{2, 6}},
					Components: []InterfaceComponent{&Method{Name: "meth1", Pos: Pos{2, 20},
						InArgs: []*Field{
							{Name: "a", Pos: Pos{2, 26},
								Type: &NamedType{TypeName: "b", Pos: Pos{2, 28}}}},
						OutArgs: []*Field{
							{Name: "c", Pos: Pos{2, 32},
								Type: &NamedType{TypeName: "d", Pos: Pos{2, 34}}}}}}}}},
		nil},
	{
		"InterfaceErrors",
		`package testpkg
type foo interface{meth1(err error) error}`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			Interfaces: []*Interface{
				{Name: "foo", Pos: Pos{2, 6}, Def: &TypeDef{Name: "foo", Base: &InterfaceType{}, Pos: Pos{2, 6}},
					Components: []InterfaceComponent{&Method{Name: "meth1", Pos: Pos{2, 20},
						InArgs: []*Field{
							{Name: "err", Pos: Pos{2, 26},
								Type: &NamedType{TypeName: "error", Pos: Pos{2, 30}}}},
						OutArgs: []*Field{
							{Name: "", Pos: Pos{2, 37},
								Type: &NamedType{TypeName: "error", Pos: Pos{2, 37}}}}}}}}},
		nil},
	{
		"InterfaceMixedMethods",
		`package testpkg
type foo interface{
  meth1(a b) (c d);meth2()
  meth3(e f, g, h i) (j k, l, m n)
}`,
		&File{BaseName: "testfile", PackageName: "testpkg", PackagePos: Pos{1, 1},
			Interfaces: []*Interface{
				{Name: "foo", Pos: Pos{2, 6}, Def: &TypeDef{Name: "foo", Base: &InterfaceType{}, Pos: Pos{2, 6}},
					Components: []InterfaceComponent{&Method{Name: "meth1", Pos: Pos{3, 3},
						InArgs: []*Field{
							{Name: "a", Pos: Pos{3, 9},
								Type: &NamedType{TypeName: "b", Pos: Pos{3, 11}}}},
						OutArgs: []*Field{
							{Name: "c", Pos: Pos{3, 15},
								Type: &NamedType{TypeName: "d", Pos: Pos{3, 17}}}}},
						&Method{Name: "meth2", Pos: Pos{3, 20}},
						&Method{Name: "meth3", Pos: Pos{4, 3},
							InArgs: []*Field{
								{Name: "e", Pos: Pos{4, 9},
									Type: &NamedType{TypeName: "f", Pos: Pos{4, 11}}},
								{Name: "g", Pos: Pos{4, 14},
									Type: &NamedType{TypeName: "i", Pos: Pos{4, 19}}},
								{Name: "h", Pos: Pos{4, 17},
									Type: &NamedType{TypeName: "i", Pos: Pos{4, 19}}}},
							OutArgs: []*Field{
								{Name: "j", Pos: Pos{4, 23},
									Type: &NamedType{TypeName: "k", Pos: Pos{4, 25}}},
								{Name: "l", Pos: Pos{4, 28},
									Type: &NamedType{TypeName: "n", Pos: Pos{4, 33}}},
								{Name: "m", Pos: Pos{4, 31},
									Type: &NamedType{TypeName: "n", Pos: Pos{4, 33}}}}}}}}},
		nil},
	{
		"FAILInterfaceUnclosedInterface",
		`package testpkg
type foo interface{
  meth1()`,
		nil,
		[]string{"testfile:3:10 syntax error"}},
	{
		"FAILInterfaceUnclosedArgs",
		`package testpkg
type foo interface{
  meth1(
}`,
		nil,
		[]string{"testfile:4:1 syntax error"}},
	{
		"FAILInterfaceVariableNames",
		`package testpkg
type foo interface{
  meth1([]a, []b []c)
}`,
		nil,
		[]string{"Expected one or more variable names",
			"testfile:3:18 Perhaps you forgot a comma"}},
}

func testParseFile(t *testing.T, test parseTest, opts parseOpts) {
	// You can't create recursive values using composite literals, so we fill in
	// the File field in every TypeDef and ConstDef here.
	if test.expect != nil {
		for _, def := range test.expect.TypeDefs {
			def.File = test.expect
		}
		for _, def := range test.expect.ConstDefs {
			def.File = test.expect
		}
		for _, iface := range test.expect.Interfaces {
			if iface.Def == nil {
				t.Errorf("%v interface %v has no def", test.name, iface.Name)
				continue
			}
			iface.Def.File = test.expect
		}
	}

	env := NewEnv(100)
	actual := parseFile("testfile", strings.NewReader(test.src), opts, env)
	expectResult(t, env, test.name, test.errors...)
	if !reflect.DeepEqual(test.expect, actual) {
		t.Errorf("%v\nEXPECT %+v\nACTUAL %+v", test.name, test.expect, actual)
	}
}

func TestImportsOnlyParser(t *testing.T) {
	for _, test := range importTests {
		testParseFile(t, test, parseOpts{ImportsOnly: true})
	}
	for _, test := range fullTests {
		// We only run the success tests from fullTests on the imports only parser,
		// since the failure tests are testing failures in stuff after the imports,
		// which won't cause failures in the imports only parser.
		//
		// The imports-only parser isn't supposed to fill in fields after the
		// imports, so we clear them from the expected result.  We must copy the
		// file to ensure the actual parseTests isn't overwritten since the
		// full-parser tests needs the full expectations.  The test itself doesn't
		// need to be copied, since it's already copied in the range-for.
		if test.expect != nil {
			copyFile := *test.expect
			test.expect = &copyFile
			test.expect.TypeDefs = nil
			test.expect.ConstDefs = nil
			test.expect.ErrorIDs = nil
			test.expect.Interfaces = nil
			testParseFile(t, test, parseOpts{ImportsOnly: true})
		}
	}
}

func TestFullParser(t *testing.T) {
	for _, test := range append(importTests, fullTests...) {
		testParseFile(t, test, parseOpts{ImportsOnly: false})
	}
}
