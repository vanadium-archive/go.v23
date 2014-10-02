package vdl

import (
	"fmt"
	"strings"
	"testing"
)

// TODO(toddw): Add tests for ValidKey, ValidOneOfType

func TestSplitIdent(t *testing.T) {
	tests := []struct {
		ident, pkgpath, name string
	}{
		{".", "", ""},
		{"a", "", "a"},
		{"Foo", "", "Foo"},
		{"a.Foo", "a", "Foo"},
		{"a/Foo", "", "a/Foo"},
		{"a/b.Foo", "a/b", "Foo"},
		{"a.b.Foo", "a.b", "Foo"},
		{"a/b/c.Foo", "a/b/c", "Foo"},
		{"a/b.c.Foo", "a/b.c", "Foo"},
	}
	for _, test := range tests {
		pkgpath, name := SplitIdent(test.ident)
		if got, want := pkgpath, test.pkgpath; got != want {
			t.Errorf("%s got pkgpath %q, want %q", test.ident, got, want)
		}
		if got, want := name, test.name; got != want {
			t.Errorf("%s got name %q, want %q", test.ident, got, want)
		}
	}
}

var singletons = []struct {
	k Kind
	t *Type
	s string
}{
	{Any, AnyType, "any"},
	{Bool, BoolType, "bool"},
	{Byte, ByteType, "byte"},
	{Uint16, Uint16Type, "uint16"},
	{Uint32, Uint32Type, "uint32"},
	{Uint64, Uint64Type, "uint64"},
	{Int16, Int16Type, "int16"},
	{Int32, Int32Type, "int32"},
	{Int64, Int64Type, "int64"},
	{Float32, Float32Type, "float32"},
	{Float64, Float64Type, "float64"},
	{Complex64, Complex64Type, "complex64"},
	{Complex128, Complex128Type, "complex128"},
	{String, StringType, "string"},
	{TypeVal, TypeValType, "typeval"},
}

type l []string

var enums = []struct {
	name   string
	labels l
	str    string
	errstr string
}{
	{"FailNoLabels", l{}, "", "no enum labels"},
	{"FailEmptyLabel", l{""}, "", "empty enum label"},
	{"", l{"A"}, "enum{A}", ""},
	{"A", l{"A"}, "A enum{A}", ""},
	{"AB", l{"A", "B"}, "AB enum{A;B}", ""},
}

type f []StructField

var structs = []struct {
	name   string
	fields f
	str    string
	errstr string
}{
	{"FailFieldName", f{{"", BoolType}}, "", "empty struct field name"},
	{"FailDupFields", f{{"A", BoolType}, {"A", Int32Type}}, "", "duplicate field name"},
	{"FailNilFieldType", f{{"A", nil}}, "", "nil struct field type"},
	{"", f{}, "struct{}", ""},
	{"Empty", f{}, "Empty struct{}", ""},
	{"A", f{{"A", BoolType}}, "A struct{A bool}", ""},
	{"AB", f{{"A", BoolType}, {"B", Int32Type}}, "AB struct{A bool;B int32}", ""},
	{"ABC", f{{"A", BoolType}, {"B", Int32Type}, {"C", Uint64Type}}, "ABC struct{A bool;B int32;C uint64}", ""},
	{"ABCD", f{{"A", BoolType}, {"B", Int32Type}, {"C", Uint64Type}, {"D", StringType}}, "ABCD struct{A bool;B int32;C uint64;D string}", ""},
}

type t []*Type

var oneofs = []struct {
	name   string
	types  t
	str    string
	errstr string
}{
	{"FailNoTypes", t{}, "", "no oneof types"},
	{"FailAny", t{AnyType}, "", "type in oneof must not be nil, oneof or any"},
	{"FailDup", t{BoolType, BoolType}, "", "duplicate oneof type"},
	{"", t{BoolType}, "oneof{bool}", ""},
	{"A", t{BoolType}, "A oneof{bool}", ""},
	{"AB", t{BoolType, Int32Type}, "AB oneof{bool;int32}", ""},
	{"ABC", t{BoolType, Int32Type, Uint64Type}, "ABC oneof{bool;int32;uint64}", ""},
	{"ABCD", t{BoolType, Int32Type, Uint64Type, StringType}, "ABCD oneof{bool;int32;uint64;string}", ""},
}

func allTypes() (types []*Type) {
	for index, test := range singletons {
		types = append(types, test.t, ArrayType(index+1, test.t))
		types = append(types, test.t, ListType(test.t))
		types = append(types, test.t, SetType(test.t))
		for _, test2 := range singletons {
			types = append(types, MapType(test.t, test2.t))
		}
		if test.k != Any && test.k != TypeVal {
			types = append(types, NamedType("Named"+test.s, test.t))
		}
		if test.k != Any {
			types = append(types, NilableType(test.t))
		}
	}
	for _, test := range enums {
		if test.errstr == "" {
			types = append(types, EnumType(test.labels...))
		}
	}
	for _, test := range structs {
		if test.errstr == "" {
			types = append(types, StructType(test.fields...))
		}
	}
	for _, test := range oneofs {
		if test.errstr == "" {
			types = append(types, OneOfType(test.types...))
		}
	}
	return
}

