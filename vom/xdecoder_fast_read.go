// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"strconv"

	"v.io/v23/vdl"
)

// TODO(toddw): Use the same technique as xencoder_fast.go to remove boilerplate

// This file contains the ReadValue* methods.  The semantics of these methods is
// the same as if StartValue, Decode*, FinishValue were called in sequence.  The
// implementation is faster than actually calling that sequence, because we can
// avoid pushing and popping the decoder stack and also avoid unnecessary
// compatibility checks.  We also get a minor improvement by avoiding extra
// method calls indirected through the Decoder interface.
//
// Each method has the same pattern:
//
// StartValue:
//   If d.ignoreNextStarValue is set, the type is already on the stack.
//   Otherwise setup the type to process Any and Optional headers.  We pass nil
//   to d.setupType to avoid the compatibility check, since the decode step will
//   naturally let us perform that check.
// Decode:
//   We implement common-case fastpaths; e.g. avoiding unnecessary conversions.
// FinishValue:
//   Mirrors StartValue, only pop the stack if necessary.

func (d *xDecoder) ReadValueBool() (value bool, err error) {
	// StartValue
	var tt *vdl.Type
	if d.ignoreNextStartValue {
		tt = d.stack[len(d.stack)-1].Type
	} else {
		if tt, err = d.dfsNextType(); err != nil {
			return false, err
		}
		if tt, _, _, err = d.setupType(tt, nil); err != nil {
			return false, err
		}
	}
	// Decode
	switch tt.Kind() {
	case vdl.Bool:
		value, err = binaryDecodeBool(d.old.buf)
	default:
		return false, errIncompatibleDecode(tt, "bool")
	}
	// FinishValue
	if d.ignoreNextStartValue {
		if err := d.FinishValue(); err != nil {
			return false, err
		}
	} else {
		d.isParentBytes = false
		d.ignoreNextStartValue = false
		if len(d.stack) == 0 {
			if err := d.old.endMessage(); err != nil {
				return false, err
			}
		}
	}
	return value, err
}

func (d *xDecoder) ReadValueString() (value string, err error) {
	// StartValue
	var tt *vdl.Type
	if d.ignoreNextStartValue {
		tt = d.stack[len(d.stack)-1].Type
	} else {
		if tt, err = d.dfsNextType(); err != nil {
			return "", err
		}
		if tt, _, _, err = d.setupType(tt, nil); err != nil {
			return "", err
		}
	}
	// Decode
	switch tt.Kind() {
	case vdl.String:
		value, err = binaryDecodeString(d.old.buf)
	case vdl.Enum:
		value, err = d.binaryDecodeEnum(tt)
	default:
		return "", errIncompatibleDecode(tt, "string")
	}
	// FinishValue
	if d.ignoreNextStartValue {
		if err := d.FinishValue(); err != nil {
			return "", err
		}
	} else {
		d.isParentBytes = false
		d.ignoreNextStartValue = false
		if len(d.stack) == 0 {
			if err := d.old.endMessage(); err != nil {
				return "", err
			}
		}
	}
	return value, err
}

func (d *xDecoder) ReadValueUint(bitlen int) (value uint64, err error) {
	// StartValue
	var tt *vdl.Type
	if d.ignoreNextStartValue {
		tt = d.stack[len(d.stack)-1].Type
	} else {
		if tt, err = d.dfsNextType(); err != nil {
			return 0, err
		}
		if tt, _, _, err = d.setupType(tt, nil); err != nil {
			return 0, err
		}
	}
	// Decode, avoiding unnecessary number conversions.
	switch kind := tt.Kind(); kind {
	case vdl.Uint16, vdl.Uint32, vdl.Uint64:
		if kind.BitLen() <= bitlen {
			value, err = binaryDecodeUint(d.old.buf)
		} else {
			value, err = d.decodeUint(tt, uint(bitlen))
		}
	case vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64:
		value, err = d.decodeUint(tt, uint(bitlen))
	case vdl.Byte:
		var b byte
		b, err = d.binaryDecodeByte()
		value = uint64(b)
	default:
		return 0, errIncompatibleDecode(tt, "uint"+strconv.Itoa(bitlen))
	}
	// FinishValue
	if d.ignoreNextStartValue {
		if err := d.FinishValue(); err != nil {
			return 0, err
		}
	} else {
		d.isParentBytes = false
		d.ignoreNextStartValue = false
		if len(d.stack) == 0 {
			if err := d.old.endMessage(); err != nil {
				return 0, err
			}
		}
	}
	return value, err
}

