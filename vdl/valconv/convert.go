package valconv

// Naming conventions to distinguish this package from reflect:
//   tt - refers to *vdl.Type
//   vv - refers to *vdl.Value
//   rt - refers to reflect.Type
//   rv - refers to reflect.Value

import (
	"fmt"
	"reflect"
	"sync"

	"veyron2/vdl"
	"veyron2/verror"
)

var (
	errTargetInvalid    = verror.BadArgf("invalid target")
	errTargetUnsettable = verror.BadArgf("unsettable target")
	errArrayIndex       = verror.BadArgf("array index out of range")
	errFieldNotFound    = verror.NotFoundf("struct field not found")
)

var (
	rtByte       = reflect.TypeOf(byte(0))
	rtBool       = reflect.TypeOf(bool(false))
	rtString     = reflect.TypeOf(string(""))
	rtType       = reflect.TypeOf(vdl.Type{})
	rtValue      = reflect.TypeOf(vdl.Value{})
	rtPtrToType  = reflect.PtrTo(rtType)
	rtPtrToValue = reflect.PtrTo(rtValue)
)

// compatible returns true if types a and b are compatible with each other.
// Type compatibility is a lower threshold than value convertibility; values of
// incompatible types are never convertible, while values of compatible types
// might not be convertible.  E.g. float32 and byte are compatible, and
// float32(1.0) is convertible with byte(1), but float32(-1.0) is not
// convertible with any byte value.
//
// Compatibility is commutative.  The basic rules:
//   o Nilability is ignored for all rules (e.g. ?int is compatible with int).
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
func compatible(a, b *vdl.Type) bool {
	if a.Kind() == vdl.Nilable {
		a = a.Elem()
	}
	if b.Kind() == vdl.Nilable {
		b = b.Elem()
	}
	key := compatKey(a, b)
	if compat, ok := compatCache.lookup(key); ok {
		return compat
	}
	// Concurrent updates may cause compatCache to be updated multiple times.
	// This race is benign; we always end up with the same result.
	compat := compat(a, b, make(map[*vdl.Type]bool), make(map[*vdl.Type]bool))
	compatCache.update(key, compat)
	return compat
}

// compatRegistry is a cache of positive and negative compat results.  It is
// used to improve the performance of compatibility checks.  The only instance
// is the compatCache global cache.
type compatRegistry struct {
	sync.Mutex
	compat map[[2]*vdl.Type]bool
}

// TODO(toddw): Change this to a fixed-size LRU cache, otherwise it can grow
// without bounds and exhaust our memory.
var compatCache = compatRegistry{compat: make(map[[2]*vdl.Type]bool)}

// The compat cache key is just the two types, which are already hash-consed.
// We make a minor attempt to normalize the order.
func compatKey(a, b *vdl.Type) [2]*vdl.Type {
	if a.Kind() > b.Kind() ||
		(a.Kind() == vdl.Struct && b.Kind() == vdl.Struct && a.NumField() > b.NumField()) {
		a, b = b, a
	}
	return [2]*vdl.Type{a, b}
}

func (reg compatRegistry) lookup(key [2]*vdl.Type) (bool, bool) {
	reg.Lock()
	compat, ok := reg.compat[key]
	reg.Unlock()
	return compat, ok
}

func (reg compatRegistry) update(key [2]*vdl.Type, compat bool) {
	reg.Lock()
	reg.compat[key] = compat
	reg.Unlock()
}

