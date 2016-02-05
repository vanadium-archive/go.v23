// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"io"
	"math"

	"v.io/v23/vdl"
	"v.io/v23/verror"
)

// TODO(toddw) Reduce the number of functions and optimize.
const pkgPath = "v.io/v23/vom3"

var (
	errInvalid                = verror.Register(pkgPath+".errInvalid", verror.NoRetry, "{1:}{2:} vom: invalid encoding{:_}")
	errMsgLen                 = verror.Register(pkgPath+".errMsgLen", verror.NoRetry, "{1:}{2:} vom: message larger than {3} bytes{:_}")
	errUintOverflow           = verror.Register(pkgPath+".errUintOverflow", verror.NoRetry, "{1:}{2:} vom: scalar larger than 8 bytes{:_}")
	errBadControlCode         = verror.Register(pkgPath+".errBadControlCode", verror.NoRetry, "{1:}{2:} invalid control code{:_}")
	errBadVersionByte         = verror.Register(pkgPath+".errBadVersionByte", verror.NoRetry, "{1:}{2:} bad version byte {3}")
	errEndedBeforeVersionByte = verror.Register(pkgPath+".errEndedBeforeVersionByte", verror.NoRetry, "{1:}{2:} ended before version byte received {:_}")
)

// Binary encoding and decoding routines.  The binary format is identical to the
// encoding/gob package, and much of the implementation is similar.

const (
	uint64Size          = 8
	maxEncodedUintBytes = uint64Size + 1 // +1 for length byte
	maxBinaryMsgLen     = 1 << 30        // 1GiB limit to each message
)

// lenUint retuns the number of bytes used to represent the provided
// uint value
func lenUint(v uint64) uint64 {
	switch {
	case v <= 0x7f:
		return 1
	case v <= 0xff:
		return 2
	case v <= 0xffff:
		return 3
	case v <= 0xffffff:
		return 4
	case v <= 0xffffffff:
		return 5
	case v <= 0xffffffffff:
		return 6
	case v <= 0xffffffffffff:
		return 7
	case v <= 0xffffffffffffff:
		return 8
	default:
		return 9
	}
}

func intToUint(v int64) uint64 {
	var uval uint64
	if v < 0 {
		uval = uint64(^v<<1) | 1
	} else {
		uval = uint64(v << 1)
	}
	return uval
}

func uintToInt(uval uint64) int64 {
	if uval&1 == 1 {
		return ^int64(uval >> 1)
	}
	return int64(uval >> 1)
}

func binaryEncodeControl(buf *encbuf, v byte) {
	if v < 0x80 || v > 0xef {
		panic(verror.New(errBadControlCode, nil, v))
	}
	buf.WriteOneByte(v)
}

// If the byte is not a control, this will return 0.
func binaryPeekControl(buf *decbuf) (byte, error) {
	v, err := buf.PeekByte()
	if err != nil {
		return 0, err
	}
	if v < 0x80 || v > 0xef {
		return 0, nil
	}
	return v, nil
}

// Bools are encoded as a byte where 0 = false and anything else is true.
func binaryEncodeBool(buf *encbuf, v bool) {
	if v {
		buf.WriteOneByte(1)
	} else {
		buf.WriteOneByte(0)
	}
}

func binaryDecodeBool(buf *decbuf) (bool, error) {
	v, err := buf.ReadByte()
	if err != nil {
		return false, err
	}
	if v > 0x7f {
		return false, verror.New(errInvalid, nil)
	}
	return v != 0, nil
}

