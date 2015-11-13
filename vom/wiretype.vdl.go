// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Source: wiretype.vdl

package vom

import (
	// VDL system imports
	"v.io/v23/vdl"
)

// typeId uniquely identifies a type definition within a vom stream.
type typeId uint64

func (typeId) __VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom.typeId"`
}) {
}

type (
	// wireType represents any single field of the wireType union type.
	//
	// The wireType union is used to encode the payload part of each type message,
	// using the regular rules for encoding union values.  But unlike our regular
	// encoding, the type message for wireType itself (and its fields) are never
	// encoded; we need to bootstrap the system.  Thus unlike regular values, the
	// ordering of fields within the wire* types cannot be changed.
	wireType interface {
		// Index returns the field index.
		Index() int
		// Interface returns the field value as an interface.
		Interface() interface{}
		// Name returns the field name.
		Name() string
		// __VDLReflect describes the wireType union type.
		__VDLReflect(__wireTypeReflect)
	}
	// wireTypeNamedT represents field NamedT of the wireType union type.
	//
	// FIELD INDICES MUST NOT BE CHANGED.
	wireTypeNamedT struct{ Value wireNamed } // INDEX = 0
	// wireTypeEnumT represents field EnumT of the wireType union type.
	wireTypeEnumT struct{ Value wireEnum } // INDEX = 1
	// wireTypeArrayT represents field ArrayT of the wireType union type.
	wireTypeArrayT struct{ Value wireArray } // INDEX = 2
	// wireTypeListT represents field ListT of the wireType union type.
	wireTypeListT struct{ Value wireList } // INDEX = 3
	// wireTypeSetT represents field SetT of the wireType union type.
	wireTypeSetT struct{ Value wireSet } // INDEX = 4
	// wireTypeMapT represents field MapT of the wireType union type.
	wireTypeMapT struct{ Value wireMap } // INDEX = 5
	// wireTypeStructT represents field StructT of the wireType union type.
	wireTypeStructT struct{ Value wireStruct } // INDEX = 6
	// wireTypeUnionT represents field UnionT of the wireType union type.
	wireTypeUnionT struct{ Value wireUnion } // INDEX = 7
	// wireTypeOptionalT represents field OptionalT of the wireType union type.
	wireTypeOptionalT struct{ Value wireOptional } // INDEX = 8
	// __wireTypeReflect describes the wireType union type.
	__wireTypeReflect struct {
		Name  string `vdl:"v.io/v23/vom.wireType"`
		Type  wireType
		Union struct {
			NamedT    wireTypeNamedT
			EnumT     wireTypeEnumT
			ArrayT    wireTypeArrayT
			ListT     wireTypeListT
			SetT      wireTypeSetT
			MapT      wireTypeMapT
			StructT   wireTypeStructT
			UnionT    wireTypeUnionT
			OptionalT wireTypeOptionalT
		}
	}
)

func (x wireTypeNamedT) Index() int                     { return 0 }
func (x wireTypeNamedT) Interface() interface{}         { return x.Value }
func (x wireTypeNamedT) Name() string                   { return "NamedT" }
func (x wireTypeNamedT) __VDLReflect(__wireTypeReflect) {}

func (x wireTypeEnumT) Index() int                     { return 1 }
func (x wireTypeEnumT) Interface() interface{}         { return x.Value }
func (x wireTypeEnumT) Name() string                   { return "EnumT" }
func (x wireTypeEnumT) __VDLReflect(__wireTypeReflect) {}

func (x wireTypeArrayT) Index() int                     { return 2 }
func (x wireTypeArrayT) Interface() interface{}         { return x.Value }
func (x wireTypeArrayT) Name() string                   { return "ArrayT" }
func (x wireTypeArrayT) __VDLReflect(__wireTypeReflect) {}

func (x wireTypeListT) Index() int                     { return 3 }
func (x wireTypeListT) Interface() interface{}         { return x.Value }
func (x wireTypeListT) Name() string                   { return "ListT" }
func (x wireTypeListT) __VDLReflect(__wireTypeReflect) {}

func (x wireTypeSetT) Index() int                     { return 4 }
func (x wireTypeSetT) Interface() interface{}         { return x.Value }
func (x wireTypeSetT) Name() string                   { return "SetT" }
func (x wireTypeSetT) __VDLReflect(__wireTypeReflect) {}

