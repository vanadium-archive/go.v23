package val

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

// Value is the generic representation of any value expressible in veyron.  All
// values are typed.
//
// Not all methods apply to all kinds of values.  Restrictions are noted in the
// documentation for each method.  Calling a method inappropriate to the kind of
// value causes a run-time panic.
//
// Cyclic values are not supported.
type Value struct {
	// Each value is represented by a non-nil Type, along with a different
	// representation depending on the kind of Type.
	t   *Type
	rep interface{} // see zeroRep for allowed types
}

var zeroTypeVal = AnyType // the zero TypeVal returns the any type

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
	case Int16, Int32, Int64:
		return int64(0)
	case Float32, Float64:
		return float64(0)
	case Complex64, Complex128:
		return complex128(0)
	case String:
		return ""
	case TypeVal:
		return zeroTypeVal
	case Enum:
		return enumIndex(0)
	case List:
		return make([]*Value, 0)
	case Set, Map:
		return zeroRepMap(t.key)
	case Array:
		return make(repFixedLen, t.len)
	case Struct:
		return make(repFixedLen, len(t.fields))
	case OneOf, Any:
		// The nil value represents non-existence.
		return (*Value)(nil)
	default:
		panic(fmt.Errorf("val: unhandled kind: %v", t.kind))
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
	case complex128:
		return trep == 0
	case string:
		return trep == ""
	case *Type:
		return trep == zeroTypeVal
	case enumIndex:
		return trep == 0
	case []*Value:
		return len(trep) == 0
	case repBytes:
		return trep.IsZero(t)
	case *byte:
		return *trep == 0
	case repMap:
		return trep.Len() == 0
	case repFixedLen:
		return trep.IsZero()
	case *Value:
		return trep == nil
	default:
		panic(fmt.Errorf("val: isZeroRep unhandled %v %T %v", t, rep, rep))
	}
}

// copyRep returns a copy of v.rep.
func copyRep(v *Value) interface{} {
	switch trep := v.rep.(type) {
	case bool, uint64, int64, float64, complex128, string, *Type, enumIndex:
		return trep
	case []*Value:
		return copySliceOfValues(trep)
	case repBytes:
		return copyRepBytes(trep)
	case *byte:
		return uint64(*trep) // convert to standard uint64 representation
	case repMap:
		return copyRepMap(trep)
	case repFixedLen:
		return repFixedLen(copySliceOfValues(trep))
	case *Value:
		if trep != nil {
			return Copy(trep)
		}
		return (*Value)(nil)
	default:
		panic(fmt.Errorf("val: copyRep unhandled %v %T %v", v.t.kind, v.rep, v.rep))
	}
}

func copySliceOfValues(orig []*Value) []*Value {
	slice := make([]*Value, len(orig))
	for ix := 0; ix < len(orig); ix++ {
		slice[ix] = Copy(orig[ix])
	}
	return slice
}

func stringRep(t *Type, rep interface{}) string {
	switch trep := rep.(type) {
	case bool, uint64, int64, float64:
		return fmt.Sprint(trep)
	case complex128:
		return fmt.Sprintf("%v+%vi", real(trep), imag(trep))
	case string:
		return strconv.Quote(trep)
	case *Type:
		return trep.String()
	case enumIndex:
		return t.labels[int(trep)]
	case []*Value:
		s := "{"
		for index, elem := range trep {
			if index > 0 {
				s += ", "
			}
			s += stringRep(elem.t, elem.rep)
		}
		return s + "}"
	case repBytes:
		return strconv.Quote(string(trep))
	case *byte:
		return fmt.Sprint(*trep)
	case repMap:
		return trep.String()
	case repFixedLen:
		return trep.String(t)
	case *Value:
		if trep != nil {
			return trep.String() // include the type
		}
		return "nil"
	default:
		panic(fmt.Errorf("val: stringRep unhandled %v %T %v", t.kind, rep, rep))
	}
}

