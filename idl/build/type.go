package build

import (
	"fmt"
	"reflect"
)

// Kind represents the kind of a type - all primitives and each kind of
// composite type are represented.  This makes it easy to handle each type based
// on its raw structure, regardless of whether it's named.
type Kind int

const (
	// Kinds of primitive types.
	KindBool Kind = iota
	KindByte
	KindUint32
	KindUint64
	KindInt32
	KindInt64
	KindFloat32
	KindFloat64
	KindComplex64
	KindComplex128
	KindString
	KindError
	KindAnyData
	// Kinds of composite types.
	KindArray
	KindSlice
	KindMap
	KindStruct
	KindInterface
)

func (k Kind) String() string {
	switch k {
	case KindBool:
		return "bool"
	case KindByte:
		return "byte"
	case KindUint32:
		return "uint32"
	case KindUint64:
		return "uint64"
	case KindInt32:
		return "int32"
	case KindInt64:
		return "int64"
	case KindFloat32:
		return "float32"
	case KindFloat64:
		return "float64"
	case KindComplex64:
		return "complex64"
	case KindComplex128:
		return "complex128"
	case KindString:
		return "string"
	case KindError:
		return "error"
	case KindAnyData:
		return "anydata"
	case KindArray:
		return "array"
	case KindSlice:
		return "slice"
	case KindMap:
		return "map"
	case KindStruct:
		return "struct"
	case KindInterface:
		return "interface"
	default:
		panic(fmt.Errorf("idl: unhandled kind %d", k))
	}
}

func (k Kind) Name() string {
	switch k {
	case KindBool:
		return "KindBool"
	case KindByte:
		return "KindByte"
	case KindUint32:
		return "KindUint32"
	case KindUint64:
		return "KindUint64"
	case KindInt32:
		return "KindInt32"
	case KindInt64:
		return "KindInt64"
	case KindFloat32:
		return "KindFloat32"
	case KindFloat64:
		return "KindFloat64"
	case KindComplex64:
		return "KindComplex64"
	case KindComplex128:
		return "KindComplex128"
	case KindString:
		return "KindString"
	case KindError:
		return "KindError"
	case KindAnyData:
		return "KindAnyData"
	case KindArray:
		return "KindArray"
	case KindSlice:
		return "KindSlice"
	case KindMap:
		return "KindMap"
	case KindStruct:
		return "KindStruct"
	case KindInterface:
		return "KindInterface"
	default:
		panic(fmt.Errorf("idl: unhandled kind %d", k))
	}
}

// IsPrimitive returns true iff the kind is a primitive (non-composite) type.
func (k Kind) IsPrimitive() bool {
	switch k {
	case KindArray, KindSlice, KindMap, KindStruct, KindInterface, KindError, KindAnyData:
		return false
	}
	return true
}

// Type is an interface representing symbolic occurrences of types in IDL files.
type Type interface {
	// Name returns the name of the type.
	Name() string

	// Def returns the type definition associated with the type.
	Def() *TypeDef

	// resolve traverses the type and resolves in-place all named types to their
	// underlying TypeDef.  After resolve returns, the type and all subtypes will
	// have associated TypeDefs.
	resolve(pkgs pkgMap, thisFile *File) error

	// namedDeps returns the named TypeDefs that this type depends on.  The deps
	// aren't transitive past named TypeDefs; if A depends on B which depends on
	// C, the deps of A are B (but not C).
	namedDeps() typeDefSet
}

// NamedType captures named types during parsing; both built-in primitives and
// user-defined named types become NamedType.  The PackageName may be empty,
// which indicates the type is defined in either the global package, or in the
// local package that's being compiled.
type NamedType struct {
	PackageName string
	TypeName    string
	Pos         Pos
	TypeDef     *TypeDef // filled in via resolve()
	// WantInterface specifies whether the named type is supposed to resolve
	// to an interface type or to a non-interface type.  We need to track this
	// since inside interface specs, we only want names for embedded interfaces;
	// and in any other context, we currently do not allow interface types.
	WantInterface bool
}

