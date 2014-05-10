package vom

import (
	"bytes"
	"fmt"
	"reflect"
	"sync"
)

// Type is an interface that parallels reflect.Type, but using VOM.Type objects. Some functionality
// is less complete than reflect.Type. See reflect.Type for method documentation.
// The advantage of VOM.Type is that it is more flexible than reflect.Type as it allows creation of
// struct and array types. The key reason that this is needed is that there is a unexported method
// in the reflect.Type interface which prevents us from implementing that interface with struct and
// array types.
type Type interface {
	Len() int
	Elem() Type
	Key() Type
	Kind() reflect.Kind
	ConvertibleTo(other Type) bool
	AssignableTo(other Type) bool
	NumMethod() int
	NumField() int
	Field(i int) StructField
	MethodByName(name string) (reflect.Method, bool)
	Name() string
	PkgPath() string
	String() string
}

// global state used for hash consing of custom types (non-vReflectType - vArrayType, vMapType, ...).
// vStructType is an exception and is hash consed with structTypeCache.
var customTypeCacheLock sync.Mutex
var customTypeCache map[interface{}]Type = make(map[interface{}]Type)

// reflectTypeCacheLock controls access to vReflectTypeCache and seenReflectTypeCache
var reflectTypeCacheLock sync.Mutex

// vReflectTypeCache caches the constructed vReflectType for a given reflect.Type for hash
// consing
var vReflectTypeCache map[reflect.Type]*vReflectType = make(map[reflect.Type]*vReflectType)

// seenReflectTypeCache caches seen reflect.Types and their corresponding non-vReflectType
// counterpart to enable the discovery of matching reflect.Types for non-vReflectTypes
var seenReflectTypeCache map[Type]reflect.Type = make(map[Type]reflect.Type)

// global state used for hash consing of struct types
var structTypeCacheLock sync.Mutex
var structTypeCache *structSet = newStructSet()

// hashConsCustomType returns an equivalent type to typ, but globally deduplicated so that hashconsd
// types pointers can be compared. vReflectTypes are hash consed in newVReflectType (for
// performance)
// REQUIRES: all Type subexpressions of <typ> are already hash-consed
func hashConsCustomType(typ Type) Type {
	if typ == nil {
		return nil
	}

	if st, ok := typ.(*vStructType); ok {
		structTypeCacheLock.Lock()
		ret := structTypeCache.HashCons(st)
		structTypeCacheLock.Unlock()
		return ret
	}

	val := reflect.ValueOf(typ).Elem().Interface()
	customTypeCacheLock.Lock()
	if hcTyp, ok := customTypeCache[val]; ok {
		customTypeCacheLock.Unlock()
		return hcTyp
	}
	customTypeCache[val] = typ
	customTypeCacheLock.Unlock()
	return typ
}

// vReflectType acts as a reflect.Type. Only primitives may be represented as vReflectType objects.
// Structured data (maps, slices, pointers, etc) must not be put in vReflectType objects.
type vReflectType struct {
	rt reflect.Type
}

// newVReflectType returns a hash consed Type object corresponding to the specified reflect.Type.
func newVReflectType(rt reflect.Type) *vReflectType {
	if rt == nil {
		return nil
	}

	reflectTypeCacheLock.Lock()
	if typ, ok := vReflectTypeCache[rt]; ok {
		reflectTypeCacheLock.Unlock()
		return typ
	}
	reflectTypeCacheLock.Unlock()

	var extraType Type
	switch rt.Kind() {
	case reflect.Struct:
		fields := make([]StructField, rt.NumField())
		for i := 0; i < rt.NumField(); i++ {
			fields[i] = NewStructField(rt.Field(i))
		}
		extraType = StructOf(fields)
	case reflect.Array:
		extraType = ArrayOf(rt.Len(), newVReflectType(rt.Elem()))
	}

	reflectTypeCacheLock.Lock()
	if typ, ok := vReflectTypeCache[rt]; ok {
		reflectTypeCacheLock.Unlock()
		return typ
	}
	typ := &vReflectType{rt: rt}
	vReflectTypeCache[rt] = typ
	if extraType != nil {
		seenReflectTypeCache[extraType] = rt
	}
	reflectTypeCacheLock.Unlock()
	return typ
}

func (rt *vReflectType) Kind() reflect.Kind {
	return rt.rt.Kind()
}

func (rt *vReflectType) Elem() Type {
	return ToVomType(rt.rt.Elem())
}

func (rt *vReflectType) Key() Type {
	return ToVomType(rt.rt.Key())
}

func (rt *vReflectType) NumField() int {
	return rt.rt.NumField()
}

