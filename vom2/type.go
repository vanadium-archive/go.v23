package vom2

// TODO(toddw): Add tests.

import (
	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/verror"
)

// TODO: Replace this with vdl oneof!
type WireType interface{}

// encoderTypes maintains the mapping from type to type id, used by the encoder.
type encoderTypes struct {
	typeToID map[*vdl.Type]TypeID
	nextID   TypeID
}

func newEncoderTypes() *encoderTypes {
	return &encoderTypes{make(map[*vdl.Type]TypeID), WireTypeFirstUserID}
}

func (et *encoderTypes) LookupID(t *vdl.Type) TypeID {
	if id := bootstrapTypeToID[t]; id != 0 {
		return id
	}
	return et.typeToID[t]
}

func (et *encoderTypes) LookupOrAssignID(t *vdl.Type) (TypeID, bool) {
	if id := et.LookupID(t); id != 0 {
		return id, false
	}
	// Assign a new id.
	newID := et.nextID
	et.nextID++
	et.typeToID[t] = newID
	return newID, true
}

// decoderTypes maintains the mapping from type id to type, used by the decoder.
type decoderTypes struct {
	idToWire map[TypeID]*vdl.Value
	idToType map[TypeID]*vdl.Type
}

func newDecoderTypes() *decoderTypes {
	return &decoderTypes{make(map[TypeID]*vdl.Value), make(map[TypeID]*vdl.Type)}
}

func (dt *decoderTypes) LookupOrBuildType(id TypeID) (*vdl.Type, error) {
	if t := dt.lookupType(id); t != nil {
		return t, nil
	}
	builder := new(vdl.TypeBuilder)
	pending := make(map[TypeID]vdl.PendingType)
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
		t, err := pend.Built()
		if err != nil {
			return nil, err
		}
		dt.idToType[id] = t
	}
	built, err := result.Built()
	if err != nil {
		return nil, err
	}
	return built, nil
}

func (dt *decoderTypes) lookupType(id TypeID) *vdl.Type {
	if t := bootstrapIDToType[id]; t != nil {
		return t
	}
	return dt.idToType[id]
}

func (dt *decoderTypes) makeType(id TypeID, builder *vdl.TypeBuilder, pending map[TypeID]vdl.PendingType) (vdl.PendingType, error) {
	wt := dt.idToWire[id]
	if wt == nil {
		return nil, verror.BadProtocolf("vom: unknown type ID %d", id)
	}
	// Make the type from its WireType representation.  First remove it from
	// dt.idToWire, and add it to pending, so that subsequent lookups will get the
	// pending type.  Eventually the built type will be added to dt.idToType.
	delete(dt.idToWire, id)
	if name := wt.Field(0).RawString(); name != "" {
		// Named types may be recursive, so we must create the named type first and
		// add it to pending, before we make the base type.  The base type may refer
		// back to this named type, and will find it in pending.
		named := builder.Named(name)
		pending[id] = named
		if wt.Type() == wireNamedType {
			// This is a NamedType pointing at a base type.
			baseID := TypeID(wt.Field(1).Uint())
			base, err := dt.lookupOrMakeType(baseID, builder, pending)
			if err != nil {
				return nil, err
			}
			named.AssignBase(base)
			return named, nil
		}
		// This isn't NamedType, but has a non-empty name.
		base, err := dt.makeBaseType(wt, builder, pending)
		if err != nil {
			return nil, err
		}
		named.AssignBase(base)
		return named, nil
	}
	// Unnamed types are made directly from their base type.  It's fine to update
	// pending after making the base type, since there's no way to create a
	// recursive type based solely on unnamed vdl.
	base, err := dt.makeBaseType(wt, builder, pending)
	if err != nil {
		return nil, err
	}
	pending[id] = base
	return base, nil
}

func (dt *decoderTypes) makeBaseType(wt *vdl.Value, builder *vdl.TypeBuilder, pending map[TypeID]vdl.PendingType) (vdl.PendingType, error) {
	// TODO(toddw): Consider changing wt to WireType after oneof is implemented.
	switch wt.Type() {
	case wireNamedType:
		return nil, verror.BadProtocolf("vom: NamedType has empty name: %v", wt)
	case wireEnumType:
		vLabels := wt.Field(1)
		base := builder.Enum()
		for ix := 0; ix < vLabels.Len(); ix++ {
			base.AppendLabel(vLabels.Index(ix).RawString())
		}
		return base, nil
	case wireArrayType:
		elemID, len := TypeID(wt.Field(1).Uint()), int(wt.Field(2).Uint())
		elem, err := dt.lookupOrMakeType(elemID, builder, pending)
		if err != nil {
			return nil, err
		}
		return builder.Array().AssignElem(elem).AssignLen(len), nil
	case wireListType:
		elemID := TypeID(wt.Field(1).Uint())
		elem, err := dt.lookupOrMakeType(elemID, builder, pending)
		if err != nil {
			return nil, err
		}
		return builder.List().AssignElem(elem), nil
	case wireSetType:
		keyID := TypeID(wt.Field(1).Uint())
		key, err := dt.lookupOrMakeType(keyID, builder, pending)
		if err != nil {
			return nil, err
		}
		return builder.Set().AssignKey(key), nil
	case wireMapType:
		keyID, elemID := TypeID(wt.Field(1).Uint()), TypeID(wt.Field(2).Uint())
		key, err := dt.lookupOrMakeType(keyID, builder, pending)
		if err != nil {
			return nil, err
		}
		elem, err := dt.lookupOrMakeType(elemID, builder, pending)
		if err != nil {
			return nil, err
		}
		return builder.Map().AssignKey(key).AssignElem(elem), nil
	case wireStructType:
		vFields := wt.Field(1)
		base := builder.Struct()
		for ix := 0; ix < vFields.Len(); ix++ {
			vf := vFields.Index(ix)
			fname, fieldID := vf.Field(0).RawString(), TypeID(vf.Field(1).Uint())
			field, err := dt.lookupOrMakeType(fieldID, builder, pending)
			if err != nil {
				return nil, err
			}
			base.AppendField(fname, field)
		}
		return base, nil
	case wireOneOfType:
		vTypes := wt.Field(1)
		base := builder.OneOf()
		for ix := 0; ix < vTypes.Len(); ix++ {
			oneID := TypeID(vTypes.Index(ix).Uint())
			one, err := dt.lookupOrMakeType(oneID, builder, pending)
			if err != nil {
				return nil, err
			}
			base.AppendType(one)
		}
		return base, nil
	default:
		return nil, verror.BadProtocolf("vom: unknown wire type definition %v", wt)
	}
}

