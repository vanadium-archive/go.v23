package vom

// TODO(toddw): Add tests.

import (
	"fmt"

	"v.io/v23/vdl"
)

// encoderTypes maintains the mapping from type to type id, used by the encoder.
type encoderTypes struct {
	typeToID map[*vdl.Type]typeId
	nextID   typeId
}

func newEncoderTypes() *encoderTypes {
	return &encoderTypes{make(map[*vdl.Type]typeId), WireIdFirstUserType}
}

func (et *encoderTypes) LookupID(tt *vdl.Type) typeId {
	if id := bootstrapTypeToID[tt]; id != 0 {
		return id
	}
	return et.typeToID[tt]
}

func (et *encoderTypes) LookupOrAssignID(tt *vdl.Type) (typeId, bool) {
	if id := et.LookupID(tt); id != 0 {
		return id, false
	}
	// Assign a new id.
	newID := et.nextID
	et.nextID++
	et.typeToID[tt] = newID
	return newID, true
}

// decoderTypes maintains the mapping from type id to type, used by the decoder.
type decoderTypes struct {
	idToWire map[typeId]wireType
	idToType map[typeId]*vdl.Type
}

func newDecoderTypes() *decoderTypes {
	return &decoderTypes{make(map[typeId]wireType), make(map[typeId]*vdl.Type)}
}

func (dt *decoderTypes) LookupOrBuildType(id typeId) (*vdl.Type, error) {
	if tt := dt.lookupType(id); tt != nil {
		return tt, nil
	}
	builder := new(vdl.TypeBuilder)
	pending := make(map[typeId]vdl.PendingType)
	result, err := dt.makeType(id, builder, pending)
	if err != nil {
		return nil, err
	}
	if !builder.Build() {
		// TODO(toddw): Change TypeBuilder.Build() to directly return the error.
		_, err := result.Built()
		return nil, err
	}
	for id, pend := range pending {
		tt, err := pend.Built()
		if err != nil {
			return nil, err
		}
		dt.idToType[id] = tt
	}
	built, err := result.Built()
	if err != nil {
		return nil, err
	}
	return built, nil
}

func (dt *decoderTypes) lookupType(id typeId) *vdl.Type {
	if tt := bootstrapIDToType[id]; tt != nil {
		return tt
	}
	return dt.idToType[id]
}

func (dt *decoderTypes) makeType(id typeId, builder *vdl.TypeBuilder, pending map[typeId]vdl.PendingType) (vdl.PendingType, error) {
	wt := dt.idToWire[id]
	if wt == nil {
		return nil, fmt.Errorf("vom: unknown type ID %d", id)
	}
	// Make the type from its wireType representation.  First remove it from
	// dt.idToWire, and add it to pending, so that subsequent lookups will get the
	// pending type.  Eventually the built type will be added to dt.idToType.
	delete(dt.idToWire, id)
	if name := wt.(wireTypeGeneric).TypeName(); name != "" {
		// Named types may be recursive, so we must create the named type first and
		// add it to pending, before we make the base type.  The base type may refer
		// back to this named type, and will find it in pending.
		namedType := builder.Named(name)
		pending[id] = namedType
		if wtNamed, ok := wt.(wireTypeNamedT); ok {
			// This is a NamedType pointing at a base type.
			baseType, err := dt.lookupOrMakeType(wtNamed.Value.Base, builder, pending)
			if err != nil {
				return nil, err
			}
			namedType.AssignBase(baseType)
			return namedType, nil
		}
		// This isn't NamedType, but has a non-empty name.
		baseType, err := dt.makeBaseType(wt, builder, pending)
		if err != nil {
			return nil, err
		}
		namedType.AssignBase(baseType)
		return namedType, nil
	}
	// Unnamed types are made directly from their base type.  It's fine to update
	// pending after making the base type, since there's no way to create a
	// recursive type based solely on unnamed vdl.
	baseType, err := dt.makeBaseType(wt, builder, pending)
	if err != nil {
		return nil, err
	}
	pending[id] = baseType
	return baseType, nil
}

