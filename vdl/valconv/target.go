// Package valconv provides conversion routines between vdl.Value and
// reflect.Value, as well as a generic conversion target interface.
package valconv

import (
	"fmt"
	"reflect"

	"veyron.io/veyron/veyron2/vdl"
	"veyron.io/veyron/veyron2/verror"
)

// Target represents a generic conversion target; objects that implement this
// interface may be used as the target of a value conversion.  E.g.
// ReflectTarget and ValueTarget create targets based on an underlying
// reflect.Value or vdl.Value respectively.  Other implementations of this
// interface include vom encoding targets, that produce binary and JSON vom
// encodings.
type Target interface {
	// FromBool converts from the src bool to the target, where tt represents the
	// concrete type of bool.
	FromBool(src bool, tt *vdl.Type) error
	// FromUint converts from the src uint to the target, where tt represents the
	// concrete type of uint.
	FromUint(src uint64, tt *vdl.Type) error
	// FromInt converts from the src int to the target, where tt represents the
	// concrete type of int.
	FromInt(src int64, tt *vdl.Type) error
	// FromFloat converts from the src float to the target, where tt represents
	// the concrete type of float.
	FromFloat(src float64, tt *vdl.Type) error
	// FromComplex converts from the src complex to the target, where tt
	// represents the concrete type of complex.
	FromComplex(src complex128, tt *vdl.Type) error
	// FromBytes converts from the src bytes to the target, where tt represents
	// the concrete type of bytes.
	FromBytes(src []byte, tt *vdl.Type) error
	// FromString converts from the src string to the target, where tt represents
	// the concrete type of string.
	FromString(src string, tt *vdl.Type) error
	// FromEnumLabel converts from the src enum label to the target, where tt
	// represents the concrete type of enum.
	FromEnumLabel(src string, tt *vdl.Type) error
	// FromTypeObject converts from the src type to the target.
	FromTypeObject(src *vdl.Type) error
	// FromNil converts from a nil (nonexistent) value of type tt, where tt must
	// be of kind Optional or Any.
	FromNil(tt *vdl.Type) error

	// StartList prepares conversion from a list or array of type tt, with the
	// given len.  FinishList must be called to finish the list.
	StartList(tt *vdl.Type, len int) (ListTarget, error)
	// FinishList finishes a prior StartList call.
	FinishList(x ListTarget) error

	// StartSet prepares conversion from a set of type tt, with the given len.
	// FinishSet must be called to finish the set.
	StartSet(tt *vdl.Type, len int) (SetTarget, error)
	// FinishSet finishes a prior StartSet call.
	FinishSet(x SetTarget) error

	// StartMap prepares conversion from a map of type tt, with the given len.
	// FinishMap must be called to finish the map.
	StartMap(tt *vdl.Type, len int) (MapTarget, error)
	// FinishMap finishes a prior StartMap call.
	FinishMap(x MapTarget) error

	// StartStruct prepares conversion from a struct of type tt.  FinishStruct
	// must be called to finish the struct.
	StartStruct(tt *vdl.Type) (StructTarget, error)
	// FinishStruct finishes a prior StartStruct call.
	FinishStruct(x StructTarget) error

	// StartOneOf prepares conversion from a oneof of type tt.  FinishOneOf must
	// be called to finish the oneof.
	StartOneOf(tt *vdl.Type) (Target, error)
	// FinishOneOf finishes a prior StartOneOf call.
	FinishOneOf(x Target) error
}

// ListTarget represents conversion from a list or array.
type ListTarget interface {
	// StartElem prepares conversion of the next list elem.  The given index must
	// start at 0, and be incremented by one by each successive StartElem call.
	// FinishElem must be called to finish the elem.
	//
	// TODO(toddw): Remove index?
	StartElem(index int) (elem Target, _ error)
	// FinishElem finishes a prior StartElem call.
	FinishElem(elem Target) error
}

// SetTarget represents conversion from a set.
type SetTarget interface {
	// StartKey prepares conversion of the next set key.  FinishKey must be called
	// to finish the key.
	StartKey() (key Target, _ error)
	// FinishKey finishes a prior StartKey call.  An error with ID verror.NotFound
	// indicates the underlying target is a struct that doesn't have the given
	// field name.
	FinishKey(key Target) error
}

// MapTarget represents conversion from a map.
type MapTarget interface {
	// StartKey prepares conversion of the next map key.  FinishKeyStartField must
	// be called to finish the key.
	StartKey() (key Target, _ error)
	// FinishKeyStartField finishes a prior StartKey call, and starts the
	// associated field.  An error with ID verror.NotFound indicates the
	// underlying target is a struct that doesn't have the given field name.
	FinishKeyStartField(key Target) (field Target, _ error)
	// FinishField finishes a prior FinishKeyStartField call.
	FinishField(key, field Target) error
}

