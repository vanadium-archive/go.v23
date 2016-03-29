// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"
)

var (
	errNilType       = errors.New("vdl: nil *Type is invalid")
	errNonNilZeroAny = errors.New("vdl: the any type doesn't have a non-nil zero value")
)

// Value is the generic representation of any value expressible in vanadium.  All
// values are typed.
//
// Not all methods apply to all kinds of values.  Restrictions are noted in the
// documentation for each method.  Calling a method inappropriate to the kind of
// value causes a run-time panic.
//
// Cyclic values are not supported.  The zero Value is invalid; use Zero or one
// of the *Value helper functions to create a valid Value.
type Value struct {
	// Each value is represented by a non-nil Type, along with a different
	// representation depending on the kind of Type.
	t   *Type
	rep interface{} // see zeroRep for allowed types
}

var zeroTypeObject = AnyType // the zero TypeObject returns the any type

// enumIndex represents an enum value by the index of its label.
type enumIndex int

// zeroRep returns the zero representation for each kind of type.
func zeroRep(t *Type) interface{} {
	if t.IsBytes() {
		// Represent []byte and [N]byte as repBytes.
		// Represent repBytes.Index as *byte.
		return make(repBytes, t.len)
	}
	switch t.kind {
	case Bool:
		return false
	case Byte, Uint16, Uint32, Uint64:
		return uint64(0)
	case Int8, Int16, Int32, Int64:
		return int64(0)
	case Float32, Float64:
		return float64(0)
	case String:
		return ""
	case Enum:
		return enumIndex(0)
	case TypeObject:
		return zeroTypeObject
	case Array:
		return make(repSequence, t.len)
	case List:
		return repSequence(nil)
	case Set, Map:
		return zeroRepMap(t.key)
	case Struct:
		return make(repSequence, len(t.fields))
	case Union:
		return repUnion{0, ZeroValue(t.fields[0].Type)}
	case Any, Optional:
		return (*Value)(nil) // nil represents nonexistence
	default:
		panic(fmt.Errorf("vdl: unhandled kind: %v", t.kind))
	}
}

func isZeroRep(t *Type, rep interface{}) bool {
	switch trep := rep.(type) {
	case bool:
		return !trep
	case uint64:
		return trep == 0
	case int64:
		return trep == 0
	case float64:
		return trep == 0
	case string:
		return trep == ""
	case enumIndex:
		return trep == 0
	case *Type:
		return trep == zeroTypeObject
	case repBytes:
		return trep.IsZero(t)
	case *byte:
		return *trep == 0
	case repMap:
		return trep.Len() == 0
	case repSequence:
		switch t.Kind() {
		case List:
			return len(trep) == 0
		case Array, Struct:
			return trep.AllValuesZero()
		}
	case repUnion:
		return trep.IsZero()
	case *Value:
		return trep == nil
	}
	panic(fmt.Errorf("vdl: isZeroRep unhandled %v %T %v", t, rep, rep))
}

func copyRep(t *Type, rep interface{}) interface{} {
	switch trep := rep.(type) {
	case bool, uint64, int64, float64, string, enumIndex, *Type:
		return trep
	case repBytes:
		return copyRepBytes(trep)
	case *byte:
		return uint64(*trep) // convert to standard uint64 representation
	case repMap:
		return copyRepMap(trep)
	case repSequence:
		return copyRepSequence(trep)
	case repUnion:
		return copyRepUnion(trep)
	case *Value:
		return CopyValue(trep)
	default:
		panic(fmt.Errorf("vdl: copyRep unhandled %v %T %v", t.kind, rep, rep))
	}
}

func stringRep(t *Type, rep interface{}) string {
	switch trep := rep.(type) {
	case bool, uint64, int64, float64:
		return fmt.Sprint(trep)
	case string:
		return strconv.Quote(trep)
	case enumIndex:
		return t.labels[int(trep)]
	case *Type:
		return trep.String()
	case repBytes:
		return strconv.Quote(string(trep))
	case *byte:
		return fmt.Sprint(*trep)
	case repMap:
		return trep.String()
	case repSequence:
		return trep.String(t)
	case repUnion:
		return trep.String(t)
	case *Value:
		switch {
		case trep == nil:
			return "nil"
		case t.kind == Optional:
			return stringRep(t.elem, trep.rep) // don't include the type
		}
		return trep.String() // include the type
	default:
		panic(fmt.Errorf("vdl: stringRep unhandled %v %T %v", t.kind, rep, rep))
	}
}

