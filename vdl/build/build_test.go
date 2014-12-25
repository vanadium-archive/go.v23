package build_test

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vdl/build"
	"veyron.io/veyron/veyron2/vdl/compile"
	"veyron.io/veyron/veyron2/vdl/testdata/base"
	"veyron.io/veyron/veyron2/vdl/valconv"
	"veyron.io/veyron/veyron2/vdl/vdlroot/src/vdltool"
	"veyron.io/veyron/veyron2/vdl/vdltest"
	"veyron.io/veyron/veyron2/vdl/vdlutil"
)

func init() {
	// Uncomment this to enable verbose logs for debugging.
	//vdlutil.SetVerbose()
}

// The cwd is set to the directory containing this file.  Currently we have the
// following directory structure:
//   .../veyron/go/src/veyron.io/veyron/veyron2/vdl/build/build_test.go
// We want to end up with the following:
//   VDLROOT = .../veyron/go/src/veyron.io/veyron/veyron2/vdl/vdlroot
//   VDLPATH = .../veyron/go
//
// TODO(toddw): Put a full VDLPATH tree under ../testdata and only use that.
const (
	defaultVDLRoot = "../vdlroot"
	defaultVDLPath = "../../../../../.."
)

func setEnvironment(t *testing.T, vdlroot, vdlpath string) bool {
	errRoot := os.Setenv("VDLROOT", vdlroot)
	errPath := os.Setenv("VDLPATH", vdlpath)
	if errRoot != nil {
		t.Errorf("Setenv(VDLROOT, %q) failed: %v", vdlroot, errRoot)
	}
	if errPath != nil {
		t.Errorf("Setenv(VDLPATH, %q) failed: %v", vdlpath, errPath)
	}
	return errRoot == nil && errPath == nil
}

func setVanadiumRoot(t *testing.T, root string) bool {
	if err := os.Setenv("VANADIUM_ROOT", root); err != nil {
		t.Errorf("Setenv(VANADIUM_ROOT, %q) failed: %v", root, err)
		return false
	}
	return true
}

// Tests the VDLROOT part of SrcDirs().
func TestSrcDirsVdlRoot(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() failed: %v", err)
	}
	abs := func(relative string) string {
		return filepath.Join(cwd, relative)
	}
	tests := []struct {
		VDLRoot    string
		VanadiumRoot string
		Want       []string
		ErrRE      string
	}{
		{"", "", nil, "Either VDLROOT or VANADIUM_ROOT must be set"},
		{"/a", "", []string{"/a/src"}, ""},
		{"/a/b/c", "", []string{"/a/b/c/src"}, ""},
		{"", "/veyron", []string{"/veyron/veyron/go/src/veyron.io/veyron/veyron2/vdl/vdlroot/src"}, ""},
		{"", "/a/b/c", []string{"/a/b/c/veyron/go/src/veyron.io/veyron/veyron2/vdl/vdlroot/src"}, ""},
		// If both VDLROOT and VANADIUM_ROOT are specified, VDLROOT takes precedence.
		{"/a", "/veyron", []string{"/a/src"}, ""},
		{"/a/b/c", "/x/y/z", []string{"/a/b/c/src"}, ""},
	}
	for _, test := range tests {
		if !setEnvironment(t, test.VDLRoot, defaultVDLPath) || !setVanadiumRoot(t, test.VanadiumRoot) {
			continue
		}
		name := fmt.Sprintf("%+v", test)
		errs := vdlutil.NewErrors(-1)
		got := build.SrcDirs(errs)
		vdltest.ExpectResult(t, errs, name, test.ErrRE)
		// Every result will have our valid VDLPATH srcdir.
		vdlpathsrc := filepath.Join(abs(defaultVDLPath), "src")
		want := append(test.Want, vdlpathsrc)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("SrcDirs(%s) got %v, want %v", name, got, want)
		}
	}
}

