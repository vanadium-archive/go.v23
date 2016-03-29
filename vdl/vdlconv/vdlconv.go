// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdlconv

import (
	"fmt"
)

// TODO(bprosnitz) The go compiler probably won't inline these functions because of the
// Errorf() call. Consider other variants, such as having the functions just return a
// boolean for overflow and output the error in the body of the generated function.

const (
	// IEEE 754 represents float64 using 52 bits to represent the mantissa, with
	// an extra implied leading bit.  That gives us 53 bits to store integers
	// without overflow - i.e. [0, (2^53)-1].  And since 2^53 is a small power of
	// two, it can also be stored without loss via mantissa=1 exponent=53.  Thus
	// we have our max and min values.  Ditto for float32, which uses 23 bits with
	// an extra implied leading bit.
	float64MaxInt = (1 << 53)
	float64MinInt = -(1 << 53)
	float32MaxInt = (1 << 24)
	float32MinInt = -(1 << 24)
)

const (
	shiftFor8Bit  = 64 - 8
	shiftFor16Bit = 64 - 16
	shiftFor32Bit = 64 - 32
)

func Uint64ToUint8(x uint64) (uint8, error) {
	if x != (x<<shiftFor8Bit)>>shiftFor8Bit {
		return 0, fmt.Errorf("uint64 value %d cannot be converted to uint8 without loss", x)
	}
	return uint8(x), nil
}

func Uint64ToUint16(x uint64) (uint16, error) {
	if x != (x<<shiftFor16Bit)>>shiftFor16Bit {
		return 0, fmt.Errorf("uint64 value %d cannot be converted to uint16 without loss", x)
	}
	return uint16(x), nil
}

func Uint64ToUint32(x uint64) (uint32, error) {
	if x != (x<<shiftFor32Bit)>>shiftFor32Bit {
		return 0, fmt.Errorf("uint64 value %d cannot be converted to uint32 without loss", x)
	}
	return uint32(x), nil
}

func Uint64ToInt8(x uint64) (int8, error) {
	ix := int64(x)
	if ix < 0 || ix != (ix<<shiftFor8Bit)>>shiftFor8Bit {
		return 0, fmt.Errorf("uint64 value %d cannot be converted to int8 without loss", x)
	}
	return int8(ix), nil
}

func Uint64ToInt16(x uint64) (int16, error) {
	ix := int64(x)
	if ix < 0 || ix != (ix<<shiftFor16Bit)>>shiftFor16Bit {
		return 0, fmt.Errorf("uint64 value %d cannot be converted to int16 without loss", x)
	}
	return int16(ix), nil
}

func Uint64ToInt32(x uint64) (int32, error) {
	ix := int64(x)
	if ix < 0 || ix != (ix<<shiftFor32Bit)>>shiftFor32Bit {
		return 0, fmt.Errorf("uint64 value %d cannot be converted to int32 without loss", x)
	}
	return int32(ix), nil
}

func Uint64ToInt64(x uint64) (int64, error) {
	ix := int64(x)
	if ix < 0 {
		return 0, fmt.Errorf("uint64 value %d cannot be converted to int64 without loss", x)
	}
	return ix, nil
}

func Uint64ToFloat32(x uint64) (float32, error) {
	if x > float32MaxInt {
		return 0, fmt.Errorf("uint64 value %d cannot be converted to float32 without loss", x)
	}
	return float32(x), nil
}

func Uint64ToFloat64(x uint64) (float64, error) {
	if x > float64MaxInt {
		return 0, fmt.Errorf("uint64 value %d cannot be converted to float64 without loss", x)
	}
	return float64(x), nil
}

func Int64ToUint8(x int64) (uint8, error) {
	ux := uint64(x)
	if x < 0 || ux != (ux<<shiftFor8Bit)>>shiftFor8Bit {
		return 0, fmt.Errorf("int64 value %d cannot be converted to uint8 without loss", x)
	}
	return uint8(ux), nil
}

func Int64ToUint16(x int64) (uint16, error) {
	ux := uint64(x)
	if x < 0 || ux != (ux<<shiftFor16Bit)>>shiftFor16Bit {
		return 0, fmt.Errorf("int64 value %d cannot be converted to uint16 without loss", x)
	}
	return uint16(ux), nil
}

