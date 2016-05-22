// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"sync"
)

var (
	errEmptyPipeStack        = errors.New("vdl: empty pipe stack")
	errEncCallDuringDecPhase = errors.New("vdl: pipe encoder method called during decode phase")
	errInvalidPipeState      = errors.New("vdl: invalid pipe state")
	errIncompatibleTypes     = errors.New("vdl: incompatible types")
)

func convertPipe(dst, src interface{}) error {
	enc, dec := newPipe()
	go func() {
		enc.Close(Write(enc, src))
	}()
	return dec.Close(Read(dec, dst))
}

func convertPipeReflect(dst, src reflect.Value) error {
	enc, dec := newPipe()
	go func() {
		enc.Close(WriteReflect(enc, src))
	}()
	return dec.Close(ReadReflect(dec, dst))
}

type pipeStackEntry struct {
	Type       *Type
	NextOp     pipeOp
	LenHint    int
	Index      int
	NumStarted int
	IsOptional bool
	IsNil      bool
}

// pipeState represents the current state of the pipe controller
// pipeState is used to control our blocking, which ping-pongs between the encoder and decoder.
// Once the closed state is reached, everything unblocks.
type pipeState int

const (
	pipeStateEncoder pipeState = iota
	pipeStateDecoder
	pipeStateClosed
)

func (x pipeState) String() string {
	switch x {
	case pipeStateEncoder:
		return "Encoder"
	case pipeStateDecoder:
		return "Decoder"
	case pipeStateClosed:
		return "Closed"
	default:
		panic("bad controller state")
	}
}

type pipeOp int

// pipeOp is used to check our invariants for state transitions.

const (
	pipeStartEnc pipeOp = iota
	pipeStartDec
	pipeFinishEnc
	pipeFinishDec
)

func (op pipeOp) String() string {
	switch op {
	case pipeStartEnc:
		return "StartEnc"
	case pipeStartDec:
		return "StartDec"
	case pipeFinishEnc:
		return "FinishEnc"
	case pipeFinishDec:
		return "FinishDec"
	default:
		panic("bad op")
	}
}

func (op pipeOp) Next() pipeOp {
	if op == pipeFinishDec {
		op = pipeStartEnc
	} else {
		op++
	}
	return op
}

// We can only determine whether the next value is AnyType
// by checking the next type of the entry.
func (entry *pipeStackEntry) nextValueIsAny() bool {
	if entry == nil {
		return false
	}
	switch entry.Type.Kind() {
	case List, Array:
		return entry.Type.Elem() == AnyType
	case Set:
		return entry.Type.Key() == AnyType
	case Map:
		switch entry.NumStarted % 2 {
		case 1:
			return entry.Type.Key() == AnyType
		case 0:
			return entry.Type.Elem() == AnyType
		}
	case Struct, Union:
		return entry.Type.Field(entry.Index).Type == AnyType
	}
	return false
}

type numberType int

const (
	numberUint numberType = iota
	numberInt
	numberFloat
)

type pipeEncoder struct {
	Stack                    []pipeStackEntry
	NextEntryDone            bool
	NextFieldName            string
	NextStartValueIsOptional bool       // The StartValue refers to an optional type.
	NumberType               numberType // The number type X in EncodeX.
	IsBytes                  bool       // Is the current list from EncodeBytes().
	DecHandlingBytes         bool       // When decoding from []byte received from EncodeBytes, skip
	// the calls to the Decoder's StartValue/FinishValue
	// because the encoder will not receive matching calls.
	DecStarted bool // Decoding has started
	Err        error

	Mu    sync.Mutex
	Cond  sync.Cond
	State pipeState

	// Arguments from Encode* to be passed to Decode*:
	ArgBool   bool
	ArgUint   uint64
	ArgInt    int64
	ArgFloat  float64
	ArgString string
	ArgBytes  []byte
	ArgType   *Type
}

type pipeDecoder struct {
	Enc                    pipeEncoder
	IgnoringNextStartValue bool
}

func (e *pipeEncoder) top() *pipeStackEntry {
	if len(e.Stack) == 0 {
		return nil
	}
	return &e.Stack[len(e.Stack)-1]
}

