package val

import (
	"fmt"
	"strings"
	"testing"

	testutil "veyron/lib/testutil"
)

var (
	anyT     = AnyType("")
	boolT    = BoolType("")
	intT     = IntType("")
	uintT    = UintType("")
	floatT   = FloatType("")
	complexT = ComplexType("")
	stringT  = StringType("")
	bytesT   = BytesType("")
	typevalT = TypeValType("")
)

var singletons = []struct {
	k Kind
	t *Type
	s string
}{
	{Any, anyT, "any"},
	{Bool, boolT, "bool"},
	{Int, intT, "int"},
	{Uint, uintT, "uint"},
	{Float, floatT, "float"},
	{Complex, complexT, "complex"},
	{String, stringT, "string"},
	{Bytes, bytesT, "bytes"},
	{TypeVal, typevalT, "typeval"},
}

type l []string

var enums = []struct {
	name   string
	labels l
	str    string
	errstr string
}{
	{"FailNoLabels", l{}, "", "no Enum labels"},
	{"FailEmptyLabel", l{""}, "", "empty Enum label"},
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
	{"FailFieldName", f{{"", boolT}}, "", "empty Struct field name"},
	{"FailDupFields", f{{"A", boolT}, {"A", intT}}, "", "duplicate field name"},
	{"FailNilFieldType", f{{"A", nil}}, "", "nil Struct field type"},
	{"", f{}, "struct{}", ""},
	{"Empty", f{}, "Empty struct{}", ""},
	{"A", f{{"A", boolT}}, "A struct{A bool}", ""},
	{"AB", f{{"A", boolT}, {"B", intT}}, "AB struct{A bool;B int}", ""},
	{"ABC", f{{"A", boolT}, {"B", intT}, {"C", uintT}}, "ABC struct{A bool;B int;C uint}", ""},
	{"ABCD", f{{"A", boolT}, {"B", intT}, {"C", uintT}, {"D", stringT}}, "ABCD struct{A bool;B int;C uint;D string}", ""},
}

type t []*Type

var oneofs = []struct {
	name   string
	types  t
	str    string
	errstr string
}{
	{"FailNoTypes", t{}, "", "no OneOf types"},
	{"FailAny", t{anyT}, "", "type in OneOf must not be nil, OneOf or Any"},
	{"FailDup", t{boolT, boolT}, "", "duplicate type"},
	{"", t{boolT}, "oneof{bool}", ""},
	{"A", t{boolT}, "A oneof{bool}", ""},
	{"AB", t{boolT, intT}, "AB oneof{bool;int}", ""},
	{"ABC", t{boolT, intT, uintT}, "ABC oneof{bool;int;uint}", ""},
	{"ABCD", t{boolT, intT, uintT, stringT}, "ABCD oneof{bool;int;uint;string}", ""},
}

func allTypes() (types []*Type) {
	for _, test := range singletons {
		types = append(types, test.t, ListType("List"+test.s, test.t))
		for _, test2 := range singletons {
			types = append(types, MapType("Map"+test.s+test2.s, test.t, test2.t))
		}
	}
	for _, test := range enums {
		if test.errstr == "" {
			types = append(types, EnumType(test.name, []string(test.labels)))
		}
	}
	for _, test := range structs {
		if test.errstr == "" {
			types = append(types, StructType(test.name, []StructField(test.fields)))
		}
	}
	for _, test := range oneofs {
		if test.errstr == "" {
			types = append(types, OneOfType(test.name, []*Type(test.types)))
		}
	}
	return
}

func expectPanic(t *testing.T, f func(), wantstr string, format string, args ...interface{}) {
	got := testutil.CallAndRecover(f)
	gotstr := fmt.Sprint(got)
	msg := fmt.Sprintf(format, args...)
	if wantstr != "" && !strings.Contains(gotstr, wantstr) {
		t.Errorf(`%s got panic %q, want substr %q`, msg, gotstr, wantstr)
	}
	if wantstr == "" && got != nil {
		t.Errorf(`%s got panic %q, want nil`, msg, gotstr)
	}
}

func expectMismatchedKind(t *testing.T, f func()) {
	expectPanic(t, f, "mismatched kind", "")
}

