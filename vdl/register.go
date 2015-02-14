package vdl

import (
	"fmt"
	"reflect"
	"sync"
)

// Register registers a type, identified by a value for that type.  The type
// should be a type that will be sent over the wire; types defined in vdl files
// are automatically registered.  Subtypes are recursively registered.
//
// There are two reasons types must be registered:
//   1) To create a type name -> reflect.Type mapping, so that when we decode
//      into interface{}, we can generate values of the correct type.
//   2) To create a wire type <-> native type mapping, so that we can
//      auto-marshal native types.
//
// Panics if wire is not a valid wire type, or if any mappings are not
// bijective.  See ReflectInfo for details.
func Register(wire interface{}) {
	if wire == nil {
		return
	}
	if err := registerRecursive(reflect.TypeOf(wire)); err != nil {
		panic(err)
	}
}

func registerRecursive(rt reflect.Type) error {
	// 1) Normalize and derive reflect information.
	rt = normalizeType(rt)
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	ri, err := DeriveReflectInfo(rt)
	if err != nil {
		return err
	}
	// 2) Add reflect information to the registry.
	first, err := riReg.addReflectInfo(rt, ri)
	if err != nil {
		return err
	}
	if !first {
		// Break cycles for recursive types.
		return nil
	}
	// 3) Register subtypes, if this is the first time we've seen the type.
	//
	// Special-case to recurse on union fields.
	if len(ri.UnionFields) > 0 {
		for _, field := range ri.UnionFields {
			if err := registerRecursive(field.Type); err != nil {
				return err
			}
		}
		return nil
	}
	// Recurse on subtypes contained in regular composite types.
	switch wt := ri.WireType; wt.Kind() {
	case reflect.Array, reflect.Slice, reflect.Ptr:
		return registerRecursive(wt.Elem())
	case reflect.Map:
		if err := registerRecursive(wt.Key()); err != nil {
			return err
		}
		return registerRecursive(wt.Elem())
	case reflect.Struct:
		for ix := 0; ix < wt.NumField(); ix++ {
			if err := registerRecursive(wt.Field(ix).Type); err != nil {
				return err
			}
		}
		return nil
	}
	return nil
}

// riRegistry holds the ReflectInfo registry.  Unlike rtRegistry (used for the
// rtCache), this information cannot be regenerated at will.  We expect a
// limited number of types to be used within a single address space.
type riRegistry struct {
	sync.RWMutex
	fromName   map[string]*ReflectInfo
	fromWire   map[reflect.Type]*ReflectInfo
	fromNative map[reflect.Type]*ReflectInfo
}

var riReg = &riRegistry{
	fromName:   make(map[string]*ReflectInfo),
	fromWire:   make(map[reflect.Type]*ReflectInfo),
	fromNative: make(map[reflect.Type]*ReflectInfo),
}

func (reg *riRegistry) addReflectInfo(rt reflect.Type, ri *ReflectInfo) (bool, error) {
	reg.Lock()
	defer reg.Unlock()
	// Allow duplicates only if they're identical, meaning the same wire type was
	// registered multiple times.  Otherwise it indicates a non-bijective mapping.
	if riDup := reg.fromWire[ri.WireType]; riDup != nil {
		if !equalRI(ri, riDup) {
			return false, fmt.Errorf("vdl: Register(%v) duplicate wire type %v: %#v and %#v", rt, ri.WireType, ri, riDup)
		}
		// We've seen the wire type before.
		return false, nil
	}
	reg.fromWire[ri.WireType] = ri
	if ri.WireName != "" {
		if riDup := reg.fromName[ri.WireName]; riDup != nil && !equalRI(ri, riDup) {
			return false, fmt.Errorf("vdl: Register(%v) duplicate name %q: %#v and %#v", rt, ri.WireName, ri, riDup)
		}
		reg.fromName[ri.WireName] = ri
	}
	if ri.NativeType != nil {
		if riDup := reg.fromNative[ri.NativeType]; riDup != nil && !equalRI(ri, riDup) {
			return false, fmt.Errorf("vdl: Register(%v) duplicate native type %v: %#v and %#v", rt, ri.NativeType, ri, riDup)
		}
		reg.fromNative[ri.NativeType] = ri
	}
	return true, nil
}