func (t *NamedType) Name() string {
	if t.PackageName != "" {
		return t.PackageName + "." + t.TypeName
	}
	return t.TypeName
}

func (t *NamedType) Def() *TypeDef {
	return t.TypeDef
}

func (t *NamedType) resolve(pkgs pkgMap, thisFile *File) error {
	if t.TypeDef != nil {
		return nil
	}
	if t.TypeDef = t.lookupTypeDef(pkgs, thisFile); t.TypeDef == nil {
		return fmt.Errorf("undefined type %s", t.Name())
	} else if t.WantInterface && t.TypeDef.Kind != KindInterface {
		return fmt.Errorf("expecting interface type for %s; got type %s instead", t.Name(), t.TypeDef.Kind)
	} else if !t.WantInterface && t.TypeDef.Kind == KindInterface {
		return fmt.Errorf("expecting non-interface type for %s; got type %s instead", t.Name(), t.TypeDef.Kind)
	}
	return nil
}

// lookupTypeDef looks up the TypeDef for the named type.
func (t *NamedType) lookupTypeDef(pkgs pkgMap, thisFile *File) *TypeDef {
	if t.PackageName == "" {
		// The name is either in the global package, or in this package.  The order
		// of the lookups doesn't matter; we don't allow name collisions with the
		// global package.
		if def := globalPackage.typeResolver[t.TypeName]; def != nil {
			return def
		}
		return thisFile.Package.typeResolver[t.TypeName]
	}
	// Translate the package name into its path and lookup in pkgs.
	pkgPath := thisFile.LookupImportPath(t.PackageName)
	if pkgPath == "" {
		return nil
	}
	if pkg, pkgExists := pkgs[pkgPath]; pkgExists {
		return pkg.typeResolver[t.TypeName]
	}
	return nil
}

func (t *NamedType) namedDeps() typeDefSet {
	if t.TypeDef == nil {
		panic(fmt.Errorf("idl: namedDeps called on unresolved type %s at pos %v", t.Name(), t.Pos))
	}
	return typeDefSet{t.TypeDef: true}
}

// ArrayType represents arrays.
type ArrayType struct {
	Len     int
	Elem    Type
	TypeDef *TypeDef // filled in via resolve()
}

func (t *ArrayType) Name() string {
	return fmt.Sprintf("[%v]%v", t.Len, t.Elem.Name())
}

func (t *ArrayType) Def() *TypeDef {
	return t.TypeDef
}

func (t *ArrayType) resolve(pkgs pkgMap, thisFile *File) error {
	if t.TypeDef != nil {
		return nil
	}
	if err := t.Elem.resolve(pkgs, thisFile); err != nil {
		return err
	}
	elem := TypeDefType{t.Elem.Def()}
	t.TypeDef = globalComposite(&ArrayType{t.Len, elem, nil}, KindArray)
	return nil
}

func (t *ArrayType) namedDeps() typeDefSet {
	return t.Elem.namedDeps()
}

// SliceType represents slices.
type SliceType struct {
	Elem    Type
	TypeDef *TypeDef // filled in via resolve()
}

func (t *SliceType) Name() string {
	return fmt.Sprintf("[]%v", t.Elem.Name())
}

func (t *SliceType) Def() *TypeDef {
	return t.TypeDef
}

func (t *SliceType) resolve(pkgs pkgMap, thisFile *File) error {
	if t.TypeDef != nil {
		return nil
	}
	if err := t.Elem.resolve(pkgs, thisFile); err != nil {
		return err
	}
	elem := TypeDefType{t.Elem.Def()}
	t.TypeDef = globalComposite(&SliceType{elem, nil}, KindSlice)
	return nil
}

func (t *SliceType) namedDeps() typeDefSet {
	return t.Elem.namedDeps()
}

// MapType represents unordered maps.
type MapType struct {
	Key     Type
	Elem    Type
	TypeDef *TypeDef // filled in via resolve()
}

func (t *MapType) Name() string {
	return fmt.Sprintf("map[%v]%v", t.Key.Name(), t.Elem.Name())
}

