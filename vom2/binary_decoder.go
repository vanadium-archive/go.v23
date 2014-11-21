package vom2

import (
	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/vdl/valconv"
	"veyron.io/veyron/veyron2/verror"
)

var (
	errDecodeZeroTypeID = verror.BadProtocolf("vom: type ID 0")
	errIndexOutOfRange  = verror.BadProtocolf("vom: index out of range")
)

type binaryDecoder struct {
	buf       *decbuf
	recvTypes *decoderTypes
}

func newBinaryDecoder(buf *decbuf, types *decoderTypes) *binaryDecoder {
	return &binaryDecoder{buf, types}
}

func (d *binaryDecoder) Decode(target valconv.Target) error {
	valType, err := d.decodeValueType()
	if err != nil {
		return err
	}
	return d.decodeValueMsg(valType, target)
}

func (d *binaryDecoder) DecodeRaw(raw *RawValue) error {
	valType, err := d.decodeValueType()
	if err != nil {
		return err
	}
	valLen, err := d.decodeValueByteLen(valType)
	if err != nil {
		return err
	}
	raw.recvTypes = d.recvTypes
	raw.valType = valType
	if cap(raw.data) >= valLen {
		raw.data = raw.data[:valLen]
	} else {
		raw.data = make([]byte, valLen)
	}
	return d.buf.ReadFull(raw.data)
}

func (d *binaryDecoder) Ignore() error {
	valType, err := d.decodeValueType()
	if err != nil {
		return err
	}
	valLen, err := d.decodeValueByteLen(valType)
	if err != nil {
		return err
	}
	return d.buf.Skip(valLen)
}

// decodeValueType returns the type of the next value message.  Any type
// definition messages it encounters along the way are decoded and added to
// recvTypes.
func (d *binaryDecoder) decodeValueType() (*vdl.Type, error) {
	for {
		id, err := binaryDecodeInt(d.buf)
		if err != nil {
			return nil, err
		}
		switch {
		case id == 0:
			return nil, errDecodeZeroTypeID
		case id > 0:
			// This is a value message, the TypeID is +id.
			tid := TypeID(+id)
			t, err := d.recvTypes.LookupOrBuildType(tid)
			if err != nil {
				return nil, err
			}
			return t, nil
		}
		// This is a type message, the TypeID is -id.
		tid := TypeID(-id)
		// Decode the WireType like a regular value, and store it in recvTypes.  The
		// type will actually be built when a value message arrives using this tid.
		wireType := vdl.ZeroValue(vdl.AnyType)
		target, err := valconv.ValueTarget(wireType)
		if err != nil {
			return nil, err
		}
		if err := d.decodeValueMsg(vdl.AnyType, target); err != nil {
			return nil, err
		}
		if err := d.recvTypes.AddWireType(tid, wireType.Elem()); err != nil {
			return nil, err
		}
	}
}

// decodeValueByteLen returns the byte length of the next value.
func (d *binaryDecoder) decodeValueByteLen(t *vdl.Type) (int, error) {
	if hasBinaryMsgLen(t) {
		// Use the explicit message length.
		msgLen, err := binaryDecodeLen(d.buf)
		if err != nil {
			return 0, err
		}
		return msgLen, nil
	}
	// No explicit message length, but the length can be computed.
	switch {
	case t.Kind() == vdl.Byte:
		// Single byte is always encoded as 1 byte.
		return 1, nil
	case t.Kind() == vdl.Array && t.IsBytes():
		// Byte arrays are exactly their length.
		return t.Len(), nil
	case t.Kind() == vdl.String || t.IsBytes():
		// Strings and byte lists are encoded with a length header.
		strlen, bytelen, err := binaryPeekUint(d.buf)
		switch {
		case err != nil:
			return 0, err
		case strlen > maxBinaryMsgLen:
			return 0, errMsgLen
		}
		return int(strlen) + bytelen, nil
	default:
		// Must be a primitive, which is encoded as an underlying uint.
		return binaryPeekUintByteLen(d.buf)
	}
}

// decodeValueMsg decodes the rest of the message assuming type t, handling the
// optional message length.
func (d *binaryDecoder) decodeValueMsg(t *vdl.Type, target valconv.Target) error {
	if hasBinaryMsgLen(t) {
		msgLen, err := binaryDecodeLen(d.buf)
		if err != nil {
			return err
		}
		d.buf.SetLimit(msgLen)
	}
	if err := d.decodeValue(t, target); err != nil {
		return err
	}
	if leftover := d.buf.RemoveLimit(); leftover > 0 {
		return verror.BadProtocolf("vom: %d leftover bytes", leftover)
	}
	return nil
}