// ReflectInfoFromName returns the ReflectInfo for the given vdl type name, or
// nil if Register has not been called for a type with the given name.
func ReflectInfoFromName(name string) *ReflectInfo {
	riReg.RLock()
	ri := riReg.fromName[name]
	riReg.RUnlock()
	return ri
}

// ReflectInfoFromWire returns the ReflectInfo for the given wire type, or nil
// if Register has not been called for the given wire type.
func ReflectInfoFromWire(wire reflect.Type) *ReflectInfo {
	riReg.RLock()
	ri := riReg.fromWire[wire]
	riReg.RUnlock()
	return ri
}

// ReflectInfoFromNative returns the ReflectInfo for the given native type, or
// nil if Register has not been called for the given native type.
func ReflectInfoFromNative(native reflect.Type) *ReflectInfo {
	riReg.RLock()
	ri := riReg.fromNative[native]
	riReg.RUnlock()
	return ri
}

// wireValueFromNative converts rv from its native to wire type representation.
// There are three return cases:
//   (!IsValid(),  nil): rv doesn't have a wire representation.
//   (!IsValid(), !nil): conversion of rv to wire representation failed.
//   ( IsValid(),  nil): conversion of rv to wire representation succeeded.
func wireValueFromNative(rv reflect.Value) (reflect.Value, error) {
	ri := ReflectInfoFromNative(rv.Type())
	if ri == nil {
		return reflect.Value{}, nil
	}
	newWire := reflect.New(ri.WireType)
	if ierr := ri.FromNativeFunc.Call([]reflect.Value{newWire, rv})[0].Interface(); ierr != nil {
		return reflect.Value{}, ierr.(error)
	}
	return newWire.Elem(), nil
}

// ReflectInfo holds the reflection information for a type.  All fields are
// populated via reflection over the WireType.
//
// The type may include a special __VDLReflect function to describe metadata.
// This is only required for enum and union vdl types, which don't have a
// canonical Go representation.  All other fields are optional.
//
//   type Foo struct{}
//   func (Foo) __VDLReflect(struct{
//     // Type represents the base type.  This is used by union to describe the
//     // union interface type, as opposed to the concrete struct field types.
//     Type Foo
//
//     // Name holds the vdl type name, including the package path, in a tag.
//     Name string "vdl/pkg.Foo"
//
//     // Only one of Enum or Union should be set; they're both shown here for
//     // explanatory purposes.
//
//     // Enum describes the labels for an enum type.
//     Enum struct { A, B string }
//
//     // Union describes the union field names, along with the concrete struct
//     // field types, which contain the actual field types.
//     Union struct {
//       A FieldA
//       B FieldB
//     }
//   }
type ReflectInfo struct {
	// WireType is the type sent over the wire (e.g. types generated by the vdl
	// compiler), and is the basis for all other information in this struct.
	WireType reflect.Type

	// WireName is the vdl type name for the wire type.  Type names include the
	// vdl package path, e.g. "v.io/core/veyron2/vdl.Foo".
	WireName string

	// NativeType is the native Go type for the wire type.  Users specify the
	// native type by adding a pair of conversion functions to the wire type:
	//
	//   type Wire ...
	//   func (x Wire) VDLToNative(n *Native) error
	//   func (x *Wire) VDLFromNative(n Native) error
	NativeType reflect.Type
	// ToNativeFunc holds the VDLToNative function above.
	ToNativeFunc reflect.Value
	// ToNativeFunc holds the VDLFromNative function above.
	FromNativeFunc reflect.Value

	// EnumLabels holds the labels of an enum; it is non-empty iff the WireType
	// represents a vdl enum.
	EnumLabels []string

	// UnionFields holds the fields of a union; it is non-empty iff the WireType
	// represents a vdl union.
	UnionFields []ReflectField
}

// TODO(toddw): Currently all wire types must be registered before TypeOf is
// called.  There's a chicken-and-egg problem; if the user has a package-level
// "var x = vdl.TypeOf(Native{})" in the same package that the Wire type is
// defined, where Wire has the Native type, the init order gets screwed up,
// since the vdl.Register(Wire{}) will occur after the TypeOf.  Perhaps this is
// rare enough to not worry?  But how about typeobject in the vdl?
//
// Also note that we must disallow mutually cyclic types that have native types,
// otherwise we can never get the register order correct.

