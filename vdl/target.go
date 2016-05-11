// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"fmt"
	"reflect"
)

// Target represents a generic conversion target; objects that implement this
// interface may be used as the target of a value conversion.  E.g.
// ReflectTarget and ValueTarget create targets based on an underlying
// reflect.Value or Value respectively.  Other implementations of this
// interface include vom encoding targets, that produce binary and JSON vom
// encodings.
type Target interface {
	// FromBool converts from the src bool to the target, where tt represents the
	// concrete type of bool.
	FromBool(src bool, tt *Type) error
	// FromUint converts from the src uint to the target, where tt represents the
	// concrete type of uint.
	FromUint(src uint64, tt *Type) error
	// FromInt converts from the src int to the target, where tt represents the
	// concrete type of int.
	FromInt(src int64, tt *Type) error
	// FromFloat converts from the src float to the target, where tt represents
	// the concrete type of float.
	FromFloat(src float64, tt *Type) error
	// FromBytes converts from the src bytes to the target, where tt represents
	// the concrete type of bytes.
	FromBytes(src []byte, tt *Type) error
	// FromString converts from the src string to the target, where tt represents
	// the concrete type of string.
	FromString(src string, tt *Type) error
	// FromEnumLabel converts from the src enum label to the target, where tt
	// represents the concrete type of enum.
	FromEnumLabel(src string, tt *Type) error
	// FromTypeObject converts from the src type to the target.
	FromTypeObject(src *Type) error
	// FromNil converts from a nil (nonexistent) value of type tt, where tt must
	// be of kind Optional or Any.
	FromNil(tt *Type) error

	// StartList prepares conversion from a list or array of type tt, with the
	// given len.  FinishList must be called to finish the list.
	StartList(tt *Type, len int) (ListTarget, error)
	// FinishList finishes a prior StartList call.
	FinishList(x ListTarget) error

	// StartSet prepares conversion from a set of type tt, with the given len.
	// FinishSet must be called to finish the set.
	StartSet(tt *Type, len int) (SetTarget, error)
	// FinishSet finishes a prior StartSet call.
	FinishSet(x SetTarget) error

	// StartMap prepares conversion from a map of type tt, with the given len.
	// FinishMap must be called to finish the map.
	StartMap(tt *Type, len int) (MapTarget, error)
	// FinishMap finishes a prior StartMap call.
	FinishMap(x MapTarget) error

	// StartFields prepares conversion from a struct or union of type tt.
	// FinishFields must be called to finish the fields.
	StartFields(tt *Type) (FieldsTarget, error)
	// FinishFields finishes a prior StartFields call.
	FinishFields(x FieldsTarget) error
}

// Targeter represents an underlying data object that provides its own target
// making and filling operations.  During conversions, types that implement
// Targeter skip the reflection-based codepath and instead make and fill the
// target directly.  This is used during vdl code generation to provide
// high-performance encoders and decoders.
type Targeter interface {
	// MakeVDLTarget returns the target corresponding to the underlying data.
	MakeVDLTarget() Target
	// FillVDLTarget fills target with the contents of the underlying data.
	FillVDLTarget(target Target, expectedType *Type) error
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
	// FinishKey finishes a prior StartKey call.  ErrFieldNoExist indicates the
	// key doesn't exist on the target.
	FinishKey(key Target) error
}

// MapTarget represents conversion from a map.
type MapTarget interface {
	// StartKey prepares conversion of the next map key.  FinishKeyStartField must
	// be called to finish the key.
	StartKey() (key Target, _ error)
	// FinishKeyStartField finishes a prior StartKey call, and starts the
	// associated field.  ErrFieldNoExist indicates the key doesn't exist on the
	// target.
	FinishKeyStartField(key Target) (field Target, _ error)
	// FinishField finishes a prior FinishKeyStartField call.
	FinishField(key, field Target) error
}

// FieldsTarget represents conversion from struct or union fields.
type FieldsTarget interface {
	// StartField prepares conversion of the field with the given name.
	// FinishField must be called to finish the field.  ErrFieldNoExist indicates
	// the field name doesn't exist on the target.
	StartField(name string) (key, field Target, _ error)
	// FinishField finishes a prior StartField call.
	FinishField(key, field Target) error
	// ZeroField writes a zero-valued field.
	// This is a no-op in the encoder for struct fields.
	ZeroField(name string) error
}

