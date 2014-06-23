package val

// Naming conventions to distinguish this package from reflect:
//   tt - refers to *Type
//   vv - refers to *Value
//   rt - refers to reflect.Type
//   rv - refers to reflect.Value

import (
	"fmt"
	"reflect"
	"sync"

	"veyron2/verror"
)

var (
	errConverterInvalid    = verror.BadArgf("invalid Converter dst")
	errConverterUnsettable = verror.BadArgf("unsettable Converter dst")
	errArrayIndex          = verror.BadArgf("array index out of range")
	errFieldNotFound       = verror.NotFoundf("struct field not found")
)

// compatible returns true if types a and b are compatible with each other.
// Type compatibility is a lower threshold than value convertibility; values of
// incompatible types are never convertible, but values of compatible types may
// be inconvertible.  E.g. float32 and byte are compatible, and float32(1.0) is
// convertible with byte(1), but float32(-1.0) is not convertible with any byte
// value.
//
// Compatibility is commutative.  The basic rules:
//   o Bool is only compatible with bool.
//   o TypeVal is only compatible with TypeVal.
//   o Numbers are mutually compatible.
//   o String, enum, []byte and [N]byte are mutually compatible.
//   o Array and list are compatible if their elems are compatible.
//   o Set, map and struct are compatible if keys K* are compatible,
//     and fields F* are compatible:
//     - set[Ka] is compatible with set[Kb] and map[Kb]bool
//       (all bools must be true)
//     - map[string]Fa is compatible with struct{_ Fb, _ Fc, ...}
//     - transitively combining the first two rules:
//       set[string] is compatible with map[string]bool and struct{_ bool, ...}
//       (all bools must be true)
//     - Two structs are compatible if all fields with the same name are
//       compatible, and at least one field as the same name or one of the
//       structs is empty.
//   o OneOf X is compatible with type Y if any type in X is compatible with Y.
//   o Any is compatible with anything.
//
// Recursive types are checked for compatibility up to the first occurrence of a
// cycle in either type.  This leaves open the possibility of "obvious" false
// positives where types are compatible but values are not; this is a tradeoff
// favoring a simpler implementation and better performance over exhaustive
// checking.  This seems fine in practice since type compatibility is weaker
// than value convertibility, and since recursive types are not common.
func compatible(a, b *Type) bool {
	key := compatKey(a, b)
	if compat, ok := compatCache.lookup(key); ok {
		return compat
	}
	// Concurrent updates may cause compatCache to be updated multiple times.
	// This race is benign; we always end up with the same result.
	compat := compat(a, b, make(map[*Type]bool), make(map[*Type]bool))
	compatCache.update(key, compat)
	return compat
}

// compatRegistry is a cache of positive and negative compat results.  It is
// used to improve the performance of compatibility checks.  The only instance
// is the compatCache global cache.
type compatRegistry struct {
	sync.Mutex
	compat map[[2]*Type]bool
}

// TODO(toddw): Change this to a fixed-size LRU cache, otherwise it can grow
// without bounds and exhaust our memory.
var compatCache = compatRegistry{compat: make(map[[2]*Type]bool)}

// The compat cache key is just the two types, which are already hash-consed.
// We make a minor attempt to normalize the order.
func compatKey(a, b *Type) [2]*Type {
	if a.Kind() > b.Kind() ||
		(a.Kind() == Struct && b.Kind() == Struct && a.NumField() > b.NumField()) {
		a, b = b, a
	}
	return [2]*Type{a, b}
}

func (reg compatRegistry) lookup(key [2]*Type) (bool, bool) {
	reg.Lock()
	compat, ok := reg.compat[key]
	reg.Unlock()
	return compat, ok
}

func (reg compatRegistry) update(key [2]*Type, compat bool) {
	reg.Lock()
	reg.compat[key] = compat
	reg.Unlock()
}

// compat is a recursive helper that implements compatible.
func compat(a, b *Type, seenA, seenB map[*Type]bool) bool {
	if a == b || seenA[a] || seenB[b] {
		return true
	}
	seenA[a], seenB[b] = true, true
	// Handle variant cases Any and OneOf
	switch {
	case a.Kind() == Any || b.Kind() == Any:
		return true
	case a.Kind() == OneOf:
		return compatOneOf(a, b, seenA, seenB)
	case b.Kind() == OneOf:
		return compatOneOf(b, a, seenB, seenA)
	}
	// Handle simple scalar types.
	if ax, bx := ttIsNumber(a), ttIsNumber(b); ax || bx {
		return ax && bx
	}
	if ax, bx := a.Kind() == Bool, b.Kind() == Bool; ax || bx {
		return ax && bx
	}
	if ax, bx := a.Kind() == TypeVal, b.Kind() == TypeVal; ax || bx {
		return ax && bx
	}
	// We must check if either a or b is []byte and handle it here first, to
	// ensure it doesn't fall through to the standard array/list handling.  This
	// ensures that []byte isn't compatible with []uint16 and other lists or
	// arrays of numbers.
	if ax, bx := ttIsStringEnumBytes(a), ttIsStringEnumBytes(b); ax || bx {
		return ax && bx
	}
	// Handle composite types.
	switch a.Kind() {
	case Array, List:
		switch b.Kind() {
		case Array, List:
			return compat(a.Elem(), b.Elem(), seenA, seenB)
		}
		return false
	case Set:
		switch b.Kind() {
		case Set:
			return compat(a.Key(), b.Key(), seenA, seenB)
		case Map:
			return compatMapKeyElem(b, a.Key(), BoolType, seenB, seenA)
		case Struct:
			return compatStructKeyElem(b, a.Key(), BoolType, seenB, seenA)
		}
		return false
	case Map:
		switch b.Kind() {
		case Set:
			return compatMapKeyElem(a, b.Key(), BoolType, seenA, seenB)
		case Map:
			return compatMapKeyElem(a, b.Key(), b.Elem(), seenA, seenB)
		case Struct:
			return compatStructKeyElem(b, a.Key(), a.Elem(), seenB, seenA)
		}
		return false
	case Struct:
		switch b.Kind() {
		case Set:
			return compatStructKeyElem(a, b.Key(), BoolType, seenA, seenB)
		case Map:
			return compatStructKeyElem(a, b.Key(), b.Elem(), seenA, seenB)
		case Struct:
			return compatStructStruct(a, b, seenA, seenB)
		}
		return false
	default:
		panic(fmt.Errorf("val: compat unhandled types %q %q", a, b))
	}
}

