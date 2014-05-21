package compile

import (
	"fmt"

	"veyron2/val"
	"veyron2/vdl/parse"
)

// TypeDef represents a user-defined named type definition in the compiled
// results.
type TypeDef struct {
	NamePos           // name, parse position and docs
	Type    *val.Type // type of this type definition

	// BaseType is the type that Type is based on.  The BaseType may be named or
	// unnamed.  The base type is nil if it corresponds to an enum, struct or
	// oneof literal, simply because val.Type doesn't allow these to be unnamed.
	// E.g.
	//                                 BaseType
	//   type Bool    bool;            bool
	//   type Bool2   Bool;            Bool
	//   type List    []int32;         []int32
	//   type List2   List;            List
	//   type Struct  struct{A bool};  nil
	//   type Struct2 Struct;          Struct
	BaseType *val.Type

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
		tbuilder: &val.TypeBuilder{},
		builders: make(map[string]*typeBuilder),
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
// It holds a builders map from type name to typeBuilder, where the typeBuilder
// is responsible for compiling and defining a single type.
type typeDefiner struct {
	pkg      *Package
	pfiles   []*parse.File
	env      *Env
	tbuilder *val.TypeBuilder
	builders map[string]*typeBuilder
}

type typeBuilder struct {
	def     *TypeDef
	ptype   parse.Type
	pending val.PendingType // pending type that we're building
	base    val.PendingType // base type that pending is based on, may be nil
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
			td.builders[pdef.Name] = td.makeTypeBuilder(file, pdef)
		}
	}
}

func (td typeDefiner) makeTypeBuilder(file *File, pdef *parse.TypeDef) *typeBuilder {
	if err := ValidIdent(pdef.Name); err != nil {
		td.env.prefixErrorf(file, pdef.Pos, err, "type %s invalid name", pdef.Name)
		return nil
	}
	ret := new(typeBuilder)
	ret.def = &TypeDef{NamePos: NamePos(pdef.NamePos), File: file}
	ret.ptype = pdef.Type
	// We use the qualified name to actually name the type, to ensure types
	// defined in separate packages are hash-consed separately.
	qname := file.Package.QualifiedName(pdef.Name)
	switch pt := pdef.Type.(type) {
	case *parse.TypeNamed, *parse.TypeArray, *parse.TypeList, *parse.TypeMap:
		ret.pending = td.tbuilder.Named(qname)
	case *parse.TypeStruct:
		ret.pending = td.tbuilder.Struct(qname)
		ret.def.FieldDoc = make([]string, len(pt.Fields))
		ret.def.FieldDocSuffix = make([]string, len(pt.Fields))
		for index, pfield := range pt.Fields {
			ret.def.FieldDoc[index] = pfield.Doc
			ret.def.FieldDocSuffix[index] = pfield.DocSuffix
		}
		// TODO(toddw): Implement Enum, OneOf
	default:
		td.env.errorf(file, pt.Pos(), "type %s invalid (type definition can't be based on %s type)", pdef.Name, pt.Kind())
	}
	return ret
}

// Define uses the builders to describe each type.  Named types defined in
// other packages must have already been compiled, and in env.  Named types
// defined in this package are represented by the builders.
func (td typeDefiner) Define() {
	for _, b := range td.builders {
		def, file := b.def, b.def.File
		switch pt := b.ptype.(type) {
		case *parse.TypeNamed, *parse.TypeArray, *parse.TypeList, *parse.TypeMap:
			if base := compilePendingType(pt, file, td.env, td.tbuilder, td.builders); base != nil {
				switch tbase := base.(type) {
				case *val.Type:
					if tbase == TypeError {
						td.env.errorf(file, def.Pos, "error cannot be renamed")
						continue // keep going to catch more errors
					}
					def.BaseType = tbase
				case val.PendingType:
					b.base = tbase
				default:
					panic(fmt.Errorf("vdl: typeDefiner.Define unhandled TypeOrPending %T %v", tbase, tbase))
				}
				b.pending.(val.PendingNamed).SetBase(base)
			}
		case *parse.TypeStruct:
			for _, pfield := range pt.Fields {
				if ftype := compilePendingType(pfield.Type, file, td.env, td.tbuilder, td.builders); ftype != nil {
					b.pending.(val.PendingStruct).AppendField(pfield.Name, ftype)
				}
			}
		default:
			// This type switch must mirror the types in makeTypeBuilder.
			panic(fmt.Errorf("vdl: unhandled parse.Type %T %#v", pt, pt))
		}
	}
}

