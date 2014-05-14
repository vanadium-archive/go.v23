package val

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

var (
	errNameNonEmpty   = errors.New("val: Any and TypeVal cannot be named")
	errNameEmpty      = errors.New("val: OneOf, Enum and Struct must be named")
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
	errBaseNil        = errors.New("val: base type unset for named type")
	errBaseCycle      = errors.New("val: invalid named type cycle")
	errNotBuilt       = errors.New("val: TypeBuilder.Build must be called before Pending.Built")
)

// Primitive types, the basis for all other types.  All have empty names.
var (
	AnyType        = &Type{kind: Any}
	BoolType       = &Type{kind: Bool}
	Int32Type      = &Type{kind: Int32}
	Int64Type      = &Type{kind: Int64}
	Uint32Type     = &Type{kind: Uint32}
	Uint64Type     = &Type{kind: Uint64}
	Float32Type    = &Type{kind: Float32}
	Float64Type    = &Type{kind: Float64}
	Complex64Type  = &Type{kind: Complex64}
	Complex128Type = &Type{kind: Complex128}
	StringType     = &Type{kind: String}
	BytesType      = &Type{kind: Bytes}
	TypeValType    = &Type{kind: TypeVal}
)

// TypeOrPending only allows *Type or Pending values; other values cause a
// compile-time error.  It's used as the argument type for TypeBuilder methods,
// to allow either fully built *Type values or Pending values as subtypes.
type TypeOrPending interface {
	// ptype returns the pending type, which may be only partially built.
	ptype() *Type
}

// PendingType represents a type that's being built by the TypeBuilder.
type PendingType interface {
	TypeOrPending
	// Built returns the final built and hash-consed type.  Build must be called
	// on the TypeBuilder before Built is called on any pending type.  If any
	// pending type has a build error, Built returns a nil type for all pending
	// types, and returns non-nil errors for at least one pending type.
	Built() (*Type, error)
}

// PendingEnum represents an Enum type that is being built.
type PendingEnum interface {
	PendingType
	// SetLabels sets the Enum labels.  Every Enum must have at least one label,
	// and each label must not be empty.
	SetLabels(labels []string) PendingEnum
}

// PendingList represents a List type that is being built.
type PendingList interface {
	PendingType
	// SetElem sets the List element type.
	SetElem(elem TypeOrPending) PendingList
}

// PendingMap represents a Map type that is being built.
type PendingMap interface {
	PendingType
	// SetKey sets the Map key type.
	SetKey(key TypeOrPending) PendingMap
	// SetElem sets the Map element type.
	SetElem(elem TypeOrPending) PendingMap
}

// PendingStruct represents a Struct type that is being built.
type PendingStruct interface {
	PendingType
	// AppendField appends the Struct field with the given name and t.  The name
	// must not be empty.  The ordering of fields is preserved; different
	// orderings create different types.
	AppendField(name string, t TypeOrPending) PendingStruct
}

// PendingOneOf represents a OneOf type that is being built.
type PendingOneOf interface {
	PendingType
	// AppendType appends the type t to the OneOf.  The ordering of types is
	// preserved; different orderings create different types.
	AppendType(t TypeOrPending) PendingOneOf
}

// PendingNamed represents a named type that is being built.  Given a base type
// you can build a new type with an identical underlying structure, but a
// different name.
type PendingNamed interface {
	PendingType
	// SetBase sets the base type of the named type.  The resulting built type
	// will have the same underlying structure as base, but with the given name.
	SetBase(base TypeOrPending) PendingNamed
}

type (
	// pending implements common functionality for all pending objects.
	pending struct {
		*Type       // Holds pending type pre-Build, and the result post-Build.
		err   error // Build error for this pending type.
	}

	// Each pending object holds a *Type that it fills in as the user calls
	// methods to describe the type.  When Build is called, the type is
	// hash-consed to the final result.
	pendingEnum   struct{ *pending }
	pendingList   struct{ *pending }
	pendingMap    struct{ *pending }
	pendingStruct struct{ *pending }
	pendingOneOf  struct{ *pending }
	pendingNamed  struct{ *pending }
)

func (p pendingEnum) SetLabels(labels []string) PendingEnum {
	p.labels = labels
	return p
}

