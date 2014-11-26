package vdl

import (
	"fmt"
	"reflect"
	"sync"
)

// Register registers a type, identified by a value for that type.  The type
// must be a type that will be sent over the wire; types defined in vdl files
// are automatically registered.
//
// There are two reasons types must be registered:
//   1) To create a type name -> reflect.Type mapping, so that when we decode
//      into interface{}, we can generate values of the correct type.
//   2) To create a wire type <-> external type mapping, so that we can
//      auto-marshal external types.
//
// Panics if any mappings are not bijective.  See ReflectInfo for details.
func Register(wire interface{}) {
	if wire == nil {
		return
	}
	rt := normalizeType(reflect.TypeOf(wire))
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	ri, err := DeriveReflectInfo(rt)
	if err != nil {
		panic(err)
	}
	riReg.addReflectInfo(rt, ri)
}

// riRegistry holds the ReflectInfo registry.  Unlike rtRegistry (used for the
// rtCache), this information cannot be regenerated at will.  We expect a
// limited number of types to be used within a single address space.
type riRegistry struct {
	sync.Mutex
	needsUpdate  bool
	fromName     map[string]*ReflectInfo
	fromWire     map[reflect.Type]*ReflectInfo
	fromExternal map[reflect.Type]*ReflectInfo
	fromWireVDL  map[*Type]*ReflectInfo
}

var riReg = &riRegistry{
	fromName:     make(map[string]*ReflectInfo),
	fromWire:     make(map[reflect.Type]*ReflectInfo),
	fromExternal: make(map[reflect.Type]*ReflectInfo),
	fromWireVDL:  make(map[*Type]*ReflectInfo),
}

func (reg *riRegistry) addReflectInfo(rt reflect.Type, ri *ReflectInfo) {
	reg.Lock()
	defer reg.Unlock()
	reg.needsUpdate = true
	// Allow duplicates only if they're identical, meaning the same wire type was
	// registered multiple times.  Otherwise it indicates a non-bijective mapping.
	if riDup := reg.fromWire[ri.WireType]; riDup != nil && !equalRI(ri, riDup) {
		panic(fmt.Errorf("vdl: Register(%v) duplicate wire type %v: %#v and %#v", rt, ri.WireType, ri, riDup))
	}
	reg.fromWire[ri.WireType] = ri
	if ri.WireName != "" {
		if riDup := reg.fromName[ri.WireName]; riDup != nil && !equalRI(ri, riDup) {
			panic(fmt.Errorf("vdl: Register(%v) duplicate name %q: %#v and %#v", rt, ri.WireName, ri, riDup))
		}
		reg.fromName[ri.WireName] = ri
	}
	if ri.ExternalType != nil {
		if riDup := reg.fromExternal[ri.ExternalType]; riDup != nil && !equalRI(ri, riDup) {
			panic(fmt.Errorf("vdl: Register(%v) duplicate external type %v: %#v and %#v", rt, ri.ExternalType, ri, riDup))
		}
		reg.fromExternal[ri.ExternalType] = ri
	}
}

func (reg *riRegistry) maybeUpdateLocked() {
	if !reg.needsUpdate {
		return
	}
	reg.needsUpdate = false
	// TODO(toddw): A given VDL type may have multiple ReflectInfos.  Pick the
	// first for now.
	for _, ri := range reg.fromWire {
		wireVDL, err := TypeFromReflect(ri.WireType)
		if err != nil {
			continue // TODO(toddw): Handle errors?
		}
		if riDup := reg.fromWireVDL[wireVDL]; riDup != nil {
			continue // TODO(toddw): How do we deal with non-equal dups?
		}
		reg.fromWireVDL[wireVDL] = ri
		// TODO(toddw): register subtypes recursively?
	}
}

// ReflectInfoFromName returns the ReflectInfo for the given vdl type name, or
// nil if Register has not been called for a type with the given name.
func ReflectInfoFromName(name string) *ReflectInfo {
	riReg.Lock()
	riReg.maybeUpdateLocked()
	ri := riReg.fromName[name]
	riReg.Unlock()
	return ri
}

// ReflectInfoFromWire returns the ReflectInfo for the given wire type, or nil
// if Register has not been called for the given wire type.
func ReflectInfoFromWire(wire reflect.Type) *ReflectInfo {
	riReg.Lock()
	riReg.maybeUpdateLocked()
	ri := riReg.fromWire[wire]
	riReg.Unlock()
	return ri
}

// ReflectInfoFromExternal returns the ReflectInfo for the given external type,
// or nil if Register has not been called for the given external type.
func ReflectInfoFromExternal(external reflect.Type) *ReflectInfo {
	riReg.Lock()
	riReg.maybeUpdateLocked()
	ri := riReg.fromExternal[external]
	riReg.Unlock()
	return ri
}

