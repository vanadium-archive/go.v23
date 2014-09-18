package vom

import (
	"fmt"
	"math"
	"reflect"
	"strings"

	"veyron.io/veyron/veyron2/wiretype"
)

const (
	// Go represents float64 using IEEE 754, which uses 52 bits to represent the
	// mantissa, with an extra implied leading bit.  That gives us 53 bits to
	// store integers without overflow - i.e. [0, (2^53)-1].  And since 2^53 is a
	// small power of two, it can also be stored without loss via mantissa=1
	// exponent=53.  Thus we have our max and min values.  Ditto for float32,
	// which uses 23 bits with an extra implied leading bit.
	float64MaxInt = (1 << 53)
	float64MinInt = -(1 << 53)
	float32MaxInt = (1 << 24)
	float32MinInt = -(1 << 24)
)

// Format describes the encoding and decoding format.
type Format int

const (
	// FormatBinary is the most compact and has the fastest encoder and decoder.
	FormatBinary Format = iota
	// FormatJSON is in JSON format, and is human-readable and editable.
	FormatJSON
)

func (f Format) String() string {
	switch f {
	case FormatBinary:
		return "binary"
	case FormatJSON:
		return "JSON"
	default:
		panic(fmt.Errorf("vom: unknown format %d", f))
	}
}

const (
	// The first item of each messages is a TypeID encoded as a signed integer.
	// The following ids are special-cased to denote the start and end of the
	// json decoding mode.
	jsonStartID = int64(-46) // ascii '['
	jsonEndID   = int64(-47) // ascii ']'

	jsonStartByte = '['
	jsonEndByte   = ']'

	jsonNullString = "null"
)

// isByteSlice returns true iff rt is a named or unnamed []byte.  Returns false
// for named bytes; e.g. "type Foo byte; var foo []Foo".
func isByteSlice(rt Type) bool {
	return rt.Kind() == reflect.Slice && rt.Elem() == rtByte
}

// bytesToByteSlice converts an array or slice of named bytes to []byte.
func bytesToByteSlice(rv Value) []byte {
	ret := make([]byte, rv.Len())
	for ix := 0; ix < rv.Len(); ix++ {
		ret[ix] = rv.Index(ix).Convert(rtByte).Interface().(byte)
	}
	return ret
}

// isComplex64Def returns true iff the typeDef represents a complex64.
func isComplex64Def(def *wireDef) bool {
	return len(def.fields) == 2 &&
		def.fields[0].name == "R" && def.fields[0].def.kind == typeKindFloat &&
		def.fields[1].name == "I" && def.fields[1].def.kind == typeKindFloat &&
		containsTag(def.tags, "complex64")
}

// isComplex128Def returns true iff the typeDef represents a complex128.
func isComplex128Def(def *wireDef) bool {
	return len(def.fields) == 2 &&
		def.fields[0].name == "R" && def.fields[0].def.kind == typeKindFloat &&
		def.fields[1].name == "I" && def.fields[1].def.kind == typeKindFloat &&
		containsTag(def.tags, "complex128")
}

// containsTag returns true iff tags contains tag.
func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// containsTagPrefix returns true iff tags contains a tag with the given prefix.
func containsTagPrefix(tags []string, prefix string) bool {
	for _, t := range tags {
		if strings.HasPrefix(t, prefix) {
			return true
		}
	}
	return false
}

// isZeroValue returns true iff rv is the zero value for its type.
func isZeroValue(rv Value) bool {
	switch rv.Kind() {
	case reflect.Bool:
		return !rv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0
	case reflect.Complex64, reflect.Complex128:
		return rv.Complex() == 0
	case reflect.String, reflect.Slice, reflect.Map:
		return rv.Len() == 0
	case reflect.Array:
		for ix := 0; ix < rv.Len(); ix++ {
			if !isZeroValue(rv.Index(ix)) {
				return false
			}
		}
		return true
	case reflect.Struct:
		for fx := 0; fx < rv.NumField(); fx++ {
			if !isZeroValue(rv.Field(fx)) {
				return false
			}
		}
		return true
	case reflect.Ptr, reflect.Interface:
		return rv.IsNil()
	}
	panic(fmt.Errorf("vom: isZeroValue unhandled kind %q type %q", rv.Kind(), rv.Type()))
}

// alignStars deals with cases where rv is a pointer type, and it has more stars
// than def.  We guarantee that the number of stars in the returned rv <= the
// number of stars in def, taking custom decoder methods (e.g. VomDecode) into
// account.
//
// Note that if all pointers in rv are fully-allocated but represent a different
// sharing structure than the encoded value, we may end up with arbitrary
// results.  This is the same behavior as gob.
// TODO(toddw): We could attempt to detect this scenario, but it could be too
// expensive.  Add this detection as an option if necessary.
func alignStars(def *wireDef, rti *rtInfo, rv Value, format Format) (*rtInfo, Value) {
	rvStars := rti.numStars
	customStars := rti.numCustomStars(format)
	defStars := def.numStars
	// We must take customStars into account, but cannot dereference extra stars
	// past the base stars in rv.  This function will be called again after the
	// VomDecode method has been handled, to dereference the custom stars.
	extra := rvStars + customStars - defStars
	if extra > rvStars {
		extra = rvStars
	}
	for ; extra > 0; extra-- {
		if rv.IsNil() {
			rv.Set(New(rti.elem.rt))
		}
		rti = rti.elem
		rv = rv.Elem()
	}
	return rti, rv
}