// ReflectField describes the reflection info for a Union field.
type ReflectField struct {
	// Given a vdl type Foo union{A bool;B string}, we generate:
	//   type Foo interface{...}
	//   type FooA struct{ Value bool }
	//   type FooB struct{ Value string }
	Name    string       // Field name, e.g. "A", "B"
	Type    reflect.Type // Field type, e.g. bool, string
	RepType reflect.Type // Concrete type representing the field, e.g. FooA, FooB
}

func equalRI(a, b *ReflectInfo) bool {
	// Since all information is derived from the WireType, that's all we compare.
	return a.WireType == b.WireType
}

// isUnion returns true iff rt is a union vdl type; it runs a quicker form of
// DeriveReflectInfo, only to check for union.  It's used while normalizing types,
// and is necessary since the generated union type is an interface, and must be
// distinguished from the any type.
func isUnion(rt reflect.Type) bool {
	if method, ok := rt.MethodByName("__VDLReflect"); ok {
		mtype := method.Type
		offsetIn := 1
		if rt.Kind() == reflect.Interface {
			offsetIn = 0
		}
		if mtype.NumIn() == 1+offsetIn && mtype.In(offsetIn).Kind() == reflect.Struct {
			rtReflect := mtype.In(offsetIn)
			if _, ok := rtReflect.FieldByName("Union"); ok {
				return true
			}
		}
	}
	return false
}

// DeriveReflectInfo returns the ReflectInfo corresponding to rt.
// REQUIRES: rt has been normalized, and pointers have been flattened.
//
// TODO(toddw): Unexport this function when valconv has been merged into this
// package; it's only used in makeReflectUnion.
func DeriveReflectInfo(rt reflect.Type) (*ReflectInfo, error) {
	// Set reasonable defaults for types that don't have the __VDLReflect method
	// or native types.
	ri := new(ReflectInfo)
	ri.WireType = rt
	if rt.PkgPath() != "" {
		ri.WireName = rt.PkgPath() + "." + rt.Name()
	}
	// Fill in native type info, if it exists.
	if err := describeNative(rt, ri); err != nil {
		return nil, err
	}
	// If rt is an non-interface type, methods include the receiver as the first
	// in-arg, otherwise they don't.
	offsetIn := 1
	if rt.Kind() == reflect.Interface {
		offsetIn = 0
	}
	// If rt has a __VDLReflect method, use it to extract metadata.
	if method, ok := rt.MethodByName("__VDLReflect"); ok {
		mtype := method.Type
		if mtype.NumOut() != 0 || mtype.NumIn() != 1+offsetIn || mtype.In(offsetIn).Kind() != reflect.Struct {
			return nil, fmt.Errorf("type %q invalid __VDLReflect (want __VDLReflect(struct{...}))", rt)
		}
		// rtReflect corresponds to the argument to __VDLReflect.
		rtReflect := mtype.In(offsetIn)
		if field, ok := rtReflect.FieldByName("Type"); ok {
			ri.WireType = field.Type
			if wt := ri.WireType; wt.PkgPath() != "" {
				ri.WireName = wt.PkgPath() + "." + wt.Name()
			} else {
				ri.WireName = ""
			}
		}
		if field, ok := rtReflect.FieldByName("Name"); ok {
			ri.WireName = string(field.Tag)
		}
		if field, ok := rtReflect.FieldByName("Enum"); ok {
			if err := describeEnum(field.Type, rt, ri); err != nil {
				return nil, err
			}
		}
		if field, ok := rtReflect.FieldByName("Union"); ok {
			if err := describeUnion(field.Type, rt, ri); err != nil {
				return nil, err
			}
		}
		if len(ri.EnumLabels) > 0 && len(ri.UnionFields) > 0 {
			return nil, fmt.Errorf("type %q is both an enum and a union", rt)
		}
	}
	return ri, nil
}