func (d *pipeDecoder) top() *pipeStackEntry {
	if len(d.Enc.Stack) == 0 {
		return nil
	}
	return &d.Enc.Stack[len(d.Enc.Stack)-1]
}

func newPipe() (*pipeEncoder, *pipeDecoder) {
	dec := &pipeDecoder{Enc: pipeEncoder{}}
	dec.Enc.Cond.L = &dec.Enc.Mu
	return &dec.Enc, dec
}

func (e *pipeEncoder) closeLocked(err error) error {
	if err != nil && e.Err == nil {
		e.Err = err
	}
	e.State = pipeStateClosed
	e.Cond.Broadcast()
	return e.Err
}

func (e *pipeEncoder) Close(err error) error {
	e.Mu.Lock()
	defer e.Mu.Unlock()
	return e.closeLocked(err)
}

func (d *pipeDecoder) Close(err error) error {
	return d.Enc.Close(err)
}

func (e *pipeEncoder) SetNextStartValueIsOptional() {
	e.NextStartValueIsOptional = true
}

func (e *pipeEncoder) NilValue(tt *Type) error {
	switch tt.Kind() {
	case Any:
	case Optional:
		e.SetNextStartValueIsOptional()
	default:
		return fmt.Errorf("concrete types disallowed for NilValue (type was %v)", tt)
	}
	if err := e.StartValue(tt); err != nil {
		return err
	}
	top := e.top()
	if top == nil {
		return e.closeLocked(errEmptyPipeStack)
	}
	top.IsNil = true
	return e.FinishValue()
}

func (e *pipeEncoder) StartValue(tt *Type) error {
	e.Mu.Lock()
	defer e.Mu.Unlock()
	if e.State != pipeStateEncoder {
		return e.closeLocked(errInvalidPipeState)
	}
	if err := e.waitLocked(); err != nil {
		return err
	}
	top := e.top()
	if top != nil {
		top.NumStarted++
	}
	e.Stack = append(e.Stack, pipeStackEntry{
		Type:       tt,
		NextOp:     pipeStartDec,
		LenHint:    -1,
		IsOptional: e.NextStartValueIsOptional,
		Index:      -1,
	})
	e.NextStartValueIsOptional = false
	return e.Err
}

func (e *pipeEncoder) FinishValue() error {
	e.Mu.Lock()
	defer e.Mu.Unlock()
	if e.State != pipeStateEncoder {
		return e.closeLocked(errInvalidPipeState)
	}
	if err := e.waitLocked(); err != nil {
		return err
	}
	top := e.top()
	if top == nil {
		return e.closeLocked(errEmptyPipeStack)
	}
	if got, want := top.NextOp, pipeFinishEnc; got != want {
		return e.closeLocked(fmt.Errorf("vdl: pipe got state %v, want %v", got, want))
	}
	top.NextOp = top.NextOp.Next()
	return e.Err
}

func (e *pipeEncoder) waitLocked() error {
	top := e.top()
	if e.State == pipeStateClosed {
		return e.Err
	}
	if top != nil {
		e.State = pipeStateDecoder
		e.Cond.Broadcast()
		for e.State == pipeStateDecoder {
			e.Cond.Wait()
		}
		if e.State == pipeStateClosed {
			return e.Err
		}
	}
	return nil
}

func (d *pipeDecoder) StartValue(want *Type) error {
	d.Enc.Mu.Lock()
	defer d.Enc.Mu.Unlock()
	if d.Enc.DecStarted && d.Enc.State != pipeStateDecoder {
		return d.Enc.closeLocked(errInvalidPipeState)
	}
	if d.IgnoringNextStartValue {
		d.IgnoringNextStartValue = false
		return d.Enc.Err
	}
	if d.Enc.DecHandlingBytes {
		return d.Enc.Err
	}
	if err := d.waitLocked(false); err != nil {
		return err
	}
	top := d.top()
	if top == nil {
		return d.Enc.closeLocked(errEmptyPipeStack)
	}
	// Check compatibility between the actual type and the want type.  Since
	// compatibility applies to the entire static type, we only need to perform
	// this check for top-level decoded values, and subsequently for decoded any
	// values.
	if len(d.Enc.Stack) == 1 || d.IsAny() {
		if !Compatible2(d.Type(), want) {
			return d.Enc.closeLocked(fmt.Errorf("vdl: pipe incompatible decode from %v into %v", d.Type(), want))
		}
	}
	if got, want := top.NextOp, pipeStartDec; got != want {
		return d.Enc.closeLocked(fmt.Errorf("vdl: pipe got state %v, want %v", got, want))
	}
	top.NextOp = top.NextOp.Next()
	return d.Enc.Err
}

