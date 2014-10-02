package codegen

import (
	"path"
	"sort"
	"strconv"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vdl/compile"
)

// Import represents a single package import.
type Import struct {
	Name string // Name of the import; may be empty.
	Path string // Path of the imported package; e.g. "veyron/vdl"

	// Local name that refers to the imported package; either the non-empty import
	// name, or the name of the imported package.
	Local string
}

// Imports is a collection of package imports.
// REQUIRED: The imports must be sorted by path.
type Imports []Import

// LookupLocal returns the local name that identifies the given pkgPath.
func (x Imports) LookupLocal(pkgPath string) string {
	ix := sort.Search(len(x), func(i int) bool { return x[i].Path >= pkgPath })
	if ix < len(x) && x[ix].Path == pkgPath {
		return x[ix].Local
	}
	return ""
}

// Each import must end up with a unique local name - when we see a collision we
// simply add a "_N" suffix where N starts at 2 and increments.
func uniqueImport(pkgName, pkgPath string, seen map[string]bool) Import {
	name := ""
	iter := 1
	for {
		local := pkgName
		if iter > 1 {
			local += "_" + strconv.Itoa(iter)
			name = local
		}
		if !seen[local] {
			// Found a unique local name - return the import.
			seen[local] = true
			return Import{name, pkgPath, local}
		}
		iter++
	}
}

// ImportsForFile returns the imports required for the given file f.
func ImportsForFile(f *compile.File) Imports {
	var ret Imports
	seen := make(map[string]bool)
	// Note that f.PackageDeps is already sorted by path, so we're guaranteed that
	// the returned Imports are also sorted by path.
	for _, dep := range f.PackageDeps {
		ret = append(ret, uniqueImport(dep.Name, dep.Path, seen))
	}
	return ret
}

// ImportsForValue returns the imports required to represent the given value v,
// from within the given pkgPath.  It requires that type names used in
// v are of the form "PKGPATH.NAME".
func ImportsForValue(v *vdl.Value, pkgPath string) Imports {
	deps := pkgDeps{}
	deps.MergeValue(v)
	var ret Imports
	seen := make(map[string]bool)
	for _, p := range deps.SortedPkgPaths() {
		if p != pkgPath {
			ret = append(ret, uniqueImport(path.Base(p), p, seen))
		}
	}
	return ret
}

// pkgDeps maintains a set of package path dependencies.
type pkgDeps map[string]bool

func (deps pkgDeps) insertIdent(ident string) {
	if pkgPath, _ := vdl.SplitIdent(ident); pkgPath != "" {
		deps[pkgPath] = true
	}
}

// MergeValue merges the package paths required to represent v into deps.
func (deps pkgDeps) MergeValue(v *vdl.Value) {
	deps.insertIdent(v.Type().Name())
	switch v.Kind() {
	case vdl.Any, vdl.OneOf, vdl.Nilable:
		elem := v.Elem()
		if elem != nil {
			deps.MergeValue(elem)
		}
	case vdl.Array, vdl.List:
		deps.insertIdent(v.Type().Elem().Name())
	case vdl.Set:
		deps.insertIdent(v.Type().Key().Name())
	case vdl.Map:
		deps.insertIdent(v.Type().Key().Name())
		deps.insertIdent(v.Type().Elem().Name())
	case vdl.Struct:
		for fx := 0; fx < v.Type().NumField(); fx++ {
			deps.insertIdent(v.Type().Field(fx).Type.Name())
		}
	}
}

// SortedPkgPaths deps as a sorted slice.
func (deps pkgDeps) SortedPkgPaths() []string {
	var ret []string
	for pkgPath, _ := range deps {
		ret = append(ret, pkgPath)
	}
	sort.Strings(ret)
	return ret
}