// decodeValue decodes the rest of the message assuming type t.
func (d *binaryDecoder) decodeValue(t *vdl.Type, target valconv.Target) error {
	if t.IsBytes() {
		len, err := binaryDecodeLenOrArrayLen(d.buf, t)
		if err != nil {
			return err
		}
		bytes, err := d.buf.ReadBuf(len)
		if err != nil {
			return err
		}
		return target.FromBytes(bytes, t)
	}
	switch kind := t.Kind(); kind {
	case vdl.Bool:
		v, err := binaryDecodeBool(d.buf)
		if err != nil {
			return err
		}
		return target.FromBool(v, t)
	case vdl.Byte:
		v, err := d.buf.ReadByte()
		if err != nil {
			return err
		}
		return target.FromUint(uint64(v), t)
	case vdl.Uint16, vdl.Uint32, vdl.Uint64:
		v, err := binaryDecodeUint(d.buf)
		if err != nil {
			return err
		}
		return target.FromUint(v, t)
	case vdl.Int16, vdl.Int32, vdl.Int64:
		v, err := binaryDecodeInt(d.buf)
		if err != nil {
			return err
		}
		return target.FromInt(v, t)
	case vdl.Float32, vdl.Float64:
		v, err := binaryDecodeFloat(d.buf)
		if err != nil {
			return err
		}
		return target.FromFloat(v, t)
	case vdl.Complex64, vdl.Complex128:
		re, err := binaryDecodeFloat(d.buf)
		if err != nil {
			return err
		}
		im, err := binaryDecodeFloat(d.buf)
		if err != nil {
			return err
		}
		return target.FromComplex(complex(re, im), t)
	case vdl.String:
		v, err := binaryDecodeString(d.buf)
		if err != nil {
			return err
		}
		return target.FromString(v, t)
	case vdl.Enum:
		index, err := binaryDecodeUint(d.buf)
		switch {
		case err != nil:
			return err
		case index >= uint64(t.NumEnumLabel()):
			return errIndexOutOfRange
		}
		return target.FromEnumLabel(t.EnumLabel(int(index)), t)
	case vdl.TypeObject:
		id, err := binaryDecodeUint(d.buf)
		if err != nil {
			return err
		}
		typeobj, err := d.recvTypes.LookupOrBuildType(TypeID(id))
		if err != nil {
			return err
		}
		return target.FromTypeObject(typeobj)
	case vdl.Array, vdl.List:
		len, err := binaryDecodeLenOrArrayLen(d.buf, t)
		if err != nil {
			return err
		}
		listTarget, err := target.StartList(t, len)
		if err != nil {
			return err
		}
		for ix := 0; ix < len; ix++ {
			elem, err := listTarget.StartElem(ix)
			if err != nil {
				return err
			}
			if err := d.decodeValue(t.Elem(), elem); err != nil {
				return err
			}
			if err := listTarget.FinishElem(elem); err != nil {
				return err
			}
		}
		return target.FinishList(listTarget)
	case vdl.Set:
		len, err := binaryDecodeLen(d.buf)
		if err != nil {
			return err
		}
		setTarget, err := target.StartSet(t, len)
		if err != nil {
			return err
		}
		for ix := 0; ix < len; ix++ {
			key, err := setTarget.StartKey()
			if err != nil {
				return err
			}
			if err := d.decodeValue(t.Key(), key); err != nil {
				return err
			}
			switch err := setTarget.FinishKey(key); {
			case verror.Is(err, verror.NoExist):
				continue
			case err != nil:
				return err
			}
		}
		return target.FinishSet(setTarget)
	case vdl.Map:
		len, err := binaryDecodeLen(d.buf)
		if err != nil {
			return err
		}
		mapTarget, err := target.StartMap(t, len)
		if err != nil {
			return err
		}
		for ix := 0; ix < len; ix++ {
			key, err := mapTarget.StartKey()
			if err != nil {
				return err
			}
			if err := d.decodeValue(t.Key(), key); err != nil {
				return err
			}
			switch field, err := mapTarget.FinishKeyStartField(key); {
			case verror.Is(err, verror.NoExist):
				if err := d.ignoreValue(t.Elem()); err != nil {
					return err
				}
			case err != nil:
				return err
			default:
				if err := d.decodeValue(t.Elem(), field); err != nil {
					return err
				}
				if err := mapTarget.FinishField(key, field); err != nil {
					return err
				}
			}
		}
		return target.FinishMap(mapTarget)
	case vdl.Struct:
		fieldsTarget, err := target.StartFields(t)
		if err != nil {
			return err
		}
		// Loop through decoding the 1-based field index and corresponding field.
		for {
			index, err := binaryDecodeUint(d.buf)
			switch {
			case err != nil:
				return err
			case index > uint64(t.NumField()):
				return errIndexOutOfRange
			case index == 0:
				return target.FinishFields(fieldsTarget)
			}
			tfield := t.Field(int(index - 1))
			switch key, field, err := fieldsTarget.StartField(tfield.Name); {
			case verror.Is(err, verror.NoExist):
				if err := d.ignoreValue(tfield.Type); err != nil {
					return err
				}
			case err != nil:
				return err
			default:
				if err := d.decodeValue(tfield.Type, field); err != nil {
					return err
				}
				if err := fieldsTarget.FinishField(key, field); err != nil {
					return err
				}
			}
		}
	case vdl.OneOf:
		fieldsTarget, err := target.StartFields(t)
		if err != nil {
			return err
		}
		index, err := binaryDecodeUint(d.buf)
		switch {
		case err != nil:
			return err
		case index == 0 || index > uint64(t.NumField()):
			return errIndexOutOfRange
		}
		tfield := t.Field(int(index - 1))
		key, field, err := fieldsTarget.StartField(tfield.Name)
		if err != nil {
			return err
		}
		if err := d.decodeValue(tfield.Type, field); err != nil {
			return err
		}
		return fieldsTarget.FinishField(key, field)
	case vdl.Any:
		switch id, err := binaryDecodeUint(d.buf); {
		case err != nil:
			return err
		case id == 0:
			return target.FromNil(nil) // TODO: Fix nil handling.
		default:
			subType, err := d.recvTypes.LookupOrBuildType(TypeID(id))
			if err != nil {
				return err
			}
			return d.decodeValue(subType, target)
		}
	default:
		panic(verror.Internalf("vom: decodeValue unhandled type %v", t))
	}
}