func TestTypeMismatch(t *testing.T) {
	// Make sure we panic if a method is called for a mismatched kind.
	for _, ty := range allTypes() {
		k := ty.Kind()
		if k != Enum {
			expectMismatchedKind(t, func() { ty.EnumLabel(0) })
			expectMismatchedKind(t, func() { ty.EnumIndex("") })
			expectMismatchedKind(t, func() { ty.NumEnumLabel() })
		}
		if k != Array {
			expectMismatchedKind(t, func() { ty.Len() })
		}
		if k != Nilable && k != Array && k != List && k != Map {
			expectMismatchedKind(t, func() { ty.Elem() })
		}
		if k != Set && k != Map {
			expectMismatchedKind(t, func() { ty.Key() })
		}
		if k != Struct {
			expectMismatchedKind(t, func() { ty.Field(0) })
			expectMismatchedKind(t, func() { ty.FieldByName("") })
			expectMismatchedKind(t, func() { ty.NumField() })
		}
		if k != OneOf {
			expectMismatchedKind(t, func() { ty.OneOfType(0) })
			expectMismatchedKind(t, func() { ty.OneOfIndex(AnyType) })
			expectMismatchedKind(t, func() { ty.NumOneOfType() })
		}
	}
}

func testSingleton(t *testing.T, k Kind, ty *Type, s string) {
	if got, want := ty.Kind(), k; got != want {
		t.Errorf(`%s got kind %q, want %q`, k, got, want)
	}
	if got, want := ty.Name(), ""; got != want {
		t.Errorf(`%s got name %q, want %q`, k, got, want)
	}
	if got, want := k.String(), s; got != want {
		t.Errorf(`%s got kind %q, want %q`, k, got, want)
	}
	if got, want := ty.String(), s; got != want {
		t.Errorf(`%s got string %q, want %q`, k, got, want)
	}
}

func TestSingletonTypes(t *testing.T) {
	for _, test := range singletons {
		testSingleton(t, test.k, test.t, test.s)
	}
}

func TestNilableTypes(t *testing.T) {
	for _, test := range singletons {
		if test.k == Any {
			continue
		}
		nilable := NilableType(test.t)
		if got, want := nilable.Kind(), Nilable; got != want {
			t.Errorf(`%s got kind %q, want %q`, nilable, got, want)
		}
		if got, want := nilable.Name(), ""; got != want {
			t.Errorf(`%s got name %q, want %q`, nilable, got, want)
		}
		if got, want := Nilable.String(), "nilable"; got != want {
			t.Errorf(`%s got kind %q, want %q`, nilable, got, want)
		}
		if got, want := nilable.String(), "?"+test.s; got != want {
			t.Errorf(`%s got string %q, want %q`, nilable, got, want)
		}
		testSingleton(t, test.k, nilable.Elem(), test.s)
	}
}

func TestEnumTypes(t *testing.T) {
	for _, test := range enums {
		var x *Type
		create := func() {
			x = EnumType(test.labels...)
			if test.name != "" {
				x = NamedType(test.name, x)
			}
		}
		expectPanic(t, create, test.errstr, "%s EnumType", test.name)
		if x == nil {
			continue
		}
		if got, want := x.Kind(), Enum; got != want {
			t.Errorf(`Enum %s got kind %q, want %q`, test.name, got, want)
		}
		if got, want := x.Kind().String(), "enum"; got != want {
			t.Errorf(`Enum %s got kind %q, want %q`, test.name, got, want)
		}
		if got, want := x.Name(), test.name; got != want {
			t.Errorf(`Enum %s got name %q, want %q`, test.name, got, want)
		}
		if got, want := x.String(), test.str; got != want {
			t.Errorf(`Enum %s got string %q, want %q`, test.name, got, want)
		}
		if got, want := x.NumEnumLabel(), len(test.labels); got != want {
			t.Errorf(`Enum %s got num labels %d, want %d`, test.name, got, want)
		}
		for index, label := range test.labels {
			if got, want := x.EnumLabel(index), label; got != want {
				t.Errorf(`Enum %s got label[%d] %s, want %s`, test.name, index, got, want)
			}
			if got, want := x.EnumIndex(label), index; got != want {
				t.Errorf(`Enum %s got index[%s] %d, want %d`, test.name, label, got, want)
			}
		}
	}
}

func TestArrayTypes(t *testing.T) {
	for index, test := range singletons {
		len := index + 1
		x := ArrayType(len, test.t)
		if got, want := x.Kind(), Array; got != want {
			t.Errorf(`Array %s got kind %q, want %q`, test.k, got, want)
		}
		if got, want := x.Kind().String(), "array"; got != want {
			t.Errorf(`Array %s got kind %q, want %q`, test.k, got, want)
		}
		if got, want := x.Name(), ""; got != want {
			t.Errorf(`Array %s got name %q, want %q`, test.k, got, want)
		}
		if got, want := x.String(), fmt.Sprintf("[%d]%s", len, test.s); got != want {
			t.Errorf(`Array %s got string %q, want %q`, test.k, got, want)
		}
		if got, want := x.Len(), len; got != want {
			t.Errorf(`Array %s got len %v, want %v`, test.k, got, want)
		}
		if got, want := x.Elem(), test.t; got != want {
			t.Errorf(`Array %s got elem %q, want %q`, test.k, got, want)
		}
	}
}