func (dt *decoderTypes) lookupOrMakeType(id TypeID, builder *vdl.TypeBuilder, pending map[TypeID]vdl.PendingType) (vdl.TypeOrPending, error) {
	if t := dt.lookupType(id); t != nil {
		return t, nil
	}
	if p, ok := pending[id]; ok {
		return p, nil
	}
	return dt.makeType(id, builder, pending)
}

func (dt *decoderTypes) AddWireType(id TypeID, wt *vdl.Value) error {
	// TODO(toddw): check that wireType is one of our known structs!
	if id < WireTypeFirstUserID {
		return verror.BadProtocolf("vom: type %q id %d invalid, the min user id is %d", wt, id, WireTypeFirstUserID)
	}
	if dup := dt.idToWire[id]; dup != nil {
		return verror.BadProtocolf("vom: type %q id %d already defined as %q", wt, id, dup)
	}
	if dup := dt.idToType[id]; dup != nil {
		return verror.BadProtocolf("vom: type %q id %d already defined as %q", wt, id, dup)
	}
	dt.idToWire[id] = wt
	return nil
}

// TODO(toddw): Provide type management routines

// Bootstrap mappings between type, id and kind.
var (
	bootstrapIDToType map[TypeID]*vdl.Type
	bootstrapTypeToID map[*vdl.Type]TypeID
	bootstrapKindToID map[vdl.Kind]TypeID

	wireNamedType      = bootstrapTypeOf(WireNamed{})
	wireEnumType       = bootstrapTypeOf(WireEnum{})
	wireArrayType      = bootstrapTypeOf(WireArray{})
	wireListType       = bootstrapTypeOf(WireList{})
	wireSetType        = bootstrapTypeOf(WireSet{})
	wireMapType        = bootstrapTypeOf(WireMap{})
	wireStructType     = bootstrapTypeOf(WireStruct{})
	wireFieldType      = bootstrapTypeOf(WireField{})
	wireFieldListType  = bootstrapTypeOf([]WireField{})
	wireOneOfType      = bootstrapTypeOf(WireOneOf{})
	wireByteListType   = bootstrapTypeOf([]byte{})
	wireStringListType = bootstrapTypeOf([]string{})
	wireTypeListType   = bootstrapTypeOf([]*vdl.Type{})
)

func init() {
	bootstrapIDToType = make(map[TypeID]*vdl.Type)
	bootstrapTypeToID = make(map[*vdl.Type]TypeID)
	bootstrapKindToID = make(map[vdl.Kind]TypeID)
	// The basic bootstrap types can be converted between type, id and kind.
	for id, t := range map[TypeID]*vdl.Type{
		WireAnyID:        vdl.AnyType,
		WireTypeID:       vdl.TypeValType,
		WireBoolID:       vdl.BoolType,
		WireStringID:     vdl.StringType,
		WireByteID:       vdl.ByteType,
		WireUint16ID:     vdl.Uint16Type,
		WireUint32ID:     vdl.Uint32Type,
		WireUint64ID:     vdl.Uint64Type,
		WireInt16ID:      vdl.Int16Type,
		WireInt32ID:      vdl.Int32Type,
		WireInt64ID:      vdl.Int64Type,
		WireFloat32ID:    vdl.Float32Type,
		WireFloat64ID:    vdl.Float64Type,
		WireComplex64ID:  vdl.Complex64Type,
		WireComplex128ID: vdl.Complex128Type,
	} {
		bootstrapIDToType[id] = t
		bootstrapTypeToID[t] = id
		bootstrapKindToID[t.Kind()] = id
	}
	// The extra bootstrap types can be converted between type and id.
	for id, t := range map[TypeID]*vdl.Type{
		WireNamedID:      wireNamedType,
		WireEnumID:       wireEnumType,
		WireArrayID:      wireArrayType,
		WireListID:       wireListType,
		WireSetID:        wireSetType,
		WireMapID:        wireMapType,
		WireStructID:     wireStructType,
		WireFieldID:      wireFieldType,
		WireFieldListID:  wireFieldListType,
		WireOneOfID:      wireOneOfType,
		WireByteListID:   wireByteListType,
		WireStringListID: wireStringListType,
		WireTypeListID:   wireTypeListType,
	} {
		bootstrapIDToType[id] = t
		bootstrapTypeToID[t] = id
	}
}

func bootstrapTypeOf(v interface{}) *vdl.Type {
	t, err := vdl.TypeOf(v)
	if err != nil {
		panic(verror.Internalf("vom: can't take TypeOf(%T %#v)", v, v))
	}
	return t
}