func (x wireTypeMapT) Index() int                     { return 5 }
func (x wireTypeMapT) Interface() interface{}         { return x.Value }
func (x wireTypeMapT) Name() string                   { return "MapT" }
func (x wireTypeMapT) __VDLReflect(__wireTypeReflect) {}

func (x wireTypeStructT) Index() int                     { return 6 }
func (x wireTypeStructT) Interface() interface{}         { return x.Value }
func (x wireTypeStructT) Name() string                   { return "StructT" }
func (x wireTypeStructT) __VDLReflect(__wireTypeReflect) {}

func (x wireTypeUnionT) Index() int                     { return 7 }
func (x wireTypeUnionT) Interface() interface{}         { return x.Value }
func (x wireTypeUnionT) Name() string                   { return "UnionT" }
func (x wireTypeUnionT) __VDLReflect(__wireTypeReflect) {}

func (x wireTypeOptionalT) Index() int                     { return 8 }
func (x wireTypeOptionalT) Interface() interface{}         { return x.Value }
func (x wireTypeOptionalT) Name() string                   { return "OptionalT" }
func (x wireTypeOptionalT) __VDLReflect(__wireTypeReflect) {}

// wireNamed represents a type definition for named primitives.
type wireNamed struct {
	Name string
	Base typeId
}

func (wireNamed) __VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom.wireNamed"`
}) {
}

// wireEnum represents an type definition for enum types.
type wireEnum struct {
	Name   string
	Labels []string
}

func (wireEnum) __VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom.wireEnum"`
}) {
}

// wireArray represents an type definition for array types.
type wireArray struct {
	Name string
	Elem typeId
	Len  uint64
}

func (wireArray) __VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom.wireArray"`
}) {
}

// wireList represents a type definition for list types.
type wireList struct {
	Name string
	Elem typeId
}

func (wireList) __VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom.wireList"`
}) {
}

// wireSet represents a type definition for set types.
type wireSet struct {
	Name string
	Key  typeId
}

func (wireSet) __VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom.wireSet"`
}) {
}

// wireMap represents a type definition for map types.
type wireMap struct {
	Name string
	Key  typeId
	Elem typeId
}

func (wireMap) __VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom.wireMap"`
}) {
}

// wireField represents a field in a struct or union type.
type wireField struct {
	Name string
	Type typeId
}

func (wireField) __VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom.wireField"`
}) {
}

// wireStruct represents a type definition for struct types.
type wireStruct struct {
	Name   string
	Fields []wireField
}

func (wireStruct) __VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom.wireStruct"`
}) {
}

// wireUnion represents a type definition for union types.
type wireUnion struct {
	Name   string
	Fields []wireField
}

func (wireUnion) __VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom.wireUnion"`
}) {
}

// wireOptional represents an type definition for optional types.
type wireOptional struct {
	Name string
	Elem typeId
}

func (wireOptional) __VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom.wireOptional"`
}) {
}

func init() {
	vdl.Register((*typeId)(nil))
	vdl.Register((*wireType)(nil))
	vdl.Register((*wireNamed)(nil))
	vdl.Register((*wireEnum)(nil))
	vdl.Register((*wireArray)(nil))
	vdl.Register((*wireList)(nil))
	vdl.Register((*wireSet)(nil))
	vdl.Register((*wireMap)(nil))
	vdl.Register((*wireField)(nil))
	vdl.Register((*wireStruct)(nil))
	vdl.Register((*wireUnion)(nil))
	vdl.Register((*wireOptional)(nil))
}

// Primitive types.
const WireIdBool = typeId(1)

const WireIdByte = typeId(2)

const WireIdString = typeId(3)

const WireIdUint16 = typeId(4)

const WireIdUint32 = typeId(5)

const WireIdUint64 = typeId(6)

const WireIdInt16 = typeId(7)

const WireIdInt32 = typeId(8)

const WireIdInt64 = typeId(9)

const WireIdFloat32 = typeId(10)

const WireIdFloat64 = typeId(11)

const WireIdComplex64 = typeId(12)

const WireIdComplex128 = typeId(13)

const WireIdTypeObject = typeId(14)

const WireIdAny = typeId(15)

const WireIdInt8 = typeId(16)

// Other commonly used composites.
const WireIdByteList = typeId(39)

const WireIdStringList = typeId(40)

// The first user-defined typeId is 41.
const WireIdFirstUserType = typeId(41)

const WireCtrlNil = byte(224)

const WireCtrlEnd = byte(225)

const WireCtrlTypeCont = byte(226)

const WireCtrlValueCont = byte(227)
