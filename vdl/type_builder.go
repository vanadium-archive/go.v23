package vdl

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

var (
	errNameNonEmpty   = errors.New("any and typeobject cannot be renamed")
	errNoLabels       = errors.New("no enum labels")
	errLabelEmpty     = errors.New("empty enum label")
	errHasLabels      = errors.New("labels only valid for enum")
	errLenZero        = errors.New("negative or zero array length")
	errLenNonZero     = errors.New("length only valid for array")
	errElemNil        = errors.New("nil elem type")
	errElemNonNil     = errors.New("elem only valid for array, list and map")
	errKeyNil         = errors.New("nil key type")
	errKeyNonNil      = errors.New("key only valid for set and map")
	errFieldTypeNil   = errors.New("nil field type")
	errFieldNameEmpty = errors.New("empty field name")
	errNoFields       = errors.New("no oneof fields")
	errHasFields      = errors.New("fields only valid for struct or oneof")
	errBaseNil        = errors.New("nil base type for named type")
	errBaseCycle      = errors.New("invalid named type cycle")
	errNotBuilt       = errors.New("TypeBuilder.Build must be called before Pending.Built")
)

// Primitive types, the basis for all other types.  All have empty names.
var (
	AnyType        = primitiveType(Any)
	BoolType       = primitiveType(Bool)
	ByteType       = primitiveType(Byte)
	Uint16Type     = primitiveType(Uint16)
	Uint32Type     = primitiveType(Uint32)
	Uint64Type     = primitiveType(Uint64)
	Int16Type      = primitiveType(Int16)
	Int32Type      = primitiveType(Int32)
	Int64Type      = primitiveType(Int64)
	Float32Type    = primitiveType(Float32)
	Float64Type    = primitiveType(Float64)
	Complex64Type  = primitiveType(Complex64)
	Complex128Type = primitiveType(Complex128)
	StringType     = primitiveType(String)
	TypeObjectType = primitiveType(TypeObject)
)

// ErrorType describes the built-in error type.
//
// TODO(toddw): Describe error in a built-in VDL file based on verror2.  At the
// moment this definition must be kept in sync with verror.Standard.
var ErrorType = OptionalType(NamedType("error", StructType(
	Field{"Id", StringType},
	Field{"Msg", StringType},
)))

func primitiveType(k Kind) *Type {
	prim, err := typeCons(&Type{kind: k})
	if err != nil {
		panic(err)
	}
	return prim
}

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

// PendingOptional represents an Optional type that is being built.  Given a base
// type that is non-optional, you can build a new type that is optional.
type PendingOptional interface {
	PendingType
	// AssignElem assigns the Optional elem type.
	AssignElem(elem TypeOrPending) PendingOptional
}

// PendingEnum represents an Enum type that is being built.
type PendingEnum interface {
	PendingType
	// AppendLabel appends an Enum label.  Every Enum must have at least one
	// label, and each label must not be empty.
	AppendLabel(label string) PendingEnum
}

// PendingArray represents an Array type that is being built.
type PendingArray interface {
	PendingType
	// AssignLen assigns the Array length.
	AssignLen(len int) PendingArray
	// AssignElem assigns the Array element type.
	AssignElem(elem TypeOrPending) PendingArray
}

// PendingList represents a List type that is being built.
type PendingList interface {
	PendingType
	// AssignElem assigns the List element type.
	AssignElem(elem TypeOrPending) PendingList
}

// PendingSet represents a Set type that is being built.
type PendingSet interface {
	PendingType
	// AssignKey assigns the Set key type.
	AssignKey(key TypeOrPending) PendingSet
}

// PendingMap represents a Map type that is being built.
type PendingMap interface {
	PendingType
	// AssignKey assigns the Map key type.
	AssignKey(key TypeOrPending) PendingMap
	// AssignElem assigns the Map element type.
	AssignElem(elem TypeOrPending) PendingMap
}

