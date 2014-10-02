package golang

import (
	"testing"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vdl/compile"
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
		// TODO(toddw): Add tests for any, oneof, nilable, typeval
	}
	data := goData{Env: compile.NewEnv(-1)}
	for _, test := range tests {
		if got, want := typedConst(data, test.V), test.Want; got != want {
			t.Errorf("%s\n GOT %s\nWANT %s", test.Name, got, want)
		}
	}
}

var (
	vArray  = vdl.ZeroValue(tArray)
	vList   = vdl.ZeroValue(tList)
	vSet    = vdl.ZeroValue(tSet)
	vMap    = vdl.ZeroValue(tMap)
	vStruct = vdl.ZeroValue(tStruct)
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

	vStruct.Field(0).AssignString("foo")
	vStruct.Field(1).AssignInt(123)
}
