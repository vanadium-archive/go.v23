package wiretype

import (
	"fmt"
)

// Bootstrap TypeIDs, known by all clients.  This is the basis
// upon which all other types may be generated.  These ids cannot be changed
// without breaking protocol compatibility.
//
// Multiple type ids may map down to the same underlying coding kind; e.g. all
// the uint types are encoded identically, ditto for int and float types, and
// string and byteslices also.  We use separate type ids so that generic
// decoding into a nil interface can cheaply recover the type information.
//
// As an alternative we could have used MetaType with special tags to encode
// this information separately, but that would take more space in the wire
// encoding, and would also require more work when encoding and decoding.  These
// types seem reasonably common in various languages, so using special type ids
// seems worthwhile.  Coders in languages that don't have support for each
// variant may simply pick a single type representation for each kind.
//
// The gaps in ids are to allow future expansion of types in their logical
// place.  This is purely stylistic; the protocol doesn't care about the ranges.
//
// Each message starts with the TypeID encoded as a signed integer, where
// +typeid indicates a value follows, while -typeid indicates a typedef follows.
// Since the bootstrap types are never defined over the wire, the TypeID range
// [-64, 0] is unused; this corresponds to all odd-numbered ascii codes.  In
// addition we reserve a few TypeIDs which have an ascii representation useful
// for our json representation, or other future representations.
const (
	TypeIDInvalid TypeID = 0
	// Basic types
	TypeIDInterface TypeID = 1 // 0x2 == STX
	TypeIDBool      TypeID = 2 // 0x4 == EOT
	TypeIDString    TypeID = 3 // 0x6 == ACK
	TypeIDByteSlice TypeID = 4 // 0x8 == '\b'
	//              TypeID = 5 is 0xa == '\n', reserved.
	//              TypeID = 6 is 0xc == '\f', reserved.
	TypeIDTypeID TypeID = 7
	// Composite types, allowing users to define their own types.
	TypeIDNamedPrimitiveType TypeID = 8  // 0x10 == DLE
	TypeIDSliceType          TypeID = 9  // 0x12 == DC2
	TypeIDArrayType          TypeID = 10 // 0x14 == DC4
	TypeIDMapType            TypeID = 11 // 0x16 == SYN
	TypeIDStructType         TypeID = 12 // 0x18 == CAN
	TypeIDPtrType            TypeID = 13 // 0x1a == SUB
	TypeIDFieldType          TypeID = 14
	TypeIDFieldSliceType     TypeID = 15
	//               TypeID = 16 is 0x20 == ' ', reserved.
	//               TypeID = 20 is 0x28 == '(', reserved.
	// Float types
	TypeIDFloat32 TypeID = 25 // 0x32 == '1'
	TypeIDFloat64 TypeID = 26 // 0x34 == '2'
	//            TypeID = 30 is 0x3c == '<', reserved.
	//            TypeID = 31 is 0x3e == '>', reserved.
	// Int types
	//          TypeID = 32 is 0x40 == '@', reserved.
	TypeIDInt   TypeID = 33 // 0x42 == 'B'
	TypeIDInt8  TypeID = 34 // 0x44 == 'D'
	TypeIDInt16 TypeID = 35 // 0x46 == 'F'
	TypeIDInt32 TypeID = 36 // 0x48 == 'H'
	TypeIDInt64 TypeID = 37 // 0x4a == 'J'
	// Uint types
	TypeIDUintptr TypeID = 48 // 0x60 == '`'
	TypeIDUint    TypeID = 49 // 0x62 == 'b'
	TypeIDUint8   TypeID = 50 // 0x64 == 'd'
	TypeIDUint16  TypeID = 51 // 0x66 == 'f'
	TypeIDUint32  TypeID = 52 // 0x68 == 'h'
	TypeIDUint64  TypeID = 53 // 0x6a == 'j'
	// Complex types
	TypeIDComplex64  TypeID = 56
	TypeIDComplex128 TypeID = 57
	// Additional types:
	TypeIDStringSlice TypeID = 61
	// The first TypeID we assign to user types.
	TypeIDFirst TypeID = 65
)

func (id TypeID) Name() string {
	switch id {
	case TypeIDInterface:
		return "interface"
	case TypeIDBool:
		return "bool"
	case TypeIDString:
		return "string"
	case TypeIDByteSlice:
		return "[]byte"
	case TypeIDTypeID:
		return "TypeID"
	case TypeIDFloat32:
		return "float32"
	case TypeIDFloat64:
		return "float64"
	case TypeIDInt:
		return "int"
	case TypeIDInt8:
		return "int8"
	case TypeIDInt16:
		return "int16"
	case TypeIDInt32:
		return "int32"
	case TypeIDInt64:
		return "int64"
	case TypeIDUintptr:
		return "uintptr"
	case TypeIDUint:
		return "uint"
	case TypeIDUint8:
		return "byte"
	case TypeIDUint16:
		return "uint16"
	case TypeIDUint32:
		return "uint32"
	case TypeIDUint64:
		return "uint64"
	case TypeIDStringSlice:
		return "[]string"
	default:
		return fmt.Sprintf("TypeID(%d)", id)
	}
}