// AnyValue is a convenience to create an Any value.
func AnyValue(x *Value) *Value { return ZeroValue(AnyType).Assign(x) }

// OptionalValue returns an optional value with elem assigned to x.  Panics if
// the type of x cannot be made optional.
func OptionalValue(x *Value) *Value { return &Value{OptionalType(x.t), x} }

// BoolValue is a convenience to create a Bool value.
func BoolValue(x bool) *Value { return ZeroValue(BoolType).AssignBool(x) }

// ByteValue is a convenience to create an Byte value.
func ByteValue(x byte) *Value { return ZeroValue(ByteType).AssignUint(uint64(x)) }

// Uint16Value is a convenience to create an Uint16 value.
func Uint16Value(x uint16) *Value { return ZeroValue(Uint16Type).AssignUint(uint64(x)) }

// Uint32Value is a convenience to create an Uint32 value.
func Uint32Value(x uint32) *Value { return ZeroValue(Uint32Type).AssignUint(uint64(x)) }

// Uint64Value is a convenience to create an Uint64 value.
func Uint64Value(x uint64) *Value { return ZeroValue(Uint64Type).AssignUint(x) }

// Int8Value is a convenience to create an Int8 value.
func Int8Value(x int8) *Value { return ZeroValue(Int8Type).AssignInt(int64(x)) }

// Int16Value is a convenience to create an Int16 value.
func Int16Value(x int16) *Value { return ZeroValue(Int16Type).AssignInt(int64(x)) }

// Int32Value is a convenience to create an Int32 value.
func Int32Value(x int32) *Value { return ZeroValue(Int32Type).AssignInt(int64(x)) }

// Int64Value is a convenience to create an Int64 value.
func Int64Value(x int64) *Value { return ZeroValue(Int64Type).AssignInt(x) }

// Float32Value is a convenience to create a Float32 value.
func Float32Value(x float32) *Value { return ZeroValue(Float32Type).AssignFloat(float64(x)) }

// Float64Value is a convenience to create a Float64 value.
func Float64Value(x float64) *Value { return ZeroValue(Float64Type).AssignFloat(x) }

// StringValue is a convenience to create a String value.
func StringValue(x string) *Value { return ZeroValue(StringType).AssignString(x) }

// BytesValue is a convenience to create a []byte value.  The bytes are copied.
func BytesValue(x []byte) *Value { return ZeroValue(ListType(ByteType)).AssignBytes(x) }

// TypeObjectValue is a convenience to create a TypeObject value.
func TypeObjectValue(x *Type) *Value { return ZeroValue(TypeObjectType).AssignTypeObject(x) }

// ZeroValue returns a new Value of type t representing the zero value for t:
//   o Bool:       false
//   o Numbers:    0
//   o String:     ""
//   o Enum:       label at index 0
//   o TypeObject: AnyType
//   o List:       empty collection
//   o Set:        empty collection
//   o Map:        empty collection
//   o Array:      zero values for all elems
//   o Struct:     zero values for all fields
//   o Union:      zero value of the type at index 0
//   o Any:        nil value, representing nonexistence
//   o Optional:   nil value, representing nonexistence
//
// Panics if t == nil.
func ZeroValue(t *Type) *Value {
	if t == nil {
		panic(errNilType)
	}
	return &Value{t, zeroRep(t)}
}

// NonNilZeroValue returns a new Value of type t representing the non-nil zero
// value for t.  It is is the same as ZeroValue, except if t is Optional, in
// which case it returns a Value representing the zero value of the elem type.
//
// Panics if t == nil or t is Any.
func NonNilZeroValue(t *Type) *Value {
	if t == nil {
		panic(errNilType)
	}
	switch t.kind {
	case Any:
		panic(errNonNilZeroAny)
	case Optional:
		return &Value{t, ZeroValue(t.elem)}
	}
	return ZeroValue(t)
}

// CopyValue returns a copy of the Value v.
func CopyValue(v *Value) *Value {
	if v == nil {
		return nil
	}
	return &Value{v.t, copyRep(v.t, v.rep)}
}

