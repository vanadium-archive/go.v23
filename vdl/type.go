// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"fmt"
	"strings"
)

// Kind represents the kind of type that a Type represents.
type Kind uint

const (
	// Variant kinds
	Any      Kind = iota // any type
	Optional             // value might not exist
	// Scalar kinds
	Bool       // boolean
	Byte       // 8 bit unsigned integer
	Uint16     // 16 bit unsigned integer
	Uint32     // 32 bit unsigned integer
	Uint64     // 64 bit unsigned integer
	Int16      // 16 bit signed integer
	Int32      // 32 bit signed integer
	Int64      // 64 bit signed integer
	Float32    // 32 bit IEEE 754 floating point
	Float64    // 64 bit IEEE 754 floating point
	Complex64  // {real,imag} each 32 bit IEEE 754 floating point
	Complex128 // {real,imag} each 64 bit IEEE 754 floating point
	String     // unicode string (encoded as UTF-8 in memory)
	Enum       // one of a set of labels
	TypeObject // type represented as a value
	// Composite kinds
	Array  // fixed-length ordered sequence of elements
	List   // variable-length ordered sequence of elements
	Set    // unordered collection of distinct keys
	Map    // unordered association between distinct keys and values
	Struct // conjunction of an ordered sequence of (name,type) fields
	Union  // disjunction of an ordered sequence of (name,type) fields

	// Internal kinds; they never appear in a *Type returned to the user.
	internalNamed // placeholder for named types while they're being built.
)

func (k Kind) String() string {
	switch k {
	case Any:
		return "any"
	case Optional:
		return "optional"
	case Bool:
		return "bool"
	case Byte:
		return "byte"
	case Uint16:
		return "uint16"
	case Uint32:
		return "uint32"
	case Uint64:
		return "uint64"
	case Int16:
		return "int16"
	case Int32:
		return "int32"
	case Int64:
		return "int64"
	case Float32:
		return "float32"
	case Float64:
		return "float64"
	case Complex64:
		return "complex64"
	case Complex128:
		return "complex128"
	case String:
		return "string"
	case Enum:
		return "enum"
	case TypeObject:
		return "typeobject"
	case Array:
		return "array"
	case List:
		return "list"
	case Set:
		return "set"
	case Map:
		return "map"
	case Struct:
		return "struct"
	case Union:
		return "union"
	}
	panic(fmt.Errorf("vdl: unhandled kind: %d", k))
}

// SplitIdent splits the given identifier into its package path and local name.
//   a/b.Foo   -> (a/b, Foo)
//   a.b/c.Foo -> (a.b/c, Foo)
//   Foo       -> ("",  Foo)
//   a/b       -> ("",  a/b)
func SplitIdent(ident string) (pkgpath, name string) {
	dot := strings.LastIndex(ident, ".")
	if dot == -1 {
		return "", ident
	}
	return ident[:dot], ident[dot+1:]
}

// Type is the representation of a vanadium type.  Types are hash-consed; each
// unique type is represented by exactly one *Type instance, so to test for type
// equality you just compare the *Type instances.
//
// Not all methods apply to all kinds of types.  Restrictions are noted in the
// documentation for each method.  Calling a method inappropriate to the kind of
// type causes a run-time panic.
//
// Cyclic types are supported; e.g. you can represent a tree via:
//   type Node struct {
//     Val      string
//     Children []Node
//   }
type Type struct {
	kind                    Kind     // used by all kinds
	name                    string   // used by all kinds
	labels                  []string // used by Enum
	len                     int      // used by Array
	elem                    *Type    // used by Optional, Array, List, Map
	key                     *Type    // used by Set, Map
	fields                  []Field  // used by Struct, Union
	unique                  string   // used by all kinds, filled in by typeCons
	containsAnyOrTypeObject bool     // used by all kinds
}

// Field describes a single field in a Struct or Union.
type Field struct {
	Name string
	Type *Type
}