func (d *pipeDecoder) FinishValue() error {
	d.Enc.Mu.Lock()
	defer d.Enc.Mu.Unlock()
	switch {
	case d.Enc.State == pipeStateEncoder:
		return d.Enc.closeLocked(errInvalidPipeState)
	case d.Enc.State == pipeStateClosed:
		return d.Enc.Err
	}
	if d.Enc.DecHandlingBytes {
		return d.Enc.Err
	}
	if err := d.waitLocked(true); err != nil {
		return err
	}
	top := d.top()
	if top == nil {
		return d.Enc.closeLocked(errEmptyPipeStack)
	}
	if got, want := top.NextOp, pipeFinishDec; got != want {
		return d.Enc.closeLocked(fmt.Errorf("vdl: pipe got state %v, want %v", got, want))
	}
	d.Enc.Stack = d.Enc.Stack[:len(d.Enc.Stack)-1]
	return d.Enc.Err
}

func (d *pipeDecoder) waitLocked(isFinish bool) error {
	if d.Enc.State == pipeStateClosed {
		return d.Enc.Err
	}
	if isFinish || d.Enc.DecStarted {
		d.Enc.State = pipeStateEncoder
		d.Enc.Cond.Broadcast()
	}
	d.Enc.DecStarted = true
	for d.Enc.State == pipeStateEncoder {
		d.Enc.Cond.Wait()
	}
	if d.Enc.State == pipeStateClosed {
		return d.Enc.Err
	}
	return nil
}

func (d *pipeDecoder) SkipValue() error {
	if err := d.StartValue(AnyType); err != nil {
		return err
	}
	return d.FinishValue()
}

func (d *pipeDecoder) IgnoreNextStartValue() {
	d.IgnoringNextStartValue = true
}

func (e *pipeEncoder) SetLenHint(lenHint int) error {
	top := e.top()
	if top == nil {
		return e.closeLocked(errEmptyPipeStack)
	}
	top.LenHint = lenHint
	return e.Err
}

func (e *pipeEncoder) NextEntry(done bool) error {
	if e.State == pipeStateDecoder {
		return e.closeLocked(errEncCallDuringDecPhase)
	}
	e.NextEntryDone = done
	e.IsBytes = false
	return e.Err
}
func (e *pipeEncoder) NextField(name string) error {
	if e.State == pipeStateDecoder {
		return e.closeLocked(errEncCallDuringDecPhase)
	}
	e.NextFieldName = name
	return e.Err
}

func (d *pipeDecoder) NextEntry() (bool, error) {
	top := d.top()
	if top == nil {
		return false, d.Enc.closeLocked(errEmptyPipeStack)
	}
	top.Index++
	var done bool
	if d.Enc.IsBytes {
		done = top.Index >= len(d.Enc.ArgBytes)
		// End handling bytes mode once done.
		d.Enc.DecHandlingBytes = !done
	} else {
		done = d.Enc.NextEntryDone
	}
	d.Enc.NextEntryDone = false
	return done, d.Enc.Err
}
func (d *pipeDecoder) NextField() (string, error) {
	top := d.top()
	if top == nil {
		return "", d.Enc.closeLocked(errEmptyPipeStack)
	}
	top.Index++
	name := d.Enc.NextFieldName
	d.Enc.NextFieldName = ""
	return name, d.Enc.Err
}

func (d *pipeDecoder) Type() *Type {
	top := d.top()
	if top == nil {
		return nil
	}
	if d.Enc.IsBytes && d.Enc.DecHandlingBytes {
		return ByteType
	}
	return top.Type
}

