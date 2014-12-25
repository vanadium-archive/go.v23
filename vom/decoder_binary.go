package vom

import (
	"fmt"
	"io"
	"reflect"

	"v.io/veyron/veyron2/wiretype"
)

// DecodeVOMBinary is a helper to decode VOM binary. This assumes that the user is decoding into
// an empty interface.
func DecodeVOMBinary(r io.Reader) (Value, error) {
	dec := NewDecoder(r)
	it := TypeOf([]interface{}{}).Elem()
	v := newPointerLike(newInterfaceLike(nil, it), PtrTo(it))
	if err := dec.DecodeValue(v); err != nil {
		return nil, fmt.Errorf("decode failed: %v", err)
	}
	return v.Elem(), nil
}

// decoderBinary decodes type and value messages from the binary format.
type decoderBinary struct {
	decbuf *decbuf
	state  decState
	// For managing type definitions.
	wireTypes map[wiretype.TypeID]interface{}
	wireDefs  map[wiretype.TypeID]*wireDef
}

func newDecoderBinary(decbuf *decbuf) *decoderBinary {
	return &decoderBinary{
		decbuf:    decbuf,
		wireTypes: make(map[wiretype.TypeID]interface{}),
		wireDefs:  make(map[wiretype.TypeID]*wireDef),
	}
}

// DecodeMsg decodes a single binary message with the given id, and returns true
// if it is a value message (rather than a type message).
func (d *decoderBinary) DecodeMsg(id int64, rv Value) (bool, error) {
	d.state.reset()
	// Negative ids indicate a type msg, positive ids indicate a value msg.
	isTypeMsg := id < 0
	if isTypeMsg || !primitiveTypeIDs[wiretype.TypeID(id)] {
		// All typedefs and non-primitive values always have a msglen.  We set a
		// limit to ensure subsequent reads can't read past the msglen.
		msglen, err := rawDecodeUint(d.decbuf)
		if err != nil {
			return !isTypeMsg, err
		}
		d.decbuf.SetLimit(int(msglen))
	}

	// Decode the type or value.  Typically we'd factor this logic into a separate
	// function, but that increases runtime by more than 10% in micro-benchmarks.
	ignoreValue := !rv.IsValid()
	var err error
	if isTypeMsg {
		err = d.decodeWireType(wiretype.TypeID(-id))
	} else {
		var def *wireDef
		if def, err = d.lookupWireDef(wiretype.TypeID(+id)); err == nil {
			if ignoreValue {
				// If we don't have a msglen limit we need to explicitly ignore the
				// encoded value; if we do we'll just skip the entire message.
				if !d.decbuf.HasLimit() {
					err = d.ignoreTypedValue(def)
				}
			} else {
				var rti *rtInfo
				if rti, err = lookupRTInfo(rv.Type()); err == nil {
					err = d.decodeTypedValue(def, rti, rv)
				}
			}
		}
	}

	// Consume the leftover bytes (if any) at the end of the message.
	leftover, err2 := d.decbuf.SkipToLimit()
	if err == nil {
		// Only propagate leftover-skipping errors if we were error-free.
		if err2 != nil {
			err = err2
		} else if leftover > 0 && (isTypeMsg || !ignoreValue) {
			err = fmt.Errorf("vom: decoding saw %d leftover bytes", leftover)
		}
	}
	return !isTypeMsg, err
}

// decodeWireType decodes a WireType value and stores it in our wireTypes map
// under the given id.  We cannot define the type at this point, since we might
// not have seen all the inner types yet; we allow mutually-recursive types.  So
// we postpone type definition until the first time a value of the type is
// encountered, at which point defineType will be invoked.
func (d *decoderBinary) decodeWireType(id wiretype.TypeID) error {
	var wire interface{}
	rvWire := ValueOf(&wire).Elem()
	rtiWire, err := lookupRTInfo(rvWire.Type())
	if err != nil {
		return err
	}
	if err := d.decodeInterface(rtiWire, rvWire); err != nil {
		return err
	}
	// TODO(toddw): Allow duplicate compatible wire types (i.e. new fields ok)
	if exist, ok := d.wireTypes[id]; ok {
		return fmt.Errorf("vom: duplicate wiredef for id %v, %#v and %#v", id, exist, wire)
	}
	d.wireTypes[id] = wire
	return nil
}

