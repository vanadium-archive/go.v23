package golang

// TODO(toddw): Add tests for this logic.

import (
	"sort"
	"strconv"

	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vdl/compile"
)

// goImport represents a single import in the generated Go file.
//   Example A: import     "v.io/core/abc"
//   Example B: import foo "v.io/core/abc"
type goImport struct {
	// Name of the import.
	//   Example A: ""
	//   Example B: "foo"
	Name string
	// Path of the import.
	//   Example A: "v.io/core/abc"
	//   Example B: "v.io/core/abc"
	Path string
	// Local identifier within the generated go file to reference the imported
	// package.
	//   Example A: "abc"
	//   Example B: "foo"
	Local string
}

// goImports holds all imports for a generated Go file, splitting into two
// groups "system" and "user".  The splitting is just for slightly nicer output,
// and to ensure we prefer system over user imports when dealing with package
// name collisions.
type goImports struct {
	System, User []goImport
}

func newImports(file *compile.File, env *compile.Env) *goImports {
	deps, user := computeDeps(file, env)
	system := systemImports(deps, file)
	seen := make(map[string]bool)
	return &goImports{
		System: system.Sort(seen),
		User:   user.Sort(seen),
	}
}

// importMap maps from package path to package name.  It's used to collect
// package import information.
type importMap map[string]string

// AddPackage adds a regular dependency on pkg; some block of generated code
// will reference the pkg.
func (im importMap) AddPackage(pkg *compile.Package) {
	im[pkg.GenPath] = pkg.Name
}

// AddForcedPackage adds a "forced" dependency on pkg.  This means that we need
// to import pkg even if no other block of generated code references the pkg.
func (im importMap) AddForcedPackage(pkg *compile.Package) {
	if im[pkg.GenPath] == "" {
		im[pkg.GenPath] = "_"
	}
}

func (im importMap) DeletePackage(pkg *compile.Package) {
	delete(im, pkg.GenPath)
}

func (im importMap) Sort(seen map[string]bool) []goImport {
	var sortedPaths []string
	for path := range im {
		sortedPaths = append(sortedPaths, path)
	}
	sort.Strings(sortedPaths)
	var ret []goImport
	for _, path := range sortedPaths {
		ret = append(ret, uniqueImport(im[path], path, seen))
	}
	return ret
}

// Each import must end up with a unique local name.  Here's some examples.
//   uniqueImport("a", "v.io/a", {})           -> goImport{"", "v.io/a", "a"}
//   uniqueImport("z", "v.io/a", {})           -> goImport{"", "v.io/a", "z"}
//   uniqueImport("a", "v.io/a", {"a"})        -> goImport{"a_2", "v.io/a", "a_2"}
//   uniqueImport("a", "v.io/a", {"a", "a_2"}) -> goImport{"a_3", "v.io/a", "a_3"}
//   uniqueImport("_", "v.io/a", {})           -> goImport{"_", "v.io/a", ""}
//   uniqueImport("_", "v.io/a", {"a"})        -> goImport{"_", "v.io/a", ""}
//   uniqueImport("_", "v.io/a", {"a", "a_2"}) -> goImport{"_", "v.io/a", ""}
func uniqueImport(pkgName, pkgPath string, seen map[string]bool) goImport {
	if pkgName == "_" {
		return goImport{"_", pkgPath, ""}
	}
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
			return goImport{name, pkgPath, local}
		}
		iter++
	}
}

// LookupLocal returns the local identifier within the generated go file that
// identifies the given pkgPath.
func (x *goImports) LookupLocal(pkgPath string) string {
	if local := lookupLocal(pkgPath, x.System); local != "" {
		return local
	}
	return lookupLocal(pkgPath, x.User)
}

func lookupLocal(pkgPath string, imports []goImport) string {
	ix := sort.Search(
		len(imports),
		func(i int) bool { return imports[i].Path >= pkgPath },
	)
	if ix < len(imports) && imports[ix].Path == pkgPath {
		return imports[ix].Local
	}
	return ""
}

type deps struct {
	any              bool
	typeObject       bool
	enumTypeDef      bool
	streamingMethods bool
	methodTags       bool
}