func ttIsNumber(tt *Type) bool {
	switch tt.Kind() {
	case Byte, Uint16, Uint32, Uint64, Int16, Int32, Int64, Float32, Float64, Complex64, Complex128:
		return true
	}
	return false
}

func ttIsStringEnumBytes(tt *Type) bool {
	return tt.Kind() == String || tt.Kind() == Enum || tt.IsBytes()
}

func ttIsEmptyStruct(tt *Type) bool {
	return tt.Kind() == Struct && tt.NumField() == 0
}

// REQUIRED: a is OneOf
func compatOneOf(a, b *Type, seenA, seenB map[*Type]bool) bool {
	// OneOf is a disjunction - only one of the types needs to be compatible.
	for ax := 0; ax < a.NumOneOfType(); ax++ {
		if compat(a.OneOfType(ax), b, seenA, seenB) {
			return true
		}
	}
	return false
}

// REQUIRED: a is Map
func compatMapKeyElem(a, bKey, bElem *Type, seenA, seenB map[*Type]bool) bool {
	return compat(a.Key(), bKey, seenA, seenB) && compat(a.Elem(), bElem, seenA, seenB)
}

// REQUIRED: a is Struct
func compatStructKeyElem(a, bKey, bElem *Type, seenA, seenB map[*Type]bool) bool {
	// Struct is a conjunction, all fields must be compatible.
	if ttIsEmptyStruct(a) {
		return false // empty struct isn't compatible with set or map
	}
	if !compat(StringType, bKey, seenA, seenB) {
		return false
	}
	for ax := 0; ax < a.NumField(); ax++ {
		if !compat(a.Field(ax).Type, bElem, seenA, seenB) {
			return false
		}
	}
	return true
}

// REQUIRED: a and b are Struct
func compatStructStruct(a, b *Type, seenA, seenB map[*Type]bool) bool {
	// Struct is a conjunction, all fields with the same name must be compatible.
	if ttIsEmptyStruct(a) || ttIsEmptyStruct(b) {
		return true // empty struct is compatible all other structs
	}
	if a.NumField() > b.NumField() {
		a, b, seenA, seenB = b, a, seenB, seenA
	}
	fieldMatch := false
	for ax := 0; ax < a.NumField(); ax++ {
		afield := a.Field(ax)
		bfield, bindex := b.FieldByName(afield.Name)
		if bindex < 0 {
			continue
		}
		if !compat(afield.Type, bfield.Type, seenA, seenB) {
			return false
		}
		fieldMatch = true
	}
	// At least one field must have matched.
	return fieldMatch
}

// Conv represents the state and logic for value conversion.  It is designed to
// allow filling a destination value incrementally from a source, e.g. by a
// stream-based decoder.  Used in this style, the From+ methods fill in scalar
// values, and the StartComposite / EndComposite methods bracket composite
// values.  It may also be used to perform direct value conversion via the From
// method.
//
// The zero Conv is invalid; use Converter to create one.
type Conv struct {
	// Conceptually this is a disjoint union between the two types of destination
	// values: *Value and reflect.Value.  Only one of the vv and rv fields is ever
	// valid.  We split into two fields and perform "if vv == nil" checks in each
	// method rather than using an interface for performance reasons.
	//
	// The tt field is always non-nil, and represents the type of the value.
	tt *Type
	vv *Value
	rv reflect.Value
}

// Converter returns a Conv object for value conversion into dst.  There are
// some rules depending on the type of dst:
//   o If dst is a valid *Value, it is filled in directly.
//   o Otherwise dst must be a settable value (i.e. it must be a pointer).
//   o Pointers are followed, and logically "flattened".
//   o New values are automatically created for nil pointers.
//   o If the flattened dst is convertible to *Type, it is filled in directly.
//   o If the flattened dst is convertible to *Value, a valid *Value is created
//     and filled in.
func Converter(dst interface{}) (Conv, error) {
	rv := reflect.ValueOf(dst)
	tt, err := TypeFromReflect(rv.Type())
	if err != nil {
		return Conv{}, err
	}
	return reflectConv(rv, tt)
}