func TestListTypes(t *testing.T) {
	for _, test := range singletons {
		x := ListType(test.t)
		if got, want := x.Kind(), List; got != want {
			t.Errorf(`List %s got kind %q, want %q`, test.k, got, want)
		}
		if got, want := x.Kind().String(), "list"; got != want {
			t.Errorf(`List %s got kind %q, want %q`, test.k, got, want)
		}
		if got, want := x.Name(), ""; got != want {
			t.Errorf(`List %s got name %q, want %q`, test.k, got, want)
		}
		if got, want := x.String(), "[]"+test.s; got != want {
			t.Errorf(`List %s got string %q, want %q`, test.k, got, want)
		}
		if got, want := x.Elem(), test.t; got != want {
			t.Errorf(`List %s got elem %q, want %q`, test.k, got, want)
		}
	}
}

func TestSetTypes(t *testing.T) {
	for _, test := range singletons {
		x := SetType(test.t)
		if got, want := x.Kind(), Set; got != want {
			t.Errorf(`Set %s got kind %q, want %q`, test.k, got, want)
		}
		if got, want := x.Kind().String(), "set"; got != want {
			t.Errorf(`Set %s got kind %q, want %q`, test.k, got, want)
		}
		if got, want := x.Name(), ""; got != want {
			t.Errorf(`Set %s got name %q, want %q`, test.k, got, want)
		}
		if got, want := x.String(), "set["+test.s+"]"; got != want {
			t.Errorf(`Set %s got string %q, want %q`, test.k, got, want)
		}
		if got, want := x.Key(), test.t; got != want {
			t.Errorf(`Set %s got key %q, want %q`, test.k, got, want)
		}
	}
}

func TestMapTypes(t *testing.T) {
	for _, key := range singletons {
		for _, elem := range singletons {
			x := MapType(key.t, elem.t)
			if got, want := x.Kind(), Map; got != want {
				t.Errorf(`Map[%s]%s got kind %q, want %q`, key.k, elem.k, got, want)
			}
			if got, want := x.Kind().String(), "map"; got != want {
				t.Errorf(`Map[%s]%s got kind %q, want %q`, key.k, elem.k, got, want)
			}
			if got, want := x.Name(), ""; got != want {
				t.Errorf(`Map[%s]%s got name %q, want %q`, key.k, elem.k, got, want)
			}
			if got, want := x.String(), fmt.Sprintf("map[%s]%s", key.s, elem.s); got != want {
				t.Errorf(`Map[%s]%s got name %q, want %q`, key.k, elem.k, got, want)
			}
			if got, want := x.Key(), key.t; got != want {
				t.Errorf(`Map[%s]%s got key %q, want %q`, key.k, elem.k, got, want)
			}
			if got, want := x.Elem(), elem.t; got != want {
				t.Errorf(`Map[%s]%s got elem %q, want %q`, key.k, elem.k, got, want)
			}
		}
	}
}

func TestStructTypes(t *testing.T) {
	for _, test := range structs {
		var x *Type
		create := func() {
			x = StructType(test.fields...)
			if test.name != "" {
				x = NamedType(test.name, x)
			}
		}
		expectPanic(t, create, test.errstr, "%s StructType", test.name)
		if x == nil {
			continue
		}
		if got, want := x.Kind(), Struct; got != want {
			t.Errorf(`Struct %s got kind %q, want %q`, test.name, got, want)
		}
		if got, want := x.Kind().String(), "struct"; got != want {
			t.Errorf(`Map %s got kind %q, want %q`, test.name, got, want)
		}
		if got, want := x.Name(), test.name; got != want {
			t.Errorf(`Struct %s got name %q, want %q`, test.name, got, want)
		}
		if got, want := x.String(), test.str; got != want {
			t.Errorf(`Struct %s got string %q, want %q`, test.name, got, want)
		}
		if got, want := x.NumField(), len(test.fields); got != want {
			t.Errorf(`Struct %s got num fields %d, want %d`, test.name, got, want)
		}
		for index, field := range test.fields {
			if got, want := x.Field(index), field; got != want {
				t.Errorf(`Struct %s got field[%d] %v, want %v`, test.name, index, got, want)
			}
			gotf, goti := x.FieldByName(field.Name)
			if wantf := field; gotf != wantf {
				t.Errorf(`Struct %s got field[%s] %v, want %v`, test.name, field.Name, gotf, wantf)
			}
			if wanti := index; goti != wanti {
				t.Errorf(`Struct %s got field[%s] index %d, want %d`, test.name, field.Name, goti, wanti)
			}
		}
	}
	// Make sure hash consing of struct types respects the ordering of the fields.
	A, B, C := BoolType, Int32Type, Uint64Type
	x := StructType([]StructField{{"A", A}, {"B", B}, {"C", C}}...)
	for iter := 0; iter < 10; iter++ {
		abc := StructType([]StructField{{"A", A}, {"B", B}, {"C", C}}...)
		acb := StructType([]StructField{{"A", A}, {"C", C}, {"B", B}}...)
		bac := StructType([]StructField{{"B", B}, {"A", A}, {"C", C}}...)
		bca := StructType([]StructField{{"B", B}, {"C", C}, {"A", A}}...)
		cab := StructType([]StructField{{"C", C}, {"A", A}, {"B", B}}...)
		cba := StructType([]StructField{{"C", C}, {"B", B}, {"A", A}}...)
		if x != abc || x == acb || x == bac || x == bca || x == cab || x == cba {
			t.Errorf(`Struct ABC hash consing broken: %v, %v, %v, %v, %v, %v, %v`, x, abc, acb, bac, bca, cab, cba)
		}
		ac := StructType([]StructField{{"A", A}, {"C", C}}...)
		ca := StructType([]StructField{{"C", C}, {"A", A}}...)
		if x == ac || x == ca {
			t.Errorf(`Struct ABC / AC hash consing broken: %v, %v, %v`, x, ac, ca)
		}
	}
}