// PendingStruct represents a Struct type that is being built.
type PendingStruct interface {
	PendingType
	// AppendField appends the Struct field with the given name and t.  The name
	// must not be empty.  The ordering of fields is preserved; different
	// orderings create different types.
	AppendField(name string, t TypeOrPending) PendingStruct
	// NumField returns the number of fields appended so far.
	NumField() int
}

// PendingOneOf represents a OneOf type that is being built.
type PendingOneOf interface {
	PendingType
	// AppendField appends the OneOf field with the given name and t.  The name
	// must not be empty.  The ordering of fields is preserved; different
	// orderings create different types.
	AppendField(name string, t TypeOrPending) PendingOneOf
	// NumField returns the number of fields appended so far.
	NumField() int
}

// PendingNamed represents a named type that is being built.  Given a base type
// you can build a new type with an identical underlying structure, but a
// different name.
type PendingNamed interface {
	PendingType
	// AssignBase assigns the base type of the named type.  The resulting built
	// type will have the same underlying structure as base, but with the given
	// name.
	AssignBase(base TypeOrPending) PendingNamed
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
	pendingOptional struct{ *pending }
	pendingEnum     struct{ *pending }
	pendingArray    struct{ *pending }
	pendingList     struct{ *pending }
	pendingSet      struct{ *pending }
	pendingMap      struct{ *pending }
	pendingStruct   struct{ *pending }
	pendingOneOf    struct{ *pending }
	pendingNamed    struct{ *pending }
)

func (p pendingOptional) AssignElem(elem TypeOrPending) PendingOptional {
	p.elem = elem.ptype()
	return p
}

func (p pendingEnum) AppendLabel(label string) PendingEnum {
	p.labels = append(p.labels, label)
	return p
}

func (p pendingArray) AssignLen(len int) PendingArray {
	p.len = len
	return p
}

func (p pendingArray) AssignElem(elem TypeOrPending) PendingArray {
	p.elem = elem.ptype()
	return p
}

func (p pendingList) AssignElem(elem TypeOrPending) PendingList {
	p.elem = elem.ptype()
	return p
}

func (p pendingSet) AssignKey(key TypeOrPending) PendingSet {
	p.key = key.ptype()
	return p
}

func (p pendingMap) AssignKey(key TypeOrPending) PendingMap {
	p.key = key.ptype()
	return p
}

func (p pendingMap) AssignElem(elem TypeOrPending) PendingMap {
	p.elem = elem.ptype()
	return p
}

func (p pendingStruct) AppendField(name string, t TypeOrPending) PendingStruct {
	p.fields = append(p.fields, Field{name, t.ptype()})
	return p
}

func (p pendingStruct) NumField() int {
	return len(p.fields)
}

func (p pendingOneOf) AppendField(name string, t TypeOrPending) PendingOneOf {
	p.fields = append(p.fields, Field{name, t.ptype()})
	return p
}

func (p pendingOneOf) NumField() int {
	return len(p.fields)
}

