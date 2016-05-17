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
		case 0:
			return entry.Type.Key() == AnyType
		case 1:
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

func (e *pipeEncoder) encTop() *pipeStackEntry {
	if len(e.Stack) == 0 {
		return nil
	}
	return &e.Stack[len(e.Stack)-1]
}

func (d *pipeDecoder) decTop() *pipeStackEntry {
	if len(d.Enc.Stack) == 0 {
		return nil
	}
	return &d.Enc.Stack[len(d.Enc.Stack)-1]
}

func (d *pipeDecoder) decParent() *pipeStackEntry {
	if len(d.Enc.Stack) < 2 {
		return nil
	}
	return &d.Enc.Stack[len(d.Enc.Stack)-2]
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
	top := e.encTop()
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
	top := e.encTop()
	if top != nil {
		top.NumStarted++
	}

	e.Stack = append(e.Stack, pipeStackEntry{
		Type:       tt,
		NextOp:     pipeStartDec,
		LenHint:    -1,
		IsOptional: e.NextStartValueIsOptional,
		Index:      -1,
		NumStarted: -1,
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
	top := e.encTop()
	if top == nil {
		return e.closeLocked(errEmptyPipeStack)
	}
	if top.NextOp != pipeFinishEnc {
		return e.closeLocked(fmt.Errorf("vdl: pipe expected state %v, but got %v", pipeFinishEnc, top.NextOp))
	}
	top.NextOp = top.NextOp.Next()
	return e.Err
}

func (e *pipeEncoder) waitLocked() error {
	top := e.encTop()
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

func (d *pipeDecoder) StartValue() error {
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
	top := d.decTop()
	if top == nil {
		return d.Enc.closeLocked(errEmptyPipeStack)
	}
	if top.NextOp != pipeStartDec {
		return d.Enc.closeLocked(fmt.Errorf("vdl: pipe expected state %v, but got %v", pipeStartDec, top.NextOp))
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
	top := d.decTop()
	if top == nil {
		return d.Enc.closeLocked(errEmptyPipeStack)
	}

	if top.NextOp != pipeFinishDec {
		return d.Enc.closeLocked(fmt.Errorf("vdl: pipe expected state %v, but got %v", pipeFinishDec, top.NextOp))
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
	if err := d.StartValue(); err != nil {
		return err
	}
	return d.FinishValue()
}

func (d *pipeDecoder) IgnoreNextStartValue() {
	d.IgnoringNextStartValue = true
}

func (e *pipeEncoder) SetLenHint(lenHint int) error {
	top := e.encTop()
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
	top := d.decTop()
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
	top := d.decTop()
	if top == nil {
		return "", d.Enc.closeLocked(errEmptyPipeStack)
	}
	top.Index++
	name := d.Enc.NextFieldName
	d.Enc.NextFieldName = ""
	return name, d.Enc.Err
}

func (d *pipeDecoder) StackDepth() int {
	return len(d.Enc.Stack)
}

func (d *pipeDecoder) Type() *Type {
	top := d.decTop()
	if top == nil {
		return nil
	}
	if d.Enc.IsBytes && d.Enc.DecHandlingBytes {
		return ByteType
	}
	return top.Type
}

func (d *pipeDecoder) IsAny() bool {
	parent := d.decParent()
	if parent == nil {
		return false
	}
	return parent.nextValueIsAny()
}

func (d *pipeDecoder) IsOptional() bool {
	top := d.decTop()
	if top == nil {
		return false
	}
	return top.IsOptional
}

func (d *pipeDecoder) IsNil() bool {
	top := d.decTop()
	if top == nil {
		return false
	}
	return top.IsNil
}

func (d *pipeDecoder) Index() int {
	top := d.decTop()
	if top == nil {
		return 0
	}
	return top.Index
}

func (d *pipeDecoder) LenHint() int {
	top := d.decTop()
	if top == nil {
		return -1
	}
	return top.LenHint
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
	const errFmt = "vdl: %v conversion to uint%d loses precision: %v"
	switch d.Enc.NumberType {
	case numberUint:
		x := d.Enc.ArgUint
		if d.Enc.IsBytes {
			top := d.decTop()
			if top == nil {
				return 0, d.Enc.closeLocked(errEmptyPipeStack)
			}
			x = uint64(d.Enc.ArgBytes[top.Index])
		}
		if shift := 64 - uint(bitlen); x != (x<<shift)>>shift {
			return 0, d.Enc.closeLocked(fmt.Errorf(errFmt, "uint", bitlen, x))
		}
		return x, d.Enc.Err
	case numberInt:
		x := d.Enc.ArgInt
		ux := uint64(x)
		if shift := 64 - uint(bitlen); x < 0 || ux != (ux<<shift)>>shift {
			return 0, d.Enc.closeLocked(fmt.Errorf(errFmt, "int", bitlen, x))
		}
		return ux, d.Enc.Err
	case numberFloat:
		x := d.Enc.ArgFloat
		ux := uint64(x)
		if shift := 64 - uint(bitlen); x != float64(ux) || ux != (ux<<shift)>>shift {
			return 0, d.Enc.closeLocked(fmt.Errorf(errFmt, "float", bitlen, x))
		}
		return ux, d.Enc.Err
	}
	return 0, d.Enc.closeLocked(fmt.Errorf("vdl: number not set"))
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
	const errFmt = "vdl: %v conversion to int%d loses precision: %v"
	switch d.Enc.NumberType {
	case numberUint:
		x := d.Enc.ArgUint
		if d.Enc.IsBytes {
			top := d.decTop()
			if top == nil {
				return 0, d.Enc.closeLocked(errEmptyPipeStack)
			}
			x = uint64(d.Enc.ArgBytes[top.Index])
		}
		ix := int64(x)
		// The shift uses 65 since the topmost bit is the sign bit.  I.e. 32 bit
		// numbers should be shifted by 33 rather than 32.
		if shift := 65 - uint(bitlen); ix < 0 || x != (x<<shift)>>shift {
			return 0, d.Enc.closeLocked(fmt.Errorf(errFmt, "uint", bitlen, x))
		}
		return ix, d.Enc.Err
	case numberInt:
		x := d.Enc.ArgInt
		if shift := 64 - uint(bitlen); x != (x<<shift)>>shift {
			return 0, d.Enc.closeLocked(fmt.Errorf(errFmt, "int", bitlen, x))
		}
		return x, d.Enc.Err
	case numberFloat:
		x := d.Enc.ArgFloat
		ix := int64(x)
		if shift := 64 - uint(bitlen); x != float64(ix) || ix != (ix<<shift)>>shift {
			return 0, d.Enc.closeLocked(fmt.Errorf(errFmt, "float", bitlen, x))
		}
		return ix, d.Enc.Err
	}
	return 0, d.Enc.closeLocked(fmt.Errorf("vdl: number not set"))
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
	const errFmt = "vdl: %v conversion to float%d loses precision: %v"
	switch d.Enc.NumberType {
	case numberUint:
		x := d.Enc.ArgUint
		if d.Enc.IsBytes {
			top := d.decTop()
			if top == nil {
				return 0, d.Enc.closeLocked(errEmptyPipeStack)
			}
			x = uint64(d.Enc.ArgBytes[top.Index])
		}
		var max uint64
		if bitlen > 32 {
			max = float64MaxInt
		} else {
			max = float32MaxInt
		}
		if x > max {
			return 0, d.Enc.closeLocked(fmt.Errorf(errFmt, "uint", bitlen, x))
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
			return 0, d.Enc.closeLocked(fmt.Errorf(errFmt, "int", bitlen, x))
		}
		return float64(x), d.Enc.Err
	case numberFloat:
		x := d.Enc.ArgFloat
		if bitlen <= 32 && (x < -math.MaxFloat32 || x > math.MaxFloat32) {
			return 0, d.Enc.closeLocked(fmt.Errorf(errFmt, "float", bitlen, x))
		}
		return x, d.Enc.Err
	}
	return 0, fmt.Errorf("vdl: number not set")
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
	if !d.Enc.IsBytes {
		if err := DecodeConvertedBytes(d, fixedLen, b); err != nil {
			return d.Enc.closeLocked(err)
		}
		return nil
	}
	if fixedLen >= 0 && fixedLen != len(d.Enc.ArgBytes) {
		return d.Enc.closeLocked(fmt.Errorf("invalid byte len: got %v, want %v", len(d.Enc.ArgBytes), fixedLen))
	}
	if len(*b) < len(d.Enc.ArgBytes) {
		*b = make([]byte, len(d.Enc.ArgBytes))
	} else {
		*b = (*b)[:len(d.Enc.ArgBytes)]
	}
	copy(*b, d.Enc.ArgBytes)
	return d.Enc.Err
}
