package compile

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vdl/opconst"
	"v.io/core/veyron2/vdl/parse"
	"v.io/core/veyron2/vdl/vdlutil"
)

// Env is the environment for compilation.  It contains all errors that were
// collected during the execution - you can pass Errors to the parse phase to
// collect all errors together.  As packages are compiled it also collects the
// output; after a sequence of dependent packages is compiled, all compiled
// output will be collected.
//
// Always create a new Env via NewEnv; the zero Env is invalid.
type Env struct {
	Errors    *vdlutil.Errors
	pkgs      map[string]*Package
	typeDefs  map[*vdl.Type]*TypeDef
	constDefs map[*vdl.Value]*ConstDef

	disallowPathQualifiers bool // Disallow syntax like "a/b/c".Type
}

// NewEnv creates a new Env, allowing up to maxErrors errors before we stop.
func NewEnv(maxErrors int) *Env {
	return NewEnvWithErrors(vdlutil.NewErrors(maxErrors))
}

// NewEnvWithErrors creates a new Env, using the given errs to collect errors.
func NewEnvWithErrors(errs *vdlutil.Errors) *Env {
	env := &Env{
		Errors:    errs,
		pkgs:      make(map[string]*Package),
		typeDefs:  make(map[*vdl.Type]*TypeDef),
		constDefs: make(map[*vdl.Value]*ConstDef),
	}
	// The env always starts out with the built-in package.
	env.pkgs[BuiltInPackage.Name] = BuiltInPackage
	for _, def := range BuiltInFile.TypeDefs {
		env.typeDefs[def.Type] = def
	}
	for _, def := range BuiltInFile.ConstDefs {
		env.constDefs[def.Value] = def
	}
	return env
}

// FindTypeDef returns the type definition corresponding to t, or nil if t isn't
// a defined type.  All built-in and user-defined named types are considered
// defined; e.g. unnamed lists don't have a corresponding type def.
func (e *Env) FindTypeDef(t *vdl.Type) *TypeDef { return e.typeDefs[t] }

// FindConstDef returns the const definition corresponding to v, or nil if v
// isn't a defined const.  All user-defined named consts are considered defined;
// e.g. method tags don't have a corresponding const def.
func (e *Env) FindConstDef(v *vdl.Value) *ConstDef { return e.constDefs[v] }

// ResolvePackage resolves a package path to its previous compiled results.
func (e *Env) ResolvePackage(path string) *Package {
	return e.pkgs[path]
}

// Resolves a name against the current package and imported package namespace.
func (e *Env) resolve(name string, file *File) (val interface{}, matched string) {
	// First handle package-path qualified identifiers, which look like this:
	//   "a/b/c".Ident   (qualified with package path "a/b/c")
	// These must be handled first, since the package-path may include dots.
	if strings.HasPrefix(name, `"`) {
		if parts := strings.SplitN(name[1:], `".`, 2); len(parts) == 2 {
			path, remain := parts[0], parts[1]
			if e.disallowPathQualifiers {
				// TODO(toddw): Add real position.
				e.Errorf(file, parse.Pos{}, "package path qualified identifier %s not allowed", name)
			}
			if file.ValidateImportPackagePath(path) {
				if pkg := e.ResolvePackage(path); pkg != nil {
					if dotParts := strings.Split(remain, "."); len(dotParts) > 0 {
						if val := pkg.resolve(dotParts[0], false); val != nil {
							return val, `"` + path + `".` + dotParts[0]
						}
					}
				}
			}
		}
	}
	// Now handle built-in and package-local identifiers.  Examples:
	//   string
	//   TypeName
	//   EnumType.Label
	//   ConstName
	//   StructConst.Field
	//   InterfaceName
	nameParts := strings.Split(name, ".")
	if len(nameParts) == 0 {
		return nil, ""
	}
	if builtin := BuiltInPackage.resolve(nameParts[0], false); builtin != nil {
		return builtin, nameParts[0]
	}
	if local := file.Package.resolve(nameParts[0], true); local != nil {
		return local, nameParts[0]
	}
	// Now handle package qualified identifiers, which look like this:
	//   pkg.Ident   (qualified with local package identifier pkg)
	if len(nameParts) > 1 {
		if path := file.LookupImportPath(nameParts[0]); path != "" {
			if pkg := e.ResolvePackage(path); pkg != nil {
				if val := pkg.resolve(nameParts[1], false); val != nil {
					return val, nameParts[0] + "." + nameParts[1]
				}
			}
		}
	}
	// No match found.
	return nil, ""
}