func (p pendingNamed) AssignBase(base TypeOrPending) PendingNamed {
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

// Optional returns PendingOptional, used to describe an Optional type.
func (b *TypeBuilder) Optional() PendingOptional {
	return pendingOptional{b.add(&Type{kind: Optional})}
}

// Enum returns PendingEnum, used to describe an Enum type.
func (b *TypeBuilder) Enum() PendingEnum {
	return pendingEnum{b.add(&Type{kind: Enum})}
}

// Array returns PendingArray, used to describe an Array type.
func (b *TypeBuilder) Array() PendingArray {
	return pendingArray{b.add(&Type{kind: Array})}
}

// List returns PendingList, used to describe a List type.
func (b *TypeBuilder) List() PendingList {
	return pendingList{b.add(&Type{kind: List})}
}

// Set returns PendingSet, used to describe a Set type.
func (b *TypeBuilder) Set() PendingSet {
	return pendingSet{b.add(&Type{kind: Set})}
}

// Map returns PendingMap, used to describe a Map type.
func (b *TypeBuilder) Map() PendingMap {
	return pendingMap{b.add(&Type{kind: Map})}
}

// Struct returns PendingStruct, used to describe a Struct type.
func (b *TypeBuilder) Struct() PendingStruct {
	return pendingStruct{b.add(&Type{kind: Struct})}
}

// OneOf returns PendingOneOf, used to describe a OneOf type.
func (b *TypeBuilder) OneOf() PendingOneOf {
	return pendingOneOf{b.add(&Type{kind: OneOf})}
}

// Named returns PendingNamed, used to describe a named type based on another
// type.
func (b *TypeBuilder) Named(name string) PendingNamed {
	return pendingNamed{b.add(&Type{kind: internalNamed, name: name})}
}

// Build builds all pending types.  Build must be called before Built may be
// called on each pending type to retrieve the final result.
//
// TODO(toddw): Change Build() to return an error rather than a boolean.
func (b *TypeBuilder) Build() bool {
	// First finalize all types, indicating no more mutations will occur.
	for _, p := range b.ptypes {
		p.err = p.finalize()
	}
	// Now enforce the rule that type names are unique.  This must occur before we
	// hash cons anything, to catch tricky cases where hash consing is difficult.
	// See uniqueTypeStr for more info.
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
			return false
		}
	}
	return true
}

func (p *pending) ptype() *Type { return p.Type }

