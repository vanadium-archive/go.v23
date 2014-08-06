package vdl

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"
)

var (
	keyType = StructType([]StructField{{"I", Int64Type}, {"S", StringType}}...)

	strA, strB, strC = StringValue("A"), StringValue("B"), StringValue("C")
	int1, int2       = Int64Value(1), Int64Value(2)
	key1, key2, key3 = makeKey(1, "A"), makeKey(2, "B"), makeKey(3, "C")
)

func makeKey(a int64, b string) *Value {
	key := ZeroValue(keyType)
	key.Field(0).AssignInt(a)
	key.Field(1).AssignString(b)
	return key
}

func TestValueInvalid(t *testing.T) {
	tests := []*Value{nil, new(Value)}
	for ix, test := range tests {
		if test.IsValid() {
			t.Errorf(`%d IsValid()`, ix)
		}
		if got, want := test.String(), "INVALID"; got != want {
			t.Errorf(`%d String() got %v, want %v`, ix, got, want)
		}
	}
}

func TestValue(t *testing.T) {
	tests := []struct {
		k Kind
		t *Type
		s string
	}{
		{Bool, BoolType, "bool(false)"},
		{Byte, ByteType, "byte(0)"},
		{Uint16, Uint16Type, "uint16(0)"},
		{Uint32, Uint32Type, "uint32(0)"},
		{Uint64, Uint64Type, "uint64(0)"},
		{Int16, Int16Type, "int16(0)"},
		{Int32, Int32Type, "int32(0)"},
		{Int64, Int64Type, "int64(0)"},
		{Float32, Float32Type, "float32(0)"},
		{Float64, Float64Type, "float64(0)"},
		{Complex64, Complex64Type, "complex64(0+0i)"},
		{Complex128, Complex128Type, "complex128(0+0i)"},
		{String, StringType, `string("")`},
		{List, ListType(ByteType), `[]byte("")`},
		{Array, ArrayType(3, ByteType), `[3]byte("\x00\x00\x00")`},
		{Enum, EnumType("A", "B", "C"), "enum{A;B;C}(A)"},
		{TypeVal, TypeValType, "typeval(any)"},
		{Array, ArrayType(2, StringType), `[2]string{"", ""}`},
		{List, ListType(StringType), "[]string{}"},
		{Set, SetType(StringType), "set[string]{}"},
		{Set, SetType(keyType), "set[struct{I int64;S string}]{}"},
		{Map, MapType(StringType, Int64Type), "map[string]int64{}"},
		{Map, MapType(keyType, Int64Type), "map[struct{I int64;S string}]int64{}"},
		{Struct, StructType([]StructField{{"A", Int64Type}, {"B", StringType}, {"C", BoolType}}...), `struct{A int64;B string;C bool}{A: 0, B: "", C: false}`},
		{OneOf, OneOfType(Int64Type, StringType), "oneof{int64;string}(int64(0))"},
		{Any, AnyType, "any(nil)"},
	}
	for _, test := range tests {
		x := ZeroValue(test.t)
		if !x.IsValid() {
			t.Errorf(`!ZeroValue(%s).IsValid`, test.k)
		}
		if !x.IsZero() {
			t.Errorf(`ZeroValue(%s) isn't zero`, test.k)
		}
		if test.k == Any && !x.IsNil() {
			t.Errorf(`ZeroValue(Any) isn't nil`)
		}
		if got, want := x.Kind(), test.k; got != want {
			t.Errorf(`ZeroValue(%s) got kind %v, want %v`, test.k, got, want)
		}
		if got, want := x.Type(), test.t; got != want {
			t.Errorf(`ZeroValue(%s) got type %v, want %v`, test.k, got, want)
		}
		if got, want := x.String(), test.s; got != want {
			t.Errorf(`ZeroValue(%s) got string %q, want %q`, test.k, got, want)
		}
		y := CopyValue(x)
		if !y.IsValid() {
			t.Errorf(`!CopyValue(ZeroValue(%s)).IsValid`, test.k)
		}
		if !y.IsZero() {
			t.Errorf(`ZeroValue(%s) of copy isn't zero, y: %v`, test.k, y)
		}
		if test.k == Any && !y.IsNil() {
			t.Errorf(`ZeroValue(Any) of copy isn't nil`)
		}
		if !EqualValue(x, y) {
			t.Errorf(`ZeroValue(%s) !Equal after copy, x: %v, y: %v`, test.k, x, y)
		}

		// Invariant here: x == y == 0
		// The assign[KIND] functions assign a nonzero value.
		assignBool(t, x)
		assignByte(t, x)
		assignUint(t, x)
		assignInt(t, x)
		assignFloat(t, x)
		assignComplex(t, x)
		assignString(t, x)
		assignEnum(t, x)
		assignTypeVal(t, x)
		assignArray(t, x)
		assignList(t, x)
		assignSet(t, x)
		assignMap(t, x)
		assignStruct(t, x)
		assignOneOf(t, x)
		assignAny(t, x)

		// Invariant here: x != 0 && y == 0
		if EqualValue(x, y) {
			t.Errorf(`ZeroValue(%s) Equal after assign, x: %v, y: %v`, test.k, x, y)
		}
		z := CopyValue(x) // x == z && x != 0 && y == 0
		if !EqualValue(x, z) {
			t.Errorf(`ZeroValue(%s) !Equal after copy, x: %v, z: %v`, test.k, x, z)
		}
		if EqualValue(y, z) {
			t.Errorf(`ZeroValue(%s) Equal after copy, y: %v, z: %v`, test.k, y, z)
		}

		z.Assign(nil) // x != 0 && y == z == 0
		if !z.IsValid() {
			t.Errorf(`%s z after Assign(nil) isn't valid`, test.k)
		}
		if !z.IsZero() {
			t.Errorf(`%s z after Assign(nil) isn't zero`, test.k)
		}
		if test.k == Any && !z.IsNil() {
			t.Errorf(`Any z after Assign(nil) isn't nil`)
		}
		if EqualValue(x, z) {
			t.Errorf(`%s Equal after Assign(nil), x: %v, z: %v`, test.k, x, z)
		}
		if !EqualValue(y, z) {
			t.Errorf(`%s !Equal after Assign(nil), y: %v, z: %v`, test.k, y, z)
		}

		y.Assign(x) // x == y && x != 0 && z == 0
		if !y.IsValid() {
			t.Errorf(`%s y after Assign(x) isn't valid`, test.k)
		}
		if y.IsZero() {
			t.Errorf(`%s y after Assign(x) is zero, y: %v`, test.k, y)
		}
		if y.IsNil() {
			t.Errorf(`%s y after Assign(x) is nil, y: %v`, test.k, y)
		}
		if !EqualValue(x, y) {
			t.Errorf(`%s Equal after Assign(x), x: %v, y: %v`, test.k, x, y)
		}
		if EqualValue(y, z) {
			t.Errorf(`%s Equal after Assign(x), y: %v, z: %v`, test.k, y, z)
		}

		// Test nilable types
		if test.k == Any {
			continue
		}
		ntype := NilableType(test.t)

		// nx == nil
		nx := ZeroValue(ntype)
		if !nx.IsValid() {
			t.Errorf(`!ZeroValue(?%s).IsValid`, test.k)
		}
		if !nx.IsZero() {
			t.Errorf(`ZeroValue(?%s) isn't zero`, test.k)
		}
		if !nx.IsNil() {
			t.Errorf(`ZeroValue(?%s) isn't nil`, test.k)
		}
		if got, want := nx.Kind(), Nilable; got != want {
			t.Errorf(`ZeroValue(?%s) got kind %v, want %v`, test.k, got, want)
		}
		if got, want := nx.Type(), ntype; got != want {
			t.Errorf(`ZeroValue(?%s) got type %v, want %v`, test.k, got, want)
		}
		if got, want := nx.String(), ntype.String()+"(nil)"; got != want {
			t.Errorf(`ZeroValue(?%s) got string %q, want %q`, test.k, got, want)
		}
		// ny != nil
		ny := NonNilZeroValue(ntype)
		if !ny.IsValid() {
			t.Errorf(`!NonNilZeroValue(?%s).IsValid`, test.k)
		}
		if ny.IsZero() {
			t.Errorf(`NonNilZeroValue(?%s) is zero`, test.k)
		}
		if ny.IsNil() {
			t.Errorf(`NonNilZeroValue(?%s) is nil`, test.k)
		}
		if got, want := ny.Kind(), Nilable; got != want {
			t.Errorf(`NonNilZeroValue(?%s) got kind %v, want %v`, test.k, got, want)
		}
		if got, want := ny.Type(), ntype; got != want {
			t.Errorf(`NonNilZeroValue(?%s) got type %v, want %v`, test.k, got, want)
		}
	}
}

