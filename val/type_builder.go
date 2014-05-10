package val

import (
	"errors"
	"fmt"
	"sync"
)

var (
	errNoLabels       = errors.New("val: no Enum labels")
	errLabelEmpty     = errors.New("val: empty Enum label")
	errHasLabels      = errors.New("val: labels only valid for Enum")
	errElemNil        = errors.New("val: nil elem type")
	errElemNonNil     = errors.New("val: elem only valid for List and Map")
	errKeyNil         = errors.New("val: nil key type")
	errKeyNonNil      = errors.New("val: key only valid for Map")
	errFieldTypeNil   = errors.New("val: nil Struct field type")
	errFieldNameEmpty = errors.New("val: empty Struct field name")
	errHasFields      = errors.New("val: fields only valid for Struct")
	errOneOfTypeBad   = errors.New("val: type in OneOf must not be nil, OneOf or Any")
	errNoTypes        = errors.New("val: no OneOf types")
	errHasTypes       = errors.New("val: types only valid on OneOf")
)

func simpleType(kind Kind, name string) *Type {
	t, err := typeCons(&Type{kind: kind, name: name})
	if err != nil {
		panic(err) // This can never happen, since all simple types are valid.
	}
	return t
}

// AnyType returns an Any type with the given name.
func AnyType(name string) *Type { return simpleType(Any, name) }

// BoolType returns a Bool type with the given name.
func BoolType(name string) *Type { return simpleType(Bool, name) }

// IntType returns a Int type with the given name.
func IntType(name string) *Type { return simpleType(Int, name) }

// UintType returns a Uint type with the given name.
func UintType(name string) *Type { return simpleType(Uint, name) }

// FloatType returns a Float type with the given name.
func FloatType(name string) *Type { return simpleType(Float, name) }

// ComplexType returns a Complex type with the given name.
func ComplexType(name string) *Type { return simpleType(Complex, name) }

// StringType returns a String type with the given name.
func StringType(name string) *Type { return simpleType(String, name) }

// TypeValType returns a Bytes type with the given name.
func BytesType(name string) *Type { return simpleType(Bytes, name) }

// TypeValType returns a TypeVal type with the given name.
func TypeValType(name string) *Type { return simpleType(TypeVal, name) }

// TypeBuilder defines the interface for building composite Types.  There are
// two phases: 1) create a builder and describe the type, and 2) call Build.
// This two-phase building enables support for recursive types, and also makes
// it easy to construct a group of dependent types without determining their
// dependency ordering.
type TypeBuilder interface {
	// Build builds the type and returns the hash-consed result.
	Build() (*Type, error)

	TypeOrBuilder
}

// TypeOrBuilder only allows *Type or Builder values; other values cause a
// compile-time error.  It's used as the argument type for builder methods, to
// allow either fully built *Type values or Builder values as subtypes.
type TypeOrBuilder interface {
	// pending returns the pending type, which may be only partially built.
	pending() *Type
}

// EnumTypeBuilder is a builder for Enum types.
type EnumTypeBuilder interface {
	TypeBuilder
	// SetLabels sets the Enum labels.  Every Enum must have at least one label,
	// and each label must not be empty.
	SetLabels(labels []string) EnumTypeBuilder
}

// ListTypeBuilder is a builder for List types.
type ListTypeBuilder interface {
	TypeBuilder
	// SetElem sets the List element type.
	SetElem(elem TypeOrBuilder) ListTypeBuilder
}

// MapTypeBuilder is a builder for Map types.
type MapTypeBuilder interface {
	TypeBuilder
	// SetKey sets the Map key type.
	SetKey(key TypeOrBuilder) MapTypeBuilder
	// SetElem sets the Map element type.
	SetElem(elem TypeOrBuilder) MapTypeBuilder
}

// StructTypeBuilder is a builder for Struct types.
type StructTypeBuilder interface {
	TypeBuilder
	// AppendField appends the Struct field with the given name and t.  The name
	// must not be empty.  The ordering of fields is preserved; different
	// orderings create different types.
	AppendField(name string, t TypeOrBuilder) StructTypeBuilder
}

// OneOfTypeBuilder is a builder for OneOf types.
type OneOfTypeBuilder interface {
	TypeBuilder
	// AppendType appends the type t to the OneOf.  The ordering of types is
	// preserved; different orderings create different types.
	AppendType(t TypeOrBuilder) OneOfTypeBuilder
}

// BuildEnumType returns a new EnumTypeBuilder.
func BuildEnumType(name string) EnumTypeBuilder {
	return enumBuilder{builder{&Type{kind: Enum, name: name}}}
}

// BuildListType returns a new ListTypeBuilder.
func BuildListType(name string) ListTypeBuilder {
	return listBuilder{builder{&Type{kind: List, name: name}}}
}

// BuildMapType returns a new MapTypeBuilder.
func BuildMapType(name string) MapTypeBuilder {
	return mapBuilder{builder{&Type{kind: Map, name: name}}}
}

// BuildStructType returns a new StructTypeBuilder.
func BuildStructType(name string) StructTypeBuilder {
	return structBuilder{builder{&Type{kind: Struct, name: name}}}
}

// BuildOneOfType returns a new OneOfTypeBuilder.
func BuildOneOfType(name string) OneOfTypeBuilder {
	return oneofBuilder{builder{&Type{kind: OneOf, name: name}}}
}

// builder implements common functionality for all concrete builders.
type builder struct{ pend *Type }

func (b builder) pending() *Type { return b.pend }

func (b builder) Build() (*Type, error) {
	return typeCons(b.pend)
}