// BootstrapTypes is a list of wire type and ID pairs for the initial bootstrap types.
// TODO(bprosnitz) Investigate whether we should keep this mapping between wiretype and ID
var BootstrapTypes = []struct {
	WT  interface{}
	TID TypeID
}{
	{createPrimitiveBootstrapType(TypeIDInterface), TypeIDInterface},
	{createPrimitiveBootstrapType(TypeIDBool), TypeIDBool},

	{createPrimitiveBootstrapType(TypeIDString), TypeIDString},
	{SliceType{
		Elem: TypeIDUint8,
	}, TypeIDByteSlice},

	{NamedPrimitiveType{
		Type: TypeIDUint64,
		Name: "veyron.io/veyron/veyron2/wiretype.TypeID",
	}, TypeIDTypeID},

	{StructType{
		Fields: []FieldType{
			{
				Name: "Type",
				Type: TypeIDTypeID,
			},
			{
				Name: "Name",
				Type: TypeIDString,
			},
			{
				Name: "Tags",
				Type: TypeIDStringSlice,
			},
		},
		Name: "veyron.io/veyron/veyron2/wiretype.NamedPrimitiveType",
	}, TypeIDNamedPrimitiveType},

	{StructType{
		Fields: []FieldType{
			{
				Name: "Elem",
				Type: TypeIDTypeID,
			},
			{
				Name: "Name",
				Type: TypeIDString,
			},
			{
				Name: "Tags",
				Type: TypeIDStringSlice,
			},
		},
		Name: "veyron.io/veyron/veyron2/wiretype.SliceType",
	}, TypeIDSliceType},

	{StructType{
		Fields: []FieldType{
			{
				Name: "Elem",
				Type: TypeIDTypeID,
			},
			{
				Name: "Len",
				Type: TypeIDUint64,
			},
			{
				Name: "Name",
				Type: TypeIDString,
			},
			{
				Name: "Tags",
				Type: TypeIDStringSlice,
			},
		},
		Name: "veyron.io/veyron/veyron2/wiretype.ArrayType",
	}, TypeIDArrayType},

	{StructType{
		Fields: []FieldType{
			{
				Name: "Key",
				Type: TypeIDTypeID,
			},
			{
				Name: "Elem",
				Type: TypeIDTypeID,
			},
			{
				Name: "Name",
				Type: TypeIDString,
			},
			{
				Name: "Tags",
				Type: TypeIDStringSlice,
			},
		},
		Name: "veyron.io/veyron/veyron2/wiretype.MapType",
	}, TypeIDMapType},

	{StructType{
		Fields: []FieldType{
			{
				Name: "Type",
				Type: TypeIDTypeID,
			},
			{
				Name: "Name",
				Type: TypeIDString,
			},
		},
		Name: "veyron.io/veyron/veyron2/wiretype.FieldType",
	}, TypeIDFieldType},

	{SliceType{
		Elem: TypeIDFieldType,
		Name: "",
	}, TypeIDFieldSliceType},

	{StructType{
		Fields: []FieldType{
			{
				Name: "Fields",
				Type: TypeIDFieldSliceType,
			},
			{
				Name: "Name",
				Type: TypeIDString,
			},
			{
				Name: "Tags",
				Type: TypeIDStringSlice,
			},
		},
		Name: "veyron.io/veyron/veyron2/wiretype.StructType",
	}, TypeIDStructType},

	{StructType{
		Fields: []FieldType{
			{
				Name: "Elem",
				Type: TypeIDTypeID,
			},
			{
				Name: "Name",
				Type: TypeIDString,
			},
			{
				Name: "Tags",
				Type: TypeIDStringSlice,
			},
		},
		Name: "veyron.io/veyron/veyron2/wiretype.PtrType",
	}, TypeIDPtrType},

	{createPrimitiveBootstrapType(TypeIDFloat32), TypeIDFloat32},
	{createPrimitiveBootstrapType(TypeIDFloat64), TypeIDFloat64},

	{createPrimitiveBootstrapType(TypeIDComplex64), TypeIDComplex64},
	{createPrimitiveBootstrapType(TypeIDComplex128), TypeIDComplex128},

	{createPrimitiveBootstrapType(TypeIDInt), TypeIDInt},
	{createPrimitiveBootstrapType(TypeIDInt8), TypeIDInt8},
	{createPrimitiveBootstrapType(TypeIDInt16), TypeIDInt16},
	{createPrimitiveBootstrapType(TypeIDInt32), TypeIDInt32},
	{createPrimitiveBootstrapType(TypeIDInt64), TypeIDInt64},

	{createPrimitiveBootstrapType(TypeIDUintptr), TypeIDUintptr},
	{createPrimitiveBootstrapType(TypeIDUint), TypeIDUint},
	{NamedPrimitiveType{
		Type: TypeIDUint8,
		Name: "uint8",
	}, TypeIDUint8},
	{createPrimitiveBootstrapType(TypeIDUint16), TypeIDUint16},
	{createPrimitiveBootstrapType(TypeIDUint32), TypeIDUint32},
	{createPrimitiveBootstrapType(TypeIDUint64), TypeIDUint64},

	{SliceType{
		Elem: TypeIDString,
	}, TypeIDStringSlice},
}

func createPrimitiveBootstrapType(typID TypeID) interface{} {
	return NamedPrimitiveType{
		Type: typID,
		Name: typID.Name(),
	}
}