func (t *MapType) Def() *TypeDef {
	return t.TypeDef
}

func (t *MapType) resolve(pkgs pkgMap, thisFile *File) error {
	if t.TypeDef != nil {
		return nil
	}
	if err := t.Key.resolve(pkgs, thisFile); err != nil {
		return err
	}
	if err := t.Elem.resolve(pkgs, thisFile); err != nil {
		return err
	}
	key := TypeDefType{t.Key.Def()}
	elem := TypeDefType{t.Elem.Def()}
	t.TypeDef = globalComposite(&MapType{key, elem, nil}, KindMap)
	return nil
}

func (t *MapType) namedDeps() typeDefSet {
	return mergeTypeDefSets(t.Key.namedDeps(), t.Elem.namedDeps())
}

// InterfaceType is a type that describes an interface.
type InterfaceType struct {
	TypeDef *TypeDef
}

// Name implements the Type interface.
func (t *InterfaceType) Name() string {
	return "interface"
}

// Def implements the Type interface.
func (t *InterfaceType) Def() *TypeDef {
	return t.TypeDef
}

// resolve implements the Type interface.
func (t *InterfaceType) resolve(pkgs pkgMap, thisFile *File) error {
	if t.TypeDef != nil {
		return nil
	}
	t.TypeDef = &TypeDef{Kind: KindInterface}
	return nil
}

// namedDeps implements the Type interface.
func (t *InterfaceType) namedDeps() (r typeDefSet) {
	return
}

// StructType represents structs; a collection of fields.
type StructType struct {
	Fields  []*Field
	TypeDef *TypeDef // filled in via resolve()
}

func (t *StructType) Name() string {
	result := "struct{"
	for fx, field := range t.Fields {
		if fx > 0 {
			result += ";"
		}
		result += field.Name + " " + field.Type.Name()
	}
	result += "}"
	return result
}

func (t *StructType) Def() *TypeDef {
	return t.TypeDef
}

func (t *StructType) resolve(pkgs pkgMap, thisFile *File) error {
	if t.TypeDef != nil {
		return nil
	}
	tdStruct := &StructType{make([]*Field, len(t.Fields)), nil}
	for fx, field := range t.Fields {
		if err := field.Type.resolve(pkgs, thisFile); err != nil {
			return err
		}
		tdStruct.Fields[fx] = &Field{Name: field.Name, Type: TypeDefType{field.Type.Def()}}
	}
	t.TypeDef = globalComposite(tdStruct, KindStruct)
	return nil
}

func (t *StructType) namedDeps() (r typeDefSet) {
	for _, field := range t.Fields {
		r = mergeTypeDefSets(r, field.Type.namedDeps())
	}
	return r
}

// typeResolver converts a string type identifier into a resolved TypeDef.
type typeResolver map[string]*TypeDef

