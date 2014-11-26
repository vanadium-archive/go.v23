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

// Normalize the rt type.  The VDL type system Optional is represented as a Go
// pointer.  Only structs may be optional in VDL, but we still allow the pointer
// forms of other types in Go.  E.g. VDL doesn't allow ?map, but we do allow Go
// **map, and consider that to be a VDL map; the pointers are flattened away.
//
// In addition all Go interfaces are represented with the single Any type,
// except for Go interfaces that describe OneOf types.
//
// By normalizing the rt type we simplify type creation, and also reduce
// redundancy in the rtCache.
func normalizeType(rt reflect.Type) reflect.Type {
	// Flatten rt to no pointers, and rtAtMostOnePtr to at most one pointer.
	rtAtMostOnePtr := rt
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
		if rt.Kind() == reflect.Ptr {
			rtAtMostOnePtr = rt
		}
	}
	// Handle special cases.  OneOf may be either an interface or a struct, and
	// should be handled first.
	if isOneOf(rt) {
		return rt
	}
	switch {
	case rt == rtError:
		return rtError
	case rt.Kind() == reflect.Interface:
		// Collapse all interfaces to interface{}
		return rtInterface
	case rt.Kind() == reflect.Struct && rt.PkgPath() != "":
		// Named structs may be optional, so we keep the pointer.
		return rtAtMostOnePtr
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
		return AnyType, nil
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
	// Flatten pointers to simplify the checks.
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
	if rt.Kind() == reflect.Ptr {
		// Pointers are turned into Optional.
		opt := builder.Optional()
		pending[rt] = opt
		elem, err := typeFromReflect(rt.Elem(), builder, pending)
		if err != nil {
			return nil, err
		}
		opt.AssignElem(elem)
		return opt, nil
	}
	ri, err := DeriveReflectInfo(rt)
	if err != nil {
		return nil, err
	}
	if ri.WireName == "" {
		// Unnamed types are made directly.  There's no way to create a recursive
		// type based solely on unnamed types, so it's ok to to update pending
		// *after* making the unnamed type.
		unnamed, err := makeUnnamedFromReflect(ri, builder, pending)
		if err != nil {
			return nil, err
		}
		pending[ri.WireType] = unnamed
		return unnamed, nil
	}
	// Named types are trickier, since they may be recursive.  First create the
	// named type and add it to pending.  We must special-case oneof types; the
	// interface type and all field types are keyed in the pending map to point to
	// the created vdl type.
	named := builder.Named(ri.WireName)
	pending[ri.WireType] = named
	for _, oneofField := range ri.OneOfFields {
		pending[oneofField.RepType] = named
	}
	// Now make the unnamed underlying type.  Recursive types will find the
	// existing entry in pending, and avoid the infinite loop.
	unnamed, err := makeUnnamedFromReflect(ri, builder, pending)
	if err != nil {
		return nil, err
	}
	// Finally assign the base type and we're done.
	named.AssignBase(unnamed)
	return named, nil
}

// makeUnnamedFromReflect makes the underlying unnamed Type or PendingType
// corresponding to ri.
func makeUnnamedFromReflect(ri *ReflectInfo, builder *TypeBuilder, pending map[reflect.Type]TypeOrPending) (TypeOrPending, error) {
	// Handle enum types
	if len(ri.EnumLabels) > 0 {
		enum := builder.Enum()
		for _, label := range ri.EnumLabels {
			enum.AppendLabel(label)
		}
		return enum, nil
	}
	// Handle oneof types
	if len(ri.OneOfFields) > 0 {
		oneof := builder.OneOf()
		for _, f := range ri.OneOfFields {
			in, err := typeFromReflect(f.Type, builder, pending)
			if err != nil {
				return nil, err
			}
			oneof.AppendField(f.Name, in)
		}
		return oneof, nil
	}
	// Handle composite types
	rt := ri.WireType
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
		if rt.Key().Kind() == reflect.Ptr {
			return nil, fmt.Errorf("invalid key %q in %q", rt.Key(), rt)
		}
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
	if t := typeFromRTKind[rt.Kind()]; t != nil {
		return t, nil
	}
	panic(fmt.Errorf("val: makeUnnamedFromReflect unhandled %v %v", rt.Kind(), rt))
}

var (
	errTypeFromReflectValue = errors.New("invalid vdl.TypeOf(reflect.Value{})")

	rtInterface          = reflect.TypeOf((*interface{})(nil)).Elem()
	rtBool               = reflect.TypeOf(false)
	rtByte               = reflect.TypeOf(byte(0))
	rtUint16             = reflect.TypeOf(uint16(0))
	rtUint32             = reflect.TypeOf(uint32(0))
	rtUint64             = reflect.TypeOf(uint64(0))
	rtInt16              = reflect.TypeOf(int16(0))
	rtInt32              = reflect.TypeOf(int32(0))
	rtInt64              = reflect.TypeOf(int64(0))
	rtFloat32            = reflect.TypeOf(float32(0))
	rtFloat64            = reflect.TypeOf(float64(0))
	rtComplex64          = reflect.TypeOf(complex64(0))
	rtComplex128         = reflect.TypeOf(complex128(0))
	rtString             = reflect.TypeOf("")
	rtError              = reflect.TypeOf((*error)(nil)).Elem()
	rtType               = reflect.TypeOf(Type{})
	rtPtrToType          = reflect.TypeOf((*Type)(nil))
	rtValue              = reflect.TypeOf(Value{})
	rtReflectValue       = reflect.TypeOf(reflect.Value{})
	rtUnnamedEmptyStruct = reflect.TypeOf(struct{}{})

	typeFromRTKind = [...]*Type{
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