// compat is a recursive helper that implements compatible.
func compat(a, b *vdl.Type, seenA, seenB map[*vdl.Type]bool) bool {
	if a.Kind() == vdl.Nilable {
		a = a.Elem()
	}
	if b.Kind() == vdl.Nilable {
		b = b.Elem()
	}
	if a == b || seenA[a] || seenB[b] {
		return true
	}
	seenA[a], seenB[b] = true, true
	// Handle variant cases Any and OneOf
	switch {
	case a.Kind() == vdl.Any || b.Kind() == vdl.Any:
		return true
	case a.Kind() == vdl.OneOf:
		return compatOneOf(a, b, seenA, seenB)
	case b.Kind() == vdl.OneOf:
		return compatOneOf(b, a, seenB, seenA)
	}
	// Handle simple scalar vdl.
	if ax, bx := ttIsNumber(a), ttIsNumber(b); ax || bx {
		return ax && bx
	}
	if ax, bx := a.Kind() == vdl.Bool, b.Kind() == vdl.Bool; ax || bx {
		return ax && bx
	}
	if ax, bx := a.Kind() == vdl.TypeVal, b.Kind() == vdl.TypeVal; ax || bx {
		return ax && bx
	}
	// We must check if either a or b is []byte and handle it here first, to
	// ensure it doesn't fall through to the standard array/list handling.  This
	// ensures that []byte isn't compatible with []uint16 and other lists or
	// arrays of numbers.
	if ax, bx := ttIsStringEnumBytes(a), ttIsStringEnumBytes(b); ax || bx {
		return ax && bx
	}
	// Handle composite vdl.
	switch a.Kind() {
	case vdl.Array, vdl.List:
		switch b.Kind() {
		case vdl.Array, vdl.List:
			return compat(a.Elem(), b.Elem(), seenA, seenB)
		}
		return false
	case vdl.Set:
		switch b.Kind() {
		case vdl.Set:
			return compat(a.Key(), b.Key(), seenA, seenB)
		case vdl.Map:
			return compatMapKeyElem(b, a.Key(), vdl.BoolType, seenB, seenA)
		case vdl.Struct:
			return compatStructKeyElem(b, a.Key(), vdl.BoolType, seenB, seenA)
		}
		return false
	case vdl.Map:
		switch b.Kind() {
		case vdl.Set:
			return compatMapKeyElem(a, b.Key(), vdl.BoolType, seenA, seenB)
		case vdl.Map:
			return compatMapKeyElem(a, b.Key(), b.Elem(), seenA, seenB)
		case vdl.Struct:
			return compatStructKeyElem(b, a.Key(), a.Elem(), seenB, seenA)
		}
		return false
	case vdl.Struct:
		switch b.Kind() {
		case vdl.Set:
			return compatStructKeyElem(a, b.Key(), vdl.BoolType, seenA, seenB)
		case vdl.Map:
			return compatStructKeyElem(a, b.Key(), b.Elem(), seenA, seenB)
		case vdl.Struct:
			return compatStructStruct(a, b, seenA, seenB)
		}
		return false
	default:
		panic(fmt.Errorf("val: compat unhandled types %q %q", a, b))
	}
}

func ttIsNumber(tt *vdl.Type) bool {
	switch tt.Kind() {
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64, vdl.Complex64, vdl.Complex128:
		return true
	}
	return false
}

func ttIsStringEnumBytes(tt *vdl.Type) bool {
	return tt.Kind() == vdl.String || tt.Kind() == vdl.Enum || tt.IsBytes()
}

func ttIsEmptyStruct(tt *vdl.Type) bool {
	return tt.Kind() == vdl.Struct && tt.NumField() == 0
}

// REQUIRED: a is OneOf
func compatOneOf(a, b *vdl.Type, seenA, seenB map[*vdl.Type]bool) bool {
	// OneOf is a disjunction - only one of the types needs to be compatible.
	for ax := 0; ax < a.NumOneOfType(); ax++ {
		if compat(a.OneOfType(ax), b, seenA, seenB) {
			return true
		}
	}
	return false
}

// REQUIRED: a is Map
func compatMapKeyElem(a, bKey, bElem *vdl.Type, seenA, seenB map[*vdl.Type]bool) bool {
	return compat(a.Key(), bKey, seenA, seenB) && compat(a.Elem(), bElem, seenA, seenB)
}