func reflectConv(rv reflect.Value, tt *Type) (Conv, error) {
	if !rv.IsValid() {
		return Conv{}, errConverterInvalid
	}
	if vv := extractValue(rv); vv.IsValid() {
		return valueConv(vv), nil
	}
	if !rv.CanSet() && rv.Kind() == reflect.Ptr && !rv.IsNil() {
		// Dereference the pointer a single time to make rv settable.
		rv = rv.Elem()
	}
	if !rv.CanSet() {
		return Conv{}, errConverterUnsettable
	}
	return Conv{tt: tt, rv: rv}, nil
}

func valueConv(vv *Value) Conv {
	return Conv{tt: vv.Type(), vv: vv}
}

func extractValue(rv reflect.Value) *Value {
	for rv.Kind() == reflect.Ptr {
		switch {
		case rv.IsNil():
			return nil
		case rv.Type().ConvertibleTo(rtPtrToValue):
			return rv.Convert(rtPtrToValue).Interface().(*Value)
		}
		rv = rv.Elem()
	}
	return nil
}

// startConvert prepares to fill in c, converting from type tt.  Returns fin and
// fill which are used by endConvert to finish the conversion; fin represents
// the final value to assign to, and fill represents the value to actually fill
// in.
func startConvert(c Conv, tt *Type) (fin, fill Conv, _ error) {
	if tt.Kind() == Any || tt.Kind() == OneOf {
		return Conv{}, Conv{}, fmt.Errorf("variant type %q used as conversion src", tt)
	}
	if !compatible(c.tt, tt) {
		return Conv{}, Conv{}, fmt.Errorf("types %q and %q aren't compatible", c.tt, tt)
	}
	if c.vv == nil {
		// Walk pointers down to their base value, creating new values as necessary.
		for c.rv.Kind() == reflect.Ptr {
			if c.rv.Type().Elem() == rtValue {
				// c.rv has underlying type *Value, fill from it directly.
				vv := c.rv.Convert(rtPtrToValue).Interface().(*Value)
				if !vv.IsValid() {
					vv = Zero(tt)
					c.rv.Set(reflect.ValueOf(vv).Convert(c.rv.Type()))
				}
				return c, valueConv(vv), nil
			}
			if c.rv.IsNil() {
				c.rv.Set(reflect.New(c.rv.Type().Elem()))
			}
			if c.rv.Type().Elem() == rtType {
				// Stop at *Type to allow TypeVal values to be assigned.
				return c, c, nil
			}
			c.rv = c.rv.Elem()
		}
		switch c.rv.Kind() {
		case reflect.Interface:
			// Create a concrete *Value to convert to, which will be assigned to the
			// interface in endConvert.
			//
			// TODO(toddw): Add type registration and create real Go objects?
			if !rtPtrToValue.AssignableTo(c.rv.Type()) {
				return Conv{}, Conv{}, fmt.Errorf("%v not assignable from *Value", c.rv.Type())
			}
			return c, valueConv(Zero(tt)), nil
		case reflect.Array, reflect.Slice, reflect.Map:
			c.rv.Set(reflect.Zero(c.rv.Type())) // start with zero collections
		}
	} else {
		switch c.vv.Kind() {
		case Any, OneOf:
			if !c.tt.AssignableFrom(tt) {
				return Conv{}, Conv{}, fmt.Errorf("%v not assignable from %v", c.tt, tt)
			}
			return c, valueConv(Zero(tt)), nil
		case Array, List, Set, Map:
			c.vv.Assign(nil) // start with zero collections
		}
	}
	return c, c, nil
}

// endConvert finishes converting a value, taking the fin and fill returned by
// startConvert.  This is necessary since interface/any/oneof values are
// assigned by value, and can't be filled in by reference.
func endConvert(fin, fill Conv) {
	if fin.vv == nil {
		switch fin.rv.Kind() {
		case reflect.Interface:
			fin.rv.Set(reflect.ValueOf(fill.vv))
		}
	} else {
		switch fin.vv.Kind() {
		case Any, OneOf:
			fin.vv.Assign(fill.vv)
		}
	}
}

// FromBool fills in the c dst, converting from the src bool, using tt to create
// new values if the c dst has a variant type.
func (c Conv) FromBool(src bool, tt *Type) error {
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromBool(src); err != nil {
		return err
	}
	endConvert(fin, fill)
	return nil
}

// FromUint fills in the c dst, converting from the src uint, using tt to create
// new values if the c dst has a variant type.
func (c Conv) FromUint(src uint64, tt *Type) error {
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromUint(src); err != nil {
		return err
	}
	endConvert(fin, fill)
	return nil
}

// FromInt fills in the c dst, converting from the src int, using tt to create
// new values if the c dst has a variant type.
func (c Conv) FromInt(src int64, tt *Type) error {
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromInt(src); err != nil {
		return err
	}
	endConvert(fin, fill)
	return nil
}

// FromFloat fills in the c dst, converting from the src float, using tt to
// create new values if the c dst has a variant type.
func (c Conv) FromFloat(src float64, tt *Type) error {
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromFloat(src); err != nil {
		return err
	}
	endConvert(fin, fill)
	return nil
}

// FromComplex fills in the c dst, converting from the src complex, using tt to
// create new values if the c dst has a variant type.
func (c Conv) FromComplex(src complex128, tt *Type) error {
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromComplex(src); err != nil {
		return err
	}
	endConvert(fin, fill)
	return nil
}

