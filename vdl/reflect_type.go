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

// TypeFromReflect returns the type corresponding to rt.  Not all reflect types
// have a valid type; reflect.Chan, reflect.Func and reflect.UnsafePointer are
// unsupported, as are maps with pointer keys, as well as structs with only
// unexported fields.
func TypeFromReflect(rt reflect.Type) (*Type, error) {
	if rt == nil {
		return nil, errTypeFromReflectNil
	}
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem() // flatten pointers down to the base type
	}
	if t := rtCache.lookup(rt); t != nil {
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

// typeFromReflect returns the Type or PendingType corresponding to rt, possibly
// returning the result from rtCache or pending.
func typeFromReflect(rt reflect.Type, builder *TypeBuilder, pending map[reflect.Type]TypeOrPending) (TypeOrPending, error) {
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem() // flatten pointers down to the base type
	}
	if t := rtCache.lookup(rt); t != nil {
		return t, nil
	}
	if p, ok := pending[rt]; ok {
		// If the type is already in our pending map, we return it now.  This breaks
		// infinite loops from recursive types.
		return p, nil
	}
	return makeTypeFromReflect(rt, builder, pending)
}

// makeTypeFromReflect makes the Type or PendingType corresponding to rt.
// PRE-CONDITION: rt doesn't exist in rtCache or pending.
// POST-CONDITION: rt exists in pending.
func makeTypeFromReflect(rt reflect.Type, builder *TypeBuilder, pending map[reflect.Type]TypeOrPending) (TypeOrPending, error) {
	kind := rt.Kind()
	// Handle error cases.
	switch {
	case rt == rtReflectValue:
		return nil, errTypeFromReflectValue
	case kind == reflect.Chan || kind == reflect.Func || kind == reflect.UnsafePointer:
		return nil, fmt.Errorf("reflect type %q not supported", rt)
	}
	switch {
	case rt == rtValue || kind == reflect.Interface:
		// Any is special-cased since it can't be named.
		pending[rt] = AnyType
		return AnyType, nil
	case rt == rtType:
		// TypeVal is special-cased since it can't be named.
		pending[rt] = TypeValType
		return TypeValType, nil
	case rt.PkgPath() == "":
		// Unnamed types are made directly from their base type.  It's fine to
		// update pending after making the base type, since there's no way to create
		// a recursive type based solely on unnamed types.
		base, err := makeBaseFromReflect(rt, builder, pending)
		if err != nil {
			return nil, err
		}
		pending[rt] = base
		return base, nil
	default:
		// Named types are trickier, since they may be recursive.  First create the
		// named type and add it to pending.
		named := builder.Named(rt.PkgPath() + "." + rt.Name())
		pending[rt] = named
		// Now make the base type.  Recursive types will find the existing entry in
		// pending, and avoid the infinite loop.
		base, err := makeBaseFromReflect(rt, builder, pending)
		if err != nil {
			return nil, err
		}
		// Finally assign the base type and we're done.
		named.AssignBase(base)
		return named, nil
	}
}