// REQUIRED: a is Struct
func compatStructKeyElem(a, bKey, bElem *vdl.Type, seenA, seenB map[*vdl.Type]bool) bool {
	// Struct is a conjunction, all fields must be compatible.
	if ttIsEmptyStruct(a) {
		return false // empty struct isn't compatible with set or map
	}
	if !compat(vdl.StringType, bKey, seenA, seenB) {
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
func compatStructStruct(a, b *vdl.Type, seenA, seenB map[*vdl.Type]bool) bool {
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

// convTarget represents the state and logic for value conversion.
//
// TODO(toddw): Split convTarget apart into reflectTarget and valueTarget.
type convTarget struct {
	// Conceptually this is a disjoint union between the two types of destination
	// values: *vdl.Value and reflect.Value.  Only one of the vv and rv fields is
	// ever valid.  We split into two fields and perform "if vv == nil" checks in
	// each method rather than using an interface for performance reasons.
	//
	// The tt field is always non-nil, and represents the type of the value.
	tt *vdl.Type
	vv *vdl.Value
	rv reflect.Value
}

// ReflectTarget returns a conversion Target based on the given reflect.Value.
// Some rules depending on the type of target:
//   o If target is a valid *vdl.Value, it is filled in directly.
//   o Otherwise target must be a settable value (i.e. it must be a pointer).
//   o Pointers are followed, and logically "flattened".
//   o New values are automatically created for nil pointers.
//   o Targets convertible to *vdl.Type and *vdl.Value are filled in directly.
func ReflectTarget(target reflect.Value) (Target, error) {
	tt, err := vdl.TypeFromReflect(target.Type())
	if err != nil {
		return nil, err
	}
	conv, err := reflectConv(target, tt)
	if err != nil {
		return nil, err
	}
	return conv, nil
}

// ValueTarget returns a conversion Target based on the given vdl.Value.
// Returns an error if the given vdl.Value isn't valid.
func ValueTarget(target *vdl.Value) (Target, error) {
	if !target.IsValid() {
		return nil, errTargetInvalid
	}
	return valueConv(target), nil
}

func reflectConv(rv reflect.Value, tt *vdl.Type) (convTarget, error) {
	if !rv.IsValid() {
		return convTarget{}, errTargetInvalid
	}
	if vv := extractValue(rv); vv.IsValid() {
		return valueConv(vv), nil
	}
	if !rv.CanSet() && rv.Kind() == reflect.Ptr && !rv.IsNil() {
		// Dereference the pointer a single time to make rv settable.
		rv = rv.Elem()
	}
	if !rv.CanSet() {
		return convTarget{}, errTargetUnsettable
	}
	return convTarget{tt: tt, rv: rv}, nil
}

func valueConv(vv *vdl.Value) convTarget {
	return convTarget{tt: vv.Type(), vv: vv}
}

func extractValue(rv reflect.Value) *vdl.Value {
	for rv.Kind() == reflect.Ptr {
		switch {
		case rv.IsNil():
			return nil
		case rv.Type().ConvertibleTo(rtPtrToValue):
			return rv.Convert(rtPtrToValue).Interface().(*vdl.Value)
		}
		rv = rv.Elem()
	}
	return nil
}

// startConvert prepares to fill in c, converting from type tt.  Returns fin and
// fill which are used by finishConvert to finish the conversion; fin represents
// the final value to assign to, and fill represents the value to actually fill
// in.
func startConvert(c convTarget, tt *vdl.Type) (fin, fill convTarget, _ error) {
	switch tt.Kind() {
	case vdl.Any, vdl.OneOf, vdl.Nilable:
		return convTarget{}, convTarget{}, fmt.Errorf("variant or nilable type %q used as conversion src", tt)
	}
	if !compatible(c.tt, tt) {
		return convTarget{}, convTarget{}, fmt.Errorf("types %q and %q aren't compatible", c.tt, tt)
	}
	if c.vv == nil {
		// Flatten pointers, creating new values as necessary.
		for c.rv.Kind() == reflect.Ptr {
			if c.rv.Type().Elem() == rtValue {
				// c.rv has underlying type *vdl.Value, fill from it directly.
				vv := c.rv.Convert(rtPtrToValue).Interface().(*vdl.Value)
				if !vv.IsValid() {
					vv = vdl.ZeroValue(tt)
					c.rv.Set(reflect.ValueOf(vv).Convert(c.rv.Type()))
				}
				return c, valueConv(vv), nil
			}
			if c.rv.IsNil() {
				c.rv.Set(reflect.New(c.rv.Type().Elem()))
			}
			if c.rv.Type().Elem() == rtType {
				// Stop at *vdl.Type to allow TypeVal values to be assigned.
				return c, c, nil
			}
			c.rv = c.rv.Elem()
		}
		if c.tt.Kind() == vdl.Nilable {
			c.tt = c.tt.Elem() // flatten c.tt to match c.rv
		}
		switch c.rv.Kind() {
		case reflect.Interface:
			// Create a concrete *vdl.Value to convert to, which will be assigned to
			// the interface in finishConvert.
			//
			// TODO(toddw): Add type registration and create real Go objects?
			if !rtPtrToValue.AssignableTo(c.rv.Type()) {
				return convTarget{}, convTarget{}, fmt.Errorf("%v not assignable from *vdl.Value", c.rv.Type())
			}
			return c, valueConv(vdl.ZeroValue(tt)), nil
		case reflect.Array, reflect.Slice, reflect.Map:
			c.rv.Set(reflect.Zero(c.rv.Type())) // start with zero collections
		}
	} else {
		// Flatten nilable, creating non-nil values as necessary.
		if c.vv.Kind() == vdl.Nilable {
			if c.vv.IsNil() {
				c.vv.Assign(vdl.NonNilZeroValue(c.tt))
			}
			c.tt, c.vv = c.tt.Elem(), c.vv.Elem()
		}
		switch c.vv.Kind() {
		case vdl.Any, vdl.OneOf:
			if !c.tt.AssignableFrom(tt) {
				return convTarget{}, convTarget{}, fmt.Errorf("%v not assignable from %v", c.tt, tt)
			}
			return c, valueConv(vdl.ZeroValue(tt)), nil
		case vdl.Array, vdl.List, vdl.Set, vdl.Map:
			c.vv.Assign(nil) // start with zero collections
		}
	}
	return c, c, nil
}

// finishConvert finishes converting a value, taking the fin and fill returned
// by startConvert.  This is necessary since interface/any/oneof values are
// assigned by value, and can't be filled in by reference.
func finishConvert(fin, fill convTarget) error {
	if fin.vv == nil {
		switch fin.rv.Kind() {
		case reflect.Interface:
			fin.rv.Set(reflect.ValueOf(fill.vv))
		}
	} else {
		switch fin.vv.Kind() {
		case vdl.Any, vdl.OneOf:
			fin.vv.Assign(fill.vv)
		}
	}
	return nil
}

// FromBool implements the Target interface method.
func (c convTarget) FromBool(src bool, tt *vdl.Type) error {
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromBool(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

// FromUint implements the Target interface method.
func (c convTarget) FromUint(src uint64, tt *vdl.Type) error {
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromUint(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

// FromInt implements the Target interface method.
func (c convTarget) FromInt(src int64, tt *vdl.Type) error {
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromInt(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

// FromFloat implements the Target interface method.
func (c convTarget) FromFloat(src float64, tt *vdl.Type) error {
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromFloat(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

// FromComplex implements the Target interface method.
func (c convTarget) FromComplex(src complex128, tt *vdl.Type) error {
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromComplex(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

// FromBytes implements the Target interface method.
func (c convTarget) FromBytes(src []byte, tt *vdl.Type) error {
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromBytes(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

// FromString implements the Target interface method.
func (c convTarget) FromString(src string, tt *vdl.Type) error {
	// TODO(toddw): Speed this up.
	return c.FromBytes([]byte(src), tt)
}

// FromEnumLabel implements the Target interface method.
func (c convTarget) FromEnumLabel(src string, tt *vdl.Type) error {
	return c.FromString(src, tt)
}

// FromTypeVal implements the Target interface method.
func (c convTarget) FromTypeVal(src *vdl.Type) error {
	fin, fill, err := startConvert(c, vdl.TypeValType)
	if err != nil {
		return err
	}
	if err := fill.fromTypeVal(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

// FromNil implements the Target interface method.
func (c convTarget) FromNil(tt *vdl.Type) error {
	// Only perform type-checking; assume the target starts at its zero value.
	//
	// TODO(toddw): Consider setting the target to nil, so that non-zero targets
	// are set correctly.  If we do this we'll also need to reset all struct
	// fields before conversion.
	switch tt.Kind() {
	case vdl.Any, vdl.Nilable:
		if compatible(c.tt, tt) {
			return nil
		}
	}
	return fmt.Errorf("invalid conversion from nil %v to %v", tt, c.tt)
}

func (c convTarget) fromBool(src bool) error {
	if c.vv == nil {
		if c.rv.Kind() == reflect.Bool {
			c.rv.SetBool(src)
			return nil
		}
	} else {
		if c.vv.Kind() == vdl.Bool {
			c.vv.AssignBool(src)
			return nil
		}
	}
	return fmt.Errorf("invalid conversion from bool to %v", c.tt)
}

func (c convTarget) fromUint(src uint64) error {
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
		case vdl.Byte:
			if !overflowUint(src, bitlenV(kind)) {
				c.vv.AssignByte(byte(src))
				return nil
			}
		case vdl.Uint16, vdl.Uint32, vdl.Uint64:
			if !overflowUint(src, bitlenV(kind)) {
				c.vv.AssignUint(src)
				return nil
			}
		case vdl.Int16, vdl.Int32, vdl.Int64:
			if isrc, ok := convertUintToInt(src, bitlenV(kind)); ok {
				c.vv.AssignInt(isrc)
				return nil
			}
		case vdl.Float32, vdl.Float64:
			if fsrc, ok := convertUintToFloat(src, bitlenV(kind)); ok {
				c.vv.AssignFloat(fsrc)
				return nil
			}
		case vdl.Complex64, vdl.Complex128:
			if fsrc, ok := convertUintToFloat(src, bitlenV(kind)); ok {
				c.vv.AssignComplex(complex(fsrc, 0))
				return nil
			}
		}
	}
	return fmt.Errorf("invalid conversion from uint(%d) to %v", src, c.tt)
}

func (c convTarget) fromInt(src int64) error {
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
		case vdl.Byte:
			if usrc, ok := convertIntToUint(src, bitlenV(kind)); ok {
				c.vv.AssignByte(byte(usrc))
				return nil
			}
		case vdl.Uint16, vdl.Uint32, vdl.Uint64:
			if usrc, ok := convertIntToUint(src, bitlenV(kind)); ok {
				c.vv.AssignUint(usrc)
				return nil
			}
		case vdl.Int16, vdl.Int32, vdl.Int64:
			if !overflowInt(src, bitlenV(kind)) {
				c.vv.AssignInt(src)
				return nil
			}
		case vdl.Float32, vdl.Float64:
			if fsrc, ok := convertIntToFloat(src, bitlenV(kind)); ok {
				c.vv.AssignFloat(fsrc)
				return nil
			}
		case vdl.Complex64, vdl.Complex128:
			if fsrc, ok := convertIntToFloat(src, bitlenV(kind)); ok {
				c.vv.AssignComplex(complex(fsrc, 0))
				return nil
			}
		}
	}
	return fmt.Errorf("invalid conversion from int(%d) to %v", src, c.tt)
}

func (c convTarget) fromFloat(src float64) error {
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
		case vdl.Byte:
			if usrc, ok := convertFloatToUint(src, bitlenV(kind)); ok {
				c.vv.AssignByte(byte(usrc))
				return nil
			}
		case vdl.Uint16, vdl.Uint32, vdl.Uint64:
			if usrc, ok := convertFloatToUint(src, bitlenV(kind)); ok {
				c.vv.AssignUint(usrc)
				return nil
			}
		case vdl.Int16, vdl.Int32, vdl.Int64:
			if isrc, ok := convertFloatToInt(src, bitlenV(kind)); ok {
				c.vv.AssignInt(isrc)
				return nil
			}
		case vdl.Float32, vdl.Float64:
			c.vv.AssignFloat(convertFloatToFloat(src, bitlenV(kind)))
			return nil
		case vdl.Complex64, vdl.Complex128:
			c.vv.AssignComplex(complex(convertFloatToFloat(src, bitlenV(kind)), 0))
			return nil
		}
	}
	return fmt.Errorf("invalid conversion from float(%g) to %v", src, c.tt)
}

func (c convTarget) fromComplex(src complex128) error {
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
		case vdl.Byte:
			if usrc, ok := convertComplexToUint(src, bitlenV(kind)); ok {
				c.vv.AssignByte(byte(usrc))
				return nil
			}
		case vdl.Uint16, vdl.Uint32, vdl.Uint64:
			if usrc, ok := convertComplexToUint(src, bitlenV(kind)); ok {
				c.vv.AssignUint(usrc)
				return nil
			}
		case vdl.Int16, vdl.Int32, vdl.Int64:
			if isrc, ok := convertComplexToInt(src, bitlenV(kind)); ok {
				c.vv.AssignInt(isrc)
				return nil
			}
		case vdl.Float32, vdl.Float64:
			if imag(src) == 0 {
				c.vv.AssignFloat(convertFloatToFloat(real(src), bitlenV(kind)))
				return nil
			}
		case vdl.Complex64, vdl.Complex128:
			re := convertFloatToFloat(real(src), bitlenV(kind))
			im := convertFloatToFloat(imag(src), bitlenV(kind))
			c.vv.AssignComplex(complex(re, im))
			return nil
		}
	}
	return fmt.Errorf("invalid conversion from complex(%g) to %v", src, c.tt)
}

func (c convTarget) fromBytes(src []byte) error {
	if c.vv == nil {
		switch {
		case c.tt.Kind() == vdl.Enum:
			// Handle special-case enum first, by calling the Assign method.  Note
			// that vdl.TypeFromReflect has already validated the Assign method, so we
			// can just call it without error checking.
			if c.rv.CanAddr() {
				in := []reflect.Value{reflect.ValueOf(string(src))}
				out := c.rv.Addr().MethodByName("Assign").Call(in)
				if out[0].Bool() {
					return nil
				}
			}
		case c.rv.Kind() == reflect.String:
			c.rv.SetString(string(src)) // TODO(toddw): check utf8
			return nil
		case c.rv.Kind() == reflect.Array:
			if c.rv.Type().Elem() == rtByte && c.rv.Len() == len(src) {
				reflect.Copy(c.rv, reflect.ValueOf(src))
				return nil
			}
		case c.rv.Kind() == reflect.Slice:
			if c.rv.Type().Elem() == rtByte {
				cp := make([]byte, len(src))
				copy(cp, src)
				c.rv.SetBytes(cp)
				return nil
			}
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case vdl.String:
			c.vv.AssignString(string(src)) // TODO(toddw): check utf8
			return nil
		case vdl.Array:
			if c.tt.IsBytes() && c.tt.Len() == len(src) {
				c.vv.AssignBytes(src)
				return nil
			}
		case vdl.List:
			if c.tt.IsBytes() {
				c.vv.AssignBytes(src)
				return nil
			}
		case vdl.Enum:
			if index := c.tt.EnumIndex(string(src)); index >= 0 {
				c.vv.AssignEnumIndex(index)
				return nil
			}
		}
	}
	return fmt.Errorf("invalid conversion from []byte to %v", c.tt)
}

func (c convTarget) fromTypeVal(src *vdl.Type) error {
	if c.vv == nil {
		if rtPtrToType.ConvertibleTo(c.rv.Type()) {
			c.rv.Set(reflect.ValueOf(src).Convert(c.rv.Type()))
			return nil
		}
	} else {
		if c.tt == vdl.TypeValType {
			c.vv.AssignTypeVal(src)
			return nil
		}
	}
	return fmt.Errorf("invalid conversion from typeval to %v", c.tt)
}

// StartList implements the Target interface method.
func (c convTarget) StartList(tt *vdl.Type, len int) (ListTarget, error) {
	fin, fill, err := startConvert(c, tt)
	return compConvTarget{fin, fill}, err
}

// FinishList implements the Target interface method.
func (c convTarget) FinishList(x ListTarget) error {
	cc := x.(compConvTarget)
	return finishConvert(cc.fin, cc.fill)
}

// StartSet implements the Target interface method.
func (c convTarget) StartSet(tt *vdl.Type, len int) (SetTarget, error) {
	fin, fill, err := startConvert(c, tt)
	return compConvTarget{fin, fill}, err
}

// FinishSet implements the Target interface method.
func (c convTarget) FinishSet(x SetTarget) error {
	cc := x.(compConvTarget)
	return finishConvert(cc.fin, cc.fill)
}

// StartMap implements the Target interface method.
func (c convTarget) StartMap(tt *vdl.Type, len int) (MapTarget, error) {
	fin, fill, err := startConvert(c, tt)
	return compConvTarget{fin, fill}, err
}

// FinishMap implements the Target interface method.
func (c convTarget) FinishMap(x MapTarget) error {
	cc := x.(compConvTarget)
	return finishConvert(cc.fin, cc.fill)
}

// StartStruct implements the Target interface method.
func (c convTarget) StartStruct(tt *vdl.Type) (StructTarget, error) {
	fin, fill, err := startConvert(c, tt)
	return compConvTarget{fin, fill}, err
}

// FinishStruct implements the Target interface method.
func (c convTarget) FinishStruct(x StructTarget) error {
	cc := x.(compConvTarget)
	return finishConvert(cc.fin, cc.fill)
}

// compConvTarget represents the state and logic for composite value conversion.
type compConvTarget struct {
	fin, fill convTarget // fields returned by startConvert.
}

// StartElem implements the ListTarget interface method.
func (cc compConvTarget) StartElem(index int) (elem Target, _ error) {
	return cc.fill.startElem(index)
}

// FinishElem implements the ListTarget interface method.
func (cc compConvTarget) FinishElem(elem Target) error {
	return cc.fill.finishElem(elem.(convTarget))
}

// StartKey implements the SetTarget and MapTarget interface method.
func (cc compConvTarget) StartKey() (key Target, _ error) {
	return cc.fill.startKey()
}

// FinishKeyStartField implements the MapTarget interface method.
func (cc compConvTarget) FinishKeyStartField(key Target) (field Target, _ error) {
	return cc.fill.finishKeyStartField(key.(convTarget))
}

// FinishField implements the MapTarget and StructTarget interface method.
func (cc compConvTarget) FinishField(key, field Target) error {
	return cc.fill.finishField(key.(convTarget), field.(convTarget))
}

// StartField implements the StructTarget interface method.
func (cc compConvTarget) StartField(name string) (key, field Target, _ error) {
	var err error
	if key, err = cc.StartKey(); err != nil {
		return nil, nil, err
	}
	if err = key.FromString(name, vdl.StringType); err != nil {
		return nil, nil, err
	}
	if field, err = cc.FinishKeyStartField(key); err != nil {
		return nil, nil, err
	}
	return
}

// FinishKey implements the SetTarget interface method.
func (cc compConvTarget) FinishKey(key Target) error {
	field, err := cc.FinishKeyStartField(key)
	if err != nil {
		return err
	}
	return cc.FinishField(key, field)
}

func (c convTarget) startElem(index int) (convTarget, error) {
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Array:
			if index >= c.rv.Len() {
				return convTarget{}, errArrayIndex
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
		case vdl.Array:
			if index >= c.vv.Len() {
				return convTarget{}, errArrayIndex
			}
			return valueConv(c.vv.Index(index)), nil
		case vdl.List:
			newlen := index + 1
			if newlen < c.vv.Len() {
				newlen = c.vv.Len()
			}
			return valueConv(c.vv.AssignLen(newlen).Index(index)), nil
		}
	}
	return convTarget{}, fmt.Errorf("type %v doesn't support StartElem", c.tt)
}

func (c convTarget) finishElem(elem convTarget) error {
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Array, reflect.Slice:
			return nil
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case vdl.Array, vdl.List:
			return nil
		}
	}
	return fmt.Errorf("type %v doesn't support FinishElem", c.tt)
}

func (c convTarget) startKey() (convTarget, error) {
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Map:
			return reflectConv(reflect.New(c.rv.Type().Key()).Elem(), c.tt.Key())
		case reflect.Struct:
			// The key for structs is the field name, which is a string.
			return reflectConv(reflect.New(rtString).Elem(), vdl.StringType)
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case vdl.Set, vdl.Map:
			return valueConv(vdl.ZeroValue(c.tt.Key())), nil
		case vdl.Struct:
			// The key for structs is the field name, which is a string.
			return valueConv(vdl.ZeroValue(vdl.StringType)), nil
		}
	}
	return convTarget{}, fmt.Errorf("type %v doesn't support StartKey", c.tt)
}

func (c convTarget) finishKeyStartField(key convTarget) (convTarget, error) {
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
			var ttField *vdl.Type
			switch rtField := c.rv.Type().Elem(); {
			case c.tt.Kind() == vdl.Set:
				// The map actually represents a set
				rvField = reflect.New(rtBool).Elem()
				rvField.SetBool(true)
				ttField = vdl.BoolType
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
				return convTarget{}, errFieldNotFound
			}
			if rvField.Kind() == reflect.Bool {
				rvField.SetBool(true)
			}
			return reflectConv(rvField, ttField.Type)
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case vdl.Set:
			return valueConv(vdl.BoolValue(true)), nil
		case vdl.Map:
			vvField := vdl.ZeroValue(c.tt.Elem())
			if vvField.Kind() == vdl.Bool {
				vvField.AssignBool(true)
			}
			return valueConv(vvField), nil
		case vdl.Struct:
			_, index := c.tt.FieldByName(key.vv.RawString())
			if index < 0 {
				// TODO(toddw): Add a way to track extra and missing fields.
				return convTarget{}, errFieldNotFound
			}
			vvField := c.vv.Field(index)
			if vvField.Kind() == vdl.Bool {
				vvField.AssignBool(true)
			}
			return valueConv(vvField), nil
		}
	}
	return convTarget{}, fmt.Errorf("type %v doesn't support FinishKeyStartField", c.tt)
}

func (c convTarget) finishField(key, field convTarget) error {
	// The special-case handling of bool fields matches the special-cases in
	// FinishKeyStartField.
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Map:
			if c.rv.IsNil() {
				c.rv.Set(reflect.MakeMap(c.rv.Type()))
			}
			rvField := field.rv
			if c.tt.Kind() == vdl.Set {
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
		case vdl.Set:
			if !field.vv.Bool() {
				return fmt.Errorf("%v can only be converted from true fields", c.tt)
			}
			c.vv.AssignSetKey(key.vv)
			return nil
		case vdl.Map:
			c.vv.AssignMapIndex(key.vv, field.vv)
			return nil
		case vdl.Struct:
			return nil
		}
	}
	return fmt.Errorf("type %v doesn't support FinishField", c.tt)
}