// BoolValue is a convenience to create a Bool value.
func BoolValue(x bool) *Value { return Zero(BoolType).AssignBool(x) }

// ByteValue is a convenience to create an Byte value.
func ByteValue(x byte) *Value { return Zero(ByteType).AssignByte(x) }

// Uint16Value is a convenience to create an Uint16 value.
func Uint16Value(x uint16) *Value { return Zero(Uint16Type).AssignUint(uint64(x)) }

// Uint32Value is a convenience to create an Uint32 value.
func Uint32Value(x uint32) *Value { return Zero(Uint32Type).AssignUint(uint64(x)) }

// Uint64Value is a convenience to create an Uint64 value.
func Uint64Value(x uint64) *Value { return Zero(Uint64Type).AssignUint(x) }

// Int16Value is a convenience to create an Int16 value.
func Int16Value(x int16) *Value { return Zero(Int16Type).AssignInt(int64(x)) }

// Int32Value is a convenience to create an Int32 value.
func Int32Value(x int32) *Value { return Zero(Int32Type).AssignInt(int64(x)) }

// Int64Value is a convenience to create an Int64 value.
func Int64Value(x int64) *Value { return Zero(Int64Type).AssignInt(x) }

// Float32Value is a convenience to create a Float32 value.
func Float32Value(x float32) *Value { return Zero(Float32Type).AssignFloat(float64(x)) }

// Float64Value is a convenience to create a Float64 value.
func Float64Value(x float64) *Value { return Zero(Float64Type).AssignFloat(x) }

// Complex64Value is a convenience to create a Complex64 value.
func Complex64Value(x complex64) *Value { return Zero(Complex64Type).AssignComplex(complex128(x)) }

// Complex128Value is a convenience to create a Complex128 value.
func Complex128Value(x complex128) *Value { return Zero(Complex128Type).AssignComplex(x) }

// StringValue is a convenience to create a String value.
func StringValue(x string) *Value { return Zero(StringType).AssignString(x) }

// BytesValue is a convenience to create a []byte value.  The bytes are copied.
func BytesValue(x []byte) *Value { return Zero(bytesType).AssignLen(len(x)).CopyBytes(x) }

// TypeValValue is a convenience to create a TypeVal value.
func TypeValValue(x *Type) *Value { return Zero(TypeValType).AssignTypeVal(x) }

// Zero returns a new Value containing the zero value for the given Type t.
func Zero(t *Type) *Value {
	if t == nil {
		return nil
	}
	return &Value{t, zeroRep(t)}
}

// Copy returns a copy of the Value v.
func Copy(v *Value) *Value {
	if v == nil {
		return nil
	}
	return &Value{v.t, copyRep(v)}
}

// Equal returns true iff a and b have the same type, and equal values.
func Equal(a, b *Value) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if a.t != b.t {
		return false
	}
	if a.t == ByteType {
		// ByteType has two representations, either uint64 or *byte.
		return a.Byte() == b.Byte()
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
	case complex128:
		return arep == b.rep.(complex128)
	case string:
		return arep == b.rep.(string)
	case *Type:
		return arep == b.rep.(*Type)
	case enumIndex:
		return arep == b.rep.(enumIndex)
	case []*Value:
		brep := b.rep.([]*Value)
		if len(arep) != len(brep) {
			return false
		}
		for index := range arep {
			if !Equal(arep[index], brep[index]) {
				return false
			}
		}
		return true
	case repBytes:
		return bytes.Equal(arep, b.rep.(repBytes))
	case repMap:
		return equalRepMap(a.t, arep, b.rep.(repMap))
	case repFixedLen:
		return equalRepFixedLen(arep, b.rep.(repFixedLen))
	case *Value:
		return Equal(arep, b.rep.(*Value))
	default:
		panic(fmt.Errorf("val: Equal unhandled %v %T %v", a.t.kind, arep, arep))
	}
}

