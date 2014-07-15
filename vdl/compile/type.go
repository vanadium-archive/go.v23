package compile

import (
	"fmt"

	"veyron2/vdl"
	"veyron2/vdl/parse"
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

	LabelDoc       []string // [only valid for enum] docs for each label
	LabelDocSuffix []string // [only valid for enum] suffix docs for each label
	FieldDoc       []string // [only valid for struct] docs for each field
	FieldDocSuffix []string // [only valid for struct] suffix docs for each field
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
			if b, dup := td.builders[pdef.Name]; dup {
				td.env.errorf(file, pdef.Pos, "type %s redefined (previous at %s)", pdef.Name, fpString(b.def.File, b.def.Pos))
				continue // keep going to catch more errors
			}
			if err := file.ValidateNotDefined(pdef.Name); err != nil {
				td.env.prefixErrorf(file, pdef.Pos, err, "type %s name conflict", pdef.Name)
				continue
			}
			td.builders[pdef.Name] = td.makeTypeDefBuilder(file, pdef)
		}
	}
}

func (td typeDefiner) makeTypeDefBuilder(file *File, pdef *parse.TypeDef) *typeDefBuilder {
	export, err := ValidIdent(pdef.Name)
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
			_, err := ValidIdent(plabel.Name)
			if err != nil {
				td.env.prefixErrorf(file, plabel.Pos, err, "invalid enum label name %s", plabel.Name)
				return nil
			}
			ret.def.LabelDoc[index] = plabel.Doc
			ret.def.LabelDocSuffix[index] = plabel.DocSuffix
		}
	case *parse.TypeStruct:
		ret.def.FieldDoc = make([]string, len(pt.Fields))
		ret.def.FieldDocSuffix = make([]string, len(pt.Fields))
		for index, pfield := range pt.Fields {
			_, err := ValidIdent(pfield.Name)
			if err != nil {
				td.env.prefixErrorf(file, pfield.Pos, err, "invalid struct field name %s", pfield.Name)
				return nil
			}
			ret.def.FieldDoc[index] = pfield.Doc
			ret.def.FieldDocSuffix[index] = pfield.DocSuffix
		}
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
			if tbase == ErrorType {
				td.env.errorf(file, def.Pos, "error cannot be renamed")
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

// compileDefinedType compiles ptype.  It can handle definitions based on enum,
// struct and oneof, as well as definitions based on any literal type.
func compileDefinedType(ptype parse.Type, file *File, env *Env, tbuilder *vdl.TypeBuilder, builders map[string]*typeDefBuilder) vdl.TypeOrPending {
	switch pt := ptype.(type) {
	case *parse.TypeEnum:
		env.experimentalOnly(file, pt.Pos(), "enum not supported")
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
	case *parse.TypeOneOf:
		env.experimentalOnly(file, pt.Pos(), "oneof not supported")
		oneof := tbuilder.OneOf()
		for _, ptype := range pt.Types {
			otype := compileLiteralType(ptype, file, env, tbuilder, builders)
			if otype == nil {
				return nil
			}
			oneof.AppendType(otype)
		}
		return oneof
	}
	return compileLiteralType(ptype, file, env, tbuilder, builders)
}

// compileLiteralType compiles ptype.  It can handle any literal type.  Note
// that enum, struct and oneof are required to be defined and named, and aren't
// allowed as regular literal types.
func compileLiteralType(ptype parse.Type, file *File, env *Env, tbuilder *vdl.TypeBuilder, builders map[string]*typeDefBuilder) vdl.TypeOrPending {
	switch pt := ptype.(type) {
	case *parse.TypeNamed:
		if def, _ := env.ResolveType(pt.Name, file); def != nil {
			return def.Type
		}
		if b, ok := builders[pt.Name]; ok {
			return b.pending
		}
		env.errorf(file, pt.Pos(), "type %s undefined", pt.Name)
	case *parse.TypeArray:
		elem := compileLiteralType(pt.Elem, file, env, tbuilder, builders)
		if elem != nil {
			return tbuilder.Array().AssignLen(pt.Len).AssignElem(elem)
		}
	case *parse.TypeList:
		elem := compileLiteralType(pt.Elem, file, env, tbuilder, builders)
		if elem != nil {
			return tbuilder.List().AssignElem(elem)
		}
	case *parse.TypeSet:
		key := compileLiteralType(pt.Key, file, env, tbuilder, builders)
		if key != nil {
			return tbuilder.Set().AssignKey(key)
		}
	case *parse.TypeMap:
		key := compileLiteralType(pt.Key, file, env, tbuilder, builders)
		elem := compileLiteralType(pt.Elem, file, env, tbuilder, builders)
		if key != nil && elem != nil {
			return tbuilder.Map().AssignKey(key).AssignElem(elem)
		}
	default:
		env.errorf(file, pt.Pos(), "unnamed %s type invalid (type must be defined)", ptype.Kind())
	}
	return nil
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

var (
	// Built-in types defined by the compiler.
	// TODO(toddw): Represent error in a built-in VDL file.
	ErrorType = vdl.NamedType("error", vdl.StructType(
		vdl.StructField{"Id", vdl.StringType},
		vdl.StructField{"Msg", vdl.StringType},
	))
)