// Tests the VDLPATH part of SrcDirs().
func TestSrcDirsVdlPath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() failed: %v", err)
	}
	abs := func(relative string) string {
		return filepath.Join(cwd, relative)
	}
	tests := []struct {
		VDLPath string
		Want    []string
	}{
		{"", nil},
		// Test absolute paths.
		{"/a", []string{"/a/src"}},
		{"/a/b", []string{"/a/b/src"}},
		{"/a:/b", []string{"/a/src", "/b/src"}},
		{"/a/1:/b/2", []string{"/a/1/src", "/b/2/src"}},
		{"/a/1:/b/2:/c/3", []string{"/a/1/src", "/b/2/src", "/c/3/src"}},
		{":::/a/1::::/b/2::::/c/3:::", []string{"/a/1/src", "/b/2/src", "/c/3/src"}},
		// Test relative paths.
		{"a", []string{abs("a/src")}},
		{"a/b", []string{abs("a/b/src")}},
		{"a:b", []string{abs("a/src"), abs("b/src")}},
		{"a/1:b/2", []string{abs("a/1/src"), abs("b/2/src")}},
		{"a/1:b/2:c/3", []string{abs("a/1/src"), abs("b/2/src"), abs("c/3/src")}},
		{":::a/1::::b/2::::c/3:::", []string{abs("a/1/src"), abs("b/2/src"), abs("c/3/src")}},
		// Test mixed absolute / relative paths.
		{"a:/b", []string{abs("a/src"), "/b/src"}},
		{"/a/1:b/2", []string{"/a/1/src", abs("b/2/src")}},
		{"/a/1:b/2:/c/3", []string{"/a/1/src", abs("b/2/src"), "/c/3/src"}},
		{":::/a/1::::b/2::::/c/3:::", []string{"/a/1/src", abs("b/2/src"), "/c/3/src"}},
	}
	for _, test := range tests {
		if !setEnvironment(t, defaultVDLRoot, test.VDLPath) {
			continue
		}
		name := fmt.Sprintf("SrcDirs(%q)", test.VDLPath)
		errs := vdlutil.NewErrors(-1)
		got := build.SrcDirs(errs)
		var errRE string
		if test.Want == nil {
			errRE = "No src dirs; set VDLPATH to a valid value"
		}
		vdltest.ExpectResult(t, errs, name, errRE)
		// Every result will have our valid VDLROOT srcdir.
		vdlrootsrc := filepath.Join(abs(defaultVDLRoot), "src")
		want := append([]string{vdlrootsrc}, test.Want...)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s got %v, want %v", name, got, want)
		}
	}
}

// Tests Is{Dir,Import}Path.
func TestIsDirImportPath(t *testing.T) {
	tests := []struct {
		Path  string
		IsDir bool
	}{
		// Import paths.
		{"", false},
		{"...", false},
		{".../", false},
		{"all", false},
		{"foo", false},
		{"foo/", false},
		{"foo...", false},
		{"foo/...", false},
		{"a/b/c", false},
		{"a/b/c/", false},
		{"a/b/c...", false},
		{"a/b/c/...", false},
		{"...a/b/c...", false},
		{"...a/b/c/...", false},
		{".../a/b/c/...", false},
		{".../a/b/c...", false},
		// Dir paths.
		{".", true},
		{"..", true},
		{"./", true},
		{"../", true},
		{"./...", true},
		{"../...", true},
		{".././.././...", true},
		{"/", true},
		{"/.", true},
		{"/..", true},
		{"/...", true},
		{"/./...", true},
		{"/foo", true},
		{"/foo/", true},
		{"/foo...", true},
		{"/foo/...", true},
		{"/a/b/c", true},
		{"/a/b/c/", true},
		{"/a/b/c...", true},
		{"/a/b/c/...", true},
		{"/a/b/c/../../...", true},
	}
	for _, test := range tests {
		if got, want := build.IsDirPath(test.Path), test.IsDir; got != want {
			t.Errorf("IsDirPath(%q) want %v", want)
		}
		if got, want := build.IsImportPath(test.Path), !test.IsDir; got != want {
			t.Errorf("IsImportPath(%q) want %v", want)
		}
	}
}