// Unsigned integers are the basis for all other primitive values.  This is a
// two-state encoding.  If the number is less than 128 (0 through 0x7f), its
// value is written directly.  Otherwise the value is written in big-endian byte
// order preceded by the negated byte length.
func binaryEncodeUint(buf *encbuf, v uint64) {
	switch {
	case v <= 0x7f:
		buf.Grow(1)[0] = byte(v)
	case v <= 0xff:
		buf := buf.Grow(2)
		buf[0] = 0xff
		buf[1] = byte(v)
	case v <= 0xffff:
		buf := buf.Grow(3)
		buf[0] = 0xfe
		buf[1] = byte(v >> 8)
		buf[2] = byte(v)
	case v <= 0xffffff:
		buf := buf.Grow(4)
		buf[0] = 0xfd
		buf[1] = byte(v >> 16)
		buf[2] = byte(v >> 8)
		buf[3] = byte(v)
	case v <= 0xffffffff:
		buf := buf.Grow(5)
		buf[0] = 0xfc
		buf[1] = byte(v >> 24)
		buf[2] = byte(v >> 16)
		buf[3] = byte(v >> 8)
		buf[4] = byte(v)
	case v <= 0xffffffffff:
		buf := buf.Grow(6)
		buf[0] = 0xfb
		buf[1] = byte(v >> 32)
		buf[2] = byte(v >> 24)
		buf[3] = byte(v >> 16)
		buf[4] = byte(v >> 8)
		buf[5] = byte(v)
	case v <= 0xffffffffffff:
		buf := buf.Grow(7)
		buf[0] = 0xfa
		buf[1] = byte(v >> 40)
		buf[2] = byte(v >> 32)
		buf[3] = byte(v >> 24)
		buf[4] = byte(v >> 16)
		buf[5] = byte(v >> 8)
		buf[6] = byte(v)
	case v <= 0xffffffffffffff:
		buf := buf.Grow(8)
		buf[0] = 0xf9
		buf[1] = byte(v >> 48)
		buf[2] = byte(v >> 40)
		buf[3] = byte(v >> 32)
		buf[4] = byte(v >> 24)
		buf[5] = byte(v >> 16)
		buf[6] = byte(v >> 8)
		buf[7] = byte(v)
	default:
		buf := buf.Grow(9)
		buf[0] = 0xf8
		buf[1] = byte(v >> 56)
		buf[2] = byte(v >> 48)
		buf[3] = byte(v >> 40)
		buf[4] = byte(v >> 32)
		buf[5] = byte(v >> 24)
		buf[6] = byte(v >> 16)
		buf[7] = byte(v >> 8)
		buf[8] = byte(v)
	}
}

func byteSliceBinaryPeekUint(p []byte) (val uint64, byteLen int, err error) {
	v, cr, bytelen, err := byteSliceBinaryPeekUintWithControl(p)
	if err != nil {
		return 0, 0, err
	}
	if cr != 0 {
		return 0, 0, verror.New(errInvalid, nil)
	}
	return v, bytelen, nil
}

func binaryDecodeUint(buf *decbuf) (uint64, error) {
	v, bytelen, err := binaryPeekUint(buf)
	if err != nil {
		return 0, err
	}
	return v, buf.Skip(bytelen)
}

func byteSliceBinaryPeekUintWithControl(b []byte) (val uint64, cr byte, lenRead int, err error) {
	if len(b) == 0 {
		return 0, 0, 0, io.EOF
	}
	firstByte := b[0]
	// Handle single-byte encoding.
	if firstByte <= 0x7f {
		return uint64(firstByte), 0, 1, nil
	}
	// Handle control code.
	if firstByte <= 0xef {
		return 0, byte(firstByte), 1, nil
	}
	// Handle multi-byte encoding.
	byteLen := int(-int8(firstByte))
	if byteLen < 1 || byteLen > uint64Size {
		return 0, 0, 0, verror.New(errUintOverflow, nil)
	}
	if byteLen >= len(b) {
		return 0, 0, 0, io.EOF
	}
	var v uint64
	for _, b := range b[1 : byteLen+1] {
		v = v<<8 | uint64(b)
	}
	return v, 0, byteLen + 1, nil
}

func byteSliceBinaryPeekIntWithControl(b []byte) (val int64, cr byte, lenRead int, err error) {
	var uval uint64
	uval, cr, lenRead, err = byteSliceBinaryPeekUintWithControl(b)
	val = uintToInt(uval)
	return
}

func binaryPeekUint(buf *decbuf) (uint64, int, error) {
	firstByte, err := buf.PeekByte()
	if err != nil {
		return 0, 0, err
	}
	// Handle single-byte encoding.
	if firstByte <= 0x7f {
		return uint64(firstByte), 1, nil
	}
	// Verify not a control code.
	if firstByte <= 0xef {
		return 0, 0, verror.New(errInvalid, nil)
	}
	// Handle multi-byte encoding.
	byteLen := int(-int8(firstByte))
	if byteLen < 1 || byteLen > uint64Size {
		return 0, 0, verror.New(errUintOverflow, nil)
	}
	byteLen++ // account for initial len byte
	bytes, err := buf.PeekSmall(byteLen)
	if err != nil {
		return 0, 0, err
	}
	var v uint64
	for _, b := range bytes[1:byteLen] {
		v = v<<8 | uint64(b)
	}
	return v, byteLen, nil
}