// FromBytes fills in the c dst, converting from the src bytes, using tt to
// create new values if the c dst has a variant type.
func (c Conv) FromBytes(src []byte, tt *Type) error {
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromBytes(src); err != nil {
		return err
	}
	endConvert(fin, fill)
	return nil
}

// FromString fills in the c dst, converting from the src string, using tt to
// create new values if the c dst has a variant type.
func (c Conv) FromString(src string, tt *Type) error {
	// TODO(toddw): Speed this up.
	return c.FromBytes([]byte(src), tt)
}

// FromTypeVal fills in the c dst, converting from the src type.  There is no tt
// argument since TypeVal only has a single type representation.
func (c Conv) FromTypeVal(src *Type) error {
	fin, fill, err := startConvert(c, TypeValType)
	if err != nil {
		return err
	}
	if err := fill.fromTypeVal(src); err != nil {
		return err
	}
	endConvert(fin, fill)
	return nil
}

// FromNil fills in the c dst, converting from the src nil value, using tt to
// create new values if the c dst has a variant type.
func (c Conv) FromNil(tt *Type) error {
	// TODO(toddw): Is this right?
	return c.fromValue(Zero(tt))
}

func (c Conv) fromBool(src bool) error {
	if c.vv == nil {
		if c.rv.Kind() == reflect.Bool {
			c.rv.SetBool(src)
			return nil
		}
	} else {
		if c.vv.Kind() == Bool {
			c.vv.AssignBool(src)
			return nil
		}
	}
	return fmt.Errorf("invalid conversion from bool to %v", c.tt)
}

func (c Conv) fromUint(src uint64) error {
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
			if !overflowUint(src, bitlenR(kind)) {
				c.rv.SetUint(src)
				return nil
			}
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			if isrc, ok := convertUintToInt(src, bitlenR(kind)); ok {
				c.rv.SetInt(isrc)
				return nil
			}
		case reflect.Float32, reflect.Float64:
			if fsrc, ok := convertUintToFloat(src, bitlenR(kind)); ok {
				c.rv.SetFloat(fsrc)
				return nil
			}
		case reflect.Complex64, reflect.Complex128:
			if fsrc, ok := convertUintToFloat(src, bitlenR(kind)); ok {
				c.rv.SetComplex(complex(fsrc, 0))
				return nil
			}
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case Byte:
			if !overflowUint(src, bitlenV(kind)) {
				c.vv.AssignByte(byte(src))
				return nil
			}
		case Uint16, Uint32, Uint64:
			if !overflowUint(src, bitlenV(kind)) {
				c.vv.AssignUint(src)
				return nil
			}
		case Int16, Int32, Int64:
			if isrc, ok := convertUintToInt(src, bitlenV(kind)); ok {
				c.vv.AssignInt(isrc)
				return nil
			}
		case Float32, Float64:
			if fsrc, ok := convertUintToFloat(src, bitlenV(kind)); ok {
				c.vv.AssignFloat(fsrc)
				return nil
			}
		case Complex64, Complex128:
			if fsrc, ok := convertUintToFloat(src, bitlenV(kind)); ok {
				c.vv.AssignComplex(complex(fsrc, 0))
				return nil
			}
		}
	}
	return fmt.Errorf("invalid conversion from uint(%d) to %v", src, c.tt)
}

func (c Conv) fromInt(src int64) error {
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
			if usrc, ok := convertIntToUint(src, bitlenR(kind)); ok {
				c.rv.SetUint(usrc)
				return nil
			}
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			if !overflowInt(src, bitlenR(kind)) {
				c.rv.SetInt(src)
				return nil
			}
		case reflect.Float32, reflect.Float64:
			if fsrc, ok := convertIntToFloat(src, bitlenR(kind)); ok {
				c.rv.SetFloat(fsrc)
				return nil
			}
		case reflect.Complex64, reflect.Complex128:
			if fsrc, ok := convertIntToFloat(src, bitlenR(kind)); ok {
				c.rv.SetComplex(complex(fsrc, 0))
				return nil
			}
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case Byte:
			if usrc, ok := convertIntToUint(src, bitlenV(kind)); ok {
				c.vv.AssignByte(byte(usrc))
				return nil
			}
		case Uint16, Uint32, Uint64:
			if usrc, ok := convertIntToUint(src, bitlenV(kind)); ok {
				c.vv.AssignUint(usrc)
				return nil
			}
		case Int16, Int32, Int64:
			if !overflowInt(src, bitlenV(kind)) {
				c.vv.AssignInt(src)
				return nil
			}
		case Float32, Float64:
			if fsrc, ok := convertIntToFloat(src, bitlenV(kind)); ok {
				c.vv.AssignFloat(fsrc)
				return nil
			}
		case Complex64, Complex128:
			if fsrc, ok := convertIntToFloat(src, bitlenV(kind)); ok {
				c.vv.AssignComplex(complex(fsrc, 0))
				return nil
			}
		}
	}
	return fmt.Errorf("invalid conversion from int(%d) to %v", src, c.tt)
}

