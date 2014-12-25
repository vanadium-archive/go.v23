package valconv

import (
	"reflect"
	"unsafe"

	"v.io/veyron/veyron2/vdl"
)

var (
	bitlenReflect = [...]uintptr{
		reflect.Uint8:      8,
		reflect.Uint16:     16,
		reflect.Uint32:     32,
		reflect.Uint64:     64,
		reflect.Uint:       8 * unsafe.Sizeof(uint(0)),
		reflect.Uintptr:    8 * unsafe.Sizeof(uintptr(0)),
		reflect.Int8:       8,
		reflect.Int16:      16,
		reflect.Int32:      32,
		reflect.Int64:      64,
		reflect.Int:        8 * unsafe.Sizeof(int(0)),
		reflect.Float32:    32,
		reflect.Float64:    64,
		reflect.Complex64:  32, // bitlen of each float
		reflect.Complex128: 64, // bitlen of each float
	}

	bitlenVDL = [...]uintptr{
		vdl.Byte:       8,
		vdl.Uint16:     16,
		vdl.Uint32:     32,
		vdl.Uint64:     64,
		vdl.Int16:      16,
		vdl.Int32:      32,
		vdl.Int64:      64,
		vdl.Float32:    32,
		vdl.Float64:    64,
		vdl.Complex64:  32, // bitlen of each float
		vdl.Complex128: 64, // bitlen of each float
	}
)

// bitlen{R,V} enforce static type safety on kind.
func bitlenR(kind reflect.Kind) uintptr { return bitlenReflect[kind] }
func bitlenV(kind vdl.Kind) uintptr     { return bitlenVDL[kind] }

// isRTBytes returns true iff rt is an array or slice of bytes.
func isRTBytes(rt reflect.Type) bool {
	return (rt.Kind() == reflect.Array || rt.Kind() == reflect.Slice) && rt.Elem().Kind() == reflect.Uint8
}

// rtBytes extracts []byte from rv.  Assumes isRTBytes(rv.Type()) == true.
func rtBytes(rv reflect.Value) []byte {
	// Fastpath if the underlying type is []byte
	if rv.Kind() == reflect.Slice && rv.Type().Elem() == rtByte {
		return rv.Bytes()
	}
	// Slowpath copying bytes one by one.
	ret := make([]byte, rv.Len())
	for ix := 0; ix < rv.Len(); ix++ {
		ret[ix] = rv.Index(ix).Convert(rtByte).Interface().(byte)
	}
	return ret
}