func computeDeps(file *compile.File, env *compile.Env) (deps, importMap) {
	deps, user := &deps{}, make(importMap)
	// TypeDef.Type is always defined in our package; add deps on the base type.
	for _, def := range file.TypeDefs {
		addTypeDeps(def.BaseType, env, deps, user)
		if def.Type.Kind() == vdl.Enum {
			deps.enumTypeDef = true
		}
	}
	// Consts contribute their value types.
	for _, def := range file.ConstDefs {
		addValueTypeDeps(def.Value, env, deps, user)
	}
	// Interfaces contribute their arg types and tag values, as well as embedded
	// interfaces.
	for _, iface := range file.Interfaces {
		for _, embed := range iface.TransitiveEmbeds() {
			user.AddPackage(embed.File.Package)
		}
		for _, method := range iface.Methods {
			for _, arg := range method.InArgs {
				addTypeDeps(arg.Type, env, deps, user)
			}
			for _, arg := range method.OutArgs {
				addTypeDeps(arg.Type, env, deps, user)
			}
			if stream := method.InStream; stream != nil {
				addTypeDeps(stream, env, deps, user)
				deps.streamingMethods = true
			}
			if stream := method.OutStream; stream != nil {
				addTypeDeps(stream, env, deps, user)
				deps.streamingMethods = true
			}
			for _, tag := range method.Tags {
				addValueTypeDeps(tag, env, deps, user)
				deps.methodTags = true
			}
		}
	}
	// Errors contribute their param types.
	for _, def := range file.ErrorDefs {
		for _, param := range def.Params {
			addTypeDeps(param.Type, env, deps, user)
		}
	}
	// Native types contribute their imports, for the auto-generated interface
	// assertion.
	for _, native := range file.Package.Config.Go.WireToNativeTypes {
		for _, imp := range native.Imports {
			user[imp.Path] = imp.Name
		}
	}
	// Now remove self and built-in package dependencies.  Every package can use
	// itself and the built-in package, so we don't need to record this.
	user.DeletePackage(file.Package)
	user.DeletePackage(compile.BuiltInPackage)
	return *deps, user
}

// Add package deps iff t is a defined (named) type.
func addTypeDepIfDefined(t *vdl.Type, env *compile.Env, deps *deps, user importMap) bool {
	if t == vdl.AnyType {
		deps.any = true
	}
	if t == vdl.TypeObjectType {
		deps.typeObject = true
	}
	if def := env.FindTypeDef(t); def != nil {
		pkg := def.File.Package
		if native, ok := pkg.Config.Go.WireToNativeTypes[def.Name]; ok {
			// There is a native type configured for this defined type.  Add the
			// imports corresponding to the native type.
			for _, imp := range native.Imports {
				user[imp.Path] = imp.Name
			}
			// Also add a "forced" import on the regular VDL package, to ensure the
			// wire type is registered, to establish the wire<->native mapping.
			user.AddForcedPackage(pkg)
		} else {
			// There's no native type configured for this defined type.  Add the
			// imports corresponding to the VDL package.
			user.AddPackage(pkg)
		}
		return true
	}
	return false
}

// Add immediate package deps for t and subtypes of t.
func addTypeDeps(t *vdl.Type, env *compile.Env, deps *deps, user importMap) {
	if addTypeDepIfDefined(t, env, deps, user) {
		// We don't track transitive dependencies, only immediate dependencies.
		return
	}
	// Not all types have TypeDefs; e.g. unnamed lists have no corresponding
	// TypeDef, so we need to traverse those recursively.
	addSubTypeDeps(t, env, deps, user)
}

// Add immediate package deps for subtypes of t.
func addSubTypeDeps(t *vdl.Type, env *compile.Env, deps *deps, user importMap) {
	switch t.Kind() {
	case vdl.Array, vdl.List, vdl.Optional:
		addTypeDeps(t.Elem(), env, deps, user)
	case vdl.Set:
		addTypeDeps(t.Key(), env, deps, user)
	case vdl.Map:
		addTypeDeps(t.Key(), env, deps, user)
		addTypeDeps(t.Elem(), env, deps, user)
	case vdl.Struct, vdl.Union:
		for ix := 0; ix < t.NumField(); ix++ {
			addTypeDeps(t.Field(ix).Type, env, deps, user)
		}
	}
}

