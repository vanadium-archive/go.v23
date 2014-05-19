package parse_test

// TODO(toddw): Add tests for embedded interfaces, imaginary literals.

import (
	"math/big"
	"reflect"
	"strings"
	"testing"

	"veyron2/idl2"
	"veyron2/idl2/idltest"
	"veyron2/idl2/parse"
)

type parseTest struct {
	name   string
	src    string
	expect *parse.File
	errors []string
}

func pos(line, col int) parse.Pos {
	return parse.Pos{line, col}
}

func np(name string, line, col int) parse.NamePos {
	return parse.NamePos{Name: name, Pos: pos(line, col)}
}

func tn(name string, line, col int) *parse.TypeNamed {
	return &parse.TypeNamed{Name: name, P: pos(line, col)}
}

// importTests contains tests of stuff up to and including the imports.
var importTests = []parseTest{
	// Empty file isn't allowed (need at least a package).
	{
		"FAILEmptyFile",
		"",
		nil,
		[]string{"file must start with package statement"}},

	// Comment tests.
	{
		"PackageDocOneLiner",
		`// One liner
// Another line
package testpkg`,
		&parse.File{BaseName: "testfile", PackageDef: parse.NamePos{Name: "testpkg", Pos: pos(3, 9), Doc: `// One liner
// Another line
`}},
		nil},
	{
		"PackageDocMultiLiner",
		`/* Multi liner
Another line
*/
package testpkg`,
		&parse.File{BaseName: "testfile", PackageDef: parse.NamePos{Name: "testpkg", Pos: pos(4, 9), Doc: `/* Multi liner
Another line
*/
`}},
		nil},
	{
		"NotPackageDoc",
		`// Extra newline, not package doc

package testpkg`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 3, 9)},
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
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9)},
		nil},
	{
		"PackageNoSemi",
		"package testpkg",
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9)},
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
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9)},
		nil},
	{
		"OneImport",
		`package testpkg;
import "foo/bar";`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Imports: []*parse.Import{{Path: "foo/bar", NamePos: np("", 2, 8)}}},
		nil},
	{
		"OneImportLocalNameNoSemi",
		`package testpkg
import baz "foo/bar"`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Imports: []*parse.Import{{Path: "foo/bar", NamePos: np("baz", 2, 8)}}},
		nil},
	{
		"OneImportParens",
		`package testpkg
import (
  "foo/bar";
)`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Imports: []*parse.Import{{Path: "foo/bar", NamePos: np("", 3, 3)}}},
		nil},
	{
		"OneImportParensNoSemi",
		`package testpkg
import (
  "foo/bar"
)`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Imports: []*parse.Import{{Path: "foo/bar", NamePos: np("", 3, 3)}}},
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
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Imports: []*parse.Import{
				{Path: "foo/bar", NamePos: np("", 2, 8)},
				{Path: "baz", NamePos: np("", 4, 3)},
				{Path: "a/b", NamePos: np("", 4, 9)},
				{Path: "c/d", NamePos: np("", 5, 3)},
				{Path: "z", NamePos: np("", 7, 8)}}},
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
		"TypeNamed",
		`package testpkg
type foo bar`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: tn("bar", 2, 10)}}},
		nil},
	{
		"TypeNamedQualified",
		`package testpkg
type foo bar.baz`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: tn("bar.baz", 2, 10)}}},
		nil},
	{
		"TypeArray",
		`package testpkg
type foo [2]bar`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeArray{
					Len: 2, Elem: tn("bar", 2, 13), P: pos(2, 10)}}}},
		nil},
	{
		"TypeList",
		`package testpkg
type foo []bar`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeList{
					Elem: tn("bar", 2, 12), P: pos(2, 10)}}}},
		nil},
	{
		"TypeMap",
		`package testpkg
type foo map[bar]baz`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeMap{
					Key: tn("bar", 2, 14), Elem: tn("baz", 2, 18), P: pos(2, 10)}}}},
		nil},
	{
		"TypeStructOneField",
		`package testpkg
type foo struct{a b;}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeStruct{
					Fields: []*parse.Field{{NamePos: np("a", 2, 17), Type: tn("b", 2, 19)}},
					P:      pos(2, 10)}}}},
		nil},
	{
		"TypeStructOneFieldNoSemi",
		`package testpkg
type foo struct{a b}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeStruct{
					Fields: []*parse.Field{{NamePos: np("a", 2, 17), Type: tn("b", 2, 19)}},
					P:      pos(2, 10)}}}},
		nil},
	{
		"TypeStructOneFieldNewline",
		`package testpkg
type foo struct{
  a b;
}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeStruct{
					Fields: []*parse.Field{{NamePos: np("a", 3, 3), Type: tn("b", 3, 5)}},
					P:      pos(2, 10)}}}},
		nil},
	{
		"TypeStructOneFieldNewlineNoSemi",
		`package testpkg
type foo struct{
  a b
}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeStruct{
					Fields: []*parse.Field{{NamePos: np("a", 3, 3), Type: tn("b", 3, 5)}},
					P:      pos(2, 10)}}}},
		nil},
	{
		"TypeStructOneFieldList",
		`package testpkg
type foo struct{a,b,c d}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeStruct{
					Fields: []*parse.Field{
						{NamePos: np("a", 2, 17), Type: tn("d", 2, 23)},
						{NamePos: np("b", 2, 19), Type: tn("d", 2, 23)},
						{NamePos: np("c", 2, 21), Type: tn("d", 2, 23)}},
					P: pos(2, 10)}}}},
		nil},
	{
		"TypeStructMixed",
		`package testpkg
type foo struct{
  a b;c,d e
  f,g h
}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeStruct{
					Fields: []*parse.Field{
						{NamePos: np("a", 3, 3), Type: tn("b", 3, 5)},
						{NamePos: np("c", 3, 7), Type: tn("e", 3, 11)},
						{NamePos: np("d", 3, 9), Type: tn("e", 3, 11)},
						{NamePos: np("f", 4, 3), Type: tn("h", 4, 7)},
						{NamePos: np("g", 4, 5), Type: tn("h", 4, 7)}},
					P: pos(2, 10)}}}},
		nil},
	{
		"FAILTypeStructNotClosed",
		`package testpkg
type foo struct{
  a b`,
		nil,
		[]string{"testfile:3:6 syntax error"}},
	{
		"FAILTypeStructUnnamedField",
		`package testpkg
type foo struct{a}`,
		nil,
		[]string{"testfile:2:18 syntax error"}},
	{
		"FAILTypeStructUnnamedFieldList",
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
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: &parse.ConstLit{true, pos(2, 13)}},
				{NamePos: np("bar", 3, 7), Expr: &parse.ConstLit{false, pos(3, 13)}}}},
		nil},
	{
		"StringConst",
		"package testpkg\nconst foo = \"abc\"\nconst bar = `def`",
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: &parse.ConstLit{"abc", pos(2, 13)}},
				{NamePos: np("bar", 3, 7), Expr: &parse.ConstLit{"def", pos(3, 13)}}}},
		nil},
	{
		"IntegerConst",
		`package testpkg
const foo = 123`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: &parse.ConstLit{big.NewInt(123), pos(2, 13)}}}},
		nil},
	{
		"FloatConst",
		`package testpkg
const foo = 1.5`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: &parse.ConstLit{big.NewRat(3, 2), pos(2, 13)}}}},
		nil},
	{
		"NamedConst",
		`package testpkg
const foo = baz
const bar = pkg.box`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: &parse.ConstNamed{"baz", pos(2, 13)}},
				{NamePos: np("bar", 3, 7), Expr: &parse.ConstNamed{"pkg.box", pos(3, 13)}}}},
		nil},
	{
		"UnaryOpConst",
		`package testpkg
const foo = !false
const bar = +1
const baz = -2
const box = ^3`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: &parse.ConstUnaryOp{"!",
					&parse.ConstLit{false, pos(2, 14)}, pos(2, 13)}},
				{NamePos: np("bar", 3, 7), Expr: &parse.ConstUnaryOp{"+",
					&parse.ConstLit{big.NewInt(1), pos(3, 14)}, pos(3, 13)}},
				{NamePos: np("baz", 4, 7), Expr: &parse.ConstUnaryOp{"-",
					&parse.ConstLit{big.NewInt(2), pos(4, 14)}, pos(4, 13)}},
				{NamePos: np("box", 5, 7), Expr: &parse.ConstUnaryOp{"^",
					&parse.ConstLit{big.NewInt(3), pos(5, 14)}, pos(5, 13)}}}},
		nil},
	{
		"TypeConvConst",
		`package testpkg
const foo = baz(true)
const bar = pkg.box(false)`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: &parse.ConstTypeConv{tn("baz", 2, 13),
					&parse.ConstLit{true, pos(2, 17)}, pos(2, 13)}},
				{NamePos: np("bar", 3, 7), Expr: &parse.ConstTypeConv{tn("pkg.box", 3, 13),
					&parse.ConstLit{false, pos(3, 21)}, pos(3, 13)}}}},
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
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("a", 2, 7),
					Expr: &parse.ConstBinaryOp{"||", &parse.ConstLit{true, pos(2, 11)},
						&parse.ConstLit{false, pos(2, 19)}, pos(2, 16)}},
				{NamePos: np("b", 3, 7),
					Expr: &parse.ConstBinaryOp{"&&", &parse.ConstLit{true, pos(3, 11)},
						&parse.ConstLit{false, pos(3, 19)}, pos(3, 16)}},
				{NamePos: np("c", 4, 7),
					Expr: &parse.ConstBinaryOp{"<", &parse.ConstLit{big.NewInt(1), pos(4, 11)},
						&parse.ConstLit{big.NewInt(2), pos(4, 15)}, pos(4, 13)}},
				{NamePos: np("d", 5, 7),
					Expr: &parse.ConstBinaryOp{">", &parse.ConstLit{big.NewInt(3), pos(5, 11)},
						&parse.ConstLit{big.NewInt(4), pos(5, 15)}, pos(5, 13)}},
				{NamePos: np("e", 6, 7),
					Expr: &parse.ConstBinaryOp{"<=", &parse.ConstLit{big.NewInt(5), pos(6, 11)},
						&parse.ConstLit{big.NewInt(6), pos(6, 16)}, pos(6, 13)}},
				{NamePos: np("f", 7, 7),
					Expr: &parse.ConstBinaryOp{">=", &parse.ConstLit{big.NewInt(7), pos(7, 11)},
						&parse.ConstLit{big.NewInt(8), pos(7, 16)}, pos(7, 13)}},
				{NamePos: np("g", 8, 7),
					Expr: &parse.ConstBinaryOp{"!=", &parse.ConstLit{big.NewInt(9), pos(8, 11)},
						&parse.ConstLit{big.NewInt(8), pos(8, 16)}, pos(8, 13)}},
				{NamePos: np("h", 9, 7),
					Expr: &parse.ConstBinaryOp{"==", &parse.ConstLit{big.NewInt(7), pos(9, 11)},
						&parse.ConstLit{big.NewInt(6), pos(9, 16)}, pos(9, 13)}},
				{NamePos: np("i", 10, 7),
					Expr: &parse.ConstBinaryOp{"+", &parse.ConstLit{big.NewInt(5), pos(10, 11)},
						&parse.ConstLit{big.NewInt(4), pos(10, 15)}, pos(10, 13)}},
				{NamePos: np("j", 11, 7),
					Expr: &parse.ConstBinaryOp{"-", &parse.ConstLit{big.NewInt(3), pos(11, 11)},
						&parse.ConstLit{big.NewInt(2), pos(11, 15)}, pos(11, 13)}},
				{NamePos: np("k", 12, 7),
					Expr: &parse.ConstBinaryOp{"*", &parse.ConstLit{big.NewInt(1), pos(12, 11)},
						&parse.ConstLit{big.NewInt(2), pos(12, 15)}, pos(12, 13)}},
				{NamePos: np("l", 13, 7),
					Expr: &parse.ConstBinaryOp{"/", &parse.ConstLit{big.NewInt(3), pos(13, 11)},
						&parse.ConstLit{big.NewInt(4), pos(13, 15)}, pos(13, 13)}},
				{NamePos: np("m", 14, 7),
					Expr: &parse.ConstBinaryOp{"%", &parse.ConstLit{big.NewInt(5), pos(14, 11)},
						&parse.ConstLit{big.NewInt(6), pos(14, 15)}, pos(14, 13)}},
				{NamePos: np("n", 15, 7),
					Expr: &parse.ConstBinaryOp{"|", &parse.ConstLit{big.NewInt(7), pos(15, 11)},
						&parse.ConstLit{big.NewInt(8), pos(15, 15)}, pos(15, 13)}},
				{NamePos: np("o", 16, 7),
					Expr: &parse.ConstBinaryOp{"&", &parse.ConstLit{big.NewInt(9), pos(16, 11)},
						&parse.ConstLit{big.NewInt(8), pos(16, 15)}, pos(16, 13)}},
				{NamePos: np("p", 17, 7),
					Expr: &parse.ConstBinaryOp{"^", &parse.ConstLit{big.NewInt(7), pos(17, 11)},
						&parse.ConstLit{big.NewInt(6), pos(17, 15)}, pos(17, 13)}},
				{NamePos: np("q", 18, 7),
					Expr: &parse.ConstBinaryOp{"<<", &parse.ConstLit{big.NewInt(5), pos(18, 11)},
						&parse.ConstLit{big.NewInt(4), pos(18, 16)}, pos(18, 13)}},
				{NamePos: np("r", 19, 7),
					Expr: &parse.ConstBinaryOp{">>", &parse.ConstLit{big.NewInt(3), pos(19, 11)},
						&parse.ConstLit{big.NewInt(2), pos(19, 16)}, pos(19, 13)}}}},
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
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9)},
		nil},
	{
		"ErrorID",
		`package testpkg
errorid ErrIDFoo`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ErrorIDs: []*parse.ErrorID{{NamePos: np("ErrIDFoo", 2, 9)}}},
		nil},
	{
		"ErrorIDOverwrite",
		`package testpkg
errorid ErrIDFoo = "pkg/path.ErrName"`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ErrorIDs: []*parse.ErrorID{
				{NamePos: np("ErrIDFoo", 2, 9), ID: "pkg/path.ErrName"}}},
		nil},
	{
		"ErrorMixed",
		`package testpkg
errorid ErrIDFoo
errorid (
  ErrIDBar
  ErrIDBaz = "pkg/path.ErrIDBaz"
)`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ErrorIDs: []*parse.ErrorID{
				{NamePos: np("ErrIDFoo", 2, 9)},
				{NamePos: np("ErrIDBar", 4, 3)},
				{NamePos: np("ErrIDBaz", 5, 3), ID: "pkg/path.ErrIDBaz"}}},
		nil},
	{
		"FAILErrorEmptyID",
		`package testpkg
errorid ErrIDFoo = ""`,
		nil,
		[]string{"testfile:2:20 error id must be non-empty if specified"}},

	// Interface tests.
	{
		"InterfaceEmpty",
		`package testpkg
type foo interface{}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Interfaces: []*parse.Interface{{NamePos: np("foo", 2, 6)}}},
		nil},
	{
		"InterfaceOneMethodNoArgs",
		`package testpkg
type foo interface{meth1()}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Interfaces: []*parse.Interface{{NamePos: np("foo", 2, 6),
				Methods: []*parse.Method{{NamePos: np("meth1", 2, 20)}}}}},
		nil},
	{
		"InterfaceOneMethodOneInUnnamedOut",
		`package testpkg
type foo interface{meth1(a b) c}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Interfaces: []*parse.Interface{{NamePos: np("foo", 2, 6),
				Methods: []*parse.Method{{NamePos: np("meth1", 2, 20),
					InArgs:  []*parse.Field{{NamePos: np("a", 2, 26), Type: tn("b", 2, 28)}},
					OutArgs: []*parse.Field{{NamePos: np("", 2, 31), Type: tn("c", 2, 31)}}}}}}},
		nil},
	{
		"InterfaceOneMethodOneInNamedOut",
		`package testpkg
type foo interface{meth1(a b) (c d)}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Interfaces: []*parse.Interface{{NamePos: np("foo", 2, 6),
				Methods: []*parse.Method{{NamePos: np("meth1", 2, 20),
					InArgs:  []*parse.Field{{NamePos: np("a", 2, 26), Type: tn("b", 2, 28)}},
					OutArgs: []*parse.Field{{NamePos: np("c", 2, 32), Type: tn("d", 2, 34)}}}}}}},
		nil},
	{
		"InterfaceErrors",
		`package testpkg
type foo interface{meth1(err error) error}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Interfaces: []*parse.Interface{{NamePos: np("foo", 2, 6),
				Methods: []*parse.Method{{NamePos: np("meth1", 2, 20),
					InArgs:  []*parse.Field{{NamePos: np("err", 2, 26), Type: tn("error", 2, 30)}},
					OutArgs: []*parse.Field{{NamePos: np("", 2, 37), Type: tn("error", 2, 37)}}}}}}},
		nil},
	{
		"InterfaceMixedMethods",
		`package testpkg
type foo interface{
  meth1(a b) (c d);meth2()
  meth3(e f, g, h i) (j k, l, m n)
}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Interfaces: []*parse.Interface{{NamePos: np("foo", 2, 6),
				Methods: []*parse.Method{
					{NamePos: np("meth1", 3, 3),
						InArgs:  []*parse.Field{{NamePos: np("a", 3, 9), Type: tn("b", 3, 11)}},
						OutArgs: []*parse.Field{{NamePos: np("c", 3, 15), Type: tn("d", 3, 17)}}},
					{NamePos: np("meth2", 3, 20)},
					{NamePos: np("meth3", 4, 3),
						InArgs: []*parse.Field{
							{NamePos: np("e", 4, 9), Type: tn("f", 4, 11)},
							{NamePos: np("g", 4, 14), Type: tn("i", 4, 19)},
							{NamePos: np("h", 4, 17), Type: tn("i", 4, 19)}},
						OutArgs: []*parse.Field{
							{NamePos: np("j", 4, 23), Type: tn("k", 4, 25)},
							{NamePos: np("l", 4, 28), Type: tn("n", 4, 33)},
							{NamePos: np("m", 4, 31), Type: tn("n", 4, 33)}}}}}}},
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
		[]string{"expected one or more variable names",
			"testfile:3:18 perhaps you forgot a comma"}},
}

func testParse(t *testing.T, test parseTest, opts parse.Opts) {
	errs := idl2.NewErrors(100)
	actual := parse.Parse("testfile", strings.NewReader(test.src), opts, errs)
	idltest.ExpectResult(t, errs, test.name, test.errors...)
	if !reflect.DeepEqual(test.expect, actual) {
		t.Errorf("%v\nEXPECT %+v\nACTUAL %+v", test.name, test.expect, actual)
	}
}

func TestImportsOnlyParser(t *testing.T) {
	for _, test := range importTests {
		testParse(t, test, parse.Opts{ImportsOnly: true})
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
			testParse(t, test, parse.Opts{ImportsOnly: true})
		}
	}
}

func TestFullParser(t *testing.T) {
	for _, test := range append(importTests, fullTests...) {
		testParse(t, test, parse.Opts{ImportsOnly: false})
	}
}