// Convert converts from src to dst - it is a helper for calling
// ReflectTarget(reflect.ValueOf(dst)).FromReflect(reflect.ValueOf(src)).
func Convert(dst, src interface{}) error {
	// TODO(bprosnitz) Flip useOldConvert=false to enable the new convert.
	const useOldConvert = true
	if useOldConvert {
		target, err := ReflectTarget(reflect.ValueOf(dst))
		if err != nil {
			return err
		}
		return FromReflect(target, reflect.ValueOf(src))
	}
	return convertPipe(dst, src)
}

// ValueOf returns the value corresponding to v.  It's a helper for calling
// ValueFromReflect, and panics on any errors.
func ValueOf(v interface{}) *Value {
	vv, err := ValueFromReflect(reflect.ValueOf(v))
	if err != nil {
		panic(err)
	}
	return vv
}

// ValueFromReflect returns the value corresponding to rv.
func ValueFromReflect(rv reflect.Value) (*Value, error) {
	const useOldConvert = true
	if useOldConvert {
		var result *Value
		target, err := ReflectTarget(reflect.ValueOf(&result))
		if err != nil {
			return nil, err
		}
		if err := FromReflect(target, rv); err != nil {
			return nil, err
		}
		return result, nil
	}
	if !rv.IsValid() {
		// TODO(bprosnitz) Is this the behavior we want?
		return ZeroValue(AnyType), nil
	}
	var result *Value
	err := convertPipe(&result, rv.Interface())
	return result, err
}

