package build

import (
	"reflect"
	"testing"

	"veyron2/idl"
	"veyron2/wiretype"
	wiretype_build "veyron2/wiretype/build"
)

// Tests converting IDL build type descriptions to wiretype descriptions.
func TestWireTypeID(t *testing.T) {
	// Types to use in test cases:
	type customIntType int64
	type customSliceType []int64
	type customArrayType [1]int64
	type customMapType map[string]int64
	type customStructType struct {
		X uint64
	}
	type anydataStruct struct {
		AD idl.AnyData
	}
	type errorStruct struct {
		Err error
	}

	tests := []struct {
		input  []Type
		output []wiretype.TypeID
		defs   wiretype_build.TypeDefs
	}{
		{
			[]Type{
				ReflectTypeToIDLType(reflect.TypeOf("")),
			},
			[]wiretype.TypeID{
				wiretype.TypeIDString,
			},
			wiretype_build.TypeDefs{},
		},
		{
			[]Type{
				ReflectTypeToIDLType(reflect.TypeOf(uint8(0))),
			},
			[]wiretype.TypeID{
				wiretype.TypeIDUint8,
			},
			wiretype_build.TypeDefs{},
		},
		{
			[]Type{
				ReflectTypeToIDLType(reflect.TypeOf([]byte{})),
			},
			[]wiretype.TypeID{
				wiretype.TypeIDByteSlice,
			},
			wiretype_build.TypeDefs{},
		},
		{
			[]Type{
				ReflectTypeToIDLType(reflect.TypeOf([]string{})),
			},
			[]wiretype.TypeID{
				wiretype.TypeIDStringSlice,
			},
			wiretype_build.TypeDefs{},
		},

		// Test WireType types:
		{
			[]Type{
				ReflectTypeToIDLType(reflect.TypeOf(wiretype.TypeID(4))),
			},
			[]wiretype.TypeID{
				wiretype.TypeIDTypeID,
			},
			wiretype_build.TypeDefs{},
		},

		{
			[]Type{
				ReflectTypeToIDLType(reflect.TypeOf(wiretype.NamedPrimitiveType{})),
			},
			[]wiretype.TypeID{
				wiretype.TypeIDNamedPrimitiveType,
			},
			wiretype_build.TypeDefs{},
		},
		{
			[]Type{
				ReflectTypeToIDLType(reflect.TypeOf(wiretype.SliceType{})),
			},
			[]wiretype.TypeID{
				wiretype.TypeIDSliceType,
			},
			wiretype_build.TypeDefs{},
		},
		{
			[]Type{
				ReflectTypeToIDLType(reflect.TypeOf(wiretype.ArrayType{})),
			},
			[]wiretype.TypeID{
				wiretype.TypeIDArrayType,
			},
			wiretype_build.TypeDefs{},
		},
		{
			[]Type{
				ReflectTypeToIDLType(reflect.TypeOf(wiretype.MapType{})),
			},
			[]wiretype.TypeID{
				wiretype.TypeIDMapType,
			},
			wiretype_build.TypeDefs{},
		},
		{
			[]Type{
				ReflectTypeToIDLType(reflect.TypeOf(wiretype.FieldType{})),
			},
			[]wiretype.TypeID{
				wiretype.TypeIDFieldType,
			},
			wiretype_build.TypeDefs{},
		},
		{
			[]Type{
				ReflectTypeToIDLType(reflect.TypeOf(wiretype.StructType{})),
			},
			[]wiretype.TypeID{
				wiretype.TypeIDStructType,
			},
			wiretype_build.TypeDefs{},
		},

		// Test adding new types:
		{
			[]Type{
				ReflectTypeToIDLType(reflect.TypeOf(customIntType(0))),
			},
			[]wiretype.TypeID{
				wiretype.TypeIDFirst,
			},
			wiretype_build.TypeDefs{
				wiretype.NamedPrimitiveType{
					Type: wiretype.TypeIDInt64,
					Name: "veyron2/idl/build.customIntType",
				},
			},
		},
		{
			[]Type{
				ReflectTypeToIDLType(reflect.TypeOf(customSliceType{})),
			},
			[]wiretype.TypeID{
				wiretype.TypeIDFirst,
			},
			wiretype_build.TypeDefs{
				wiretype.SliceType{
					Elem: wiretype.TypeIDInt64,
					Name: "veyron2/idl/build.customSliceType",
				},
			},
		},
		{
			[]Type{
				ReflectTypeToIDLType(reflect.TypeOf(customArrayType{})),
			},
			[]wiretype.TypeID{
				wiretype.TypeIDFirst,
			},
			wiretype_build.TypeDefs{
				wiretype.ArrayType{
					Elem: wiretype.TypeIDInt64,
					Len:  1,
					Name: "veyron2/idl/build.customArrayType",
				},
			},
		},
		{
			[]Type{
				ReflectTypeToIDLType(reflect.TypeOf(customMapType{})),
			},
			[]wiretype.TypeID{
				wiretype.TypeIDFirst,
			},
			wiretype_build.TypeDefs{
				wiretype.MapType{
					Key:  wiretype.TypeIDString,
					Elem: wiretype.TypeIDInt64,
					Name: "veyron2/idl/build.customMapType",
				},
			},
		},
		{
			[]Type{
				ReflectTypeToIDLType(reflect.TypeOf(customStructType{})),
			},
			[]wiretype.TypeID{
				wiretype.TypeIDFirst,
			},
			wiretype_build.TypeDefs{
				wiretype.StructType{
					Fields: []wiretype.FieldType{
						wiretype.FieldType{
							Name: "X",
							Type: wiretype.TypeIDUint64,
						},
					},
					Name: "veyron2/idl/build.customStructType",
				},
			},
		},

		// Test common special types:
		{
			[]Type{
				ReflectTypeToIDLType(reflect.TypeOf(anydataStruct{})),
			},
			[]wiretype.TypeID{
				wiretype.TypeIDFirst + 1,
			},
			wiretype_build.TypeDefs{
				wiretype.NamedPrimitiveType{
					Type: wiretype.TypeIDInterface,
					Name: "veyron2/idl.AnyData",
				},
				wiretype.StructType{
					Name: "veyron2/idl/build.anydataStruct",
					Fields: []wiretype.FieldType{
						wiretype.FieldType{
							Name: "AD",
							Type: wiretype.TypeIDFirst,
						},
					},
				},
			},
		},
		{
			[]Type{
				ReflectTypeToIDLType(reflect.TypeOf(errorStruct{})),
			},
			[]wiretype.TypeID{
				wiretype.TypeIDFirst + 1,
			},
			wiretype_build.TypeDefs{
				wiretype.NamedPrimitiveType{
					Type: wiretype.TypeIDInterface,
					Name: "error",
				},
				wiretype.StructType{
					Name: "veyron2/idl/build.errorStruct",
					Fields: []wiretype.FieldType{
						wiretype.FieldType{
							Name: "Err",
							Type: wiretype.TypeIDFirst,
						},
					},
				},
			},
		},

		// Test that we only define types once when multiple copies are sent.
		{
			[]Type{
				ReflectTypeToIDLType(reflect.TypeOf(customIntType(9))),
				ReflectTypeToIDLType(reflect.TypeOf(uint64(4))),
				ReflectTypeToIDLType(reflect.TypeOf(customIntType(9))),
			},
			[]wiretype.TypeID{
				wiretype.TypeIDFirst,
				wiretype.TypeIDUint64,
				wiretype.TypeIDFirst,
			},
			wiretype_build.TypeDefs{
				wiretype.NamedPrimitiveType{
					Type: wiretype.TypeIDInt64,
					Name: "veyron2/idl/build.customIntType",
				},
			},
		},
	}

	for _, test := range tests {
		wtc := wireTypeConverter{}
		for i, typ := range test.input {
			if tid := wtc.WireTypeID(typ); tid != test.output[i] {
				t.Errorf("Test %v: Got tid %v. Expected %v.", test, tid, test.output[i])
			}
		}

		if len(wtc.Defs) != len(test.defs) {
			t.Errorf("Test %v: length of defs differed. Got %v. Expected %v.", test, wtc.Defs, test.defs)
			continue
		}

		for i, def := range wtc.Defs {
			if !reflect.DeepEqual(def, test.defs[i]) {
				t.Errorf("Test %v: Got def %v. Expected %v.", test, def, test.defs[i])
			}
		}
	}
}
