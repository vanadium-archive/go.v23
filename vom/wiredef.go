package vom

import (
	"fmt"
	"strings"

	"veyron2/wiretype"
)

// wireDef holds the "compiled" representation of WireType type definitions read
// over the wire.  This is only used by the decoder.  Some information is
// pre-computed as a performance optimization.
//
// wireDef is a single struct that holds the union of all fields we need for all
// kinds of types.  An alternative is to make wireDef an interface, and have
// different underlying structs for each kind of type.  That costs us ~25%
// decoder performance, so we go with this.
type wireDef struct {
	// Common fields for all kinds of types.
	defined  bool
	name     string
	snames   []string
	tags     []string
	kind     typeKind
	rt       Type
	numStars int

	// Valid for primitive types.
	id wiretype.TypeID

	// Valid for slice, array, map, ptr.
	elem *wireDef

	// Valid for array.
	len int

	// Valid for map.
	key *wireDef

	// Valid for struct.
	fields []wireDefField
}

type wireDefField struct {
	name string
	def  *wireDef
}

func (def *wireDef) String() string       { return def.name }
func (def *wireDef) shortNames() []string { return def.snames }

func (def *wireDef) fieldByName(name string) *wireDef {
	// TODO(toddw): See if a map yields faster lookups.
	for _, field := range def.fields {
		if field.name == name {
			return field.def
		}
	}
	return nil
}

// fullName computes the full name of rt.  This code must be kept in-sync with
// fullName in type.go so that the decoder can re-create unnamed arrays and
// structs.
func (def *wireDef) fullName() string {
	if def.name != "" {
		return def.name
	}
	if def.id != wiretype.TypeIDInvalid {
		return def.id.Name()
	}
	switch def.kind {
	case typeKindArray:
		return fmt.Sprintf("[%d]%s", def.len, def.elem.fullName())
	case typeKindSlice:
		return "[]" + def.elem.fullName()
	case typeKindMap:
		return "map[" + def.key.fullName() + "]" + def.elem.fullName()
	case typeKindStruct:
		name := "struct{"
		for fx, field := range def.fields {
			if fx > 0 {
				name += ";"
			}
			name += field.name + " " + field.def.fullName()
		}
		return name + "}"
	case typeKindPtr:
		return "*" + def.elem.fullName()
	case typeKindInterface:
		return "interface"
	default:
		panic(fmt.Errorf("vom: can't take wireDef.fullName of kind %v", def.kind))
	}
}

// define the wireDef, filling in computed fields.
func (def *wireDef) define() {
	def.defined = true
	def.startDefine()
	def.finishDefine()
}

// startDefine starts defining the wireDef, filling in its name and snames.
func (def *wireDef) startDefine() {
	if def.name == "" {
		// Unnamed types are assigned their full name, with no other short names.
		def.name = def.fullName()
		def.snames = []string{def.name}
	} else {
		def.snames = computeShortTypeNames(def.name)
	}
}