// TypeDef represents a type definition.  It does not implement Type, but it has
// a Base field which contains the base type for the typedef.  Typedefs are
// hash-consed; each unique type has exactly one *TypeDef that represents it, so
// pointer-equality may be used to check for identical types.
//
// User-defined named type definitions have typedefs associated with the package
// in which they are defined.  They always have a non-empty name, and the base
// type contains the information from the parse; e.g. struct field comments are
// contained in the base type.
//
// Unnamed composite types and the built-in primitives have typedefs associated
// with the global package.  All such types with the same underlying structure
// are represented by the same typedef.  The built-in primitives always have a
// non-empty name and an empty base type; the unnamed composite types always
// have an empty name and a non-empty base type representing the composite type.
//
// Each typedef may also be used as a regular Type by wrapping it in
// TypeDefType.  This is used to represent the built-in primitives, and the base
// type for unnamed composite types; it reduces redundant code to represent the
// structure of the composite type.
//
// Some examples:
//   package P
//   type Foo int32
//   type Bar struct {
//     // Bar field comments
//     A, B int32
//   }
//   type Baz struct {
//     // Baz field comments
//     A, B int32
//   }
//   type Iface interface {
//     Method(C int32, D *int32, E []Foo)
//   }
//
// The typedef for Foo is a named typedef in the P package, with name "Foo", and
// with a TypeDefType base type containing the globalInt32 typedef.  That is the
// same typedef used to represent the type for argument C in Method.
//
// The typedefs for Bar and Baz are distinct and in the P package, with their
// respective names.  Each has its own StructType base type, containing their
// respective comments.  Each StructType has the same typedef in the global
// package, and that typedef has a StructType base type describing the structure
// of the struct, without the comments.  Thus the typedefs for Bar and Baz are
// distinct, but the typedefs for their base type is the same.
//
// The typedefs for Method args D and E are unnamed composite types; neither has a
// name.  D has a PtrType base type, with a TypeDefType elem type containing the
// globalInt32 typedef.  E has a SliceType base type, with a TypeDefType elem
// type containing the Foo typedef.
type TypeDef struct {
	Name string // Name of the typedef; may be empty for unnamed types.
	Base Type   // Base type of the typedef.
	Kind Kind   // Kind of type.
	Pos  Pos    // Position for user-defined types, or invalid for unnamed types.
	File *File  // File the typedef occurs in; globalFile for unnamed types.
	Doc  string // Documentation string for the typedef.
}

type typeDefSet map[*TypeDef]bool

// mergeTypeDefSets returns the union of a and b.  It may mutate either a or b
// and return the mutated set as a result.
func mergeTypeDefSets(a, b typeDefSet) (r typeDefSet) {
	if a != nil {
		for def, _ := range b {
			a[def] = true
		}
		return a
	}
	return b
}

// declare declares the user-defined type in its package - it must be called
// before define().
func (def *TypeDef) declare() error {
	if exist := globalPackage.typeResolver[def.Name]; exist != nil {
		return fmt.Errorf("already defined in global package")
	}
	if exist := def.File.Package.typeResolver[def.Name]; exist != nil {
		return fmt.Errorf("already defined at %v:%v", exist.File.BaseName, exist.Pos)
	}
	// Declaring the type is the first phase - we put the unresolved def directly
	// into the typeResolver map, so the name and pointer are available.
	def.File.Package.typeResolver[def.Name] = def
	return nil
}

// define defines a user typedef in-place.  It resolves the base type to ensure
// it's a known type and all underlying typedefs have been set, and sets the
// kind of this typedef.
func (def *TypeDef) define(pkgs pkgMap) error {
	if exist := def.File.Package.typeResolver[def.Name]; exist != def {
		return fmt.Errorf("internal error, type declared with wrong name")
	}
	if err := def.Base.resolve(pkgs, def.File); err != nil {
		return err
	}
	def.Kind = def.Base.Def().Kind
	vlog.Printf("User typedef %s (%v)", def.String(), def.Kind)
	return nil
}

func (def *TypeDef) String() string {
	if def.File.Package == nil {
		// We're being called during the parse, so the Package isn't set up yet.
		return fmt.Sprintf("(%v %s %s)", def.Pos, def.Name, def.Base.Name())
	}
	if def.File.Package == globalPackage {
		if def.Name != "" {
			// Global primitives just use their name.
			return def.Name
		}
		// Global composites use the typename of their base type.
		return def.Base.Name()
	}
	// Otherwise this is a user TypeDef, and we want e.g. veyron/lib/idl.Foo.  We
	// prefix with the package path to ensure each global composite has a unique
	// typename for hash-consing.
	return def.File.Package.Path + "." + def.Name
}

func (def *TypeDef) namedDeps() typeDefSet {
	if def.Name != "" {
		// This typedef is named, so the dependencies stop here.
		return typeDefSet{def: true}
	}
	// Otherwise for unnamed typedefs we traverse the base type.
	return def.Base.namedDeps()
}

// TypeDefType allows a TypeDef to be used as a regular Type; it allows us to
// re-use the regular composite types to describe the structure of the base
// types for unnamed typedefs.  Technically we could just make TypeDef implement
// the Type methods, but this is more clear since it makes us explicitly declare
// where we're using TypeDef as a Type.
type TypeDefType struct {
	def *TypeDef
}

