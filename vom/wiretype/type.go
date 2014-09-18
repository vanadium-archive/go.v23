package wiretype

import (
	"fmt"
	"reflect"
	"strings"

	"veyron.io/veyron/veyron2/vdl/vdlutil"
	"veyron.io/veyron/veyron2/vom"
	"veyron.io/veyron/veyron2/wiretype"
	wiretype_build "veyron.io/veyron/veyron2/wiretype/build"
)

// Type represents the full type information needed to describe a specific type.
// TODO(bprosnitz) Look into replacing all VOM Types with a single representation.
type Type struct {
	Defs *wiretype_build.TypeDefs // Type definitions. Array index is the TypeID of the type.
	ID   wiretype.TypeID          // The TypeID of this type, as an index into Defs.
}

func (t Type) wiretype() vdlutil.Any {
	if t.ID >= wiretype.TypeIDFirst {
		return (*t.Defs)[t.ID-wiretype.TypeIDFirst]
	}

	for i := 0; i < int(wiretype.TypeIDFirst); i++ {
		if wiretype.BootstrapTypes[i].TID == t.ID {
			return wiretype.BootstrapTypes[i].WT
		}
	}

	panic(fmt.Sprintf("matching type not found for TypeID %v", t.ID))
}

func (t Type) derivedType(tid wiretype.TypeID) Type {
	return Type{
		Defs: t.Defs,
		ID:   tid,
	}
}

func (t Type) Align() int {
	panic("not yet implemented")
}

func (t Type) Len() int {
	switch wt := t.wiretype().(type) {
	case wiretype.ArrayType:
		return int(wt.Len)
	default:
		panic(fmt.Sprintf("invalid kind for Len(): %v", t.Kind()))
	}
}

func (t Type) Elem() vom.Type {
	switch wt := t.wiretype().(type) {
	case wiretype.ArrayType:
		return t.derivedType(wt.Elem)
	case wiretype.SliceType:
		return t.derivedType(wt.Elem)
	case wiretype.MapType:
		return t.derivedType(wt.Elem)
	case wiretype.PtrType:
		return t.derivedType(wt.Elem)
	default:
		panic(fmt.Sprintf("invalid kind for Elem(): %v", t.Kind()))
	}
}

func (t Type) Key() vom.Type {
	switch wt := t.wiretype().(type) {
	case wiretype.MapType:
		return t.derivedType(wt.Key)
	default:
		panic(fmt.Sprintf("invalid kind for Key(): %v", t.Kind()))
	}
}

func (t Type) Kind() reflect.Kind {
	switch wt := t.wiretype().(type) {
	case wiretype.NamedPrimitiveType:
		switch wt.Type {
		case wiretype.TypeIDInterface:
			return reflect.Interface
		case wiretype.TypeIDBool:
			return reflect.Bool
		case wiretype.TypeIDString:
			return reflect.String
		case wiretype.TypeIDTypeID:
			return reflect.TypeOf(wiretype.TypeID(0)).Kind()
		case wiretype.TypeIDFloat32:
			return reflect.Float32
		case wiretype.TypeIDFloat64:
			return reflect.Float64
		case wiretype.TypeIDInt:
			return reflect.Int
		case wiretype.TypeIDInt8:
			return reflect.Int8
		case wiretype.TypeIDInt16:
			return reflect.Int16
		case wiretype.TypeIDInt32:
			return reflect.Int32
		case wiretype.TypeIDInt64:
			return reflect.Int64
		case wiretype.TypeIDUint:
			return reflect.Uint
		case wiretype.TypeIDUint8:
			return reflect.Uint8
		case wiretype.TypeIDUint16:
			return reflect.Uint16
		case wiretype.TypeIDUint32:
			return reflect.Uint32
		case wiretype.TypeIDUint64:
			return reflect.Uint64
		case wiretype.TypeIDUintptr:
			return reflect.Uintptr
		default:
			panic(fmt.Sprintf("unkown primitive TypeID: %v", wt.Type))
		}
	case wiretype.SliceType:
		return reflect.Slice
	case wiretype.ArrayType:
		return reflect.Array
	case wiretype.MapType:
		return reflect.Map
	case wiretype.StructType:
		return reflect.Struct
	case wiretype.PtrType:
		return reflect.Ptr
	default:
		panic(fmt.Sprintf("unknown wiretype: %v", reflect.TypeOf(t.wiretype())))
	}
}