// Kind returns the kind of type t.
func (t *Type) Kind() Kind { return t.kind }

// Name returns the name of type t.  Empty names are allowed.
func (t *Type) Name() string { return t.name }

// String returns a human-readable description of type t.
func (t *Type) String() string {
	if t.unique != "" {
		return t.unique
	}
	return uniqueTypeStr(t, make(map[*Type]bool))
}

// CanBeNil returns true iff values of t can be nil.
//
// Any and Optional values can be nil.
func (t *Type) CanBeNil() bool {
	return t.kind == Any || t.kind == Optional
}

// CanBeNamed returns true iff t can be made into a named type.
//
// Any and TypeObject cannot be named.
func (t *Type) CanBeNamed() bool {
	return t.kind != Any && t.kind != TypeObject
}

// CanBeKey returns true iff t can be used as a set or map key.
//
// Any, List, Map, Optional, Set and TypeObject cannot be keys, nor can
// composite types that contain these types.
func (t *Type) CanBeKey() bool {
	return isValidKey(t, make(map[*Type]bool))
}

// CanBeOptional returns true iff t can be made into an optional type.
//
// Only named structs can be optional.
func (t *Type) CanBeOptional() bool {
	// Our philosophy is that we should retain the full type information in our
	// generated code, and generating annotations to distinguish optional from
	// non-optional types is awkward for unnamed types.
	//
	// Allowing optionality for named types other than structs is also awkward.
	// E.g. if we allowed optional named maps, it's unclear how we'd generate it
	// in Go.  We might just generate a map, which is already a reference type and
	// may be nil, but then we can't distinguish optional map types from
	// non-optional map types.
	return t.name != "" && t.kind == Struct
}

// IsBytes returns true iff the kind of type is []byte or [N]byte.
func (t *Type) IsBytes() bool {
	return (t.kind == List || t.kind == Array) && t.elem.kind == Byte
}

// EnumLabel returns the Enum label at the given index.  It panics if the index
// is out of range.
func (t *Type) EnumLabel(index int) string {
	t.checkKind("EnumLabel", Enum)
	return t.labels[index]
}

// EnumIndex returns the Enum index for the given label.  Returns -1 if the
// label doesn't exist.
func (t *Type) EnumIndex(label string) int {
	t.checkKind("EnumIndex", Enum)
	// We typically have a small number of labels, so linear search is fine.
	for index, l := range t.labels {
		if l == label {
			return index
		}
	}
	return -1
}

// NumEnumLabel returns the number of labels in an Enum.
func (t *Type) NumEnumLabel() int {
	t.checkKind("NumEnumLabel", Enum)
	return len(t.labels)
}

// Len returns the length of an Array.
func (t *Type) Len() int {
	t.checkKind("Len", Array)
	return t.len
}

// Elem returns the element type of an Optional, Array, List or Map.
func (t *Type) Elem() *Type {
	t.checkKind("Elem", Optional, Array, List, Map)
	return t.elem
}

// Key returns the key type of a Set or Map.
func (t *Type) Key() *Type {
	t.checkKind("Key", Set, Map)
	return t.key
}

// Field returns a description of the Struct or Union field at the given index.
func (t *Type) Field(index int) Field {
	t.checkKind("Field", Struct, Union)
	return t.fields[index]
}

// FieldByName returns a description of the Struct or Union field with the given
// name, and its integer field index.  Returns -1 if the name doesn't exist.
func (t *Type) FieldByName(name string) (Field, int) {
	t.checkKind("FieldByName", Struct, Union)
	// We typically have a small number of fields, so linear search is fine.
	for index, f := range t.fields {
		if f.Name == name {
			return f, index
		}
	}
	return Field{}, -1
}

// NumField returns the number of fields in a Struct or Union.
func (t *Type) NumField() int {
	t.checkKind("NumField", Struct, Union)
	return len(t.fields)
}