func (c Conv) fromFloat(src float64) error {
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
			if usrc, ok := convertFloatToUint(src, bitlenR(kind)); ok {
				c.rv.SetUint(usrc)
				return nil
			}
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			if isrc, ok := convertFloatToInt(src, bitlenR(kind)); ok {
				c.rv.SetInt(isrc)
				return nil
			}
		case reflect.Float32, reflect.Float64:
			c.rv.SetFloat(convertFloatToFloat(src, bitlenR(kind)))
			return nil
		case reflect.Complex64, reflect.Complex128:
			c.rv.SetComplex(complex(convertFloatToFloat(src, bitlenR(kind)), 0))
			return nil
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case Byte:
			if usrc, ok := convertFloatToUint(src, bitlenV(kind)); ok {
				c.vv.AssignByte(byte(usrc))
				return nil
			}
		case Uint16, Uint32, Uint64:
			if usrc, ok := convertFloatToUint(src, bitlenV(kind)); ok {
				c.vv.AssignUint(usrc)
				return nil
			}
		case Int16, Int32, Int64:
			if isrc, ok := convertFloatToInt(src, bitlenV(kind)); ok {
				c.vv.AssignInt(isrc)
				return nil
			}
		case Float32, Float64:
			c.vv.AssignFloat(convertFloatToFloat(src, bitlenV(kind)))
			return nil
		case Complex64, Complex128:
			c.vv.AssignComplex(complex(convertFloatToFloat(src, bitlenV(kind)), 0))
			return nil
		}
	}
	return fmt.Errorf("invalid conversion from float(%g) to %v", src, c.tt)
}

func (c Conv) fromComplex(src complex128) error {
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
			if usrc, ok := convertComplexToUint(src, bitlenR(kind)); ok {
				c.rv.SetUint(usrc)
				return nil
			}
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			if isrc, ok := convertComplexToInt(src, bitlenR(kind)); ok {
				c.rv.SetInt(isrc)
				return nil
			}
		case reflect.Float32, reflect.Float64:
			if imag(src) == 0 {
				c.rv.SetFloat(convertFloatToFloat(real(src), bitlenR(kind)))
				return nil
			}
		case reflect.Complex64, reflect.Complex128:
			re := convertFloatToFloat(real(src), bitlenR(kind))
			im := convertFloatToFloat(imag(src), bitlenR(kind))
			c.rv.SetComplex(complex(re, im))
			return nil
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case Byte:
			if usrc, ok := convertComplexToUint(src, bitlenV(kind)); ok {
				c.vv.AssignByte(byte(usrc))
				return nil
			}
		case Uint16, Uint32, Uint64:
			if usrc, ok := convertComplexToUint(src, bitlenV(kind)); ok {
				c.vv.AssignUint(usrc)
				return nil
			}
		case Int16, Int32, Int64:
			if isrc, ok := convertComplexToInt(src, bitlenV(kind)); ok {
				c.vv.AssignInt(isrc)
				return nil
			}
		case Float32, Float64:
			if imag(src) == 0 {
				c.vv.AssignFloat(convertFloatToFloat(real(src), bitlenV(kind)))
				return nil
			}
		case Complex64, Complex128:
			re := convertFloatToFloat(real(src), bitlenV(kind))
			im := convertFloatToFloat(imag(src), bitlenV(kind))
			c.vv.AssignComplex(complex(re, im))
			return nil
		}
	}
	return fmt.Errorf("invalid conversion from complex(%g) to %v", src, c.tt)
}

func (c Conv) fromBytes(src []byte) error {
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.String:
			c.rv.SetString(string(src)) // TODO(toddw): check utf8
			return nil
		case reflect.Array:
			if c.rv.Type().Elem() == rtByte && c.rv.Len() == len(src) {
				reflect.Copy(c.rv, reflect.ValueOf(src))
				return nil
			}
		case reflect.Slice:
			if c.rv.Type().Elem() == rtByte {
				cp := make([]byte, len(src))
				copy(cp, src)
				c.rv.SetBytes(cp)
				return nil
			}
		}
		// TODO(toddw): Handle enum if it's represented as an index?
	} else {
		switch kind := c.vv.Kind(); kind {
		case String:
			c.vv.AssignString(string(src)) // TODO(toddw): check utf8
			return nil
		case Array:
			if c.tt.IsBytes() && c.tt.Len() == len(src) {
				c.vv.AssignBytes(src)
				return nil
			}
		case List:
			if c.tt.IsBytes() {
				c.vv.AssignBytes(src)
				return nil
			}
		case Enum:
			if index := c.tt.EnumIndex(string(src)); index >= 0 {
				c.vv.rep = enumIndex(index)
				return nil
			}
		}
	}
	return fmt.Errorf("invalid conversion from []byte to %v", c.tt)
}

func (c Conv) fromTypeVal(src *Type) error {
	if c.vv == nil {
		if rtPtrToType.ConvertibleTo(c.rv.Type()) {
			c.rv.Set(reflect.ValueOf(src).Convert(c.rv.Type()))
			return nil
		}
	} else {
		if c.tt == TypeValType {
			c.vv.AssignTypeVal(src)
			return nil
		}
	}
	return fmt.Errorf("invalid conversion from typeval to %v", c.tt)
}

// From fills in the c dst, converting from the src value.  Similar rules are
// obeyed as the c dst, depending on the type of src:
//   o If src is a valid *Value, the semantic value represented by src is used.
//   o If src is a valid *Type, FromTypeVal is used.
//   o Pointers are follwed and "flattened".
func (c Conv) From(src interface{}) error {
	return c.fromReflect(reflect.ValueOf(src))
}