func binaryDecodeUintWithControl(buf *decbuf) (uint64, byte, error) {
	v, c, bytelen, err := binaryPeekUintWithControl(buf)
	if err != nil {
		return 0, 0, err
	}
	return v, c, buf.Skip(bytelen)
}

func binaryDecodeIntWithControl(buf *decbuf) (int64, byte, error) {
	v, c, bytelen, err := binaryPeekUintWithControl(buf)
	if err != nil {
		return 0, 0, err
	}
	return uintToInt(v), c, buf.Skip(bytelen)
}

func binaryPeekUintWithControl(buf *decbuf) (uint64, byte, int, error) {
	firstByte, err := buf.PeekByte()
	if err != nil {
		return 0, 0, 0, err
	}
	// Handle single-byte encoding.
	if firstByte <= 0x7f {
		return uint64(firstByte), 0, 1, nil
	}
	// Handle control code.
	if firstByte <= 0xef {
		return 0, byte(firstByte), 1, nil
	}
	// Handle multi-byte encoding.
	byteLen := int(-int8(firstByte))
	if byteLen < 1 || byteLen > uint64Size {
		return 0, 0, 0, verror.New(errUintOverflow, nil)
	}
	byteLen++ // account for initial len byte
	bytes, err := buf.PeekSmall(byteLen)
	if err != nil {
		return 0, 0, 0, err
	}
	var v uint64
	for _, b := range bytes[1:byteLen] {
		v = v<<8 | uint64(b)
	}
	return v, 0, byteLen, nil
}

func binaryPeekIntWithControl(buf *decbuf) (int64, byte, int, error) {
	v, cr, l, err := binaryPeekUintWithControl(buf)
	return uintToInt(v), cr, l, err
}

func binaryPeekUintByteLen(buf *decbuf) (int, error) {
	firstByte, err := buf.PeekByte()
	if err != nil {
		return 0, err
	}
	if firstByte <= 0xef {
		return 1, nil
	}
	byteLen := int(-int8(firstByte))
	if byteLen > uint64Size {
		return 0, verror.New(errUintOverflow, nil)
	}
	return 1 + byteLen, nil
}

func binaryIgnoreUint(buf *decbuf) error {
	byteLen, err := binaryPeekUintByteLen(buf)
	if err != nil {
		return err
	}
	return buf.Skip(byteLen)
}

func byteSliceBinaryPeekLen(b []byte) (len, byteLen int, err error) {
	ulen, byteLen, err := byteSliceBinaryPeekUint(b)
	switch {
	case err != nil:
		return 0, 0, err
	case ulen > maxBinaryMsgLen:
		return 0, 0, verror.New(errMsgLen, nil, maxBinaryMsgLen)
	}
	return int(ulen), byteLen, nil
}

func binaryDecodeLen(buf *decbuf) (int, error) {
	ulen, err := binaryDecodeUint(buf)
	switch {
	case err != nil:
		return 0, err
	case ulen > maxBinaryMsgLen:
		return 0, verror.New(errMsgLen, nil, maxBinaryMsgLen)
	}
	return int(ulen), nil
}

func binaryDecodeLenOrArrayLen(buf *decbuf, t *vdl.Type) (int, error) {
	len, err := binaryDecodeLen(buf)
	if err != nil {
		return 0, err
	}
	if t.Kind() == vdl.Array {
		if len != 0 {
			return 0, verror.New(errInvalid, nil)
		}
		return t.Len(), nil
	}
	return len, nil
}

// Signed integers are encoded as unsigned integers, where the low bit says
// whether to complement the other bits to recover the int.
func binaryEncodeInt(buf *encbuf, v int64) {
	var uval uint64
	if v < 0 {
		uval = uint64(^v<<1) | 1
	} else {
		uval = uint64(v << 1)
	}
	binaryEncodeUint(buf, uval)
}

func binaryDecodeInt(buf *decbuf) (int64, error) {
	uval, err := binaryDecodeUint(buf)
	return uintToInt(uval), err
}

func binaryPeekInt(buf *decbuf) (int64, int, error) {
	uval, bytelen, err := binaryPeekUint(buf)
	if err != nil {
		return 0, 0, err
	}
	return uintToInt(uval), bytelen, err
}

// Floating point numbers are encoded as byte-reversed ieee754.
func binaryEncodeFloat(buf *encbuf, v float64) {
	ieee := math.Float64bits(v)
	// Manually-unrolled byte-reversing.
	uval := (ieee&0xff)<<56 |
		(ieee&0xff00)<<40 |
		(ieee&0xff0000)<<24 |
		(ieee&0xff000000)<<8 |
		(ieee&0xff00000000)>>8 |
		(ieee&0xff0000000000)>>24 |
		(ieee&0xff000000000000)>>40 |
		(ieee&0xff00000000000000)>>56
	binaryEncodeUint(buf, uval)
}

