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
	rtCache        = rtRegistry{rtmap: make(map[reflect.Type]*Type)}
	rtCacheEnabled = true
)

func (reg rtRegistry) lookup(rt reflect.Type) *Type {
	if !rtCacheEnabled {
		return nil
	}
	reg.Lock()
	t := reg.rtmap[rt]
	reg.Unlock()
	return t
}

func (reg rtRegistry) update(pending map[reflect.Type]TypeOrPending) {
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
// TypeFromReflect.
func TypeOf(v interface{}) (*Type, error) {
	return TypeFromReflect(reflect.TypeOf(v))
}

// Normalize the rt type.  The VDL type system represents the concept of
// pointers as Nilable (?), and doesn't allow multiple nilables (??int) or
// Nilable Any (?any).  In addition all interfaces are represented with the
// single Any type.  By normalizing the rt type we simplify the type creation
// logic, and also reduce redundancy in the rtCache.
func normalizeType(rt reflect.Type) reflect.Type {
	// Flatten multiple pointers down to a single pointer.
	for rt.Kind() == reflect.Ptr && rt.Elem().Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	// Collapse all interfaces to interface{}.
	if rt.Kind() == reflect.Interface || (rt.Kind() == reflect.Ptr && rt.Elem().Kind() == reflect.Interface) {
		return rtInterface
	}
	return rt
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
	case rtInterface, rtValue:
		return AnyType
	case rtType:
		return TypeValType
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
		// Pointers are turned into Nilable.
		nilable := builder.Nilable()
		pending[rt] = nilable
		base, err := typeFromReflect(rt.Elem(), builder, pending)
		if err != nil {
			return nil, err
		}
		nilable.AssignBase(base)
		return nilable, nil
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
	default:
		// Named types are trickier, since they may be recursive.  First create the
		// named type and add it to pending.
		named := builder.Named(rt.PkgPath() + "." + rt.Name())
		pending[rt] = named
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
}

// makeUnnamedFromReflect makes the underlying unnamed Type or PendingType
// corresponding to rt.
func makeUnnamedFromReflect(rt reflect.Type, builder *TypeBuilder, pending map[reflect.Type]TypeOrPending) (TypeOrPending, error) {
	// Enums are expected to have special methods like this:
	//   func (Foo) vdlEnumLabels(struct{ A, B, C bool})
	//   func (Foo) String() string
	//   func (*Foo) Assign(string) bool
	if method, ok := rt.MethodByName("vdlEnumLabels"); ok {
		enum := builder.Enum()
		mtype := method.Type
		// Note that mtype.In(0) is the method receiver.
		if mtype.NumOut() != 0 || mtype.NumIn() != 2 || mtype.In(1).Kind() != reflect.Struct {
			return nil, fmt.Errorf("enum type %q must have method vdlEnumLabels with no out-args and one struct in-arg", rt)
		}
		stype := mtype.In(1)
		for fx := 0; fx < stype.NumField(); fx++ {
			enum.AppendLabel(stype.Field(fx).Name)
		}
		if s, ok := rt.MethodByName("String"); !ok || s.Type.NumIn() != 1 ||
			s.Type.NumOut() != 1 || s.Type.Out(0) != rtString {
			return nil, fmt.Errorf("enum type %q must have method String() string", rt)
		}
		_, nonptr := rt.MethodByName("Assign")
		if a, ok := reflect.PtrTo(rt).MethodByName("Assign"); !ok || nonptr ||
			a.Type.NumIn() != 2 || a.Type.In(1) != rtString ||
			a.Type.NumOut() != 1 || a.Type.Out(0) != rtBool {
			return nil, fmt.Errorf("enum type %q must have pointer method Assign(string) bool", rt)
		}
		return enum, nil
	}
	// OneOfs are expected to have special methods like this:
	//   func (Foo) vdlOneOfTypes(_ A, _ B, _ C)
	//   func (*Foo) Assign(interface{}) bool
	if method, ok := rt.MethodByName("vdlOneOfTypes"); ok {
		oneof := builder.OneOf()
		mtype := method.Type
		// Note that mtype.In(0) is the method receiver.
		if mtype.NumOut() != 0 || mtype.NumIn() < 2 {
			return nil, fmt.Errorf("oneof type %q must have method vdlOneOfTypes with no out-args and at least one in-arg", rt)
		}
		for ix := 1; ix < mtype.NumIn(); ix++ {
			in, err := typeFromReflect(mtype.In(ix), builder, pending)
			if err != nil {
				return nil, err
			}
			oneof.AppendType(in)
		}
		_, nonptr := rt.MethodByName("Assign")
		if a, ok := reflect.PtrTo(rt).MethodByName("Assign"); !ok || nonptr ||
			a.Type.NumIn() != 2 ||
			a.Type.In(1).Kind() != reflect.Interface ||
			a.Type.In(1).NumMethod() != 0 ||
			a.Type.NumOut() != 1 || a.Type.Out(0) != rtBool {
			return nil, fmt.Errorf("oneof type %q must have pointer method Assign(interface{}) bool", rt)
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

// MatchOneOfReflectType returns the reflect.Type from oneof that matches
// target.  Returns nil iff no match was found.
func MatchOneOfReflectType(oneof reflect.Type, target *Type) reflect.Type {
	// Look for an exact vdl.Type match within the oneof types.
	// TODO(toddw): Add backoff second-pass matching based on type name.
	if method, ok := oneof.MethodByName("vdlOneOfTypes"); ok {
		mtype := method.Type
		for ix := 1; ix < mtype.NumIn(); ix++ {
			rt := mtype.In(ix)
			if tt, err := TypeFromReflect(rt); err == nil && tt == target {
				return rt
			}
		}
	}
	// TODO(toddw): cache the results for better performance.
	return nil
}

var (
	errTypeFromReflectNil   = errors.New("invalid val.TypeOf(nil)")
	errTypeFromReflectValue = errors.New("invalid val.TypeOf(reflect.Value{})")

	rtInterface          = reflect.TypeOf((*interface{})(nil)).Elem()
	rtBool               = reflect.TypeOf(false)
	rtString             = reflect.TypeOf("")
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
