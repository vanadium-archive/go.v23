package vom

import (
	"fmt"
	"io"

	"v.io/veyron/veyron2/wiretype"
)

// binaryTypeEncode performs binary vom encoding of VOM types.
type binaryTypeEncoder struct {
	writer     io.Writer                   // writer to output to
	es         *encState                   // encoder state, also contains buffer for output
	nextTypeID wiretype.TypeID             // next new type id to encode
	sentTypes  map[*rtInfo]wiretype.TypeID // cache of already encoded types
}

func newBinaryTypeEncoder(writer io.Writer) *binaryTypeEncoder {
	e := &binaryTypeEncoder{writer: writer,
		es:         &encState{},
		nextTypeID: wiretype.TypeIDFirst,
		sentTypes:  make(map[*rtInfo]wiretype.TypeID)}
	e.es.init()
	return e
}

// lookupTypeID returns the type id for rt.  The returned bool is true iff a new
// id was assigned, and thus its type definition needs to be transmitted.
func (e *binaryTypeEncoder) lookupTypeID(rti *rtInfo) (wiretype.TypeID, bool) {
	if id, ok := bootstrapTypeIDs[rti]; ok {
		// No need to send bootstrap types.
		return id, false
	}
	if id, ok := e.sentTypes[rti]; ok {
		// Either we've already sent this type, or encodeType will send it as it
		// traverses unexplored children.
		return id, false
	}
	// Assign a new id and put it in sentTypes.
	newID := e.nextTypeID
	e.nextTypeID++
	e.sentTypes[rti] = newID
	return newID, true
}

// encodeUnsentTypes ensures that all necessary type definitions for rv are
// transmitted before the value.  We always buffer types in stateT, separately
// from the value in stateV.
//
// This is slightly complicated, since we need to assign type ids for inner
// types before we encode outer types, and the type may be recursive.  The
// strategy is a depth-first pre-order through the type.  As we traverse the
// guarantee is that if we encode a type, we'll explore its unexplored children.
// This ensures all unsent types will be visited.
func (e *binaryTypeEncoder) encodeUnsentTypes(rti *rtInfo) (wiretype.TypeID, error) {
	id, isNew := e.lookupTypeID(rti)
	if isNew {
		if err := e.encodeType(rti); err != nil {
			return 0, err
		}
	}
	return id, nil
}

// encodeType performs recursive depth-first pre-order traversal of the type
// graph, encoding unexplored types along the way.  Any types that have already
// been explored (even if they're still pending encoding) terminate that branch
// of the traversal, to ensure termination for recursive types.
func (e *binaryTypeEncoder) encodeType(rti *rtInfo) error {
	// The type must be in sentTypes.
	id, ok := e.sentTypes[rti]
	if !ok {
		panic(fmt.Errorf("vom: encodeType rti isn't in sentTypes %+v", rti))
	}

	// Make the wire type; we may assign new type ids, but we never recursively
	// encode, to satisfy the pre-order traversal.
	wire, children := e.makeWireType(rti)

	// Encode the type message and write it out.
	msg, err := e.encodeTypeMsg(id, wire)
	if err != nil {
		return err
	}
	if _, err := e.writer.Write(msg); err != nil {
		return err
	}

	// Now continue the recursive pre-order traversal of the unexplored children.
	for _, child := range children {
		if err := e.encodeType(child); err != nil {
			return err
		}
	}
	return nil
}

// encodeTypeMsg encodes a type def message and returns the encoded bytes.
func (e *binaryTypeEncoder) encodeTypeMsg(id wiretype.TypeID, wire interface{}) ([]byte, error) {
	initBinaryMsg(e.es)
	if err := encodeBinaryInterface(e.es, e, ValueOf(&wire).Elem()); err != nil {
		return nil, err
	}
	return encodeBinaryMsg(e.es.buf, -int64(id)), nil // -id means a typedef follows
}

// makeWireType returns the WireType corresponding to rti, along with a list of
// unexplored children.
func (e *binaryTypeEncoder) makeWireType(rti *rtInfo) (interface{}, []*rtInfo) {
	var name string
	if rti.isNamed {
		// Only transmit the name if it's named; built-in and unnamed types don't
		// need their name to be sent over the wire.
		name = rti.name
	}
	if c := rti.customBinary; c != nil {
		// The type has a custom coder method, so encode the wire type as if it were
		// of the custom type with its original name.
		rti = c.rti
	}

	var isNew bool
	var children []*rtInfo
	switch rti.kind {
	case typeKindInterface, typeKindBool, typeKindString, typeKindByteSlice, typeKindByte, typeKindUint, typeKindInt, typeKindFloat:
		return wiretype.NamedPrimitiveType{Type: rti.id, Name: name, Tags: rti.tags}, nil
	case typeKindSlice:
		wt := wiretype.SliceType{Name: name, Tags: rti.tags}
		if wt.Elem, isNew = e.lookupTypeID(rti.elem); isNew {
			children = append(children, rti.elem)
		}
		return wt, children
	case typeKindArray:
		wt := wiretype.ArrayType{Name: name, Len: uint64(rti.len), Tags: rti.tags}
		if wt.Elem, isNew = e.lookupTypeID(rti.elem); isNew {
			children = append(children, rti.elem)
		}
		return wt, children
	case typeKindMap:
		wt := wiretype.MapType{Name: name, Tags: rti.tags}
		if wt.Key, isNew = e.lookupTypeID(rti.key); isNew {
			children = append(children, rti.key)
		}
		if wt.Elem, isNew = e.lookupTypeID(rti.elem); isNew {
			children = append(children, rti.elem)
		}
		return wt, children
	case typeKindStruct:
		wt := wiretype.StructType{Fields: make([]wiretype.FieldType, len(rti.fields)), Name: name, Tags: rti.tags}
		for fx, f := range rti.fields {
			wt.Fields[fx].Name = f.name
			if wt.Fields[fx].Type, isNew = e.lookupTypeID(f.info); isNew {
				children = append(children, f.info)
			}
		}
		return wt, children
	case typeKindPtr:
		wt := wiretype.PtrType{Name: name, Tags: rti.tags}
		if wt.Elem, isNew = e.lookupTypeID(rti.elem); isNew {
			children = append(children, rti.elem)
		}
		return wt, children
	default:
		panic(fmt.Errorf("vom: makeWireType.WireType unhandled rti %+v", rti))
	}
}