// AssignableFrom returns true iff values of t may be assigned from f:
//   o Allowed if t and the type of f are identical.
//   o Allowed if t is Any.
//   o Allowed if t is Optional, and f is Any(nil).
//
// The first rule establishes strict static typing.  The second rule relaxes
// things for Any, which is dynamically typed.  The third rule relaxes things
// further, to allow implicit conversions from Any(nil) to all Optional types.
func (t *Type) AssignableFrom(f *Value) bool {
	return t == f.t || t.kind == Any || (t.kind == Optional && f.t.kind == Any && f.IsNil())
}

// ptype implements the TypeOrPending interface.
func (t *Type) ptype() *Type { return t }

func (t *Type) errKind(method string, allowed ...Kind) error {
	return fmt.Errorf("vdl: %s mismatched kind; got: %v, want: %v", method, t, allowed)
}

func (t *Type) errBytes(method string) error {
	return fmt.Errorf("vdl: %s mismatched type; got: %v, want: bytes", method, t)
}

func (t *Type) checkKind(method string, allowed ...Kind) {
	if t != nil {
		for _, k := range allowed {
			if k == t.kind {
				return
			}
		}
	}
	panic(t.errKind(method, allowed...))
}

func (t *Type) checkIsBytes(method string) {
	if !t.IsBytes() {
		panic(t.errBytes(method))
	}
}

// ContainsKind returns true iff t or subtypes of t match any of the kinds.
func (t *Type) ContainsKind(mode WalkMode, kinds ...Kind) bool {
	return !t.Walk(mode, func(visit *Type) bool {
		for _, k := range kinds {
			if k == visit.Kind() {
				return false
			}
		}
		return true
	})
}

// ContainsType returns true iff t or subtypes of t match any of the types.
func (t *Type) ContainsType(mode WalkMode, types ...*Type) bool {
	return !t.Walk(mode, func(visit *Type) bool {
		for _, ty := range types {
			if ty == visit {
				return false
			}
		}
		return true
	})
}

// ContainsAnyOrTypeObject returns true iff t or subtypes of t match any or typeobject.
func (t *Type) ContainsAnyOrTypeObject() bool {
	return t.containsAnyOrTypeObject
}

// Walk performs a DFS walk through the type graph starting from t, calling fn
// for each visited type.  If fn returns false on a visited type, the walk is
// terminated early, and false is returned by Walk.  The mode controls which
// types in the type graph we will visit.
func (t *Type) Walk(mode WalkMode, fn func(*Type) bool) bool {
	return typeWalk(mode, t, fn, make(map[*Type]bool))
}

// WalkMode is the mode to perform a Walk through the type graph.
type WalkMode int

const (
	// WalkAll indicates we should walk through all types in the type graph.
	WalkAll WalkMode = iota
	// WalkInline indicates we should only visit subtypes of array, struct and
	// union.  Values of array, struct and union always include values of their
	// subtypes, thus the subtypes are considered to be inline.  Values of
	// optional, list, set and map might not include values of their subtypes, and
	// are not considered to be inline.
	WalkInline
)

func (t *Type) subTypesInline() bool {
	switch t.kind {
	case Array, Struct, Union:
		return true
	}
	return false
}

func typeWalk(mode WalkMode, t *Type, fn func(*Type) bool, seen map[*Type]bool) bool {
	if seen[t] {
		return true
	}
	seen[t] = true
	if !fn(t) {
		return false
	}
	if mode == WalkInline && !t.subTypesInline() {
		return true
	}
	switch t.kind {
	case Optional, Array, List:
		return typeWalk(mode, t.elem, fn, seen)
	case Set:
		return typeWalk(mode, t.key, fn, seen)
	case Map:
		return typeWalk(mode, t.key, fn, seen) && typeWalk(mode, t.elem, fn, seen)
	case Struct, Union:
		for _, field := range t.fields {
			if !typeWalk(mode, field.Type, fn, seen) {
				return false
			}
		}
	}
	return true
}