// ReflectInfoFromWireVDL returns the ReflectInfo for the given vdl wire type,
// or nil if Register has not been called for the given wire type.
func ReflectInfoFromWireVDL(wire *Type) *ReflectInfo {
	riReg.Lock()
	riReg.maybeUpdateLocked()
	ri := riReg.fromWireVDL[wire]
	riReg.Unlock()
	return ri
}

// ReflectInfo holds the reflection information for a type.  All fields are
// populated via reflection over the WireType.
//
// The type may include a special __VDLReflect function to describe metadata.
// This is only required for enum and oneof vdl types, which don't have a
// canonical Go representation.  All other fields are optional.
//
//   type Foo struct{}
//   func (Foo) __VDLReflect(struct{
//     // Type represents the base type.  This is used by oneof to describe the
//     // oneof interface type, as opposed to the concrete struct field types.
//     Type Foo
//
//     // Name holds the vdl type name, including the package path, in a tag.
//     Name string "vdl/pkg.Foo"
//
//     // Only one of Enum or OneOf should be set; they're both shown here for
//     // explanatory purposes.
//
//     // Enum describes the labels for an enum type.
//     Enum struct { A, B string }
//
//     // OneOf describes the oneof field names, along with the concrete struct
//     // field types, which contain the actual field types.
//     OneOf struct {
//       A FieldA
//       B FieldB
//     }
//   }
type ReflectInfo struct {
	// WireType is the type sent over the wire (e.g. types generated by the vdl
	// compiler), and is the basis for all other information in this struct.
	WireType reflect.Type

	// WireName is the vdl type name for the wire type.  Type names include the
	// vdl package path, e.g. "veyron.io/veyron/veyron2/vdl.Foo".
	WireName string

	// TODO(toddw): Implement external types and describe them here.
	//   type Foo struct{}
	//   func (x *Foo) VDLToExternal() (time.Time, error)
	//   func (x *Foo) VDLFromExternal(t time.Time) error
	ExternalType reflect.Type
	ToExternal   reflect.Value
	FromExternal reflect.Value

	// EnumLabels holds the labels of an enum; it is non-empty iff the WireType
	// represents a vdl enum.
	EnumLabels []string

	// OneOfFields holds the fields of a oneof; it is non-empty iff the WireType
	// represents a vdl oneof.
	OneOfFields []ReflectField
}