func (c Conv) fromReflect(rv reflect.Value) error {
	rt := rv.Type()
	tt, err := TypeFromReflect(rt)
	if err != nil {
		return err
	}
	// Flatten pointers in rv and handle special cases.
	for rt.Kind() == reflect.Ptr {
		switch {
		case rv.IsNil():
			return c.FromNil(tt)
		case rt.ConvertibleTo(rtPtrToType):
			// If rv is convertible to *Type, fill from it directly.
			return c.FromTypeVal(rv.Convert(rtPtrToType).Interface().(*Type))
		case rt.ConvertibleTo(rtPtrToValue):
			// If rv is convertible to *Value, fill from it directly.
			return c.fromValue(rv.Convert(rtPtrToValue).Interface().(*Value))
		}
		rt, rv = rt.Elem(), rv.Elem()
	}
	// Recursive walk through the reflect value to fill in c.
	if isRTBytes(rt) {
		return c.FromBytes(rtBytes(rv), tt)
	}
	switch rt.Kind() {
	case reflect.Interface:
		if !rv.IsNil() {
			return c.fromReflect(rv.Elem())
		}
		return nil
	case reflect.Bool:
		return c.FromBool(rv.Bool(), tt)
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
		return c.FromUint(rv.Uint(), tt)
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		return c.FromInt(rv.Int(), tt)
	case reflect.Float32, reflect.Float64:
		return c.FromFloat(rv.Float(), tt)
	case reflect.Complex64, reflect.Complex128:
		return c.FromComplex(rv.Complex(), tt)
	case reflect.String:
		return c.FromString(rv.String(), tt)
	case reflect.Array, reflect.Slice:
		comp, err := c.StartComposite(tt)
		if err != nil {
			return err
		}
		for ix := 0; ix < rv.Len(); ix++ {
			elem, err := comp.StartElem(ix)
			if err != nil {
				return err
			}
			if err := elem.fromReflect(rv.Index(ix)); err != nil {
				return err
			}
			if err := comp.EndElem(elem); err != nil {
				return err
			}
		}
		c.EndComposite(comp)
		return nil
	case reflect.Map:
		comp, err := c.StartComposite(tt)
		if err != nil {
			return err
		}
		for _, rvkey := range rv.MapKeys() {
			key, err := comp.StartKey()
			if err != nil {
				return err
			}
			if err := key.fromReflect(rvkey); err != nil {
				return err
			}
			if tt.Kind() == Set {
				// The map is actually a set
				if err := comp.EndKey(key); err != nil && !verror.Is(err, verror.NotFound) {
					return err
				}
				continue
			}
			field, err := comp.EndKeyStartField(key)
			switch {
			case verror.Is(err, verror.NotFound):
				continue
			case err != nil:
				return err
			}
			if err := field.fromReflect(rv.MapIndex(rvkey)); err != nil {
				return err
			}
			if err := comp.EndField(key, field); err != nil {
				return err
			}
		}
		c.EndComposite(comp)
		return nil
	case reflect.Struct:
		comp, err := c.StartComposite(tt)
		if err != nil {
			return err
		}
		for fx := 0; fx < rt.NumField(); fx++ {
			key, field, err := comp.StartField(rt.Field(fx).Name)
			switch {
			case verror.Is(err, verror.NotFound):
				continue
			case err != nil:
				return err
			}
			if err := field.fromReflect(rv.Field(fx)); err != nil {
				return err
			}
			if err := comp.EndField(key, field); err != nil {
				return err
			}
		}
		c.EndComposite(comp)
		return nil
	default:
		return fmt.Errorf("fromReflect invalid type %v", rt)
	}
}