// StructTarget represents conversion from a struct.
type StructTarget interface {
	// StartField prepares conversion of the struct field with the given name.
	// FinishField must be called to finish the field.  An error with ID
	// verror.NotFound indicates the underlying target is a struct that doesn't
	// have the given field name.
	StartField(name string) (key, field Target, _ error)
	// FinishField finishes a prior StartField call.
	FinishField(key, field Target) error
}

// Convert converts from src to target - it is a helper for calling
// ReflectTarget(reflect.ValueOf(target)).FromReflect(reflect.ValueOf(src)).
func Convert(target, src interface{}) error {
	rtarget, err := ReflectTarget(reflect.ValueOf(target))
	if err != nil {
		return err
	}
	return FromReflect(rtarget, reflect.ValueOf(src))
}

// FromReflect converts from rv to the target, by walking through rv and calling
// the appropriate methods on the target.
func FromReflect(target Target, rv reflect.Value) error {
	rt := rv.Type()
	tt, err := vdl.TypeFromReflect(rt)
	if err != nil {
		return err
	}
	// Flatten pointers in rv and handle special cases.
	for rt.Kind() == reflect.Ptr {
		switch {
		case rv.IsNil():
			return target.FromNil(tt)
		case rt.ConvertibleTo(rtPtrToType):
			// If rv is convertible to *vdl.Type, fill from it directly.
			return target.FromTypeObject(rv.Convert(rtPtrToType).Interface().(*vdl.Type))
		case rt.ConvertibleTo(rtPtrToValue):
			// If rv is convertible to *vdl.Value, fill from it directly.
			return FromValue(target, rv.Convert(rtPtrToValue).Interface().(*vdl.Value))
		}
		rt, rv = rt.Elem(), rv.Elem()
	}
	if tt.Kind() == vdl.Optional {
		tt = tt.Elem() // flatten tt to match rt and rv
	}
	// Recursive walk through the reflect value to fill in target.
	//
	// First handle special-cases enum and oneof.  Note that vdl.TypeFromReflect
	// has already validated the String and OneOf methods, so we can call without
	// error checking.
	switch tt.Kind() {
	case vdl.Enum:
		out := rv.MethodByName("String").Call(nil)
		return target.FromEnumLabel(out[0].String(), tt)
	case vdl.OneOf:
		oneof, err := target.StartOneOf(tt)
		if err != nil {
			return err
		}
		elem := rv.MethodByName("OneOf").Call(nil)[0]
		if err := FromReflect(oneof, elem); err != nil {
			return err
		}
		return target.FinishOneOf(oneof)
	}
	// Now handle special-case bytes.
	if isRTBytes(rt) {
		return target.FromBytes(rtBytes(rv), tt)
	}
	// Handle standard kinds.
	switch rt.Kind() {
	case reflect.Interface:
		if rv.IsNil() {
			return target.FromNil(tt)
		}
		return FromReflect(target, rv.Elem())
	case reflect.Bool:
		return target.FromBool(rv.Bool(), tt)
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
		return target.FromUint(rv.Uint(), tt)
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		return target.FromInt(rv.Int(), tt)
	case reflect.Float32, reflect.Float64:
		return target.FromFloat(rv.Float(), tt)
	case reflect.Complex64, reflect.Complex128:
		return target.FromComplex(rv.Complex(), tt)
	case reflect.String:
		return target.FromString(rv.String(), tt)
	case reflect.Array, reflect.Slice:
		listTarget, err := target.StartList(tt, rv.Len())
		if err != nil {
			return err
		}
		for ix := 0; ix < rv.Len(); ix++ {
			elem, err := listTarget.StartElem(ix)
			if err != nil {
				return err
			}
			if err := FromReflect(elem, rv.Index(ix)); err != nil {
				return err
			}
			if err := listTarget.FinishElem(elem); err != nil {
				return err
			}
		}
		return target.FinishList(listTarget)
	case reflect.Map:
		if tt.Kind() == vdl.Set {
			setTarget, err := target.StartSet(tt, rv.Len())
			if err != nil {
				return err
			}
			for _, rvkey := range rv.MapKeys() {
				key, err := setTarget.StartKey()
				if err != nil {
					return err
				}
				if err := FromReflect(key, rvkey); err != nil {
					return err
				}
				switch err := setTarget.FinishKey(key); {
				case verror.Is(err, verror.NoExist):
					continue
				case err != nil:
					return err
				}
			}
			return target.FinishSet(setTarget)
		}
		mapTarget, err := target.StartMap(tt, rv.Len())
		if err != nil {
			return err
		}
		for _, rvkey := range rv.MapKeys() {
			key, err := mapTarget.StartKey()
			if err != nil {
				return err
			}
			if err := FromReflect(key, rvkey); err != nil {
				return err
			}
			field, err := mapTarget.FinishKeyStartField(key)
			switch {
			case verror.Is(err, verror.NoExist):
				continue
			case err != nil:
				return err
			}
			if err := FromReflect(field, rv.MapIndex(rvkey)); err != nil {
				return err
			}
			if err := mapTarget.FinishField(key, field); err != nil {
				return err
			}
		}
		return target.FinishMap(mapTarget)
	case reflect.Struct:
		structTarget, err := target.StartStruct(tt)
		if err != nil {
			return err
		}
		for fx := 0; fx < rt.NumField(); fx++ {
			key, field, err := structTarget.StartField(rt.Field(fx).Name)
			switch {
			case verror.Is(err, verror.NoExist):
				continue
			case err != nil:
				return err
			}
			if err := FromReflect(field, rv.Field(fx)); err != nil {
				return err
			}
			if err := structTarget.FinishField(key, field); err != nil {
				return err
			}
		}
		return target.FinishStruct(structTarget)
	default:
		return fmt.Errorf("FromReflect invalid type %v", rt)
	}
}