// ResolveType resolves a name to a type definition.
// Returns the type def and the portion of name that was matched.
func (e *Env) ResolveType(name string, file *File) (td *TypeDef, matched string) {
	v, matched := e.resolve(name, file)
	td, _ = v.(*TypeDef)
	if td == nil {
		return nil, ""
	}
	return td, matched
}

// ResolveConst resolves a name to a const definition.
// Returns the const def and the portion of name that was matched.
func (e *Env) ResolveConst(name string, file *File) (cd *ConstDef, matched string) {
	v, matched := e.resolve(name, file)
	cd, _ = v.(*ConstDef)
	if cd == nil {
		return nil, ""
	}
	return cd, matched
}

// ResolveInterface resolves a name to an interface definition.
// Returns the interface and the portion of name that was matched.
func (e *Env) ResolveInterface(name string, file *File) (i *Interface, matched string) {
	v, matched := e.resolve(name, file)
	i, _ = v.(*Interface)
	if i == nil {
		return nil, ""
	}
	return i, matched
}

// evalSelectorOnConst evaluates a selector on a const to a constant.
// This returns an empty const if a selector is applied on a non-struct value.
func (e *Env) evalSelectorOnConst(def *ConstDef, selector string) (opconst.Const, error) {
	v := def.Value
	for _, fieldName := range strings.Split(selector, ".") {
		if v.Kind() != vdl.Struct {
			return opconst.Const{}, fmt.Errorf("invalid selector on const of kind: %v", v.Type().Kind())
		}
		_, i := v.Type().FieldByName(fieldName)
		if i < 0 {
			return opconst.Const{}, fmt.Errorf("invalid field name on struct %s: %s", v, fieldName)
		}
		v = v.Field(i)
	}
	return opconst.FromValue(v), nil
}

// evalSelectorOnType evaluates a selector on a type to a constant.
// This returns an empty const if a selector is applied on a non-enum type.
func (e *Env) evalSelectorOnType(def *TypeDef, selector string) (opconst.Const, error) {
	t := def.Type
	if t.Kind() != vdl.Enum {
		return opconst.Const{}, fmt.Errorf("invalid selector on type of kind: %v", t.Kind())
	}
	if t.EnumIndex(selector) < 0 {
		return opconst.Const{}, fmt.Errorf("invalid label on enum %s: %s", t.Name(), selector)
	}
	enumVal := vdl.ZeroValue(t)
	enumVal.AssignEnumLabel(selector)
	return opconst.FromValue(enumVal), nil
}

// EvalConst resolves and evaluates a name to a const.
func (e *Env) EvalConst(name string, file *File) (opconst.Const, error) {
	if cd, matched := e.ResolveConst(name, file); cd != nil {
		if matched == name {
			return opconst.FromValue(cd.Value), nil
		}
		remainder := name[len(matched)+1:]
		c, err := e.evalSelectorOnConst(cd, remainder)
		if err != nil {
			return opconst.Const{}, err
		}
		return c, nil
	}
	if td, matched := e.ResolveType(name, file); td != nil {
		if matched == name {
			return opconst.Const{}, fmt.Errorf("%s is a type", name)
		}
		remainder := name[len(matched)+1:]
		c, err := e.evalSelectorOnType(td, remainder)
		if err != nil {
			return opconst.Const{}, err
		}
		return c, nil
	}
	return opconst.Const{}, fmt.Errorf("%s undefined", name)
}

