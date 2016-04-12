// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import "fmt"

// Reader is the interface that wraps the VDLRead method.
//
// VDLRead fills in the the underlying value (that implements this method) from
// the Decoder.  This method is auto-generated for all types defined in vdl.  It
// may be implemented for regular Go types not defined in vdl, to customize the
// decoding.
type Reader interface {
	VDLRead(dec Decoder) error
}

// Writer is the interface that wraps the VDLWrite method.
//
// VDLWrite writes out the underlying value (that implements this method) to the
// Encoder.  This method is auto-generated for all types defined in vdl.  It may
// be implemented for regular Go types not defined in vdl, to customize the
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
	DecodeUint(bitlen uint) (uint64, error)
	// DecodeInt decodes and returns an int, where the result has bitlen bits.
	// Errors are returned on loss of precision.
	DecodeInt(bitlen uint) (int64, error)
	// DecodeFloat decodes and returns a float, where the result has bitlen bits.
	// Errors are returned on loss of precision.
	DecodeFloat(bitlen uint) (float64, error)
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
	// {Start,Finish}Value must be called for every new value and nested any.
	// This is needed to bracket each value for the encoder, and also to tell us
	// the elem type of any.  The encoder still needs to keep a stack of types, so
	// that it knows where we are in the DFS, e.g. for enum and field indices.
	//
	// TODO(toddw): can we make the encoder not need a type stack?
	StartValue(tt *Type) error
	FinishValue() error

	StartComposite() error
	FinishComposite() error

	// HintLen is an optional hint to the encoder.
	//
	// TODO(toddw): change VOM so that collections can either start with a length,
	// or end with a terminator.  Use a flag for "has terminator".  Or do we
	// really gain anything by having the length?
	EncodeLenHint(len int) error

	// We need two different EncodeZero because we need to distinguish between a
	// composite being zero, or its items being zero.
	//
	// EncodeTopLevelZero means that the value that we just started is zero, as
	// opposed to EncodeZero, which means that the sub-value is zero.  Calling
	// EncodeTopLevelZero is optional; it's only meant to improve performance for
	// large composite types that are zero.
	//
	// TODO(toddw): Should we split back into EncodeNil(), and then deal with
	// zero-valued struct fields specially?
	EncodeTopLevelZero() error
	EncodeZero() error

	EncodeBool(v bool) error
	EncodeUint(v uint64) error
	EncodeInt(v int64) error
	EncodeFloat(v float64) error
	EncodeBytes(v []byte) error
	EncodeString(v string) error
	EncodeTypeObject(v *Type) error

	// TODO(toddw): Add support for RawBytes, perhaps like this:
	//SupportsVomBytes() bool
	//EncodeVomBytes(src []byte) error
}

func decoderCompatible(dec Decoder, tt *Type) error {
	if (dec.StackDepth() == 1 || dec.IsAny()) && !Compatible(tt, dec.Type()) {
		return fmt.Errorf("incompatible %v %v, from %v", tt.Kind(), tt, dec.Type())
	}
	return nil
}
