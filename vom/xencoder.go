// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"errors"
	"fmt"
	"io"

	"v.io/v23/vdl"
	"v.io/v23/verror"
)

var (
	errEmptyEncoderStack = errors.New("vom: empty encoder stack")
)

type XEncoder struct {
	enc xEncoder
}

func NewXEncoder(w io.Writer) *XEncoder {
	return NewVersionedXEncoder(DefaultVersion, w)
}

func NewXEncoderWithTypeEncoder(w io.Writer, typeEnc *TypeEncoder) *XEncoder {
	return NewVersionedXEncoderWithTypeEncoder(DefaultVersion, w, typeEnc)
}

func NewVersionedXEncoder(version Version, w io.Writer) *XEncoder {
	if !isAllowedVersion(version) {
		panic(fmt.Sprintf("unsupported VOM version: %x", version))
	}
	return &XEncoder{xEncoder{
		writer:          w,
		buf:             newEncbuf(),
		typeEnc:         newTypeEncoderWithoutVersionByte(version, w),
		sentVersionByte: false,
		version:         version,
	}}
}

func NewVersionedXEncoderWithTypeEncoder(version Version, w io.Writer, typeEnc *TypeEncoder) *XEncoder {
	if !isAllowedVersion(version) {
		panic(fmt.Sprintf("unsupported VOM version: %x", version))
	}
	return &XEncoder{xEncoder{
		writer:          w,
		buf:             newEncbuf(),
		typeEnc:         typeEnc,
		sentVersionByte: false,
		version:         version,
	}}
}

func (e *XEncoder) Encoder() vdl.Encoder {
	return &e.enc
}

func (e *XEncoder) Encode(v interface{}) error {
	return vdl.Write(&e.enc, v)
}

type xEncoder struct {
	writer io.Writer
	// We use buf to buffer up the encoded value. The buffering is necessary so
	// that we can compute the total message length.
	buf *encbuf
	// Buffer for the header of messages with any or typeobject.
	bufHeader *encbuf
	// All types are sent through typeEnc.
	typeEnc         *TypeEncoder
	sentVersionByte bool
	version         Version

	tids    *typeIDList
	anyLens *anyLenList
	// TODO(bprosnitz) get rid of these fields
	hasLen, hasAny, hasTypeObject bool
	typeIncomplete                bool
	mid                           int64
	nextStartValueOptional        bool

	stack []encoderStackEntry
}

type encoderStackEntry struct {
	Type       *vdl.Type
	Index      int
	LenHint    int
	NumStarted int
	AnyRef     anyStartRef
}

// We can only determine whether the next value is AnyType
// by checking the next type of the entry.
func (entry *encoderStackEntry) nextValueIsAny() bool {
	if entry == nil {
		return false
	}
	switch entry.Type.Kind() {
	case vdl.List, vdl.Array:
		return entry.Type.Elem() == vdl.AnyType
	case vdl.Set:
		return entry.Type.Key() == vdl.AnyType
	case vdl.Map:
		switch entry.NumStarted % 2 {
		case 0:
			return entry.Type.Key() == vdl.AnyType
		case 1:
			return entry.Type.Elem() == vdl.AnyType
		}
	case vdl.Struct, vdl.Union:
		return entry.Type.Field(entry.Index).Type == vdl.AnyType
	}
	return false
}

func (e *xEncoder) top() *encoderStackEntry {
	if len(e.stack) == 0 {
		return nil
	}
	return &e.stack[len(e.stack)-1]
}

func (e *xEncoder) SetNextStartValueIsOptional() {
	e.nextStartValueOptional = true
}

func (e *xEncoder) NilValue(tt *vdl.Type) error {
	switch tt.Kind() {
	case vdl.Any, vdl.Optional:
	default:
		return fmt.Errorf("concrete types disallowed for NilValue (type was %v)", tt)
	}

	if len(e.stack) == 0 {
		if err := e.startMessage(tt); err != nil {
			return err
		}
	}

	nextValueIsAny := e.top().nextValueIsAny()
	var anyRef anyStartRef
	if nextValueIsAny {
		tid, err := e.typeEnc.encode(tt)
		if err != nil {
			return err
		}
		binaryEncodeUint(e.buf, e.tids.ReferenceTypeID(tid))
		anyRef = e.anyLens.StartAny(e.buf.Len())
		binaryEncodeUint(e.buf, uint64(anyRef.index))
	}
	binaryEncodeControl(e.buf, WireCtrlNil)
	if nextValueIsAny {
		e.anyLens.FinishAny(anyRef, e.buf.Len())
	}

	if len(e.stack) == 0 {
		if err := e.finishMessage(); err != nil {
			return err
		}
	}
	return nil
}