func (rt *vReflectType) Len() int {
	return rt.rt.Len()
}

func (rt *vReflectType) Field(i int) StructField {
	return NewStructField(rt.rt.Field(i))
}

func (rt *vReflectType) ConvertibleTo(other Type) bool {
	switch other.(type) {
	case *vReflectType:
		return rt.rt.ConvertibleTo(other.(*vReflectType).rt)
	default:
		return false
	}
}

func (rt *vReflectType) AssignableTo(other Type) bool {
	switch other.(type) {
	case *vReflectType:
		return rt.rt.AssignableTo(other.(*vReflectType).rt)
	default:
		return false
	}
}

func (rt *vReflectType) NumMethod() int {
	return rt.rt.NumMethod()
}

func (rt *vReflectType) MethodByName(name string) (reflect.Method, bool) {
	return rt.rt.MethodByName(name)
}

func (rt *vReflectType) Name() string {
	return rt.rt.Name()
}

func (rt *vReflectType) PkgPath() string {
	return rt.rt.PkgPath()
}

func (rt *vReflectType) String() string {
	return rt.rt.String()
}

// customTypeBase defines methods and data that are common to non vReflectType type objects
type customTypeBase struct {
}

func (*customTypeBase) Field(i int) StructField { panic("Not defined") }
func (*customTypeBase) Key() Type               { panic("Not defined") }
func (*customTypeBase) Len() int                { panic("Not defined") }
func (*customTypeBase) NumField() int           { panic("Not defined") }
func (*customTypeBase) Elem() Type              { panic("Not defined") }

func (*customTypeBase) Name() string {
	return "" // TODO(bprosnitz) evaluate whether we need to add a name field for customTypeBase
}

func (*customTypeBase) PkgPath() string {
	return "" // TODO(bprosnitz) evaluate whether we need to add a pkgPath field for customTypeBase
}

func (*customTypeBase) MethodByName(name string) (reflect.Method, bool) {
	return reflect.Method{}, false // don't support methods on custom types
}

func (*customTypeBase) NumMethod() int {
	return 0 // don't support methods on custom types
}

// vPointerType represents the type of a pointer
type vPointerType struct {
	customTypeBase
	elem Type // elem is nil if the pointer is nil
}

func (rt *vPointerType) Elem() Type {
	return rt.elem
}

func (rt *vPointerType) Kind() reflect.Kind {
	return reflect.Ptr
}

func (rt *vPointerType) String() string {
	return "*" + rt.elem.String()
}

func (rt *vPointerType) AssignableTo(targ Type) bool {
	return Type(rt) == targ
}

func (rt *vPointerType) ConvertibleTo(targ Type) bool {
	switch targ.(type) {
	case *vPointerType:
		return rt.AssignableTo(targ) || rt.elem.AssignableTo(targ.(*vPointerType).elem)
	default:
		return false
	}
}

// vMapType represents the type of a map
type vMapType struct {
	customTypeBase
	key Type
	val Type
}

func (rt *vMapType) Elem() Type {
	return rt.val
}

func (rt *vMapType) Key() Type {
	return rt.key
}

func (rt *vMapType) Kind() reflect.Kind {
	return reflect.Map
}

func (rt *vMapType) String() string {
	return "map[" + rt.key.String() + "]" + rt.val.String()
}

func (rt *vMapType) AssignableTo(targ Type) bool {
	return Type(rt) == targ
}

func (rt *vMapType) ConvertibleTo(targ Type) bool {
	return rt.AssignableTo(targ)
}

// vStructType represents the type of a struct
type vStructType struct {
	customTypeBase
	fields []StructField
}

func (rt *vStructType) Field(index int) StructField {
	return rt.fields[index]
}

func (rt *vStructType) NumField() int {
	return len(rt.fields)
}

func (rt *vStructType) Kind() reflect.Kind {
	return reflect.Struct
}

func (rt *vStructType) String() string {
	var buf bytes.Buffer
	buf.WriteString("struct {")
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		var connector string
		if i > 0 {
			connector = ";"
		}
		buf.WriteString(fmt.Sprintf("%v %v %v", connector, rt.Field(i).Name, f.Type.String()))
	}
	buf.WriteString(" }")
	return buf.String()
}

func (rt *vStructType) AssignableTo(targ Type) bool {
	return Type(rt) == targ
}

func (rt *vStructType) ConvertibleTo(targ Type) bool {
	return rt.AssignableTo(targ)
}

// vArrayType represents the type of an array
type vArrayType struct {
	customTypeBase
	elem  Type
	count int
}

func (rt *vArrayType) Elem() Type {
	return rt.elem
}

