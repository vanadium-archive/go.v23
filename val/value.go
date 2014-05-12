package val

import (
	"bytes"
	"fmt"
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

var (
	// the zero TypeVal returns the any type
	zeroTypeVal = AnyType
)

// enumIndex represents an enum value by the index of its label.
type enumIndex int

// zeroRep returns the zero representation for each kind of type.
func zeroRep(t *Type) interface{} {
	switch t.kind {
	case Bool:
		return false
	case Int32, Int64:
		return int64(0)
	case Uint32, Uint64:
		return uint64(0)
	case Float32, Float64:
		return float64(0)
	case Complex64, Complex128:
		return complex128(0)
	case String:
		return ""
	case Bytes:
		return make([]byte, 0)
	case TypeVal:
		return zeroTypeVal
	case Enum:
		return enumIndex(0)
	case List:
		return make([]*Value, 0)
	case Map:
		return zeroRepMap(t)
	case Struct:
		return zeroRepStruct(t)
	case OneOf, Any:
		// The nil value represents non-existence.
		return (*Value)(nil)
	default:
		panic(fmt.Errorf("val: unhandled kind: %v", t.kind))
	}
}

func isZeroRep(k Kind, rep interface{}) bool {
	switch k {
	case Bool:
		return !rep.(bool)
	case Int32, Int64:
		return rep.(int64) == 0
	case Uint32, Uint64:
		return rep.(uint64) == 0
	case Float32, Float64:
		return rep.(float64) == 0
	case Complex64, Complex128:
		return rep.(complex128) == 0
	case String:
		return len(rep.(string)) == 0
	case Bytes:
		return len(rep.([]byte)) == 0
	case TypeVal:
		return rep.(*Type) == zeroTypeVal
	case Enum:
		return rep.(enumIndex) == enumIndex(0)
	case List:
		return len(rep.([]*Value)) == 0
	case Map:
		return rep.(repMap).Len() == 0
	case Struct:
		return rep.(repStruct).IsZero()
	case OneOf, Any:
		return rep.(*Value) == nil
	default:
		panic(fmt.Errorf("val: unhandled kind: %v", k))
	}
}

// copyRep returns a copy of v.rep.
func copyRep(v *Value) interface{} {
	switch v.t.kind {
	case Bool, Int32, Int64, Uint32, Uint64, Float32, Float64, Complex64, Complex128, String, TypeVal, Enum:
		return v.rep
	case Bytes:
		return copyBytes(v.rep.([]byte))
	case List:
		return copySliceOfValues(v.rep.([]*Value))
	case Map:
		return copyRepMap(v.rep.(repMap))
	case Struct:
		return copyRepStruct(v.rep.(repStruct))
	case OneOf, Any:
		if orig := v.rep.(*Value); orig != nil {
			return Copy(orig)
		}
		return (*Value)(nil)
	default:
		panic(fmt.Errorf("val: unhandled kind: %v", v.t.kind))
	}
}

func copyBytes(orig []byte) []byte {
	bytes := make([]byte, len(orig))
	copy(bytes, orig)
	return bytes
}

func copySliceOfValues(orig []*Value) []*Value {
	slice := make([]*Value, len(orig))
	for ix := 0; ix < len(orig); ix++ {
		slice[ix] = Copy(orig[ix])
	}
	return slice
}

// TODO(toddw): Perhaps we should use the JSON format instead?
func stringRep(t *Type, rep interface{}, quotes bool) string {
	switch t.kind {
	case Bool, Int32, Int64, Uint32, Uint64, Float32, Float64, Complex64, Complex128:
		return fmt.Sprint(rep)
	case String:
		if quotes {
			return `"` + rep.(string) + `"`
		}
		return rep.(string)
	case Bytes:
		if quotes {
			return `"` + string(rep.([]byte)) + `"`
		}
		return string(rep.([]byte))
	case TypeVal:
		return rep.(*Type).String()
	case Enum:
		return t.labels[int(rep.(enumIndex))]
	case List:
		s := "["
		for index, elem := range rep.([]*Value) {
			if index > 0 {
				s += ", "
			}
			s += stringRep(elem.t, elem.rep, true)
		}
		return s + "]"
	case Map:
		return rep.(repMap).String()
	case Struct:
		return rep.(repStruct).String(t)
	case OneOf, Any:
		if elem := rep.(*Value); elem != nil {
			return stringRep(elem.t, elem.rep, quotes)
		}
		return "nil"
	default:
		panic(fmt.Errorf("val: unhandled kind: %v", t.kind))
	}
}

// BoolValue is a convenience to create a Bool value.
func BoolValue(x bool) *Value { return Zero(BoolType).AssignBool(x) }

// Int32Value is a convenience to create an Int32 value.
func Int32Value(x int32) *Value { return Zero(Int32Type).AssignInt(int64(x)) }

// Int64Value is a convenience to create an Int64 value.
func Int64Value(x int64) *Value { return Zero(Int64Type).AssignInt(x) }

// Uint32Value is a convenience to create an Uint32 value.
func Uint32Value(x uint32) *Value { return Zero(Uint32Type).AssignUint(uint64(x)) }

// Uint64Value is a convenience to create an Uint64 value.
func Uint64Value(x uint64) *Value { return Zero(Uint64Type).AssignUint(x) }

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

// BytesValue is a convenience to create a Bytes value.
func BytesValue(x []byte) *Value { return Zero(BytesType).CopyBytes(x) }

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
	switch a.t.kind {
	case Bool:
		return a.rep.(bool) == b.rep.(bool)
	case Int32, Int64:
		return a.rep.(int64) == b.rep.(int64)
	case Uint32, Uint64:
		return a.rep.(uint64) == b.rep.(uint64)
	case Float32, Float64:
		return a.rep.(float64) == b.rep.(float64)
	case Complex64, Complex128:
		return a.rep.(complex128) == b.rep.(complex128)
	case String:
		return a.rep.(string) == b.rep.(string)
	case Bytes:
		return bytes.Equal(a.rep.([]byte), b.rep.([]byte))
	case TypeVal:
		return a.rep.(*Type) == b.rep.(*Type)
	case Enum:
		return a.rep.(enumIndex) == b.rep.(enumIndex)
	case List:
		alist, blist := a.rep.([]*Value), b.rep.([]*Value)
		if len(alist) != len(blist) {
			return false
		}
		for index := 0; index < len(alist); index++ {
			if !Equal(alist[index], blist[index]) {
				return false
			}
		}
		return true
	case Map:
		return equalRepMap(a.t, a.rep.(repMap), b.rep.(repMap))
	case Struct:
		return equalRepStruct(a.rep.(repStruct), b.rep.(repStruct))
	case OneOf, Any:
		return Equal(a.rep.(*Value), b.rep.(*Value))
	default:
		panic(fmt.Errorf("val: unhandled kind: %v", a.t.kind))
	}
}