func (e *xEncoder) StartValue(tt *vdl.Type) error {
	switch tt.Kind() {
	case vdl.Any, vdl.Optional:
		return fmt.Errorf("only concrete types allowed for StartValue (type was %v)", tt)
	}

	if len(e.stack) == 0 {
		msgType := tt
		if e.nextStartValueOptional {
			msgType = vdl.OptionalType(tt)
		}
		if err := e.startMessage(msgType); err != nil {
			return err
		}
	}

	top := e.top()
	if top != nil {
		top.NumStarted++
	}

	var anyRef anyStartRef
	if top.nextValueIsAny() {
		anyType := tt
		if e.nextStartValueOptional {
			anyType = vdl.OptionalType(tt)
		}
		tid, err := e.typeEnc.encode(anyType)
		if err != nil {
			return err
		}
		binaryEncodeUint(e.buf, e.tids.ReferenceTypeID(tid))
		anyRef = e.anyLens.StartAny(e.buf.Len())
		binaryEncodeUint(e.buf, uint64(anyRef.index))
	}

	e.stack = append(e.stack, encoderStackEntry{
		Type:    tt,
		AnyRef:  anyRef,
		Index:   -1,
		LenHint: -1,
	})
	e.nextStartValueOptional = false

	return nil
}

func (e *xEncoder) startMessage(tt *vdl.Type) error {
	if !e.sentVersionByte {
		if _, err := e.writer.Write([]byte{byte(e.version)}); err != nil {
			return err
		}
		e.sentVersionByte = true
	}
	tid, err := e.typeEnc.encode(tt)
	if err != nil {
		return err
	}
	e.buf.Reset()
	e.buf.Grow(paddingLen)
	e.hasLen = hasChunkLen(tt)
	e.hasAny = containsAny(tt)
	e.hasTypeObject = containsTypeObject(tt)
	e.typeIncomplete = false
	e.mid = int64(tid)
	if e.hasAny || e.hasTypeObject {
		e.tids = newTypeIDList()
	} else {
		e.tids = nil
	}
	if e.hasAny {
		e.anyLens = newAnyLenList()
	} else {
		e.anyLens = nil
	}
	return nil
}

func (e *xEncoder) FinishValue() error {
	top := e.top()
	if top == nil {
		return errEmptyDecoderStack
	}
	e.stack = e.stack[:len(e.stack)-1]
	if e.top().nextValueIsAny() {
		e.anyLens.FinishAny(top.AnyRef, e.buf.Len())
	}
	if len(e.stack) == 0 {
		if err := e.finishMessage(); err != nil {
			return err
		}
	}
	return nil
}

func (e *xEncoder) finishMessage() error {
	if e.typeIncomplete {
		if _, err := e.writer.Write([]byte{WireCtrlTypeIncomplete}); err != nil {
			return err
		}
	}
	if e.hasAny || e.hasTypeObject {
		ids := e.tids.NewIDs()
		var anyLens []int
		if e.hasAny {
			anyLens = e.anyLens.NewAnyLens()
		}
		if e.bufHeader == nil {
			e.bufHeader = newEncbuf()
		} else {
			e.bufHeader.Reset()
		}
		binaryEncodeInt(e.bufHeader, e.mid)
		binaryEncodeUint(e.bufHeader, uint64(len(ids)))
		for _, id := range ids {
			binaryEncodeUint(e.bufHeader, uint64(id))
		}
		if e.hasAny {
			binaryEncodeUint(e.bufHeader, uint64(len(anyLens)))
			for _, anyLen := range anyLens {
				binaryEncodeUint(e.bufHeader, uint64(anyLen))
			}
		}
		msg := e.buf.Bytes()
		if e.hasLen {
			binaryEncodeUint(e.bufHeader, uint64(len(msg)-paddingLen))
		}
		if _, err := e.writer.Write(e.bufHeader.Bytes()); err != nil {
			return err
		}
		_, err := e.writer.Write(msg[paddingLen:])
		return err
	}
	msg := e.buf.Bytes()
	header := msg[:paddingLen]
	if e.hasLen {
		start := binaryEncodeUintEnd(header, uint64(len(msg)-paddingLen))
		header = header[:start]
	}
	start := binaryEncodeIntEnd(header, e.mid)
	_, err := e.writer.Write(msg[start:])
	return err
}

