package vom

import (
	"errors"
	"math"
)

// Raw encoding and decoding routines.  The raw format is identical to the
// encoding/gob package, and much of the implementation is similar.

const (
	uint64Size          = 8
	maxEncodedUintBytes = uint64Size + 1 // +1 for length byte
)

var errUintOverflow = errors.New("vom: can't handle primitive larger than uint64")

// Bools are encoded as a byte where 0 = false and anything else is true.
func rawEncodeBool(encbuf *encbuf, v bool) {
	if v {
		encbuf.WriteByte(1)
	} else {
		encbuf.WriteByte(0)
	}
}

func rawDecodeBool(decbuf *decbuf) (bool, error) {
	bval, err := decbuf.ReadByte()
	if err != nil {
		return false, err
	}
	return bval != 0, nil
}

// Unsigned integers are the basis for all other primitive values.  This is a
// two-state encoding.  If the number is less than 128 (0 through 0x7f), its
// value is written directly.  Otherwise the value is written in big-endian byte
// order preceded by the negated byte length.
func rawEncodeUint(encbuf *encbuf, v uint64) {
	switch {
	case v <= 0x7f:
		encbuf.Grow(1)[0] = byte(v)
	case v <= 0xff:
		buf := encbuf.Grow(2)
		buf[0] = 0xff
		buf[1] = byte(v)
	case v <= 0xffff:
		buf := encbuf.Grow(3)
		buf[0] = 0xfe
		buf[1] = byte(v >> 8)
		buf[2] = byte(v)
	case v <= 0xffffff:
		buf := encbuf.Grow(4)
		buf[0] = 0xfd
		buf[1] = byte(v >> 16)
		buf[2] = byte(v >> 8)
		buf[3] = byte(v)
	case v <= 0xffffffff:
		buf := encbuf.Grow(5)
		buf[0] = 0xfc
		buf[1] = byte(v >> 24)
		buf[2] = byte(v >> 16)
		buf[3] = byte(v >> 8)
		buf[4] = byte(v)
	case v <= 0xffffffffff:
		buf := encbuf.Grow(6)
		buf[0] = 0xfb
		buf[1] = byte(v >> 32)
		buf[2] = byte(v >> 24)
		buf[3] = byte(v >> 16)
		buf[4] = byte(v >> 8)
		buf[5] = byte(v)
	case v <= 0xffffffffffff:
		buf := encbuf.Grow(7)
		buf[0] = 0xfa
		buf[1] = byte(v >> 40)
		buf[2] = byte(v >> 32)
		buf[3] = byte(v >> 24)
		buf[4] = byte(v >> 16)
		buf[5] = byte(v >> 8)
		buf[6] = byte(v)
	case v <= 0xffffffffffffff:
		buf := encbuf.Grow(8)
		buf[0] = 0xf9
		buf[1] = byte(v >> 48)
		buf[2] = byte(v >> 40)
		buf[3] = byte(v >> 32)
		buf[4] = byte(v >> 24)
		buf[5] = byte(v >> 16)
		buf[6] = byte(v >> 8)
		buf[7] = byte(v)
	default:
		buf := encbuf.Grow(9)
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

// rawEncodeUintEnd writes into the trailing part of buf and returns the start
// index of the encoded data.
//
// REQUIRES: buf is big enough to hold the encoded value.
func rawEncodeUintEnd(buf []byte, v uint64) int {
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

func rawDecodeUint(decbuf *decbuf) (uint64, error) {
	firstByte, err := decbuf.ReadByte()
	if err != nil {
		return 0, err
	}

	// Handle simple single-byte case.
	if firstByte <= 0x7f {
		return uint64(firstByte), nil
	}

	// Handle multi-byte case.
	count := int(-int8(firstByte))
	if count > uint64Size {
		return 0, errUintOverflow
	}
	bytes, err := decbuf.ReadBuf(count)
	if err != nil {
		return 0, err
	}
	var v uint64
	for _, b := range bytes {
		v = v<<8 | uint64(b)
	}
	return v, nil
}

func rawIgnoreUint(decbuf *decbuf) error {
	firstByte, err := decbuf.ReadByte()
	if err != nil {
		return err
	}
	if firstByte <= 0x7f {
		return nil
	}
	count := int(-int8(firstByte))
	if count > uint64Size {
		return errUintOverflow
	}
	return decbuf.Skip(count)
}

// Signed integers are encoded as unsigned integers, where the low bit says
// whether to complement the other bits to recover the int.
func rawEncodeInt(encbuf *encbuf, v int64) {
	var uval uint64
	if v < 0 {
		uval = uint64(^v<<1) | 1
	} else {
		uval = uint64(v << 1)
	}
	rawEncodeUint(encbuf, uval)
}

// rawEncodeIntEnd writes into the trailing part of buf and returns the start
// index of the encoded data.
//
// REQUIRES: buf is big enough to hold the encoded value.
func rawEncodeIntEnd(buf []byte, v int64) int {
	var uval uint64
	if v < 0 {
		uval = uint64(^v<<1) | 1
	} else {
		uval = uint64(v << 1)
	}
	return rawEncodeUintEnd(buf, uval)
}

func rawDecodeInt(decbuf *decbuf) (int64, error) {
	uval, err := rawDecodeUint(decbuf)
	if err != nil {
		return 0, err
	}
	if uval&1 == 1 {
		return ^int64(uval >> 1), nil
	}
	return int64(uval >> 1), nil
}

// Floating point numbers are encoded as byte-reversed ieee754.
func rawEncodeFloat(encbuf *encbuf, v float64) {
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
	rawEncodeUint(encbuf, uval)
}

func rawDecodeFloat(decbuf *decbuf) (float64, error) {
	uval, err := rawDecodeUint(decbuf)
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

// ByteSlices are encoded as the byte count followed by uninterpreted bytes.
func rawEncodeByteSlice(encbuf *encbuf, v []byte) {
	rawEncodeUint(encbuf, uint64(len(v)))
	encbuf.Write(v)
}

func rawEncodeString(encbuf *encbuf, s string) {
	rawEncodeUint(encbuf, uint64(len(s)))
	encbuf.WriteString(s)
}

// There's no rawDecodeByteSlice() method; the decoder implements its own for
// performance reasons.

func rawIgnoreByteSlice(decbuf *decbuf) error {
	len, err := rawDecodeUint(decbuf)
	if err != nil {
		return err
	}
	return decbuf.Skip(int(len))
}