// lookupWireDef returns the wiredef given the id of a concrete type.
func (d *decoderBinary) lookupWireDef(id wiretype.TypeID) (*wireDef, error) {
	// Bootstrap wire definitions are special-cased.
	if def, ok := bootstrapWireDefsFromID[id]; ok {
		return def, nil
	}
	if id < wiretype.TypeIDFirst {
		return nil, fmt.Errorf("vom: unknown bootstrap type id %d", id)
	}
	// Lookup in defined types.  This acts as a cache of types that have already
	// been defined, and also serves to terminate lookups for recursive types.
	if def, ok := d.wireDefs[id]; ok {
		return def, nil
	}
	// Convert a wire type into a defined type.
	wire, ok := d.wireTypes[id]
	if !ok {
		return nil, fmt.Errorf("vom: unknown user type id %d", id)
	}
	delete(d.wireTypes, id)
	return d.defineType(id, wire)
}

// defineType defines the type described in wire and stores it in wireDefs with
// key id.  Subsequent lookups of the id will return the stored wireDef.
func (d *decoderBinary) defineType(id wiretype.TypeID, wire interface{}) (*wireDef, error) {
	// Each case follows the same pattern - first we create a def from the info in
	// wt, and add it to wireDefs.  Then we lookup inner types and determine the
	// reflect.Type to use for generic decoding.  It's crucial that we add the def
	// to wireDefs before lookupWireDef is called on inner types; we allow cyclic
	// type graphs, and the entry in wireDefs breaks the cycle.
	var err error
	def := &wireDef{}
	d.wireDefs[id] = def
	switch wt := wire.(type) {
	case wiretype.NamedPrimitiveType:
		// MetaType is a special-case.  We lookup the underlying type and simply
		// copy its def, overwriting its name, tags and rt as appropriate.
		var tmp *wireDef
		if tmp, err = d.lookupWireDef(wt.Type); err != nil {
			return nil, err
		}
		*def = *tmp
		def.name = wt.Name
		def.tags = wt.Tags
	case wiretype.SliceType:
		def.name = wt.Name
		def.tags = wt.Tags
		def.kind = typeKindSlice
		if def.elem, err = d.lookupWireDef(wt.Elem); err != nil {
			return nil, err
		}
	case wiretype.ArrayType:
		def.name = wt.Name
		def.tags = wt.Tags
		def.len = int(wt.Len)
		def.kind = typeKindArray
		if def.elem, err = d.lookupWireDef(wt.Elem); err != nil {
			return nil, err
		}
	case wiretype.MapType:
		def.name = wt.Name
		def.tags = wt.Tags
		def.kind = typeKindMap
		if def.key, err = d.lookupWireDef(wt.Key); err != nil {
			return nil, err
		}
		if def.elem, err = d.lookupWireDef(wt.Elem); err != nil {
			return nil, err
		}
	case wiretype.StructType:
		def.name = wt.Name
		def.tags = wt.Tags
		def.kind = typeKindStruct
		def.fields = make([]wireDefField, len(wt.Fields))
		for fx := 0; fx < len(wt.Fields); fx++ {
			def.fields[fx].name = wt.Fields[fx].Name
			if def.fields[fx].def, err = d.lookupWireDef(wt.Fields[fx].Type); err != nil {
				return nil, err
			}
		}
	case wiretype.PtrType:
		def.name = wt.Name
		def.tags = wt.Tags
		def.kind = typeKindPtr
		def.numStars = 1
		if def.elem, err = d.lookupWireDef(wt.Elem); err != nil {
			return nil, err
		}
	default:
		panic(fmt.Errorf("vom: defineType unhandled wire type %#v type %T", wire, wire))
	}
	def.define()
	return def, nil
}

