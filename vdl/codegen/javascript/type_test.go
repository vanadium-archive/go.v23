package javascript

import (
	"testing"

	"veyron.io/veyron/veyron2/vdl"
)

func TestType(t *testing.T) {
	tests := []struct {
		T    *vdl.Type
		Want string
	}{
		{vdl.AnyType, `Types.ANY`},
		{vdl.TypeObjectType, `{}`},
		{vdl.BoolType, `Types.BOOL`},
		{vdl.StringType, `Types.STRING`},
		{vdl.ByteType, `Types.BYTE`},
		{vdl.Uint16Type, `Types.UINT16`},
		{vdl.Uint32Type, `Types.UINT32`},
		{vdl.Uint64Type, `Types.UINT64`},
		{vdl.Int16Type, `Types.INT16`},
		{vdl.Int32Type, `Types.INT32`},
		{vdl.Int64Type, `Types.INT64`},
		{vdl.Float32Type, `Types.FLOAT32`},
		{vdl.Float64Type, `Types.FLOAT64`},
		{vdl.Complex64Type, `Types.COMPLEX64`},
		{vdl.Complex128Type, `Types.COMPLEX128`},
		{tNamedBool, `{kind: Kind.BOOL, name: 'NamedBool'}`},
		{tEnum,
			`{
    kind: Kind.ENUM,
    name: 'TestEnum',
    labels: ['A', 'B', 'C', ]
  }`},
		{tArray,
			`{
    kind: Kind.ARRAY,
    elem: Types.STRING,
    len: 3
  }`},
		{tNamedArray,
			`{
    kind: Kind.ARRAY,
    name: 'NamedArray',
    elem: Types.STRING,
    len: 3
  }`},
		{tList,
			`{
    kind: Kind.LIST,
    elem: Types.STRING
  }`},
		{tNamedList,
			`{
    kind: Kind.LIST,
    name: 'NamedList',
    elem: Types.STRING
  }`},

		{tSet,
			`{
    kind: Kind.SET,
    key: Types.STRING
  }`},
		{tNamedSet,
			`{
    kind: Kind.SET,
    name: 'NamedSet',
    key: Types.STRING
  }`},

		{tMap,
			`{
    kind: Kind.MAP,
    key: Types.STRING,
    elem: Types.INT64
  }`},
		{tNamedMap,
			`{
    kind: Kind.MAP,
    name: 'NamedMap',
    key: Types.STRING,
    elem: Types.INT64
  }`},

		{tStruct,
			`{
    kind: Kind.STRUCT,
    name: 'TestStruct',
    fields: [
    {
      name: 'a',
      type: Types.STRING
    },
    {
      name: 'b',
      type: Types.INT64
    },
  ]}`}, {tOneOf,
			`{
    kind: Kind.ONEOF,
    name: 'TestOneOf',
    types: [Types.STRING, Types.INT64, ]
  }`},
	}
	for _, test := range tests {
		if got, want := typeStruct(test.T), test.Want; got != want {
			t.Errorf("%s\nGOT \n%s\nWANT \n%s", test.T, got, want)
		}
	}
}

var (
	tEnum   = vdl.NamedType("TestEnum", vdl.EnumType("A", "B", "C"))
	tArray  = vdl.ArrayType(3, vdl.StringType)
	tList   = vdl.ListType(vdl.StringType)
	tSet    = vdl.SetType(vdl.StringType)
	tMap    = vdl.MapType(vdl.StringType, vdl.Int64Type)
	tStruct = vdl.NamedType("TestStruct", vdl.StructType(
		vdl.StructField{"A", vdl.StringType},
		vdl.StructField{"B", vdl.Int64Type},
	))
	tOneOf      = vdl.NamedType("TestOneOf", vdl.OneOfType(vdl.StringType, vdl.Int64Type))
	tNamedArray = vdl.NamedType("NamedArray", tArray)
	tNamedList  = vdl.NamedType("NamedList", tList)
	tNamedSet   = vdl.NamedType("NamedSet", tSet)
	tNamedMap   = vdl.NamedType("NamedMap", tMap)
	tNamedBool  = vdl.NamedType("NamedBool", vdl.BoolType)
)
