package compile

import (
	"fmt"

	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/vdl/parse"
)

// TypeDef represents a user-defined named type definition in the compiled
// results.
type TypeDef struct {
	NamePos            // name, parse position and docs
	Exported bool      // is this type definition exported?
	Type     *vdl.Type // type of this type definition

	// BaseType is the type that Type is based on.  The BaseType may be named or
	// unnamed.  E.g.
	//                                 BaseType
	//   type Bool    bool;            bool
	//   type Bool2   Bool;            Bool
	//   type List    []int32;         []int32
	//   type List2   List;            List
	//   type Struct  struct{A bool};  struct{A bool}
	//   type Struct2 Struct;          Struct
	BaseType *vdl.Type

	LabelDoc       []string // [valid for enum] docs for each label
	LabelDocSuffix []string // [valid for enum] suffix docs for each label
	FieldDoc       []string // [valid for struct, union] docs for each field
	FieldDocSuffix []string // [valid for struct, union] suffix docs for each field
	File           *File    // parent file that this type is defined in
}

func (x *TypeDef) String() string {
	c := *x
	c.File = nil // avoid infinite loop
	return fmt.Sprintf("%+v", c)
}

// compileTypeDefs is the "entry point" to the rest of this file.  It takes the
// types defined in pfiles and compiles them into TypeDefs in pkg.
func compileTypeDefs(pkg *Package, pfiles []*parse.File, env *Env) {
	td := typeDefiner{
		pkg:      pkg,
		pfiles:   pfiles,
		env:      env,
		tbuilder: &vdl.TypeBuilder{},
		builders: make(map[string]*typeDefBuilder),
	}
	if td.Declare(); !env.Errors.IsEmpty() {
		return
	}
	if td.Define(); !env.Errors.IsEmpty() {
		return
	}
	td.Build()
	// TODO(toddw): should we disallow inter-file cyclic type dependencies?  That
	// might be an issue for generated C++.
}

// typeDefiner defines types in a package.  This is split into three phases:
// 1) Declare ensures local type references can be resolved.
// 2) Define describes each type, resolving named references.
// 3) Build builds all types.
//
// It holds a builders map from type name to typeDefBuilder, where the
// typeDefBuilder is responsible for compiling and defining a single type.
type typeDefiner struct {
	pkg      *Package
	pfiles   []*parse.File
	env      *Env
	tbuilder *vdl.TypeBuilder
	builders map[string]*typeDefBuilder
}

type typeDefBuilder struct {
	def     *TypeDef
	ptype   parse.Type
	pending vdl.PendingNamed // named type that's being built
	base    vdl.PendingType  // base type that pending is based on
}

// Declare creates builders for each type defined in the package.
func (td typeDefiner) Declare() {
	for ix := range td.pkg.Files {
		file, pfile := td.pkg.Files[ix], td.pfiles[ix]
		for _, pdef := range pfile.TypeDefs {
			detail := identDetail("type", file, pdef.Pos)
			if err := file.DeclareIdent(pdef.Name, detail); err != nil {
				td.env.prefixErrorf(file, pdef.Pos, err, "type %s name conflict", pdef.Name)
				continue
			}
			td.builders[pdef.Name] = td.makeTypeDefBuilder(file, pdef)
		}
	}
}

func (td typeDefiner) makeTypeDefBuilder(file *File, pdef *parse.TypeDef) *typeDefBuilder {
	export, err := ValidIdent(pdef.Name, ReservedNormal)
	if err != nil {
		td.env.prefixErrorf(file, pdef.Pos, err, "type %s invalid name", pdef.Name)
		return nil
	}
	ret := new(typeDefBuilder)
	ret.def = &TypeDef{NamePos: NamePos(pdef.NamePos), Exported: export, File: file}
	ret.ptype = pdef.Type
	// We use the qualified name to actually name the type, to ensure types
	// defined in separate packages are hash-consed separately.
	qname := file.Package.QualifiedName(pdef.Name)
	ret.pending = td.tbuilder.Named(qname)
	switch pt := pdef.Type.(type) {
	case *parse.TypeEnum:
		ret.def.LabelDoc = make([]string, len(pt.Labels))
		ret.def.LabelDocSuffix = make([]string, len(pt.Labels))
		for index, plabel := range pt.Labels {
			if err := ValidExportedIdent(plabel.Name, ReservedCamelCase); err != nil {
				td.env.prefixErrorf(file, plabel.Pos, err, "invalid enum label name %s", plabel.Name)
				return nil
			}
			ret.def.LabelDoc[index] = plabel.Doc
			ret.def.LabelDocSuffix[index] = plabel.DocSuffix
		}
	case *parse.TypeStruct:
		ret = attachFieldDoc(ret, pt.Fields, file, td.env)
	case *parse.TypeUnion:
		ret = attachFieldDoc(ret, pt.Fields, file, td.env)
	}
	return ret
}