// finishDefine finishes defining the wireDef.
func (def *wireDef) finishDefine() {
	// Pre-compute the total number of stars before we get to a non-pointer type.
	// The chain of wireDef objects is *not* collapsed, since we still might need
	// separate rt fields for each pointer.
	if def.kind == typeKindPtr {
		if def.elem.kind == typeKindPtr {
			def.numStars += def.elem.numStars
		}
	}
	// If the final name is registered, it always takes priority.
	if rt := typeRegistry.nameToType(def.name); rt != nil {
		def.rt = ToVomType(rt)
		return
	}
	// Otherwise if rt is already filled in there's nothing left to do.
	if def.rt != nil {
		return
	}
	// Otherwise we try to construct the type dynamically.
	switch def.kind {
	case typeKindSlice:
		if def.elem.rt != nil {
			def.rt = SliceOf(def.elem.rt)
		}
	case typeKindArray:
		if def.elem.rt != nil {
			def.rt = ArrayOf(def.len, def.elem.rt)
		}
	case typeKindMap:
		if def.key.rt != nil && def.elem.rt != nil {
			def.rt = MapOf(def.key.rt, def.elem.rt)
		}
	case typeKindStruct:
		if isComplex64Def(def) {
			def.rt = rtComplex64
		} else if isComplex128Def(def) {
			def.rt = rtComplex128
		} else {
			fields := make([]StructField, len(def.fields))
			for i := 0; i < len(def.fields); i++ {
				fields[i].Name = def.fields[i].name
				fields[i].Type = def.fields[i].def.rt
				if len(fields[i].Name) == 0 {
					panic(fmt.Sprintf("Can't create struct with empty-named field %#v", def))
				}
				if fields[i].Name[0] >= 'a' && fields[i].Name[0] <= 'z' {
					fields[i].PkgPath = def.fields[i].def.rt.PkgPath()
				}
			}
			def.rt = StructOf(fields)
		}
	case typeKindPtr:
		if def.elem.rt != nil {
			def.rt = PtrTo(def.elem.rt)
		}
	}
}