// IsZero returns true iff v is the zero value for its type.
func (v *Value) IsZero() bool {
	return isZeroRep(v.t, v.rep)
}

// Kind returns the kind of type of v.
func (v *Value) Kind() Kind { return v.t.kind }

// Type returns the type of v.  All values have a non-nil type.
func (v *Value) Type() *Type { return v.t }

// Convert converts v to the target type t, and returns the resulting value.
//
// TODO(toddw): Define conversion rules and implement this function.
func (v *Value) Convert(t *Type) (*Value, error) {
	if v.t == t {
		return v, nil
	}
	return nil, errors.New("val: Value.Convert isn't implemented")
}

// Bool returns the underlying value of a Bool.
func (v *Value) Bool() bool {
	v.t.checkKind("Bool", Bool)
	return v.rep.(bool)
}

// Byte returns the underlying value of a Byte.
func (v *Value) Byte() byte {
	v.t.checkKind("Byte", Byte)
	switch trep := v.rep.(type) {
	case uint64:
		return byte(trep)
	case *byte:
		return *trep
	}
	panic(fmt.Errorf("val: Byte mismatched rep %v %T %v", v.t, v.rep, v.rep))
}

// Uint returns the underlying value of a Uint{16,32,64}.
func (v *Value) Uint() uint64 {
	v.t.checkKind("Uint", Uint16, Uint32, Uint64)
	return v.rep.(uint64)
}

// Int returns the underlying value of an Int{16,32,64}.
func (v *Value) Int() int64 {
	v.t.checkKind("Int", Int16, Int32, Int64)
	return v.rep.(int64)
}

// Float returns the underlying value of a Float{32,64}.
func (v *Value) Float() float64 {
	v.t.checkKind("Float", Float32, Float64)
	return v.rep.(float64)
}

// Complex returns the underlying value of a Complex{64,128}.
func (v *Value) Complex() complex128 {
	v.t.checkKind("Complex", Complex64, Complex128)
	return v.rep.(complex128)
}

// RawString returns the underlying value of a String.
func (v *Value) RawString() string {
	v.t.checkKind("RawString", String)
	return v.rep.(string)
}

// String returns a human-readable representation of the value.
// To retrieve the underlying value of a String, use RawString.
func (v *Value) String() string {
	switch v.t.Kind() {
	case Array, List, Set, Map, Struct:
		// { } are used instead of ( ) for composites, except for []byte and [N]byte
		if !v.t.IsBytes() {
			return v.t.String() + stringRep(v.t, v.rep)
		}
	case OneOf, Any:
		// Just show the inner type for oneof or any.
		return stringRep(v.t, v.rep)
	}
	return v.t.String() + "(" + stringRep(v.t, v.rep) + ")"
}

// Bytes returns the underlying value of a []byte or [N]byte.  Mutations of the
// returned value are reflected in the underlying value.
func (v *Value) Bytes() []byte {
	v.t.checkIsBytes("Bytes")
	return v.rep.(repBytes)
}

// TypeVal returns the underlying value of a TypeVal.
func (v *Value) TypeVal() *Type {
	v.t.checkKind("TypeVal", TypeVal)
	return v.rep.(*Type)
}

// Len returns the length of the underlying Array, List, Set or Map.
func (v *Value) Len() int {
	switch trep := v.rep.(type) {
	case []*Value:
		return len(trep)
	case repMap:
		return trep.Len()
	case repFixedLen:
		if v.t.kind == Array { // don't allow Struct
			return len(trep)
		}
	case repBytes:
		return len(trep)
	}
	panic(v.t.errKind("Len", Array, List, Set, Map))
}

