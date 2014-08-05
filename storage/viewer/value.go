package viewer

import (
	"path/filepath"
	"sort"
	"time"

	"veyron2/rt"
	"veyron2/storage"
)

// Value is the type used to pass store values to the template.  The Value
// contains the name of the value, the actual value, and a list of
// subdirectories.
type Value struct {
	store   storage.Store
	Name    string
	Value   interface{}
	Subdirs []string
}

// glob performs a glob expansion of the pattern.  The results are sorted.
func glob(st storage.Store, path, pattern string) ([]string, error) {
	results := st.BindObject(path).Glob(rt.R().TODOContext(), pattern)
	names := []string{}
	rStream := results.RecvStream()
	for rStream.Advance() {
		names = append(names, "/"+rStream.Value())
	}
	if err := rStream.Err(); err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

// fullpath returns the absolute path from a relative path.
func (v *Value) fullpath(path string) string {
	if len(path) == 0 || path[0] != '/' {
		return v.Name + "/" + path
	}
	return path
}

// Date performs a Time conversion, given an integer argument that represents a
// time in nanoseconds since the Unix epoch.
func (v *Value) Date(ns int64) time.Time {
	return time.Unix(0, ns)
}

// Join joins the path elements.
func (v *Value) Join(elem ...string) string {
	return filepath.ToSlash(filepath.Join(elem...))
}

// Base returns the last element of the path.
func (v *Value) Base(path string) string {
	return filepath.Base(path)
}

// Glob performs a glob expansion of a pattern.
func (v *Value) Glob(pattern string) ([]string, error) {
	return glob(v.store, v.Name, pattern)
}

// Get fetches a value from the store.  The result is nil if the value does not
// exist.
func (v *Value) Get(path string) interface{} {
	path = v.fullpath(path)
	e, err := v.store.BindObject(path).Get(rt.R().TODOContext())
	if err != nil {
		return nil
	}
	return e.Value
}