func TestTypeMismatch(t *testing.T) {
	// Make sure we panic if a method is called for a mismatched kind.
	for _, ty := range allTypes() {
		if ty.Kind() != Enum {
			expectMismatchedKind(t, func() { ty.EnumLabel(0) })
			expectMismatchedKind(t, func() { ty.EnumIndex("") })
			expectMismatchedKind(t, func() { ty.NumEnumLabel() })
		}
		if ty.Kind() != List && ty.Kind() != Map {
			expectMismatchedKind(t, func() { ty.Elem() })
		}
		if ty.Kind() != Map {
			expectMismatchedKind(t, func() { ty.Key() })
		}
		if ty.Kind() != Struct {
			expectMismatchedKind(t, func() { ty.Field(0) })
			expectMismatchedKind(t, func() { ty.FieldByName("") })
			expectMismatchedKind(t, func() { ty.NumField() })
		}
		if ty.Kind() != OneOf {
			expectMismatchedKind(t, func() { ty.OneOfType(0) })
			expectMismatchedKind(t, func() { ty.OneOfIndex(anyT) })
			expectMismatchedKind(t, func() { ty.NumOneOfType() })
		}
	}
}

func TestSingletonTypes(t *testing.T) {
	for _, test := range singletons {
		if got, want := test.t.Kind(), test.k; got != want {
			t.Errorf(`%s got kind %q, want %q`, test.k, got, want)
		}
		if got, want := test.t.Name(), ""; got != want {
			t.Errorf(`%s got name %q, want %q`, test.k, got, want)
		}
		if got, want := test.k.String(), test.s; got != want {
			t.Errorf(`%s got kind %q, want %q`, test.k, got, want)
		}
		if got, want := test.t.String(), test.s; got != want {
			t.Errorf(`%s got string %q, want %q`, test.k, got, want)
		}
	}
}