func (p pendingList) SetElem(elem TypeOrPending) PendingList {
	p.elem = elem.ptype()
	return p
}

func (p pendingMap) SetElem(elem TypeOrPending) PendingMap {
	p.elem = elem.ptype()
	return p
}

func (p pendingMap) SetKey(key TypeOrPending) PendingMap {
	p.key = key.ptype()
	return p
}

func (p pendingStruct) AppendField(name string, t TypeOrPending) PendingStruct {
	p.fields = append(p.fields, StructField{name, t.ptype()})
	return p
}

func (p pendingOneOf) AppendType(t TypeOrPending) PendingOneOf {
	p.types = append(p.types, t.ptype())
	return p
}

func (p pendingNamed) SetBase(base TypeOrPending) PendingNamed {
	// Pending named types are special - they have the internalNamed kind, and put
	// the base type in elem.  See pending.finalize() for the extra logic.
	p.elem = base.ptype()
	return p
}

// TypeBuilder builds Types.  There are two phases: 1) Create Pending* objects
// and describe each type, and 2) call Build.  When Build is called, all types
// are created and may be retrieved by calling Built on the pending type.  This
// two-phase building enables support for recursive types, and also makes it
// easy to construct a group of dependent types without determining their
// dependency ordering.  The separation between Build and Built allows
// individual errors to be returned for each pending type, and easily associated
// with additional information for the pending type, e.g. position information
// in a compiler.
//
// Each TypeBuilder instance enforces the rule that type names are unique; each
// named type must be represented by exactly one Type or PendingType object.
// E.g. you can't create an enum "Foo" and a struct "Foo" via the same
// TypeBuilder, nor can you create two structs named "Foo", even if they have
// the same fields.  This rule simplifies the hash consing logic.
//
// There is no enforcement of unique names across TypeBuilder instances; the val
// package allows different types with the same names.  This allows support for
// a single named type with multiple versions, all handled within a single
// address space.
//
// The zero TypeBuilder represents an empty builder.
type TypeBuilder struct {
	ptypes []*pending
}

func (b *TypeBuilder) add(t *Type) *pending {
	// Every pending object starts with the errNotBuilt error, which will be
	// overridden when the type is actually built.
	p := &pending{Type: t, err: errNotBuilt}
	b.ptypes = append(b.ptypes, p)
	return p
}

// Enum returns PendingEnum, used to describe an Enum type.
func (b *TypeBuilder) Enum(name string) PendingEnum {
	return pendingEnum{b.add(&Type{kind: Enum, name: name})}
}

// List returns PendingList, used to describe a List type.
func (b *TypeBuilder) List() PendingList {
	return pendingList{b.add(&Type{kind: List})}
}

// Map returns PendingMap, used to describe a Map type.
func (b *TypeBuilder) Map() PendingMap {
	return pendingMap{b.add(&Type{kind: Map})}
}

// Struct returns PendingStruct, used to describe a Struct type.
func (b *TypeBuilder) Struct(name string) PendingStruct {
	return pendingStruct{b.add(&Type{kind: Struct, name: name})}
}

// OneOf returns PendingOneOf, used to describe a OneOf type.
func (b *TypeBuilder) OneOf(name string) PendingOneOf {
	return pendingOneOf{b.add(&Type{kind: OneOf, name: name})}
}

// Named returns PendingNamed, used to describe a named type based on another
// type.
func (b *TypeBuilder) Named(name string) PendingNamed {
	return pendingNamed{b.add(&Type{kind: internalNamed, name: name})}
}

// Build builds all pending types.  Build must be called before Built may be
// called on each pending type to retrieve the final result.
func (b *TypeBuilder) Build() {
	// First finalize all types, indicating no more mutations will occur.
	for _, p := range b.ptypes {
		p.err = p.finalize()
	}
	// Now enforce the rule that type names are unique.  This must occur before we
	// hash cons anything, to catch tricky cases where hash consing is difficult.
	// See uniqueType for more info.
	names := make(map[string]*Type)
	for _, p := range b.ptypes {
		if err := enforceUniqueNames(p.Type, names); err != nil && p.err == nil {
			p.err = err
		}
	}
	// Now hash cons each pending type.
	for _, p := range b.ptypes {
		if p.err != nil {
			continue // skip this type since it already has an error
		}
		p.Type, p.err = typeCons(p.Type)
	}
	// If any pending type has a build error, make sure all built types are nil.
	for _, p := range b.ptypes {
		if p.err != nil {
			for _, q := range b.ptypes {
				q.Type = nil
			}
			break
		}
	}
}

