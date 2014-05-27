package gen

// TODO(bprosnitz) Remove this after vom2 is implemented and we switch to the new signatures.

import (
	"fmt"

	"veyron2/val"
	"veyron2/vdl"
	"veyron2/vdl/compile"
	"veyron2/wiretype"
	"veyron2/wiretype/build"
)

// wireTypeConverter converts VDL build types to wiretype IDs and maintains a slice of the type
// definitions needed to fully describe the type.
type wireTypeConverter struct {
	Defs build.TypeDefs
}

func (wtc *wireTypeConverter) bootstrapTypeID(t *val.Type) wiretype.TypeID {
	if t.Kind() == val.List && t.Elem().Kind() == val.Byte {
		return wiretype.TypeIDByteSlice
	}
	switch t.Kind() {
	case val.Bool:
		return wiretype.TypeIDBool
	case val.Byte:
		return wiretype.TypeIDUint8
	case val.Uint16:
		return wiretype.TypeIDUint16
	case val.Uint32:
		return wiretype.TypeIDUint32
	case val.Uint64:
		return wiretype.TypeIDUint64
	case val.Int16:
		return wiretype.TypeIDInt16
	case val.Int32:
		return wiretype.TypeIDInt32
	case val.Int64:
		return wiretype.TypeIDInt64
	case val.Float32:
		return wiretype.TypeIDFloat32
	case val.Float64:
		return wiretype.TypeIDFloat64
	case val.Complex64:
		return wiretype.TypeIDComplex64
	case val.Complex128:
		return wiretype.TypeIDComplex128
	case val.String:
		return wiretype.TypeIDString
	case val.Any, val.OneOf:
		return wiretype.TypeIDInterface
	case val.TypeVal:
		return wiretype.TypeIDTypeID
	default:
		panic(fmt.Sprintf("unknown primitive type: %v", t))
	}
}

func (wtc *wireTypeConverter) wireType(typ *val.Type) vdl.Any {
	if typ == compile.ErrorType {
		// Hack error as an interface for now, since that was the old behavior.
		return wiretype.NamedPrimitiveType{
			Type: wtc.bootstrapTypeID(val.AnyType),
			Name: "error",
		}
	}
	switch typ.Kind() {
	case val.Enum:
		// Hack enum as an int for now, this will all go away with vom2.
		return wiretype.NamedPrimitiveType{
			Type: wtc.bootstrapTypeID(val.Uint64Type),
			Name: typ.Name(),
		}
	case val.Array:
		return wiretype.ArrayType{
			Elem: wtc.WireTypeID(typ.Elem()),
			Len:  uint64(typ.Len()),
			Name: typ.Name(),
		}
	case val.List:
		return wiretype.SliceType{
			Elem: wtc.WireTypeID(typ.Elem()),
			Name: typ.Name(),
		}
	case val.Set:
		// Hack set as a map for now, this will all go away with vom2.
		return wiretype.MapType{
			Key:  wtc.WireTypeID(typ.Key()),
			Elem: wtc.WireTypeID(val.BoolType),
			Name: typ.Name(),
		}
	case val.Map:
		return wiretype.MapType{
			Key:  wtc.WireTypeID(typ.Key()),
			Elem: wtc.WireTypeID(typ.Elem()),
			Name: typ.Name(),
		}
	case val.Struct:
		flds := make([]wiretype.FieldType, typ.NumField())
		for i := 0; i < typ.NumField(); i++ {
			fld := typ.Field(i)
			flds[i].Name = fld.Name
			flds[i].Type = wtc.WireTypeID(fld.Type)
		}
		return wiretype.StructType{
			Fields: flds,
			Name:   typ.Name(),
		}
	case val.OneOf:
		// Hack oneof as a named any for now, this will all go away with vom2.
		return wiretype.NamedPrimitiveType{
			Type: wtc.bootstrapTypeID(val.AnyType),
			Name: typ.Name(),
		}
	default:
		wt := wiretype.NamedPrimitiveType{
			Type: wtc.bootstrapTypeID(typ),
			Name: typ.Name(),
		}
		if typ.Kind() == val.Any {
			wt.Name = "anydata"
		}
		if wt.Name == "" {
			wt.Name = wt.Type.Name()
		}
		return wt
	}
}

// WireTypeID gets the TypeID of the given VDL build type and stores the needed type definitions
// in the wireTypeConverter.
func (wtc *wireTypeConverter) WireTypeID(typ *val.Type) wiretype.TypeID {
	wt := wtc.wireType(typ)
	var tid wiretype.TypeID
	wtc.Defs, tid = wtc.Defs.Put(wt)
	return tid
}