func TestOneOfTypes(t *testing.T) {
	for _, test := range oneofs {
		var x *Type
		create := func() {
			x = OneOfType(test.types...)
			if test.name != "" {
				x = NamedType(test.name, x)
			}
		}
		expectPanic(t, create, test.errstr, "%s OneOfType", test.name)
		if x == nil {
			continue
		}
		if got, want := x.Kind(), OneOf; got != want {
			t.Errorf(`OneOf %s got kind %q, want %q`, test.name, got, want)
		}
		if got, want := x.Kind().String(), "oneof"; got != want {
			t.Errorf(`Map %s got kind %q, want %q`, test.name, got, want)
		}
		if got, want := x.Name(), test.name; got != want {
			t.Errorf(`OneOf %s got name %q, want %q`, test.name, got, want)
		}
		if got, want := x.String(), test.str; got != want {
			t.Errorf(`OneOf %s got string %q, want %q`, test.name, got, want)
		}
		if got, want := x.NumOneOfType(), len(test.types); got != want {
			t.Errorf(`OneOf %s got num types %d, want %d`, test.name, got, want)
		}
		for index, one := range test.types {
			if got, want := x.OneOfType(index), one; got != want {
				t.Errorf(`OneOf %s got type[%d] %s, want %s`, test.name, index, got, want)
			}
			if got, want := x.OneOfIndex(one), index; got != want {
				t.Errorf(`OneOf %s got index[%s] %d, want %d`, test.name, one, got, want)
			}
		}
	}
	// Make sure hash consing of oneof types respects ordering.
	A, B, C := BoolType, Int32Type, Uint64Type
	x := OneOfType(A, B, C)
	for iter := 0; iter < 10; iter++ {
		abc := OneOfType(A, B, C)
		acb := OneOfType(A, C, B)
		bac := OneOfType(B, A, C)
		bca := OneOfType(B, C, A)
		cab := OneOfType(C, A, B)
		cba := OneOfType(C, B, A)
		if x != abc || x == acb || x == bac || x == bca || x == cab || x == cba {
			t.Errorf(`OneOf ABC hash consing broken: %v, %v, %v, %v, %v, %v, %v`, x, abc, acb, bac, bca, cab, cba)
		}
		ac := OneOfType(A, C)
		ca := OneOfType(C, A)
		if x == ac || x == ca {
			t.Errorf(`OneOf ABC / AC hash consing broken: %v, %v, %v`, x, ac, ca)
		}
	}
}

