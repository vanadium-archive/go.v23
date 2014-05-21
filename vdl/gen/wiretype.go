package gen

// TODO(bprosnitz) Remove this after vom2 is implemented and we switch to the new signatures.

import (
	"fmt"

	"veyron2/val"
	"veyron2/vdl"
	"veyron2/wiretype"
	"veyron2/wiretype/build"
)

// wireTypeConverter converts VDL build types to wiretype IDs and maintains a slice of the type
// definitions needed to fully describe the type.
type wireTypeConverter struct {
	Defs build.TypeDefs
}

func (wtc *wireTypeConverter) bootstrapTypeID(kind val.Kind) wiretype.TypeID {
	switch kind {
	case val.Bool:
		return wiretype.TypeIDBool
	case val.Uint32:
		return wiretype.TypeIDUint32
	case val.Uint64:
		return wiretype.TypeIDUint64
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
	case val.Bytes:
		return wiretype.TypeIDByteSlice
	case val.String:
		return wiretype.TypeIDString
	case val.Any, val.OneOf:
		return wiretype.TypeIDInterface
	default:
		panic(fmt.Sprintf("unknown primitive kind: %v", kind))
	}
}

func (wtc *wireTypeConverter) wireType(typ *val.Type) vdl.Any {
	switch typ.Kind() {
	case val.List:
		return wiretype.SliceType{
			Elem: wtc.WireTypeID(typ.Elem()),
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
	default:
		wt := wiretype.NamedPrimitiveType{
			Type: wtc.bootstrapTypeID(typ.Kind()),
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