func (d *pipeDecoder) IsAny() bool {
	if stackTop2 := len(d.Enc.Stack) - 2; stackTop2 >= 0 {
		return d.Enc.Stack[stackTop2].nextValueIsAny()
	}
	return false
}

func (d *pipeDecoder) IsOptional() bool {
	if top := d.top(); top != nil {
		return top.IsOptional
	}
	return false
}

func (d *pipeDecoder) IsNil() bool {
	if top := d.top(); top != nil {
		return top.IsNil
	}
	return false
}

func (d *pipeDecoder) Index() int {
	if top := d.top(); top != nil {
		return top.Index
	}
	return -1
}

func (d *pipeDecoder) LenHint() int {
	if top := d.top(); top != nil {
		return top.LenHint
	}
	return -1
}

func (e *pipeEncoder) EncodeBool(v bool) error {
	if e.State == pipeStateDecoder {
		return e.closeLocked(errEncCallDuringDecPhase)
	}
	e.ArgBool = v
	return e.Err
}

func (d *pipeDecoder) DecodeBool() (bool, error) {
	return d.Enc.ArgBool, d.Enc.Err
}

func (e *pipeEncoder) EncodeString(v string) error {
	if e.State == pipeStateDecoder {
		return e.closeLocked(errEncCallDuringDecPhase)
	}
	e.ArgString = v
	return e.Err
}

func (d *pipeDecoder) DecodeString() (string, error) {
	return d.Enc.ArgString, d.Enc.Err
}

func (e *pipeEncoder) EncodeTypeObject(v *Type) error {
	if e.State == pipeStateDecoder {
		return e.closeLocked(errEncCallDuringDecPhase)
	}
	e.ArgType = v
	return e.Err
}

func (d *pipeDecoder) DecodeTypeObject() (*Type, error) {
	return d.Enc.ArgType, d.Enc.Err
}

func (e *pipeEncoder) EncodeUint(v uint64) error {
	if e.State == pipeStateDecoder {
		return e.closeLocked(errEncCallDuringDecPhase)
	}
	e.ArgUint = v
	e.NumberType = numberUint
	return e.Err
}

func (d *pipeDecoder) DecodeUint(bitlen int) (uint64, error) {
	const errFmt = "vdl: conversion from %v into uint%d loses precision: %v"
	top, tt := d.top(), d.Type()
	if top == nil {
		return 0, d.Enc.closeLocked(errEmptyPipeStack)
	}
	switch d.Enc.NumberType {
	case numberUint:
		x := d.Enc.ArgUint
		if d.Enc.IsBytes {
			x = uint64(d.Enc.ArgBytes[top.Index])
		}
		if shift := 64 - uint(bitlen); x != (x<<shift)>>shift {
			return 0, d.Enc.closeLocked(fmt.Errorf(errFmt, tt, bitlen, x))
		}
		return x, d.Enc.Err
	case numberInt:
		x := d.Enc.ArgInt
		ux := uint64(x)
		if shift := 64 - uint(bitlen); x < 0 || ux != (ux<<shift)>>shift {
			return 0, d.Enc.closeLocked(fmt.Errorf(errFmt, tt, bitlen, x))
		}
		return ux, d.Enc.Err
	case numberFloat:
		x := d.Enc.ArgFloat
		ux := uint64(x)
		if shift := 64 - uint(bitlen); x != float64(ux) || ux != (ux<<shift)>>shift {
			return 0, d.Enc.closeLocked(fmt.Errorf(errFmt, tt, bitlen, x))
		}
		return ux, d.Enc.Err
	}
	return 0, d.Enc.closeLocked(fmt.Errorf("vdl: incompatible decode from %v into uint%d", tt, bitlen))
}

func (e *pipeEncoder) EncodeInt(v int64) error {
	if e.State == pipeStateDecoder {
		return e.closeLocked(errEncCallDuringDecPhase)
	}
	e.ArgInt = v
	e.NumberType = numberInt
	return e.Err
}