func TestNamedTypes(t *testing.T) {
	for _, test := range singletons {
		var errstr string
		switch test.k {
		case Any, TypeVal:
			errstr = "any and typeval cannot be renamed"
		}
		name := "Named" + test.s
		var x *Type
		create := func() { x = NamedType(name, test.t) }
		expectPanic(t, create, errstr, name)
		if x == nil {
			continue
		}
		if got, want := x.Kind(), test.k; got != want {
			t.Errorf(`Named %s got kind %q, want %q`, test.k, got, want)
		}
		if got, want := x.Name(), name; got != want {
			t.Errorf(`Named %s got name %q, want %q`, test.k, got, want)
		}
		if got, want := x.String(), name+" "+test.s; got != want {
			t.Errorf(`Named %s got string %q, want %q`, test.k, got, want)
		}
	}
	// Try a chain of named types:
	// type A B
	// type B C
	// type C D
	// type D struct{X []C}
	var builder TypeBuilder
	a, b, c, d := builder.Named("A"), builder.Named("B"), builder.Named("C"), builder.Named("D")
	a.AssignBase(b)
	b.AssignBase(c)
	c.AssignBase(d)
	d.AssignBase(builder.Struct().AppendField("X", builder.List().AssignElem(c)))
	builder.Build()
	bA, errA := a.Built()
	bB, errB := b.Built()
	bC, errC := c.Built()
	bD, errD := d.Built()
	if errA != nil || errB != nil || errC != nil || errD != nil {
		t.Errorf(`Named chain got (%q,%q,%q,%q), want nil`, errA, errB, errC, errD)
	}
	if got, want := bA.Kind(), Struct; got != want {
		t.Errorf(`Named chain got kind %q, want %q`, got, want)
	}
	if got, want := bB.Kind(), Struct; got != want {
		t.Errorf(`Named chain got kind %q, want %q`, got, want)
	}
	if got, want := bC.Kind(), Struct; got != want {
		t.Errorf(`Named chain got kind %q, want %q`, got, want)
	}
	if got, want := bD.Kind(), Struct; got != want {
		t.Errorf(`Named chain got kind %q, want %q`, got, want)
	}
	if got, want := bA.Name(), "A"; got != want {
		t.Errorf(`Named chain got name %q, want %q`, got, want)
	}
	if got, want := bB.Name(), "B"; got != want {
		t.Errorf(`Named chain got name %q, want %q`, got, want)
	}
	if got, want := bC.Name(), "C"; got != want {
		t.Errorf(`Named chain got name %q, want %q`, got, want)
	}
	if got, want := bD.Name(), "D"; got != want {
		t.Errorf(`Named chain got name %q, want %q`, got, want)
	}
	if got, want := bA.String(), "A struct{X []C struct{X []C}}"; got != want {
		t.Errorf(`Named chain got name %q, want %q`, got, want)
	}
	if got, want := bB.String(), "B struct{X []C struct{X []C}}"; got != want {
		t.Errorf(`Named chain got name %q, want %q`, got, want)
	}
	if got, want := bC.String(), "C struct{X []C}"; got != want {
		t.Errorf(`Named chain got name %q, want %q`, got, want)
	}
	if got, want := bD.String(), "D struct{X []C struct{X []C}}"; got != want {
		t.Errorf(`Named chain got name %q, want %q`, got, want)
	}
	if got, want := bA.NumField(), 1; got != want {
		t.Errorf(`Named chain got NumField %q, want %q`, got, want)
	}
	if got, want := bB.NumField(), 1; got != want {
		t.Errorf(`Named chain got NumField %q, want %q`, got, want)
	}
	if got, want := bC.NumField(), 1; got != want {
		t.Errorf(`Named chain got NumField %q, want %q`, got, want)
	}
	if got, want := bD.NumField(), 1; got != want {
		t.Errorf(`Named chain got NumField %q, want %q`, got, want)
	}
	if got, want := bA.Field(0).Name, "X"; got != want {
		t.Errorf(`Named chain got Field(0).Name %q, want %q`, got, want)
	}
	if got, want := bB.Field(0).Name, "X"; got != want {
		t.Errorf(`Named chain got Field(0).Name %q, want %q`, got, want)
	}
	if got, want := bC.Field(0).Name, "X"; got != want {
		t.Errorf(`Named chain got Field(0).Name %q, want %q`, got, want)
	}
	if got, want := bD.Field(0).Name, "X"; got != want {
		t.Errorf(`Named chain got Field(0).Name %q, want %q`, got, want)
	}
	listC := ListType(bC)
	if got, want := bA.Field(0).Type, listC; got != want {
		t.Errorf(`Named chain got Field(0).Type %q, want %q`, got, want)
	}
	if got, want := bB.Field(0).Type, listC; got != want {
		t.Errorf(`Named chain got Field(0).Type %q, want %q`, got, want)
	}
	if got, want := bC.Field(0).Type, listC; got != want {
		t.Errorf(`Named chain got Field(0).Type %q, want %q`, got, want)
	}
	if got, want := bD.Field(0).Type, listC; got != want {
		t.Errorf(`Named chain got Field(0).Type %q, want %q`, got, want)
	}
}

func TestHashConsTypes(t *testing.T) {
	// Create a bunch of distinct types multiple times.
	var types [3][]*Type
	for iter := 0; iter < 3; iter++ {
		for _, a := range singletons {
			types[iter] = append(types[iter], a.t)
			if a.t != AnyType && a.t != TypeValType {
				types[iter] = append(types[iter], NamedType("Named"+a.s, a.t))
			}
			if a.t != AnyType {
				types[iter] = append(types[iter], NilableType(a.t))
				types[iter] = append(types[iter], NamedType("Nilable"+a.s, NilableType(a.t)))
			}
			types[iter] = append(types[iter], ListType(a.t))
			types[iter] = append(types[iter], NamedType("List"+a.s, ListType(a.t)))
			for _, b := range singletons {
				lA, lB := "A"+a.s, "B"+b.s
				name := lA + lB
				types[iter] = append(types[iter], EnumType(lA, lB))
				types[iter] = append(types[iter], MapType(a.t, b.t))
				types[iter] = append(types[iter], StructType([]StructField{{lA, a.t}, {lB, b.t}}...))
				types[iter] = append(types[iter], NamedType("Enum"+name, EnumType(lA, lB)))
				types[iter] = append(types[iter], NamedType("Map"+name, MapType(a.t, b.t)))
				types[iter] = append(types[iter], NamedType("Struct"+name, StructType([]StructField{{lA, a.t}, {lB, b.t}}...)))
				if a.t != b.t && a.k != Any && b.k != Any {
					types[iter] = append(types[iter], OneOfType(a.t, b.t))
					types[iter] = append(types[iter], NamedType("OneOf"+name, OneOfType(a.t, b.t)))
				}
			}
		}
	}

	// Make sure the pointers are the same across iterations, and different within
	// an iteration.
	seen := map[*Type]bool{}
	for ix := 0; ix < len(types[0]); ix++ {
		a, b, c := types[0][ix], types[1][ix], types[2][ix]
		if a != b || a != c {
			t.Errorf(`HashCons mismatched pointer[%d]: %p %p %p`, ix, a, b, c)
		}
		if seen[a] {
			t.Errorf(`HashCons dup pointer[%d]: %v %v`, ix, a, seen)
		}
		seen[a] = true
	}
}