func (t TypeDefType) Name() string {
	return t.def.String()
}

func (t TypeDefType) Def() *TypeDef {
	return t.def
}

func (t TypeDefType) namedDeps() typeDefSet {
	return t.def.namedDeps()
}

func (t TypeDefType) resolve(pkgs pkgMap, thisFile *File) error {
	panic(fmt.Errorf("idl: resolve should never be called on TypeDefType, %s", t.Name()))
}

var (
	// Create a fake global file and package representing the unnamed primitive
	// and composite types.  The global package has an empty package name.
	globalPackage = &Package{typeResolver: make(typeResolver)}
	globalFile    = &File{BaseName: "global.idl", PackageName: globalPackage.Name, Package: globalPackage}
	// The primitive types are a restricted subset of the regular Go types - these
	// should be reasonable to represent in other languages as well.  Composite
	// types may be built using combinations of these primitives.
	//
	// We don't support int, uint, byte or rune - use explicit sizes instead.
	// We don't support uintptr, since raw pointers aren't allowed in the idl.
	// We don't allow interfaces to be passed as method arguments.
	globalBool       = globalPrimitive(KindBool)
	globalByte       = globalPrimitive(KindByte)
	globalUint32     = globalPrimitive(KindUint32)
	globalUint64     = globalPrimitive(KindUint64)
	globalInt32      = globalPrimitive(KindInt32)
	globalInt64      = globalPrimitive(KindInt64)
	globalFloat32    = globalPrimitive(KindFloat32)
	globalFloat64    = globalPrimitive(KindFloat64)
	globalComplex64  = globalPrimitive(KindComplex64)
	globalComplex128 = globalPrimitive(KindComplex128)
	globalString     = globalPrimitive(KindString)
	// The error type is known to the idl; we'll translate it into an appropriate
	// type when generating code for languages other than Go.
	globalError = globalPrimitive(KindError)
	// The anydata type is a special built-in type, representing a value of any
	// type.  In generated Go it's translated into idl.AnyData defined in
	// veyron/api/idl, with underlying type "interface{}".
	globalAnyData = globalPrimitive(KindAnyData)
)

func init() {
	globalPackage.Files = []*File{globalFile}
}

func addGlobalTypeDef(name string, def *TypeDef) *TypeDef {
	globalFile.TypeDefs = append(globalFile.TypeDefs, def)
	globalPackage.typeResolver[name] = def
	vlog.Printf("Global typedef %s (%v)", name, def.Kind)
	return def
}

func globalPrimitive(kind Kind) *TypeDef {
	// Typedefs for global primitives are named, but have no base type; they're
	// terminal nodes.
	name := kind.String()
	return addGlobalTypeDef(name, &TypeDef{Name: name, Kind: kind, File: globalFile})
}

func globalComposite(base Type, kind Kind) *TypeDef {
	// Typedefs for global composites aren't named, but have a base type.  We
	// register the typedef in the global package using the typename of the base
	// type, as a simple way to hash-cons these typedefs.  User-defined named
	// types include their package path; see TypeDef.String for details.
	name := base.Name()
	if def := globalPackage.typeResolver[name]; def != nil {
		return def
	}
	return addGlobalTypeDef(name, &TypeDef{Base: base, Kind: kind, File: globalFile})
}