func (c Conv) fromValue(vv *Value) error {
	tt := vv.Type()
	if tt.IsBytes() {
		return c.FromBytes(vv.Bytes(), tt)
	}
	switch vv.Kind() {
	case Any, OneOf:
		if elem := vv.Elem(); elem != nil {
			return c.fromValue(elem)
		}
		return nil
	case Bool:
		return c.FromBool(vv.Bool(), tt)
	case Byte:
		return c.FromUint(uint64(vv.Byte()), tt)
	case Uint16, Uint32, Uint64:
		return c.FromUint(vv.Uint(), tt)
	case Int16, Int32, Int64:
		return c.FromInt(vv.Int(), tt)
	case Float32, Float64:
		return c.FromFloat(vv.Float(), tt)
	case Complex64, Complex128:
		return c.FromComplex(vv.Complex(), tt)
	case String:
		return c.FromString(vv.RawString(), tt)
	case Enum:
		return c.FromString(vv.EnumLabel(), tt)
	case TypeVal:
		return c.FromTypeVal(vv.TypeVal())
	case Array, List:
		comp, err := c.StartComposite(tt)
		if err != nil {
			return err
		}
		for ix := 0; ix < vv.Len(); ix++ {
			elem, err := comp.StartElem(ix)
			if err != nil {
				return err
			}
			if err := elem.fromValue(vv.Index(ix)); err != nil {
				return err
			}
			if err := comp.EndElem(elem); err != nil {
				return err
			}
		}
		c.EndComposite(comp)
		return nil
	case Set:
		comp, err := c.StartComposite(tt)
		if err != nil {
			return err
		}
		for _, vvkey := range vv.Keys() {
			key, err := comp.StartKey()
			if err != nil {
				return err
			}
			if err := key.fromValue(vvkey); err != nil {
				return err
			}
			if err := comp.EndKey(key); err != nil && !verror.Is(err, verror.NotFound) {
				return err
			}
		}
		c.EndComposite(comp)
		return nil
	case Map:
		comp, err := c.StartComposite(tt)
		if err != nil {
			return err
		}
		for _, vvkey := range vv.Keys() {
			key, err := comp.StartKey()
			if err != nil {
				return err
			}
			if err := key.fromValue(vvkey); err != nil {
				return err
			}
			field, err := comp.EndKeyStartField(key)
			switch {
			case verror.Is(err, verror.NotFound):
				continue
			case err != nil:
				return err
			}
			if err := field.fromValue(vv.MapIndex(vvkey)); err != nil {
				return err
			}
			if err := comp.EndField(key, field); err != nil {
				return err
			}
		}
		c.EndComposite(comp)
		return nil
	case Struct:
		comp, err := c.StartComposite(tt)
		if err != nil {
			return err
		}
		for fx := 0; fx < tt.NumField(); fx++ {
			key, field, err := comp.StartField(tt.Field(fx).Name)
			switch {
			case verror.Is(err, verror.NotFound):
				continue
			case err != nil:
				return err
			}
			if err := field.fromValue(vv.Field(fx)); err != nil {
				return err
			}
			if err := comp.EndField(key, field); err != nil {
				return err
			}
		}
		c.EndComposite(comp)
		return nil
	default:
		panic(fmt.Errorf("val: fromValue unhandled %v %v", tt.Kind(), tt))
	}
}

// StartComposite prepares to fill a composite value into c, using tt to create
// new values if the c dst has a variant type.  EndComposite must be called to
// finish filling the composite value.
func (c Conv) StartComposite(tt *Type) (CompositeConv, error) {
	fin, fill, err := startConvert(c, tt)
	return CompositeConv{fin, fill}, err
}

// EndComposite finishes filling a composite value into c.
func (c Conv) EndComposite(comp CompositeConv) {
	endConvert(comp.fin, comp.fill)
}

// CompositeConv represents the state and logic for composite value conversion.
// The zero CompositeConv is invalid; use Conv.StartComposite to create one.
type CompositeConv struct {
	fin, fill Conv // fields returned by startConvert.
}

// StartElem prepares to convert the next sequence (Array or List) element at
// index.  Typically elements in a sequence are filled in strictly increasing
// order, starting from 0.  The returned elem should be filled in, and then
// EndElem(elem) must be called.
func (cc CompositeConv) StartElem(index int) (elem Conv, _ error) {
	return cc.fill.startElem(index)
}

// EndElem finishes converting elem, which must have been returned by a previous
// call to StartElem.
func (cc CompositeConv) EndElem(elem Conv) error {
	return cc.fill.endElem(elem)
}

// StartKey prepares to convert the next Set, Map or Struct key.  The order of
// starting keys doesn't matter.  The returned key should be filled in, and then
// one of EndKeyStartField(key) or EndKey(key) must be called.
func (cc CompositeConv) StartKey() (key Conv, _ error) {
	return cc.fill.startKey()
}

// EndKeyStartField finishes converting the Map or Struct key, which must have
// been returned by a previous call to StartKey.  It also prepares to convert
// the field associated with the key.  The returned field should be filled in,
// and then EndField(key, field) must be called.
func (cc CompositeConv) EndKeyStartField(key Conv) (field Conv, _ error) {
	return cc.fill.endKeyStartField(key)
}

// EndField finishes converting the Map or Struct key and field, which must have
// been returned by previous calls to StartKey and EndKeyStartField, or
// StartField.
func (cc CompositeConv) EndField(key, field Conv) error {
	return cc.fill.endField(key, field)
}

// StartField prepares to convert the next Map or Struct field with the given
// key name.  The order of starting fields doesn't matter.  The returned field
// should be filled in, and then EndField(key, field) must be called.
func (cc CompositeConv) StartField(name string) (key, field Conv, _ error) {
	// StartField is a helper to call StartKey + FromString(name) + EndKeyStartField
	var err error
	if key, err = cc.StartKey(); err != nil {
		return Conv{}, Conv{}, err
	}
	if err = key.FromString(name, StringType); err != nil {
		return Conv{}, Conv{}, err
	}
	if field, err = cc.EndKeyStartField(key); err != nil {
		return Conv{}, Conv{}, err
	}
	return
}

// EndKey finishes converting the Set key, which must have been returned by a
// previous call to StartKey.
func (cc CompositeConv) EndKey(key Conv) error {
	// EndField is a helper to call EndKeyStartField + EndField
	field, err := cc.EndKeyStartField(key)
	if err != nil {
		return err
	}
	return cc.EndField(key, field)
}