var allModes = []build.UnknownPathMode{
	build.UnknownPathIsIgnored,
	build.UnknownPathIsError,
}

// Tests TransitivePackages success cases.
func TestTransitivePackages(t *testing.T) {
	if !setEnvironment(t, defaultVDLRoot, defaultVDLPath) {
		t.Fatalf("Couldn't setEnvironment")
	}
	tests := []struct {
		InPaths, OutPaths []string
	}{
		{nil, nil},
		{[]string{}, nil},
		// Single-package, both import and dir path.
		{
			[]string{"veyron.io/veyron/veyron2/vdl/testdata/base"},
			[]string{"veyron.io/veyron/veyron2/vdl/testdata/base"},
		},
		{
			[]string{"../testdata/base"},
			[]string{"veyron.io/veyron/veyron2/vdl/testdata/base"},
		},
		// Single-package with wildcard, both import and dir path.
		{
			[]string{"veyron.io/veyron/veyron2/vdl/testdata/base..."},
			[]string{"veyron.io/veyron/veyron2/vdl/testdata/base"},
		},
		{
			[]string{"veyron.io/veyron/veyron2/vdl/testdata/base/..."},
			[]string{"veyron.io/veyron/veyron2/vdl/testdata/base"},
		},
		{
			[]string{"../testdata/base..."},
			[]string{"veyron.io/veyron/veyron2/vdl/testdata/base"},
		},
		{
			[]string{"../testdata/base/..."},
			[]string{"veyron.io/veyron/veyron2/vdl/testdata/base"},
		},
		// Redundant specification as both import and dir path.
		{
			[]string{"veyron.io/veyron/veyron2/vdl/testdata/base", "../testdata/base"},
			[]string{"veyron.io/veyron/veyron2/vdl/testdata/base"},
		},
		{
			[]string{"veyron.io/veyron/veyron2/vdl/testdata/arith", "../testdata/arith"},
			[]string{
				"veyron.io/veyron/veyron2/vdl/testdata/arith/exp",
				"veyron.io/veyron/veyron2/vdl/testdata/base",
				"veyron.io/veyron/veyron2/vdl/testdata/arith",
			},
		},
		// Wildcards as both import and dir path.
		{
			[]string{"veyron.io/veyron/veyron2/vdl/testdata..."},
			[]string{
				"veyron.io/veyron/veyron2/vdl/testdata/arith/exp",
				"veyron.io/veyron/veyron2/vdl/testdata/base",
				"veyron.io/veyron/veyron2/vdl/testdata/arith",
				"veyron.io/veyron/veyron2/vdl/testdata/testconfig",
			},
		},
		{
			[]string{"veyron.io/veyron/veyron2/vdl/testdata/..."},
			[]string{
				"veyron.io/veyron/veyron2/vdl/testdata/arith/exp",
				"veyron.io/veyron/veyron2/vdl/testdata/base",
				"veyron.io/veyron/veyron2/vdl/testdata/arith",
				"veyron.io/veyron/veyron2/vdl/testdata/testconfig",
			},
		},
		{
			[]string{"../testdata..."},
			[]string{
				"veyron.io/veyron/veyron2/vdl/testdata/arith/exp",
				"veyron.io/veyron/veyron2/vdl/testdata/base",
				"veyron.io/veyron/veyron2/vdl/testdata/arith",
				"veyron.io/veyron/veyron2/vdl/testdata/testconfig",
			},
		},
		{
			[]string{"../testdata/..."},
			[]string{
				"veyron.io/veyron/veyron2/vdl/testdata/arith/exp",
				"veyron.io/veyron/veyron2/vdl/testdata/base",
				"veyron.io/veyron/veyron2/vdl/testdata/arith",
				"veyron.io/veyron/veyron2/vdl/testdata/testconfig",
			},
		},
		// Multi-Wildcards as both import and dir path.
		{
			[]string{"v...vdl/testdata/..."},
			[]string{
				"veyron.io/veyron/veyron2/vdl/testdata/arith/exp",
				"veyron.io/veyron/veyron2/vdl/testdata/base",
				"veyron.io/veyron/veyron2/vdl/testdata/arith",
				"veyron.io/veyron/veyron2/vdl/testdata/testconfig",
			},
		},
		{
			[]string{"../../../...vdl/testdata/..."},
			[]string{
				"veyron.io/veyron/veyron2/vdl/testdata/arith/exp",
				"veyron.io/veyron/veyron2/vdl/testdata/base",
				"veyron.io/veyron/veyron2/vdl/testdata/arith",
				"veyron.io/veyron/veyron2/vdl/testdata/testconfig",
			},
		},
		// Multi-Wildcards as both import and dir path.
		{
			[]string{"v...vdl/testdata/...exp"},
			[]string{"veyron.io/veyron/veyron2/vdl/testdata/arith/exp"},
		},
		{
			[]string{"../../../...vdl/testdata/...exp"},
			[]string{"veyron.io/veyron/veyron2/vdl/testdata/arith/exp"},
		},
		// Standard vdl package, as both import and dir path.
		{
			[]string{"veyron.io/veyron/veyron2/vdl/vdlroot/src/vdltool"},
			[]string{"vdltool"},
		},
		{
			[]string{"../vdlroot/src/vdltool"},
			[]string{"vdltool"},
		},
	}
	for _, test := range tests {
		// All modes should result in the same successful output.
		for _, mode := range allModes {
			name := fmt.Sprintf("%v %v", mode, test.InPaths)
			errs := vdlutil.NewErrors(-1)
			pkgs := build.TransitivePackages(test.InPaths, mode, build.Opts{}, errs)
			vdltest.ExpectResult(t, errs, name, "")
			var got []string
			for _, pkg := range pkgs {
				got = append(got, pkg.Path)
			}
			if want := []string(test.OutPaths); !reflect.DeepEqual(got, want) {
				t.Errorf("%v got %v, want %v", name, got, want)
			}
		}
	}
}