// EqualValue returns true iff a and b have the same type, and equal values.
func EqualValue(a, b *Value) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if a.t != b.t {
		return false
	}
	if a.t == ByteType {
		// ByteType has two representations, either uint64 or *byte.
		return a.Uint() == b.Uint()
	}
	switch arep := a.rep.(type) {
	case bool:
		return arep == b.rep.(bool)
	case uint64:
		return arep == b.rep.(uint64)
	case int64:
		return arep == b.rep.(int64)
	case float64:
		return arep == b.rep.(float64)
	case string:
		return arep == b.rep.(string)
	case enumIndex:
		return arep == b.rep.(enumIndex)
	case *Type:
		return arep == b.rep.(*Type)
	case repBytes:
		return bytes.Equal(arep, b.rep.(repBytes))
	case repMap:
		return equalRepMap(arep, b.rep.(repMap))
	case repSequence:
		return equalRepSequence(arep, b.rep.(repSequence))
	case repUnion:
		return equalRepUnion(arep, b.rep.(repUnion))
	case *Value:
		return EqualValue(arep, b.rep.(*Value))
	default:
		panic(fmt.Errorf("vdl: EqualValue unhandled %v %T %v", a.t.kind, arep, arep))
	}
}

// IsZero returns true iff v is the zero value for its type.
func (v *Value) IsZero() bool {
	return isZeroRep(v.t, v.rep)
}

// IsNil returns true iff v is Optional or Any and has the nil value.
func (v *Value) IsNil() bool {
	vrep, ok := v.rep.(*Value)
	return ok && vrep == nil
}

// IsValid returns true iff v is valid.  Nil and new(Value) are invalid; most
// other methods panic if called on an invalid Value.
func (v *Value) IsValid() bool {
	return v != nil && v.t != nil
}

// Kind returns the kind of type of v.
func (v *Value) Kind() Kind { return v.t.kind }

// Type returns the type of v.  All valid values have a non-nil type.
func (v *Value) Type() *Type { return v.t }

// Bool returns the underlying value of a Bool.
func (v *Value) Bool() bool {
	v.t.checkKind("Bool", Bool)
	return v.rep.(bool)
}

// Uint returns the underlying value of a Byte or Uint{16,32,64}.
func (v *Value) Uint() uint64 {
	v.t.checkKind("Uint", Byte, Uint16, Uint32, Uint64)
	switch trep := v.rep.(type) {
	case uint64:
		return trep
	case *byte:
		return uint64(*trep)
	}
	panic(fmt.Errorf("vdl: Uint mismatched rep %v %T %v", v.t, v.rep, v.rep))
}

// Int returns the underlying value of an Int{8,16,32,64}.
func (v *Value) Int() int64 {
	v.t.checkKind("Int", Int8, Int16, Int32, Int64)
	return v.rep.(int64)
}

// Float returns the underlying value of a Float{32,64}.
func (v *Value) Float() float64 {
	v.t.checkKind("Float", Float32, Float64)
	return v.rep.(float64)
}

// RawString returns the underlying value of a String.
func (v *Value) RawString() string {
	v.t.checkKind("RawString", String)
	return v.rep.(string)
}

// String returns a human-readable representation of the value.
// To retrieve the underlying value of a String, use RawString.
func (v *Value) String() string {
	if !v.IsValid() {
		// This occurs if the user calls new(Value).String().
		return "INVALID"
	}
	switch v.t {
	case BoolType, StringType:
		// Unnamed bool and string don't need the type to be printed.
		return stringRep(v.t, v.rep)
	}
	switch v.t.Kind() {
	case Array, List, Set, Map, Struct, Union:
		// { } are used instead of ( ) for composites, except for []byte and [N]byte
		if !v.t.IsBytes() {
			return v.t.String() + stringRep(v.t, v.rep)
		}
	}
	return v.t.String() + "(" + stringRep(v.t, v.rep) + ")"
}

// Bytes returns the underlying value of a []byte or [N]byte.  Mutations of the
// returned value are reflected in the underlying value.
func (v *Value) Bytes() []byte {
	v.t.checkIsBytes("Bytes")
	return v.rep.(repBytes)
}

