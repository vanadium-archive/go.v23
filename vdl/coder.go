// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import "fmt"

// Reader is the interface that wraps the VDLRead method.
//
// VDLRead fills in the the receiver that implements this method from the
// Decoder.  This method is auto-generated for all types defined in vdl.  It may
// be implemented for regular Go types not defined in vdl, to customize the
// decoding.
type Reader interface {
	VDLRead(dec Decoder) error
}

// Writer is the interface that wraps the VDLWrite method.
//
// VDLWrite writes out the receiver that implements this method to the Encoder.
// This method is auto-generated for all types defined in vdl.  It may be
// implemented for regular Go types not defined in vdl, to customize the
// encoding.
type Writer interface {
	VDLWrite(enc Encoder) error
}

// ReadWriter is the interface that groups the VDLRead and VDLWrite methods.
type ReadWriter interface {
	Reader
	Writer
}

// Decoder defines the interface for a decoder of vdl values.  The Decoder is
// passed as the argument to VDLRead.  An example of an implementation of this
// interface is vom.Decoder.
//
// The Decoder provides an API to read vdl values of all types in depth-first
// order.  The ordering is based on the type of the value being read.
// E.g. given the following value:
//    type MyStruct struct {
//      A []string
//      B map[int64]bool
//      C any
//    }
//    value := MyStruct{
//      A: {"abc", "def"},
//      B: {123: true, 456: false},
//      C: float32(1.5),
//    }
// The values will be read in the following order:
//    "abc"
//    "def"
//    (123, true)
//    (456, false)
//    1.5
type Decoder interface {
	// StartValue must be called before decoding each value, for both scalar and
	// composite values.  The want type is the type of value being decoded into,
	// used to check compatibility with the value in the decoder; use AnyType if
	// you don't know, or want to decode any type of value.  Each call pushes the
	// type of the next value on to the stack.
	StartValue(want *Type) error
	// FinishValue must be called after decoding each value, for both scalar and
	// composite values.  Each call pops the type of the top value off of the
	// stack.
	FinishValue() error
	// SkipValue skips the next value; logically it behaves as if a full sequence
	// of StartValue / ...Decode*... / FinishValue were called.  It enables
	// optimizations when the caller doesn't care about the next value.
	SkipValue() error
	// IgnoreNextStartValue instructs the Decoder to ignore the next call to
	// StartValue.  It is used to simplify implementations of VDLRead; e.g. a
	// caller might call StartValue to check for nil values, and subsequently call
	// StartValue again to read non-nil values.  IgnoreNextStartValue is used to
	// ignore the second StartValue call.
	IgnoreNextStartValue()

	// NextEntry instructs the Decoder to move to the next element of an Array or
	// List, the next key of a Set, or the next (key,elem) pair of a Map.  Returns
	// done=true when there are no remaining entries.
	NextEntry() (done bool, _ error)
	// NextField instructs the Decoder to move to the next field of a Struct or
	// Union.  Returns the name of the next field, or the empty string when there
	// are no remaining fields.
	NextField() (name string, _ error)

	// Type returns the type of the top value on the stack.  Returns nil when the
	// stack is empty.  The returned type is only Any or Optional iff the value is
	// nil; non-nil values are "auto-dereferenced" to their underlying elem value.
	Type() *Type
	// IsAny returns true iff the type of the top value on the stack was Any,
	// despite the "auto-dereference" behavior of non-nil values.
	IsAny() bool
	// IsOptional returns true iff the type of the top value on the stack was
	// Optional, despite the "auto-dereference" behavior of non-nil values.
	IsOptional() bool
	// IsNil returns true iff the top value on the stack is nil.  It is equivalent
	// to Type() == AnyType || Type().Kind() == Optional.
	IsNil() bool
	// Index returns the index of the current entry or field of the top value on
	// the stack.  Returns -1 if the top value is a scalar, or if NextEntry /
	// NextField has not been called.
	Index() int
	// LenHint returns the length of the top value on the stack, if it is
	// available.  Returns -1 if the top value is a scalar, or if the length is
	// not available.
	LenHint() int

	// DecodeBool returns the top value on the stack as a bool.
	DecodeBool() (bool, error)
	// DecodeString returns the top value on the stack as a string.
	DecodeString() (string, error)
	// DecodeUint returns the top value on the stack as a uint, where the result
	// has bitlen bits.  Errors are returned on loss of precision.
	DecodeUint(bitlen int) (uint64, error)
	// DecodeInt returns the top value on the stack as an int, where the result
	// has bitlen bits.  Errors are returned on loss of precision.
	DecodeInt(bitlen int) (int64, error)
	// DecodeFloat returns the top value on the stack as a float, where the result
	// has bitlen bits.  Errors are returned on loss of precision.
	DecodeFloat(bitlen int) (float64, error)
	// DecodeBytes decodes the top value on the stack as bytes, into x.  If
	// fixedlen >= 0 the decoded bytes must be exactly that length, otherwise
	// there is no restriction on the number of decoded bytes.  If cap(*x) is not
	// large enough to fit the decoded bytes, a new byte slice is assigned to *x.
	DecodeBytes(fixedlen int, x *[]byte) error
	// DecodeTypeObject returns the top value on the stack as a type.
	DecodeTypeObject() (*Type, error)
}