func TestAssignableFrom(t *testing.T) {
	// Systematic testing of AssignableFrom over allTypes will just duplicate the
	// actual logic, so we just spot-check some results manually.
	tests := []struct {
		t, f   *Type
		expect bool
	}{
		{BoolType, BoolType, true},
		{NilableType(BoolType), NilableType(BoolType), true},
		{AnyType, BoolType, true},
		{AnyType, NilableType(BoolType), true},
		{AnyType, OneOfType(BoolType, Int32Type), true},
		{OneOfType(BoolType, NilableType(Int32Type)), BoolType, true},
		{OneOfType(BoolType, NilableType(Int32Type)), NilableType(Int32Type), true},

		{BoolType, Int32Type, false},
		{BoolType, AnyType, false},
		{BoolType, NilableType(BoolType), false},
		{BoolType, OneOfType(BoolType), false},
		{BoolType, OneOfType(BoolType, Int32Type), false},
		{OneOfType(BoolType), NilableType(BoolType), false},
		{OneOfType(BoolType), StringType, false},
		{OneOfType(BoolType, Int32Type), StringType, false},
		{NilableType(BoolType), BoolType, false},
		{NilableType(BoolType), StringType, false},
		{NilableType(OneOfType(BoolType)), NilableType(BoolType), false},
		{OneOfType(NilableType(BoolType)), BoolType, false},
	}
	for _, test := range tests {
		if test.t.AssignableFrom(test.f) != test.expect {
			t.Errorf(`%v.AssignableFrom(%v) expect %v`, test.t, test.f, test.expect)
		}
	}
}

func TestSelfRecursiveType(t *testing.T) {
	buildTree := func() (*Type, error, *Type, error) {
		// type Node struct {
		//   Val      string
		//   Children []Node
		// }
		var builder TypeBuilder
		pendN := builder.Named("Node")
		pendC := builder.List().AssignElem(pendN)
		structN := builder.Struct()
		structN.AppendField("Val", StringType)
		structN.AppendField("Children", pendC)
		pendN.AssignBase(structN)
		builder.Build()
		c, cerr := pendC.Built()
		n, nerr := pendN.Built()
		return c, cerr, n, nerr
	}
	c, cerr, n, nerr := buildTree()
	if cerr != nil || nerr != nil {
		t.Errorf(`build got cerr %q nerr %q, want nil`, cerr, nerr)
	}
	// Check node
	if got, want := n.Kind(), Struct; got != want {
		t.Errorf(`node Kind got %s, want %s`, got, want)
	}
	if got, want := n.Name(), "Node"; got != want {
		t.Errorf(`node Name got %q, want %q`, got, want)
	}
	if got, want := n.String(), "Node struct{Val string;Children []Node}"; got != want {
		t.Errorf(`node String got %q, want %q`, got, want)
	}
	if got, want := n.NumField(), 2; got != want {
		t.Errorf(`node NumField got %q, want %q`, got, want)
	}
	if got, want := n.Field(0).Name, "Val"; got != want {
		t.Errorf(`node Field(0).Name got %q, want %q`, got, want)
	}
	if got, want := n.Field(0).Type, StringType; got != want {
		t.Errorf(`node Field(0).Type got %q, want %q`, got, want)
	}
	if got, want := n.Field(1).Name, "Children"; got != want {
		t.Errorf(`node Field(1).Name got %q, want %q`, got, want)
	}
	if got, want := n.Field(1).Type, c; got != want {
		t.Errorf(`node Field(1).Type got %q, want %q`, got, want)
	}
	// Check children
	if got, want := c.Kind(), List; got != want {
		t.Errorf(`children Kind got %s, want %s`, got, want)
	}
	if got, want := c.Name(), ""; got != want {
		t.Errorf(`children Name got %q, want %q`, got, want)
	}
	if got, want := c.String(), "[]Node struct{Val string;Children []Node}"; got != want {
		t.Errorf(`children String got %q, want %q`, got, want)
	}
	if got, want := c.Elem(), n; got != want {
		t.Errorf(`children Elem got %q, want %q`, got, want)
	}
	// Check hash-consing
	for iter := 0; iter < 5; iter++ {
		c2, cerr2, n2, nerr2 := buildTree()
		if cerr2 != nil || nerr2 != nil {
			t.Errorf(`build got cerr %q nerr %q, want nil`, cerr, nerr)
		}
		if got, want := c2, c; got != want {
			t.Errorf(`cons children got %q, want %q`, got, want)
		}
		if got, want := n2, n; got != want {
			t.Errorf(`cons node got %q, want %q`, got, want)
		}
	}
}