func (t Type) ConvertibleTo(other vom.Type) bool {
	return false // Not implemented
}

func (t Type) AssignableTo(other vom.Type) bool {
	return false // Not implemented
}

func (t Type) NumMethod() int {
	return 0 // Not implemented
}

func (t Type) NumField() int {
	switch wt := t.wiretype().(type) {
	case wiretype.StructType:
		return len(wt.Fields)
	default:
		panic(fmt.Sprintf("invalid kind for NumField(): %v", t.Kind()))
	}
}

func (t Type) Field(i int) vom.StructField {
	switch wt := t.wiretype().(type) {
	case wiretype.StructType:
		fld := wt.Fields[i]
		splt := strings.Split(fld.Name, ".")
		return vom.StructField{
			Name:    splt[len(splt)-1],
			PkgPath: strings.Join(splt[:len(splt)-1], "."),
			Type:    t.derivedType(wt.Fields[i].Type),
		}
	default:
		panic(fmt.Sprintf("invalid kind for NumField(): %v", t.Kind()))
	}
}

func (t Type) MethodByName(name string) (reflect.Method, bool) {
	return reflect.Method{}, false
}

func (t Type) nameField() string {
	switch wt := t.wiretype().(type) {
	case wiretype.NamedPrimitiveType:
		return wt.Name
	case wiretype.SliceType:
		return wt.Name
	case wiretype.ArrayType:
		return wt.Name
	case wiretype.MapType:
		return wt.Name
	case wiretype.StructType:
		return wt.Name
	case wiretype.PtrType:
		return wt.Name
	default:
		panic(fmt.Sprintf("unknown wiretype: %v", reflect.TypeOf(t.wiretype())))
	}
}

func (t Type) Name() string {
	splt := strings.Split(t.nameField(), ".")
	return splt[len(splt)-1]
}

func (t Type) PkgPath() string {
	splt := strings.Split(t.nameField(), ".")
	return strings.Join(splt[:len(splt)-1], ".")
}

func (t Type) String() string {
	if t.ID < wiretype.TypeIDFirst {
		return fmt.Sprintf("Type{%d, %q}", t.ID, t.Name())
	} else {

		return fmt.Sprintf("Type{%d, %#t}", t.ID, t.wiretype())
	}
}

// Implements vom.ReflectTypeConverter
func (t Type) ToReflectType() (reflect.Type, error) {
	switch t.Kind() {
	case reflect.Bool:
		return reflect.TypeOf(false), nil
	case reflect.Uint8:
		return reflect.TypeOf(uint8(0)), nil
	case reflect.Uint16:
		return reflect.TypeOf(uint16(0)), nil
	case reflect.Uint32:
		return reflect.TypeOf(uint32(0)), nil
	case reflect.Uint64:
		return reflect.TypeOf(uint64(0)), nil
	case reflect.Int8:
		return reflect.TypeOf(int8(0)), nil
	case reflect.Int16:
		return reflect.TypeOf(int16(0)), nil
	case reflect.Int32:
		return reflect.TypeOf(int32(0)), nil
	case reflect.Int64:
		return reflect.TypeOf(int64(0)), nil
	case reflect.Float32:
		return reflect.TypeOf(float32(0)), nil
	case reflect.Float64:
		return reflect.TypeOf(float64(0)), nil
	case reflect.String:
		return reflect.TypeOf(""), nil
	case reflect.Interface:
		switch t.Name() {
		case "interface", "anydata", "AnyData":
			return reflect.TypeOf([]interface{}{}), nil
		case "error":
			return reflect.TypeOf([]error{}), nil
		default:
			panic(fmt.Sprintf("conversions to interface '%v.%v' not yet supported", t.PkgPath(), t.Name()))
		}
	default:
		return nil, fmt.Errorf("won't convert non-primitive %v to reflect type", t.Kind())
	}
}
