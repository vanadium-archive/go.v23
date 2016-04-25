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

// Decoder defines the interface for a decoder of vdl values.
//
// TODO(toddw): This is a work in progress.  Update the comments.
type Decoder interface {
	StartValue() error
	FinishValue() error
	StackDepth() int
	SkipValue() error
	IgnoreNextStartValue()

	NextEntry() (bool, error)
	NextField() (string, error)

	Type() *Type
	IsAny() bool
	IsOptional() bool
	IsNil() bool
	Index() int
	LenHint() int

	// DecodeBool decodes and returns a bool.
	DecodeBool() (bool, error)
	// DecodeString decodes and returns a string.
	DecodeString() (string, error)
	// DecodeTypeObject decodes and returns a type.
	DecodeTypeObject() (*Type, error)
	// DecodeUint decodes and returns a uint, where the result has bitlen bits.
	// Errors are returned on loss of precision.
	DecodeUint(bitlen int) (uint64, error)
	// DecodeInt decodes and returns an int, where the result has bitlen bits.
	// Errors are returned on loss of precision.
	DecodeInt(bitlen int) (int64, error)
	// DecodeFloat decodes and returns a float, where the result has bitlen bits.
	// Errors are returned on loss of precision.
	DecodeFloat(bitlen int) (float64, error)
	// DecodeBytes decodes bytes into x.  If fixedlen >= 0 the decoded bytes must
	// be exactly that length, otherwise there is no restriction on the number of
	// decoded bytes.  If cap(*x) is not large enough to fit the decoded bytes, a
	// new byte slice is assigned to *x.
	DecodeBytes(fixedlen int, x *[]byte) error
}

// Encoder defines the interface for an encoder of vdl values.
//
// TODO(toddw): This is a work in progress.  Update the comments.
type Encoder interface {
	SetNextStartValueIsOptional()
	// {Start,Finish}Value must be called before / after every concrete value
	// tt must be non-any and non-optional
	StartValue(tt *Type) error
	FinishValue() error
	// NilValue takes the place of StartValue and FinishValue for nil values.
	NilValue(tt *Type) error

	// NextEntry must be called for every entry.
	NextEntry(done bool) error
	// NextField must be called for every field.
	NextField(name string) error

	SetLenHint(lenHint int) error

	EncodeBool(v bool) error        // bool
	EncodeUint(v uint64) error      // byte, uint16, uint32, uint64
	EncodeInt(v int64) error        // int8, int16, int32, int64
	EncodeFloat(v float64) error    // float32, float64
	EncodeBytes(v []byte) error     // []byte
	EncodeString(v string) error    // string, enum
	EncodeTypeObject(v *Type) error // *Type
}

func decoderCompatible(dec Decoder, tt *Type) error {
	if (dec.StackDepth() == 1 || dec.IsAny()) && !Compatible(tt, dec.Type()) {
		return fmt.Errorf("incompatible %v, from %v", tt, dec.Type())
	}
	return nil
}
