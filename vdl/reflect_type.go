package vdl

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"unsafe"
)

// rtRegistry is a map from reflect.Type to *Type.  The only instance is
// rtCache, which is a global cache to speed up repeated lookups.
type rtRegistry struct {
	sync.Mutex
	rtmap map[reflect.Type]*Type
}

var (
	rtCache        = &rtRegistry{rtmap: make(map[reflect.Type]*Type)}
	rtCacheEnabled = true
)

func (reg *rtRegistry) lookup(rt reflect.Type) *Type {
	if !rtCacheEnabled {
		return nil
	}
	reg.Lock()
	t := reg.rtmap[rt]
	reg.Unlock()
	return t
}

func (reg *rtRegistry) update(pending map[reflect.Type]TypeOrPending) {
	if !rtCacheEnabled {
		return
	}
	reg.Lock()
	for rt, top := range pending {
		reg.rtmap[rt] = getBuiltType(top)
	}
	reg.Unlock()
}

// getBuiltType returns the built type from top.  Panics if the type couldn't be
// retrieved; the caller must ensure a valid type is contained in top.
func getBuiltType(top TypeOrPending) *Type {
	switch ttop := top.(type) {
	case *Type:
		return ttop
	case PendingType:
		t, err := ttop.Built()
		if err != nil {
			panic(err)
		}
		if t != nil {
			return t
		}
	}
	panic(fmt.Errorf("val: nil type extracted from TypeOrPending"))
}

// TypeOf returns the type corresponding to v.  It's a helper for calling
// TypeFromReflect, and panics on any errors.
func TypeOf(v interface{}) *Type {
	t, err := TypeFromReflect(reflect.TypeOf(v))
	if err != nil {
		panic(fmt.Errorf("vdl: can't take TypeOf(%T): %v", v, err))
	}
	return t
}

// Normalize the rt type.  The VDL type system represents the concept of
// pointers as Optional, and doesn't allow multiple Optionals or Optional Any or
// Error (??int, ?any, ?error).  All Go interfaces are represented with the
// single Any type, except for Go interfaces that describe OneOf types.  By
// normalizing the rt type we simplify type creation, and also reduce redundancy
// in the rtCache.
func normalizeType(rt reflect.Type) reflect.Type {
	// Flatten rt to no pointers, and rtAtMostOnePtr to at most one pointer.
	rtAtMostOnePtr := rt
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
		if rt.Kind() == reflect.Ptr {
			rtAtMostOnePtr = rt
		}
	}
	// Special-case the error type.
	if rt == rtError {
		return rtError
	}
	// Collapse all interfaces to interface{}, except for interfaces that describe
	// OneOf types.  We allow Optional OneOf types.
	if rt.Kind() == reflect.Interface {
		if _, isOneOf := rt.MethodByName("__DescribeOneOf"); isOneOf {
			return rtAtMostOnePtr
		}
		return rtInterface
	}
	return rtAtMostOnePtr
}

// Lookup the rt type and return the corresponding *Type.  Returns a non-nil
// *Type if rt exists in the cache, or has a well-known conversion.
func lookupType(rt reflect.Type) *Type {
	if t := rtCache.lookup(rt); t != nil {
		return t
	}
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	switch rt {
	case rtError:
		return ErrorType
	case rtInterface, rtValue:
		return AnyType
	case rtType:
		return TypeObjectType
	}
	return nil
}

// TypeFromReflect returns the type corresponding to rt.  Not all reflect types
// have a valid type; reflect.Chan, reflect.Func and reflect.UnsafePointer are
// unsupported, as are maps with pointer keys, as well as structs with only
// unexported fields.
func TypeFromReflect(rt reflect.Type) (*Type, error) {
	if rt == nil {
		return nil, errTypeFromReflectNil
	}
	rt = normalizeType(rt)
	if t := lookupType(rt); t != nil {
		return t, nil
	}
	// The strategy is to recursively populate the builder with the type and
	// subtypes, keeping track of new types in pending.  After all types have been
	// populated, we build the types and update rtCache with all pending types.
	//
	// Concurrent updates may cause rtCache to be updated multiple times.  This
	// race is benign; we always end up with the same result, and we never mutate
	// a type once it has been returned.
	builder := new(TypeBuilder)
	pending := make(map[reflect.Type]TypeOrPending)
	result, err := makeTypeFromReflect(rt, builder, pending)
	if err != nil {
		return nil, err
	}
	if !builder.Build() {
		// The result must be a pending type with a non-nil error.
		_, err := result.(PendingType).Built()
		return nil, err
	}
	rtCache.update(pending)
	return getBuiltType(result), nil
}

