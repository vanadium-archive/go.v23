package val

import (
	"fmt"
)

// Kind represents the kind of type that a Type represents.
type Kind uint

const (
	// Variant kinds
	Any Kind = iota
	OneOf
	// Scalar kinds
	Bool
	Int32
	Int64
	Uint32
	Uint64
	Float32
	Float64
	Complex64
	Complex128
	String
	Bytes
	TypeVal
	Enum
	// Composite kinds
	List
	Map
	Struct
	// Internal kinds; they never appear in a *Type returned to the user.
	internalNamed
)

func (k Kind) String() string {
	switch k {
	case Any:
		return "any"
	case OneOf:
		return "oneof"
	case Bool:
		return "bool"
	case Int32:
		return "int32"
	case Int64:
		return "int64"
	case Uint32:
		return "uint32"
	case Uint64:
		return "uint64"
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
	case Bytes:
		return "bytes"
	case TypeVal:
		return "typeval"
	case Enum:
		return "enum"
	case List:
		return "list"
	case Map:
		return "map"
	case Struct:
		return "struct"
	}
	panic(fmt.Errorf("val: unhandled kind: %d", k))
}

// Type is the representation of a veyron type.  Types are hash-consed; each
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
	kind   Kind          // used by all kinds
	name   string        // used by all kinds
	labels []string      // used by Enum
	elem   *Type         // used by List, Map
	key    *Type         // used by Map
	fields []StructField // used by Struct
	types  []*Type       // used by OneOf
	unique string        // used by all kinds, filled in by typeCons
}

// StructField describes a single field in a Struct.
type StructField struct {
	Name string
	Type *Type
}

// Kind returns the kind of type t.
func (t *Type) Kind() Kind { return t.kind }

// Name returns the name of type t.  Empty names are allowed.
func (t *Type) Name() string { return t.name }

// String returns a human-readable description of type t.
func (t *Type) String() string { return t.unique }

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

// Elem returns the element type of a List or Map.
func (t *Type) Elem() *Type {
	t.checkKind("Elem", List, Map)
	return t.elem
}

// Key returns the key type of a Map.
func (t *Type) Key() *Type {
	t.checkKind("Key", Map)
	return t.key
}

// Field returns a description of the Struct field at the given index.
func (t *Type) Field(index int) StructField {
	t.checkKind("Field", Struct)
	return t.fields[index]
}

// FieldByName returns a description of the Struct field with the given name,
// and its integer field index.  Returns -1 if the field name doesn't exist.
func (t *Type) FieldByName(name string) (StructField, int) {
	t.checkKind("FieldByName", Struct)
	// We typically have a small number of fields, so linear search is fine.
	for index, f := range t.fields {
		if f.Name == name {
			return f, index
		}
	}
	return StructField{}, -1
}

// NumField returns the number of fields in a Struct.
func (t *Type) NumField() int {
	t.checkKind("NumField", Struct)
	return len(t.fields)
}

// OneOfType returns the OneOf type at the given index.
func (t *Type) OneOfType(index int) *Type {
	t.checkKind("OneofType", OneOf)
	return t.types[index]
}

// OneOfIndex returns the OneOf index for the given target type.  Returns -1 if
// target doesn't exist.
func (t *Type) OneOfIndex(target *Type) int {
	t.checkKind("OneOfIndex", OneOf)
	// We typically have a small number of types, so linear search is fine.
	for index, one := range t.types {
		if one == target {
			return index
		}
	}
	return -1
}

// NumOneOfType returns the number of types in a OneOf.
func (t *Type) NumOneOfType() int {
	t.checkKind("NumOneOfType", OneOf)
	return len(t.types)
}

// AssignableFrom returns true iff a value of type t may be assigned from a
// value of type f.  The following cases are allowed:
// + The types t and f are identical, or
// + The type t is Any, or
// + The type t is OneOf, and f is one of the allowed types in t.
func (t *Type) AssignableFrom(f *Type) bool {
	return t == f || t.kind == Any || (t.kind == OneOf && t.OneOfIndex(f) != -1)
}

// ptype implements the TypeOrPending interface.
func (t *Type) ptype() *Type { return t }

func errKindMismatch(method string, k Kind, allowed ...Kind) error {
	return fmt.Errorf("val: method %q called on mismatched kind; got: %v, want: %v", method, k, allowed)
}

func (t *Type) checkKind(method string, allowed ...Kind) {
	for _, k := range allowed {
		if k == t.kind {
			return
		}
	}
	panic(errKindMismatch(method, t.kind, allowed...))
}