func attachFieldDoc(ret *typeDefBuilder, fields []*parse.Field, file *File, env *Env) *typeDefBuilder {
	ret.def.FieldDoc = make([]string, len(fields))
	ret.def.FieldDocSuffix = make([]string, len(fields))
	for index, pfield := range fields {
		if err := ValidExportedIdent(pfield.Name, ReservedCamelCase); err != nil {
			env.prefixErrorf(file, pfield.Pos, err, "invalid field name %s", pfield.Name)
			return nil
		}
		ret.def.FieldDoc[index] = pfield.Doc
		ret.def.FieldDocSuffix[index] = pfield.DocSuffix
	}
	return ret
}

// Define uses the builders to describe each type.  Named types defined in
// other packages must have already been compiled, and in env.  Named types
// defined in this package are represented by the builders.
func (td typeDefiner) Define() {
	for _, b := range td.builders {
		def, file := b.def, b.def.File
		base := compileDefinedType(b.ptype, file, td.env, td.tbuilder, td.builders)
		switch tbase := base.(type) {
		case nil:
			continue // keep going to catch  more errors
		case *vdl.Type:
			if tbase == vdl.ErrorType {
				td.env.Errorf(file, def.Pos, "error cannot be renamed")
				continue // keep going to catch more errors
			}
			def.BaseType = tbase
		case vdl.PendingType:
			b.base = tbase
		default:
			panic(fmt.Errorf("vdl: typeDefiner.Define unhandled TypeOrPending %T %v", tbase, tbase))
		}
		b.pending.AssignBase(base)
	}
}

// compileType returns the *vdl.Type corresponding to ptype.  All named types
// referenced by ptype must already be defined.
func compileType(ptype parse.Type, file *File, env *Env) *vdl.Type {
	var tbuilder vdl.TypeBuilder
	typeOrPending := compileLiteralType(ptype, file, env, &tbuilder, nil)
	tbuilder.Build()
	switch top := typeOrPending.(type) {
	case nil:
		return nil
	case *vdl.Type:
		return top
	case vdl.PendingType:
		t, err := top.Built()
		if err != nil {
			env.prefixErrorf(file, ptype.Pos(), err, "invalid type")
			return nil
		}
		return t
	default:
		panic(fmt.Errorf("vdl: compileType unhandled TypeOrPending %T %v", top, top))
	}
}

// compileDefinedType compiles ptype.  It can handle definitions based on array,
// enum, struct and union, as well as definitions based on any literal type.
func compileDefinedType(ptype parse.Type, file *File, env *Env, tbuilder *vdl.TypeBuilder, builders map[string]*typeDefBuilder) vdl.TypeOrPending {
	switch pt := ptype.(type) {
	case *parse.TypeArray:
		elem := compileLiteralType(pt.Elem, file, env, tbuilder, builders)
		if elem == nil {
			return nil
		}
		return tbuilder.Array().AssignLen(pt.Len).AssignElem(elem)
	case *parse.TypeEnum:
		enum := tbuilder.Enum()
		for _, plabel := range pt.Labels {
			enum.AppendLabel(plabel.Name)
		}
		return enum
	case *parse.TypeStruct:
		st := tbuilder.Struct()
		for _, pfield := range pt.Fields {
			ftype := compileLiteralType(pfield.Type, file, env, tbuilder, builders)
			if ftype == nil {
				return nil
			}
			st.AppendField(pfield.Name, ftype)
		}
		return st
	case *parse.TypeUnion:
		union := tbuilder.Union()
		for _, pfield := range pt.Fields {
			ftype := compileLiteralType(pfield.Type, file, env, tbuilder, builders)
			if ftype == nil {
				return nil
			}
			union.AppendField(pfield.Name, ftype)
		}
		return union
	}
	lit := compileLiteralType(ptype, file, env, tbuilder, builders)
	if _, ok := lit.(vdl.PendingOptional); ok {
		// Don't allow Optional at the top-level of a type definition.  The purpose
		// of this rule is twofold:
		// 1) Reduce confusion; the Optional modifier cannot be hidden in a type
		//    definition, it must be explicitly mentioned on each use.
		// 2) The Optional concept is typically translated to pointers in generated
		//    languages, and many languages don't support named pointer types.
		//
		//   type A string            // ok
		//   type B []?string         // ok
		//   type C struct{X ?string} // ok
		//   type C ?string           // bad
		//   type D ?struct{X string} // bad
		env.Errorf(file, ptype.Pos(), "can't define type based on top-level optional")
		return nil
	}
	return lit
}