// typeFromReflect returns the Type or PendingType corresponding to rt.  It
// either returns the type directly from the cache or pending map, or makes the
// type based on rt.
func typeFromReflect(rt reflect.Type, builder *TypeBuilder, pending map[reflect.Type]TypeOrPending) (TypeOrPending, error) {
	rt = normalizeType(rt)
	if t := lookupType(rt); t != nil {
		return t, nil
	}
	if p, ok := pending[rt]; ok {
		// If the type is already in our pending map, we return it now.  This breaks
		// infinite loops from recursive types.
		return p, nil
	}
	return makeTypeFromReflect(rt, builder, pending)
}

// validateType returns a non-nil error if rt is not a valid vdl type.
func validateType(rt reflect.Type) error {
	// First flatten all pointers.
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	// Now check error conditions.
	if rt == rtReflectValue {
		return errTypeFromReflectValue
	}
	switch rt.Kind() {
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return fmt.Errorf("reflect type %q not supported", rt)
	}
	return nil
}

// makeTypeFromReflect makes the Type or PendingType corresponding to rt.  Calls
// typeFromReflect to recursively generate subtypes.
//
// PRE-CONDITION:  rt doesn't exist in rtCache or pending.
// POST-CONDITION: rt exists in pending.
func makeTypeFromReflect(rt reflect.Type, builder *TypeBuilder, pending map[reflect.Type]TypeOrPending) (TypeOrPending, error) {
	if err := validateType(rt); err != nil {
		return nil, err
	}
	switch {
	case rt.Kind() == reflect.Ptr:
		// Pointers are turned into Optional.
		opt := builder.Optional()
		pending[rt] = opt
		base, err := typeFromReflect(rt.Elem(), builder, pending)
		if err != nil {
			return nil, err
		}
		opt.AssignBase(base)
		return opt, nil
	case rt.PkgPath() == "":
		// Unnamed types are made directly.  There's no way to create a recursive
		// type based solely on unnamed types, so it's ok to to update pending
		// *after* making the unnamed type.
		unnamed, err := makeUnnamedFromReflect(rt, builder, pending)
		if err != nil {
			return nil, err
		}
		pending[rt] = unnamed
		return unnamed, nil
	}
	// Named types are trickier, since they may be recursive.  First create the
	// named type and add it to pending.  We must special-case oneof types; we
	// allow rt to be either the interface type (Foo below) or one of the concrete
	// field types (FooA or FooB below), and in all cases we want the name to be
	// based on the interface type, and all keys in the pending map to point to
	// the created vdl type.
	name, keys := rt.PkgPath()+"."+rt.Name(), []reflect.Type{rt}
	switch rtOneOf, fields, err := DescribeOneOf(rt); {
	case err != nil:
		return nil, err
	case rtOneOf != nil:
		name = rtOneOf.PkgPath() + "." + rtOneOf.Name()
		keys = []reflect.Type{rtOneOf}
		for _, f := range fields {
			keys = append(keys, f.Type)
		}
	}
	named := builder.Named(name)
	for _, key := range keys {
		pending[key] = named
	}
	// Now make the unnamed underlying type.  Recursive types will find the
	// existing entry in pending, and avoid the infinite loop.
	unnamed, err := makeUnnamedFromReflect(rt, builder, pending)
	if err != nil {
		return nil, err
	}
	// Finally assign the base type and we're done.
	named.AssignBase(unnamed)
	return named, nil
}

// describeEnum returns a non-nil rtEnum if rt is an enum type, along with the
// enum labels and a nil error.  Returns a non-nil error iff rt is an enum type
// but is mis-specified.  Returns all nils if rt isn't an enum type.
//
// Enum has the following format; assume vdl type Foo enum{A;B}
//   type Foo int
//   const (
//     FooA Foo = iota
//     FooB
//   )
//   func (Foo) __DescribeEnum(struct{ A, B Foo }) {}
//   func (Foo) String() string {}
//   func (*Foo) Assign(string) bool {}
func describeEnum(rt reflect.Type) (rtEnum reflect.Type, labels []string, _ error) {
	if method, ok := rt.MethodByName("__DescribeEnum"); ok {
		mtype := method.Type
		// Note that mtype.In(0) is the method receiver.
		if mtype.NumOut() != 0 || mtype.NumIn() != 2 || mtype.In(1).Kind() != reflect.Struct {
			return nil, nil, fmt.Errorf("enum type %q must have method __DescribeEnum(struct{...})", rt)
		}
		rtDesc := mtype.In(1)
		for ix := 0; ix < rtDesc.NumField(); ix++ {
			labels = append(labels, rtDesc.Field(ix).Name)
		}
		if s, ok := rt.MethodByName("String"); !ok || s.Type.NumIn() != 1 ||
			s.Type.NumOut() != 1 || s.Type.Out(0) != rtString {
			return nil, nil, fmt.Errorf("enum type %q must have method String() string", rt)
		}
		_, nonptr := rt.MethodByName("Assign")
		if a, ok := reflect.PtrTo(rt).MethodByName("Assign"); !ok || nonptr ||
			a.Type.NumIn() != 2 || a.Type.In(1) != rtString ||
			a.Type.NumOut() != 1 || a.Type.Out(0) != rtBool {
			return nil, nil, fmt.Errorf("enum type %q must have pointer method Assign(string) bool", rt)
		}
		rtEnum = rt
	}
	return
}