func (dt *decoderTypes) makeBaseType(wt wireType, builder *vdl.TypeBuilder, pending map[typeId]vdl.PendingType) (vdl.PendingType, error) {
	switch wt := wt.(type) {
	case wireTypeNamedT:
		return nil, fmt.Errorf("vom: NamedType has empty name: %v", wt)
	case wireTypeEnumT:
		enumType := builder.Enum()
		for _, label := range wt.Value.Labels {
			enumType.AppendLabel(label)
		}
		return enumType, nil
	case wireTypeArrayT:
		elemType, err := dt.lookupOrMakeType(wt.Value.Elem, builder, pending)
		if err != nil {
			return nil, err
		}
		return builder.Array().AssignElem(elemType).AssignLen(int(wt.Value.Len)), nil
	case wireTypeListT:
		elemType, err := dt.lookupOrMakeType(wt.Value.Elem, builder, pending)
		if err != nil {
			return nil, err
		}
		return builder.List().AssignElem(elemType), nil
	case wireTypeSetT:
		keyType, err := dt.lookupOrMakeType(wt.Value.Key, builder, pending)
		if err != nil {
			return nil, err
		}
		return builder.Set().AssignKey(keyType), nil
	case wireTypeMapT:
		keyType, err := dt.lookupOrMakeType(wt.Value.Key, builder, pending)
		if err != nil {
			return nil, err
		}
		elemType, err := dt.lookupOrMakeType(wt.Value.Elem, builder, pending)
		if err != nil {
			return nil, err
		}
		return builder.Map().AssignKey(keyType).AssignElem(elemType), nil
	case wireTypeStructT:
		structType := builder.Struct()
		for _, field := range wt.Value.Fields {
			fieldType, err := dt.lookupOrMakeType(field.Type, builder, pending)
			if err != nil {
				return nil, err
			}
			structType.AppendField(field.Name, fieldType)
		}
		return structType, nil
	case wireTypeUnionT:
		unionType := builder.Union()
		for _, field := range wt.Value.Fields {
			fieldType, err := dt.lookupOrMakeType(field.Type, builder, pending)
			if err != nil {
				return nil, err
			}
			unionType.AppendField(field.Name, fieldType)
		}
		return unionType, nil
	case wireTypeOptionalT:
		elemType, err := dt.lookupOrMakeType(wt.Value.Elem, builder, pending)
		if err != nil {
			return nil, err
		}
		return builder.Optional().AssignElem(elemType), nil
	default:
		return nil, fmt.Errorf("vom: unknown wire type definition %v", wt)
	}
}

func (dt *decoderTypes) lookupOrMakeType(id typeId, builder *vdl.TypeBuilder, pending map[typeId]vdl.PendingType) (vdl.TypeOrPending, error) {
	if tt := dt.lookupType(id); tt != nil {
		return tt, nil
	}
	if p, ok := pending[id]; ok {
		return p, nil
	}
	return dt.makeType(id, builder, pending)
}

func (dt *decoderTypes) AddWireType(id typeId, wt wireType) error {
	if id < WireIdFirstUserType {
		return fmt.Errorf("vom: type %q id %d invalid, the min user id is %d", wt, id, WireIdFirstUserType)
	}
	// TODO(toddw): Allow duplicates according to some heuristic (e.g. only
	// identical, or only if the later one is a "superset", etc).
	if _, dup := dt.idToWire[id]; dup {
		return fmt.Errorf("vom: type %q id %d already defined as %q", wt, id, dup)
	}
	if _, dup := dt.idToType[id]; dup {
		return fmt.Errorf("vom: type %q id %d already defined as %q", wt, id, dup)
	}
	dt.idToWire[id] = wt
	return nil
}

func isWireTypeType(tt *vdl.Type) bool {
	_, exist := bootstrapWireTypes[tt]
	return exist
}

// TODO(toddw): Provide type management routines

