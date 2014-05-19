package compile_test

import (
	"fmt"
	"path"
	"strings"
	"testing"

	"veyron2/vdl/build"
	"veyron2/vdl/compile"
	"veyron2/vdl/vdltest"
)

func TestValidIdent(t *testing.T) {
	const errInvalidID = "invalid identifier"
	tests := []struct {
		ident  string
		errstr string
	}{
		{"", errInvalidID},
		{"xFirstLetterLower", errInvalidID},
		{"0FirstLetterDigit", errInvalidID},
		{"_FirstLetterPunct", errInvalidID},
		{" FirstLetterSpace", errInvalidID},
		{"X.InvalidPunct", errInvalidID},
		{"X InvalidSpace", errInvalidID},
		{"X\nNonAlphaNum", errInvalidID},
		{"X", ""},
		{"XYZ", ""},
		{"Xyz", ""},
		{"Xyz123", ""},
		{"Xyz_123", ""},
	}
	for _, test := range tests {
		err := compile.ValidIdent(test.ident)
		errstr := fmt.Sprint(err)
		if test.errstr != "" && !strings.Contains(errstr, test.errstr) {
			t.Errorf(`ValidIdent(%s) got error %q, want substr %q`, test.ident, errstr, test.errstr)
		}
		if test.errstr == "" && err != nil {
			t.Errorf(`ValidIdent(%s) got error %q, want nil`, test.ident, errstr)
		}
	}
}

func TestValidArgName(t *testing.T) {
	const errInvalidArgName = "invalid arg name"
	tests := []struct {
		name   string
		errstr string
	}{
		{"", errInvalidArgName},
		{"0FirstLetterDigit", errInvalidArgName},
		{"_FirstLetterPunct", errInvalidArgName},
		{" FirstLetterSpace", errInvalidArgName},
		{"x.InvalidPunct", errInvalidArgName},
		{"x InvalidSpace", errInvalidArgName},
		{"x\nNonAlphaNum", errInvalidArgName},
		{"X", ""},
		{"XYZ", ""},
		{"Xyz", ""},
		{"Xyz123", ""},
		{"Xyz_123", ""},
		{"x", ""},
		{"xYZ", ""},
		{"xyz", ""},
		{"xyz123", ""},
		{"xyz_123", ""},
	}
	for _, test := range tests {
		err := compile.ValidArgName(test.name)
		errstr := fmt.Sprint(err)
		if test.errstr != "" && !strings.Contains(errstr, test.errstr) {
			t.Errorf(`ValidArgName(%s) got error %q, want substr %q`, test.name, errstr, test.errstr)
		}
		if test.errstr == "" && err != nil {
			t.Errorf(`ValidArgName(%s) got error %q, want nil`, test.name, errstr)
		}
	}
}

type f map[string]string

func TestParseAndCompile(t *testing.T) {
	tests := []struct {
		name   string
		files  map[string]string
		errRE  string
		expect func(t *testing.T, name string, pkg *compile.Package)
	}{
		{"test1", f{"1.vdl": pkg1file1, "2.vdl": pkg1file2}, "", expectPkg1},
	}
	for _, test := range tests {
		path := path.Join("a/b", test.name)
		buildPkg := vdltest.FakeBuildPackage(test.name, path, test.files)
		env := compile.NewEnv(-1)
		pkg := build.CompilePackage(buildPkg, env)
		vdltest.ExpectResult(t, env.Errors, test.name, test.errRE)
		if pkg == nil {
			continue
		}
		if got, want := pkg.Name, test.name; got != want {
			t.Errorf("%s got package name %s, want %s", got, want)
		}
		if got, want := pkg.Path, path; got != want {
			t.Errorf("%s got package path %s, want %s", got, want)
		}
		test.expect(t, test.name, pkg)
	}
}

const pkg1file1 = `package test1
errorid (
	ErrIDFoo
	ErrIDBar = "some/path.ErrIdOther"
)

type Scalars struct {
	A bool
	B byte
	C int32
	D int64
	E uint32
	F uint64
	G float32
	H float64
	I complex64
	J complex128
	K string
	L error
	M any
}

type CompComp struct {
	A Composites
	B []Composites
	C map[string]Composites
}

const (
	Cbool = true
	Cbyte = byte(1)
	Cint32 = int32(2)
	Cint64 = int64(3)
	Cuint32 = uint32(4)
	Cuint64 = uint64(5)
	Cfloat32 = float32(6)
	Cfloat64 = float64(7)
	Ccomplex64 = complex64(8+9i)
	Ccomplex128 = complex128(10+11i)
	Cstring = "foo"
  Cany = Cbool

	True = true
	Foo = "foo"
	Five = int32(5)
	SixSquared = Six*Six
)

type ServiceA interface {
	MethodA1() error
	MethodA2(a int32, b string) (s string, err error)
	MethodA3(a int32) stream<_, Scalars> (s string, err error) {"tag", Six}
	MethodA4(a int32) stream<int32, string> error
}
`

const pkg1file2 = `package test1
type Composites struct {
	A Scalars
	B []Scalars
	C map[string]Scalars
	D map[Scalars][]map[string]complex128
}

const (
	FiveSquared = Five*Five
	Six = uint64(6)
)

type ServiceB interface {
	ServiceA
	MethodB1(a Scalars, b Composites) (c CompComp, err error)
}
`

func expectPkg1(t *testing.T, name string, pkg *compile.Package) {
	// TODO(toddw): veryfiy real expectations, and add more tests.
	fmt.Println(pkg)
}