func binaryDecodeFloat(buf *decbuf) (float64, error) {
	uval, err := binaryDecodeUint(buf)
	if err != nil {
		return 0, err
	}
	// Manually-unrolled byte-reversing.
	ieee := (uval&0xff)<<56 |
		(uval&0xff00)<<40 |
		(uval&0xff0000)<<24 |
		(uval&0xff000000)<<8 |
		(uval&0xff00000000)>>8 |
		(uval&0xff0000000000)>>24 |
		(uval&0xff000000000000)>>40 |
		(uval&0xff00000000000000)>>56
	return math.Float64frombits(ieee), nil
}

// Strings are encoded as the byte count followed by uninterpreted bytes.
func binaryEncodeString(buf *encbuf, s string) {
	binaryEncodeUint(buf, uint64(len(s)))
	buf.Write([]byte(s))
}

func binaryDecodeString(buf *decbuf) (string, error) {
	len, err := binaryDecodeLen(buf)
	if err != nil {
		return "", err
	}
	p := make([]byte, len)
	if err := buf.ReadIntoBuf(p); err != nil {
		return "", err
	}
	return string(p), nil
}

func binaryIgnoreString(buf *decbuf) error {
	len, err := binaryDecodeLen(buf)
	if err != nil {
		return err
	}
	return buf.Skip(len)
}

// binaryEncodeUintEnd writes into the trailing part of buf and returns the start
// index of the encoded data.
//
// REQUIRES: buf is big enough to hold the encoded value.
func binaryEncodeUintEnd(buf []byte, v uint64) int {
	end := len(buf) - 1
	switch {
	case v <= 0x7f:
		buf[end] = byte(v)
		return end
	case v <= 0xff:
		buf[end-1] = 0xff
		buf[end] = byte(v)
		return end - 1
	case v <= 0xffff:
		buf[end-2] = 0xfe
		buf[end-1] = byte(v >> 8)
		buf[end] = byte(v)
		return end - 2
	case v <= 0xffffff:
		buf[end-3] = 0xfd
		buf[end-2] = byte(v >> 16)
		buf[end-1] = byte(v >> 8)
		buf[end] = byte(v)
		return end - 3
	case v <= 0xffffffff:
		buf[end-4] = 0xfc
		buf[end-3] = byte(v >> 24)
		buf[end-2] = byte(v >> 16)
		buf[end-1] = byte(v >> 8)
		buf[end] = byte(v)
		return end - 4
	case v <= 0xffffffffff:
		buf[end-5] = 0xfb
		buf[end-4] = byte(v >> 32)
		buf[end-3] = byte(v >> 24)
		buf[end-2] = byte(v >> 16)
		buf[end-1] = byte(v >> 8)
		buf[end] = byte(v)
		return end - 5
	case v <= 0xffffffffffff:
		buf[end-6] = 0xfa
		buf[end-5] = byte(v >> 40)
		buf[end-4] = byte(v >> 32)
		buf[end-3] = byte(v >> 24)
		buf[end-2] = byte(v >> 16)
		buf[end-1] = byte(v >> 8)
		buf[end] = byte(v)
		return end - 6
	case v <= 0xffffffffffffff:
		buf[end-7] = 0xf9
		buf[end-6] = byte(v >> 48)
		buf[end-5] = byte(v >> 40)
		buf[end-4] = byte(v >> 32)
		buf[end-3] = byte(v >> 24)
		buf[end-2] = byte(v >> 16)
		buf[end-1] = byte(v >> 8)
		buf[end] = byte(v)
		return end - 7
	default:
		buf[end-8] = 0xf8
		buf[end-7] = byte(v >> 56)
		buf[end-6] = byte(v >> 48)
		buf[end-5] = byte(v >> 40)
		buf[end-4] = byte(v >> 32)
		buf[end-3] = byte(v >> 24)
		buf[end-2] = byte(v >> 16)
		buf[end-1] = byte(v >> 8)
		buf[end] = byte(v)
		return end - 8
	}
}

// binaryEncodeIntEnd writes into the trailing part of buf and returns the start
// index of the encoded data.
//
// REQUIRES: buf is big enough to hold the encoded value.
func binaryEncodeIntEnd(buf []byte, v int64) int {
	return binaryEncodeUintEnd(buf, intToUint(v))
}
