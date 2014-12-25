package golang

// TODO(bprosnitz) Remove this after vom2 is implemented and we switch to the new signatures.

import (
	"fmt"

	"v.io/veyron/veyron2/vdl"
	"v.io/veyron/veyron2/vdl/vdlutil"
	"v.io/veyron/veyron2/wiretype"
	"v.io/veyron/veyron2/wiretype/build"
)

// wireTypeConverter converts VDL build types to wiretype IDs and maintains a slice of the type
// definitions needed to fully describe the type.
type wireTypeConverter struct {
	Defs build.TypeDefs
}

func (wtc *wireTypeConverter) bootstrapTypeID(t *vdl.Type) wiretype.TypeID {
	if t.Kind() == vdl.List && t.Elem().Kind() == vdl.Byte {
		return wiretype.TypeIDByteSlice
	}
	switch t.Kind() {
	case vdl.Bool:
		return wiretype.TypeIDBool
	case vdl.Byte:
		return wiretype.TypeIDUint8
	case vdl.Uint16:
		return wiretype.TypeIDUint16
	case vdl.Uint32:
		return wiretype.TypeIDUint32
	case vdl.Uint64:
		return wiretype.TypeIDUint64
	case vdl.Int16:
		return wiretype.TypeIDInt16
	case vdl.Int32:
		return wiretype.TypeIDInt32
	case vdl.Int64:
		return wiretype.TypeIDInt64
	case vdl.Float32:
		return wiretype.TypeIDFloat32
	case vdl.Float64:
		return wiretype.TypeIDFloat64
	case vdl.Complex64:
		return wiretype.TypeIDComplex64
	case vdl.Complex128:
		return wiretype.TypeIDComplex128
	case vdl.String:
		return wiretype.TypeIDString
	case vdl.Any, vdl.Union:
		return wiretype.TypeIDInterface
	case vdl.TypeObject:
		return wiretype.TypeIDTypeID
	default:
		panic(fmt.Sprintf("unknown primitive type: %v", t))
	}
}

func (wtc *wireTypeConverter) wireType(typ *vdl.Type) vdlutil.Any {
	if typ == vdl.ErrorType {
		// Hack error as an interface for now, since that was the old behavior.
		return wiretype.NamedPrimitiveType{
			Type: wtc.bootstrapTypeID(vdl.AnyType),
			Name: "error",
		}
	}
	switch typ.Kind() {
	case vdl.Enum:
		// Hack enum as an int for now, this will all go away with vom2.
		return wiretype.NamedPrimitiveType{
			Type: wtc.bootstrapTypeID(vdl.Uint64Type),
			Name: typ.Name(),
		}
	case vdl.Array:
		return wiretype.ArrayType{
			Elem: wtc.WireTypeID(typ.Elem()),
			Len:  uint64(typ.Len()),
			Name: typ.Name(),
		}
	case vdl.List:
		return wiretype.SliceType{
			Elem: wtc.WireTypeID(typ.Elem()),
			Name: typ.Name(),
		}
	case vdl.Set:
		// Hack set as a map for now, this will all go away with vom2.
		return wiretype.MapType{
			Key:  wtc.WireTypeID(typ.Key()),
			Elem: wtc.WireTypeID(vdl.BoolType),
			Name: typ.Name(),
		}
	case vdl.Map:
		return wiretype.MapType{
			Key:  wtc.WireTypeID(typ.Key()),
			Elem: wtc.WireTypeID(typ.Elem()),
			Name: typ.Name(),
		}
	case vdl.Struct:
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
	case vdl.Union:
		// Hack union as a named any for now, this will all go away with vom2.
		return wiretype.NamedPrimitiveType{
			Type: wtc.bootstrapTypeID(vdl.AnyType),
			Name: typ.Name(),
		}
	default:
		wt := wiretype.NamedPrimitiveType{
			Type: wtc.bootstrapTypeID(typ),
			Name: typ.Name(),
		}
		if typ.Kind() == vdl.Any {
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
func (wtc *wireTypeConverter) WireTypeID(typ *vdl.Type) wiretype.TypeID {
	wt := wtc.wireType(typ)
	var tid wiretype.TypeID
	wtc.Defs, tid = wtc.Defs.Put(wt)
	return tid
}
