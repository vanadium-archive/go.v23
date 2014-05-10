package build

import (
	"fmt"

	"veyron2/idl"
	"veyron2/wiretype"
	wiretype_build "veyron2/wiretype/build"
)

// wireTypeConverter converts IDL build types to wiretype IDs and maintains a slice of the type
// definitions needed to fully describe the type.
type wireTypeConverter struct {
	Defs wiretype_build.TypeDefs
}

func (wtc *wireTypeConverter) setWiretypeName(wt idl.AnyData, name string) idl.AnyData {
	switch t := wt.(type) {
	case wiretype.NamedPrimitiveType:
		t.Name = name
		return t
	case wiretype.SliceType:
		t.Name = name
		return t
	case wiretype.ArrayType:
		t.Name = name
		return t
	case wiretype.MapType:
		t.Name = name
		return t
	case wiretype.StructType:
		t.Name = name
		return t
	default:
		panic(fmt.Sprintf("got unexpected wiretype: %T", wt))
	}
}

func (wtc *wireTypeConverter) bootstrapTypeID(kind Kind) wiretype.TypeID {
	switch kind {
	case KindBool:
		return wiretype.TypeIDBool
	case KindByte:
		return wiretype.TypeIDUint8
	case KindUint32:
		return wiretype.TypeIDUint32
	case KindUint64:
		return wiretype.TypeIDUint64
	case KindInt32:
		return wiretype.TypeIDInt32
	case KindInt64:
		return wiretype.TypeIDInt64
	case KindFloat32:
		return wiretype.TypeIDFloat32
	case KindFloat64:
		return wiretype.TypeIDFloat64
	case KindComplex64:
		return wiretype.TypeIDComplex64
	case KindComplex128:
		return wiretype.TypeIDComplex128
	case KindString:
		return wiretype.TypeIDString
	case KindError, KindAnyData:
		return wiretype.TypeIDInterface
	default:
		panic(fmt.Sprintf("unknown primitive kind: %v", kind))
	}
}

func (wtc *wireTypeConverter) wireType(typ Type) idl.AnyData {
	switch t := typ.(type) {
	case *NamedType:
		if t.Def().Kind.IsPrimitive() || t.Def().Kind == KindError || t.Def().Kind == KindAnyData {
			return wiretype.NamedPrimitiveType{
				Type: wtc.bootstrapTypeID(typ.Def().Kind),
				Name: t.Name(),
			}
		} else {
			return wtc.setWiretypeName(wtc.wireType(t.Def().Base), t.Name())
		}
	case *ArrayType:
		return wiretype.ArrayType{
			Elem: wtc.WireTypeID(t.Elem),
			Len:  uint64(t.Len),
		}
	case *SliceType:
		return wiretype.SliceType{
			Elem: wtc.WireTypeID(t.Elem),
		}
	case *MapType:
		return wiretype.MapType{
			Key:  wtc.WireTypeID(t.Key),
			Elem: wtc.WireTypeID(t.Elem),
		}
	case *InterfaceType:
		return wiretype.NamedPrimitiveType{
			Type: wiretype.TypeIDInterface,
		}
	case *StructType:
		flds := make([]wiretype.FieldType, len(t.Fields))
		for i, fld := range t.Fields {
			flds[i].Name = fld.Name
			flds[i].Type = wtc.WireTypeID(fld.Type)
		}
		return wiretype.StructType{
			Fields: flds,
		}
	case TypeDefType:
		return wtc.wireType(t.def.Base)
	default:
		panic(fmt.Sprintf("unknown type %T", typ))
	}
}

// WireTypeID gets the TypeID of the given IDL build type and stores the needed type definitions
// in the wireTypeConverter.
func (wtc *wireTypeConverter) WireTypeID(typ Type) wiretype.TypeID {
	wt := wtc.wireType(typ)
	var tid wiretype.TypeID
	wtc.Defs, tid = wtc.Defs.Put(wt)
	return tid
}