func (d *pipeDecoder) DecodeInt(bitlen int) (int64, error) {
	const errFmt = "vdl: conversion from %v into int%d loses precision: %v"
	top, tt := d.top(), d.Type()
	if top == nil {
		return 0, d.Enc.closeLocked(errEmptyPipeStack)
	}
	switch d.Enc.NumberType {
	case numberUint:
		x := d.Enc.ArgUint
		if d.Enc.IsBytes {
			x = uint64(d.Enc.ArgBytes[top.Index])
		}
		ix := int64(x)
		// The shift uses 65 since the topmost bit is the sign bit.  I.e. 32 bit
		// numbers should be shifted by 33 rather than 32.
		if shift := 65 - uint(bitlen); ix < 0 || x != (x<<shift)>>shift {
			return 0, d.Enc.closeLocked(fmt.Errorf(errFmt, tt, bitlen, x))
		}
		return ix, d.Enc.Err
	case numberInt:
		x := d.Enc.ArgInt
		if shift := 64 - uint(bitlen); x != (x<<shift)>>shift {
			return 0, d.Enc.closeLocked(fmt.Errorf(errFmt, tt, bitlen, x))
		}
		return x, d.Enc.Err
	case numberFloat:
		x := d.Enc.ArgFloat
		ix := int64(x)
		if shift := 64 - uint(bitlen); x != float64(ix) || ix != (ix<<shift)>>shift {
			return 0, d.Enc.closeLocked(fmt.Errorf(errFmt, tt, bitlen, x))
		}
		return ix, d.Enc.Err
	}
	return 0, d.Enc.closeLocked(fmt.Errorf("vdl: incompatible decode from %v into int%d", tt, bitlen))
}

func (e *pipeEncoder) EncodeFloat(v float64) error {
	if e.State == pipeStateDecoder {
		return e.closeLocked(errEncCallDuringDecPhase)
	}
	e.ArgFloat = v
	e.NumberType = numberFloat
	return e.Err
}

func (d *pipeDecoder) DecodeFloat(bitlen int) (float64, error) {
	const errFmt = "vdl: conversion from %v into float%d loses precision: %v"
	top, tt := d.top(), d.Type()
	if top == nil {
		return 0, d.Enc.closeLocked(errEmptyPipeStack)
	}
	switch d.Enc.NumberType {
	case numberUint:
		x := d.Enc.ArgUint
		if d.Enc.IsBytes {
			x = uint64(d.Enc.ArgBytes[top.Index])
		}
		var max uint64
		if bitlen > 32 {
			max = float64MaxInt
		} else {
			max = float32MaxInt
		}
		if x > max {
			return 0, d.Enc.closeLocked(fmt.Errorf(errFmt, tt, bitlen, x))
		}
		return float64(x), d.Enc.Err
	case numberInt:
		x := d.Enc.ArgInt
		var min, max int64
		if bitlen > 32 {
			min, max = float64MinInt, float64MaxInt
		} else {
			min, max = float32MinInt, float32MaxInt
		}
		if x < min || x > max {
			return 0, d.Enc.closeLocked(fmt.Errorf(errFmt, tt, bitlen, x))
		}
		return float64(x), d.Enc.Err
	case numberFloat:
		x := d.Enc.ArgFloat
		if bitlen <= 32 && (x < -math.MaxFloat32 || x > math.MaxFloat32) {
			return 0, d.Enc.closeLocked(fmt.Errorf(errFmt, tt, bitlen, x))
		}
		return x, d.Enc.Err
	}
	return 0, d.Enc.closeLocked(fmt.Errorf("vdl: incompatible decode from %v into float%d", tt, bitlen))
}

func (e *pipeEncoder) EncodeBytes(v []byte) error {
	if e.State == pipeStateDecoder {
		return e.closeLocked(errEncCallDuringDecPhase)
	}
	e.IsBytes = true
	e.NumberType = numberUint
	e.ArgBytes = v
	return e.Err
}