// describeNative fills in ri with native type information from rt.  Users
// specify native type information through a pair of conversion functions on the
// wire type:
//
//   type Wire struct{...}
//   func (x Wire) VDLToNative(n *Native) error
//   func (x *Wire) VDLFromNative(n Native) error
func describeNative(rt reflect.Type, ri *ReflectInfo) error {
	if rt.Kind() == reflect.Interface {
		return nil
	}
	// Reminder: methods include the receiver as in-arg 0.
	var to, from reflect.Type
	if method, ok := rt.MethodByName("VDLToNative"); ok {
		mtype := method.Type
		if mtype.NumIn() != 2 || mtype.In(1).Kind() != reflect.Ptr || mtype.NumOut() != 1 || mtype.Out(0) != rtError {
			return fmt.Errorf("type %q invalid VDLToNative (want method VDLToNative(*Native) error)", rt)
		}
		to = mtype.In(1).Elem()
		ri.ToNativeFunc = method.Func
	}
	if method, ok := reflect.PtrTo(rt).MethodByName("VDLFromNative"); ok {
		mtype := method.Type
		if _, nonptr := rt.MethodByName("VDLFromNative"); nonptr ||
			mtype.NumIn() != 2 || mtype.NumOut() != 1 || mtype.Out(0) != rtError {
			return fmt.Errorf("type %q invalid VDLFromNative (want pointer method VDLFromNative(Native) error)", rt)
		}
		from = mtype.In(1)
		ri.FromNativeFunc = method.Func
	}
	switch {
	case to == nil && from == nil:
		// Neither VDLToNative nor VDLFromNative is specified.
		return nil
	case to != from:
		// Only one of VDLToNative or VDLFromNative is specified, or the native type
		// isn't identical for the two functions.
		return fmt.Errorf("type %q invalid native type (must specify both VDLToNative and VDLFromNative with the same Native type)", rt)
	}
	ri.NativeType = to
	return nil
}

// describeEnum fills in ri; we expect enumReflect has this format:
//   struct {A, B, C Foo}
//
// Here's the full type for vdl type Foo enum{A;B}
//   type Foo int
//   const (
//     FooA Foo = iota
//     FooB
//   )
//   func (Foo) __VDLReflect(struct{
//     Type Foo
//     Enum struct { A, B Foo }
//   }) {}
//   func (Foo) String() string {}
//   func (*Foo) Set(string) error {}
func describeEnum(enumReflect, rt reflect.Type, ri *ReflectInfo) error {
	if rt != ri.WireType || rt.Kind() == reflect.Interface {
		return fmt.Errorf("enum type %q invalid (mismatched type %q)", rt, ri.WireType)
	}
	if enumReflect.Kind() != reflect.Struct || enumReflect.NumField() == 0 {
		return fmt.Errorf("enum type %q invalid (no labels)", rt)
	}
	for ix := 0; ix < enumReflect.NumField(); ix++ {
		ri.EnumLabels = append(ri.EnumLabels, enumReflect.Field(ix).Name)
	}
	if s, ok := rt.MethodByName("String"); !ok ||
		s.Type.NumIn() != 1 ||
		s.Type.NumOut() != 1 || s.Type.Out(0) != rtString {
		return fmt.Errorf("enum type %q must have method String() string", rt)
	}
	_, nonptr := rt.MethodByName("Set")
	if a, ok := reflect.PtrTo(rt).MethodByName("Set"); !ok || nonptr ||
		a.Type.NumIn() != 2 || a.Type.In(1) != rtString ||
		a.Type.NumOut() != 1 || a.Type.Out(0) != rtError {
		return fmt.Errorf("enum type %q must have pointer method Set(string) error", rt)
	}
	return nil
}