// FromReflect converts from rv to the target, by walking through rv and calling
// the appropriate methods on the target.
func FromReflect(target Target, rv reflect.Value) error {
	// Special-case to treat interface{}(nil) as any(nil).
	if !rv.IsValid() {
		return target.FromNil(AnyType)
	}

	// Flatten pointers and interfaces in rv, and handle special-cases.  We track
	// whether the final flattened value had any pointers via hasPtr, in order to
	// track optional types correctly.
	hasPtr := false
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		// Handle special-case for errors.
		// TODO(bprosnitz) Remove this case.
		if rv.Type().ConvertibleTo(rtError) && !rv.IsNil() {
			return fromError(target, rv)
		}
		// Handle marshaling from native type to wire type.
		if ni := nativeInfoFromNative(rv.Type()); ni != nil {
			newWire := reflect.New(ni.WireType)
			if err := ni.FromNative(newWire, rv); err != nil {
				return err
			}
			if hasPtr {
				return FromReflect(target, newWire)
			}
			return FromReflect(target, newWire.Elem())
		}
		hasPtr = rv.Kind() == reflect.Ptr
		switch rt := rv.Type(); {
		case rv.IsNil():
			tt, err := TypeFromReflect(rv.Type())
			if err != nil {
				return err
			}
			switch {
			case tt.Kind() == TypeObject:
				// Treat nil *Type as AnyType.
				return target.FromTypeObject(AnyType)
			case tt.Kind() == Union && rt.Kind() == reflect.Interface:
				// Treat nil Union interface as the value of the type at index 0.
				return FromValue(target, ZeroValue(tt))
			}
			return target.FromNil(tt)
		case rt.ConvertibleTo(rtPtrToType):
			// If rv is convertible to *Type, fill from it directly.
			return target.FromTypeObject(rv.Convert(rtPtrToType).Interface().(*Type))
		case rt.ConvertibleTo(rtPtrToValue):
			// If rv is convertible to *Value, fill from it directly.
			return FromValue(target, rv.Convert(rtPtrToValue).Interface().(*Value))
		case rt.Implements(rtTargeter):
			tt, err := TypeFromReflect(rt)
			if err != nil {
				return err
			}
			return rv.Interface().(Targeter).FillVDLTarget(target, tt)
		}
		rv = rv.Elem()
	}
	if reflect.PtrTo(rv.Type()).Implements(rtTargeter) {
		rvPtr := reflect.New(rv.Type())
		rvPtr.Elem().Set(rv)
		tt, err := TypeFromReflect(rv.Type())
		if err != nil {
			return err
		}
		return rvPtr.Interface().(Targeter).FillVDLTarget(target, tt)
	}

	// Handle special-case for errors.
	// TODO(bprosnitz) Remove this case.
	if rv.Type().ConvertibleTo(rtError) {
		return fromError(target, rv)
	}
	// Handle marshaling from native type to wire type.
	if ni := nativeInfoFromNative(rv.Type()); ni != nil {
		newWire := reflect.New(ni.WireType)
		if err := ni.FromNative(newWire, rv); err != nil {
			return err
		}
		if hasPtr {
			return FromReflect(target, newWire)
		}
		return FromReflect(target, newWire.Elem())
	}
	// Initialize type information.  The optionality is a bit tricky.  Both rt and
	// rv refer to the flattened value, with no pointers or interfaces.  But tt
	// refers to the optional type, iff the original value had a pointer.  This is
	// required so that each of the Target.From* methods can get full type
	// information, including optionality.
	rt := rv.Type()
	tt, err := TypeFromReflect(rv.Type())
	if err != nil {
		return err
	}
	ttFrom := tt
	if tt.CanBeOptional() && hasPtr {
		ttFrom = OptionalType(tt)
	}
	// Recursive walk through the reflect value to fill in target.
	//
	// First handle special-cases enum and union.  Note that TypeFromReflect
	// has already validated the methods, so we can call without error checking.
	switch tt.Kind() {
	case Enum:
		label := rv.Interface().(stringer).String()
		return target.FromEnumLabel(label, ttFrom)
	case Union:
		// We're guaranteed rv is the concrete field struct.
		name := rv.Interface().(namer).Name()
		fieldsTarget, err := target.StartFields(ttFrom)
		if err != nil {
			return err
		}
		key, field, err := fieldsTarget.StartField(name)
		if err != nil {
			return err // no ErrFieldNoExist special-case; union field is required
		}
		// Grab the "Value" field of the concrete field struct.
		rvFieldValue := rv.Field(0)
		if err := FromReflect(field, rvFieldValue); err != nil {
			return err
		}
		if err := fieldsTarget.FinishField(key, field); err != nil {
			return err
		}
		return target.FinishFields(fieldsTarget)
	}
	// Now handle special-case bytes.
	if isRTBytes(rt) {
		return target.FromBytes(rtBytes(rv), ttFrom)
	}
	// Handle standard kinds.
	switch rt.Kind() {
	case reflect.Bool:
		return target.FromBool(rv.Bool(), ttFrom)
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
		return target.FromUint(rv.Uint(), ttFrom)
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		return target.FromInt(rv.Int(), ttFrom)
	case reflect.Float32, reflect.Float64:
		return target.FromFloat(rv.Float(), ttFrom)
	case reflect.String:
		return target.FromString(rv.String(), ttFrom)
	case reflect.Array, reflect.Slice:
		listTarget, err := target.StartList(ttFrom, rv.Len())
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
		if tt.Kind() == Set {
			setTarget, err := target.StartSet(ttFrom, rv.Len())
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
				case err == ErrFieldNoExist:
					continue // silently drop unknown fields
				case err != nil:
					return err
				}
			}
			return target.FinishSet(setTarget)
		}
		mapTarget, err := target.StartMap(ttFrom, rv.Len())
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
			case err == ErrFieldNoExist:
				continue // silently drop unknown fields
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
		fieldsTarget, err := target.StartFields(ttFrom)
		if err != nil {
			return err
		}
		// Loop through tt fields rather than rt fields, since the VDL type tt might
		// have ignored some of the fields in rt, e.g. unexported fields.
		for fx := 0; fx < tt.NumField(); fx++ {
			name := tt.Field(fx).Name
			key, field, err := fieldsTarget.StartField(name)
			switch {
			case err == ErrFieldNoExist:
				continue // silently drop unknown fields
			case err != nil:
				return err
			}
			rvField := rv.FieldByName(name)
			if !rvField.IsValid() {
				// This case occurs if the VDL type tt has a field name that doesn't
				// exist in the Go reflect.Type rt.  This should never occur; it
				// indicates a bug in the TypeOf logic.  Panic here to give us a
				// stack trace to make it easier to debug.
				panic(fmt.Errorf("missing struct field %q, tt: %v, rt: %v", name, tt, rt))
			}
			if err := FromReflect(field, rvField); err != nil {
				return err
			}
			if err := fieldsTarget.FinishField(key, field); err != nil {
				return err
			}
		}
		return target.FinishFields(fieldsTarget)
	default:
		return fmt.Errorf("FromReflect invalid type %v", rt)
	}
}