// decodeTypedValue is identical to the same method in decoderJSON - it calls
// decodeTypedValueHelper for actual per-kind decoding.  Trying to share the
// code incurs a 2x slowdown.  Keep the two versions in sync.
func (d *decoderBinary) decodeTypedValue(def *wireDef, rtin *rtInfo, v Value) error {
	// Here's the pointer handling strategy.  If def and rv have the same
	// structure (i.e. same number of pointers), we want pointer sharing and nils
	// to be preserved in the decoded result.  If rv has more stars than def, we
	// should be able to dereference the decoded result until they have the same
	// number of stars, and again the pointer sharing and nils should be
	// preserved.
	//
	// We use alignStars to make this happen.  If rv is a pointer type and has
	// more stars than def, alignStars will dereference until we end up with the
	// same number of stars.  Thereafter calls to decodePtr will walk def and rv
	// in lockstep, re-creating the same sharing structure.
	//
	// There is a catch wrt interfaces.  Let's say the encoded value represented
	// by def is ***interface{}, where the interface holds concrete value **int.
	// Intuitively we'd expect to be able to decode into a concrete *****int and
	// get the proper sharing.  However this would be complicated to implement,
	// since we don't know the type of the concrete value held in the interface
	// until we actually decode it.  So we punt on this unusual case.
	rti, rv := alignStars(def, rtin, v, FormatBinary)

	if rti.kind != typeKindInterface || def.kind == typeKindInterface {
		// Either the item we're decoding into (rti) isn't an interface, or both the
		// item we're decoding into and the encoded value (def) are interfaces, so
		// pass through to the helper directly.  For the latter case we let the
		// decodeInterface logic run; if the encoded value is nil, rv will also be
		// set to nil, which is the zero value.  Otherwise decodeTypedValue will be
		// recursively called again, but with the concrete def.
		return d.decodeTypedValueHelper(def, rti, rv)
	}

	// The item we're decoding into is an interface, and the encoded value is
	// concrete.  We must fill in rvIface with a value and call rv.Set afterwards,
	// since interfaces have value semantics and can't be modified by side-effect.
	var rvIface Value
	switch {
	case !rv.IsNil():
		// Create a new instance of the underlying concrete value to fill in.
		rvIface = New(rv.Elem().Type()).Elem()
	case def.rt != nil:
		// We're performing generic decoding into a nil interface - create the
		// appropriate type of value based on the encoded value (def).  Note that
		// rv may be a nil interface with actual methods defined, so we need to
		// check assignability.
		if rti.numMethods > 0 && !def.rt.AssignableTo(rv.Type()) {
			return fmt.Errorf("vom: can't assign type %q to value of type %q", def.rt, rv.Type())
		}
		rvIface = New(def.rt).Elem()
	default:
		return fmt.Errorf("vom: type not registered or creatable, type %q", def)
	}
	rtiIface, err := lookupRTInfo(rvIface.Type())
	if err != nil {
		return err
	}
	if err := d.decodeTypedValueHelper(def, rtiIface, rvIface); err != nil {
		return err
	}
	return rv.trySet(rvIface)
}