// ReflectField describes the reflection info for a OneOf field.
type ReflectField struct {
	// Given a vdl type Foo oneof{A bool;B string}, we generate:
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

// TODO(toddw): TypeFromReflect needs to detect the external type, and translate
// into the wire type automatically, by checking ReflectInfoFromExternal.  This
// means that all types must be registered before TypeOf is called.  There's a
// chicken-and-egg problem here; if the user has a package-level "var x =
// vdl.TypeOf(External{})" in the same package that the Wire type is defined,
// where Wire has the External type, the init order gets screwed up, since the
// vdl.Register(Wire{}) will occur after the TypeOf.  Perhaps this is rare
// enough to not worry?  But how about typeobject in the vdl?
//
// Also note that we must disallow mutually cyclic types that have external
// types, otherwise we can never get the register order correct.

// isOneOf returns true iff rt is a oneof vdl type; it runs a quicker form of
// DeriveReflectInfo, only to check for oneof.  It's used while normalizing types,
// and is necessary since the generated oneof type is an interface, and must be
// distinguished from the any type.
func isOneOf(rt reflect.Type) bool {
	if method, ok := rt.MethodByName("__VDLReflect"); ok {
		mtype := method.Type
		offsetIn := 1
		if rt.Kind() == reflect.Interface {
			offsetIn = 0
		}
		if mtype.NumIn() == 1+offsetIn && mtype.In(offsetIn).Kind() == reflect.Struct {
			rtReflect := mtype.In(offsetIn)
			if _, ok := rtReflect.FieldByName("OneOf"); ok {
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
// package; it's only used in makeReflectOneOf.
func DeriveReflectInfo(rt reflect.Type) (*ReflectInfo, error) {
	// Set reasonable defaults for types that don't have external types or the
	// __VDLReflect method.
	ri := new(ReflectInfo)
	ri.WireType = rt
	if rt.PkgPath() != "" {
		ri.WireName = rt.PkgPath() + "." + rt.Name()
	}
	// If rt is an non-interface type, methods include the receiver as the first
	// in-arg, otherwise they don't.
	offsetIn := 1
	if rt.Kind() == reflect.Interface {
		offsetIn = 0
	}
	// TODO(toddw): Fill in external type info

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
		if field, ok := rtReflect.FieldByName("OneOf"); ok {
			if err := describeOneOf(field.Type, rt, ri); err != nil {
				return nil, err
			}
		}
		if len(ri.EnumLabels) > 0 && len(ri.OneOfFields) > 0 {
			return nil, fmt.Errorf("type %q is both an enum and a oneof", rt)
		}
	}
	return ri, nil
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
//   func (*Foo) Assign(string) bool {}
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
	_, nonptr := rt.MethodByName("Assign")
	if a, ok := reflect.PtrTo(rt).MethodByName("Assign"); !ok || nonptr ||
		a.Type.NumIn() != 2 || a.Type.In(1) != rtString ||
		a.Type.NumOut() != 1 || a.Type.Out(0) != rtBool {
		return fmt.Errorf("enum type %q must have pointer method Assign(string) bool", rt)
	}
	return nil
}

// describeOneOf fills in ri; we expect oneofReflect has this format:
//   struct {
//     A FooA
//     B FooB
//   }
//
// Here's the full type for vdl type Foo oneof{A bool; B string}
//   type (
//     // Foo is the oneof interface type, that can hold any field.
//     Foo interface {
//       Index() int
//       Name() string
//       __VDLReflect(__FooReflect)
//     }
//     // FooA and FooB are the concrete field types.
//     FooA struct { Value bool }
//     FooB struct { Value string }
//     // __FooReflect lets us re-construct the oneof type via reflection.
//     __FooReflect struct {
//       Type  Foo // Tells us the oneof interface type.
//       OneOf struct {
//         A FooA  // Tells us field 0 has name A and concrete type FooA.
//         B FooB  // Tells us field 1 has name B and concrete type FooB.
//       }
//     }
//   )
func describeOneOf(oneofReflect, rt reflect.Type, ri *ReflectInfo) error {
	if ri.WireType.Kind() != reflect.Interface {
		return fmt.Errorf("oneof type %q has non-interface type %q", rt, ri.WireType)
	}
	if oneofReflect.Kind() != reflect.Struct || oneofReflect.NumField() == 0 {
		return fmt.Errorf("oneof type %q invalid (no fields)", rt)
	}
	for ix := 0; ix < oneofReflect.NumField(); ix++ {
		f := oneofReflect.Field(ix)
		if f.PkgPath != "" {
			return fmt.Errorf("oneof type %q field %q.%q must be exported", rt, f.PkgPath, f.Name)
		}
		// f.Type corresponds to FooA and FooB in __FooReflect above.
		if f.Type.Kind() != reflect.Struct || f.Type.NumField() != 1 || f.Type.Field(0).Name != "Value" {
			return fmt.Errorf("oneof type %q field %q has bad concrete field type %q", rt, f.Name, f.Type)
		}
		ri.OneOfFields = append(ri.OneOfFields, ReflectField{
			Name:    f.Name,
			Type:    f.Type.Field(0).Type,
			RepType: f.Type,
		})
	}
	// Check for Name method on interface and all concrete field structs.
	if n, ok := ri.WireType.MethodByName("Name"); !ok || n.Type.NumIn() != 0 ||
		n.Type.NumOut() != 1 || n.Type.Out(0) != rtString {
		return fmt.Errorf("oneof interface type %q must have method Name() string", ri.WireType)
	}
	for _, f := range ri.OneOfFields {
		if n, ok := f.RepType.MethodByName("Name"); !ok || n.Type.NumIn() != 1 ||
			n.Type.NumOut() != 1 || n.Type.Out(0) != rtString {
			return fmt.Errorf("oneof field %q type %q must have method Name() string", f.Name, f.RepType)
		}
	}
	return nil
}

// ReflectFromType returns the reflect.Type corresponding to t.  We look up
// named types in our registry, and build the unnamed types that we can via the
// Go reflect package.
func ReflectFromType(t *Type) reflect.Type {
	if t.Name() != "" {
		// Named types cannot be manufactured via Go reflect, so we lookup in our
		// registry instead.
		if ri := ReflectInfoFromName(t.Name()); ri != nil {
			return ri.WireType
		}
		return nil
	}
	// If the unnamed type is registered, use that.
	// TODO(toddw): Need the comparison to consider the same type name as equal.
	if ri := ReflectInfoFromWireVDL(t); ri != nil {
		return ri.WireType
	}
	// We can make some unnamed types via Go reflect.  Everything else drops
	// through and returns nil.
	switch t.Kind() {
	case Any, Array, Enum, OneOf:
		// We can't make unnamed versions of any of these types.
	case Optional:
		if elem := ReflectFromType(t.Elem()); elem != nil {
			return reflect.PtrTo(elem)
		}
	case List:
		if elem := ReflectFromType(t.Elem()); elem != nil {
			return reflect.SliceOf(elem)
		}
	case Set:
		if key := ReflectFromType(t.Key()); key != nil {
			return reflect.MapOf(key, rtUnnamedEmptyStruct)
		}
	case Map:
		if key := ReflectFromType(t.Key()); key != nil {
			if elem := ReflectFromType(t.Elem()); elem != nil {
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