// Each of the below assign{KIND} functions assigns a nonzero value to x for
// matching kinds, otherwise it expects a mismatched-kind panic when trying the
// kind-specific methods.  The point is to ensure we've tried all combinations
// of kinds and methods.

func assignBool(t *testing.T, x *Value) {
	newval, newstr := true, "bool(true)"
	if x.Kind() == Bool {
		if got, want := x.Bool(), false; got != want {
			t.Errorf(`Bool zero value got %v, want %v`, got, want)
		}
		x.AssignBool(newval)
		if got, want := x.Bool(), newval; got != want {
			t.Errorf(`Bool assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), newstr; got != want {
			t.Errorf(`Bool string got %v, want %v`, got, want)
		}
	} else {
		expectMismatchedKind(t, func() { x.Bool() })
		expectMismatchedKind(t, func() { x.AssignBool(newval) })
	}
}

func assignByte(t *testing.T, x *Value) {
	newval := byte(123)
	switch x.Kind() {
	case Byte:
		if got, want := x.Byte(), byte(0); got != want {
			t.Errorf(`Byte value got %v, want %v`, got, want)
		}
		x.AssignByte(newval)
		if got, want := x.Byte(), newval; got != want {
			t.Errorf(`Byte assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), x.Kind().String()+"(123)"; got != want {
			t.Errorf(`Byte string got %v, want %v`, got, want)
		}
	default:
		expectMismatchedKind(t, func() { x.Byte() })
		expectMismatchedKind(t, func() { x.AssignByte(newval) })
	}
}

func assignUint(t *testing.T, x *Value) {
	newval := uint64(123)
	switch x.Kind() {
	case Uint16, Uint32, Uint64:
		if got, want := x.Uint(), uint64(0); got != want {
			t.Errorf(`Uint zero value got %v, want %v`, got, want)
		}
		x.AssignUint(newval)
		if got, want := x.Uint(), newval; got != want {
			t.Errorf(`Uint assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), x.Kind().String()+"(123)"; got != want {
			t.Errorf(`Uint string got %v, want %v`, got, want)
		}
	default:
		expectMismatchedKind(t, func() { x.Uint() })
		expectMismatchedKind(t, func() { x.AssignUint(newval) })
	}
}

func assignInt(t *testing.T, x *Value) {
	newval := int64(123)
	switch x.Kind() {
	case Int16, Int32, Int64:
		if got, want := x.Int(), int64(0); got != want {
			t.Errorf(`Int zero value got %v, want %v`, got, want)
		}
		x.AssignInt(newval)
		if got, want := x.Int(), newval; got != want {
			t.Errorf(`Int assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), x.Kind().String()+"(123)"; got != want {
			t.Errorf(`Int string got %v, want %v`, got, want)
		}
	default:
		expectMismatchedKind(t, func() { x.Int() })
		expectMismatchedKind(t, func() { x.AssignInt(newval) })
	}
}

func assignFloat(t *testing.T, x *Value) {
	newval := float64(1.23)
	switch x.Kind() {
	case Float32, Float64:
		if got, want := x.Float(), float64(0); got != want {
			t.Errorf(`Float zero value got %v, want %v`, got, want)
		}
		x.AssignFloat(newval)
		if got, want := x.Float(), newval; got != want {
			t.Errorf(`Float assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), x.Kind().String()+"(1.23)"; got != want {
			t.Errorf(`Float string got %v, want %v`, got, want)
		}
	default:
		expectMismatchedKind(t, func() { x.Float() })
		expectMismatchedKind(t, func() { x.AssignFloat(newval) })
	}
}

func assignComplex(t *testing.T, x *Value) {
	newval := complex128(1 + 2i)
	switch x.Kind() {
	case Complex64, Complex128:
		if got, want := x.Complex(), complex128(0); got != want {
			t.Errorf(`Complex zero value got %v, want %v`, got, want)
		}
		x.AssignComplex(newval)
		if got, want := x.Complex(), newval; got != want {
			t.Errorf(`Complex assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), x.Kind().String()+"(1+2i)"; got != want {
			t.Errorf(`Complex string got %v, want %v`, got, want)
		}
	default:
		expectMismatchedKind(t, func() { x.Complex() })
		expectMismatchedKind(t, func() { x.AssignComplex(newval) })
	}
}

func assignString(t *testing.T, x *Value) {
	zerostr, newval, newstr := `string("")`, "abc", `string("abc")`
	if x.Kind() == String {
		if got, want := x.String(), zerostr; got != want {
			t.Errorf(`String zero value got %v, want %v`, got, want)
		}
		x.AssignString(newval)
		if got, want := x.RawString(), newval; got != want {
			t.Errorf(`String assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), newstr; got != want {
			t.Errorf(`String assign string rep got %v, want %v`, got, want)
		}
	} else {
		expectMismatchedKind(t, func() { x.RawString() })
		expectMismatchedKind(t, func() { x.AssignString(newval) })
	}
}

func assignEnum(t *testing.T, x *Value) {
	if x.Kind() == Enum {
		if gi, gl, wi, wl := x.EnumIndex(), x.EnumLabel(), 0, "A"; gi != wi || gl != wl {
			t.Errorf(`Enum zero value got [%d]%v, want [%d]%v`, gi, gl, wi, wl)
		}
		x.AssignEnumIndex(1)
		if gi, gl, wi, wl := x.EnumIndex(), x.EnumLabel(), 1, "B"; gi != wi || gl != wl {
			t.Errorf(`Enum assign index value got [%d]%v, want [%d]%v`, gi, gl, wi, wl)
		}
		if got, want := x.String(), "enum{A;B;C}(B)"; got != want {
			t.Errorf(`Enum string got %v, want %v`, got, want)
		}
		x.AssignEnumIndex(2)
		if gi, gl, wi, wl := x.EnumIndex(), x.EnumLabel(), 2, "C"; gi != wi || gl != wl {
			t.Errorf(`Enum assign label value got [%d]%v, want [%d]%v`, gi, gl, wi, wl)
		}
		if got, want := x.String(), "enum{A;B;C}(C)"; got != want {
			t.Errorf(`Enum string got %v, want %v`, got, want)
		}
	} else {
		expectMismatchedKind(t, func() { x.EnumIndex() })
		expectMismatchedKind(t, func() { x.EnumLabel() })
		expectMismatchedKind(t, func() { x.AssignEnumIndex(0) })
		expectMismatchedKind(t, func() { x.AssignEnumLabel("A") })
	}
}

func assignTypeVal(t *testing.T, x *Value) {
	newval, newstr := BoolType, "typeval(bool)"
	if x.Kind() == TypeVal {
		if got, want := x.TypeVal(), zeroTypeVal; got != want {
			t.Errorf(`TypeVal zero value got %v, want %v`, got, want)
		}
		x.AssignTypeVal(newval)
		if got, want := x.TypeVal(), newval; got != want {
			t.Errorf(`TypeVal assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), newstr; got != want {
			t.Errorf(`TypeVal string got %v, want %v`, got, want)
		}
		x.AssignTypeVal(nil) // assigning nil sets the zero typeval
		if got, want := x.TypeVal(), zeroTypeVal; got != want {
			t.Errorf(`TypeVal assign value got %v, want %v`, got, want)
		}
		x.AssignTypeVal(newval)
	} else {
		expectMismatchedKind(t, func() { x.TypeVal() })
		expectMismatchedKind(t, func() { x.AssignTypeVal(newval) })
	}
}

func assignArray(t *testing.T, x *Value) {
	if x.Kind() == Array {
		if x.Type().IsBytes() {
			assignBytes(t, x)
			return
		}
		if got, want := x.Len(), 2; got != want {
			t.Errorf(`Array zero len got %v, want %v`, got, want)
		}
		if g0, g1, w0, w1 := x.Index(0).String(), x.Index(1).String(), `string("")`, `string("")`; g0 != w0 || g1 != w1 {
			t.Errorf(`Array assign values got %v %v, want %v %v`, g0, g1, w0, w1)
		}
		x.Index(0).AssignString("A")
		x.Index(1).AssignString("B")
		if g0, g1, w0, w1 := x.Index(0).String(), x.Index(1).String(), `string("A")`, `string("B")`; g0 != w0 || g1 != w1 {
			t.Errorf(`Array assign values got %v %v, want %v %v`, g0, g1, w0, w1)
		}
		if got, want := x.String(), `[2]string{"A", "B"}`; got != want {
			t.Errorf(`Array string got %v, want %v`, got, want)
		}
	} else {
		if x.Kind() != List {
			// Index is allowed for Array and List
			expectMismatchedKind(t, func() { x.Index(0) })
		}
		if x.Kind() != List && x.Kind() != Set && x.Kind() != Map {
			// Len is allowed for Array, List, Set and Map
			expectMismatchedKind(t, func() { x.Len() })
		}
	}
}

func assignList(t *testing.T, x *Value) {
	if x.Kind() == List {
		if x.Type().IsBytes() {
			assignBytes(t, x)
			return
		}
		if got, want := x.Len(), 0; got != want {
			t.Errorf(`List zero len got %v, want %v`, got, want)
		}
		x.AssignLen(2)
		if got, want := x.Len(), 2; got != want {
			t.Errorf(`List assign len got %v, want %v`, got, want)
		}
		if g0, g1, w0, w1 := x.Index(0).String(), x.Index(1).String(), `string("")`, `string("")`; g0 != w0 || g1 != w1 {
			t.Errorf(`List assign values got %v %v, want %v %v`, g0, g1, w0, w1)
		}
		if got, want := x.String(), `[]string{"", ""}`; got != want {
			t.Errorf(`List string got %v, want %v`, got, want)
		}
		x.Index(0).AssignString("A")
		x.Index(1).AssignString("B")
		if g0, g1, w0, w1 := x.Index(0).String(), x.Index(1).String(), `string("A")`, `string("B")`; g0 != w0 || g1 != w1 {
			t.Errorf(`List assign values got %v %v, want %v %v`, g0, g1, w0, w1)
		}
		if got, want := x.String(), `[]string{"A", "B"}`; got != want {
			t.Errorf(`List string got %v, want %v`, got, want)
		}
		x.AssignLen(1)
		if got, want := x.Len(), 1; got != want {
			t.Errorf(`List assign len got %v, want %v`, got, want)
		}
		if g0, w0 := x.Index(0).String(), `string("A")`; g0 != w0 {
			t.Errorf(`List assign values got %v, want %v`, g0, w0)
		}
		if got, want := x.String(), `[]string{"A"}`; got != want {
			t.Errorf(`List string got %v, want %v`, got, want)
		}
		x.AssignLen(3)
		if got, want := x.Len(), 3; got != want {
			t.Errorf(`List assign len got %v, want %v`, got, want)
		}
		if g0, g1, g2, w0, w1, w2 := x.Index(0).String(), x.Index(1).String(), x.Index(2).String(), `string("A")`, `string("")`, `string("")`; g0 != w0 || g1 != w1 || g2 != w2 {
			t.Errorf(`List assign values got %v %v %v, want %v %v %v`, g0, g1, g2, w0, w1, w2)
		}
		if got, want := x.String(), `[]string{"A", "", ""}`; got != want {
			t.Errorf(`List string got %v, want %v`, got, want)
		}
	} else {
		expectMismatchedKind(t, func() { x.AssignLen(0) })
		if x.Kind() != Array {
			// Index is allowed for Array and List
			expectMismatchedKind(t, func() { x.Index(0) })
		}
		if x.Kind() != Array && x.Kind() != Set && x.Kind() != Map {
			// Len is allowed for Array, List, Set and Map
			expectMismatchedKind(t, func() { x.Len() })
		}
	}
}

func assignBytes(t *testing.T, x *Value) {
	abval, abcval, abcdval := []byte("ab"), []byte("abc"), []byte("abcd")
	efgval, efghval := []byte("efg"), []byte("efgh")
	zeroval, typestr := []byte{}, "[]byte"
	if x.Kind() == Array {
		zeroval, typestr = []byte{0, 0, 0}, "[3]byte"
	}

	if got, want := x.Bytes(), zeroval; !bytes.Equal(got, want) {
		t.Errorf(`Bytes zero value got %v, want %v`, got, want)
	}
	// AssignBytes fills all bytes of the array if the array len is equal to the
	// bytes len, and automatically assigns the len for lists.
	x.AssignBytes(abcval)
	if got, want := x.Bytes(), abcval; !bytes.Equal(got, want) {
		t.Errorf(`Bytes CopyBytes got %v, want %v`, got, want)
	}
	if got, want := x.String(), typestr+`("abc")`; got != want {
		t.Errorf(`Bytes string got %v, want %v`, got, want)
	}
	abcval[1] = 'Z' // doesn't affect x
	if got, want := x.Bytes(), []byte("abc"); !bytes.Equal(got, want) {
		t.Errorf(`Bytes got %v, want %v`, got, want)
	}
	if got, want := x.String(), typestr+`("abc")`; got != want {
		t.Errorf(`Bytes string got %v, want %v`, got, want)
	}
	// AssignBytes panics for arrays if the array len is not equal to the bytes
	// len, and automatically assigns the len for lists.
	if x.Kind() == Array {
		expectPanic(t, func() { x.AssignBytes(abval) }, "AssignBytes", "[3]byte AssignBytes(%v)", abval)
		expectPanic(t, func() { x.AssignBytes(abcdval) }, "AssignBytes", "[3]byte AssignBytes(%v)", abcdval)
	} else {
		x.AssignBytes(abval)
		if got, want := x.Bytes(), abval; !bytes.Equal(got, want) {
			t.Errorf(`Bytes CopyBytes got %v, want %v`, got, want)
		}
		if got, want := x.String(), typestr+`("ab")`; got != want {
			t.Errorf(`Bytes string got %v, want %v`, got, want)
		}
		x.AssignBytes(abcdval)
		if got, want := x.Bytes(), abcdval; !bytes.Equal(got, want) {
			t.Errorf(`Bytes CopyBytes got %v, want %v`, got, want)
		}
		if got, want := x.String(), typestr+`("abcd")`; got != want {
			t.Errorf(`Bytes string got %v, want %v`, got, want)
		}
		x.AssignLen(3)
		if got, want := x.Bytes(), []byte("abc"); !bytes.Equal(got, want) {
			t.Errorf(`Bytes CopyBytes got %v, want %v`, got, want)
		}
		if got, want := x.String(), typestr+`("abc")`; got != want {
			t.Errorf(`Bytes string got %v, want %v`, got, want)
		}
	}
	// CopyBytes ignores extra bytes, just like regular copy().
	x.CopyBytes(efghval)
	if got, want := x.Bytes(), efgval; !bytes.Equal(got, want) {
		t.Errorf(`Bytes CopyBytes got %v, want %v`, got, want)
	}
	if got, want := x.String(), typestr+`("efg")`; got != want {
		t.Errorf(`Bytes string got %v, want %v`, got, want)
	}
	// Bytes gives the underlying byteslice, which may be mutated.
	x.Bytes()[1] = 'Z'
	if got, want := x.Bytes(), []byte("eZg"); !bytes.Equal(got, want) {
		t.Errorf(`Bytes got %v, want %v`, got, want)
	}
	if got, want := x.String(), typestr+`("eZg")`; got != want {
		t.Errorf(`Bytes string got %v, want %v`, got, want)
	}
	// Indexing also works, just like for any other list.
	index := x.Index(1)
	if got, want := index.Type(), ByteType; got != want {
		t.Errorf(`Bytes index type got %v, want %v`, got, want)
	}
	if got, want := index.String(), fmt.Sprintf("byte(%d)", 'Z'); got != want {
		t.Errorf(`Bytes index string got %v, want %v`, got, want)
	}
	if got, want := index.Byte(), byte('Z'); got != want {
		t.Errorf(`Bytes index Byte got %v, want %v`, got, want)
	}
	if got, want := index, ByteValue('Z'); !EqualValue(got, want) {
		t.Errorf(`Bytes index value got %v, want %v`, got, want)
	}
	index.AssignByte('Y')
	if got, want := index.String(), fmt.Sprintf("byte(%d)", 'Y'); got != want {
		t.Errorf(`Bytes index string got %v, want %v`, got, want)
	}
	if got, want := index.Byte(), byte('Y'); got != want {
		t.Errorf(`Bytes index Byte got %v, want %v`, got, want)
	}
	if got, want := index, ByteValue('Y'); !EqualValue(got, want) {
		t.Errorf(`Bytes index value got %v, want %v`, got, want)
	}
	// Make sure the original bytes were mutated.
	if got, want := x.Bytes(), []byte("eYg"); !bytes.Equal(got, want) {
		t.Errorf(`Bytes got %v, want %v`, got, want)
	}
	if got, want := x.String(), typestr+`("eYg")`; got != want {
		t.Errorf(`Bytes string got %v, want %v`, got, want)
	}
}

func toStringSlice(a []*Value) []string {
	ret := make([]string, len(a))
	for ix := 0; ix < len(a); ix++ {
		ret[ix] = a[ix].String()
	}
	return ret
}

// matchKeys returns true iff a and b hold the same values, in any order.
func matchKeys(a, b []*Value) bool {
	ass := toStringSlice(a)
	bss := toStringSlice(a)
	sort.Strings(ass)
	sort.Strings(bss)
	return strings.Join(ass, "") == strings.Join(bss, "")
}

// matchMapString returns true iff a and b are equivalent string representations
// of a map, dealing with entries in different orders.
func matchMapString(a, b string) bool {
	atypeval := strings.SplitN(a, "{", 2)
	btypeval := strings.SplitN(b, "{", 2)
	if len(atypeval) != 2 || len(btypeval) != 2 || atypeval[0] != btypeval[0] {
		return false
	}

	a = atypeval[1]
	b = btypeval[1]

	n := len(a)
	if n != len(b) || n < 1 || a[n-1] != '}' || b[n-1] != '}' {
		return false
	}
	asplit := strings.Split(a[:n-1], ", ")
	bsplit := strings.Split(b[:n-1], ", ")
	sort.Strings(asplit)
	sort.Strings(bsplit)
	return strings.Join(asplit, "") == strings.Join(bsplit, "")
}

func assignSet(t *testing.T, x *Value) {
	if x.Kind() == Set {
		k1, k2, k3 := key1, key2, key3
		setstr1 := `set[struct{I int64;S string}]{{I: 1, S: "A"}, {I: 2, S: "B"}}`
		setstr2 := `set[struct{I int64;S string}]{{I: 2, S: "B"}}`
		if x.Type().Key() == StringType {
			k1, k2, k3 = strA, strB, strC
			setstr1, setstr2 = `set[string]{"A", "B"}`, `set[string]{"B"}`
		}

		if got, want := x.Keys(), []*Value{}; !matchKeys(got, want) {
			t.Errorf(`Set Keys got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k1), false; got != want {
			t.Errorf(`Set ContainsKey k1 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k2), false; got != want {
			t.Errorf(`Set ContainsKey k2 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k3), false; got != want {
			t.Errorf(`Set ContainsKey k3 got %v, want %v`, got, want)
		}
		x.AssignSetKey(k1)
		x.AssignSetKey(k2)
		if got, want := x.Keys(), []*Value{k1, k2}; !matchKeys(got, want) {
			t.Errorf(`Set Keys got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k1), true; got != want {
			t.Errorf(`Set ContainsKey k1 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k2), true; got != want {
			t.Errorf(`Set ContainsKey k2 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k3), false; got != want {
			t.Errorf(`Set ContainsKey k3 got %v, want %v`, got, want)
		}
		if got, want := x.String(), setstr1; !matchMapString(got, want) {
			t.Errorf(`Set String got %v, want %v`, got, want)
		}
		x.DeleteSetKey(k1)
		if got, want := x.Keys(), []*Value{k2}; !matchKeys(got, want) {
			t.Errorf(`Set Keys got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k1), false; got != want {
			t.Errorf(`Set ContainsKey k1 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k2), true; got != want {
			t.Errorf(`Set ContainsKey k2 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k3), false; got != want {
			t.Errorf(`Set ContainsKey k3 got %v, want %v`, got, want)
		}
		if got, want := x.String(), setstr2; !matchMapString(got, want) {
			t.Errorf(`Set String got %v, want %v`, got, want)
		}
	} else {
		expectMismatchedKind(t, func() { x.AssignSetKey(nil) })
		expectMismatchedKind(t, func() { x.DeleteSetKey(nil) })
		if x.Kind() != Map {
			// Keys and ContainsKey are allowed for Set and Map
			expectMismatchedKind(t, func() { x.Keys() })
			expectMismatchedKind(t, func() { x.ContainsKey(nil) })
		}
		if x.Kind() != Array && x.Kind() != List && x.Kind() != Map {
			// Len is allowed for Array, List, Set and Map
			expectMismatchedKind(t, func() { x.Len() })
		}
	}
}

func assignMap(t *testing.T, x *Value) {
	if x.Kind() == Map {
		k1, k2, k3 := key1, key2, key3
		v1, v2 := int1, int2
		mapstr1 := `map[struct{I int64;S string}]int64{{I: 1, S: "A"}: 1, {I: 2, S: "B"}: 2}`
		mapstr2 := `map[struct{I int64;S string}]int64{{I: 2, S: "B"}: 2}`
		if x.Type().Key() == StringType {
			k1, k2, k3 = strA, strB, strC
			mapstr1 = `map[string]int64{"A": 1, "B": 2}`
			mapstr2 = `map[string]int64{"B": 2}`
		}

		if got, want := x.Keys(), []*Value{}; !matchKeys(got, want) {
			t.Errorf(`Map Keys got %v, want %v`, got, want)
		}
		if got := x.MapIndex(k1); got != nil {
			t.Errorf(`Map MapIndex k1 got %v, want nil`, got)
		}
		if got := x.MapIndex(k2); got != nil {
			t.Errorf(`Map MapIndex k2 got %v, want nil`, got)
		}
		if got := x.MapIndex(k3); got != nil {
			t.Errorf(`Map MapIndex k3 got %v, want nil`, got)
		}
		if got, want := x.ContainsKey(k1), false; got != want {
			t.Errorf(`Map ContainsKey k1 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k2), false; got != want {
			t.Errorf(`Map ContainsKey k2 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k3), false; got != want {
			t.Errorf(`Map ContainsKey k3 got %v, want %v`, got, want)
		}
		x.AssignMapIndex(k1, v1)
		x.AssignMapIndex(k2, v2)
		if got, want := x.Keys(), []*Value{k1, k2}; !matchKeys(got, want) {
			t.Errorf(`Map Keys got %v, want %v`, got, want)
		}
		if got, want := x.MapIndex(k1), v1; !EqualValue(got, want) {
			t.Errorf(`Map MapIndex k1 got %v, want %v`, got, want)
		}
		if got, want := x.MapIndex(k2), v2; !EqualValue(got, want) {
			t.Errorf(`Map MapIndex k2 got %v, want %v`, got, want)
		}
		if got := x.MapIndex(k3); got != nil {
			t.Errorf(`Map MapIndex k3 got %v, want nil`, got)
		}
		if got, want := x.ContainsKey(k1), true; got != want {
			t.Errorf(`Map ContainsKey k1 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k2), true; got != want {
			t.Errorf(`Map ContainsKey k2 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k3), false; got != want {
			t.Errorf(`Map ContainsKey k3 got %v, want %v`, got, want)
		}
		if got, want := x.String(), mapstr1; !matchMapString(got, want) {
			t.Errorf(`Map string got %v, want %v`, got, want)
		}
		x.AssignMapIndex(k1, nil)
		if got, want := x.Keys(), []*Value{k1, k2}; !matchKeys(got, want) {
			t.Errorf(`Map Keys got %v, want %v`, got, want)
		}
		if got := x.MapIndex(k1); got != nil {
			t.Errorf(`Map MapIndex k1 got %v, want nil`, got)
		}
		if got, want := x.MapIndex(k2), v2; !EqualValue(got, want) {
			t.Errorf(`Map MapIndex k2 got %v, want %v`, got, want)
		}
		if got := x.MapIndex(k3); got != nil {
			t.Errorf(`Map MapIndex k3 got %v, want nil`, got)
		}
		if got, want := x.ContainsKey(k1), false; got != want {
			t.Errorf(`Map ContainsKey k1 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k2), true; got != want {
			t.Errorf(`Map ContainsKey k2 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k3), false; got != want {
			t.Errorf(`Map ContainsKey k3 got %v, want %v`, got, want)
		}
		if got, want := x.String(), mapstr2; !matchMapString(got, want) {
			t.Errorf(`Map string got %v, want %v`, got, want)
		}
	} else {
		expectMismatchedKind(t, func() { x.MapIndex(nil) })
		expectMismatchedKind(t, func() { x.AssignMapIndex(nil, nil) })
		if x.Kind() != Set {
			// Keys and ContainsKey are allowed for Set and Map
			expectMismatchedKind(t, func() { x.Keys() })
			expectMismatchedKind(t, func() { x.ContainsKey(nil) })
		}
		if x.Kind() != Array && x.Kind() != List && x.Kind() != Set {
			// Len is allowed for Array, List, Set and Map
			expectMismatchedKind(t, func() { x.Len() })
		}
	}
}

func assignStruct(t *testing.T, x *Value) {
	if x.Kind() == Struct {
		x.Field(0).AssignInt(1)
		if x.IsZero() {
			t.Errorf(`Struct assign index 0 is zero`)
		}
		if got, want := x.String(), `struct{A int64;B string;C bool}{A: 1, B: "", C: false}`; got != want {
			t.Errorf(`Struct assign index 0 got %v, want %v`, got, want)
		}
		x.Field(1).AssignString("a")
		if x.IsZero() {
			t.Errorf(`Struct assign index 1 is zero`)
		}
		if got, want := x.String(), `struct{A int64;B string;C bool}{A: 1, B: "a", C: false}`; got != want {
			t.Errorf(`Struct assign index 1 got %v, want %v`, got, want)
		}
		y := CopyValue(x)
		y.Field(2).AssignBool(false)
		if !EqualValue(x, y) {
			t.Errorf(`Struct !equal %v and %v`, x, y)
		}
	} else {
		expectMismatchedKind(t, func() { x.Field(0) })
	}
}

func assignOneOf(t *testing.T, x *Value) {
	if x.Kind() == OneOf {
		if got, want := x.Elem(), Int64Value(0); !EqualValue(got, want) {
			t.Errorf(`OneOf zero value got %v, want %v`, got, want)
		}
		x.Assign(int1)
		if got, want := x.Elem(), int1; !EqualValue(got, want) {
			t.Errorf(`OneOf assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), "oneof{int64;string}(int64(1))"; got != want {
			t.Errorf(`OneOf assign string got %v, want %v`, got, want)
		}
		x.Assign(strA)
		if got, want := x.Elem(), strA; !EqualValue(got, want) {
			t.Errorf(`OneOf assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), `oneof{int64;string}(string("A"))`; got != want {
			t.Errorf(`OneOf assign string got %v, want %v`, got, want)
		}
	} else if x.Kind() != Any && x.Kind() != Nilable {
		expectMismatchedKind(t, func() { x.Elem() })
	}
}

func assignAny(t *testing.T, x *Value) {
	if x.Kind() == Any {
		if got := x.Elem(); got != nil {
			t.Errorf(`Any zero value got %v, want nil`, got)
		}
		x.Assign(int1)
		if got, want := x.Elem(), int1; !EqualValue(got, want) {
			t.Errorf(`Any assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), "any(int64(1))"; got != want {
			t.Errorf(`Any assign string got %v, want %v`, got, want)
		}
		x.Assign(strA)
		if got, want := x.Elem(), strA; !EqualValue(got, want) {
			t.Errorf(`Any assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), `any(string("A"))`; got != want {
			t.Errorf(`Any assign string got %v, want %v`, got, want)
		}
	} else if x.Kind() != OneOf && x.Kind() != Nilable {
		expectMismatchedKind(t, func() { x.Elem() })
	}
}