func (d *decoderBinary) decodeTypedValueHelper(def *wireDef, rti *rtInfo, rv Value) error {
	var customDecode *customDecode
	rti, rv, customDecode = rti.initCustomDecode(rv, FormatBinary)

	// We need to align the pointers again since the argument to the VomDecode
	// method may be a pointer type.
	rti, rv = alignStars(def, rti, rv, FormatBinary)

	// Decode the encoded value into rv.
	var err error
	switch def.kind {
	case typeKindBool:
		err = decodeBool(d.decbuf, rti, rv)
	case typeKindString, typeKindByteSlice:
		err = decodeString(d.decbuf, rti, rv)
	case typeKindByte:
		err = decodeByte(d.decbuf, rti, rv)
	case typeKindUint:
		err = decodeUint(d.decbuf, rti, rv)
	case typeKindInt:
		err = decodeInt(d.decbuf, rti, rv)
	case typeKindFloat:
		err = decodeFloat(d.decbuf, rti, rv)
	case typeKindSlice:
		err = d.decodeSlice(def, rti, rv)
	case typeKindArray:
		// TODO(toddw): Check if special-casing byte arrays is worth it.
		err = d.decodeArray(def, rti, rv)
	case typeKindMap:
		err = d.decodeMap(def, rti, rv)
	case typeKindStruct:
		err = d.decodeStruct(def, rti, rv)
	case typeKindPtr:
		err = d.decodePtr(def, rti, rv)
	case typeKindInterface:
		err = d.decodeInterface(rti, rv)
	default:
		panic(fmt.Errorf("vom: decodeTypedValue unhandled kind %v def %#v", def.kind, def))
	}
	if err != nil {
		return err
	}
	// Call the custom decoder method if it exists, otherwise we've already
	// decoded into the final value by side-effect.
	if customDecode != nil {
		return customDecode.call()
	}
	return nil
}

func (d *decoderBinary) decodeSlice(def *wireDef, rti *rtInfo, rv Value) error {
	// Slices may be decoded into either slices or arrays with compatible elems.
	ulen, err := rawDecodeUint(d.decbuf)
	if err != nil {
		return err
	}
	len := int(ulen)
	switch rti.rt.Kind() {
	case reflect.Slice:
		if len <= rv.Cap() {
			rv.SetLen(len)
		} else {
			rv.Set(MakeSlice(rti.rt, len, len))
		}
		for ix := 0; ix < len; ix++ {
			if err := d.decodeTypedValue(def.elem, rti.elem, rv.Index(ix)); err != nil {
				return err
			}
		}
		return nil
	case reflect.Array:
		// If there are more encoded items than the array can hold we drop through
		// to the final error.  If there are fewer encoded items, the extra array
		// items are left untouched.
		if len <= rti.rt.Len() {
			for ix := 0; ix < len; ix++ {
				if err := d.decodeTypedValue(def.elem, rti.elem, rv.Index(ix)); err != nil {
					return err
				}
			}
			return nil
		}
	}
	return fmt.Errorf("vom: type mismatch, can't decode slice %q into value of type %q", def, rti.rt)
}

func (d *decoderBinary) decodeArray(def *wireDef, rti *rtInfo, rv Value) error {
	// Arrays may be decoded into either slices or arrays with compatible elems.
	switch rti.rt.Kind() {
	case reflect.Slice:
		if def.len <= rv.Cap() {
			rv.SetLen(def.len)
		} else {
			rv.Set(MakeSlice(rti.rt, def.len, def.len))
		}
		for ix := 0; ix < def.len; ix++ {
			if err := d.decodeTypedValue(def.elem, rti.elem, rv.Index(ix)); err != nil {
				return err
			}
		}
		return nil
	case reflect.Array:
		// If there are more encoded items than the array can hold we drop through
		// to the final error.  If there are fewer encoded items, the extra array
		// items are left untouched.
		if def.len <= rti.rt.Len() {
			for ix := 0; ix < def.len; ix++ {
				if err := d.decodeTypedValue(def.elem, rti.elem, rv.Index(ix)); err != nil {
					return err
				}
			}
			return nil
		}
	}
	return fmt.Errorf("vom: type mismatch, can't decode array %q into value of type %q", def, rti.rt)
}