// FromValue converts from vv to the target, by walking through vv and calling
// the appropriate methods on the target.
func FromValue(target Target, vv *vdl.Value) error {
	tt := vv.Type()
	if tt.IsBytes() {
		return target.FromBytes(vv.Bytes(), tt)
	}
	switch vv.Kind() {
	case vdl.Any, vdl.Optional:
		if vv.IsNil() {
			return target.FromNil(tt)
		}
		return FromValue(target, vv.Elem())
	case vdl.OneOf:
		oneof, err := target.StartOneOf(tt)
		if err != nil {
			return err
		}
		if err := FromValue(oneof, vv.Elem()); err != nil {
			return err
		}
		return target.FinishOneOf(oneof)
	case vdl.Bool:
		return target.FromBool(vv.Bool(), tt)
	case vdl.Byte:
		return target.FromUint(uint64(vv.Byte()), tt)
	case vdl.Uint16, vdl.Uint32, vdl.Uint64:
		return target.FromUint(vv.Uint(), tt)
	case vdl.Int16, vdl.Int32, vdl.Int64:
		return target.FromInt(vv.Int(), tt)
	case vdl.Float32, vdl.Float64:
		return target.FromFloat(vv.Float(), tt)
	case vdl.Complex64, vdl.Complex128:
		return target.FromComplex(vv.Complex(), tt)
	case vdl.String:
		return target.FromString(vv.RawString(), tt)
	case vdl.Enum:
		return target.FromEnumLabel(vv.EnumLabel(), tt)
	case vdl.TypeObject:
		return target.FromTypeObject(vv.TypeObject())
	case vdl.Array, vdl.List:
		listTarget, err := target.StartList(tt, vv.Len())
		if err != nil {
			return err
		}
		for ix := 0; ix < vv.Len(); ix++ {
			elem, err := listTarget.StartElem(ix)
			if err != nil {
				return err
			}
			if err := FromValue(elem, vv.Index(ix)); err != nil {
				return err
			}
			if err := listTarget.FinishElem(elem); err != nil {
				return err
			}
		}
		return target.FinishList(listTarget)
	case vdl.Set:
		setTarget, err := target.StartSet(tt, vv.Len())
		if err != nil {
			return err
		}
		for _, vvkey := range vv.Keys() {
			key, err := setTarget.StartKey()
			if err != nil {
				return err
			}
			if err := FromValue(key, vvkey); err != nil {
				return err
			}
			switch err := setTarget.FinishKey(key); {
			case verror.Is(err, verror.NoExist):
				continue
			case err != nil:
				return err
			}
		}
		return target.FinishSet(setTarget)
	case vdl.Map:
		mapTarget, err := target.StartMap(tt, vv.Len())
		if err != nil {
			return err
		}
		for _, vvkey := range vv.Keys() {
			key, err := mapTarget.StartKey()
			if err != nil {
				return err
			}
			if err := FromValue(key, vvkey); err != nil {
				return err
			}
			field, err := mapTarget.FinishKeyStartField(key)
			switch {
			case verror.Is(err, verror.NoExist):
				continue
			case err != nil:
				return err
			}
			if err := FromValue(field, vv.MapIndex(vvkey)); err != nil {
				return err
			}
			if err := mapTarget.FinishField(key, field); err != nil {
				return err
			}
		}
		return target.FinishMap(mapTarget)
	case vdl.Struct:
		structTarget, err := target.StartStruct(tt)
		if err != nil {
			return err
		}
		for fx := 0; fx < tt.NumField(); fx++ {
			key, field, err := structTarget.StartField(tt.Field(fx).Name)
			switch {
			case verror.Is(err, verror.NoExist):
				continue
			case err != nil:
				return err
			}
			if err := FromValue(field, vv.Field(fx)); err != nil {
				return err
			}
			if err := structTarget.FinishField(key, field); err != nil {
				return err
			}
		}
		return target.FinishStruct(structTarget)
	default:
		panic(fmt.Errorf("FromValue unhandled %v %v", tt.Kind(), tt))
	}
}