func TestMutuallyRecursiveType(t *testing.T) {
	build := func() (*Type, error, *Type, error, *Type, error, *Type, error) {
		// type D A
		// type A struct{X int32;B B;C C}
		// type B struct{Y int32;A A;C C}
		// type C struct{Z string}
		var builder TypeBuilder
		a, b, c, d := builder.Named("A"), builder.Named("B"), builder.Named("C"), builder.Named("D")
		stA, stB, stC := builder.Struct(), builder.Struct(), builder.Struct()
		d.AssignBase(a)
		a.AssignBase(stA)
		b.AssignBase(stB)
		c.AssignBase(stC)
		stA.AppendField("X", Int32Type).AppendField("B", b).AppendField("C", c)
		stB.AppendField("Y", Int32Type).AppendField("A", a).AppendField("C", c)
		stC.AppendField("Z", StringType)
		builder.Build()
		builtD, derr := d.Built()
		builtA, aerr := a.Built()
		builtB, berr := b.Built()
		builtC, cerr := c.Built()
		return builtD, derr, builtA, aerr, builtB, berr, builtC, cerr
	}
	d, derr, a, aerr, b, berr, c, cerr := build()
	if derr != nil || aerr != nil || berr != nil || cerr != nil {
		t.Errorf(`build got (%q,%q,%q,%q), want nil`, derr, aerr, berr, cerr)
	}
	// Check D
	if got, want := d.Kind(), Struct; got != want {
		t.Errorf(`D Kind got %s, want %s`, got, want)
	}
	if got, want := d.Name(), "D"; got != want {
		t.Errorf(`D Name got %q, want %q`, got, want)
	}
	if got, want := d.String(), "D struct{X int32;B B struct{Y int32;A A struct{X int32;B B;C C struct{Z string}};C C};C C}"; got != want {
		t.Errorf(`D String got %q, want %q`, got, want)
	}
	if got, want := d.NumField(), 3; got != want {
		t.Errorf(`D NumField got %q, want %q`, got, want)
	}
	if got, want := d.Field(0).Name, "X"; got != want {
		t.Errorf(`D Field(0).Name got %q, want %q`, got, want)
	}
	if got, want := d.Field(0).Type, Int32Type; got != want {
		t.Errorf(`D Field(0).Type got %q, want %q`, got, want)
	}
	if got, want := d.Field(1).Name, "B"; got != want {
		t.Errorf(`D Field(1).Name got %q, want %q`, got, want)
	}
	if got, want := d.Field(1).Type, b; got != want {
		t.Errorf(`D Field(1).Type got %q, want %q`, got, want)
	}
	if got, want := d.Field(2).Name, "C"; got != want {
		t.Errorf(`D Field(2).Name got %q, want %q`, got, want)
	}
	if got, want := d.Field(2).Type, c; got != want {
		t.Errorf(`D Field(2).Type got %q, want %q`, got, want)
	}
	// Check A
	if got, want := a.Kind(), Struct; got != want {
		t.Errorf(`A Kind got %s, want %s`, got, want)
	}
	if got, want := a.Name(), "A"; got != want {
		t.Errorf(`A Name got %q, want %q`, got, want)
	}
	if got, want := a.String(), "A struct{X int32;B B struct{Y int32;A A;C C struct{Z string}};C C}"; got != want {
		t.Errorf(`A String got %q, want %q`, got, want)
	}
	if got, want := a.NumField(), 3; got != want {
		t.Errorf(`A NumField got %q, want %q`, got, want)
	}
	if got, want := a.Field(0).Name, "X"; got != want {
		t.Errorf(`A Field(0).Name got %q, want %q`, got, want)
	}
	if got, want := a.Field(0).Type, Int32Type; got != want {
		t.Errorf(`A Field(0).Type got %q, want %q`, got, want)
	}
	if got, want := a.Field(1).Name, "B"; got != want {
		t.Errorf(`A Field(1).Name got %q, want %q`, got, want)
	}
	if got, want := a.Field(1).Type, b; got != want {
		t.Errorf(`A Field(1).Type got %q, want %q`, got, want)
	}
	if got, want := a.Field(2).Name, "C"; got != want {
		t.Errorf(`A Field(2).Name got %q, want %q`, got, want)
	}
	if got, want := a.Field(2).Type, c; got != want {
		t.Errorf(`A Field(2).Type got %q, want %q`, got, want)
	}
	// Check B
	if got, want := b.Kind(), Struct; got != want {
		t.Errorf(`B Kind got %s, want %s`, got, want)
	}
	if got, want := b.Name(), "B"; got != want {
		t.Errorf(`B Name got %q, want %q`, got, want)
	}
	if got, want := b.String(), "B struct{Y int32;A A struct{X int32;B B;C C struct{Z string}};C C}"; got != want {
		t.Errorf(`B String got %q, want %q`, got, want)
	}
	if got, want := b.NumField(), 3; got != want {
		t.Errorf(`B NumField got %q, want %q`, got, want)
	}
	if got, want := b.Field(0).Name, "Y"; got != want {
		t.Errorf(`B Field(0).Name got %q, want %q`, got, want)
	}
	if got, want := b.Field(0).Type, Int32Type; got != want {
		t.Errorf(`B Field(0).Type got %q, want %q`, got, want)
	}
	if got, want := b.Field(1).Name, "A"; got != want {
		t.Errorf(`B Field(1).Name got %q, want %q`, got, want)
	}
	if got, want := b.Field(1).Type, a; got != want {
		t.Errorf(`B Field(1).Type got %q, want %q`, got, want)
	}
	if got, want := b.Field(2).Name, "C"; got != want {
		t.Errorf(`B Field(2).Name got %q, want %q`, got, want)
	}
	if got, want := b.Field(2).Type, c; got != want {
		t.Errorf(`B Field(2).Type got %q, want %q`, got, want)
	}
	// Check C
	if got, want := c.Kind(), Struct; got != want {
		t.Errorf(`C Kind got %s, want %s`, got, want)
	}
	if got, want := c.Name(), "C"; got != want {
		t.Errorf(`C Name got %q, want %q`, got, want)
	}
	if got, want := c.String(), "C struct{Z string}"; got != want {
		t.Errorf(`C String got %q, want %q`, got, want)
	}
	if got, want := c.NumField(), 1; got != want {
		t.Errorf(`C NumField got %q, want %q`, got, want)
	}
	if got, want := c.Field(0).Name, "Z"; got != want {
		t.Errorf(`C Field(0).Name got %q, want %q`, got, want)
	}
	if got, want := c.Field(0).Type, StringType; got != want {
		t.Errorf(`C Field(0).Type got %q, want %q`, got, want)
	}
	// Check hash-consing
	for iter := 0; iter < 5; iter++ {
		d2, derr, a2, aerr, b2, berr, c2, cerr := build()
		if derr != nil || aerr != nil || berr != nil || cerr != nil {
			t.Errorf(`build got (%q,%q,%q,%q), want nil`, derr, aerr, berr, cerr)
		}
		if got, want := d2, d; got != want {
			t.Errorf(`build got %q, want %q`, got, want)
		}
		if got, want := a2, a; got != want {
			t.Errorf(`build got %q, want %q`, got, want)
		}
		if got, want := b2, b; got != want {
			t.Errorf(`build got %q, want %q`, got, want)
		}
		if got, want := c2, c; got != want {
			t.Errorf(`build got %q, want %q`, got, want)
		}
	}
}