// makeBaseFromReflect makes the base (underlying unnamed) Type or PendingType
// corresponding to rt.
func makeBaseFromReflect(rt reflect.Type, builder *TypeBuilder, pending map[reflect.Type]TypeOrPending) (TypeOrPending, error) {
	kind := rt.Kind()
	// Handle composite types
	switch kind {
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
		if rt.Key().Kind() == reflect.Ptr && !rt.Key().ConvertibleTo(rtPtrToType) {
			// We don't allow maps with pointer keys; we can't just flatten the
			// pointers since we may end up with duplicates.  We make an exception for
			// *Type, since we know it's hash-consed.
			return nil, fmt.Errorf("type %q has pointer keys", rt)
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
	if t := rtScalarTypes[kind]; t != nil {
		return t, nil
	}
	panic(fmt.Errorf("val: makeBaseFromReflect unhandled %v %v", rt.Kind(), rt))
}

var (
	errTypeFromReflectNil   = errors.New("invalid val.TypeOf(nil)")
	errTypeFromReflectValue = errors.New("invalid val.TypeOf(reflect.Value{})")

	rtByte               = reflect.TypeOf(byte(0))
	rtBool               = reflect.TypeOf(bool(false))
	rtString             = reflect.TypeOf(string(""))
	rtType               = reflect.TypeOf(Type{})
	rtValue              = reflect.TypeOf(Value{})
	rtPtrToType          = reflect.TypeOf((*Type)(nil))
	rtPtrToValue         = reflect.TypeOf((*Value)(nil))
	rtReflectValue       = reflect.TypeOf(reflect.Value{})
	rtUnnamedEmptyStruct = reflect.TypeOf(struct{}{})

	rtScalarTypes = [...]*Type{
		reflect.Bool:       BoolType,
		reflect.Uint8:      ByteType,
		reflect.Uint16:     Uint16Type,
		reflect.Uint32:     Uint32Type,
		reflect.Uint64:     Uint64Type,
		reflect.Uint:       uintType(bitlenR(reflect.Uint)),
		reflect.Uintptr:    uintType(bitlenR(reflect.Uintptr)),
		reflect.Int8:       Int16Type,
		reflect.Int16:      Int16Type,
		reflect.Int32:      Int32Type,
		reflect.Int64:      Int64Type,
		reflect.Int:        intType(bitlenR(reflect.Int)),
		reflect.Float32:    Float32Type,
		reflect.Float64:    Float64Type,
		reflect.Complex64:  Complex64Type,
		reflect.Complex128: Complex128Type,
		reflect.String:     StringType,
	}

	bitlenReflect = [...]uintptr{
		reflect.Uint8:      8,
		reflect.Uint16:     16,
		reflect.Uint32:     32,
		reflect.Uint64:     64,
		reflect.Uint:       8 * unsafe.Sizeof(uint(0)),
		reflect.Uintptr:    8 * unsafe.Sizeof(uintptr(0)),
		reflect.Int8:       8,
		reflect.Int16:      16,
		reflect.Int32:      32,
		reflect.Int64:      64,
		reflect.Int:        8 * unsafe.Sizeof(int(0)),
		reflect.Float32:    32,
		reflect.Float64:    64,
		reflect.Complex64:  32, // bitlen of each float
		reflect.Complex128: 64, // bitlen of each float
	}

	bitlenValue = [...]uintptr{
		Byte:       8,
		Uint16:     16,
		Uint32:     32,
		Uint64:     64,
		Int16:      16,
		Int32:      32,
		Int64:      64,
		Float32:    32,
		Float64:    64,
		Complex64:  32, // bitlen of each float
		Complex128: 64, // bitlen of each float
	}
)

// bitlen{R,V} enforce static type safety on kind.
func bitlenR(kind reflect.Kind) uintptr { return bitlenReflect[kind] }
func bitlenV(kind Kind) uintptr         { return bitlenValue[kind] }

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

// isRTBytes returns true iff rt is an array or slice of bytes.
func isRTBytes(rt reflect.Type) bool {
	return (rt.Kind() == reflect.Array || rt.Kind() == reflect.Slice) && rt.Elem().Kind() == reflect.Uint8
}

// rtBytes extracts []byte from rv.  Assumes isRTBytes(rv.Type()) == true.
func rtBytes(rv reflect.Value) []byte {
	// Fastpath if the underlying type is []byte
	if rv.Kind() == reflect.Slice && rv.Type().Elem() == rtByte {
		return rv.Bytes()
	}
	// Slowpath copying bytes one by one.
	ret := make([]byte, rv.Len())
	for ix := 0; ix < rv.Len(); ix++ {
		ret[ix] = rv.Index(ix).Convert(rtByte).Interface().(byte)
	}
	return ret
}