// Add immediate package deps for v.Type(), and subvalues.  We must traverse the
// value to know which types are actually used; e.g. an empty struct doesn't
// have a dependency on its field types.
//
// The purpose of this method is to identify the package and type dependencies
// for const or tag values.
func addValueTypeDeps(v *vdl.Value, env *compile.Env, deps *deps, user importMap) {
	t := v.Type()
	addTypeDepIfDefined(t, env, deps, user)
	// Track transitive dependencies, by traversing subvalues recursively.
	switch t.Kind() {
	case vdl.Array, vdl.List:
		for ix := 0; ix < v.Len(); ix++ {
			addValueTypeDeps(v.Index(ix), env, deps, user)
		}
	case vdl.Set:
		for _, key := range v.Keys() {
			addValueTypeDeps(key, env, deps, user)
		}
	case vdl.Map:
		for _, key := range v.Keys() {
			addValueTypeDeps(key, env, deps, user)
			addValueTypeDeps(v.MapIndex(key), env, deps, user)
		}
	case vdl.Struct:
		if v.IsZero() {
			// We print zero-valued structs as {}, so we stop the traversal here.
			return
		}
		for ix := 0; ix < t.NumField(); ix++ {
			addValueTypeDeps(v.StructField(ix), env, deps, user)
		}
	case vdl.Union:
		_, field := v.UnionField()
		addValueTypeDeps(field, env, deps, user)
	case vdl.Any, vdl.Optional:
		if elem := v.Elem(); elem != nil {
			addValueTypeDeps(elem, env, deps, user)
		}
	case vdl.TypeObject:
		// TypeObject has dependencies on everything its zero value depends on.
		addValueTypeDeps(vdl.ZeroValue(v.TypeObject()), env, deps, user)
	}
}

// systemImports returns the vdl system imports for the given file and deps.
// You might think we could simply capture the imports during code generation,
// and then dump out all imports afterwards.  Unfortunately that strategy
// doesn't work, because of potential package name collisions in the imports.
//
// When generating code we need to know the local identifier used to reference a
// given imported package.  But the local identifier changes if there are
// collisions with other imported packages.  An example:
//
//   package pkg
//
//   import       "a/foo"
//   import foo_2 "b/foo"
//
//   type X foo.T
//   type Y foo_2.T
//
// Note that in order to generate code for X and Y, we need to know what local
// identifier to use.  But we only know what local identifier to use after we've
// collected all imports and resolved collisions.
func systemImports(deps deps, file *compile.File) importMap {
	system := make(importMap)
	if deps.any || deps.typeObject || deps.methodTags || len(file.TypeDefs) > 0 {
		// System import for vdl.Value, vdl.Type and vdl.Register.
		system["v.io/core/veyron2/vdl"] = "vdl"
	}
	if deps.enumTypeDef {
		system["fmt"] = "fmt"
	}
	if len(file.Interfaces) > 0 {
		system["v.io/core/veyron2"] = "veyron2"
		system["v.io/core/veyron2/context"] = "context"
		system["v.io/core/veyron2/ipc"] = "ipc"
	}
	if deps.streamingMethods {
		system["io"] = "io"
	}
	if len(file.ErrorDefs) > 0 {
		system["v.io/core/veyron2/context"] = "context"
		system["v.io/core/veyron2/i18n"] = "i18n"
		// If the user has specified any errors, typically we need to import the
		// "v.io/core/veyron2/verror" package.  However we allow vdl code-generation
		// in the "v.io/core/veyron2/verror" package itself, to specify common
		// errors.  Special-case this scenario to avoid self-cyclic package
		// dependencies.
		if file.Package.Path != "v.io/core/veyron2/verror" {
			system["v.io/core/veyron2/verror"] = "verror"
		}
	}
	// Now remove self package dependencies.
	system.DeletePackage(file.Package)
	return system
}