func decodeByte(buf *decbuf, rti *rtInfo, rv Value) error {
	// Bytes may be decoded into any numeric type, as long as it can hold the
	// value without overflow.
	bval, err := buf.ReadByte()
	if err != nil {
		return err
	}
	return fillFromByte(bval, rti, rv)
}

func fillFromByte(bval byte, rti *rtInfo, rv Value) error {
	switch rti.kind {
	case typeKindUint, typeKindByte:
		rv.SetUint(uint64(bval))
		return nil
	case typeKindInt:
		// Byte might overflow int8.
		if rti.id != wiretype.TypeIDInt8 || bval <= 127 {
			rv.SetInt(int64(bval))
			return nil
		}
	case typeKindFloat:
		rv.SetFloat(float64(bval))
		return nil
	}
	return fmt.Errorf("vom: type mismatch, can't fill byte %v into value of type %q", bval, rti.rt)
}

func decodeUint(buf *decbuf, rti *rtInfo, rv Value) error {
	// Uints may be decoded into any numeric type, as long as it can hold the
	// value without overflow.
	uval, err := rawDecodeUint(buf)
	if err != nil {
		return err
	}
	return fillFromUint(uval, rti, rv)
}

func fillFromUint(uval uint64, rti *rtInfo, rv Value) error {
	switch rti.kind {
	case typeKindUint, typeKindByte:
		if !rv.OverflowUint(uval) {
			rv.SetUint(uval)
			return nil
		}
	case typeKindInt:
		ival := int64(uval)
		if ival >= 0 && !rv.OverflowInt(ival) {
			rv.SetInt(ival)
			return nil
		}
	case typeKindFloat:
		if rti.id == wiretype.TypeIDFloat32 {
			// OverflowFloat only checks that the float64 can fit in the range of the
			// float32, it doesn't check for loss of precision.
			fval := float64(uval)
			if uval <= float32MaxInt && !rv.OverflowFloat(fval) {
				rv.SetFloat(fval)
				return nil
			}
		} else if rti.id == wiretype.TypeIDFloat64 {
			fval := float64(uval)
			if uval <= float64MaxInt {
				rv.SetFloat(fval)
				return nil
			}
		}
	}

	return fmt.Errorf("vom: type mismatch, can't fill uint %v into value of type %q", uval, rti.rt)
}

func decodeInt(buf *decbuf, rti *rtInfo, rv Value) error {
	// Ints may be decoded into any numeric type, as long as it can hold the value
	// without overflow.
	ival, err := rawDecodeInt(buf)
	if err != nil {
		return err
	}
	return fillFromInt(ival, rti, rv)
}

func fillFromInt(ival int64, rti *rtInfo, rv Value) error {
	switch rti.kind {
	case typeKindUint, typeKindByte:
		uval := uint64(ival)
		if ival >= 0 && !rv.OverflowUint(uval) {
			rv.SetUint(uval)
			return nil
		}
	case typeKindInt:
		if !rv.OverflowInt(ival) {
			rv.SetInt(ival)
			return nil
		}
	case typeKindFloat:
		if rti.id == wiretype.TypeIDFloat32 {
			// OverflowFloat only checks that the float64 can fit in the range of the
			// float32, it doesn't check for loss of precision.
			fval := float64(ival)
			if float32MinInt <= ival && ival <= float32MaxInt && !rv.OverflowFloat(fval) {
				rv.SetFloat(fval)
				return nil
			}
		} else if rti.id == wiretype.TypeIDFloat64 {
			fval := float64(ival)
			if float64MinInt <= ival && ival <= float64MaxInt {
				rv.SetFloat(fval)
				return nil
			}
		}
	}
	return fmt.Errorf("vom: type mismatch, can't fill int %v into value of type %q", ival, rti.rt)
}

func decodeFloat(buf *decbuf, rti *rtInfo, rv Value) error {
	// Floats may be decoded into any numeric type, as long as it can hold the
	// value without overflow.
	fval, err := rawDecodeFloat(buf)
	if err != nil {
		return err
	}
	return fillFromFloat(fval, rti, rv)
}