// Encoder defines the interface for an encoder of vdl values.  The Encoder is
// passed as the argument to VDLWrite.  An example of an implementation of this
// interface is vom.Encoder.
//
// The Encoder provides an API to write vdl values of all types in depth-first
// order.  The ordering is based on the type of the value being written; see
// Decoder for examples.
type Encoder interface {
	// StartValue must be called before encoding each non-nil value, for both
	// scalar and composite values.  The tt type cannot be Any or Optional; use
	// NilValue to encode nil values.
	StartValue(tt *Type) error
	// FinishValue must be called after encoding each non-nil value, for both
	// scalar and composite values.
	FinishValue() error
	// NilValue encodes a nil value.  The tt type must be Any or Optional.
	NilValue(tt *Type) error
	// SetNextStartValueIsOptional instructs the encoder that the next call to
	// StartValue represents a value with an Optional type.
	SetNextStartValueIsOptional()

	// NextEntry instructs the Encoder to move to the next element of an Array or
	// List, the next key of a Set, or the next (key,elem) pair of a Map.  Set
	// done=true when there are no remaining entries.
	NextEntry(done bool) error
	// NextField instructs the Encoder to move to the next field of a Struct or
	// Union.  Set name to the name of the next field, or set name="" when there
	// are no remaining fields.
	NextField(name string) error

	// SetLenHint sets the length of the List, Set or Map value.  It may only be
	// called immediately after StartValue, before NextEntry has been called.  Do
	// not call this method if the length is not known.
	SetLenHint(lenHint int) error

	// EncodeBool encodes a bool value.
	EncodeBool(v bool) error
	// EncodeString encodes a string value.
	EncodeString(v string) error
	// EncodeUint encodes a uint value.
	EncodeUint(v uint64) error
	// EncodeInt encodes an int value.
	EncodeInt(v int64) error
	// EncodeFloat encodes a float value.
	EncodeFloat(v float64) error
	// EncodeBytes encodes a bytes value; either an array or list of bytes.
	EncodeBytes(v []byte) error
	// EncodeTypeObject encodes a type.
	EncodeTypeObject(v *Type) error
}

// DecodeConvertedBytes is a helper function for implementations of
// Decoder.DecodeBytes, to deal with cases where the decoder value is
// convertible to []byte.  E.g. if the decoder value is []float64, we need to
// decode each element as a uint8, performing conversion checks.
//
// Since this is meant to be used in the implementation of DecodeBytes, there is
// no outer call to StartValue/FinishValue.
func DecodeConvertedBytes(dec Decoder, fixedlen int, buf *[]byte) error {
	if len := dec.LenHint(); len >= 0 && cap(*buf) < len {
		*buf = make([]byte, 0, len)
	} else {
		*buf = (*buf)[:0]
	}
	index := 0
	for {
		switch done, err := dec.NextEntry(); {
		case err != nil:
			return err
		case fixedlen >= 0 && done != (index >= fixedlen):
			return fmt.Errorf("array len mismatch, done:%v index:%d len:%d", done, index, fixedlen)
		case done:
			return nil
		}
		if err := dec.StartValue(ByteType); err != nil {
			return err
		}
		elem, err := dec.DecodeUint(8)
		if err != nil {
			return err
		}
		if err := dec.FinishValue(); err != nil {
			return err
		}
		*buf = append(*buf, byte(elem))
		index++
	}
}