func Int64ToUint32(x int64) (uint32, error) {
	ux := uint64(x)
	if x < 0 || ux != (ux<<shiftFor32Bit)>>shiftFor32Bit {
		return 0, fmt.Errorf("int64 value %d cannot be converted to uint32 without loss", x)
	}
	return uint32(ux), nil
}

func Int64ToUint64(x int64) (uint64, error) {
	ux := uint64(x)
	if x < 0 {
		return 0, fmt.Errorf("int64 value %d cannot be converted to uint64 without loss", x)

	}
	return ux, nil
}

func Int64ToInt8(x int64) (int8, error) {
	if x != (x<<shiftFor8Bit)>>shiftFor8Bit {
		return 0, fmt.Errorf("int64 value %d cannot be converted to int8 without loss", x)
	}
	return int8(x), nil
}

func Int64ToInt16(x int64) (int16, error) {
	if x != (x<<shiftFor16Bit)>>shiftFor16Bit {
		return 0, fmt.Errorf("int64 value %d cannot be converted to int16 without loss", x)
	}
	return int16(x), nil
}

func Int64ToInt32(x int64) (int32, error) {
	if x != (x<<shiftFor32Bit)>>shiftFor32Bit {
		return 0, fmt.Errorf("int64 value %d cannot be converted to int32 without loss", x)
	}
	return int32(x), nil
}

func Int64ToFloat32(x int64) (float32, error) {
	if x < float32MinInt || x > float32MaxInt {
		return 0, fmt.Errorf("int64 value %d cannot be converted to float32 without loss", x)
	}
	return float32(x), nil
}

func Int64ToFloat64(x int64) (float64, error) {
	if x < float64MinInt || x > float64MaxInt {
		return 0, fmt.Errorf("int64 value %d cannot be converted to float64 without loss", x)
	}
	return float64(x), nil
}

func Float64ToUint8(x float64) (uint8, error) {
	ux := uint64(x)
	if x != float64(ux) || ux != (ux<<shiftFor8Bit)>>shiftFor8Bit {
		return 0, fmt.Errorf("float64 value %f cannot be converted to uint8 without loss", x)
	}
	return uint8(ux), nil
}

func Float64ToUint16(x float64) (uint16, error) {
	ux := uint64(x)
	if x != float64(ux) || ux != (ux<<shiftFor16Bit)>>shiftFor16Bit {
		return 0, fmt.Errorf("float64 value %f cannot be converted to uint16 without loss", x)
	}
	return uint16(ux), nil
}

func Float64ToUint32(x float64) (uint32, error) {
	ux := uint64(x)
	if x != float64(ux) || ux != (ux<<shiftFor32Bit)>>shiftFor32Bit {
		return 0, fmt.Errorf("float64 value %f cannot be converted to uint32 without loss", x)
	}
	return uint32(ux), nil
}

func Float64ToUint64(x float64) (uint64, error) {
	ux := uint64(x)
	if x != float64(ux) {
		return 0, fmt.Errorf("float64 value %f cannot be converted to uint64 without loss", x)
	}
	return ux, nil
}

func Float64ToInt8(x float64) (int8, error) {
	ix := int64(x)
	if x != float64(ix) || ix != (ix<<shiftFor8Bit)>>shiftFor8Bit {
		return 0, fmt.Errorf("float64 value %f cannot be converted to int8 without loss", x)
	}
	return int8(ix), nil
}

func Float64ToInt16(x float64) (int16, error) {
	ix := int64(x)
	if x != float64(ix) || ix != (ix<<shiftFor16Bit)>>shiftFor16Bit {
		return 0, fmt.Errorf("float64 value %f cannot be converted to int16 without loss", x)
	}
	return int16(ix), nil
}

func Float64ToInt32(x float64) (int32, error) {
	ix := int64(x)
	if x != float64(ix) || ix != (ix<<shiftFor32Bit)>>shiftFor32Bit {
		return 0, fmt.Errorf("float64 value %f cannot be converted to int32 without loss", x)
	}
	return int32(ix), nil
}

func Float64ToInt64(x float64) (int64, error) {
	ix := int64(x)
	if x != float64(ix) {
		return 0, fmt.Errorf("float64 value %f cannot be converted to int64 without loss", x)
	}
	return ix, nil
}

func Float64ToFloat32(x float64) (float32, error) {
	return float32(x), nil
}