type (
	enumBuilder   struct{ builder }
	listBuilder   struct{ builder }
	mapBuilder    struct{ builder }
	structBuilder struct{ builder }
	oneofBuilder  struct{ builder }
)

func (b enumBuilder) SetLabels(labels []string) EnumTypeBuilder {
	b.pend.labels = labels
	return b
}

func (b listBuilder) SetElem(elem TypeOrBuilder) ListTypeBuilder {
	b.pend.elem = elem.pending()
	return b
}

func (b mapBuilder) SetKey(key TypeOrBuilder) MapTypeBuilder {
	b.pend.key = key.pending()
	return b
}

func (b mapBuilder) SetElem(elem TypeOrBuilder) MapTypeBuilder {
	b.pend.elem = elem.pending()
	return b
}

func (b structBuilder) AppendField(name string, t TypeOrBuilder) StructTypeBuilder {
	b.pend.fields = append(b.pend.fields, StructField{name, t.pending()})
	return b
}

func (b oneofBuilder) AppendType(t TypeOrBuilder) OneOfTypeBuilder {
	b.pend.types = append(b.pend.types, t.pending())
	return b
}

func checkedBuild(b TypeBuilder) *Type {
	t, err := b.Build()
	if err != nil {
		panic(err)
	}
	return t
}

// EnumType is a helper using EnumTypeBuilder to create a Enum type.  Panics on
// all Build errors.
func EnumType(name string, labels []string) *Type {
	return checkedBuild(BuildEnumType(name).SetLabels(labels))
}

// ListType is a helper using ListTypeBuilder to create a List type.  Panics on
// all Build errors.
func ListType(name string, elem *Type) *Type {
	return checkedBuild(BuildListType(name).SetElem(elem))
}

// MapType is a helper using MapTypeBuilder to create Map types.  Panics on all
// Build errors.
func MapType(name string, key, elem *Type) *Type {
	return checkedBuild(BuildMapType(name).SetKey(key).SetElem(elem))
}

// StructType is a helper using StructTypeBuilder to create Struct types.
// Panics on all Build errors.
func StructType(name string, fields []StructField) *Type {
	b := BuildStructType(name)
	for _, f := range fields {
		b.AppendField(f.Name, f.Type)
	}
	return checkedBuild(b)
}

// OneOfType is a helper using OneOfTypeBuilder to create OneOf types.  Panics
// on all Build errors.
func OneOfType(name string, types []*Type) *Type {
	b := BuildOneOfType(name)
	for _, t := range types {
		b.AppendType(t)
	}
	return checkedBuild(b)
}

var (
	// typeReg holds a global set of hash-consed types.
	typeReg   = typeSet{}
	typeRegMu sync.Mutex
)

// typeCons returns the hash-consed Type for a given Type t.
func typeCons(t *Type) (*Type, error) {
	if t == nil {
		return nil, nil
	}
	if err := normalizeType(t); err != nil {
		return nil, err
	}
	hashcode := hashType(t)
	typeRegMu.Lock()
	defer typeRegMu.Unlock()
	if found := typeReg.lookup(hashcode, t); found != nil {
		return found, nil
	}
	typeReg.insert(hashcode, t)
	return t, nil
}

func normalizeType(t *Type) error {
	// Check elem
	switch t.kind {
	case List, Map:
		if t.elem == nil {
			return errElemNil
		}
	default:
		if t.elem != nil {
			return errElemNonNil
		}
	}
	// Check key
	switch t.kind {
	case Map:
		if err := ValidMapKey(t.key); err != nil {
			return err
		}
	default:
		if t.key != nil {
			return errKeyNonNil
		}
	}
	// Check labels
	switch t.kind {
	case Enum:
		if len(t.labels) == 0 {
			return errNoLabels
		}
		for _, l := range t.labels {
			if l == "" {
				return errLabelEmpty
			}
		}
	default:
		if len(t.labels) > 0 {
			return errHasLabels
		}
	}
	// Check fields
	switch t.kind {
	case Struct:
		seen := make(map[string]bool, len(t.fields))
		for _, f := range t.fields {
			if f.Type == nil {
				return errFieldTypeNil
			}
			if f.Name == "" {
				return errFieldNameEmpty
			}
			if seen[f.Name] {
				return fmt.Errorf("val: struct %q has duplicate field name %q", t.name, f.Name)
			}
			seen[f.Name] = true
		}
	default:
		if len(t.fields) > 0 {
			return errHasFields
		}
	}
	// Check types
	switch t.kind {
	case OneOf:
		if len(t.types) == 0 {
			return errNoTypes
		}
		dups := typeSet{}
		for _, u := range t.types {
			if err := ValidOneOfType(u); err != nil {
				return err
			}
			hashcode := hashType(u)
			if dups.lookup(hashcode, u) != nil {
				return fmt.Errorf("val: oneof %q has duplicate type %q", t.name, u)
			}
			dups.insert(hashcode, u)
		}
	default:
		if len(t.types) > 0 {
			return errHasTypes
		}
	}
	return nil
}

// ValidMapKey returns a nil error iff the key may be used as a map key type.
func ValidMapKey(key *Type) error {
	if key == nil {
		return errKeyNil
	}
	// TODO(toddw): Disallow lists, maps, etc?  Also consider that JSON /
	// javascript only supports string keys, and doesn't have good support for
	// object keys.
	return nil
}

// ValidOneOfType returns a nil error iff t may be contained in a OneOf type.
func ValidOneOfType(t *Type) error {
	if t == nil || t.kind == OneOf || t.kind == Any {
		return errOneOfTypeBad
	}
	// TODO(toddw): Disallow primitives?
	return nil
}
