package valconv

import (
	"fmt"
	"sync"

	"veyron.io/veyron/veyron2/vdl"
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
//   o TypeObject is only compatible with TypeObject.
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
	if ax, bx := a.Kind() == vdl.TypeObject, b.Kind() == vdl.TypeObject; ax || bx {
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