// Index returns the i'th element of the underlying Array or List.  It panics if
// the index is out of range.
func (v *Value) Index(index int) *Value {
	switch trep := v.rep.(type) {
	case []*Value:
		return trep[index]
	case repFixedLen:
		if v.t.kind == Array { // don't allow Struct
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

// Keys returns all keys present in the underlying Set or Map.
func (v *Value) Keys() []*Value {
	v.t.checkKind("Keys", Set, Map)
	return v.rep.(repMap).Keys()
}

// ContainsKey returns true iff key is present in the underlying Set or Map.
func (v *Value) ContainsKey(key *Value) bool {
	v.t.checkKind("ContainsKey", Set, Map)
	_, ok := v.rep.(repMap).Index(v.t, key)
	return ok
}

// MapIndex returns the value associated with the key in the underlying Map, or
// nil if the key is not found in the map.  It panics if the key isn't
// assignable to the map's key type.
func (v *Value) MapIndex(key *Value) *Value {
	v.t.checkKind("MapIndex", Map)
	val, _ := v.rep.(repMap).Index(v.t, key)
	return val
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

// Field returns the Struct field at the given index.  It panics if the index is
// out of range.
func (v *Value) Field(index int) *Value {
	v.t.checkKind("Field", Struct)
	return v.rep.(repFixedLen).Index(v.t.fields[index].Type, index)
}

// Elem returns the element value contained in a Value of kind OneOf or Any.  It
// returns nil if the OneOf or Any is the zero value.
func (v *Value) Elem() *Value {
	v.t.checkKind("Elem", OneOf, Any)
	return v.rep.(*Value)
}

// Assign the value v to x.  If x is nil, v is set to its zero value.  It panics
// if the type of v is not assignable from the type of x.
//
// TODO(toddw): Restrict OneOf and Any to only allowing structs?
func (v *Value) Assign(x *Value) *Value {
	if x == nil {
		v.rep = zeroRep(v.t)
		return v
	}
	if !v.t.AssignableFrom(x.t) {
		panic(fmt.Errorf("val: value of type %q isn't assignable from value of type %q", v.t, x.t))
	}
	if v.t.kind == OneOf || v.t.kind == Any {
		if x.t.kind == OneOf || x.t.kind == Any {
			x = x.rep.(*Value) // get the concrete element value
		}
		v.rep = Copy(x)
		return v
	}
	v.rep = copyRep(x)
	return v
}

// AssignBool assigns the underlying Bool to x.
func (v *Value) AssignBool(x bool) *Value {
	v.t.checkKind("AssignBool", Bool)
	v.rep = x
	return v
}

// AssignByte assigns the underlying Byte to x.
func (v *Value) AssignByte(x byte) *Value {
	v.t.checkKind("AssignByte", Byte)
	switch trep := v.rep.(type) {
	case uint64:
		v.rep = uint64(x)
	case *byte:
		*trep = x
	default:
		panic(fmt.Errorf("val: AssignByte mismatched rep %v %T %v", v.t, v.rep, v.rep))
	}
	return v
}

// AssignUint assigns the underlying Uint{16,32,64} to x.
func (v *Value) AssignUint(x uint64) *Value {
	v.t.checkKind("AssignUint", Uint16, Uint32, Uint64)
	v.rep = x
	return v
}

// AssignInt assigns the underlying Int{16,32,64} to x.
func (v *Value) AssignInt(x int64) *Value {
	v.t.checkKind("AssignInt", Int16, Int32, Int64)
	v.rep = x
	return v
}

// AssignFloat assigns the underlying Float{32,64} to x.
func (v *Value) AssignFloat(x float64) *Value {
	v.t.checkKind("AssignFloat", Float32, Float64)
	v.rep = x
	return v
}

// AssignComplex assigns the underlying Complex{64,128} to x.
func (v *Value) AssignComplex(x complex128) *Value {
	v.t.checkKind("AssignComplex", Complex64, Complex128)
	v.rep = x
	return v
}

// AssignString assigns the underlying String to x.
func (v *Value) AssignString(x string) *Value {
	v.t.checkKind("AssignString", String)
	v.rep = x
	return v
}

// CopyBytes assigns the underlying []byte or [N]byte to a copy of x.  Copy
// semantics are the same as the built-in copy function; we copy min(v.Len(),
// len(x)) bytes from x to v.
func (v *Value) CopyBytes(x []byte) *Value {
	v.t.checkIsBytes("CopyBytes")
	copy(v.rep.(repBytes), x)
	return v
}

// AssignTypeVal assigns the underlying TypeVal to x.  If x == nil we assign the
// zero TypeVal.
func (v *Value) AssignTypeVal(x *Type) *Value {
	v.t.checkKind("AssignTypeVal", TypeVal)
	if x == nil {
		x = zeroTypeVal
	}
	v.rep = x
	return v
}

// AssignEnumIndex assigns the underlying Enum to the label corresponding to
// index.  It panics if the index is out of range.
func (v *Value) AssignEnumIndex(index int) *Value {
	v.t.checkKind("AssignEnumIndex", Enum)
	if index < 0 || index >= len(v.t.labels) {
		panic(fmt.Errorf("val: enum %q index %d out of range", v.t.name, index))
	}
	v.rep = enumIndex(index)
	return v
}

// AssignEnumLabel assigns the underlying Enum to the label.  It panics if the
// label doesn't exist in the Enum.
func (v *Value) AssignEnumLabel(label string) *Value {
	v.t.checkKind("AssignEnumLabel", Enum)
	index := v.t.EnumIndex(label)
	if index == -1 {
		panic(fmt.Errorf("val: enum %q doesn't have label %q", v.t.name, label))
	}
	v.rep = enumIndex(index)
	return v
}

// AssignLen assigns the length of the underlying List to n.  Unlike Go slices,
// Lists do not have a separate notion of capacity.
func (v *Value) AssignLen(n int) *Value {
	v.t.checkKind("AssignLen", List)
	if oldrep, ok := v.rep.(repBytes); ok {
		// TODO(toddw): Speed up the loops below that zero each byte.
		var newrep repBytes
		if n <= cap(oldrep) {
			newrep = oldrep[:n]
			for ix := len(oldrep); ix < n; ix++ {
				newrep[ix] = 0
			}
		} else {
			newrep = make(repBytes, n)
			for ix := copy(newrep, oldrep); ix < n; ix++ {
				newrep[ix] = 0
			}
		}
		v.rep = newrep
		return v
	}
	oldrep := v.rep.([]*Value)
	var newrep []*Value
	if n <= cap(oldrep) {
		newrep = oldrep[:n]
		for ix := len(oldrep); ix < n; ix++ {
			newrep[ix] = Zero(v.t.elem)
		}
	} else {
		newrep = make([]*Value, n)
		for ix := copy(newrep, oldrep); ix < n; ix++ {
			newrep[ix] = Zero(v.t.elem)
		}
	}
	v.rep = newrep
	return v
}

// AssignSetKey assigns key to the underlying Set.  It panics if the key isn't
// assignable to the set's key type.
func (v *Value) AssignSetKey(key *Value) *Value {
	v.t.checkKind("AssignSetKey", Set)
	v.rep.(repMap).Assign(v.t, key, nil)
	return v
}

// DeleteSetKey deletes key from the underlying Set.
func (v *Value) DeleteSetKey(key *Value) *Value {
	v.t.checkKind("DeleteSetKey", Set)
	v.rep.(repMap).Delete(v.t, key)
	return v
}

// AssignMapIndex assigns the value associated with key to val in the underlying
// Map.  If val is nil, AssignMapIndex deletes the key from the Map.  It panics
// if the key isn't assignable to the map's key type, and ditto for val.
func (v *Value) AssignMapIndex(key, val *Value) *Value {
	v.t.checkKind("AssignMapIndex", Map)
	if val == nil {
		v.rep.(repMap).Delete(v.t, key)
	} else {
		v.rep.(repMap).Assign(v.t, key, val)
	}
	return v
}
