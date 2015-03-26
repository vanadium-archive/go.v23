// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"io"
	"math"
	"sync"

	"v.io/v23/vdl"
	"v.io/v23/verror"
)

var (
	errEncodeTypeIdOverflow = verror.Register(pkgPath+".errEncodeTypeIdOverflow", verror.NoRetry, "{1:}{2:} vom: encoder type id overflow{:_}")
	errUnhandledType        = verror.Register(pkgPath+".errUnhandledType", verror.NoRetry, "{1:}{2:} vom: encode unhandled type {3}{:_}")
)

// TypeEncoder manages the transmission and marshaling of types to the other
// side of a connection.
type TypeEncoder struct {
	typeMu   sync.RWMutex
	typeToId map[*vdl.Type]typeId // GUARDED_BY(typeMu)
	nextId   typeId               // GUARDED_BY(typeMu)

	encMu sync.Mutex
	enc   *Encoder // GUARDED_BY(encMu)
}

// NewTypeEncoder returns a new TypeEncoder that writes types to the given
// writer in the binary format.
func NewTypeEncoder(w io.Writer) (*TypeEncoder, error) {
	if err := writeMagicByte(w); err != nil {
		return nil, err
	}
	return newTypeEncoder(w), nil
}

func newTypeEncoder(w io.Writer) *TypeEncoder {
	return &TypeEncoder{
		typeToId: make(map[*vdl.Type]typeId),
		nextId:   WireIdFirstUserType,
		enc:      newEncoder(w, nil),
	}
}

// encode encodes the wire type tt recursively in depth-first order, encoding
// any children of the type before the type itself. Type ids are allocated in
// the order that we recurse and consequentially may be sent out of sequential
// order if type information for children is sent (before the parent type).
func (e *TypeEncoder) encode(tt *vdl.Type) (typeId, error) {
	// Lookup a type Id for tt or assign a new one.
	tid, isNew, err := e.lookupOrAssignTypeId(tt)
	if err != nil {
		return 0, err
	}
	if !isNew {
		return tid, nil
	}

	// Construct the wireType.
	var wt wireType
	switch kind := tt.Kind(); kind {
	case vdl.Bool, vdl.Byte, vdl.String, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64, vdl.Complex64, vdl.Complex128:
		wt = wireTypeNamedT{wireNamed{tt.Name(), bootstrapKindToId[kind]}}
	case vdl.Enum:
		wireEnum := wireEnum{tt.Name(), make([]string, tt.NumEnumLabel())}
		for ix := 0; ix < tt.NumEnumLabel(); ix++ {
			wireEnum.Labels[ix] = tt.EnumLabel(ix)
		}
		wt = wireTypeEnumT{wireEnum}
	case vdl.Array:
		elm, err := e.encode(tt.Elem())
		if err != nil {
			return 0, err
		}
		wt = wireTypeArrayT{wireArray{tt.Name(), elm, uint64(tt.Len())}}
	case vdl.List:
		elm, err := e.encode(tt.Elem())
		if err != nil {
			return 0, err
		}
		wt = wireTypeListT{wireList{tt.Name(), elm}}
	case vdl.Set:
		key, err := e.encode(tt.Key())
		if err != nil {
			return 0, err
		}
		wt = wireTypeSetT{wireSet{tt.Name(), key}}
	case vdl.Map:
		key, err := e.encode(tt.Key())
		if err != nil {
			return 0, err
		}
		elm, err := e.encode(tt.Elem())
		if err != nil {
			return 0, err
		}
		wt = wireTypeMapT{wireMap{tt.Name(), key, elm}}
	case vdl.Struct:
		wireStruct := wireStruct{tt.Name(), make([]wireField, tt.NumField())}
		for ix := 0; ix < tt.NumField(); ix++ {
			field, err := e.encode(tt.Field(ix).Type)
			if err != nil {
				return 0, err
			}
			wireStruct.Fields[ix] = wireField{tt.Field(ix).Name, field}
		}
		wt = wireTypeStructT{wireStruct}
	case vdl.Union:
		wireUnion := wireUnion{tt.Name(), make([]wireField, tt.NumField())}
		for ix := 0; ix < tt.NumField(); ix++ {
			field, err := e.encode(tt.Field(ix).Type)
			if err != nil {
				return 0, err
			}
			wireUnion.Fields[ix] = wireField{tt.Field(ix).Name, field}
		}
		wt = wireTypeUnionT{wireUnion}
	case vdl.Optional:
		elm, err := e.encode(tt.Elem())
		if err != nil {
			return 0, err
		}
		wt = wireTypeOptionalT{wireOptional{tt.Name(), elm}}
	default:
		panic(verror.New(errUnhandledType, nil, tt))
	}

	// Encode and write the wire type definition.
	//
	// We use the same binary encoder as values for wire types.
	e.encMu.Lock()
	defer e.encMu.Unlock()
	if err := e.enc.encodeWireType(tid, wt); err != nil {
		return 0, err
	}
	return tid, nil
}

// lookupTypeId returns the id for the type tt if it is already encoded;
// otherwise zero id is returned.
func (e *TypeEncoder) lookupTypeId(tt *vdl.Type) typeId {
	if tid := bootstrapTypeToId[tt]; tid != 0 {
		return tid
	}
	e.typeMu.RLock()
	tid := e.typeToId[tt]
	e.typeMu.RUnlock()
	return tid
}

func (e *TypeEncoder) lookupOrAssignTypeId(tt *vdl.Type) (typeId, bool, error) {
	if tid := bootstrapTypeToId[tt]; tid != 0 {
		return tid, false, nil
	}
	e.typeMu.RLock()
	tid := e.typeToId[tt]
	e.typeMu.RUnlock()
	if tid > 0 {
		return tid, false, nil
	}

	e.typeMu.Lock()
	// Check it again to avoid a race.
	if tid := e.typeToId[tt]; tid > 0 {
		e.typeMu.Unlock()
		return tid, false, nil
	}
	// Assign a new id.
	newId := e.nextId
	if newId > math.MaxInt64 {
		e.typeMu.Unlock()
		return 0, false, verror.New(errEncodeTypeIdOverflow, nil)
	}
	e.nextId++
	e.typeToId[tt] = newId
	e.typeMu.Unlock()
	return newId, true, nil
}