// describeUnion fills in ri; we expect unionReflect has this format:
//   struct {
//     A FooA
//     B FooB
//   }
//
// Here's the full type for vdl type Foo union{A bool; B string}
//   type (
//     // Foo is the union interface type, that can hold any field.
//     Foo interface {
//       Index() int
//       Name() string
//       __VDLReflect(__FooReflect)
//     }
//     // FooA and FooB are the concrete field types.
//     FooA struct { Value bool }
//     FooB struct { Value string }
//     // __FooReflect lets us re-construct the union type via reflection.
//     __FooReflect struct {
//       Type  Foo // Tells us the union interface type.
//       Union struct {
//         A FooA  // Tells us field 0 has name A and concrete type FooA.
//         B FooB  // Tells us field 1 has name B and concrete type FooB.
//       }
//     }
//   )
func describeUnion(unionReflect, rt reflect.Type, ri *ReflectInfo) error {
	if ri.WireType.Kind() != reflect.Interface {
		return fmt.Errorf("union type %q has non-interface type %q", rt, ri.WireType)
	}
	if unionReflect.Kind() != reflect.Struct || unionReflect.NumField() == 0 {
		return fmt.Errorf("union type %q invalid (no fields)", rt)
	}
	for ix := 0; ix < unionReflect.NumField(); ix++ {
		f := unionReflect.Field(ix)
		if f.PkgPath != "" {
			return fmt.Errorf("union type %q field %q.%q must be exported", rt, f.PkgPath, f.Name)
		}
		// f.Type corresponds to FooA and FooB in __FooReflect above.
		if f.Type.Kind() != reflect.Struct || f.Type.NumField() != 1 || f.Type.Field(0).Name != "Value" {
			return fmt.Errorf("union type %q field %q has bad concrete field type %q", rt, f.Name, f.Type)
		}
		ri.UnionFields = append(ri.UnionFields, ReflectField{
			Name:    f.Name,
			Type:    f.Type.Field(0).Type,
			RepType: f.Type,
		})
	}
	// Check for Name method on interface and all concrete field structs.
	if n, ok := ri.WireType.MethodByName("Name"); !ok || n.Type.NumIn() != 0 ||
		n.Type.NumOut() != 1 || n.Type.Out(0) != rtString {
		return fmt.Errorf("union interface type %q must have method Name() string", ri.WireType)
	}
	for _, f := range ri.UnionFields {
		if n, ok := f.RepType.MethodByName("Name"); !ok || n.Type.NumIn() != 1 ||
			n.Type.NumOut() != 1 || n.Type.Out(0) != rtString {
			return fmt.Errorf("union field %q type %q must have method Name() string", f.Name, f.RepType)
		}
	}
	return nil
}

// TypeToReflect returns the reflect.Type corresponding to t.  We look up
// named types in our registry, and build the unnamed types that we can via the
// Go reflect package.
func TypeToReflect(t *Type) reflect.Type {
	if t.Name() != "" {
		// Named types cannot be manufactured via Go reflect, so we lookup in our
		// registry instead.
		if ri := ReflectInfoFromName(t.Name()); ri != nil {
			return ri.WireType
		}
		return nil
	}
	// We can make some unnamed types via Go reflect.  Everything else drops
	// through and returns nil.
	switch t.Kind() {
	case Any, Array, Enum, Union:
		// We can't make unnamed versions of any of these types.
	case Optional:
		if elem := TypeToReflect(t.Elem()); elem != nil {
			return reflect.PtrTo(elem)
		}
	case List:
		if elem := TypeToReflect(t.Elem()); elem != nil {
			return reflect.SliceOf(elem)
		}
	case Set:
		if key := TypeToReflect(t.Key()); key != nil {
			return reflect.MapOf(key, rtUnnamedEmptyStruct)
		}
	case Map:
		if key := TypeToReflect(t.Key()); key != nil {
			if elem := TypeToReflect(t.Elem()); elem != nil {
				return reflect.MapOf(key, elem)
			}
		}
	case Struct:
		if t.NumField() == 0 {
			return rtUnnamedEmptyStruct
		}
	default:
		return rtFromKind[t.Kind()]
	}
	return nil
}

var rtFromKind = [...]reflect.Type{
	Bool:       rtBool,
	Byte:       rtByte,
	Uint16:     rtUint16,
	Uint32:     rtUint32,
	Uint64:     rtUint64,
	Int16:      rtInt16,
	Int32:      rtInt32,
	Int64:      rtInt64,
	Float32:    rtFloat32,
	Float64:    rtFloat64,
	Complex64:  rtComplex64,
	Complex128: rtComplex128,
	String:     rtString,
	TypeObject: rtPtrToType,
}