func (d *pipeDecoder) DecodeBytes(fixedLen int, b *[]byte) error {
	top := d.top()
	if top == nil {
		return d.Enc.closeLocked(errEmptyPipeStack)
	}
	if !d.Enc.IsBytes {
		if err := DecodeConvertedBytes(d, fixedLen, b); err != nil {
			return d.Enc.closeLocked(err)
		}
		return nil
	}
	len := len(d.Enc.ArgBytes)
	if fixedLen >= 0 && fixedLen != len {
		return d.Enc.closeLocked(fmt.Errorf("vdl: %v got %d bytes, want fixed len %d", d.Type(), len, fixedLen))
	}
	if cap(*b) >= len {
		*b = (*b)[:len]
	} else {
		*b = make([]byte, len)
	}
	copy(*b, d.Enc.ArgBytes)
	return d.Enc.Err
}

// The ReadValue* and NextEntryValue* methods just call methods in sequence.

func (d *pipeDecoder) ReadValueBool() (bool, error) {
	if err := d.StartValue(BoolType); err != nil {
		return false, err
	}
	value, err := d.DecodeBool()
	if err != nil {
		return false, err
	}
	return value, d.FinishValue()
}

func (d *pipeDecoder) ReadValueString() (string, error) {
	if err := d.StartValue(StringType); err != nil {
		return "", err
	}
	value, err := d.DecodeString()
	if err != nil {
		return "", err
	}
	return value, d.FinishValue()
}

func (d *pipeDecoder) ReadValueUint(bitlen int) (uint64, error) {
	if err := d.StartValue(Uint64Type); err != nil {
		return 0, err
	}
	value, err := d.DecodeUint(bitlen)
	if err != nil {
		return 0, err
	}
	return value, d.FinishValue()
}

func (d *pipeDecoder) ReadValueInt(bitlen int) (int64, error) {
	if err := d.StartValue(Int64Type); err != nil {
		return 0, err
	}
	value, err := d.DecodeInt(bitlen)
	if err != nil {
		return 0, err
	}
	return value, d.FinishValue()
}

func (d *pipeDecoder) ReadValueFloat(bitlen int) (float64, error) {
	if err := d.StartValue(Float64Type); err != nil {
		return 0, err
	}
	value, err := d.DecodeFloat(bitlen)
	if err != nil {
		return 0, err
	}
	return value, d.FinishValue()
}

func (d *pipeDecoder) ReadValueTypeObject() (*Type, error) {
	if err := d.StartValue(TypeObjectType); err != nil {
		return nil, err
	}
	value, err := d.DecodeTypeObject()
	if err != nil {
		return nil, err
	}
	return value, d.FinishValue()
}

func (d *pipeDecoder) ReadValueBytes(fixedLen int, x *[]byte) error {
	if err := d.StartValue(ttByteList); err != nil {
		return err
	}
	if err := d.DecodeBytes(fixedLen, x); err != nil {
		return err
	}
	return d.FinishValue()
}

func (d *pipeDecoder) NextEntryValueBool() (done bool, _ bool, _ error) {
	if done, err := d.NextEntry(); done || err != nil {
		return done, false, err
	}
	value, err := d.ReadValueBool()
	return false, value, err
}

func (d *pipeDecoder) NextEntryValueString() (done bool, _ string, _ error) {
	if done, err := d.NextEntry(); done || err != nil {
		return done, "", err
	}
	value, err := d.ReadValueString()
	return false, value, err
}

func (d *pipeDecoder) NextEntryValueUint(bitlen int) (done bool, _ uint64, _ error) {
	if done, err := d.NextEntry(); done || err != nil {
		return done, 0, err
	}
	value, err := d.ReadValueUint(bitlen)
	return false, value, err
}

func (d *pipeDecoder) NextEntryValueInt(bitlen int) (done bool, _ int64, _ error) {
	if done, err := d.NextEntry(); done || err != nil {
		return done, 0, err
	}
	value, err := d.ReadValueInt(bitlen)
	return false, value, err
}

func (d *pipeDecoder) NextEntryValueFloat(bitlen int) (done bool, _ float64, _ error) {
	if done, err := d.NextEntry(); done || err != nil {
		return done, 0, err
	}
	value, err := d.ReadValueFloat(bitlen)
	return false, value, err
}

func (d *pipeDecoder) NextEntryValueTypeObject() (done bool, _ *Type, _ error) {
	if done, err := d.NextEntry(); done || err != nil {
		return done, nil, err
	}
	value, err := d.ReadValueTypeObject()
	return false, value, err
}
