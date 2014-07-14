package valconv

import (
	"math"
)

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

func overflowUint(x uint64, bitlen uintptr) bool {
	shift := 64 - bitlen
	return x != (x<<shift)>>shift
}

func overflowInt(x int64, bitlen uintptr) bool {
	shift := 64 - bitlen
	return x != (x<<shift)>>shift
}

func convertIntToUint(x int64, bitlen uintptr) (uint64, bool) {
	ux := uint64(x)
	return ux, x >= 0 && !overflowUint(ux, bitlen)
}

func convertUintToInt(x uint64, bitlen uintptr) (int64, bool) {
	ix := int64(x)
	return ix, ix >= 0 && !overflowInt(ix, bitlen)
}

func convertUintToFloat(x uint64, bitlen uintptr) (float64, bool) {
	switch bitlen {
	case 32:
		return float64(x), x <= float32MaxInt
	default:
		return float64(x), x <= float64MaxInt
	}
}

func convertIntToFloat(x int64, bitlen uintptr) (float64, bool) {
	switch bitlen {
	case 32:
		return float64(x), float32MinInt <= x && x <= float32MaxInt
	default:
		return float64(x), float64MinInt <= x && x <= float64MaxInt
	}
}

func convertFloatToUint(x float64, bitlen uintptr) (uint64, bool) {
	ux := uint64(x)
	intPart, fracPart := math.Modf(x)
	ok := x >= 0 && fracPart == 0 && intPart <= math.MaxUint64 && !overflowUint(ux, bitlen)
	return ux, ok
}

func convertFloatToInt(x float64, bitlen uintptr) (int64, bool) {
	ix := int64(x)
	intPart, fracPart := math.Modf(x)
	ok := fracPart == 0 && intPart >= math.MinInt64 && intPart <= math.MaxInt64 && !overflowInt(ix, bitlen)
	return ix, ok
}

func convertComplexToUint(x complex128, bitlen uintptr) (uint64, bool) {
	re, im := real(x), imag(x)
	ux, ok := convertFloatToUint(re, bitlen)
	return ux, im == 0 && ok
}

func convertComplexToInt(x complex128, bitlen uintptr) (int64, bool) {
	re, im := real(x), imag(x)
	ix, ok := convertFloatToInt(re, bitlen)
	return ix, im == 0 && ok
}

func convertFloatToFloat(x float64, bitlen uintptr) float64 {
	switch bitlen {
	case 32:
		return float64(float32(x))
	default:
		return x
	}
}