// DescribeOneOf returns a non-nil rtOneOf if rt is a oneof type, along with the
// fields from the description __FooDesc, and a nil error.  Returns a non-nil
// error iff rt is a oneof type but is mis-specified.  Returns all nils if rt
// isn't a oneof type.
//
// OneOf has the following format; assume vdl type Foo oneof{A bool; B string}
//   type (
//     // Foo is the oneof interface type, that can hold any field.
//     Foo interface {
//       Index() int
//       Name() string
//       __DescribeOneOf(__FooDesc)
//     }
//     // FooA and FooB are the concrete field types.
//     FooA struct { Value bool }
//     FooB struct { Value string }
//     // __FooDesc lets us re-construct the oneof type via reflection.
//     __FooDesc struct {
//       Foo    // Tells us the oneof interface type.
//       A FooA // Tells us field 0 has name A and concrete type FooA.
//       B FooB // Tells us field 1 has name B and concrete type FooB.
//     }
//   )
//
// TODO(toddw): Unexport after vdl/valconv is merged into this package.
func DescribeOneOf(rt reflect.Type) (rtOneOf reflect.Type, fields []reflect.StructField, _ error) {
	if method, ok := rt.MethodByName("__DescribeOneOf"); ok {
		mtype := method.Type
		// If rt is a non-interface type (e.g. FooA and FooB above), mtype includes
		// the receiver as the first in-arg, otherwise (e.g. Foo above) it doesn't.
		offsetIn := 1
		if rt.Kind() == reflect.Interface {
			offsetIn = 0
		}
		if mtype.NumOut() != 0 || mtype.NumIn() != 1+offsetIn {
			return nil, nil, fmt.Errorf("oneof type %q must have method __DescribeOneOf(struct{...})", rt)
		}
		// rtDesc corresponds to __FooDesc above.
		rtDesc := mtype.In(offsetIn)
		if rtDesc.Kind() != reflect.Struct || rtDesc.NumField() < 2 {
			return nil, nil, fmt.Errorf("oneof type %q must have method __DescribeOneOf(struct{...})", rt)
		}
		// rtOneOf corresponds to field Foo in __FooDesc above.
		rtOneOf = rtDesc.Field(0).Type
		if rtOneOf.Kind() != reflect.Interface || rtOneOf.PkgPath() == "" {
			return nil, nil, fmt.Errorf("oneof type %q has bad interface type %q", rt, rtOneOf)
		}
		// Loop through the concrete fields of the description.
		for ix := 1; ix < rtDesc.NumField(); ix++ {
			f := rtDesc.Field(ix)
			if f.PkgPath != "" {
				return nil, nil, fmt.Errorf("oneof type %q field %q.%q must be exported", rt, f.PkgPath, f.Name)
			}
			// f.Type corresponds to FooA and FooB in __FooDesc above.
			if f.Type.Kind() != reflect.Struct || f.Type.NumField() != 1 || f.Type.Field(0).Name != "Value" {
				return nil, nil, fmt.Errorf("oneof type %q field %q has bad concrete field type %q", rt, f.Name, f.Type)
			}
			fields = append(fields, f)
		}
		if n, ok := rt.MethodByName("Name"); !ok || n.Type.NumIn() != offsetIn ||
			n.Type.NumOut() != 1 || n.Type.Out(0) != rtString {
			return nil, nil, fmt.Errorf("oneof type %q must have method Name() string", rt)
		}
	}
	return
}