func (p *pending) ptype() *Type { return p.Type }

// finalize indicates Build has been called, and the pending type will not be
// mutated anymore.
func (p *pending) finalize() error {
	if p.Type.kind == internalNamed {
		// Now that the mutations have finished, we can copy the base type into
		// p.Type, keeping the name of p.Type.
		name, base := p.Type.name, p.Type.elem
		// There may be a chain of named types, in which case we'll need to follow the
		// chain to the first type that's not internalNamed.
		for {
			if base == nil {
				return errBaseNil
			}
			if base == p.Type {
				return errBaseCycle
			}
			if base.kind != internalNamed {
				break
			}
			base = base.elem
		}
		*p.Type = *base
		p.Type.name = name
	}
	return nil
}

func (p *pending) Built() (*Type, error) {
	return p.Type, p.err
}

func checkedBuild(b TypeBuilder, p PendingType) *Type {
	b.Build()
	t, err := p.Built()
	if err != nil {
		panic(err)
	}
	return t
}

// EnumType is a helper using TypeBuilder to create a single Enum type.  Panics
// on all errors.
func EnumType(name string, labels []string) *Type {
	var b TypeBuilder
	return checkedBuild(b, b.Enum(name).SetLabels(labels))
}

// ListType is a helper using TypeBuilder to create a single List type.  Panics
// on all errors.
func ListType(elem *Type) *Type {
	var b TypeBuilder
	return checkedBuild(b, b.List().SetElem(elem))
}

// MapType is a helper using TypeBuilder to create a single Map type.  Panics on
// all errors.
func MapType(key, elem *Type) *Type {
	var b TypeBuilder
	return checkedBuild(b, b.Map().SetKey(key).SetElem(elem))
}

// StructType is a helper using TypeBuilder to create a single Struct type.
// Panics on all errors.
func StructType(name string, fields []StructField) *Type {
	var b TypeBuilder
	s := b.Struct(name)
	for _, f := range fields {
		s.AppendField(f.Name, f.Type)
	}
	return checkedBuild(b, s)
}

// OneOfType is a helper using TypeBuilder to create a single OneOf type.
// Panics on all errors.
func OneOfType(name string, types []*Type) *Type {
	var b TypeBuilder
	o := b.OneOf(name)
	for _, t := range types {
		o.AppendType(t)
	}
	return checkedBuild(b, o)
}

// NamedType is a helper using TypeBuilder to create a single named type based
// on another type.  Panics on all errors.
func NamedType(name string, base *Type) *Type {
	var b TypeBuilder
	return checkedBuild(b, b.Named(name).SetBase(base))
}

// enforceUniqueNames ensures that t and its subtypes contain unique type names;
// every non-empty type name corresponds to exactly one *Type.
func enforceUniqueNames(t *Type, names map[string]*Type) error {
	if t == nil || t.name == "" {
		return nil
	}
	if found := names[t.name]; found != nil {
		if found != t {
			one := uniqueType(found, make(map[*Type]bool))
			two := uniqueType(t, make(map[*Type]bool))
			return fmt.Errorf("val: duplicate type names %q and %q", one, two)
		}
		return nil
	}
	// First time seeing this type, put it in names and call recursively.
	names[t.name] = t
	if err := enforceUniqueNames(t.elem, names); err != nil {
		return err
	}
	if err := enforceUniqueNames(t.key, names); err != nil {
		return err
	}
	for _, x := range t.fields {
		if err := enforceUniqueNames(x.Type, names); err != nil {
			return err
		}
	}
	for _, x := range t.types {
		if err := enforceUniqueNames(x, names); err != nil {
			return err
		}
	}
	return nil
}

