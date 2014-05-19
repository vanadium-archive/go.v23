package compile

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"veyron2/idl2"
	"veyron2/idl2/parse"
	"veyron2/val"
)

// Env is the environment for compilation.  It contains all errors that were
// collected during the execution - you can pass Errors to the parse phase to
// collect all errors together.  As packages are compiled it also collects the
// output; after a sequence of dependent packages is compiled, all compiled
// output will be collected.
//
// Always create a new Env via NewEnv; the zero Env is invalid.
type Env struct {
	Errors    *idl2.Errors
	pkgs      map[string]*Package
	typeDefs  map[*val.Type]*TypeDef
	constDefs map[*val.Value]*ConstDef
}

// NewEnv creates a new Env, allowing up to maxErrors errors before we stop.
func NewEnv(maxErrors int) *Env {
	env := &Env{
		Errors:    idl2.NewErrors(maxErrors),
		pkgs:      make(map[string]*Package),
		typeDefs:  make(map[*val.Type]*TypeDef),
		constDefs: make(map[*val.Value]*ConstDef),
	}
	// The env always starts out with the global package.
	env.pkgs[GlobalPackage.Name] = GlobalPackage
	for _, def := range GlobalFile.TypeDefs {
		env.typeDefs[def.Type] = def
	}
	for _, def := range GlobalFile.ConstDefs {
		env.constDefs[def.Value] = def
	}
	return env
}

// FindTypeDef returns the type definition corresponding to t, or nil if t isn't
// a defined type.  All built-in and user-defined named types are considered
// defined; e.g. unnamed lists don't have a corresponding type def.
func (e *Env) FindTypeDef(t *val.Type) *TypeDef { return e.typeDefs[t] }

// FindConstDef returns the const definition corresponding to v, or nil if v
// isn't a defined const.  All user-defined named consts are considered defined;
// e.g. method tags don't have a corresponding const def.
func (e *Env) FindConstDef(v *val.Value) *ConstDef { return e.constDefs[v] }

// ResolvePackage resolves a package path to its previous compiled results.
func (e *Env) ResolvePackage(path string) *Package {
	return e.pkgs[path]
}

// resolvePartial resolves the name to a partially-resolved name, and a list of
// packages it might be defined in.  The caller should complete resolution by
// trying the partial name on each package, in the order listed.
func (e *Env) resolvePartial(name string, file *File) (string, []*Package) {
	switch n := strings.SplitN(name, ".", 2); len(n) {
	case 2:
		// Fully qualified name "A.B", where A refers to the package and B is the
		// local name within the package.
		if path := file.LookupImportPath(n[0]); path != "" {
			if pkg := e.ResolvePackage(path); pkg != nil {
				return n[1], []*Package{pkg}
			}
		}
	case 1:
		// No dots name "A", where A may either refer to a global identifier, or the
		// local package the file is contained in.
		return n[0], []*Package{GlobalPackage, file.Package}
	}
	return "", nil
}

// ResolveType resolves a type name to its definition.  Name resolution uses the
// context of the file it appears in; e.g. the name "a.foo" requires the file
// imports in order to resolve.
func (e *Env) ResolveType(name string, file *File) *TypeDef {
	n, pkgs := e.resolvePartial(name, file)
	for _, pkg := range pkgs {
		if x := pkg.ResolveType(n); x != nil {
			return x
		}
	}
	return nil
}

// ResolveConst resolves a const name to its definition.  Name resolution uses
// the context of the file it appears in; e.g. the name "a.foo" requires the
// file imports in order to resolve.
func (e *Env) ResolveConst(name string, file *File) *ConstDef {
	n, pkgs := e.resolvePartial(name, file)
	for _, pkg := range pkgs {
		if x := pkg.ResolveConst(n); x != nil {
			return x
		}
	}
	return nil
}

// ResolveInterface resolves an interface name to its definition.  Name
// resolution uses the context of the file it appears in; e.g. the name "a.foo"
// requires the file imports in order to resolve.
func (e *Env) ResolveInterface(name string, file *File) *Interface {
	n, pkgs := e.resolvePartial(name, file)
	for _, pkg := range pkgs {
		if x := pkg.ResolveInterface(n); x != nil {
			return x
		}
	}
	return nil
}

// errorf and the fpString{,f} functions are helpers for error reporting; we
// want all errors to consistently contain the file and position of the error
// when possible.
func (e *Env) errorf(file *File, pos parse.Pos, format string, v ...interface{}) {
	e.Errors.Error(fpStringf(file, pos, format, v...))
}

func (e *Env) prefixErrorf(file *File, pos parse.Pos, err error, format string, v ...interface{}) {
	e.Errors.Error(fpStringf(file, pos, format, v...) + " (" + err.Error() + ")")
}

func fpString(file *File, pos parse.Pos) string {
	return path.Join(file.Package.Path, file.BaseName) + ":" + pos.String()
}

func fpStringf(file *File, pos parse.Pos, format string, v ...interface{}) string {
	return fmt.Sprintf(fpString(file, pos)+" "+format, v...)
}

// Representation of the components of an idl file.  These data types represent
// the results of the compilation, used by generators for different languages.

// Package represents an idl package, containing a set of files.
type Package struct {
	Name  string  // e.g. "base"
	Path  string  // e.g. "veyron/rt/base"
	Files []*File // Files contained in the package

	// We hold some internal maps to make local name resolution cheap and easy.
	typeDefs  map[string]*TypeDef
	constDefs map[string]*ConstDef
	ifaceDefs map[string]*Interface
}