// Errorf is a helper for error reporting, to consistently contain the file and
// position of the error when possible.
func (e *Env) Errorf(file *File, pos parse.Pos, format string, v ...interface{}) {
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

// DisallowPathQualifiers disables syntax like "a/b/c".Type.
func (e *Env) DisallowPathQualifiers() *Env {
	e.disallowPathQualifiers = true
	return e
}

// Representation of the components of an vdl file.  These data types represent
// the results of the compilation, used by generators for different languages.

// Package represents an vdl package, containing a set of files.
type Package struct {
	Name  string  // e.g. "base"
	Path  string  // e.g. "v.io/core/veyron/rt/base"
	Files []*File // Files contained in the package

	// We hold some internal maps to make local name resolution cheap and easy.
	typeDefs  map[string]*TypeDef
	constDefs map[string]*ConstDef
	ifaceDefs map[string]*Interface

	// lowercaseIdents maps from lowercased identifier to a detail string; it's
	// used to detect and report identifier conflicts.
	lowercaseIdents map[string]string
}

func newPackage(name, path string) *Package {
	return &Package{
		Name:            name,
		Path:            path,
		typeDefs:        make(map[string]*TypeDef),
		constDefs:       make(map[string]*ConstDef),
		ifaceDefs:       make(map[string]*Interface),
		lowercaseIdents: make(map[string]string),
	}
}

// QualifiedName returns the fully-qualified name of an identifier, by
// prepending the identifier with the package path.
func (p *Package) QualifiedName(id string) string {
	if p.Path == "" {
		return id
	}
	return p.Path + "." + id
}

// ResolveType resolves the type name to its definition.
func (p *Package) ResolveType(name string) *TypeDef { return p.typeDefs[name] }

// ResolveConst resolves the const name to its definition.
func (p *Package) ResolveConst(name string) *ConstDef { return p.constDefs[name] }

// ResolveInterface resolves the interface name to its definition.
func (p *Package) ResolveInterface(name string) *Interface { return p.ifaceDefs[name] }

// resolve resolves a name against the package.
// Checks for duplicate definitions should be performed before this is called.
func (p *Package) resolve(name string, isLocal bool) interface{} {
	if t := p.ResolveType(name); t != nil && (t.Exported || isLocal) {
		return t
	}
	if c := p.ResolveConst(name); c != nil && (c.Exported || isLocal) {
		return c
	}
	if i := p.ResolveInterface(name); i != nil && (i.Exported || isLocal) {
		return i
	}
	return nil
}

// File represents a compiled vdl file.
type File struct {
	BaseName   string       // Base name of the vdl file, e.g. "foo.vdl"
	PackageDef NamePos      // Name, position and docs of the "package" clause
	ErrorDefs  []*ErrorDef  // Errors defined in this file
	TypeDefs   []*TypeDef   // Types defined in this file
	ConstDefs  []*ConstDef  // Consts defined in this file
	Interfaces []*Interface // Interfaces defined in this file
	Package    *Package     // Parent package

	TypeDeps    map[*vdl.Type]bool // Types the file depends on
	PackageDeps []*Package         // Packages the file depends on, sorted by path

	// Imports maps the user-supplied imports from local package name to package
	// path.  They may be different from PackageDeps since we evaluate all consts
	// to their final typed value.  E.g. let's say we have three vdl files:
	//
	//   a/a.vdl  type Foo int32; const A1 = Foo(1)
	//   b/b.vdl  import "a";     const B1 = a.Foo(1); const B2 = a.A1 + 1
	//   c/c.vdl  import "b";     const C1 = b.B1;     const C2 = b.B1 + 1
	//
	// The final type and value of the constants:
	//   A1 = a.Foo(1); B1 = a.Foo(1); C1 = a.Foo(1)
	//                  B2 = a.Foo(2); C2 = a.Foo(2)
	//
	// Note that C1 and C2 both have final type a.Foo, even though c.vdl doesn't
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

// ValidateImportPackagePath returns true iff path is listed in the file's
// imports, and marks the import as used.
func (f *File) ValidateImportPackagePath(path string) bool {
	for _, imp := range f.imports {
		if imp.path == path {
			imp.used = true
			return true
		}
	}
	return false
}

// identDetail formats a detail string for calls to DeclareIdent.
func identDetail(kind string, file *File, pos parse.Pos) string {
	return fmt.Sprintf("%s at %s:%s", kind, file.BaseName, pos)
}

// DeclareIdent declares ident with the given detail string.  Returns an error
// if ident conflicts with an existing identifier in this file or package, where
// the error includes the the previous declaration detail.
func (f *File) DeclareIdent(ident, detail string) error {
	// Identifiers must be distinct from the the import names used in this file,
	// but can differ by only their capitalization.  E.g.
	//   import "foo"
	//   type foo string // BAD, type "foo" collides with import "foo"
	//   type Foo string //  OK, type "Foo" distinct from import "foo"
	//   type FoO string //  OK, type "FoO" distinct from import "foo"
	if i, ok := f.imports[ident]; ok {
		return fmt.Errorf("previous import at %s", i.pos)
	}
	// Identifiers must be distinct from all other identifiers within this
	// package, and cannot differ by only their capitalization.  E.g.
	//   type foo string
	//   const foo = "a" // BAD, const "foo" collides with type "foo"
	//   const Foo = "A" // BAD, const "Foo" collides with type "foo"
	//   const FoO = "A" // BAD, const "FoO" collides with type "foo"
	lower := strings.ToLower(ident)
	if prevDetail := f.Package.lowercaseIdents[lower]; prevDetail != "" {
		return fmt.Errorf("previous %s", prevDetail)
	}
	f.Package.lowercaseIdents[lower] = detail
	return nil
}

// Interface represents a set of embedded interfaces and methods.
type Interface struct {
	NamePos               // interface name, pos and doc
	Exported bool         // is this interface exported?
	Embeds   []*Interface // list of embedded interfaces
	Methods  []*Method    // list of methods
	File     *File        // parent file
}

// Method represents a method in an interface.
type Method struct {
	NamePos                // method name, pos and doc
	InArgs    []*Arg       // list of positional in-args
	OutArgs   []*Arg       // list of positional out-args
	InStream  *vdl.Type    // in-stream type, may be nil
	OutStream *vdl.Type    // out-stream type, may be nil
	Tags      []*vdl.Value // list of method tags
}

// Arg represents method arguments and error params.
//
// TODO(toddw): Rename to Field, to match the parse package.
type Arg struct {
	NamePos           // arg name, pos and doc
	Type    *vdl.Type // arg type, never nil
}

// NamePos represents a name, its associated position and documentation.
type NamePos parse.NamePos

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
func (x *Interface) AllMethods() []*Method {
	result := make([]*Method, len(x.Methods))
	copy(result, x.Methods)
	for _, embed := range x.Embeds {
		result = append(result, embed.AllMethods()...)
	}
	return result
}
func (x *Interface) TransitiveEmbeds() []*Interface {
	return x.transitiveEmbeds(make(map[*Interface]bool))
}
func (x *Interface) transitiveEmbeds(seen map[*Interface]bool) []*Interface {
	var ret []*Interface
	for _, e := range x.Embeds {
		if !seen[e] {
			seen[e] = true
			ret = append(ret, e)
			ret = append(ret, e.transitiveEmbeds(seen)...)
		}
	}
	return ret
}

// We might consider allowing more characters, but we'll need to ensure they're
// allowed in all our codegen languages.
var (
	regexpIdent = regexp.MustCompile("^[A-Za-z][A-Za-z0-9_]*$")
)

// ValidIdent returns (exported, err) where err is non-nil iff the identifer is
// valid, and exported is true if the identifier is exported.
// Valid: "^[A-Za-z][A-Za-z0-9_]*$"
func ValidIdent(ident string, mode ReservedMode) (bool, error) {
	if re := regexpIdent; !re.MatchString(ident) {
		return false, fmt.Errorf("%q invalid, allowed regexp: %q", ident, re)
	}
	if reservedWord(ident, mode) {
		return false, fmt.Errorf("%q invalid identifier (keyword in a generated language)", ident)
	}
	return ident[0] >= 'A' && ident[0] <= 'Z', nil
}

// ValidExportedIdent returns a non-nil error iff the identifier is valid and
// exported.
func ValidExportedIdent(ident string, mode ReservedMode) error {
	exported, err := ValidIdent(ident, mode)
	if err != nil {
		return err
	}
	if !exported {
		return fmt.Errorf("%q must be exported", ident)
	}
	return nil
}
