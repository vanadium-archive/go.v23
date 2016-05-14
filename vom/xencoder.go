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
	typeEnc := newTypeEncoderInternal(version, newXEncoderForTypes(version, w))
	return NewVersionedXEncoderWithTypeEncoder(version, w, typeEnc)
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

// TODO(toddw): Flip useOldEncoderForTypes=false to enable XEncoder for types.
const useOldEncoderForTypes = true

func newXEncoderForTypes(version Version, w io.Writer) *xEncoder {
	if !isAllowedVersion(version) {
		panic(fmt.Sprintf("unsupported VOM version: %x", version))
	}
	buf := newEncbuf()
	e := &xEncoder{
		writer:          w,
		buf:             buf,
		sentVersionByte: true,
		version:         version,
		mode:            encoderForTypes,
	}
	if useOldEncoderForTypes {
		// TEMPORARY HACK: Create the old encoder if we're using it for types.
		e.old = &encoder{
			writer:          w,
			buf:             buf,
			typeStack:       make([]typeStackEntry, 0, 10),
			sentVersionByte: true,
			version:         version,
		}
	}
	return e
}

func newXEncoderForRawBytes(w io.Writer) *xEncoder {
	// RawBytes doesn't need the types to be encoded, since it holds the in-memory
	// representation.  We still need a type encoder to collect the unique types,
	// but we give it a dummy encoder that doesn't have any state set up.
	typeEnc := newTypeEncoderInternal(DefaultVersion, &xEncoder{
		sentVersionByte: true,
		version:         DefaultVersion,
		mode:            encoderForRawBytes,
	})
	return &xEncoder{
		writer:          w,
		buf:             newEncbuf(),
		typeEnc:         typeEnc,
		sentVersionByte: true,
		version:         DefaultVersion,
		mode:            encoderForRawBytes,
	}
}

func (e *XEncoder) Encoder() vdl.Encoder {
	return &e.enc
}

func (e *XEncoder) Encode(v interface{}) error {
	return vdl.Write(&e.enc, v)
}

type encoderMode int

const (
	encoderRegular     encoderMode = iota
	encoderForTypes                // xEncoder is embedded in TypeEncoder
	encoderForRawBytes             // xEncoder is used to encode RawBytes
)

type xEncoder struct {
	stack []encoderStackEntry
	// We use buf to buffer up the encoded value. The buffering is necessary so
	// that we can compute the total message length.
	buf *encbuf
	// Buffer for the header of messages with any or typeobject.
	bufHeader *encbuf
	// Underlying writer.
	writer io.Writer
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

	// msgType captures the type of the top-level value.  Unlike stack[0].Type, it
	// also captures optionality for non-nil types.
	msgType *vdl.Type

	mode encoderMode

	// As a temporary hack, before we've switched to the XEncoder for everything,
	// we still need to support the old encoder.
	//
	// TODO(toddw): Remove this when the switch to XEncoder is complete.
	old *encoder
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
		// NumStarted is already incremented by the time we check it.
		if entry.NumStarted%2 == 1 {
			return entry.Type.Key() == vdl.AnyType
		} else {
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

func (e *xEncoder) encodeWireType(tid TypeId, wt wireType, typeIncomplete bool) error {
	if useOldEncoderForTypes {
		return e.old.encodeWireType(tid, wt, typeIncomplete)
	}
	// RawBytes doesn't need type messages to be encoded, since it holds the types
	// in-memory.
	if e.mode == encoderForRawBytes {
		return nil
	}
	// Set up the state that would normally be set in startMessage, and use
	// VDLWrite to encode wt as a regular value.
	e.mid = int64(-tid)
	e.typeIncomplete = typeIncomplete
	e.hasAny = false
	e.hasTypeObject = false
	e.hasLen = true
	e.tids = nil
	e.anyLens = nil
	return wt.VDLWrite(e)
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
	if nextValueIsAny && tt.Kind() == vdl.Optional {
		tid, err := e.typeEnc.encode(tt)
		if err != nil {
			return err
		}
		binaryEncodeUint(e.buf, e.tids.ReferenceTypeID(tid))
		anyRef = e.anyLens.StartAny(e.buf.Len())
		binaryEncodeUint(e.buf, uint64(anyRef.index))
	}
	binaryEncodeControl(e.buf, WireCtrlNil)
	if nextValueIsAny && tt.Kind() == vdl.Optional {
		e.anyLens.FinishAny(anyRef, e.buf.Len())
	}

	if len(e.stack) == 0 {
		if err := e.finishMessage(); err != nil {
			return err
		}
	}
	e.nextStartValueOptional = false
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
	e.buf.Reset()
	e.buf.Grow(paddingLen)
	e.msgType = tt
	if e.mode == encoderForTypes {
		// We've already set up the state in encodeWireType.
		return nil
	}
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
	if e.mode == encoderForRawBytes {
		// Only encode the value portion for RawBytes.
		msg := e.buf.Bytes()
		_, err := e.writer.Write(msg[paddingLen:])
		return err
	}
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

// writeRawBytes writes rb to e.  This only works if e at the top-level; if it
// has already encoded some values, rb.Data needs to be re-written with new
// indices for type ids and any lengths.
//
// REQUIRES: e.version == rb.Version && len(e.stack) == 0
//
// TODO(toddw): Code a variant of this that performs the re-writing.
func (e *xEncoder) writeRawBytes(rb *RawBytes) error {
	if rb.IsNil() {
		return e.NilValue(rb.Type)
	}
	tt := rb.Type
	if tt.Kind() == vdl.Optional {
		e.SetNextStartValueIsOptional()
		tt = tt.Elem()
	}
	if err := e.StartValue(tt); err != nil {
		return err
	}
	if containsAny(tt) || containsTypeObject(tt) {
		for _, refType := range rb.RefTypes {
			tid, err := e.typeEnc.encode(refType)
			if err != nil {
				return err
			}
			e.tids.ReferenceTypeID(tid)
		}
	}
	if containsAny(tt) {
		e.anyLens.lens = rb.AnyLengths
	}
	e.buf.Write(rb.Data)
	return e.FinishValue()
}
