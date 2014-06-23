package storage

import (
	"runtime"
	"testing"
)

func eqPath(path1, path2 PathName) bool {
	if len(path1) != len(path2) {
		return false
	}
	for i, pl1 := range path1 {
		if pl1 != path2[i] {
			return false
		}
	}
	return true
}

func expectPath(t *testing.T, path1, path2 PathName) {
	if !eqPath(path1, path2) {
		_, file, line, _ := runtime.Caller(1)
		t.Errorf("%s(%d): Expected path %v, got %v", file, line, path2, path1)
	}
}

// Test the isRoot method.
func TestIsRoot(t *testing.T) {
	root := PathName{}
	if !root.IsRoot() {
		t.Errorf("Should be root")
	}
	nonroot := PathName{"a"}
	if nonroot.IsRoot() {
		t.Errorf("Should not be root")
	}
}

// Test the ParsePath function.
func TestParsePath(t *testing.T) {
	expectPath(t, ParsePath(""), PathName{})
	expectPath(t, ParsePath("/"), PathName{})
	expectPath(t, ParsePath("a"), PathName{"a"})
	expectPath(t, ParsePath("a/b"), PathName{"a", "b"})
	expectPath(t, ParsePath("a/b/./c/d/../e"), PathName{"a", "b", "c", "d", "..", "e"})
}