// Bootstrap, used by the implementation during decoding.  The typeDefs
// for user-defined types is generated dynamically based on the WireType
// received by the decoder, based on these typeDefs.
var (
	wdInterface = &wireDef{kind: typeKindInterface, id: wiretype.TypeIDInterface, rt: rtInterface}
	wdBool      = &wireDef{kind: typeKindBool, id: wiretype.TypeIDBool, rt: TypeOf(false)}
	wdString    = &wireDef{kind: typeKindString, id: wiretype.TypeIDString, rt: rtString}
	wdByteSlice = &wireDef{kind: typeKindByteSlice, id: wiretype.TypeIDByteSlice, rt: rtByteSlice}

	wdUint    = &wireDef{kind: typeKindUint, id: wiretype.TypeIDUint, rt: TypeOf(uint(0))}
	wdUint8   = &wireDef{kind: typeKindByte, id: wiretype.TypeIDUint8, rt: TypeOf(uint8(0))}
	wdUint16  = &wireDef{kind: typeKindUint, id: wiretype.TypeIDUint16, rt: TypeOf(uint16(0))}
	wdUint32  = &wireDef{kind: typeKindUint, id: wiretype.TypeIDUint32, rt: TypeOf(uint32(0))}
	wdUint64  = &wireDef{kind: typeKindUint, id: wiretype.TypeIDUint64, rt: TypeOf(uint64(0))}
	wdUintptr = &wireDef{kind: typeKindUint, id: wiretype.TypeIDUintptr, rt: TypeOf(uintptr(0))}

	wdInt   = &wireDef{kind: typeKindInt, id: wiretype.TypeIDInt, rt: TypeOf(int(0))}
	wdInt8  = &wireDef{kind: typeKindInt, id: wiretype.TypeIDInt8, rt: TypeOf(int8(0))}
	wdInt16 = &wireDef{kind: typeKindInt, id: wiretype.TypeIDInt16, rt: TypeOf(int16(0))}
	wdInt32 = &wireDef{kind: typeKindInt, id: wiretype.TypeIDInt32, rt: TypeOf(int32(0))}
	wdInt64 = &wireDef{kind: typeKindInt, id: wiretype.TypeIDInt64, rt: TypeOf(int64(0))}

	wdFloat32 = &wireDef{kind: typeKindFloat, id: wiretype.TypeIDFloat32, rt: TypeOf(float32(0))}
	wdFloat64 = &wireDef{kind: typeKindFloat, id: wiretype.TypeIDFloat64, rt: TypeOf(float64(0))}

	wdField = &wireDef{
		kind: typeKindStruct,
		fields: []wireDefField{
			{name: "Type", def: wdUint},
			{name: "Name", def: wdString},
		},
		rt: TypeOf(wiretype.FieldType{}),
	}
	wdFieldSlice = &wireDef{
		kind: typeKindSlice,
		elem: wdField,
		rt:   TypeOf([]wiretype.FieldType{}),
	}
	wdStringSlice = &wireDef{
		kind: typeKindSlice,
		elem: wdString,
		rt:   TypeOf([]string{}),
	}

	bootstrapWireDefsFromID = map[wiretype.TypeID]*wireDef{
		wiretype.TypeIDInterface: wdInterface,
		wiretype.TypeIDBool:      wdBool,
		wiretype.TypeIDString:    wdString,
		wiretype.TypeIDByteSlice: wdByteSlice,

		wiretype.TypeIDUint:    wdUint,
		wiretype.TypeIDUint8:   wdUint8,
		wiretype.TypeIDUint16:  wdUint16,
		wiretype.TypeIDUint32:  wdUint32,
		wiretype.TypeIDUint64:  wdUint64,
		wiretype.TypeIDUintptr: wdUintptr,

		wiretype.TypeIDInt:   wdInt,
		wiretype.TypeIDInt8:  wdInt8,
		wiretype.TypeIDInt16: wdInt16,
		wiretype.TypeIDInt32: wdInt32,
		wiretype.TypeIDInt64: wdInt64,

		wiretype.TypeIDFloat32: wdFloat32,
		wiretype.TypeIDFloat64: wdFloat64,

		wiretype.TypeIDNamedPrimitiveType: &wireDef{
			kind: typeKindStruct,
			fields: []wireDefField{
				{name: "Type", def: wdUint},
				{name: "Name", def: wdString},
				{name: "Tags", def: wdStringSlice},
			},
			rt: TypeOf(wiretype.NamedPrimitiveType{}),
		},
		wiretype.TypeIDSliceType: &wireDef{
			kind: typeKindStruct,
			fields: []wireDefField{
				{name: "Elem", def: wdUint},
				{name: "Name", def: wdString},
				{name: "Tags", def: wdStringSlice},
			},
			rt: TypeOf(wiretype.SliceType{}),
		},
		wiretype.TypeIDArrayType: &wireDef{
			kind: typeKindStruct,
			fields: []wireDefField{
				{name: "Elem", def: wdUint},
				{name: "Len", def: wdUint},
				{name: "Name", def: wdString},
				{name: "Tags", def: wdStringSlice},
			},
			rt: TypeOf(wiretype.ArrayType{}),
		},
		wiretype.TypeIDMapType: &wireDef{
			kind: typeKindStruct,
			fields: []wireDefField{
				{name: "Key", def: wdUint},
				{name: "Elem", def: wdUint},
				{name: "Name", def: wdString},
				{name: "Tags", def: wdStringSlice},
			},
			rt: TypeOf(wiretype.MapType{}),
		},
		wiretype.TypeIDStructType: &wireDef{
			kind: typeKindStruct,
			fields: []wireDefField{
				{name: "Fields", def: wdFieldSlice},
				{name: "Name", def: wdString},
				{name: "Tags", def: wdStringSlice},
			},
			rt: TypeOf(wiretype.StructType{}),
		},
		wiretype.TypeIDPtrType: &wireDef{
			kind: typeKindStruct,
			fields: []wireDefField{
				{name: "Elem", def: wdUint},
				{name: "Name", def: wdString},
				{name: "Tags", def: wdStringSlice},
			},
			rt: TypeOf(wiretype.PtrType{}),
		},
	}

	bootstrapWireDefsFromName map[string]*wireDef
)

func init() {
	for _, def := range bootstrapWireDefsFromID {
		def.define()
	}
	bootstrapWireDefsFromName = make(map[string]*wireDef)
	for id, def := range bootstrapWireDefsFromID {
		if name := id.Name(); !strings.HasPrefix(name, "TypeID") {
			bootstrapWireDefsFromName[name] = def
		}
	}
}