// Tests TransitivePackages error cases.
func TestTransitivePackagesUnknownPathError(t *testing.T) {
	if !setEnvironment(t, defaultVDLRoot, defaultVDLPath) {
		t.Fatalf("Couldn't setEnvironment")
	}
	tests := []struct {
		InPaths []string
		ErrRE   string
	}{
		// Non-existent as both import and dir path.
		{
			[]string{"noexist"},
			`Can't resolve "noexist" to any packages`,
		},
		{
			[]string{"./noexist"},
			`Can't resolve "./noexist" to any packages`,
		},
		// Invalid package path, as both import and dir path.
		{
			[]string{".foo"},
			`Import path ".foo" is invalid`,
		},
		{
			[]string{"foo/.bar"},
			`Import path "foo/.bar" is invalid`,
		},
		{
			[]string{"_foo"},
			`Import path "_foo" is invalid`,
		},
		{
			[]string{"foo/_bar"},
			`Import path "foo/_bar" is invalid`,
		},
		{
			[]string{"../../../../../.foo"},
			`package path ".foo" is invalid`,
		},
		{
			[]string{"../../../../../foo/.bar"},
			`package path "foo/.bar" is invalid`,
		},
		{
			[]string{"../../../../../_foo"},
			`package path "_foo" is invalid`,
		},
		{
			[]string{"../../../../../foo/_bar"},
			`package path "foo/_bar" is invalid`,
		},
	}
	for _, test := range tests {
		for _, mode := range allModes {
			name := fmt.Sprintf("%v %v", mode, test.InPaths)
			errs := vdlutil.NewErrors(-1)
			pkgs := build.TransitivePackages(test.InPaths, mode, build.Opts{}, errs)
			errRE := test.ErrRE
			if mode == build.UnknownPathIsIgnored {
				// Ignore mode returns success, while error mode returns error.
				errRE = ""
			}
			vdltest.ExpectResult(t, errs, name, errRE)
			if pkgs != nil {
				t.Errorf("%v got unexpected packages %v", name, pkgs)
			}
		}
	}
}

