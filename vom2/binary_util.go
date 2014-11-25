package vom2

import (
	"math"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/verror"
)

// Binary encoding and decoding routines.  The binary format is identical to the
// encoding/gob package, and much of the implementation is similar.

const (
	uint64Size          = 8
	maxEncodedUintBytes = uint64Size + 1 // +1 for length byte
	maxBinaryMsgLen     = 1 << 30        // 1GiB limit to each message

	// Every binary stream starts with this magic byte, to distinguish the binary
	// encoding from the JSON encoding.  Note that every valid JSON encoding must
	// start with an ASCII character, or the BOM U+FEFF, and this magic byte is
	// unambiguous regardless of the endianness of the JSON encoding.
	binaryMagicByte byte = 0x80
)

var (
	errUintOverflow = verror.BadProtocolf("vom: scalar larger than 8 bytes")
	errMsgLen       = verror.BadProtocolf("vom: message larger than %d bytes", maxBinaryMsgLen)
)

// hasBinaryMsgLen returns true iff the type t is encoded with a top-level
// message length.
func hasBinaryMsgLen(t *vdl.Type) bool {
	if t.IsBytes() {
		return false
	}
	switch t.Kind() {
	case vdl.Complex64, vdl.Complex128, vdl.Array, vdl.List, vdl.Set, vdl.Map, vdl.Struct, vdl.Any, vdl.OneOf, vdl.Optional:
		return true
	}
	return false
}

// Bools are encoded as a byte where 0 = false and anything else is true.
func binaryEncodeBool(buf *encbuf, v bool) {
	if v {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
}

func binaryDecodeBool(buf *decbuf) (bool, error) {
	bval, err := buf.ReadByte()
	if err != nil {
		return false, err
	}
	return bval != 0, nil
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

func binaryDecodeUint(buf *decbuf) (uint64, error) {
	v, bytelen, err := binaryPeekUint(buf)
	if err != nil {
		return 0, err
	}
	return v, buf.Skip(bytelen)
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
	// Handle multi-byte encoding.
	byteLen := int(-int8(firstByte))
	if byteLen > uint64Size {
		return 0, 0, errUintOverflow
	}
	byteLen++ // account for initial len byte
	bytes, err := buf.PeekAtLeast(byteLen)
	if err != nil {
		return 0, 0, err
	}
	var v uint64
	for _, b := range bytes[1:byteLen] {
		v = v<<8 | uint64(b)
	}
	return v, byteLen, nil
}

func binaryPeekUintByteLen(buf *decbuf) (int, error) {
	firstByte, err := buf.PeekByte()
	if err != nil {
		return 0, err
	}
	if firstByte <= 0x7f {
		return 1, nil
	}
	byteLen := int(-int8(firstByte))
	if byteLen > uint64Size {
		return 0, errUintOverflow
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

func binaryDecodeLen(buf *decbuf) (int, error) {
	ulen, err := binaryDecodeUint(buf)
	switch {
	case err != nil:
		return 0, err
	case ulen > maxBinaryMsgLen:
		return 0, errMsgLen
	}
	return int(ulen), nil
}

func binaryDecodeLenOrArrayLen(buf *decbuf, t *vdl.Type) (int, error) {
	if t.Kind() == vdl.Array {
		return t.Len(), nil
	}
	return binaryDecodeLen(buf)
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

// binaryEncodeIntEnd writes into the trailing part of buf and returns the start
// index of the encoded data.
//
// REQUIRES: buf is big enough to hold the encoded value.
func binaryEncodeIntEnd(buf []byte, v int64) int {
	var uval uint64
	if v < 0 {
		uval = uint64(^v<<1) | 1
	} else {
		uval = uint64(v << 1)
	}
	return binaryEncodeUintEnd(buf, uval)
}

func binaryDecodeInt(buf *decbuf) (int64, error) {
	uval, err := binaryDecodeUint(buf)
	if err != nil {
		return 0, err
	}
	if uval&1 == 1 {
		return ^int64(uval >> 1), nil
	}
	return int64(uval >> 1), nil
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
	buf.WriteString(s)
}

func binaryDecodeString(buf *decbuf) (string, error) {
	len, err := binaryDecodeLen(buf)
	if err != nil {
		return "", err
	}
	bytes, err := buf.ReadBuf(len)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func binaryIgnoreString(buf *decbuf) error {
	len, err := binaryDecodeLen(buf)
	if err != nil {
		return err
	}
	return buf.Skip(len)
}