func (rt *vArrayType) Len() int {
	return rt.count
}

func (rt *vArrayType) Kind() reflect.Kind {
	return reflect.Array
}

func (rt *vArrayType) String() string {
	return fmt.Sprintf("[%v]%v", rt.Len(), rt.Elem().String())
}

func (rt *vArrayType) AssignableTo(targ Type) bool {
	return Type(rt) == targ
}

func (rt *vArrayType) ConvertibleTo(targ Type) bool {
	return rt.AssignableTo(targ)
}

// vSliceType represents the type of a slice
type vSliceType struct {
	customTypeBase
	elem Type
}

func (rt *vSliceType) Elem() Type {
	return rt.elem
}

func (rt *vSliceType) Kind() reflect.Kind {
	return reflect.Slice
}

func (rt *vSliceType) String() string {
	return "[]" + rt.Elem().String()
}

func (rt *vSliceType) AssignableTo(targ Type) bool {
	return Type(rt) == targ
}

func (rt *vSliceType) ConvertibleTo(targ Type) bool {
	return rt.AssignableTo(targ)
}

// ToVomType converts a reflect.Type object to a Type
func ToVomType(typ reflect.Type) Type {
	if typ == nil {
		return nil
	}
	return newVReflectType(typ)
}

// ReflectTypeConvertable is a type that can be converted to a reflect type.
type ReflectTypeConvertable interface {
	ToReflectType() (reflect.Type, error)
}

// ToReflectType attempts to convert a Type object to a reflect.Type. If this is not possible (type
// is not a *vReflectType), an error is returned.
func ToReflectType(typ Type) (reflect.Type, error) {
	if typ == nil {
		return nil, nil
	}
	if vrt, ok := typ.(*vReflectType); ok {
		return vrt.rt, nil
	}

	if rtc, ok := typ.(ReflectTypeConvertable); ok {
		if typ, err := rtc.ToReflectType(); err == nil {
			return typ, nil
		}
	}

	return nil, fmt.Errorf("not possible to convert %v to reflect.Type (type: %v)", typ, reflect.TypeOf(typ))
}

// TypeOf returns a Type object representing the type of the value
func TypeOf(val interface{}) Type {
	rt := reflect.TypeOf(val)
	return ToVomType(rt)
}

// PtrTo returns a pointer to the specified type
func PtrTo(t Type) Type {
	if _, ok := t.(*vReflectType); ok {
		return newVReflectType(reflect.PtrTo(t.(*vReflectType).rt))
	}
	return hashConsCustomType(&vPointerType{elem: t})
}

// SliceOf returns a slice type containing the specified type
func SliceOf(t Type) Type {
	if _, ok := t.(*vReflectType); ok {
		return newVReflectType(reflect.SliceOf(t.(*vReflectType).rt))
	}
	return hashConsCustomType(&vSliceType{elem: t})
}

// ArrayOf returns an array type containing the specified type
func ArrayOf(l int, t Type) Type {
	arr := hashConsCustomType(&vArrayType{elem: t, count: l})

	// Now try to find a matching vReflectType and return it if possible, otherwise return arr.
	reflectTypeCacheLock.Lock()
	if rt, ok := seenReflectTypeCache[arr]; ok {
		arr = vReflectTypeCache[rt]
	}
	reflectTypeCacheLock.Unlock()
	return arr
}

// MapOf returns a map type with the specified key and value types
func MapOf(k, v Type) Type {
	krt, kok := k.(*vReflectType)
	vrt, vok := v.(*vReflectType)
	if kok && vok {
		return newVReflectType(reflect.MapOf(krt.rt, vrt.rt))
	}
	return hashConsCustomType(&vMapType{key: k, val: v})
}

// StructOf returns a struct with fields of the specified types
func StructOf(fields []StructField) Type {
	st := hashConsCustomType(&vStructType{fields: fields})

	// Now try to find a matching vReflectType and return it if possible, otherwise return st.
	reflectTypeCacheLock.Lock()
	if rt, ok := seenReflectTypeCache[st]; ok {
		ret := vReflectTypeCache[rt]
		reflectTypeCacheLock.Unlock()
		return ret
	}
	reflectTypeCacheLock.Unlock()
	return st
}

// StructField holds information about struct fields
type StructField struct {
	Name    string
	PkgPath string
	Type    Type
}

// NewStructField creates a VOM struct field object from a reflect.StructField
func NewStructField(sf reflect.StructField) StructField {
	return StructField{
		Name:    sf.Name,
		PkgPath: sf.PkgPath,
		Type:    ToVomType(sf.Type),
	}
}