func newPackage(name, path string) *Package {
	return &Package{
		Name:      name,
		Path:      path,
		typeDefs:  make(map[string]*TypeDef),
		constDefs: make(map[string]*ConstDef),
		ifaceDefs: make(map[string]*Interface),
	}
}

// QualifiedName returns the fully-qualified name of an identifier, by
// prepending the identifier with the package path.
func (p *Package) QualifiedName(id string) string { return p.Path + "." + id }

// ResolveType resolves the type name to its definition.
func (p *Package) ResolveType(name string) *TypeDef { return p.typeDefs[name] }

// ResolveConst resolves the const name to its definition.
func (p *Package) ResolveConst(name string) *ConstDef { return p.constDefs[name] }

// ResolveInterface resolves the interface name to its definition.
func (p *Package) ResolveInterface(name string) *Interface { return p.ifaceDefs[name] }

// File represents a compiled idl file.
type File struct {
	BaseName   string       // Base name of the idl file, e.g. "foo.idl"
	PackageDef NamePos      // Name, position and docs of the "package" clause
	ErrorIDs   []*ErrorID   // ErrorIDs defined in this file
	TypeDefs   []*TypeDef   // Types defined in this file
	ConstDefs  []*ConstDef  // Consts defined in this file
	Interfaces []*Interface // Interfaces defined in this file
	Package    *Package     // Parent package

	TypeDeps    map[*val.Type]bool // Types the file depends on
	PackageDeps []*Package         // Packages the file depends on, sorted by path

	// Imports maps the user-supplied imports from local package name to package
	// path.  They may be different from PackageDeps since we evaluate all consts
	// to their final typed value.  E.g. let's say we have three idl files:
	//
	//   a/a.idl  type Foo int32; const A1 = Foo(1)
	//   b/b.idl  import "a";     const B1 = a.Foo(1); const B2 = a.A1 + 1
	//   c/c.idl  import "b";     const C1 = b.B1;     const C2 = b.B1 + 1
	//
	// The final type and value of the constants:
	//   A1 = a.Foo(1); B1 = a.Foo(1); C1 = a.Foo(1)
	//                  B2 = a.Foo(2); C2 = a.Foo(2)
	//
	// Note that C1 and C2 both have final type a.Foo, even though c.idl doesn't
	// explicitly import "a", and the generated c.go shouldn't import "b" since
	// it's not actually used anymore.
	imports map[string]*importPath
}

type importPath struct {
	path string
	pos  parse.Pos
	used bool // was this import path ever used?
}

// LookupImportPath translates local into a package path name, based on the
// imports associated with the file.  Returns the empty string "" if local
// couldn't be found; every valid package path is non-empty.
func (f *File) LookupImportPath(local string) string {
	if imp, ok := f.imports[local]; ok {
		imp.used = true
		return imp.path
	}
	return ""
}

// ErrorID represents an error id.
type ErrorID parse.ErrorID

// Interface represents a set of embedded interfaces and methods.
type Interface struct {
	NamePos              // interface name, pos and doc
	Embeds  []*Interface // list of embedded interfaces
	Methods []*Method    // list of methods
	File    *File        // parent file
}

// Method represents a method in an interface.
type Method struct {
	NamePos                // method name, pos and doc
	InArgs    []*Arg       // list of positional in-args
	OutArgs   []*Arg       // list of positional out-args
	InStream  *val.Type    // in-stream type, may be nil
	OutStream *val.Type    // out-stream type, may be nil
	Tags      []*val.Value // list of method tags
}

// Arg represents method arguments.
type Arg struct {
	NamePos           // arg name, pos and doc
	Type    *val.Type // arg type, never nil
}

// NamePos represents a name, its associated position and documentation.
type NamePos parse.NamePos

func (x *ErrorID) String() string { return fmt.Sprintf("%+v", *x) }
func (x *Method) String() string  { return fmt.Sprintf("%+v", *x) }
func (x *Arg) String() string     { return fmt.Sprintf("%+v", *x) }
func (x *NamePos) String() string { return fmt.Sprintf("%+v", *x) }
func (x *Package) String() string {
	c := *x
	c.typeDefs = nil
	c.constDefs = nil
	c.ifaceDefs = nil
	return fmt.Sprintf("%+v", c)
}
func (x *File) String() string {
	c := *x
	c.Package = nil // avoid infinite loop
	return fmt.Sprintf("%+v", c)
}
func (x *Interface) String() string {
	c := *x
	c.File = nil // avoid infinite loop
	return fmt.Sprintf("%+v", c)
}

// We might consider allowing more characters, but we'll need to ensure they're
// allowed in all our codegen languages.
var (
	regexpIdent   = regexp.MustCompile("^[A-Z][A-Za-z0-9_]*$")
	regexpArgName = regexp.MustCompile("^[A-Za-z][A-Za-z0-9_]*$")
)

// ValidIdent returns a nil error iff the identifier may be used as a type name,
// method name, struct field name or enum label. Allowed: "^[A-Z][A-Za-z0-9_]*$"
func ValidIdent(ident string) error {
	if re := regexpIdent; !re.MatchString(ident) {
		return fmt.Errorf("invalid identifier %q, allowed regexp: %q", ident, re)
	}
	return nil
}

// ValidArgName returns a nil error iff the name may be used as an input or
// output argument name. Allowed: "^[A-Za-z][A-Za-z0-9_]*$"
func ValidArgName(name string) error {
	if re := regexpArgName; !re.MatchString(name) {
		return fmt.Errorf("invalid arg name %q, allowed regexp: %q", name, re)
	}
	return nil
}