func TestUniqueTypeNames(t *testing.T) {
	var builder TypeBuilder
	var pending [2][]PendingType
	pending[0] = makeAllPending(&builder)
	pending[1] = makeAllPending(&builder)
	builder.Build()
	// The first pending types have no errors, but have nil types since the other
	// pending types fail to build.
	for _, p := range pending[0] {
		ty, err := p.Built()
		if ty != nil {
			t.Errorf(`built[0] got type %q, want nil`, ty)
		}
		if err != nil {
			t.Errorf(`built[0] got error %q, want nil`, err)
		}
	}
	// The second built types have non-unique name errors, and also nil types.
	for _, p := range pending[1] {
		ty, err := p.Built()
		if ty != nil {
			t.Errorf(`built[0] got type %q, want nil`, ty)
		}
		if got, want := fmt.Sprint(err), "duplicate type names"; !strings.Contains(got, want) {
			t.Errorf(`built[0] got error %s, want %s`, got, want)
		}
	}
}

func makeAllPending(builder *TypeBuilder) []PendingType {
	var ret []PendingType
	for _, test := range singletons {
		if test.k != Any && test.k != TypeVal {
			ret = append(ret, builder.Named("Named"+test.s).AssignBase(test.t))
		}
	}
	for _, test := range enums {
		if test.errstr == "" {
			base := builder.Enum()
			for _, l := range test.labels {
				base.AppendLabel(l)
			}
			ret = append(ret, builder.Named("Enum"+test.name).AssignBase(base))
		}
	}
	for _, test := range structs {
		if test.errstr == "" {
			base := builder.Struct()
			for _, f := range test.fields {
				base.AppendField(f.Name, f.Type)
			}
			ret = append(ret, builder.Named("Struct"+test.name).AssignBase(base))
		}
	}
	for _, test := range oneofs {
		if test.errstr == "" {
			base := builder.OneOf()
			for _, t := range test.types {
				base.AppendType(t)
			}
			ret = append(ret, builder.Named("OneOf"+test.name).AssignBase(base))
		}
	}
	return ret
}