func compositeTypeToIDLType(rt reflect.Type) Type {
	var ct Type
	switch rt.Kind() {
	case reflect.Array:
		childType := ReflectTypeToIDLType(rt.Elem())
		at := &ArrayType{Len: rt.Len(), Elem: childType}
		at.TypeDef = globalComposite(at, KindArray)
		ct = at
	case reflect.Slice:
		childType := ReflectTypeToIDLType(rt.Elem())
		st := &SliceType{Elem: childType}
		st.TypeDef = globalComposite(st, KindSlice)
		ct = st
	case reflect.Map:
		keyType := ReflectTypeToIDLType(rt.Key())
		elemType := ReflectTypeToIDLType(rt.Elem())
		mt := &MapType{Key: keyType, Elem: elemType}
		mt.TypeDef = globalComposite(mt, KindMap)
		ct = mt
	case reflect.Struct:
		fields := make([]*Field, 0)
		for i := 0; i < rt.NumField(); i++ {
			inField := rt.Field(i)
			if inField.Name[0] >= 'A' && inField.Name[0] <= 'Z' {
				fields = append(fields, &Field{
					Name: inField.Name,
					Type: ReflectTypeToIDLType(inField.Type),
				})
			}
		}
		st := &StructType{Fields: fields}
		st.TypeDef = globalComposite(st, KindStruct)
		ct = st
	default:
		panic(fmt.Sprintf("Attempting to convert unexpected type: %v", rt.Kind()))
	}

	if rt.Name() != "" {
		ct = &NamedType{TypeName: rt.Name(), PackageName: rt.PkgPath(), TypeDef: ct.Def()}
	}
	return ct
}

func basicTypeToIDLType(rt reflect.Type) Type {
	var nt *NamedType // Named types represent primitives
	switch rt.Kind() {
	case reflect.Interface:
		nt = &NamedType{TypeName: "interface", TypeDef: globalAnyData}
	case reflect.String:
		nt = &NamedType{TypeName: "string", TypeDef: globalString}
	case reflect.Uint8:
		nt = &NamedType{TypeName: "byte", TypeDef: globalByte}
	case reflect.Uint32:
		nt = &NamedType{TypeName: "uint32", TypeDef: globalUint32}
	case reflect.Uint64:
		nt = &NamedType{TypeName: "uint64", TypeDef: globalUint64}
	case reflect.Int32:
		nt = &NamedType{TypeName: "int32", TypeDef: globalInt32}
	case reflect.Int64:
		nt = &NamedType{TypeName: "int64", TypeDef: globalInt64}
	case reflect.Float32:
		nt = &NamedType{TypeName: "float32", TypeDef: globalFloat32}
	case reflect.Float64:
		nt = &NamedType{TypeName: "float64", TypeDef: globalFloat64}
	case reflect.Complex64:
		nt = &NamedType{TypeName: "complex64", TypeDef: globalComplex64}
	case reflect.Complex128:
		nt = &NamedType{TypeName: "complex128", TypeDef: globalComplex128}
	case reflect.Bool:
		nt = &NamedType{TypeName: "bool", TypeDef: globalBool}
	// Types dependent on GOOS and GOARCH:
	// We will choose the widest available width to represent these.
	case reflect.Uint:
		nt = &NamedType{TypeName: "uint", TypeDef: globalUint64}
	case reflect.Int:
		nt = &NamedType{TypeName: "int", TypeDef: globalInt64}
	case reflect.Uintptr:
		nt = &NamedType{TypeName: "uintptr", TypeDef: globalUint64}
	default:
		panic(fmt.Sprintf("Attempting to convert unsupported basic type: %v", rt.Kind()))
	}

	// If the reflect.Type has a name, set a name in the IDL type.
	if rt.Name() != "" {
		nt.PackageName = rt.PkgPath()
		nt.TypeName = rt.Name()
		nt.TypeDef = globalComposite(nt, nt.Def().Kind)
		if nt.TypeDef.Base != nil {
			nt = nt.TypeDef.Base.(*NamedType)
		}
	}
	nt.TypeDef.Base = nt
	return nt
}

// ReflectTypeToIDLType generates an IDL type representation for the given reflect type.
func ReflectTypeToIDLType(rt reflect.Type) Type {
	switch rt.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.Struct:
		return compositeTypeToIDLType(rt)
	case reflect.Ptr:
		panic("Pointer types not supported by the IDL")
	default:
		return basicTypeToIDLType(rt)
	}
}

func isByteSlice(def *TypeDef) bool {
	if def.Kind != KindSlice {
		return false
	}
	return def.Base.(*SliceType).Elem.Def().Kind == KindByte
}
