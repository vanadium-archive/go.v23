package build

import (
	"io"
	"io/ioutil"
	"regexp"
	"strings"
	"testing"
)

func expectPass(t *testing.T, env *Env, testName string) {
	if !env.Errors.IsEmpty() {
		t.Errorf("%v expected no errors but saw: %v", testName, env.Errors.ToError())
		env.Errors.Reset()
	}
}

func expectFail(t *testing.T, env *Env, testName string, re ...string) {
	if env.Errors.IsEmpty() {
		t.Errorf("%v expected errors but didn't see any", testName)
		return
	}
	actual := env.Errors.ToError().Error()
	env.Errors.Reset()
	for index, errRe := range re {
		matched, err := regexp.Match(errRe, []byte(actual))
		if err != nil {
			t.Errorf("%v bad regexp pattern [%v] %q", testName, index, errRe)
			return
		}
		if !matched {
			t.Errorf("%v couldn't match pattern [%v] %q against %q", testName, index, errRe, actual)
		}
	}
}

func expectResult(t *testing.T, env *Env, testName string, re ...string) {
	if len(re) == 0 || len(re) == 1 && re[0] == "" {
		expectPass(t, env, testName)
	} else {
		expectFail(t, env, testName, re...)
	}
}

// newFakeBuildPackage constructs a fake build package for testing, with files
// mapping from file names to file contents.
func newFakeBuildPackage(name, path string, files map[string]string) *BuildPackage {
	var fnames []string
	for fname, _ := range files {
		fnames = append(fnames, fname)
	}
	return &BuildPackage{"fakedir", name, path, fnames, fakeOpenIDLFiles(files)}
}

// fakeOpenIDLFiles returns a function that obeys the OpenIDLFilesFunc
// signature, that simply uses the files map to return readers.
func fakeOpenIDLFiles(files map[string]string) OpenIDLFilesFunc {
	return func(p *BuildPackage) (map[string]io.ReadCloser, error) {
		ret := make(map[string]io.ReadCloser)
		for fname, fdata := range files {
			ret[fname] = ioutil.NopCloser(strings.NewReader(fdata))
		}
		return ret, nil
	}
}