func (c Conv) startElem(index int) (Conv, error) {
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Array:
			if index >= c.rv.Len() {
				return Conv{}, errArrayIndex
			}
			return reflectConv(c.rv.Index(index), c.tt.Elem())
		case reflect.Slice:
			newlen := index + 1
			if newlen < c.rv.Len() {
				newlen = c.rv.Len()
			}
			if newlen > c.rv.Cap() {
				rvNew := reflect.MakeSlice(c.rv.Type(), newlen, newlen*2)
				reflect.Copy(rvNew, c.rv)
				c.rv.Set(rvNew)
			} else {
				c.rv.SetLen(newlen)
			}
			return reflectConv(c.rv.Index(index), c.tt.Elem())
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case Array:
			if index >= c.vv.Len() {
				return Conv{}, errArrayIndex
			}
			return valueConv(c.vv.Index(index)), nil
		case List:
			newlen := index + 1
			if newlen < c.vv.Len() {
				newlen = c.vv.Len()
			}
			return valueConv(c.vv.AssignLen(newlen).Index(index)), nil
		}
	}
	return Conv{}, fmt.Errorf("type %v doesn't support StartElem", c.tt)
}

func (c Conv) endElem(elem Conv) error {
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Array, reflect.Slice:
			return nil
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case Array, List:
			return nil
		}
	}
	return fmt.Errorf("type %v doesn't support EndElem", c.tt)
}

func (c Conv) startKey() (Conv, error) {
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Map:
			return reflectConv(reflect.New(c.rv.Type().Key()).Elem(), c.tt.Key())
		case reflect.Struct:
			// The key for structs is the field name, which is a string.
			return reflectConv(reflect.New(rtString).Elem(), StringType)
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case Set, Map:
			return valueConv(Zero(c.tt.Key())), nil
		case Struct:
			// The key for structs is the field name, which is a string.
			return valueConv(Zero(StringType)), nil
		}
	}
	return Conv{}, fmt.Errorf("type %v doesn't support StartKey", c.tt)
}

func (c Conv) endKeyStartField(key Conv) (Conv, error) {
	// There are various special-cases regarding bool values below.  These are to
	// handle different representations of sets; the following types are all
	// convertible to each other:
	//   set[string], map[string]bool, struct{X, Y, Z bool}
	//
	// To deal with these cases in a uniform manner, we return a bool field for
	// set[string], and initialize bool fields to true.
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Map:
			var rvField reflect.Value
			var ttField *Type
			switch rtField := c.rv.Type().Elem(); {
			case c.tt.Kind() == Set:
				// The map actually represents a set
				rvField = reflect.New(rtBool).Elem()
				rvField.SetBool(true)
				ttField = BoolType
			case rtField.Kind() == reflect.Bool:
				rvField = reflect.New(rtField).Elem()
				rvField.SetBool(true)
				ttField = c.tt.Elem()
			default:
				rvField = reflect.New(rtField).Elem()
				ttField = c.tt.Elem()
			}
			return reflectConv(rvField, ttField)
		case reflect.Struct:
			rvField := c.rv.FieldByName(key.rv.String())
			ttField, index := c.tt.FieldByName(key.rv.String())
			if !rvField.IsValid() || index < 0 {
				// TODO(toddw): Add a way to track extra and missing fields.
				return Conv{}, errFieldNotFound
			}
			if rvField.Kind() == reflect.Bool {
				rvField.SetBool(true)
			}
			return reflectConv(rvField, ttField.Type)
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case Set:
			return valueConv(BoolValue(true)), nil
		case Map:
			vvField := Zero(c.tt.Elem())
			if vvField.Kind() == Bool {
				vvField.AssignBool(true)
			}
			return valueConv(vvField), nil
		case Struct:
			_, index := c.tt.FieldByName(key.vv.RawString())
			if index < 0 {
				// TODO(toddw): Add a way to track extra and missing fields.
				return Conv{}, errFieldNotFound
			}
			vvField := c.vv.Field(index)
			if vvField.Kind() == Bool {
				vvField.AssignBool(true)
			}
			return valueConv(vvField), nil
		}
	}
	return Conv{}, fmt.Errorf("type %v doesn't support EndKeyStartField", c.tt)
}

func (c Conv) endField(key, field Conv) error {
	// The special-case handling of bool fields matches the special-cases in
	// EndKeyStartField.
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Map:
			if c.rv.IsNil() {
				c.rv.Set(reflect.MakeMap(c.rv.Type()))
			}
			rvField := field.rv
			if c.tt.Kind() == Set {
				// The map actually represents a set
				if !field.rv.Bool() {
					return fmt.Errorf("%v can only be converted from true fields", c.tt)
				}
				rvField = reflect.Zero(c.rv.Type().Elem())
			}
			c.rv.SetMapIndex(key.rv, rvField)
			return nil
		case reflect.Struct:
			return nil
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case Set:
			if !field.vv.Bool() {
				return fmt.Errorf("%v can only be converted from true fields", c.tt)
			}
			c.vv.AssignSetKey(key.vv)
			return nil
		case Map:
			c.vv.AssignMapIndex(key.vv, field.vv)
			return nil
		case Struct:
			return nil
		}
	}
	return fmt.Errorf("type %v doesn't support EndField", c.tt)
}

// Convert converts to dst from src.  It basically performs
// Converter(dst).From(src) - see those methods for details.
func Convert(dst, src interface{}) error {
	c, err := Converter(dst)
	if err != nil {
		return err
	}
	return c.From(src)
}

// ValueOf returns the *Value corresponding to src.
func ValueOf(src interface{}) (*Value, error) {
	var result *Value
	if err := Convert(&result, src); err != nil {
		return nil, err
	}
	return result, nil
}