// makeUnnamedFromReflect makes the underlying unnamed Type or PendingType
// corresponding to rt.
func makeUnnamedFromReflect(rt reflect.Type, builder *TypeBuilder, pending map[reflect.Type]TypeOrPending) (TypeOrPending, error) {
	// Handle enum types
	switch rtEnum, labels, err := describeEnum(rt); {
	case err != nil:
		return nil, err
	case rtEnum != nil:
		enum := builder.Enum()
		for _, label := range labels {
			enum.AppendLabel(label)
		}
		return enum, nil
	}
	// Handle oneof types
	switch rtOneOf, fields, err := DescribeOneOf(rt); {
	case err != nil:
		return nil, err
	case rtOneOf != nil:
		oneof := builder.OneOf()
		for _, f := range fields {
			in, err := typeFromReflect(f.Type.Field(0).Type, builder, pending)
			if err != nil {
				return nil, err
			}
			oneof.AppendField(f.Name, in)
		}
		return oneof, nil
	}
	// Handle composite types
	switch rt.Kind() {
	case reflect.Array:
		elem, err := typeFromReflect(rt.Elem(), builder, pending)
		if err != nil {
			return nil, err
		}
		return builder.Array().AssignLen(rt.Len()).AssignElem(elem), nil
	case reflect.Slice:
		elem, err := typeFromReflect(rt.Elem(), builder, pending)
		if err != nil {
			return nil, err
		}
		return builder.List().AssignElem(elem), nil
	case reflect.Map:
		key, err := typeFromReflect(rt.Key(), builder, pending)
		if err != nil {
			return nil, err
		}
		if rt.Elem() == rtUnnamedEmptyStruct {
			// The map actually represents a set
			return builder.Set().AssignKey(key), nil
		}
		elem, err := typeFromReflect(rt.Elem(), builder, pending)
		if err != nil {
			return nil, err
		}
		return builder.Map().AssignKey(key).AssignElem(elem), nil
	case reflect.Struct:
		st := builder.Struct()
		for fx := 0; fx < rt.NumField(); fx++ {
			rtField := rt.Field(fx)
			if rtField.PkgPath != "" {
				continue // field isn't exported
			}
			field, err := typeFromReflect(rtField.Type, builder, pending)
			if err != nil {
				return nil, err
			}
			st.AppendField(rtField.Name, field)
		}
		if rt.NumField() > 0 && st.NumField() == 0 {
			return nil, fmt.Errorf("type %q only has unexported fields", rt)
		}
		return st, nil
	}
	// Handle scalar types
	if t := rtScalarTypes[rt.Kind()]; t != nil {
		return t, nil
	}
	panic(fmt.Errorf("val: makeUnnamedFromReflect unhandled %v %v", rt.Kind(), rt))
}

var (
	errTypeFromReflectNil   = errors.New("invalid vdl.TypeOf(nil)")
	errTypeFromReflectValue = errors.New("invalid vdl.TypeOf(reflect.Value{})")

	rtInterface          = reflect.TypeOf((*interface{})(nil)).Elem()
	rtBool               = reflect.TypeOf(false)
	rtString             = reflect.TypeOf("")
	rtError              = reflect.TypeOf((*error)(nil)).Elem()
	rtType               = reflect.TypeOf(Type{})
	rtValue              = reflect.TypeOf(Value{})
	rtReflectValue       = reflect.TypeOf(reflect.Value{})
	rtUnnamedEmptyStruct = reflect.TypeOf(struct{}{})

	rtScalarTypes = [...]*Type{
		reflect.Bool:       BoolType,
		reflect.Uint8:      ByteType,
		reflect.Uint16:     Uint16Type,
		reflect.Uint32:     Uint32Type,
		reflect.Uint64:     Uint64Type,
		reflect.Uint:       uintType(8 * unsafe.Sizeof(uint(0))),
		reflect.Uintptr:    uintType(8 * unsafe.Sizeof(uintptr(0))),
		reflect.Int8:       Int16Type,
		reflect.Int16:      Int16Type,
		reflect.Int32:      Int32Type,
		reflect.Int64:      Int64Type,
		reflect.Int:        intType(8 * unsafe.Sizeof(int(0))),
		reflect.Float32:    Float32Type,
		reflect.Float64:    Float64Type,
		reflect.Complex64:  Complex64Type,
		reflect.Complex128: Complex128Type,
		reflect.String:     StringType,
	}
)

func uintType(bitlen uintptr) *Type {
	switch bitlen {
	case 32:
		return Uint32Type
	default:
		return Uint64Type
	}
}

func intType(bitlen uintptr) *Type {
	switch bitlen {
	case 32:
		return Int32Type
	default:
		return Int64Type
	}
}
