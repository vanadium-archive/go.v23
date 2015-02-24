package golang

import (
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vdl/compile"
)

func TestConst(t *testing.T) {
	testingMode = true
	tests := []struct {
		Name string
		V    *vdl.Value
		Want string
	}{
		{"True", vdl.BoolValue(true), `true`},
		{"False", vdl.BoolValue(false), `false`},
		{"String", vdl.StringValue("abc"), `"abc"`},
		{"Bytes", vdl.BytesValue([]byte("abc")), `[]byte("abc")`},
		{"Byte", vdl.ByteValue(111), `byte(111)`},
		{"Uint16", vdl.Uint16Value(222), `uint16(222)`},
		{"Uint32", vdl.Uint32Value(333), `uint32(333)`},
		{"Uint64", vdl.Uint64Value(444), `uint64(444)`},
		{"Int16", vdl.Int16Value(-555), `int16(-555)`},
		{"Int32", vdl.Int32Value(-666), `int32(-666)`},
		{"Int64", vdl.Int64Value(-777), `int64(-777)`},
		{"Float32", vdl.Float32Value(1.5), `float32(1.5)`},
		{"Float64", vdl.Float64Value(2.5), `float64(2.5)`},
		{"Complex64", vdl.Complex64Value(1 + 2i), `complex64(1+2i)`},
		{"Complex128", vdl.Complex128Value(3 + 4i), `complex128(3+4i)`},
		{"Enum", vdl.ZeroValue(tEnum).AssignEnumLabel("B"), `TestEnumB`},
		{"EmptyArray", vEmptyArray, "[3]string{}"},
		{"EmptyList", vEmptyList, "[]string(nil)"},
		{"EmptySet", vEmptySet, "map[string]struct{}(nil)"},
		{"EmptyMap", vEmptyMap, "map[string]int64(nil)"},
		{"EmptyStruct", vEmptyStruct, "TestStruct{}"},
		{"Array", vArray, `[3]string{
"A",
"B",
"C",
}`},
		{"List", vList, `[]string{
"A",
"B",
"C",
}`},
		{"Set", vSet, `map[string]struct{}{
"A": struct{}{},
}`},
		{"Map", vMap, `map[string]int64{
"A": 1,
}`},
		{"Struct", vStruct, `TestStruct{
A: "foo",
B: 123,
}`},
		{"UnionABC", vUnionABC, `TestUnion(TestUnionA{"abc"})`},
		{"Union123", vUnion123, `TestUnion(TestUnionB{int64(123)})`},
		{"AnyABC", vAnyABC, `vdl.ValueOf("abc")`},
		{"Any123", vAny123, `vdl.ValueOf(int64(123))`},
		{"TypeObjectBool", vdl.TypeObjectValue(vdl.BoolType), `vdl.TypeOf(false)`},
		{"TypeObjectString", vdl.TypeObjectValue(vdl.StringType), `vdl.TypeOf("")`},
		{"TypeObjectBytes", vdl.TypeObjectValue(vdl.ListType(vdl.ByteType)), `vdl.TypeOf([]byte(nil))`},
		{"TypeObjectByte", vdl.TypeObjectValue(vdl.ByteType), `vdl.TypeOf(byte(0))`},
		{"TypeObjectUint16", vdl.TypeObjectValue(vdl.Uint16Type), `vdl.TypeOf(uint16(0))`},
		{"TypeObjectInt16", vdl.TypeObjectValue(vdl.Int16Type), `vdl.TypeOf(int16(0))`},
		{"TypeObjectFloat32", vdl.TypeObjectValue(vdl.Float32Type), `vdl.TypeOf(float32(0))`},
		{"TypeObjectComplex64", vdl.TypeObjectValue(vdl.Complex64Type), `vdl.TypeOf(complex64(0))`},
		{"TypeObjectEnum", vdl.TypeObjectValue(tEnum), `vdl.TypeOf(TestEnumA)`},
		{"TypeObjectArray", vdl.TypeObjectValue(tArray), `vdl.TypeOf([3]string{})`},
		{"TypeObjectList", vdl.TypeObjectValue(tList), `vdl.TypeOf([]string(nil))`},
		{"TypeObjectSet", vdl.TypeObjectValue(tSet), `vdl.TypeOf(map[string]struct{}(nil))`},
		{"TypeObjectMap", vdl.TypeObjectValue(tMap), `vdl.TypeOf(map[string]int64(nil))`},
		{"TypeObjectStruct", vdl.TypeObjectValue(tStruct), `vdl.TypeOf(TestStruct{})`},
		{"TypeObjectUnion", vdl.TypeObjectValue(tUnion), `vdl.TypeOf(TestUnion(TestUnionA{""}))`},
		{"TypeObjectAny", vdl.TypeObjectValue(vdl.AnyType), `vdl.AnyType`},
		{"TypeObjectTypeObject", vdl.TypeObjectValue(vdl.TypeObjectType), `vdl.TypeObjectType`},
		// TODO(toddw): Add tests for optional types.
	}
	data := goData{Env: compile.NewEnv(-1)}
	for _, test := range tests {
		if got, want := typedConst(data, test.V), test.Want; got != want {
			t.Errorf("%s\n GOT %s\nWANT %s", test.Name, got, want)
		}
	}
}

var (
	vEmptyArray  = vdl.ZeroValue(tArray)
	vEmptyList   = vdl.ZeroValue(tList)
	vEmptySet    = vdl.ZeroValue(tSet)
	vEmptyMap    = vdl.ZeroValue(tMap)
	vEmptyStruct = vdl.ZeroValue(tStruct)

	vArray    = vdl.ZeroValue(tArray)
	vList     = vdl.ZeroValue(tList)
	vSet      = vdl.ZeroValue(tSet)
	vMap      = vdl.ZeroValue(tMap)
	vStruct   = vdl.ZeroValue(tStruct)
	vUnionABC = vdl.ZeroValue(tUnion)
	vUnion123 = vdl.ZeroValue(tUnion)
	vAnyABC   = vdl.ZeroValue(vdl.AnyType)
	vAny123   = vdl.ZeroValue(vdl.AnyType)
)

func init() {
	vArray.Index(0).AssignString("A")
	vArray.Index(1).AssignString("B")
	vArray.Index(2).AssignString("C")
	vList.AssignLen(3)
	vList.Index(0).AssignString("A")
	vList.Index(1).AssignString("B")
	vList.Index(2).AssignString("C")
	// TODO(toddw): Assign more items once the ordering is fixed.
	vSet.AssignSetKey(vdl.StringValue("A"))
	vMap.AssignMapIndex(vdl.StringValue("A"), vdl.Int64Value(1))

	vStruct.StructField(0).AssignString("foo")
	vStruct.StructField(1).AssignInt(123)

	vUnionABC.AssignUnionField(0, vdl.StringValue("abc"))
	vUnion123.AssignUnionField(1, vdl.Int64Value(123))

	vAnyABC.Assign(vdl.StringValue("abc"))
	vAny123.Assign(vdl.Int64Value(123))
}