// Bootstrap mappings between type, id and kind.
var (
	bootstrapWireTypes map[*vdl.Type]struct{}
	bootstrapIDToType  map[typeId]*vdl.Type
	bootstrapTypeToID  map[*vdl.Type]typeId
	bootstrapKindToID  map[vdl.Kind]typeId

	typeIDType        = vdl.TypeOf(typeId(0))
	wireTypeType      = vdl.TypeOf((*wireType)(nil))
	wireNamedType     = vdl.TypeOf(wireNamed{})
	wireEnumType      = vdl.TypeOf(wireEnum{})
	wireArrayType     = vdl.TypeOf(wireArray{})
	wireListType      = vdl.TypeOf(wireList{})
	wireSetType       = vdl.TypeOf(wireSet{})
	wireMapType       = vdl.TypeOf(wireMap{})
	wireFieldType     = vdl.TypeOf(wireField{})
	wireFieldListType = vdl.TypeOf([]wireField{})
	wireStructType    = vdl.TypeOf(wireStruct{})
	wireUnionType     = vdl.TypeOf(wireUnion{})
	wireOptionalType  = vdl.TypeOf(wireOptional{})

	wireByteListType   = vdl.TypeOf([]byte{})
	wireStringListType = vdl.TypeOf([]string{})
)

func init() {
	bootstrapWireTypes = make(map[*vdl.Type]struct{})

	// The basic wire types for type definition.
	for _, tt := range []*vdl.Type{
		typeIDType,
		wireTypeType,
		wireFieldType,
		wireFieldListType,
	} {
		bootstrapWireTypes[tt] = struct{}{}
	}

	// The extra wire types for each kind of type definition. The field indices
	// in wireType should not be changed.
	wtTypes := []*vdl.Type{
		wireNamedType,
		wireEnumType,
		wireArrayType,
		wireListType,
		wireSetType,
		wireMapType,
		wireStructType,
		wireUnionType,
		wireOptionalType,
	}
	if len(wtTypes) != wireTypeType.NumField() {
		panic("vom: wireType definition changed")
	}
	for ix, tt := range wtTypes {
		if tt != wireTypeType.Field(ix).Type {
			panic("vom: wireType definition changed")
		}
		bootstrapWireTypes[tt] = struct{}{}
	}

	bootstrapIDToType = make(map[typeId]*vdl.Type)
	bootstrapTypeToID = make(map[*vdl.Type]typeId)
	bootstrapKindToID = make(map[vdl.Kind]typeId)

	// The basic bootstrap types can be converted between type, id and kind.
	for id, tt := range map[typeId]*vdl.Type{
		WireIdBool:       vdl.BoolType,
		WireIdByte:       vdl.ByteType,
		WireIdString:     vdl.StringType,
		WireIdUint16:     vdl.Uint16Type,
		WireIdUint32:     vdl.Uint32Type,
		WireIdUint64:     vdl.Uint64Type,
		WireIdInt16:      vdl.Int16Type,
		WireIdInt32:      vdl.Int32Type,
		WireIdInt64:      vdl.Int64Type,
		WireIdFloat32:    vdl.Float32Type,
		WireIdFloat64:    vdl.Float64Type,
		WireIdComplex64:  vdl.Complex64Type,
		WireIdComplex128: vdl.Complex128Type,
		WireIdTypeObject: vdl.TypeObjectType,
		WireIdAny:        vdl.AnyType,
	} {
		bootstrapIDToType[id] = tt
		bootstrapTypeToID[tt] = id
		bootstrapKindToID[tt.Kind()] = id
	}
	// The extra bootstrap types can be converted between type and id.
	for id, tt := range map[typeId]*vdl.Type{
		WireIdByteList:   wireByteListType,
		WireIdStringList: wireStringListType,
	} {
		bootstrapIDToType[id] = tt
		bootstrapTypeToID[tt] = id
	}
}

// A generic interface for all wireType types.
type wireTypeGeneric interface {
	TypeName() string
}

func (wt wireTypeNamedT) TypeName() string {
	return wt.Value.Name
}

func (wt wireTypeEnumT) TypeName() string {
	return wt.Value.Name
}

func (wt wireTypeArrayT) TypeName() string {
	return wt.Value.Name
}

func (wt wireTypeListT) TypeName() string {
	return wt.Value.Name
}

func (wt wireTypeSetT) TypeName() string {
	return wt.Value.Name
}

func (wt wireTypeMapT) TypeName() string {
	return wt.Value.Name
}

func (wt wireTypeStructT) TypeName() string {
	return wt.Value.Name
}

func (wt wireTypeUnionT) TypeName() string {
	return wt.Value.Name
}

func (wt wireTypeOptionalT) TypeName() string {
	return wt.Value.Name
}