// EnumIndex returns the index of the underlying Enum.
func (v *Value) EnumIndex() int {
	v.t.checkKind("EnumIndex", Enum)
	return int(v.rep.(enumIndex))
}

// EnumLabel returns the label of the underlying Enum.
func (v *Value) EnumLabel() string {
	v.t.checkKind("EnumLabel", Enum)
	return v.t.labels[int(v.rep.(enumIndex))]
}

// TypeObject returns the underlying value of a TypeObject.
func (v *Value) TypeObject() *Type {
	v.t.checkKind("TypeObject", TypeObject)
	return v.rep.(*Type)
}

// Len returns the length of the underlying Array, List, Set or Map.
func (v *Value) Len() int {
	switch trep := v.rep.(type) {
	case repMap:
		return trep.Len()
	case repSequence:
		if v.t.kind != Struct { // Len not allowed on Struct
			return len(trep)
		}
	case repBytes:
		return len(trep)
	}
	panic(v.t.errKind("Len", Array, List, Set, Map))
}

// Index returns the i'th element of the underlying Array or List.  Panics if
// the index is out of range.
func (v *Value) Index(index int) *Value {
	switch trep := v.rep.(type) {
	case repSequence:
		if v.t.kind != Struct { // Index not allowed on Struct
			return trep.Index(v.t.elem, index)
		}
	case repBytes:
		// The user is trying to index into a []byte or [N]byte, and we need to
		// return a valid Value that behaves as usual; e.g. AssignByte should work
		// correctly and update the underlying byteslice.  The strategy is to return
		// a new Value with rep set to the indexed *byte.
		return &Value{ByteType, &trep[index]}
	}
	panic(v.t.errKind("Index", Array, List))
}

// Keys returns all keys present in the underlying Set or Map.  The returned
// keys are in an arbitrary order; do not rely on the ordering.
func (v *Value) Keys() []*Value {
	v.t.checkKind("Keys", Set, Map)
	return v.rep.(repMap).Keys()
}

// ContainsKey returns true iff key is present in the underlying Set or Map.
func (v *Value) ContainsKey(key *Value) bool {
	v.t.checkKind("ContainsKey", Set, Map)
	_, ok := v.rep.(repMap).Index(typedCopy(v.t.key, key))
	return ok
}

// MapIndex returns the value associated with the key in the underlying Map, or
// nil if the key is not found in the map.  Panics if the key isn't assignable
// to the map's key type.
func (v *Value) MapIndex(key *Value) *Value {
	v.t.checkKind("MapIndex", Map)
	val, _ := v.rep.(repMap).Index(typedCopy(v.t.key, key))
	return val
}

// StructField returns the Struct field at the given index.  Panics if the index
// is out of range.
func (v *Value) StructField(index int) *Value {
	v.t.checkKind("StructField", Struct)
	return v.rep.(repSequence).Index(v.t.fields[index].Type, index)
}

// StructFieldByName returns the Struct field for the given name.  Returns nil
// if the name given is not one of the struct's fields.
func (v *Value) StructFieldByName(name string) *Value {
	v.t.checkKind("StructFieldByName", Struct)
	_, index := v.t.FieldByName(name)
	if index == -1 {
		return nil
	}
	return v.rep.(repSequence).Index(v.t.fields[index].Type, index)
}

// UnionField returns the field index and value from the underlying Union.
func (v *Value) UnionField() (int, *Value) {
	v.t.checkKind("UnionField", Union)
	union := v.rep.(repUnion)
	return union.index, union.value
}

// Elem returns the element value contained in the underlying Any or Optional.
// Returns nil if v.IsNil() == true.
func (v *Value) Elem() *Value {
	v.t.checkKind("Elem", Any, Optional)
	return v.rep.(*Value)
}