func (d *xDecoder) ReadValueInt(bitlen int) (value int64, err error) {
	// StartValue
	var tt *vdl.Type
	if d.ignoreNextStartValue {
		tt = d.stack[len(d.stack)-1].Type
	} else {
		if tt, err = d.dfsNextType(); err != nil {
			return 0, err
		}
		if tt, _, _, err = d.setupType(tt, nil); err != nil {
			return 0, err
		}
	}
	// Decode, avoiding unnecessary number conversions.
	switch kind := tt.Kind(); kind {
	case vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
		if kind.BitLen() <= bitlen {
			value, err = binaryDecodeInt(d.old.buf)
		} else {
			value, err = d.decodeInt(tt, uint(bitlen))
		}
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Float32, vdl.Float64:
		value, err = d.decodeInt(tt, uint(bitlen))
	default:
		return 0, errIncompatibleDecode(tt, "int"+strconv.Itoa(bitlen))
	}
	// FinishValue
	if d.ignoreNextStartValue {
		if err := d.FinishValue(); err != nil {
			return 0, err
		}
	} else {
		d.isParentBytes = false
		d.ignoreNextStartValue = false
		if len(d.stack) == 0 {
			if err := d.old.endMessage(); err != nil {
				return 0, err
			}
		}
	}
	return value, err
}

func (d *xDecoder) ReadValueFloat(bitlen int) (value float64, err error) {
	// StartValue
	var tt *vdl.Type
	if d.ignoreNextStartValue {
		tt = d.stack[len(d.stack)-1].Type
	} else {
		if tt, err = d.dfsNextType(); err != nil {
			return 0, err
		}
		if tt, _, _, err = d.setupType(tt, nil); err != nil {
			return 0, err
		}
	}
	// Decode, avoiding unnecessary number conversions.
	switch kind := tt.Kind(); kind {
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64:
		value, err = d.decodeFloat(tt, uint(bitlen))
	default:
		return 0, errIncompatibleDecode(tt, "float"+strconv.Itoa(bitlen))
	}
	// FinishValue
	if d.ignoreNextStartValue {
		if err := d.FinishValue(); err != nil {
			return 0, err
		}
	} else {
		d.isParentBytes = false
		d.ignoreNextStartValue = false
		if len(d.stack) == 0 {
			if err := d.old.endMessage(); err != nil {
				return 0, err
			}
		}
	}
	return value, err
}

func (d *xDecoder) ReadValueTypeObject() (value *vdl.Type, err error) {
	// StartValue
	var tt *vdl.Type
	if d.ignoreNextStartValue {
		tt = d.stack[len(d.stack)-1].Type
	} else {
		if tt, err = d.dfsNextType(); err != nil {
			return nil, err
		}
		if tt, _, _, err = d.setupType(tt, nil); err != nil {
			return nil, err
		}
	}
	// Decode
	switch tt.Kind() {
	case vdl.TypeObject:
		value, err = d.binaryDecodeType()
	default:
		return nil, errIncompatibleDecode(tt, "typeobject")
	}
	// FinishValue
	if d.ignoreNextStartValue {
		if err := d.FinishValue(); err != nil {
			return nil, err
		}
	} else {
		d.isParentBytes = false
		d.ignoreNextStartValue = false
		if len(d.stack) == 0 {
			if err := d.old.endMessage(); err != nil {
				return nil, err
			}
		}
	}
	return value, err
}

// ReadValueBytes is more complicated than the other ReadValue* methods, since
// []byte lists and [n]byte arrays aren't scalar, and may need more complicated
// conversions
func (d *xDecoder) ReadValueBytes(fixedLen int, x *[]byte) (err error) {
	// StartValue.  Initialize tt and lenHint, and track whether the []byte type
	// is already on the stack via isOnStack.
	isOnStack := d.ignoreNextStartValue
	d.ignoreNextStartValue = false
	var tt *vdl.Type
	var lenHint int
	if isOnStack {
		top := d.top()
		tt, lenHint = top.Type, top.LenHint
	} else {
		if tt, err = d.dfsNextType(); err != nil {
			return err
		}
		var flag decoderFlag
		if tt, lenHint, flag, err = d.setupType(tt, nil); err != nil {
			return err
		}
		// If tt isn't []byte or [n]byte (or a named variant of these), we need to
		// perform conversion byte-by-byte.  This is complicated, and can't be
		// really fast, so we just push an entry onto the stack and handle this via
		// DecodeConvertedBytes below.
		//
		// We also need to perform the compatibility check, to make sure tt is
		// compatible with []byte.  The check is fairly expensive, so skipping it
		// when tt is actually a bytes type makes the the common case faster.
		if !tt.IsBytes() {
			if !vdl.Compatible2(tt, ttByteList) {
				return errIncompatibleDecode(tt, "bytes")
			}
			d.stack = append(d.stack, decoderStackEntry{
				Type:    tt,
				Index:   -1,
				LenHint: lenHint,
				Flag:    flag,
			})
			isOnStack = true
		}
	}
	// Decode.  The common-case fastpath reads directly from the buffer.
	if tt.IsBytes() {
		if err := d.decodeBytes(tt, lenHint, fixedLen, x); err != nil {
			return err
		}
	} else {
		if err := vdl.DecodeConvertedBytes(d, fixedLen, x); err != nil {
			return err
		}
	}
	// FinishValue
	if isOnStack {
		if err := d.FinishValue(); err != nil {
			return err
		}
	} else {
		d.isParentBytes = false
		if len(d.stack) == 0 {
			if err := d.old.endMessage(); err != nil {
				return err
			}
		}
	}
	return nil
}

var ttByteList = vdl.ListType(vdl.ByteType)