// ignoreValue ignores the rest of the value of type t.  This is used to ignore
// unknown struct fields.
func (d *binaryDecoder) ignoreValue(t *vdl.Type) error {
	if t.IsBytes() {
		len, err := binaryDecodeLenOrArrayLen(d.buf, t)
		if err != nil {
			return err
		}
		return d.buf.Skip(len)
	}
	switch kind := t.Kind(); kind {
	case vdl.Bool, vdl.Byte:
		return d.buf.Skip(1)
	case vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64, vdl.Enum, vdl.TypeObject:
		// The underlying encoding of all these types is based on uint.
		return binaryIgnoreUint(d.buf)
	case vdl.Complex64, vdl.Complex128:
		// Complex is encoded as two floats, so we can simply ignore two uints.
		if err := binaryIgnoreUint(d.buf); err != nil {
			return err
		}
		return binaryIgnoreUint(d.buf)
	case vdl.String:
		return binaryIgnoreString(d.buf)
	case vdl.Array, vdl.List, vdl.Set, vdl.Map:
		len, err := binaryDecodeLenOrArrayLen(d.buf, t)
		if err != nil {
			return err
		}
		for ix := 0; ix < len; ix++ {
			if kind == vdl.Set || kind == vdl.Map {
				if err := d.ignoreValue(t.Key()); err != nil {
					return err
				}
			}
			if kind == vdl.Array || kind == vdl.List || kind == vdl.Map {
				if err := d.ignoreValue(t.Elem()); err != nil {
					return err
				}
			}
		}
		return nil
	case vdl.Struct:
		// Loop through decoding the 1-based field index and corresponding field.
		for {
			switch index, err := binaryDecodeUint(d.buf); {
			case err != nil:
				return err
			case index > uint64(t.NumField()):
				return errIndexOutOfRange
			case index == 0:
				return nil
			default:
				tfield := t.Field(int(index - 1))
				if err := d.ignoreValue(tfield.Type); err != nil {
					return err
				}
			}
		}
	case vdl.Any, vdl.OneOf:
		switch id, err := binaryDecodeUint(d.buf); {
		case err != nil:
			return err
		case id == 0:
			return nil
		default:
			subType, err := d.recvTypes.LookupOrBuildType(TypeID(id))
			if err != nil {
				return err
			}
			return d.ignoreValue(subType)
		}
	default:
		panic(verror.Internalf("vom: ignoreValue unhandled type %v", t))
	}
}