// Assign the value v to x.  If x is nil, v is set to its zero value.  Panics if
// the type of v is not assignable from the type of x.
func (v *Value) Assign(x *Value) *Value {
	// The logic here mirrors our definition of Type.AssignableFrom.
	switch {
	case x == nil:
		// Assign(nil) sets the zero value.
		if v.t.kind == Byte {
			// Use AssignUint to handle both the value and pointer cases.
			v.AssignUint(0)
		} else {
			v.rep = zeroRep(v.t)
		}
	case v.t == x.t:
		if v.t.kind == Byte {
			// Use AssignUint to handle both the value and pointer cases.
			v.AssignUint(x.Uint())
		} else {
			// Types are identical, v is assigned a copy of the underlying rep.
			v.rep = copyRep(x.t, x.rep)
		}
	case v.t.kind == Any:
		// Assigning into Any, v is assigned a copy of the value.
		v.rep = CopyValue(x)
	case v.t.kind == Optional && x.t.kind == Any && x.IsNil():
		// Assigning into Optional from Any(nil), v is reset to nil.
		v.rep = (*Value)(nil)
	default:
		panic(fmt.Errorf("vdl: value of type %q not assignable from %q", v.t, x.t))
	}
	return v
}

// typedCopy makes a copy of v, returning a result of type t.  Panics if values
// of type t aren't assignable from v.
func typedCopy(t *Type, v *Value) *Value {
	cp := &Value{t: t}
	cp.Assign(v)
	return cp
}

// AssignBool assigns the underlying Bool to x.
func (v *Value) AssignBool(x bool) *Value {
	v.t.checkKind("AssignBool", Bool)
	v.rep = x
	return v
}

// AssignUint assigns the underlying Uint{16,32,64} or Byte to x.
func (v *Value) AssignUint(x uint64) *Value {
	v.t.checkKind("AssignUint", Byte, Uint16, Uint32, Uint64)
	switch trep := v.rep.(type) {
	case uint64, nil:
		// Handle cases where v.rep is a standalone number, or where v.rep == nil.
		// The nil case occurs when typedCopy is used to copy a uint value.
		v.rep = x
	case *byte:
		// Handle case where v.rep represents a byte in a list or array.
		*trep = byte(x)
	default:
		panic(fmt.Errorf("vdl: AssignUint mismatched rep %v %T %v", v.t, v.rep, v.rep))
	}
	return v
}

// AssignInt assigns the underlying Int{8,16,32,64} to x.
func (v *Value) AssignInt(x int64) *Value {
	v.t.checkKind("AssignInt", Int8, Int16, Int32, Int64)
	v.rep = x
	return v
}

// AssignFloat assigns the underlying Float{32,64} to x.
func (v *Value) AssignFloat(x float64) *Value {
	v.t.checkKind("AssignFloat", Float32, Float64)
	v.rep = x
	return v
}

// AssignString assigns the underlying String to x.
func (v *Value) AssignString(x string) *Value {
	v.t.checkKind("AssignString", String)
	v.rep = x
	return v
}

// AssignBytes assigns the underlying []byte or [N]byte to a copy of x.  If the
// underlying value is []byte, the resulting v has len == len(x).  If the
// underlying value is [N]byte, we require len(x) == N, otherwise panics.
func (v *Value) AssignBytes(x []byte) *Value {
	v.t.checkIsBytes("AssignBytes")
	switch v.t.kind {
	case Array:
		rep := v.rep.(repBytes)
		if v.t.len != len(x) {
			panic(fmt.Errorf("vdl: AssignBytes on type [%d]byte with len %d", v.t.len, len(x)))
		}
		copy(rep, x)
	case List:
		oldrep, newlen := v.rep.(repBytes), len(x)
		var newrep repBytes
		if newlen <= cap(oldrep) {
			newrep = oldrep[:newlen]
		} else {
			newrep = make(repBytes, newlen)
		}
		copy(newrep, x)
		v.rep = newrep
	default:
		panic(v.t.errBytes("AssignBytes"))
	}
	return v
}

// CopyBytes copies bytes from x to v for the underlying []byte or [N]byte.  The
// semantics are the same as the built-in copy function; min(v.Len(), len(x))
// bytes are copied from x to v.
func (v *Value) CopyBytes(x []byte) *Value {
	v.t.checkIsBytes("CopyBytes")
	copy(v.rep.(repBytes), x)
	return v
}

// AssignEnumIndex assigns the underlying Enum to the label corresponding to
// index.  Panics if the index is out of range.
func (v *Value) AssignEnumIndex(index int) *Value {
	v.t.checkKind("AssignEnumIndex", Enum)
	if index < 0 || index >= len(v.t.labels) {
		panic(fmt.Errorf("vdl: enum %q index %d out of range", v.t.name, index))
	}
	v.rep = enumIndex(index)
	return v
}