// compileLiteralType compiles ptype.  It can handle any literal type.  Note
// that array, enum, struct and union are required to be defined and named,
// and aren't allowed as regular literal types.
func compileLiteralType(ptype parse.Type, file *File, env *Env, tbuilder *vdl.TypeBuilder, builders map[string]*typeDefBuilder) vdl.TypeOrPending {
	switch pt := ptype.(type) {
	case *parse.TypeNamed:
		if def, matched := env.ResolveType(pt.Name, file); def != nil {
			if len(matched) < len(pt.Name) {
				env.Errorf(file, pt.Pos(), "type %s invalid (%s unmatched)", pt.Name, pt.Name[len(matched):])
				return nil
			}
			return def.Type
		}
		if b, ok := builders[pt.Name]; ok {
			return b.pending
		}
		env.Errorf(file, pt.Pos(), "type %s undefined", pt.Name)
		return nil
	case *parse.TypeList:
		elem := compileLiteralType(pt.Elem, file, env, tbuilder, builders)
		if elem == nil {
			return nil
		}
		return tbuilder.List().AssignElem(elem)
	case *parse.TypeSet:
		key := compileLiteralType(pt.Key, file, env, tbuilder, builders)
		if key == nil {
			return nil
		}
		return tbuilder.Set().AssignKey(key)
	case *parse.TypeMap:
		key := compileLiteralType(pt.Key, file, env, tbuilder, builders)
		elem := compileLiteralType(pt.Elem, file, env, tbuilder, builders)
		if key == nil || elem == nil {
			return nil
		}
		return tbuilder.Map().AssignKey(key).AssignElem(elem)
	case *parse.TypeOptional:
		elem := compileLiteralType(pt.Base, file, env, tbuilder, builders)
		if elem == nil {
			return nil
		}
		return tbuilder.Optional().AssignElem(elem)
	default:
		env.Errorf(file, pt.Pos(), "unnamed %s type invalid (type must be defined)", ptype.Kind())
		return nil
	}
}

// Build actually builds each type and updates the package with the typedefs.
// The order we call each pending type doesn't matter; the veyron2/vdl package
// deals with arbitrary orders, and supports recursive types.  However we want
// the order to be deterministic, otherwise the output will constantly change.
// So we use the same order as the parsed file.
func (td typeDefiner) Build() {
	td.tbuilder.Build()
	for _, pfile := range td.pfiles {
		for _, pdef := range pfile.TypeDefs {
			b := td.builders[pdef.Name]
			def, file := b.def, b.def.File
			if b.base != nil {
				base, err := b.base.Built()
				if err != nil {
					td.env.prefixErrorf(file, b.ptype.Pos(), err, "%s base type invalid", def.Name)
					continue // keep going to catch more errors
				}
				def.BaseType = base
			}
			t, err := b.pending.Built()
			if err != nil {
				td.env.prefixErrorf(file, def.Pos, err, "%s invalid", def.Name)
				continue // keep going to catch more errors
			}
			def.Type = t
			addTypeDef(def, td.env)
		}
	}
	// Make another pass to fill in doc and doc suffix slices for enums, structs
	// and unions.  Typically these are initialized in makeTypeDefBuilder, based
	// on the underlying parse data.  But type definitions based on other named
	// types can't be updated until the base type is actually compiled.
	//
	// TODO(toddw): This doesn't actually attach comments from the base type, it
	// just leaves everything empty.  This is fine for now, but we should revamp
	// the vdl parsing / comment attaching strategy in the future.
	for _, file := range td.pkg.Files {
		for _, def := range file.TypeDefs {
			switch t := def.Type; t.Kind() {
			case vdl.Enum:
				if len(def.LabelDoc) == 0 {
					def.LabelDoc = make([]string, t.NumEnumLabel())
				}
				if len(def.LabelDocSuffix) == 0 {
					def.LabelDocSuffix = make([]string, t.NumEnumLabel())
				}
			case vdl.Struct, vdl.Union:
				if len(def.FieldDoc) == 0 {
					def.FieldDoc = make([]string, t.NumField())
				}
				if len(def.FieldDocSuffix) == 0 {
					def.FieldDocSuffix = make([]string, t.NumField())
				}
			}
		}
	}
}

// addTypeDef updates our various structures to add a new type def.
func addTypeDef(def *TypeDef, env *Env) {
	def.File.TypeDefs = append(def.File.TypeDefs, def)
	def.File.Package.typeDefs[def.Name] = def
	if env != nil {
		// env should only be nil during initialization of the built-in package;
		// NewEnv ensures new environments have the built-in types.
		env.typeDefs[def.Type] = def
	}
}