func (d *decoderBinary) decodeMap(def *wireDef, rti *rtInfo, rv Value) error {
	// Maps may only be decoded into a map with compatible key and elem types.
	ulen, err := rawDecodeUint(d.decbuf)
	if err != nil {
		return err
	}
	len := int(ulen)
	if rti.kind == typeKindMap {
		if len == 0 {
			if rv.Len() != 0 {
				// If the encoded map is empty, set rv to nil if it's not already empty.
				rv.Set(rti.zero)
			}
			return nil
		}
		rv.Set(MakeMap(rti.rt))
		key := New(rti.key.rt).Elem()
		elem := New(rti.elem.rt).Elem()
		for ix := 0; ix < len; ix++ {
			key.Set(rti.key.zero)
			if err := d.decodeTypedValue(def.key, rti.key, key); err != nil {
				return err
			}
			elem.Set(rti.elem.zero)
			if err := d.decodeTypedValue(def.elem, rti.elem, elem); err != nil {
				return err
			}
			rv.SetMapIndex(key, elem)
		}
		return nil
	}
	return fmt.Errorf("vom: type mismatch, can't decode map %q into value of type %q", def, rti.rt)
}

func (d *decoderBinary) decodeStruct(def *wireDef, rti *rtInfo, rv Value) error {
	// Structs may be decoded into a struct with compatible fields - i.e. for
	// every field with the same name, the types must be compatible.
	//
	// TODO(toddw): Throw an error if the encoded and decoded structs have no
	// fields in common.  We should only perform that check once per encode /
	// decode type pair - it'll be easier to add when we optimize the code.
	if rti.kind == typeKindStruct {
		// Iterate through each encoded field.  Every field starts with a delta of
		// the field index, with delta 0 acting as an ending sentry.
		fx := -1
		for {
			// Decode the delta for this iteration.
			delta, err := rawDecodeUint(d.decbuf)
			if err != nil {
				return err
			}
			if delta == 0 {
				break // Saw the 0 ending sentry.
			}
			fx += int(delta)
			if fx >= len(def.fields) {
				return fmt.Errorf("vom: struct %q doesn't have field #%d", def, fx)
			}
			field := def.fields[fx]
			if def.rt != nil && def.rt == rti.rt {
				// Same types - decode into the same field index.  We still must use the
				// index from rti.fields since there may be unexported fields.
				rtf := rti.fields[fx]
				err = d.decodeTypedValue(field.def, rtf.info, rv.Field(rtf.index))
			} else if findex, finfo := rti.fieldByName(field.name); findex != -1 {
				// Decode into the matching field name.
				err = d.decodeTypedValue(field.def, finfo, rv.Field(findex))
			} else {
				err = d.ignoreTypedValue(field.def)
			}
			if err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("vom: type mismatch, can't decode struct %q into value of type %q", def, rti.rt)
}

func (d *decoderBinary) decodePtr(def *wireDef, rti *rtInfo, rv Value) error {
	// Pointers are special - the number of "stars" on the encoded and decoded
	// types may be different.
	//
	// There are many corner-cases regarding replicating the sharing; e.g. what if
	// two struct fields are shared in the encoded value, but the decoded value
	// represents those fields using different numbers of stars, or even different
	// (but compatible) types?  Or what if the user passes in pre-allocated
	// pointers that create a different sharing structure?  And how about pointers
	// hidden behind interfaces?
	//
	// To keep the implementation reasonable, the rule is that if there are the
	// same number of stars on the encoded and decoded types, the decoded value
	// will replicate the sharing from the encoded value.  Otherwise the sharing
	// in the decoded value may be arbitrary.
	refID, err := rawDecodeInt(d.decbuf)
	if err != nil {
		return err
	}

	switch {
	case refID == 0:
		// 0 represents a nil pointer; zero out rv and we're done.
		// TODO(toddw): Check compatibility between def and rti.
		rv.Set(rti.zero)
		return nil
	case refID > 0:
		// +refID represents a reference to a previously encountered value.
		return d.state.fillFromRefID(refID, rti, rv)
	default:
		// -refID defines a new refID along with its associated value.
		if rti, rv, err = d.state.defineRefID(-refID, rti, rv); err != nil {
			return err
		}
		return d.decodeTypedValue(def.elem, rti, rv)
	}
}

func (d *decoderBinary) decodeInterface(rti *rtInfo, rv Value) error {
	// Interfaces start with the wiretype.TypeID of the concrete type.
	concreteID, err := rawDecodeInt(d.decbuf)
	if err != nil {
		return err
	}

	// 0 represents a nil interface; zero out rv and we're done.  Note that this
	// means a nil interface can be decoded into an rv of any type.
	if concreteID == 0 {
		rv.Set(rti.zero)
		return nil
	}

	// Just decode the value as usual using the concrete id.
	def, err := d.lookupWireDef(wiretype.TypeID(concreteID))
	if err != nil {
		return err
	}
	return d.decodeTypedValue(def, rti, rv)
}

// The ignore* functions are just like the decode* functions, except they ignore
// the encoded value, and thus we don't need a Value to decode into.
// This is more than an optimization; we might not be able to construct a proper
// Value for regular decoding, so we really need these functions.

func (d *decoderBinary) ignoreTypedValue(def *wireDef) error {
	switch def.kind {
	case typeKindByte, typeKindBool:
		return d.decbuf.Skip(1)
	case typeKindUint, typeKindInt, typeKindFloat:
		// These types are all fundamentally encoded as a uint.
		return rawIgnoreUint(d.decbuf)
	case typeKindString, typeKindByteSlice:
		return rawIgnoreByteSlice(d.decbuf)
	case typeKindSlice:
		return d.ignoreSlice(def)
	case typeKindArray:
		return d.ignoreArray(def)
	case typeKindMap:
		return d.ignoreMap(def)
	case typeKindStruct:
		return d.ignoreStruct(def)
	case typeKindPtr:
		return d.ignorePtr(def)
	case typeKindInterface:
		return d.ignoreInterface()
	default:
		panic(fmt.Errorf("vom: ignoreTypedValue unhandled kind %v def %#v", def.kind, def))
	}
}

func (d *decoderBinary) ignoreSlice(def *wireDef) error {
	ulen, err := rawDecodeUint(d.decbuf)
	if err != nil {
		return err
	}
	for ix := 0; ix < int(ulen); ix++ {
		if err := d.ignoreTypedValue(def.elem); err != nil {
			return err
		}
	}
	return nil
}

func (d *decoderBinary) ignoreArray(def *wireDef) error {
	for ix := 0; ix < def.len; ix++ {
		if err := d.ignoreTypedValue(def.elem); err != nil {
			return err
		}
	}
	return nil
}

func (d *decoderBinary) ignoreMap(def *wireDef) error {
	ulen, err := rawDecodeUint(d.decbuf)
	if err != nil {
		return err
	}
	for ix := 0; ix < int(ulen); ix++ {
		if err := d.ignoreTypedValue(def.key); err != nil {
			return err
		}
		if err := d.ignoreTypedValue(def.elem); err != nil {
			return err
		}
	}
	return nil
}

func (d *decoderBinary) ignoreStruct(def *wireDef) error {
	fx := -1
	for {
		delta, err := rawDecodeUint(d.decbuf)
		if err != nil {
			return err
		}
		if delta == 0 {
			break
		}
		fx += int(delta)
		if fx >= len(def.fields) {
			return fmt.Errorf("vom: struct %q doesn't have field #%d", def, fx)
		}
		field := def.fields[fx]
		if err := d.ignoreTypedValue(field.def); err != nil {
			return err
		}
	}
	return nil
}

func (d *decoderBinary) ignorePtr(def *wireDef) error {
	refID, err := rawDecodeInt(d.decbuf)
	if err != nil {
		return err
	}
	if refID < 0 {
		return d.ignoreTypedValue(def.elem)
	}
	return nil
}

func (d *decoderBinary) ignoreInterface() error {
	concreteID, err := rawDecodeInt(d.decbuf)
	if err != nil {
		return err
	}
	if concreteID != 0 {
		def, err := d.lookupWireDef(wiretype.TypeID(concreteID))
		if err != nil {
			return err
		}
		return d.ignoreTypedValue(def)
	}
	return nil
}