// uniqueType returns a unique string representing t, which is also its
// human-readable representation.  Invariant: two types A and B have the same
// unique string iff they are equal, even if they haven't been hash-consed yet.
//
// Think of each type as a graph, where nodes represent each type, and edges
// point from composite type to subtype.  Recursive types form a cycle in this
// graph.  If two type graphs are the same, the two types are equal.
//
// There is a subtlety.  Since we haven't hash-consed the types yet, it's
// possible that two different graphs also represent equal types.  E.g. consider
// the type:
//   type A struct {x []string;y []string}
//
// There are two different representations:
//            A (not consed)     A (hash-consed)
//          x/ \y              x/ \y
//          /   \               \ /
//   []string   []string     []string
//
// Both of these representations must return the same unique string.  To
// accomplish this, we recursively traverse the graph and dump the semantic
// contents of each type node.  The seen map breaks infinite loops from
// recursive types.  Since type cycles may only be created via named types, we
// keep track of seen types and only dump their names.
func uniqueType(t *Type, seen map[*Type]bool) string {
	if seen[t] && t.name != "" {
		return t.name
	}
	seen[t] = true
	s := t.name
	if s != "" {
		s += " "
	}
	switch t.kind {
	case Enum:
		return s + "enum{" + strings.Join(t.labels, ";") + "}"
	case List:
		return s + "[]" + uniqueType(t.elem, seen)
	case Map:
		return s + "map[" + uniqueType(t.key, seen) + "]" + uniqueType(t.elem, seen)
	case Struct:
		s += "struct{"
		for index, f := range t.fields {
			if index > 0 {
				s += ";"
			}
			s += f.Name + " " + uniqueType(f.Type, seen)
		}
		return s + "}"
	case OneOf:
		s += "oneof{"
		for index, one := range t.types {
			if index > 0 {
				s += ";"
			}
			s += uniqueType(one, seen)
		}
		return s + "}"
	default:
		return s + t.kind.String()
	}
}

var (
	// typeReg holds a global set of hash-consed types.  Hash-consing is based on
	// the string representation of the type.  See comments in uniqueType for an
	// explanation of subtleties.
	typeReg   = map[string]*Type{}
	typeRegMu sync.Mutex
)

// typeCons returns the hash-consed Type for a given Type t.
func typeCons(t *Type) (*Type, error) {
	typeRegMu.Lock()
	defer typeRegMu.Unlock()
	return typeConsLocked(t)
}

func typeConsLocked(t *Type) (*Type, error) {
	if t == nil {
		return nil, nil
	}
	if err := validType(t); err != nil {
		return nil, err
	}
	// Look for the type in our registry, based on its unique string.
	t.unique = uniqueType(t, make(map[*Type]bool))
	if found := typeReg[t.unique]; found != nil {
		return found, nil
	}
	// Not found in the registry, add it and recursively cons subtypes.  We cons
	// the outer type first to deal with recursive types; otherwise we'd have an
	// infinite loop.
	typeReg[t.unique] = t
	var err error
	if t.elem, err = typeConsLocked(t.elem); err != nil {
		return nil, err
	}
	if t.key, err = typeConsLocked(t.key); err != nil {
		return nil, err
	}
	for x := range t.fields {
		if t.fields[x].Type, err = typeConsLocked(t.fields[x].Type); err != nil {
			return nil, err
		}
	}
	for x := range t.types {
		if t.types[x], err = typeConsLocked(t.types[x]); err != nil {
			return nil, err
		}
	}
	return t, nil
}

// validType returns a nil error iff t is a valid Type.
func validType(t *Type) error {
	// Check kind
	switch t.kind {
	case internalNamed:
		panic(fmt.Errorf("val: internal kind %d used in validType", t.kind))
	}
	// Check name
	switch t.kind {
	case Any, TypeVal:
		if t.name != "" {
			return errNameNonEmpty
		}
	case OneOf, Enum, Struct:
		if t.name == "" {
			return errNameEmpty
		}
	}
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
		seen := make(map[string]bool, len(t.types))
		for _, u := range t.types {
			if err := ValidOneOfType(u); err != nil {
				return err
			}
			if seen[u.unique] {
				return fmt.Errorf("val: duplicate OneOf type: %s", u.unique)
			}
			seen[u.unique] = true
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