// fromError handles all rv values that implement the error interface.
func fromError(target Target, rv reflect.Value) error {
	// Convert to the WireError representation of rv.
	ni, err := nativeInfoForError()
	if err != nil {
		return err
	}
	newWire := reflect.New(ni.WireType)
	if err := ni.FromNative(newWire, rv); err != nil {
		return err
	}
	// We know that rv is convertible to error; ensure we end up with the right
	// optionality, in case target is an interface.
	if rt := rv.Type(); rt.Kind() == reflect.Ptr && rt.Elem().ConvertibleTo(rtError) {
		return FromReflect(target, newWire)
	}
	return FromReflect(target, newWire.Elem())
}

// FromValue converts from vv to the target, by walking through vv and calling
// the appropriate methods on the target.
func FromValue(target Target, vv *Value) error {
	tt := vv.Type()
	if tt.Kind() == Any {
		if vv.IsNil() {
			return target.FromNil(tt)
		}
		// Non-nil any simply converts from the elem.
		vv = vv.Elem()
		tt = vv.Type()
	}
	if tt.Kind() == Optional {
		if vv.IsNil() {
			return target.FromNil(tt)
		}
		// Non-nil optional is special - we keep tt as the optional type, but use
		// the elem value for the actual value below.
		vv = vv.Elem()
	}
	if vv.Type().IsBytes() {
		return target.FromBytes(vv.Bytes(), tt)
	}
	switch vv.Kind() {
	case Bool:
		return target.FromBool(vv.Bool(), tt)
	case Byte, Uint16, Uint32, Uint64:
		return target.FromUint(vv.Uint(), tt)
	case Int8, Int16, Int32, Int64:
		return target.FromInt(vv.Int(), tt)
	case Float32, Float64:
		return target.FromFloat(vv.Float(), tt)
	case String:
		return target.FromString(vv.RawString(), tt)
	case Enum:
		return target.FromEnumLabel(vv.EnumLabel(), tt)
	case TypeObject:
		return target.FromTypeObject(vv.TypeObject())
	case Array, List:
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
	case Set:
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
			case err == ErrFieldNoExist:
				continue // silently drop unknown fields
			case err != nil:
				return err
			}
		}
		return target.FinishSet(setTarget)
	case Map:
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
			case err == ErrFieldNoExist:
				continue // silently drop unknown fields
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
	case Struct:
		fieldsTarget, err := target.StartFields(tt)
		if err != nil {
			return err
		}
		for fx := 0; fx < vv.Type().NumField(); fx++ {
			if vv.StructField(fx).IsZero() {
				err := fieldsTarget.ZeroField(vv.Type().Field(fx).Name)
				if err != nil && err != ErrFieldNoExist {
					return err
				}
				continue
			}
			key, field, err := fieldsTarget.StartField(vv.Type().Field(fx).Name)
			switch {
			case err == ErrFieldNoExist:
				continue // silently drop unknown fields
			case err != nil:
				return err
			}
			if err := FromValue(field, vv.StructField(fx)); err != nil {
				return err
			}
			if err := fieldsTarget.FinishField(key, field); err != nil {
				return err
			}
		}
		return target.FinishFields(fieldsTarget)
	case Union:
		fieldsTarget, err := target.StartFields(tt)
		if err != nil {
			return err
		}
		fx, vvFieldValue := vv.UnionField()
		key, field, err := fieldsTarget.StartField(vv.Type().Field(fx).Name)
		if err != nil {
			return err // no ErrFieldNoExist special-case; union field is required
		}
		if err := FromValue(field, vvFieldValue); err != nil {
			return err
		}
		if err := fieldsTarget.FinishField(key, field); err != nil {
			return err
		}
		return target.FinishFields(fieldsTarget)
	default:
		panic(fmt.Errorf("FromValue unhandled %v %v", vv.Kind(), tt))
	}
}