func TestEnumTypes(t *testing.T) {
	for _, test := range enums {
		var x *Type
		create := func() { x = EnumType(test.name, []string(test.labels)) }
		expectPanic(t, create, test.errstr, "%s EnumOf", test.name)
		if x == nil {
			continue
		}
		if got, want := x.Kind(), Enum; got != want {
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

func TestListTypes(t *testing.T) {
	for _, test := range singletons {
		name := "List" + test.s
		x := ListType(name, test.t)
		if got, want := x.Kind(), List; got != want {
			t.Errorf(`List %s got kind %q, want %q`, test.k, got, want)
		}
		if got, want := x.Name(), name; got != want {
			t.Errorf(`List %s got name %q, want %q`, test.k, got, want)
		}
		if got, want := x.String(), name+" []"+test.s; got != want {
			t.Errorf(`List %s got string %q, want %q`, test.k, got, want)
		}
		if got, want := x.Elem(), test.t; got != want {
			t.Errorf(`List %s got elem %q, want %q`, test.k, got, want)
		}
	}
}

func TestMapTypes(t *testing.T) {
	for _, key := range singletons {
		for _, elem := range singletons {
			name := "Map" + key.s + elem.s
			x := MapType(name, key.t, elem.t)
			if got, want := x.Kind(), Map; got != want {
				t.Errorf(`Map[%s]%s got kind %q, want %q`, key.k, elem.k, got, want)
			}
			if got, want := x.Name(), name; got != want {
				t.Errorf(`Map[%s]%s got name %q, want %q`, key.k, elem.k, got, want)
			}
			if got, want := x.String(), fmt.Sprintf("%s map[%s]%s", name, key.s, elem.s); got != want {
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
		create := func() { x = StructType(test.name, []StructField(test.fields)) }
		expectPanic(t, create, test.errstr, "%s StructOf", test.name)
		if x == nil {
			continue
		}
		if got, want := x.Kind(), Struct; got != want {
			t.Errorf(`Struct %s got kind %q, want %q`, test.name, got, want)
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
	A, B, C := boolT, intT, uintT
	x := StructType("X", []StructField{{"A", A}, {"B", B}, {"C", C}})
	for iter := 0; iter < 10; iter++ {
		abc := StructType("X", []StructField{{"A", A}, {"B", B}, {"C", C}})
		acb := StructType("X", []StructField{{"A", A}, {"C", C}, {"B", B}})
		bac := StructType("X", []StructField{{"B", B}, {"A", A}, {"C", C}})
		bca := StructType("X", []StructField{{"B", B}, {"C", C}, {"A", A}})
		cab := StructType("X", []StructField{{"C", C}, {"A", A}, {"B", B}})
		cba := StructType("X", []StructField{{"C", C}, {"B", B}, {"A", A}})
		if x != abc || x == acb || x == bac || x == bca || x == cab || x == cba {
			t.Errorf(`Struct ABC hash consing broken: %v, %v, %v, %v, %v, %v, %v`, x, abc, acb, bac, bca, cab, cba)
		}
		ac := StructType("X", []StructField{{"A", A}, {"C", C}})
		ca := StructType("X", []StructField{{"C", C}, {"A", A}})
		if x == ac || x == ca {
			t.Errorf(`Struct ABC / AC hash consing broken: %v, %v, %v`, x, ac, ca)
		}
	}
}

func TestOneOfTypes(t *testing.T) {
	for _, test := range oneofs {
		var x *Type
		create := func() { x = OneOfType(test.name, []*Type(test.types)) }
		expectPanic(t, create, test.errstr, "%s OneOfOf", test.name)
		if x == nil {
			continue
		}
		if got, want := x.Kind(), OneOf; got != want {
			t.Errorf(`OneOf %s got kind %q, want %q`, test.name, got, want)
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
	A, B, C := boolT, intT, uintT
	x := OneOfType("X", []*Type{A, B, C})
	for iter := 0; iter < 10; iter++ {
		abc := OneOfType("X", []*Type{A, B, C})
		acb := OneOfType("X", []*Type{A, C, B})
		bac := OneOfType("X", []*Type{B, A, C})
		bca := OneOfType("X", []*Type{B, C, A})
		cab := OneOfType("X", []*Type{C, A, B})
		cba := OneOfType("X", []*Type{C, B, A})
		if x != abc || x == acb || x == bac || x == bca || x == cab || x == cba {
			t.Errorf(`OneOf ABC hash consing broken: %v, %v, %v, %v, %v, %v, %v`, x, abc, acb, bac, bca, cab, cba)
		}
		ac := OneOfType("X", []*Type{A, C})
		ca := OneOfType("X", []*Type{C, A})
		if x == ac || x == ca {
			t.Errorf(`OneOf ABC / AC hash consing broken: %v, %v, %v`, x, ac, ca)
		}
	}
	// A more complicated scenario for duplicate type errors.  Here Y1 and Y2 are
	// represented by different builder instances building the same struct, which
	// are added to Z.  We want Z to detect Y1 and Y2 are dups, even though
	// they're not pointer equal (since they haven't been hash-consed yet).
	buildY1 := BuildStructType("Y").AppendField("A", intT)
	buildY2 := BuildStructType("Y").AppendField("A", intT)
	buildZ := BuildOneOfType("Z").AppendType(buildY1).AppendType(buildY2)
	z, zerr := buildZ.Build()
	if got, want := fmt.Sprint(zerr), "duplicate type"; !strings.Contains(got, want) {
		t.Errorf(`build Z got error %q, want substr %q`, got, want)
	}
	if z != nil {
		t.Errorf(`build Z got %v, want nil`, z)
	}
	y1, y1err := buildY1.Build()
	y2, y2err := buildY2.Build()
	if y1err != nil || y2err != nil {
		t.Errorf(`build got y1err %q y2err %q, want nil`, y1err, y2err)
	}
	if y1 != y2 {
		t.Errorf(`build got Y1 %s != Y2 %s`, y1, y2)
	}
}

func TestHashConsTypes(t *testing.T) {
	// Create a bunch of distinct types multiple times.
	var types [3][]*Type
	for iter := 0; iter < 3; iter++ {
		for _, a := range singletons {
			types[iter] = append(types[iter], ListType("List"+a.s, a.t))
			for _, b := range singletons {
				lA, lB := "A"+a.s, "B"+b.s
				types[iter] = append(types[iter], EnumType("Enum"+lA+lB, []string{lA, lB}))
				types[iter] = append(types[iter], MapType("Map"+a.s+b.s, a.t, b.t))
				types[iter] = append(types[iter], StructType("Struct"+lA+lB, []StructField{{lA, a.t}, {lB, b.t}}))
				if a.t != b.t && a.k != Any && b.k != Any {
					types[iter] = append(types[iter], OneOfType("OneOf"+lA+lB, []*Type{a.t, b.t}))
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
		{boolT, boolT, true},
		{anyT, boolT, true},
		{OneOfType("U", []*Type{boolT}), boolT, true},
		{OneOfType("U", []*Type{boolT, intT}), intT, true},

		{boolT, intT, false},
		{boolT, anyT, false},
		{boolT, OneOfType("U", []*Type{boolT}), false},
		{boolT, OneOfType("U", []*Type{boolT, intT}), false},
		{OneOfType("U", []*Type{boolT}), stringT, false},
		{OneOfType("U", []*Type{boolT, intT}), stringT, false},
	}
	for _, test := range tests {
		if test.t.AssignableFrom(test.f) != test.expect {
			t.Errorf(`%v.AssignableFrom(%v) expect %v`, test.t, test.f, test.expect)
		}
	}
}

// TODO(toddw): Add tests for ValidMapKey, ValidOneOfType

func TestSelfRecursiveType(t *testing.T) {
	buildTree := func() (*Type, error, *Type, error) {
		// type Node struct {
		//   Val      string
		//   Children []Node
		// }
		buildN := BuildStructType("Node")
		buildC := BuildListType("").SetElem(buildN)
		buildN.AppendField("Val", stringT)
		buildN.AppendField("Children", buildC)
		c, cerr := buildC.Build()
		n, nerr := buildN.Build()
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
	if got, want := n.Field(0).Type, stringT; got != want {
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
	build := func() (*Type, error, *Type, error) {
		// type A struct{X int;B B}
		// type B struct{Y int;A A}
		buildA, buildB := BuildStructType("A"), BuildStructType("B")
		buildA.AppendField("X", intT).AppendField("B", buildB)
		buildB.AppendField("Y", intT).AppendField("A", buildA)
		a, aerr := buildA.Build()
		b, berr := buildB.Build()
		return a, aerr, b, berr
	}
	a, aerr, b, berr := build()
	if aerr != nil || berr != nil {
		t.Errorf(`build got cerr %q nerr %q, want nil`, aerr, berr)
	}
	// Check A
	if got, want := a.Kind(), Struct; got != want {
		t.Errorf(`A Kind got %s, want %s`, got, want)
	}
	if got, want := a.Name(), "A"; got != want {
		t.Errorf(`A Name got %q, want %q`, got, want)
	}
	if got, want := a.String(), "A struct{X int;B B struct{Y int;A A}}"; got != want {
		t.Errorf(`A String got %q, want %q`, got, want)
	}
	if got, want := a.NumField(), 2; got != want {
		t.Errorf(`A NumField got %q, want %q`, got, want)
	}
	if got, want := a.Field(0).Name, "X"; got != want {
		t.Errorf(`A Field(0).Name got %q, want %q`, got, want)
	}
	if got, want := a.Field(0).Type, intT; got != want {
		t.Errorf(`A Field(0).Type got %q, want %q`, got, want)
	}
	if got, want := a.Field(1).Name, "B"; got != want {
		t.Errorf(`A Field(1).Name got %q, want %q`, got, want)
	}
	if got, want := a.Field(1).Type, b; got != want {
		t.Errorf(`A Field(1).Type got %q, want %q`, got, want)
	}
	// Check B
	if got, want := b.Kind(), Struct; got != want {
		t.Errorf(`B Kind got %s, want %s`, got, want)
	}
	if got, want := b.Name(), "B"; got != want {
		t.Errorf(`B Name got %q, want %q`, got, want)
	}
	if got, want := b.String(), "B struct{Y int;A A struct{X int;B B}}"; got != want {
		t.Errorf(`B String got %q, want %q`, got, want)
	}
	if got, want := b.NumField(), 2; got != want {
		t.Errorf(`B NumField got %q, want %q`, got, want)
	}
	if got, want := b.Field(0).Name, "Y"; got != want {
		t.Errorf(`B Field(0).Name got %q, want %q`, got, want)
	}
	if got, want := b.Field(0).Type, intT; got != want {
		t.Errorf(`B Field(0).Type got %q, want %q`, got, want)
	}
	if got, want := b.Field(1).Name, "A"; got != want {
		t.Errorf(`B Field(1).Name got %q, want %q`, got, want)
	}
	if got, want := b.Field(1).Type, a; got != want {
		t.Errorf(`B Field(1).Type got %q, want %q`, got, want)
	}
	// Check hash-consing
	for iter := 0; iter < 5; iter++ {
		a2, aerr2, b2, berr2 := build()
		if aerr2 != nil || berr2 != nil {
			t.Errorf(`build got aerr %q berr %q, want nil`, aerr, berr)
		}
		if got, want := a2, a; got != want {
			t.Errorf(`cons A got %q, want %q`, got, want)
		}
		if got, want := b2, b; got != want {
			t.Errorf(`cons B got %q, want %q`, got, want)
		}
	}
}
