// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"fmt"
	"sync"
)

// Compatible returns true if types a and b are compatible with each other.
//
// Compatibility is checked before every value conversion; it is the first-pass
// filter that disallows certain conversions.  Values of incompatible types are
// never convertible, while values of compatible types might not be convertible.
// E.g. float32 and byte are compatible types, and float32(1.0) is convertible
// to/from byte(1), but float32(1.5) is not convertible to/from any byte value.
//
// The reason we have a type compatibility check is to disallow invalid
// conversions that are hard to catch while converting values.  E.g. conversions
// between values of []bool and []float32 are invalid, but this is hard to catch
// if both lists are empty.
//
// Compatibility is reversible and transitive, except for the special Any type.
// Here are the rules:
//   o Any is compatible with all types.
//   o Optional is ignored for all rules (e.g. ?int is treated as int).
//   o Bool is only compatible with Bool.
//   o TypeObject is only compatible with TypeObject.
//   o Numbers are mutually compatible.
//   o String, enum are mutually compatible.
//   o Array and list are compatible if their elems are compatible.
//   o Set, map and struct are compatible if all keys K* are compatible,
//     and all fields F* are compatible:
//     - map[string]F* is compatible with struct{_ F*; ...}
//     - set[K*] is compatible with set[K*] and map[K*]bool
//       (during conversion all bools in the map must be true)
//     - set[string] is compatible with struct{_ bool; ...}
//       (by transitively combining the first two rules)
//     - Two struct types are compatible if all fields with the same name are
//       compatible, and at least one field has the same name, or one of the
//       types is an empty struct.
//   o Two union types are compatible if all fields with the same name are
//     compatible, and at least one field has the same name.
//
// Recursive types are checked for compatibility up to the first occurrence of a
// cycle in either type.  This leaves open the possibility of "obvious" false
// positives where types are compatible but values are not; this is a tradeoff
// favoring a simpler implementation and better performance over exhaustive
// checking.  This seems fine in practice since type compatibility is weaker
// than value convertibility, and since recursive types are not common.
//
// TODO(toddw): In the future we might allow the following compatibility:
//   union{A t2}  <-> map[string]t1  # union behaves like struct
//   union{A t2}  <-> t1             # first-field special case
//   struct{A t2} <-> t1             # first-field special case
//
// The advantage of allowing these rules is that it gives users a powerful
// mechanism for backwards/forwards compatibility.  The disadvantage is that
// we'd need more special cases, making it harder to understand and implement
// the rules.  E.g. we'd need to disallow transitivity for these links,
// otherwise we'd end up with nonsensical conversions:
//
//   union{A string} <-> string <-> union{B string}
func Compatible(a, b *Type) bool {
	a, b = a.NonOptional(), b.NonOptional()
	if a == b {
		return true // fastpath for common case
	}
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
var compatCache = &compatRegistry{compat: make(map[[2]*Type]bool)}

// The compat cache key is just the two types, which are already hash-consed.
// We make a minor attempt to normalize the order.
func compatKey(a, b *Type) [2]*Type {
	if a.Kind() > b.Kind() ||
		(a.Kind() == Enum && b.Kind() == Enum && a.NumEnumLabel() > b.NumEnumLabel()) ||
		(a.Kind() == Struct && b.Kind() == Struct && a.NumField() > b.NumField()) ||
		(a.Kind() == Union && b.Kind() == Union && a.NumField() > b.NumField()) {
		a, b = b, a
	}
	return [2]*Type{a, b}
}

func (reg *compatRegistry) lookup(key [2]*Type) (bool, bool) {
	reg.Lock()
	compat, ok := reg.compat[key]
	reg.Unlock()
	return compat, ok
}

func (reg *compatRegistry) update(key [2]*Type, compat bool) {
	reg.Lock()
	reg.compat[key] = compat
	reg.Unlock()
}

// compat is a recursive helper that implements compatible.
func compat(a, b *Type, seenA, seenB map[*Type]bool) bool {
	// Normalize and break cycles from recursive types.
	if a.Kind() == Optional {
		a = a.Elem()
	}
	if b.Kind() == Optional {
		b = b.Elem()
	}
	if a == b || seenA[a] || seenB[b] {
		return true
	}
	seenA[a], seenB[b] = true, true
	// Handle Any
	if a.Kind() == Any || b.Kind() == Any {
		return true
	}
	// Handle simple scalar
	if ax, bx := a.Kind().IsNumber(), b.Kind().IsNumber(); ax || bx {
		return ax && bx
	}
	if ax, bx := a.Kind() == Bool, b.Kind() == Bool; ax || bx {
		return ax && bx
	}
	if ax, bx := a.Kind() == TypeObject, b.Kind() == TypeObject; ax || bx {
		return ax && bx
	}
	// String and enum are compatible.
	if ax, bx := ttIsStringEnum(a), ttIsStringEnum(b); ax || bx {
		return ax && bx
	}
	// Handle composite
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
			if ttIsEmptyStruct(a) || ttIsEmptyStruct(b) {
				return true // empty struct is compatible with all other structs
			}
			return compatFields(a, b, seenA, seenB)
		}
		return false
	case Union:
		switch b.Kind() {
		case Union:
			return compatFields(a, b, seenA, seenB)
		}
		return false
	default:
		panic(fmt.Errorf("vdl: compat unhandled types %q %q", a, b))
	}
}

func ttIsStringEnum(tt *Type) bool {
	return tt.Kind() == String || tt.Kind() == Enum
}

func ttIsEmptyStruct(tt *Type) bool {
	return tt.Kind() == Struct && tt.NumField() == 0
}

// REQUIRED: a is Map
func compatMapKeyElem(a, bKey, bElem *Type, seenA, seenB map[*Type]bool) bool {
	return compat(a.Key(), bKey, seenA, seenB) && compat(a.Elem(), bElem, seenA, seenB)
}

// REQUIRED: a is Struct
func compatStructKeyElem(a, bKey, bElem *Type, seenA, seenB map[*Type]bool) bool {
	// All struct fields must be compatible.
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

// REQUIRED: a and b are either Struct or Union
func compatFields(a, b *Type, seenA, seenB map[*Type]bool) bool {
	// All fields with the same name must be compatible, and at least one field
	// must match.
	if a.NumField() > b.NumField() {
		a, seenA, b, seenB = b, seenB, a, seenA
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
	return fieldMatch
}