// AssignEnumLabel assigns the underlying Enum to the label.  Panics if the
// label doesn't exist in the Enum.
func (v *Value) AssignEnumLabel(label string) *Value {
	v.t.checkKind("AssignEnumLabel", Enum)
	index := v.t.EnumIndex(label)
	if index == -1 {
		panic(fmt.Errorf("vdl: enum %q doesn't have label %q", v.t.name, label))
	}
	v.rep = enumIndex(index)
	return v
}

// AssignTypeObject assigns the underlying TypeObject to x.  If x == nil we
// assign the zero TypeObject.
func (v *Value) AssignTypeObject(x *Type) *Value {
	v.t.checkKind("AssignTypeObject", TypeObject)
	if x == nil {
		x = zeroTypeObject
	}
	v.rep = x
	return v
}

// AssignLen assigns the length of the underlying List to n.  Unlike Go slices,
// Lists do not have a separate notion of capacity.
func (v *Value) AssignLen(n int) *Value {
	v.t.checkKind("AssignLen", List)
	if oldrep, ok := v.rep.(repBytes); ok {
		var newrep repBytes
		if n <= cap(oldrep) {
			newrep = oldrep[:n]
			// Fill newrep[oldlen:n] with zero bytes.
			for zx := len(oldrep); zx < n; {
				zx += copy(newrep[zx:], allZeroBytes)
			}
		} else {
			newrep = make(repBytes, n)
			copy(newrep, oldrep)
		}
		v.rep = newrep
		return v
	}
	oldrep := v.rep.(repSequence)
	var newrep repSequence
	if n <= cap(oldrep) {
		newrep = oldrep[:n]
		for ix := len(oldrep); ix < n; ix++ {
			newrep[ix] = nil
		}
	} else {
		newrep = make(repSequence, n)
		copy(newrep, oldrep)
	}
	v.rep = newrep
	return v
}

// AssignSetKey assigns key to the underlying Set.  Panics if the key isn't
// assignable to the set's key type.
func (v *Value) AssignSetKey(key *Value) *Value {
	v.t.checkKind("AssignSetKey", Set)
	v.rep.(repMap).Assign(typedCopy(v.t.key, key), nil)
	return v
}

// DeleteSetKey deletes key from the underlying Set.
func (v *Value) DeleteSetKey(key *Value) *Value {
	v.t.checkKind("DeleteSetKey", Set)
	v.rep.(repMap).Delete(typedCopy(v.t.key, key))
	return v
}

// AssignMapIndex assigns the value associated with key to elem in the
// underlying Map.  If elem is nil, AssignMapIndex deletes the key from the Map.
// Panics if the key isn't assignable to the map's key type, and ditto for elem.
func (v *Value) AssignMapIndex(key, elem *Value) *Value {
	v.t.checkKind("AssignMapIndex", Map)
	if elem == nil {
		v.rep.(repMap).Delete(typedCopy(v.t.key, key))
	} else {
		v.rep.(repMap).Assign(typedCopy(v.t.key, key), typedCopy(v.t.elem, elem))
	}
	return v
}

// AssignUnionField assigns the field index and value to the underlying Union.
// Panics if the index is out of range, or if the value isn't assignable to the
// field type.
func (v *Value) AssignUnionField(index int, value *Value) *Value {
	v.t.checkKind("AssignUnionField", Union)
	if index >= len(v.t.fields) {
		panic(fmt.Errorf("vdl: union %q index %d out of range", v.t, index))
	}
	v.rep = repUnion{index, typedCopy(v.t.fields[index].Type, value)}
	return v
}

// SortValuesAsString sorts values by their String representation.  The order of
// elements in values may be changed, and values is returned; no copy is made.
//
// The ordering is guaranteed to be deterministic within a single executable,
// but may change across different versions of the code.
//
// Typically used to get a deterministic ordering of set and map keys in tests.
// Do not depend on the ordering across versions of the code; it will change.
func SortValuesAsString(values []*Value) []*Value {
	sort.Sort(orderValuesAsString(values))
	return values
}

type orderValuesAsString []*Value

func (x orderValuesAsString) Len() int           { return len(x) }
func (x orderValuesAsString) Less(i, j int) bool { return x[i].String() < x[j].String() }
func (x orderValuesAsString) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
