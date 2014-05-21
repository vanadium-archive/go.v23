package val

import (
	"bytes"
	"sort"
	"strings"
	"testing"
)

var (
	keyType = StructType("Key", []StructField{{"I", Int64Type}, {"S", StringType}})

	strA, strB, strC = StringValue("A"), StringValue("B"), StringValue("C")
	int1, int2       = Int64Value(1), Int64Value(2)
	key1, key2, key3 = makeKey(1, "A"), makeKey(2, "B"), makeKey(3, "C")
)

func makeKey(a int64, b string) *Value {
	key := Zero(keyType)
	key.Field(0).AssignInt(a)
	key.Field(1).AssignString(b)
	return key
}

func TestValue(t *testing.T) {
	tests := []struct {
		k Kind
		t *Type
		s string
	}{
		{Bool, BoolType, "bool(false)"},
		{Int32, Int32Type, "int32(0)"},
		{Int64, Int64Type, "int64(0)"},
		{Uint32, Uint32Type, "uint32(0)"},
		{Uint64, Uint64Type, "uint64(0)"},
		{Float32, Float32Type, "float32(0)"},
		{Float64, Float64Type, "float64(0)"},
		{Complex64, Complex64Type, "complex64(0+0i)"},
		{Complex128, Complex128Type, "complex128(0+0i)"},
		{String, StringType, `string("")`},
		{Bytes, BytesType, `bytes("")`},
		{TypeVal, TypeValType, "typeval(any)"},
		{Enum, EnumType("Enum", []string{"A", "B", "C"}), "Enum enum{A;B;C}(A)"},
		{List, ListType(StringType), "[]string{}"},
		{Map, MapType(StringType, Int64Type), "map[string]int64{}"},
		{Map, MapType(keyType, Int64Type), "map[Key struct{I int64;S string}]int64{}"},
		{Struct, StructType("Struct", []StructField{{"A", Int64Type}, {"B", StringType}, {"C", BoolType}}), `Struct struct{A int64;B string;C bool}{A: 0, B: "", C: false}`},
		{OneOf, OneOfType("OneOf", []*Type{Int64Type, StringType}), "nil"},
		{Any, AnyType, "nil"},
	}
	for _, test := range tests {
		x := Zero(test.t)
		if !x.IsZero() {
			t.Errorf(`Zero(%s) isn't zero`, test.k)
		}
		if got, want := x.Kind(), test.k; got != want {
			t.Errorf(`Zero(%s) got kind %v, want %v`, test.k, got, want)
		}
		if got, want := x.Type(), test.t; got != want {
			t.Errorf(`Zero(%s) got type %v, want %v`, test.k, got, want)
		}
		if got, want := x.String(), test.s; got != want {
			t.Errorf(`Zero(%s) got string %q, want %q`, test.k, got, want)
		}
		y := Copy(x)
		if !y.IsZero() {
			t.Errorf(`Zero(%s) of copy isn't zero, y: %v`, test.k, y)
		}
		if !Equal(x, y) {
			t.Errorf(`Zero(%s) !Equal after copy, x: %v, y: %v`, test.k, x, y)
		}

		// Invariant here: x == y == 0
		// The assign[KIND] functions assign a nonzero value.
		assignBool(t, x)
		assignInt(t, x)
		assignUint(t, x)
		assignFloat(t, x)
		assignComplex(t, x)
		assignString(t, x)
		assignBytes(t, x)
		assignTypeVal(t, x)
		assignEnum(t, x)
		assignList(t, x)
		assignMap(t, x)
		assignStruct(t, x)
		assignOneOfAny(t, x)

		// Invariant here: x != 0 && y == 0
		if Equal(x, y) {
			t.Errorf(`Zero(%s) Equal after assign, x: %v, y: %v`, test.k, x, y)
		}
		z := Copy(x) // x == z && x != 0 && y == 0
		if !Equal(x, z) {
			t.Errorf(`Zero(%s) !Equal after copy, x: %v, z: %v`, test.k, x, z)
		}
		if Equal(y, z) {
			t.Errorf(`Zero(%s) Equal after copy, y: %v, z: %v`, test.k, y, z)
		}

		z.Assign(nil) // x != 0 && y == z == 0
		if !z.IsZero() {
			t.Errorf(`Zero(%s) z after Assign(nil) isn't zero`, test.k)
		}
		if Equal(x, z) {
			t.Errorf(`Zero(%s) Equal after Assign(nil), x: %v, z: %v`, test.k, x, z)
		}
		if !Equal(y, z) {
			t.Errorf(`Zero(%s) !Equal after Assign(nil), y: %v, z: %v`, test.k, y, z)
		}

		y.Assign(x) // x == y && x != 0 && z == 0
		if y.IsZero() {
			t.Errorf(`Zero(%s) y after Assign(x) is zero, y: %v`, test.k, y)
		}
		if !Equal(x, y) {
			t.Errorf(`Zero(%s) Equal after Assign(x), x: %v, y: %v`, test.k, x, y)
		}
		if Equal(y, z) {
			t.Errorf(`Zero(%s) Equal after Assign(x), y: %v, z: %v`, test.k, y, z)
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

func assignInt(t *testing.T, x *Value) {
	newval, newstr32, newstr64 := int64(123), "int32(123)", "int64(123)"
	switch x.Kind() {
	case Int32, Int64:
		if got, want := x.Int(), int64(0); got != want {
			t.Errorf(`Int zero value got %v, want %v`, got, want)
		}
		x.AssignInt(newval)
		if got, want := x.Int(), newval; got != want {
			t.Errorf(`Int assign value got %v, want %v`, got, want)
		}
		var newstr string
		switch x.Kind() {
		case Int32:
			newstr = newstr32
		case Int64:
			newstr = newstr64
		}
		if got, want := x.String(), newstr; got != want {
			t.Errorf(`Int string got %v, want %v`, got, want)
		}
	default:
		expectMismatchedKind(t, func() { x.Int() })
		expectMismatchedKind(t, func() { x.AssignInt(newval) })
	}
}

func assignUint(t *testing.T, x *Value) {
	newval, newstr32, newstr64 := uint64(123), "uint32(123)", "uint64(123)"
	switch x.Kind() {
	case Uint32, Uint64:
		if got, want := x.Uint(), uint64(0); got != want {
			t.Errorf(`Uint zero value got %v, want %v`, got, want)
		}
		x.AssignUint(newval)
		if got, want := x.Uint(), newval; got != want {
			t.Errorf(`Uint assign value got %v, want %v`, got, want)
		}
		var newstr string
		switch x.Kind() {
		case Uint32:
			newstr = newstr32
		case Uint64:
			newstr = newstr64
		}
		if got, want := x.String(), newstr; got != want {
			t.Errorf(`Uint string got %v, want %v`, got, want)
		}
	default:
		expectMismatchedKind(t, func() { x.Uint() })
		expectMismatchedKind(t, func() { x.AssignUint(newval) })
	}
}

func assignFloat(t *testing.T, x *Value) {
	newval, newstr32, newstr64 := float64(1.23), "float32(1.23)", "float64(1.23)"
	switch x.Kind() {
	case Float32, Float64:
		if got, want := x.Float(), float64(0); got != want {
			t.Errorf(`Float zero value got %v, want %v`, got, want)
		}
		x.AssignFloat(newval)
		if got, want := x.Float(), newval; got != want {
			t.Errorf(`Float assign value got %v, want %v`, got, want)
		}
		var newstr string
		switch x.Kind() {
		case Float32:
			newstr = newstr32
		case Float64:
			newstr = newstr64
		}
		if got, want := x.String(), newstr; got != want {
			t.Errorf(`Float string got %v, want %v`, got, want)
		}
	default:
		expectMismatchedKind(t, func() { x.Float() })
		expectMismatchedKind(t, func() { x.AssignFloat(newval) })
	}
}

func assignComplex(t *testing.T, x *Value) {
	newval, newstr64, newstr128 := complex128(1+2i), "complex64(1+2i)", "complex128(1+2i)"
	switch x.Kind() {
	case Complex64, Complex128:
		if got, want := x.Complex(), complex128(0); got != want {
			t.Errorf(`Complex zero value got %v, want %v`, got, want)
		}
		x.AssignComplex(newval)
		if got, want := x.Complex(), newval; got != want {
			t.Errorf(`Complex assign value got %v, want %v`, got, want)
		}
		var newstr string
		switch x.Kind() {
		case Complex64:
			newstr = newstr64
		case Complex128:
			newstr = newstr128
		}
		if got, want := x.String(), newstr; got != want {
			t.Errorf(`Complex string got %v, want %v`, got, want)
		}
	default:
		expectMismatchedKind(t, func() { x.Complex() })
		expectMismatchedKind(t, func() { x.AssignComplex(newval) })
	}
}

func assignString(t *testing.T, x *Value) {
	newval := "abc"
	zerostr := `string("")`
	newvalstr := `string("abc")`
	if x.Kind() == String {
		if got, want := x.String(), zerostr; got != want {
			t.Errorf(`String zero value got %v, want %v`, got, want)
		}
		x.AssignString(newval)
		if got, want := x.RawString(), newval; got != want {
			t.Errorf(`String assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), newvalstr; got != want {
			t.Errorf(`String assign string rep got %v, want %v`, got, want)
		}
	} else {
		expectMismatchedKind(t, func() { x.RawString() })
		expectMismatchedKind(t, func() { x.AssignString(newval) })
	}
}

func assignBytes(t *testing.T, x *Value) {
	newval := []byte("abc")
	newvalstr := `bytes("abc")`
	modifiedstr := `bytes("aZc")`
	if x.Kind() == Bytes {
		if got, want := x.Bytes(), []byte(""); !bytes.Equal(got, want) {
			t.Errorf(`Bytes zero value got %v, want %v`, got, want)
		}
		// AssignBytes allows aliasing.
		x.AssignBytes(newval)
		if got, want := x.Bytes(), newval; !bytes.Equal(got, want) {
			t.Errorf(`Bytes assign value got %v, want %v`, got, want)
		}
		newval[1] = 'Z'
		if got, want := x.Bytes(), newval; !bytes.Equal(got, want) {
			t.Errorf(`Bytes assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), modifiedstr; got != want {
			t.Errorf(`Bytes string got %v, want %v`, got, want)
		}
		// CopyBytes doesn't allow aliasing.
		newval = []byte("abc")
		x.CopyBytes(newval)
		if got, want := x.Bytes(), newval; !bytes.Equal(got, want) {
			t.Errorf(`Bytes assign value got %v, want %v`, got, want)
		}
		newval[1] = 'Z'
		if got, want := x.Bytes(), []byte("abc"); !bytes.Equal(got, want) {
			t.Errorf(`Bytes assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), newvalstr; got != want {
			t.Errorf(`Bytes string got %v, want %v`, got, want)
		}
	} else {
		expectMismatchedKind(t, func() { x.Bytes() })
		expectMismatchedKind(t, func() { x.AssignBytes(newval) })
		expectMismatchedKind(t, func() { x.CopyBytes(newval) })
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

func assignEnum(t *testing.T, x *Value) {
	if x.Kind() == Enum {
		if gi, gl, wi, wl := x.EnumIndex(), x.EnumLabel(), 0, "A"; gi != wi || gl != wl {
			t.Errorf(`Enum zero value got [%d]%v, want [%d]%v`, gi, gl, wi, wl)
		}
		x.AssignEnumIndex(1)
		if gi, gl, wi, wl := x.EnumIndex(), x.EnumLabel(), 1, "B"; gi != wi || gl != wl {
			t.Errorf(`Enum assign index value got [%d]%v, want [%d]%v`, gi, gl, wi, wl)
		}
		if got, want := x.String(), "Enum enum{A;B;C}(B)"; got != want {
			t.Errorf(`Bytes string got %v, want %v`, got, want)
		}
		x.AssignEnumIndex(2)
		if gi, gl, wi, wl := x.EnumIndex(), x.EnumLabel(), 2, "C"; gi != wi || gl != wl {
			t.Errorf(`Enum assign label value got [%d]%v, want [%d]%v`, gi, gl, wi, wl)
		}
		if got, want := x.String(), "Enum enum{A;B;C}(C)"; got != want {
			t.Errorf(`Bytes string got %v, want %v`, got, want)
		}
	} else {
		expectMismatchedKind(t, func() { x.EnumIndex() })
		expectMismatchedKind(t, func() { x.EnumLabel() })
		expectMismatchedKind(t, func() { x.AssignEnumIndex(0) })
		expectMismatchedKind(t, func() { x.AssignEnumLabel("A") })
	}
}

func assignList(t *testing.T, x *Value) {
	if x.Kind() == List {
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
			t.Errorf(`Bytes string got %v, want %v`, got, want)
		}
		x.Index(0).AssignString("A")
		x.Index(1).AssignString("B")
		if g0, g1, w0, w1 := x.Index(0).String(), x.Index(1).String(), `string("A")`, `string("B")`; g0 != w0 || g1 != w1 {
			t.Errorf(`List assign values got %v %v, want %v %v`, g0, g1, w0, w1)
		}
		if got, want := x.String(), `[]string{"A", "B"}`; got != want {
			t.Errorf(`Bytes string got %v, want %v`, got, want)
		}
		x.AssignLen(1)
		if got, want := x.Len(), 1; got != want {
			t.Errorf(`List assign len got %v, want %v`, got, want)
		}
		if g0, w0 := x.Index(0).String(), `string("A")`; g0 != w0 {
			t.Errorf(`List assign values got %v, want %v`, g0, w0)
		}
		if got, want := x.String(), `[]string{"A"}`; got != want {
			t.Errorf(`Bytes string got %v, want %v`, got, want)
		}
		x.AssignLen(3)
		if got, want := x.Len(), 3; got != want {
			t.Errorf(`List assign len got %v, want %v`, got, want)
		}
		if g0, g1, g2, w0, w1, w2 := x.Index(0).String(), x.Index(1).String(), x.Index(2).String(), `string("A")`, `string("")`, `string("")`; g0 != w0 || g1 != w1 || g2 != w2 {
			t.Errorf(`List assign values got %v %v %v, want %v %v %v`, g0, g1, g2, w0, w1, w2)
		}
		if got, want := x.String(), `[]string{"A", "", ""}`; got != want {
			t.Errorf(`Bytes string got %v, want %v`, got, want)
		}
	} else {
		expectMismatchedKind(t, func() { x.Index(0) })
		expectMismatchedKind(t, func() { x.AssignLen(0) })
		if x.Kind() != Map {
			// Len is allowed for List and Map
			expectMismatchedKind(t, func() { x.Len() })
		}
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

func assignMap(t *testing.T, x *Value) {
	if x.Kind() == Map {
		k1, k2, k3 := key1, key2, key3
		v1, v2 := int1, int2
		mapstr1 := `map[Key struct{I int64;S string}]int64{{I: 1, S: "A"}: 1, {I: 2, S: "B"}: 2}`
		mapstr2 := `map[Key struct{I int64;S string}]int64{{I: 2, S: "B"}: 2}`
		if x.Type().Key() == StringType {
			k1, k2, k3 = strA, strB, strC
			mapstr1, mapstr2 = `map[string]int64{"A": 1, "B": 2}`, `map[string]int64{"B": 2}`
		}

		if got, want := x.MapKeys(), []*Value{}; !matchKeys(got, want) {
			t.Errorf(`Map zero value got keys %v, want %v`, got, want)
		}
		if got := x.MapIndex(k1); got != nil {
			t.Errorf(`Map zero value got "A" %v, want nil`, got)
		}
		x.AssignMapIndex(k1, v1)
		x.AssignMapIndex(k2, v2)
		if got, want := x.MapKeys(), []*Value{k1, k2}; !matchKeys(got, want) {
			t.Errorf(`Map value got keys %v, want %v`, got, want)
		}
		if got, want := x.MapIndex(k1), v1; !Equal(got, want) {
			t.Errorf(`Map value got "A" %v, want %v`, got, want)
		}
		if got, want := x.MapIndex(k2), v2; !Equal(got, want) {
			t.Errorf(`Map value got "B" %v, want %v`, got, want)
		}
		if got := x.MapIndex(k3); got != nil {
			t.Errorf(`Map value got "C" %v, want nil`, got)
		}
		if got, want := x.String(), mapstr1; !matchMapString(got, want) {
			t.Errorf(`Map string got %v, want %v`, got, want)
		}
		x.AssignMapIndex(k1, nil)
		if got, want := x.MapKeys(), []*Value{k2}; !matchKeys(got, want) {
			t.Errorf(`Map value got keys %v, want %v`, got, want)
		}
		if got := x.MapIndex(k1); got != nil {
			t.Errorf(`Map value got "A" %v, want nil`, got)
		}
		if got, want := x.MapIndex(k2), v2; !Equal(got, want) {
			t.Errorf(`Map value got "B" %v, want %v`, got, want)
		}
		if got := x.MapIndex(k3); got != nil {
			t.Errorf(`Map value got "C" %v, want nil`, got)
		}
		if got, want := x.String(), mapstr2; !matchMapString(got, want) {
			t.Errorf(`Map string got %v, want %v`, got, want)
		}
	} else {
		expectMismatchedKind(t, func() { x.MapKeys() })
		expectMismatchedKind(t, func() { x.MapIndex(nil) })
		expectMismatchedKind(t, func() { x.AssignMapIndex(nil, nil) })
		if x.Kind() != List {
			// Len is allowed for List and Map
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
		if got, want := x.String(), `Struct struct{A int64;B string;C bool}{A: 1, B: "", C: false}`; got != want {
			t.Errorf(`Struct assign index 0 got %v, want %v`, got, want)
		}
		x.Field(1).AssignString("a")
		if x.IsZero() {
			t.Errorf(`Struct assign index 1 is zero`)
		}
		if got, want := x.String(), `Struct struct{A int64;B string;C bool}{A: 1, B: "a", C: false}`; got != want {
			t.Errorf(`Struct assign index 1 got %v, want %v`, got, want)
		}
		y := Copy(x)
		y.Field(2).AssignBool(false)
		if !Equal(x, y) {
			t.Errorf(`Struct !equal %v and %v`, x, y)
		}
	} else {
		expectMismatchedKind(t, func() { x.Field(0) })
	}
}

func assignOneOfAny(t *testing.T, x *Value) {
	if x.Kind() == OneOf || x.Kind() == Any {
		if got := x.Elem(); got != nil {
			t.Errorf(`OneOf/Any zero value got %v, want nil`, got)
		}
		x.Assign(int1)
		if got, want := x.Elem(), int1; !Equal(got, want) {
			t.Errorf(`OneOf/Any assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), "int64(1)"; got != want {
			t.Errorf(`OneOf/Any assign string got %v, want %v`, got, want)
		}
		x.Assign(strA)
		if got, want := x.Elem(), strA; !Equal(got, want) {
			t.Errorf(`OneOf/Any assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), `string("A")`; got != want {
			t.Errorf(`OneOf/Any assign string got %v, want %v`, got, want)
		}
	} else {
		expectMismatchedKind(t, func() { x.Elem() })
	}
}
