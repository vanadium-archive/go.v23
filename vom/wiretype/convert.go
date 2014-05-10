package wiretype

import (
	"fmt"
	"reflect"

	"veyron2/idl"
	"veyron2/vom"
	"veyron2/wiretype"
	wiretype_build "veyron2/wiretype/build"
)

// TypeConverter converts vom.Types to Type objects.
// This is currently used for tests, but may be useful elsewhere.
type TypeConverter struct {
	Defs wiretype_build.TypeDefs
}

func (tc *TypeConverter) getWireType(vt vom.Type) idl.AnyData {
	name := vt.Name()
	if vt.PkgPath() != "" {
		name = vt.PkgPath() + "." + vt.Name()
	}
	switch vt.Kind() {
	case reflect.Slice:
		return wiretype.SliceType{
			Elem: tc.fromVOMType(vt.Elem()),
			Name: name,
		}
	case reflect.Array:
		return wiretype.ArrayType{
			Elem: tc.fromVOMType(vt.Elem()),
			Len:  uint64(vt.Len()),
			Name: name,
		}
	case reflect.Map:
		return wiretype.MapType{
			Key:  tc.fromVOMType(vt.Key()),
			Elem: tc.fromVOMType(vt.Elem()),
			Name: name,
		}
	case reflect.Struct:
		flds := make([]wiretype.FieldType, vt.NumField())
		for i := 0; i < vt.NumField(); i++ {
			fld := vt.Field(i)
			flds[i].Name = fld.Name
			if fld.PkgPath != "" {
				flds[i].Name = fld.PkgPath + "." + fld.Name
			}
			flds[i].Type = tc.fromVOMType(fld.Type)
		}
		return wiretype.StructType{
			Fields: flds,
			Name:   name,
		}
	case reflect.Ptr:
		return wiretype.PtrType{
			Elem: tc.fromVOMType(vt.Elem()),
			Name: name,
		}
	default:
		var tid wiretype.TypeID
		switch vt.Kind() {
		case reflect.Interface:
			tid = wiretype.TypeIDInterface
		case reflect.Bool:
			tid = wiretype.TypeIDBool
		case reflect.String:
			tid = wiretype.TypeIDString
		case reflect.Float32:
			tid = wiretype.TypeIDFloat32
		case reflect.Float64:
			tid = wiretype.TypeIDFloat64
		case reflect.Int:
			tid = wiretype.TypeIDInt
		case reflect.Int8:
			tid = wiretype.TypeIDInt8
		case reflect.Int16:
			tid = wiretype.TypeIDInt16
		case reflect.Int32:
			tid = wiretype.TypeIDInt32
		case reflect.Int64:
			tid = wiretype.TypeIDInt64
		case reflect.Uintptr:
			tid = wiretype.TypeIDUintptr
		case reflect.Uint:
			tid = wiretype.TypeIDUint
		case reflect.Uint8:
			tid = wiretype.TypeIDUint8
		case reflect.Uint16:
			tid = wiretype.TypeIDUint16
		case reflect.Uint32:
			tid = wiretype.TypeIDUint32
		case reflect.Uint64:
			tid = wiretype.TypeIDUint64
		default:
			panic(fmt.Sprintf("unknown kind: %v", vt.Kind()))
		}

		if name == "" {
			name = tid.Name()
		}
		return wiretype.NamedPrimitiveType{
			Type: tid,
			Name: name,
		}
	}
}

func (tc *TypeConverter) fromVOMType(vt vom.Type) wiretype.TypeID {
	wt := tc.getWireType(vt)
	var tid wiretype.TypeID
	tc.Defs, tid = tc.Defs.Put(wt)
	return tid
}

// FromVOMType creates a Type from the specified VOM Type.
func (tc *TypeConverter) FromVOMType(vt vom.Type) Type {
	tid := tc.fromVOMType(vt)
	return Type{
		Defs: &tc.Defs,
		ID:   tid,
	}
}
