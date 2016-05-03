// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import "fmt"

// Compatible2 returns true if types a and b are compatible with each other.
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
//   o String and enum are mutually compatible.
//   o Array and list are compatible if their elem types are compatible.
//   o Sets are compatible if their key types are compatible.
//   o Maps are compatible if their key and elem types are compatible.
//   o Structs are compatible if all fields with the same name are compatible,
//     and at least one field has the same name, or one of the types has no
//     fields.
//   o Unions are compatible if all fields with the same name are compatible,
//     and at least one field has the same name.
//
// Recursive types are checked for compatibility up to the first occurrence of a
// cycle in either type.  This leaves open the possibility of "obvious" false
// positives where types are compatible but values are not; this is a tradeoff
// favoring a simpler implementation and better performance over exhaustive
// checking.  This seems fine in practice since type compatibility is weaker
// than value convertibility, and since recursive types are not common.
func Compatible2(a, b *Type) bool {
	a, b = a.NonOptional(), b.NonOptional()
	if a == b {
		return true // fastpath for common case
	}
	key := compatKey(a, b)
	if compat, ok := compat2Cache.lookup(key); ok {
		return compat
	}
	// Concurrent updates may cause compatCache to be updated multiple times.
	// This race is benign; we always end up with the same result.
	compat := compat2(a, b, make(map[*Type]bool), make(map[*Type]bool))
	compat2Cache.update(key, compat)
	return compat
}

// TODO(toddw): Change this to a fixed-size LRU cache, otherwise it can grow
// without bounds and exhaust our memory.
var compat2Cache = &compatRegistry{compat: make(map[[2]*Type]bool)}

// compat2 is a recursive helper that implements Compatible2.
func compat2(a, b *Type, seenA, seenB map[*Type]bool) bool {
	// Normalize and break cycles from recursive types.
	a, b = a.NonOptional(), b.NonOptional()
	if a == b || seenA[a] || seenB[b] {
		return true
	}
	seenA[a], seenB[b] = true, true
	// Handle Any
	if a.Kind() == Any || b.Kind() == Any {
		return true
	}
	// Handle simple scalars
	if ax, bx := a.Kind() == Bool, b.Kind() == Bool; ax || bx {
		return ax && bx
	}
	if ax, bx := ttIsStringEnum(a), ttIsStringEnum(b); ax || bx {
		return ax && bx
	}
	if ax, bx := a.Kind().IsNumber(), b.Kind().IsNumber(); ax || bx {
		return ax && bx
	}
	if ax, bx := a.Kind() == TypeObject, b.Kind() == TypeObject; ax || bx {
		return ax && bx
	}
	// Handle composites
	switch a.Kind() {
	case Array, List:
		switch b.Kind() {
		case Array, List:
			return compat2(a.Elem(), b.Elem(), seenA, seenB)
		}
		return false
	case Set:
		switch b.Kind() {
		case Set:
			return compat2(a.Key(), b.Key(), seenA, seenB)
		}
		return false
	case Map:
		switch b.Kind() {
		case Map:
			return compat2(a.Key(), b.Key(), seenA, seenB) && compat2(a.Elem(), b.Elem(), seenA, seenB)
		}
		return false
	case Struct:
		switch b.Kind() {
		case Struct:
			if ttIsEmptyStruct(a) || ttIsEmptyStruct(b) {
				return true // empty struct is compatible with all other structs
			}
			return compat2Fields(a, b, seenA, seenB)
		}
		return false
	case Union:
		switch b.Kind() {
		case Union:
			return compat2Fields(a, b, seenA, seenB)
		}
		return false
	default:
		panic(fmt.Errorf("vdl: Compatible2 unhandled types %q %q", a, b))
	}
}

// REQUIRED: a and b are either Struct or Union
func compat2Fields(a, b *Type, seenA, seenB map[*Type]bool) bool {
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
		if !compat2(afield.Type, bfield.Type, seenA, seenB) {
			return false
		}
		fieldMatch = true
	}
	return fieldMatch
}