// finalize indicates Build has been called, and the pending type will not be
// mutated anymore.
func (p *pending) finalize() error {
	if p.Type.kind == internalNamed {
		// Now that the mutations have finished, we can copy the base type into
		// p.Type, keeping the name of p.Type.
		name, base := p.Type.name, p.Type.elem
		if name == "" {
			return fmt.Errorf("PendingNamed used to build unnamed type based on %v", base)
		}
		// There may be a chain of named types, in which case we'll need to follow
		// the chain to the first type that's not internalNamed.
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
		p.Type.unique = ""
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

// OptionalType is a helper using TypeBuilder to create a single Optional type.
// Panics on all errors.
func OptionalType(elem *Type) *Type {
	var b TypeBuilder
	return checkedBuild(b, b.Optional().AssignElem(elem))
}

// EnumType is a helper using TypeBuilder to create a single Enum type.
// Panics on all errors.
func EnumType(labels ...string) *Type {
	var b TypeBuilder
	e := b.Enum()
	for _, l := range labels {
		e.AppendLabel(l)
	}
	return checkedBuild(b, e)
}

// ArrayType is a helper using TypeBuilder to create a single Array type.
// Panics on all errors.
func ArrayType(len int, elem *Type) *Type {
	var b TypeBuilder
	return checkedBuild(b, b.Array().AssignLen(len).AssignElem(elem))
}

// ListType is a helper using TypeBuilder to create a single List type.  Panics
// on all errors.
func ListType(elem *Type) *Type {
	var b TypeBuilder
	return checkedBuild(b, b.List().AssignElem(elem))
}

// SetType is a helper using TypeBuilder to create a single Set type.  Panics on
// all errors.
func SetType(key *Type) *Type {
	var b TypeBuilder
	return checkedBuild(b, b.Set().AssignKey(key))
}

// MapType is a helper using TypeBuilder to create a single Map type.  Panics
// on all errors.
func MapType(key, elem *Type) *Type {
	var b TypeBuilder
	return checkedBuild(b, b.Map().AssignKey(key).AssignElem(elem))
}

// StructType is a helper using TypeBuilder to create a single Struct type.
// Panics on all errors.
func StructType(fields ...Field) *Type {
	var b TypeBuilder
	s := b.Struct()
	for _, f := range fields {
		s.AppendField(f.Name, f.Type)
	}
	return checkedBuild(b, s)
}

// OneOfType is a helper using TypeBuilder to create a single OneOf type.
// Panics on all errors.
func OneOfType(fields ...Field) *Type {
	var b TypeBuilder
	o := b.OneOf()
	for _, f := range fields {
		o.AppendField(f.Name, f.Type)
	}
	return checkedBuild(b, o)
}

// NamedType is a helper using TypeBuilder to create a single named type based
// on another type.  Panics on all errors.
func NamedType(name string, base *Type) *Type {
	var b TypeBuilder
	return checkedBuild(b, b.Named(name).AssignBase(base))
}

// enforceUniqueNames ensures that t and its subtypes contain unique type names;
// every non-empty type name corresponds to exactly one *Type.
func enforceUniqueNames(t *Type, names map[string]*Type) error {
	if t == nil || t.name == "" {
		return nil
	}
	if found := names[t.name]; found != nil {
		if found != t {
			return fmt.Errorf("duplicate type names %q and %q", found, t)
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
	return nil
}

// uniqueTypeStr returns a unique string representing t, which is also its
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
func uniqueTypeStr(t *Type, seen map[*Type]bool) string {
	if seen[t] && t.name != "" {
		return t.name
	}
	seen[t] = true
	s := t.name
	if s != "" {
		s += " "
	}
	switch t.kind {
	case Optional:
		return s + "?" + uniqueTypeStr(t.elem, seen)
	case Enum:
		return s + "enum{" + strings.Join(t.labels, ";") + "}"
	case Array:
		return s + "[" + strconv.Itoa(t.len) + "]" + uniqueTypeStr(t.elem, seen)
	case List:
		return s + "[]" + uniqueTypeStr(t.elem, seen)
	case Set:
		return s + "set[" + uniqueTypeStr(t.key, seen) + "]"
	case Map:
		return s + "map[" + uniqueTypeStr(t.key, seen) + "]" + uniqueTypeStr(t.elem, seen)
	case Struct, OneOf:
		if t.kind == Struct {
			s += "struct{"
		} else {
			s += "oneof{"
		}
		for index, f := range t.fields {
			if index > 0 {
				s += ";"
			}
			s += f.Name + " " + uniqueTypeStr(f.Type, seen)
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
	if err := validType(t); err != nil {
		return nil, err
	}
	typeRegMu.Lock()
	cons := typeConsLocked(t)
	typeRegMu.Unlock()
	return cons, nil
}

func typeConsLocked(t *Type) *Type {
	if t == nil {
		return nil
	}
	// Look for the type in our registry, based on its unique string.
	if t.unique == "" {
		t.unique = uniqueTypeStr(t, make(map[*Type]bool))
	}
	if found := typeReg[t.unique]; found != nil {
		return found
	}
	// Not found in the registry, add it and recursively cons subtypes.  We cons
	// the outer type first to deal with recursive types; otherwise we'd have an
	// infinite loop.
	typeReg[t.unique] = t
	t.elem = typeConsLocked(t.elem)
	t.key = typeConsLocked(t.key)
	for index := range t.fields {
		t.fields[index].Type = typeConsLocked(t.fields[index].Type)
	}
	return t
}

// validType returns a nil error iff t and all subtypes are valid.
func validType(t *Type) error {
	allTypes := make(map[*Type]bool)
	if err := verifyAndCollectAllTypes(t, allTypes); err != nil {
		return err
	}
	if err := existsStrictCycle(allTypes); err != nil {
		return err
	}
	if err := existsInvalidKey(allTypes); err != nil {
		return err
	}
	return nil
}

// isValidKey returns true iff t is a valid set or map key.
func isValidKey(t *Type, seen map[*Type]bool) bool {
	if seen[t] {
		return true
	}
	seen[t] = true
	switch t.kind {
	case Any, List, Map, Optional, Set, TypeObject:
		return false
	case Array:
		return isValidKey(t.elem, seen)
	case Struct, OneOf:
		for _, x := range t.fields {
			if !isValidKey(x.Type, seen) {
				return false
			}
		}
	}
	return true
}

// existsInvalidKey returns a nil error iff the given Types all have valid set
// and map keys.
func existsInvalidKey(allTypes map[*Type]bool) error {
	seen := make(map[*Type]bool)
	for t, _ := range allTypes {
		if (t.kind == Map || t.kind == Set) && !isValidKey(t.key, seen) {
			return fmt.Errorf("invalid key %q in %q", t.key, t)
		}
	}
	return nil
}

// typeInStrictCycle returns a subtype that belongs to a strict cycle or nil if
// there are no strict cycles
func typeInStrictCycle(t *Type, seen map[*Type]int) *Type {
	if seen[t] == 1 {
		return t
	}
	if seen[t] == 2 {
		return nil
	}
	seen[t] = 1
	switch t.kind {
	case Array:
		if typeInCycle := typeInStrictCycle(t.elem, seen); typeInCycle != nil {
			return typeInCycle
		}
	case Struct, OneOf:
		for _, x := range t.fields {
			if typeInCycle := typeInStrictCycle(x.Type, seen); typeInCycle != nil {
				return typeInCycle
			}
		}
	}
	seen[t] = 2
	return nil
}

// existsStrictCycle returns a nil error iff the given type set has no strict
// cycles (e.g. type A struct{Elem: A})
func existsStrictCycle(allTypes map[*Type]bool) error {
	seen := make(map[*Type]int)
	for t, _ := range allTypes {
		if typeInCycle := typeInStrictCycle(t, seen); typeInCycle != nil {
			return fmt.Errorf("type %q is inside of a strict cycle", typeInCycle)
		}
	}
	return nil
}

// verifyAndCollectAllTypes returns a nil error iff t and all subtypes are
// correctly defined. If all subtypes are correctly defined allTypes will be
// filled with all subtypes of the given Type t.
func verifyAndCollectAllTypes(t *Type, allTypes map[*Type]bool) error {
	if t == nil || allTypes[t] {
		return nil
	}
	allTypes[t] = true
	// Check kind
	switch t.kind {
	case internalNamed:
		panic(fmt.Errorf("internal kind %d used in verifyAndCollectAllTypes", t.kind))
	}
	// Check name
	switch t.kind {
	case Any, TypeObject:
		if t.name != "" {
			return errNameNonEmpty
		}
	}
	// Check len
	switch t.kind {
	case Array:
		if t.len <= 0 {
			return errLenZero
		}
	default:
		if t.len != 0 {
			return errLenNonZero
		}
	}
	// Check elem
	switch t.kind {
	case Array, List, Map:
		if t.elem == nil {
			return errElemNil
		}
	case Optional:
		if t.elem == nil {
			return errElemNil
		}
		if !t.elem.CanBeOptional() {
			return fmt.Errorf("invalid optional type %q", t)
		}
	default:
		if t.elem != nil {
			return errElemNonNil
		}
	}
	// Check key
	switch t.kind {
	case Set, Map:
		if t.key == nil {
			return errKeyNil
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
	case Struct, OneOf:
		seenFields := make(map[string]bool, len(t.fields))
		for _, f := range t.fields {
			if f.Type == nil {
				return errFieldTypeNil
			}
			if f.Name == "" {
				return errFieldNameEmpty
			}
			if seenFields[f.Name] {
				return fmt.Errorf("%q has duplicate field name %q", t.name, f.Name)
			}
			seenFields[f.Name] = true
		}
		// We allow struct{} but not oneof{}; we rely on oneof having at least one
		// field, and we special-case field 0.  E.g. the zero value of oneof is the
		// zero value of the type of field 0.
		if t.kind == OneOf && len(t.fields) == 0 {
			return errNoFields
		}
	default:
		if len(t.fields) > 0 {
			return errHasFields
		}
	}
	// Check subtypes recursively.
	if err := verifyAndCollectAllTypes(t.elem, allTypes); err != nil {
		return err
	}
	if err := verifyAndCollectAllTypes(t.key, allTypes); err != nil {
		return err
	}
	for _, x := range t.fields {
		if err := verifyAndCollectAllTypes(x.Type, allTypes); err != nil {
			return err
		}
	}
	return nil
}