// Tests vdl.config file support.
func TestPackageConfig(t *testing.T) {
	if !setEnvironment(t, defaultVDLRoot, defaultVDLPath) {
		t.Fatalf("Couldn't setEnvironment")
	}
	tests := []struct {
		Path   string
		Config vdltool.Config
	}{
		{"veyron.io/veyron/veyron2/vdl/testdata/base", vdltool.Config{}},
		{
			"veyron.io/veyron/veyron2/vdl/testdata/testconfig",
			vdltool.Config{
				GenLanguages: map[vdltool.GenLanguage]struct{}{vdltool.GenLanguageGo: struct{}{}},
			},
		},
	}
	for _, test := range tests {
		name := path.Base(test.Path)
		env := compile.NewEnv(-1)
		deps := build.TransitivePackages([]string{test.Path}, build.UnknownPathIsError, build.Opts{}, env.Errors)
		vdltest.ExpectResult(t, env.Errors, name, "")
		if len(deps) != 1 {
			t.Fatalf("TransitivePackages(%q) got %v, want 1 dep", name, deps)
		}
		if got, want := deps[0].Name, name; got != want {
			t.Errorf("TransitivePackages(%q) got Name %q, want %q", name, got, want)
		}
		if got, want := deps[0].Path, test.Path; got != want {
			t.Errorf("TransitivePackages(%q) got Path %q, want %q", name, got, want)
		}
		if got, want := deps[0].Config, test.Config; !reflect.DeepEqual(got, want) {
			t.Errorf("TransitivePackages(%q) got Config %+v, want %+v", name, got, want)
		}
	}
}

// Tests BuildConfig, BuildConfigValue and TransitivePackagesForConfig.
func TestBuildConfig(t *testing.T) {
	if !setEnvironment(t, defaultVDLRoot, defaultVDLPath) {
		t.Fatalf("Couldn't setEnvironment")
	}
	tests := []struct {
		Src   string
		Value interface{}
	}{
		{
			`config = x;import "veyron.io/veyron/veyron2/vdl/testdata/base";const x = base.NamedBool(true)`,
			base.NamedBool(true),
		},
		{
			`config = x;import "veyron.io/veyron/veyron2/vdl/testdata/base";const x = base.NamedString("abc")`,
			base.NamedString("abc"),
		},
		{
			`config = x;import "veyron.io/veyron/veyron2/vdl/testdata/base";const x = base.Args{1, 2}`,
			base.Args{1, 2},
		},
	}
	for _, test := range tests {
		// Build import package dependencies.
		env := compile.NewEnv(-1)
		deps := build.TransitivePackagesForConfig("file", strings.NewReader(test.Src), build.Opts{}, env.Errors)
		for _, dep := range deps {
			build.BuildPackage(dep, env)
		}
		vdltest.ExpectResult(t, env.Errors, test.Src, "")
		// Test BuildConfig
		wantV := vdl.ZeroValue(vdl.TypeOf(test.Value))
		if err := valconv.Convert(wantV, test.Value); err != nil {
			t.Errorf("Convert(%v) got error %v, want nil", test.Value, err)
		}
		gotV := build.BuildConfig("file", strings.NewReader(test.Src), nil, env)
		if !vdl.EqualValue(gotV, wantV) {
			t.Errorf("BuildConfig(%v) got %v, want %v", test.Src, gotV, wantV)
		}
		vdltest.ExpectResult(t, env.Errors, test.Src, "")
		// TestBuildConfigValue
		gotRV := reflect.New(reflect.TypeOf(test.Value))
		build.BuildConfigValue("file", strings.NewReader(test.Src), env, gotRV.Interface())
		if got, want := gotRV.Elem().Interface(), test.Value; !reflect.DeepEqual(got, want) {
			t.Errorf("BuildConfigValue(%v) got %v, want %v", test.Src, got, want)
		}
		vdltest.ExpectResult(t, env.Errors, test.Src, "")
	}
}