// compileType returns the *val.Type corresponding to ptype.  All named types
// referenced by ptype must already be defined.
func compileType(ptype parse.Type, file *File, env *Env) *val.Type {
	var tbuilder val.TypeBuilder
	typeOrPending := compilePendingType(ptype, file, env, &tbuilder, nil)
	tbuilder.Build()
	switch top := typeOrPending.(type) {
	case nil:
		return nil
	case *val.Type:
		return top
	case val.PendingType:
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

// compilePendingType returns the *val.Type corresponding to ptype if it is
// already defined, or a PendingType from builders if it is currently being
// built.
func compilePendingType(ptype parse.Type, file *File, env *Env, tbuilder *val.TypeBuilder, builders map[string]*typeBuilder) val.TypeOrPending {
	switch pt := ptype.(type) {
	case *parse.TypeNamed:
		if def := env.ResolveType(pt.Name, file); def != nil {
			return def.Type
		}
		if b, ok := builders[pt.Name]; ok {
			return b.pending
		}
		env.errorf(file, pt.Pos(), "type %s undefined", pt.Name)
	case *parse.TypeArray:
		env.errorf(file, pt.Pos(), "arrays are not supported and will be removed")
	case *parse.TypeList:
		elem := compilePendingType(pt.Elem, file, env, tbuilder, builders)
		if elem != nil {
			return tbuilder.List().SetElem(elem)
		}
	case *parse.TypeMap:
		key := compilePendingType(pt.Key, file, env, tbuilder, builders)
		elem := compilePendingType(pt.Elem, file, env, tbuilder, builders)
		if key != nil && elem != nil {
			return tbuilder.Map().SetKey(key).SetElem(elem)
		}
	default:
		env.errorf(file, pt.Pos(), "unnamed %s type invalid (type must be defined)", ptype.Kind())
	}
	return nil
}

// Build actually builds each type and updates the package with the typedefs.
// The order we call each pending type doesn't matter; the veyron2/val package
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
		// env should only be nil during initialization of the global package;
		// NewEnv ensures new environments have the global types.
		env.typeDefs[def.Type] = def
	}
}

var (
	// The GlobalPackage and GlobalFile are used to hold the built-in types.
	GlobalPackage = newPackage("", "global")
	GlobalFile    = &File{BaseName: "global.vdl"}

	// Built-in types defined by the compiler.
	// TODO(toddw): Remove byte from the VDL language.
	TypeByte = val.NamedType("byte", val.Uint32Type)
	// TODO(toddw): Represent error in a built-in VDL file.
	TypeError = val.StructType("error", []val.StructField{{"Id", val.StringType}, {"Msg", val.StringType}})
)

func init() {
	GlobalPackage.Files = []*File{GlobalFile}
	GlobalFile.Package = GlobalPackage
	// The built-in types may only be initialized after the Global{Package,File}
	// are linked to each other.
	globalSingleton("any", val.AnyType)
	globalSingleton("bool", val.BoolType)
	globalSingleton("int32", val.Int32Type)
	globalSingleton("int64", val.Int64Type)
	globalSingleton("uint32", val.Uint32Type)
	globalSingleton("uint64", val.Uint64Type)
	globalSingleton("float32", val.Float32Type)
	globalSingleton("float64", val.Float64Type)
	globalSingleton("complex64", val.Complex64Type)
	globalSingleton("complex128", val.Complex128Type)
	globalSingleton("string", val.StringType)
	globalSingleton("bytes", val.BytesType)
	globalSingleton("typeval", val.TypeValType)
	globalSingleton("byte", TypeByte)
	globalSingleton("error", TypeError)
}

func globalSingleton(name string, t *val.Type) {
	def := &TypeDef{
		NamePos: NamePos{Name: name},
		Type:    t,
		File:    GlobalFile,
	}
	addTypeDef(def, nil)
}