func (e *xEncoder) NextEntry(done bool) error {
	top := e.top()
	if top == nil {
		return errEmptyEncoderStack
	}
	top.Index++
	if top.Index == 0 {
		switch {
		case top.Type.Kind() == vdl.Array:
			binaryEncodeUint(e.buf, 0)
		case top.LenHint >= 0:
			binaryEncodeUint(e.buf, uint64(top.LenHint))
		}
	}
	if done && top.Type.Kind() != vdl.Array && top.LenHint < 0 {
		// emit collection terminator
		// binaryEncodeControl(e.buf, WireCtrlCollectionTerminator)
		panic("null terminator case not yet supported")
	}
	return nil
}

func (e *xEncoder) NextField(name string) error {
	top := e.top()
	if top == nil {
		return errEmptyEncoderStack
	}
	if name == "" {
		if top.Type.Kind() == vdl.Struct {
			binaryEncodeControl(e.buf, WireCtrlEnd)
		}
		return nil
	}
	_, index := top.Type.FieldByName(name)
	if index < 0 {
		return fmt.Errorf("vom: encoder: invalid field %q", name)
	}
	binaryEncodeUint(e.buf, uint64(index))
	top.Index = index
	return nil
}

func (e *xEncoder) SetLenHint(lenHint int) error {
	top := e.top()
	if top == nil {
		return errEmptyEncoderStack
	}
	switch top.Type.Kind() {
	case vdl.List, vdl.Set, vdl.Map:
	default:
		fmt.Errorf("SetLenHint illegal for type %v", top.Type)
	}
	top.LenHint = lenHint
	return nil
}

func (e *xEncoder) EncodeBool(v bool) error {
	binaryEncodeBool(e.buf, v)
	return nil
}
func (e *xEncoder) EncodeUint(v uint64) error {
	binaryEncodeUint(e.buf, v)
	return nil
}
func (e *xEncoder) EncodeInt(v int64) error {
	binaryEncodeInt(e.buf, v)
	return nil
}
func (e *xEncoder) EncodeFloat(v float64) error {
	binaryEncodeFloat(e.buf, v)
	return nil
}
func (e *xEncoder) EncodeBytes(v []byte) error {
	top := e.top()
	if top == nil {
		return errEmptyEncoderStack
	}
	switch top.Type.Kind() {
	case vdl.List:
		binaryEncodeUint(e.buf, uint64(len(v)))
	case vdl.Array:
		binaryEncodeUint(e.buf, 0)
	default:
		return fmt.Errorf("invalid kind: %v", top.Type.Kind())
	}
	e.buf.Write(v)
	return nil
}
func (e *xEncoder) EncodeString(v string) error {
	top := e.top()
	if top == nil {
		return errEmptyEncoderStack
	}
	switch top.Type.Kind() {
	case vdl.String:
		binaryEncodeString(e.buf, v)
	case vdl.Enum:
		index := top.Type.EnumIndex(v)
		if index < 0 {
			return verror.New(errLabelNotInType, nil, v, top.Type)
		}
		binaryEncodeUint(e.buf, uint64(index))
	default:
		return fmt.Errorf("invalid kind: %v", top.Type.Kind())
	}
	return nil
}
func (e *xEncoder) EncodeTypeObject(v *vdl.Type) error {
	tid, err := e.typeEnc.encode(v)
	if err != nil {
		return err
	}
	binaryEncodeUint(e.buf, e.tids.ReferenceTypeID(tid))
	return nil
}

func (e *xEncoder) writeRawBytes(rb *RawBytes) error {
	if e.version != rb.Version {
		return fmt.Errorf("version mismatch: %v and %v", e.version, rb.Version)
	}

	if rb.IsNil() {
		return e.NilValue(rb.Type)
	}

	if err := e.StartValue(rb.Type); err != nil {
		return err
	}
	if containsTypeObject(rb.Type) || containsAny(rb.Type) {
		for _, refType := range rb.RefTypes {
			tid, err := e.typeEnc.encode(refType)
			if err != nil {
				return err
			}
			e.tids.ReferenceTypeID(tid)
		}
	}
	if containsAny(rb.Type) {
		e.anyLens.lens = rb.AnyLengths
	}
	e.buf.Write(rb.Data)
	return e.FinishValue()
}