// IsZero returns true iff v is the zero value for its type.
func (v *Value) IsZero() bool {
	return isZeroRep(v.t.kind, v.rep)
}

// Kind returns the kind of type of this value.
func (v *Value) Kind() Kind { return v.t.kind }

// Type returns the type of this value.  All values have a non-nil type.
func (v *Value) Type() *Type { return v.t }

// TODO(toddw): Should we support Convert?  Should it implement all vom
// compatibility rules?
//   func (v *Value) Convert(t *Type) *Value

// Bool returns the underlying value of a Bool.
func (v *Value) Bool() bool {
	v.t.checkKind("Bool", Bool)
	return v.rep.(bool)
}

// Int returns the underlying value of an Int{32,64}.
func (v *Value) Int() int64 {
	v.t.checkKind("Int", Int32, Int64)
	return v.rep.(int64)
}

// Uint returns the underlying value of a Uint{32,64}.
func (v *Value) Uint() uint64 {
	v.t.checkKind("Uint", Uint32, Uint64)
	return v.rep.(uint64)
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

// String returns the underlying value of a String, and a human-readable
// representation for values of other kinds of types.
func (v *Value) String() string {
	return stringRep(v.t, v.rep, false)
}

// Bytes returns the underlying value of a Bytes.
func (v *Value) Bytes() []byte {
	v.t.checkKind("Bytes", Bytes)
	return v.rep.([]byte)
}

// TypeVal returns the underlying value of a TypeVal.
func (v *Value) TypeVal() *Type {
	v.t.checkKind("TypeVal", TypeVal)
	return v.rep.(*Type)
}

// Len returns the length of the underlying List or Map.
func (v *Value) Len() int {
	switch v.t.kind {
	case List:
		return len(v.rep.([]*Value))
	case Map:
		return v.rep.(repMap).Len()
	}
	panic(errKindMismatch("Len", v.t.kind, List, Map))
}

// Index returns the i'th element of the underlying List.  It panics if the
// index is out of range.
func (v *Value) Index(index int) *Value {
	v.t.checkKind("Index", List)
	return v.rep.([]*Value)[index]
}

// MapKeys returns a slice containing all keys present in the underlying Map.
func (v *Value) MapKeys() []*Value {
	v.t.checkKind("MapKeys", Map)
	return v.rep.(repMap).Keys()
}

// MapIndex returns the value associated with the key in the underlying Map, or
// nil if the key is not found in the map.  It panics if the key isn't
// assignable to the map's key type.
func (v *Value) MapIndex(key *Value) *Value {
	v.t.checkKind("MapIndex", Map)
	return v.rep.(repMap).Index(v.t, key)
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
	return v.rep.(repStruct).Field(v.t, index)
}

// Elem returns the element value contained in a Value of kind OneOf or Any.  It
// returns nil if the OneOf or Any is the zero value.
func (v *Value) Elem() *Value {
	v.t.checkKind("Elem", OneOf, Any)
	return v.rep.(*Value)
}

// Assign sets the value v to x.  If x is nil, v is set to its zero value.
// It panics if the type of v is not assignable from the type of x.
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

// AssignBool sets the underlying Bool to x.
func (v *Value) AssignBool(x bool) *Value {
	v.t.checkKind("AssignBool", Bool)
	v.rep = x
	return v
}

// AssignInt sets the underlying Int{32,64} to x.
func (v *Value) AssignInt(x int64) *Value {
	v.t.checkKind("AssignInt", Int32, Int64)
	v.rep = x
	return v
}

// AssignUint sets the underlying Uint{32,64} to x.
func (v *Value) AssignUint(x uint64) *Value {
	v.t.checkKind("AssignUint", Uint32, Uint64)
	v.rep = x
	return v
}

// AssignFloat sets the underlying Float{32,64} to x.
func (v *Value) AssignFloat(x float64) *Value {
	v.t.checkKind("AssignFloat", Float32, Float64)
	v.rep = x
	return v
}

// AssignComplex sets the underlying Complex{32,64} to x.
func (v *Value) AssignComplex(x complex128) *Value {
	v.t.checkKind("AssignComplex", Complex64, Complex128)
	v.rep = x
	return v
}

// AssignString sets the underlying String to x.
func (v *Value) AssignString(x string) *Value {
	v.t.checkKind("AssignString", String)
	v.rep = x
	return v
}

// AssignBytes sets the underlying Bytes to x.  No copy is made.
func (v *Value) AssignBytes(x []byte) *Value {
	v.t.checkKind("AssignBytes", Bytes)
	if x == nil {
		v.rep = []byte{}
	}
	v.rep = x
	return v
}

// AssignTypeVal sets the underlying TypeVal to x.  If x == nil we assign the
// zero TypeVal.
func (v *Value) AssignTypeVal(x *Type) *Value {
	v.t.checkKind("AssignTypeVal", TypeVal)
	if x == nil {
		x = zeroTypeVal
	}
	v.rep = x
	return v
}

// CopyBytes sets the underlying Bytes to a copy of x.
func (v *Value) CopyBytes(x []byte) *Value {
	v.t.checkKind("CopyBytes", Bytes)
	v.rep = copyBytes(x)
	return v
}

// AssignEnumIndex sets the underlying Enum to the label corresponding to index.
// It panics if the index is out of range.
func (v *Value) AssignEnumIndex(index int) *Value {
	v.t.checkKind("AssignEnumIndex", Enum)
	if index < 0 || index >= len(v.t.labels) {
		panic(fmt.Errorf("val: enum %q index %d out of range", v.t.name, index))
	}
	v.rep = enumIndex(index)
	return v
}

// AssignEnumLabel sets the underlying Enum to the label.  It panics if the
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

// AssignLen sets the length of the underlying List to n.  Unlike Go slices,
// Lists do not have a separate notion of capacity.
func (v *Value) AssignLen(n int) *Value {
	v.t.checkKind("AssignLen", List)
	orig := v.rep.([]*Value)
	var list []*Value
	if n <= cap(orig) {
		list = orig[:n]
		for ix := len(orig); ix < n; ix++ {
			list[ix].rep = zeroRep(v.t.elem)
		}
	} else {
		list = make([]*Value, n)
		for ix := copy(list, orig); ix < n; ix++ {
			list[ix] = Zero(v.t.elem)
		}
	}
	v.rep = list
	return v
}

// AssignMapIndex sets the value associated with key to val in the underlying
// Map.  If val is nil, AssignMapIndex deletes the key from the Map.  It panics
// if the key isn't assignable to the map's key type, and ditto for val.
func (v *Value) AssignMapIndex(key, val *Value) *Value {
	v.t.checkKind("AssignMapIndex", Map)
	v.rep.(repMap).Assign(v.t, key, val)
	return v
}