func fillFromFloat(fval float64, rti *rtInfo, rv Value) error {
	switch rti.kind {
	case typeKindUint, typeKindByte:
		uval := uint64(fval)
		_, frac := math.Modf(fval)
		if fval >= 0 && frac == 0 && !rv.OverflowUint(uval) {
			rv.SetUint(uval)
			return nil
		}
	case typeKindInt:
		ival := int64(fval)
		_, frac := math.Modf(fval)
		if frac == 0 && !rv.OverflowInt(ival) {
			rv.SetInt(ival)
			return nil
		}
	case typeKindFloat:
		if rti.id == wiretype.TypeIDFloat32 {
			// OverflowFloat only checks that the float64 can fit in the range of the
			// float32, it doesn't check for loss of precision.
			if !rv.OverflowFloat(fval) {
				rv.SetFloat(fval)
				return nil
			}
		} else if rti.id == wiretype.TypeIDFloat64 {
			rv.SetFloat(fval)
			return nil
		}
	}
	return fmt.Errorf("vom: type mismatch, can't fill float %v into value of type %q", fval, rti.rt)
}

func decodeBool(buf *decbuf, rti *rtInfo, rv Value) error {
	// Bools must be decoded into a bool.
	bval, err := rawDecodeBool(buf)
	if err != nil {
		return err
	}
	return fillFromBool(bval, rti, rv)
}

func fillFromBool(bval bool, rti *rtInfo, rv Value) error {
	if rti.kind == typeKindBool {
		rv.SetBool(bval)
		return nil
	}
	return fmt.Errorf("vom: type mismatch, can't fill bool %v into value of type %q", bval, rti.rt)
}

func decodeString(buf *decbuf, rti *rtInfo, rv Value) error {
	// Strings start with the byte count followed by uninterpreted bytes.
	ulen, err := rawDecodeUint(buf)
	if err != nil {
		return err
	}
	len := int(ulen)
	// Only create a new slice if the decbuf can't hold the entire string.
	var isNewSlice bool
	var bs []byte
	if len <= buf.Size() {
		bs, err = buf.ReadBuf(len)
	} else {
		isNewSlice = true
		bs = make([]byte, len)
		_, err = buf.Read(bs)
	}
	if err != nil {
		return err
	}
	return fillFromByteSlice(bs, isNewSlice, rti, rv)
}

func fillFromByteSlice(bs []byte, isNewSlice bool, rti *rtInfo, rv Value) error {
	len := len(bs)
	switch rti.kind {
	case typeKindString:
		// Converting from []byte to string mandates a copy.  Here's unsafe code
		// (copied from gob) that avoids the copy.  It's super-unsafe since we're
		// depending on the memory representation of string to match the beginning
		// of the memory representation of []byte.
		// TODO(toddw): See if it's worth enabling.
		//
		//   if isNewSlice {
		//   	p := unsafe.Pointer(rv.UnsafeAddr())
		//   	if p == nil {
		//   		p = unsafe.Pointer(new(string))
		//   	}
		//   	*(*string)(p) = *(*string)(unsafe.Pointer(&bs))
		//   	return nil
		//   }
		rv.SetString(string(bs))
		return nil
	case typeKindByteSlice:
		if !isNewSlice {
			// Must make a copy of bs with a new underlying array, since
			// reflect.SetBytes only copies the slice header.
			newbs := make([]byte, len)
			copy(newbs, bs)
			bs = newbs
		}
		rv.SetBytes(bs)
		return nil
	case typeKindSlice:
		if len <= rv.Cap() {
			rv.SetLen(len)
		} else {
			rv.Set(MakeSlice(rti.rt, len, len))
		}
		if fastFillFromByteSlice(bs, rti, rv) {
			return nil
		}
	case typeKindArray:
		if len > rti.len {
			break
		}
		if fastFillFromByteSlice(bs, rti, rv) {
			return nil
		}
	}
	return fmt.Errorf("vom: type mismatch, can't fill string with len %d into value of type %q", len, rti.rt)
}

// fastFillFromByteSlice fills rv from bs, assuming that rv is a slice or array
// with enough capacity to hold the items in bs.  Returns true if rv was
// successfully filled with all bytes from bs, otherwise returns false.
//
// This is logically the same as calling fillFromByte() on each byte in bs, but
// is faster since most cases don't need error checking in the inner loop.
func fastFillFromByteSlice(bs []byte, rti *rtInfo, rv Value) bool {
	switch rti.elem.kind {
	case typeKindByte, typeKindUint:
		for bx, b := range bs {
			rv.Index(bx).SetUint(uint64(b))
		}
		return true
	case typeKindInt:
		if rti.elem.id == wiretype.TypeIDInt8 {
			// Byte might overflow int8
			for bx, b := range bs {
				if b > 127 {
					return false
				}
				rv.Index(bx).SetInt(int64(b))
			}
			return true
		}
		for bx, b := range bs {
			rv.Index(bx).SetInt(int64(b))
		}
		return true
	case typeKindFloat:
		for bx, b := range bs {
			rv.Index(bx).SetFloat(float64(b))
		}
		return true
	default:
		return false
	}
}
