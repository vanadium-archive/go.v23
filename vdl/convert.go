// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

// Naming conventions to distinguish this package from reflect:
//   tt - refers to *Type
//   vv - refers to *Value
//   rt - refers to reflect.Type
//   rv - refers to reflect.Value

import (
	"errors"
	"fmt"
	"reflect"
)

var (
	ErrFieldNoExist = errors.New("field doesn't exist")

	errTargetInvalid    = errors.New("invalid target")
	errTargetUnsettable = errors.New("unsettable target")
	errArrayIndex       = errors.New("array index out of range")
)

// convTarget represents the state and logic for value conversion.
//
// TODO(toddw): Split convTarget apart into reflectTarget and valueTarget.
type convTarget struct {
	// Conceptually this is a disjoint union between the two types of destination
	// values: *Value and reflect.Value.  Only one of the vv and rv fields is
	// ever valid.  We split into two fields and perform "if vv == nil" checks in
	// each method rather than using an interface for performance reasons.
	//
	// The tt field is always non-nil, and represents the type of the value.
	tt *Type
	vv *Value
	rv reflect.Value
}

// TODO(bprosnitz) Remove this -- it exists to get the reflect value for VOM and avoid package dependency cycles.
func (c convTarget) HackGetRv() reflect.Value {
	return c.rv
}

// ReflectTarget returns a conversion Target based on the given reflect.Value.
// Some rules depending on the type of rv:
//   o If rv is a valid *Value, it is filled in directly.
//   o Otherwise rv must be a settable value (i.e. it must be a pointer).
//   o Pointers are followed, and logically "flattened".
//   o New values are automatically created for nil pointers.
//   o Targets convertible to *Type and *Value are filled in directly.
func ReflectTarget(rv reflect.Value) (Target, error) {
	if !rv.IsValid() {
		return nil, errTargetInvalid
	}
	if vv := extractValue(rv); vv.IsValid() {
		return valueConv(vv), nil
	}
	tt, err := TypeFromReflect(rv.Type())
	if err != nil {
		return nil, err
	}
	if !rv.CanSet() && rv.Kind() == reflect.Ptr && !rv.IsNil() {
		// Dereference the pointer a single time to make rv settable.
		rv = rv.Elem()
	}
	target, err := reflectConv(rv, tt)
	if err != nil {
		return nil, err
	}
	return target, nil
}

// ValueTarget returns a conversion Target based on the given Value.
// Returns an error if the given Value isn't valid.
func ValueTarget(vv *Value) (Target, error) {
	if !vv.IsValid() {
		return nil, errTargetInvalid
	}
	return valueConv(vv), nil
}

func reflectConv(rv reflect.Value, tt *Type) (convTarget, error) {
	if !rv.IsValid() {
		return convTarget{}, errTargetInvalid
	}
	if vv := extractValue(rv); vv.IsValid() {
		return valueConv(vv), nil
	}
	if !rv.CanSet() {
		return convTarget{}, errTargetUnsettable
	}
	return convTarget{tt: tt, rv: rv}, nil
}

func valueConv(vv *Value) convTarget {
	return convTarget{tt: vv.Type(), vv: vv}
}

func extractValue(rv reflect.Value) *Value {
	for rv.Kind() == reflect.Ptr && !rv.IsNil() {
		if rv.Type().ConvertibleTo(rtPtrToValue) {
			return rv.Convert(rtPtrToValue).Interface().(*Value)
		}
		rv = rv.Elem()
	}
	return nil
}

// startConvert prepares to fill in c, converting from type ttFrom.  Returns fin
// and fill which are used by finishConvert to finish the conversion; fin
// represents the final target to assign to, and fill represents the target to
// actually fill in.
func startConvert(c convTarget, ttFrom *Type) (fin, fill convTarget, err error) {
	if ttFrom.Kind() == Any {
		return convTarget{}, convTarget{}, fmt.Errorf("can't convert from type %q - either call target.FromNil or target.From* for the element value", ttFrom)
	}
	if !Compatible(c.tt, ttFrom) {
		return convTarget{}, convTarget{}, fmt.Errorf("types %q and %q aren't compatible", c.tt, ttFrom)
	}
	fin = createFinTarget(c, ttFrom)
	fill, err = createFillTarget(fin, ttFrom)
	if err != nil {
		return convTarget{}, convTarget{}, err
	}
	return fin, fill, nil
}

// setZeroVDLValue checks whether rv is convertible to *Value, and if so,
// sets rv to the zero value of ttFrom, returning the resulting value.
func setZeroVDLValue(rv reflect.Value, ttFrom *Type) *Value {
	if rv.Type().ConvertibleTo(rtPtrToValue) {
		vv := rv.Convert(rtPtrToValue).Interface().(*Value)
		if !vv.IsValid() {
			vv = ZeroValue(ttFrom)
			rv.Set(reflect.ValueOf(vv).Convert(rv.Type()))
		}
		return vv
	}
	return nil
}

// createFinTarget returns the fin target for the conversion, flattening
// pointers and creating new non-nil values as necessary.
func createFinTarget(c convTarget, ttFrom *Type) convTarget {
	if c.vv == nil {
		// Create new pointers down to the final non-pointer value.
		for c.rv.Kind() == reflect.Ptr {
			// Handle case where rv is *Value; fill it in directly.
			if vv := setZeroVDLValue(c.rv, ttFrom); vv.IsValid() {
				if vv.Kind() == Optional {
					vv.Assign(NonNilZeroValue(vv.Type()))
					return convTarget{tt: ttFrom, vv: vv.Elem()}
				}
				return convTarget{tt: ttFrom, vv: vv}
			}
			// Set nil pointers to new pointers.
			if c.rv.IsNil() {
				c.rv.Set(reflect.New(c.rv.Type().Elem()))
			}
			// Stop at *Type to allow TypeObject values to be assigned.
			if c.rv.Type().Elem() == rtType {
				return c
			}
			c.rv = c.rv.Elem()
		}
		if c.tt.Kind() == Optional {
			c.tt = c.tt.Elem() // flatten c.tt to match c.rv
		}
	} else {
		// Create a non-nil value for optional types.
		if c.tt.Kind() == Optional {
			c.vv.Assign(NonNilZeroValue(c.tt))
			c.tt, c.vv = c.tt.Elem(), c.vv.Elem()
		}
	}
	return c
}

// createFillTarget returns the fill target for the conversion, creating new
// values of the appropriate type if necessary.  The fill target must be a
// concrete type; if fin has type interface/any, we'll create a concrete type,
// and finishConvert will assign fin from the fill target.  The type we choose
// to create is either a special-case (error or union), or based on the ttFrom
// type we're converting *from*.
//
// If fin is already a concrete type it's used directly as the fill target.
//
// TODO(toddw): The conversion logic is way too complicated, with many similar
// branches; rewrite this entire file..
func createFillTarget(fin convTarget, ttFrom *Type) (convTarget, error) {
	if fin.vv == nil {
		// Handle case where fin.rv is the native error type; create the standard
		// WireError struct to fill in.
		if ni, err := nativeInfoForError(); err == nil && fin.rv.Type() == ni.NativeType {
			return reflectConv(reflect.New(rtWireError).Elem(), ErrorType.Elem())
		}
		// Handle case where fin.rv is a native type; return the wire type as the
		// fill target, and rely on finishConvert to perform the final step of
		// converting from the wire type to the native type.
		//
		// TODO(toddw): This doesn't handle pointer native types.
		if ni := nativeInfoFromNative(fin.rv.Type()); ni != nil {
			tt, err := TypeFromReflect(ni.WireType)
			if err != nil {
				return convTarget{}, err
			}
			return reflectConv(reflect.New(ni.WireType).Elem(), tt)
		}
		if fin.rv.Kind() == reflect.Interface {
			// We're converting into an interface, so we can't just fill into the fin
			// target directly.  Return the appropriate fill value.
			switch {
			case fin.rv.Type() == rtError || ttFrom == ErrorType:
				// Create the standard WireError struct to fill in, with the pointer.
				return reflectConv(reflect.New(rtWireError).Elem(), ErrorType)
			case ttFrom == ErrorType.Elem():
				// Create the standard WireError struct to fill in, without the pointer.
				return reflectConv(reflect.New(rtWireError).Elem(), ErrorType.Elem())
			case fin.tt.Kind() == Union:
				return reflectConv(reflect.New(fin.rv.Type()).Elem(), fin.tt)
			case fin.tt.Kind() != Any:
				return convTarget{}, fmt.Errorf("internal error - cannot convert to Go type %v vdl type %v from %v", fin.rv.Type(), fin.tt, ttFrom)
			}
			// We're converting into any, and ttFrom is the type we're converting
			// *from*.  First handle the special-case *Type.
			if ttFrom.Kind() == TypeObject {
				return reflectConv(reflect.New(rtPtrToType).Elem(), TypeObjectType)
			}
			// Try to create a reflect.Type out of ttFrom, and if it exists, create a real
			// object to fill in.
			if rt := TypeToReflect(ttFrom); rt != nil {
				if rt.Kind() == reflect.Ptr && ttFrom.Kind() == Optional {
					rt = rt.Elem()
				}
				// Handle case where rt is a native type; return the wire type as the
				// fill target, and rely on finishConvert to perform the final step of
				// converting from the wire type to the native type.
				if ni := nativeInfoFromNative(rt); ni != nil {
					return reflectConv(reflect.New(ni.WireType).Elem(), ttFrom)
				}
				rv := reflect.New(rt).Elem()
				for rv.Kind() == reflect.Ptr {
					rv.Set(reflect.New(rv.Type().Elem()))
					rv = rv.Elem()
				}
				return reflectConv(rv, ttFrom)
			}
			// We don't have the type name in our registry, so create a concrete
			// *Value to fill in, based on ttFrom.
			if ttFrom.Kind() == Optional {
				return convTarget{tt: ttFrom, vv: ZeroValue(ttFrom.Elem())}, nil
			}
			return convTarget{tt: ttFrom, vv: ZeroValue(ttFrom)}, nil
		}
		// We're not converting into an interface, so we can fill into the fin
		// target directly.  Assign an appropriate zero value.
		fin.rv.Set(rvSettableZeroValue(fin.rv.Type(), ttFrom))
	} else {
		switch fin.vv.Kind() {
		case Any:
			if ttFrom.Kind() == Optional {
				return convTarget{tt: ttFrom, vv: ZeroValue(ttFrom.Elem())}, nil
			}
			return convTarget{tt: ttFrom, vv: ZeroValue(ttFrom)}, nil
		default:
			fin.vv.Assign(nil) // start with zero item
		}
	}
	return fin, nil
}

// finishConvert finishes converting a value, taking the fin and fill returned
// by startConvert.  This is necessary since interface/any values are assigned
// by value, and can't be filled in by reference.
func finishConvert(fin, fill convTarget) error {
	// The logic here mirrors the logic in createFillTarget.
	if fin.vv == nil {
		// Handle mirrored case in createFillTarget where fin.rv is the native error
		// type; fill.rv is WireError.
		if ni, err := nativeInfoForError(); err == nil && fin.rv.Type() == ni.NativeType {
			return ni.ToNative(fill.rv, fin.rv.Addr())
		}
		// Handle mirrored case in createFillTarget where fin.rv is a native type;
		// fill.rv is the wire type that has been filled in.
		if ni := nativeInfoFromNative(fin.rv.Type()); ni != nil {
			rvFill := fill.rv
			return ni.ToNative(rvFill, fin.rv.Addr())
		}
		if fin.rv.Kind() == reflect.Interface {
			// The fill value may be set to either rv or vv in startConvert above.
			var rvFill reflect.Value
			if fill.vv == nil {
				rvFill = fill.rv
				// Note: this if statement and the one in the else if block do not correctly
				// support the case of optional wiretypes.
				// TODO(bprosnitz) Fix this
				if fill.rv.Type() == rtWireError && fin.rv.Type() != rtWireError {
					// Handle case where fill.rv has type WireError; if error conversions
					// have been registered with the vdl package, we need to convert to
					// the standard error interface.
					if ni, err := nativeInfoForError(); err == nil {
						newNative := reflect.New(ni.NativeType)
						if err := ni.ToNative(fill.rv, newNative); err != nil {
							return err
						}
						rvFill = newNative.Elem()
					}
				} else if ni := nativeInfoFromWire(fill.rv.Type()); ni != nil && fin.rv.Type() != ni.WireType {
					// Handle case where fill.rv is a wire type with a native type; set
					// rvFill to a new native type and call ToNative to fill it in.
					newNative := reflect.New(ni.NativeType)
					if err := ni.ToNative(fill.rv, newNative); err != nil {
						return err
					}
					rvFill = newNative.Elem()
				}
				if fill.tt.Kind() == Optional {
					// Convert rvFill into a pointer if it should be optional.
					if rvFill.CanAddr() {
						rvFill = rvFill.Addr()
					} else {
						rvNew := reflect.New(rvFill.Type())
						rvNew.Elem().Set(rvFill)
						rvFill = rvNew
					}
				}
			} else {
				switch fill.tt.Kind() {
				case Optional:
					rvFill = reflect.ValueOf(OptionalValue(fill.vv))
				default:
					rvFill = reflect.ValueOf(fill.vv)
				}
			}
			if to, from := fin.rv.Type(), rvFill.Type(); !from.AssignableTo(to) {
				return fmt.Errorf("%v not assignable from %v", to, from)
			}
			fin.rv.Set(rvFill)
		}
	} else {
		switch fin.vv.Kind() {
		case Any:
			if fill.tt.Kind() == Optional {
				fin.vv.Assign(OptionalValue(fill.vv))
			} else {
				fin.vv.Assign(fill.vv)
			}
		}
	}
	return nil
}

// rvSettableZeroValue returns a settable zero value corresponding to rt / tt.
// This isn't trivial since VDL and Go define zero values slightly differently.
// In particular in VDL:
//    TypeObject: AnyType
//    Union:      zero value of the type at index 0
// These are translated into Go as follows, with their standard Go zero values:
//    *Type: nil
//    interface: nil
// Thus we must special-case values of these types.
//
// TODO(toddw): This logic is recursive and complicated because our convTarget
// methods don't take the convTarget as a pointer receiver, so we can't mutate
// the target as we decode.  When we split convTarget into separate valueTarget
// and reflectTarget objects, we should also take the target as a pointer
// receiver.  That'll simplify this logic.
func rvSettableZeroValue(rt reflect.Type, tt *Type) reflect.Value {
	rv := reflect.New(rt).Elem()
	// Easy fastpath; if the type doesn't contain inline typeobject or union, the
	// regular Go zero value is good enough.
	if !tt.ContainsKind(WalkInline, kkTypeObjectOrUnion...) {
		return rv
	}
	// Handle typeobject, which has the zero value of AnyType.
	if rtPtrToType.ConvertibleTo(rt) {
		return reflect.ValueOf(AnyType).Convert(rt)
	}
	// Handle composite types with inline subtypes.
	switch {
	case tt.Kind() == Union:
		if nativeInfoFromNative(rt) != nil {
			return rv
		}
		if rt.Kind() == reflect.Struct {
			// Union struct, which represents a single field.
			rv.Field(0).Set(rvSettableZeroValue(rt.Field(0).Type, tt.Field(0).Type))
			return rv
		}
		// Union interface, which represents one of the fields.  Initialize with the
		// zero value of the type at index 0.
		ri, _, err := deriveReflectInfo(rt)
		if err != nil {
			panic(fmt.Errorf("vdl: invalid union type rt: %v tt: %v err: %v", rt, tt, err))
		}
		rv.Set(rvSettableZeroValue(ri.UnionFields[0].RepType, tt.Field(0).Type))
		return rv
	case rt.Kind() == reflect.Array:
		for ix := 0; ix < rt.Len(); ix++ {
			rv.Index(ix).Set(rvSettableZeroValue(rt.Elem(), tt.Elem()))
		}
		return rv
	case rt.Kind() == reflect.Struct:
		for ix := 0; ix < rt.NumField(); ix++ {
			rtField := rt.Field(ix)
			ttField, index := tt.FieldByName(rtField.Name)
			if index < 0 {
				// Ignore fields that aren't described in tt; e.g. unexported fields.
				continue
			}
			rv.Field(ix).Set(rvSettableZeroValue(rtField.Type, ttField.Type))
		}
		return rv
	}
	panic(fmt.Errorf("vdl: rvSettableZeroValue unhandled rt: %v tt: %v", rt, tt))
}

func removeOptional(tt *Type) *Type {
	if tt.Kind() == Optional {
		tt = tt.Elem()
	}
	return tt
}

// makeDirectTarget returns the target representing the underlying value, if the
// underlying value supports direct target access.
func (c convTarget) makeDirectTarget() Target {
	if c.vv == nil {
		rv := c.rv
		if rv.Type().Implements(rtTargeter) {
			if rv.Kind() == reflect.Ptr && rv.IsNil() {
				rv.Set(reflect.New(rv.Type().Elem()))
			}
			return rv.Interface().(Targeter).MakeVDLTarget()
		}
		if rv.CanAddr() {
			rv = rv.Addr()
			if rv.Type().Implements(rtTargeter) {
				return rv.Interface().(Targeter).MakeVDLTarget()
			}
		}
	}
	return nil
}

func (c convTarget) FromNil(tt *Type) error {
	if !tt.CanBeNil() || !c.tt.CanBeNil() {
		return fmt.Errorf("invalid conversion from %v(nil) to %v", tt, c.tt)
	}
	if c.vv == nil {
		// The strategy is to create new pointers down to either a single pointer,
		// or the final interface, and set that to nil.  We create all the pointers
		// to be consistent with our behavior in the non-nil case, where we
		// similarly create all the pointers.  It also makes the tests simpler.
		rv := c.rv
		for rv.Kind() == reflect.Ptr {
			if vv := setZeroVDLValue(rv, tt); vv.IsValid() {
				return nil
			}
			if k := rv.Type().Elem().Kind(); k != reflect.Ptr && k != reflect.Interface {
				break
			}
			// Next elem is a pointer or interface, keep looping.
			if rv.IsNil() {
				rv.Set(reflect.New(rv.Type().Elem()))
			}
			rv = rv.Elem()
		}
		// Now rv.Type is either a single pointer or an interface.  If it is an
		// interface, check to see whether we can create a Go object from tt.
		rt := rv.Type()
		if rv.Kind() == reflect.Interface {
			if rtFromTT := TypeToReflect(tt); rtFromTT != nil {
				rt = rtFromTT
			}
		}
		// Set the zero value of the pointer or interface, which will give us nil of
		// the correct type.
		rv.Set(reflect.Zero(rt))
		return nil
	} else {
		vvNil := ZeroValue(tt)
		if to, from := c.vv.Type(), vvNil; !to.AssignableFrom(from) {
			return fmt.Errorf("%v not assignable from %v", to, from)
		}
		c.vv.Assign(vvNil)
		return nil
	}
}

// FromBool implements the Target interface method.
func (c convTarget) FromBool(src bool, tt *Type) error {
	if target := c.makeDirectTarget(); target != nil {
		return target.FromBool(src, tt)
	}
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromBool(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

// FromUint implements the Target interface method.
func (c convTarget) FromUint(src uint64, tt *Type) error {
	if target := c.makeDirectTarget(); target != nil {
		return target.FromUint(src, tt)
	}
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromUint(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

// FromInt implements the Target interface method.
func (c convTarget) FromInt(src int64, tt *Type) error {
	if target := c.makeDirectTarget(); target != nil {
		return target.FromInt(src, tt)
	}
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromInt(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

// FromFloat implements the Target interface method.
func (c convTarget) FromFloat(src float64, tt *Type) error {
	if target := c.makeDirectTarget(); target != nil {
		return target.FromFloat(src, tt)
	}
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromFloat(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

// FromBytes implements the Target interface method.
func (c convTarget) FromBytes(src []byte, tt *Type) error {
	if target := c.makeDirectTarget(); target != nil {
		return target.FromBytes(src, tt)
	}
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromBytes(src, tt); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

// FromString implements the Target interface method.
func (c convTarget) FromString(src string, tt *Type) error {
	if target := c.makeDirectTarget(); target != nil {
		return target.FromString(src, tt)
	}
	fin, fill, err := startConvert(c, tt)
	if err != nil {
		return err
	}
	if err := fill.fromString(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

// FromEnumLabel implements the Target interface method.
func (c convTarget) FromEnumLabel(src string, tt *Type) error {
	if target := c.makeDirectTarget(); target != nil {
		return target.FromEnumLabel(src, tt)
	}
	return c.FromString(src, tt)
}

// FromTypeObject implements the Target interface method.
func (c convTarget) FromTypeObject(src *Type) error {
	if target := c.makeDirectTarget(); target != nil {
		return target.FromTypeObject(src)
	}
	fin, fill, err := startConvert(c, TypeObjectType)
	if err != nil {
		return err
	}
	if err := fill.fromTypeObject(src); err != nil {
		return err
	}
	return finishConvert(fin, fill)
}

func (c convTarget) fromBool(src bool) error {
	if c.vv == nil {
		if c.rv.Kind() == reflect.Bool {
			c.rv.SetBool(src)
			return nil
		}
	} else {
		if c.vv.Kind() == Bool {
			c.vv.AssignBool(src)
			return nil
		}
	}
	return fmt.Errorf("invalid conversion from bool to %v", c.tt)
}

func (c convTarget) fromUint(src uint64) error {
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
			if !overflowUint(src, bitlenR(kind)) {
				c.rv.SetUint(src)
				return nil
			}
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			if isrc, ok := convertUintToInt(src, bitlenR(kind)); ok {
				c.rv.SetInt(isrc)
				return nil
			}
		case reflect.Float32, reflect.Float64:
			if fsrc, ok := convertUintToFloat(src, bitlenR(kind)); ok {
				c.rv.SetFloat(fsrc)
				return nil
			}
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case Byte, Uint16, Uint32, Uint64:
			if !overflowUint(src, bitlenV(kind)) {
				c.vv.AssignUint(src)
				return nil
			}
		case Int8, Int16, Int32, Int64:
			if isrc, ok := convertUintToInt(src, bitlenV(kind)); ok {
				c.vv.AssignInt(isrc)
				return nil
			}
		case Float32, Float64:
			if fsrc, ok := convertUintToFloat(src, bitlenV(kind)); ok {
				c.vv.AssignFloat(fsrc)
				return nil
			}
		}
	}
	return fmt.Errorf("invalid conversion from uint(%d) to %v", src, c.tt)
}

func (c convTarget) fromInt(src int64) error {
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
			if usrc, ok := convertIntToUint(src, bitlenR(kind)); ok {
				c.rv.SetUint(usrc)
				return nil
			}
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			if !overflowInt(src, bitlenR(kind)) {
				c.rv.SetInt(src)
				return nil
			}
		case reflect.Float32, reflect.Float64:
			if fsrc, ok := convertIntToFloat(src, bitlenR(kind)); ok {
				c.rv.SetFloat(fsrc)
				return nil
			}
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case Byte, Uint16, Uint32, Uint64:
			if usrc, ok := convertIntToUint(src, bitlenV(kind)); ok {
				c.vv.AssignUint(usrc)
				return nil
			}
		case Int8, Int16, Int32, Int64:
			if !overflowInt(src, bitlenV(kind)) {
				c.vv.AssignInt(src)
				return nil
			}
		case Float32, Float64:
			if fsrc, ok := convertIntToFloat(src, bitlenV(kind)); ok {
				c.vv.AssignFloat(fsrc)
				return nil
			}
		}
	}
	return fmt.Errorf("invalid conversion from int(%d) to %v", src, c.tt)
}

func (c convTarget) fromFloat(src float64) error {
	if c.vv == nil {
		switch kind := c.rv.Kind(); kind {
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint, reflect.Uintptr:
			if usrc, ok := convertFloatToUint(src, bitlenR(kind)); ok {
				c.rv.SetUint(usrc)
				return nil
			}
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			if isrc, ok := convertFloatToInt(src, bitlenR(kind)); ok {
				c.rv.SetInt(isrc)
				return nil
			}
		case reflect.Float32, reflect.Float64:
			c.rv.SetFloat(convertFloatToFloat(src, bitlenR(kind)))
			return nil
		}
	} else {
		switch kind := c.vv.Kind(); kind {
		case Byte, Uint16, Uint32, Uint64:
			if usrc, ok := convertFloatToUint(src, bitlenV(kind)); ok {
				c.vv.AssignUint(usrc)
				return nil
			}
		case Int8, Int16, Int32, Int64:
			if isrc, ok := convertFloatToInt(src, bitlenV(kind)); ok {
				c.vv.AssignInt(isrc)
				return nil
			}
		case Float32, Float64:
			c.vv.AssignFloat(convertFloatToFloat(src, bitlenV(kind)))
			return nil
		}
	}
	return fmt.Errorf("invalid conversion from float(%g) to %v", src, c.tt)
}

func (c convTarget) fromBytes(src []byte, tt *Type) error {
	if c.tt.IsBytes() {
		return c.fromBytesToBytes(src)
	}
	elemType := tt.Elem()
	for i, b := range src {
		elem, err := c.startElem(i)
		if err != nil {
			return err
		}
		if err := elem.FromUint(uint64(b), elemType); err != nil {
			return err
		}
		if err := c.finishElem(elem); err != nil {
			return err
		}
	}
	return nil
}

func (c convTarget) fromBytesToBytes(src []byte) error {
	if c.vv == nil {
		switch {
		case c.rv.Kind() == reflect.Array:
			if c.rv.Type().Elem() == rtByte && c.rv.Len() == len(src) {
				reflect.Copy(c.rv, reflect.ValueOf(src))
				return nil
			}
		case c.rv.Kind() == reflect.Slice:
			if c.rv.Type().Elem() == rtByte {
				if len(src) == 0 {
					c.rv.SetBytes(nil)
				} else {
					cp := make([]byte, len(src))
					copy(cp, src)
					c.rv.SetBytes(cp)
				}
				return nil
			}
		}
	} else {
		switch c.vv.Kind() {
		case Array:
			if c.vv.Type().Len() == len(src) {
				c.vv.AssignBytes(src)
				return nil
			}
		case List:
			c.vv.AssignBytes(src)
			return nil
		}
	}
	return fmt.Errorf("invalid conversion from bytes to %v", c.tt)
}

// settable exists to avoid a call to reflect.Call() to invoke Set()
// which results in an allocation
type settable interface {
	Set(string) error
}

func (c convTarget) fromString(src string) error {
	if c.vv == nil {
		tt := removeOptional(c.tt)
		switch {
		case tt.Kind() == Enum:
			// Handle special-case enum first, by calling the Assign method.  Note
			// that TypeFromReflect has already validated the Assign method, so we
			// can call without error checking.
			if c.rv.CanAddr() {
				err := c.rv.Addr().Interface().(settable).Set(src)
				if err != nil {
					return err
				}
				return nil
			}
		case c.rv.Kind() == reflect.String:
			c.rv.SetString(src) // TODO(toddw): check utf8
			return nil
		}
	} else {
		switch c.vv.Kind() {
		case String:
			c.vv.AssignString(src) // TODO(toddw): check utf8
			return nil
		case Enum:
			if index := c.vv.Type().EnumIndex(src); index >= 0 {
				c.vv.AssignEnumIndex(index)
				return nil
			}
		}
	}
	return fmt.Errorf("invalid conversion from string or enum to %v", c.tt)
}

func (c convTarget) fromTypeObject(src *Type) error {
	if c.vv == nil {
		if rtPtrToType.ConvertibleTo(c.rv.Type()) {
			c.rv.Set(reflect.ValueOf(src).Convert(c.rv.Type()))
			return nil
		}
	} else {
		if c.vv.Kind() == TypeObject {
			c.vv.AssignTypeObject(src)
			return nil
		}
	}
	return fmt.Errorf("invalid conversion from typeobject to %v", c.tt)
}

// wrappedListTarget is used to implement FinishList for direct targets.
type wrappedListTarget struct {
	ListTarget
	Target Target
}

// StartList implements the Target interface method.
func (c convTarget) StartList(tt *Type, len int) (ListTarget, error) {
	if target := c.makeDirectTarget(); target != nil {
		listTarget, err := target.StartList(tt, len)
		if err != nil {
			return nil, err
		}
		return wrappedListTarget{listTarget, target}, nil
	}
	// TODO(bprosnitz) Re-think allocation strategy and possibly use len (currently unused).
	fin, fill, err := startConvert(c, tt)
	return compConvTarget{fin, fill}, err
}

// FinishList implements the Target interface method.
func (c convTarget) FinishList(x ListTarget) error {
	if wrapped, ok := x.(wrappedListTarget); ok {
		return wrapped.Target.FinishList(wrapped.ListTarget)
	}
	cc := x.(compConvTarget)
	return finishConvert(cc.fin, cc.fill)
}

// wrappedSetTarget is used to implement FinishSet for direct targets.
type wrappedSetTarget struct {
	SetTarget
	Target Target
}

// StartSet implements the Target interface method.
func (c convTarget) StartSet(tt *Type, len int) (SetTarget, error) {
	if target := c.makeDirectTarget(); target != nil {
		setTarget, err := target.StartSet(tt, len)
		if err != nil {
			return nil, err
		}
		return wrappedSetTarget{setTarget, target}, nil
	}
	fin, fill, err := startConvert(c, tt)
	return compConvTarget{fin, fill}, err
}

// FinishSet implements the Target interface method.
func (c convTarget) FinishSet(x SetTarget) error {
	if wrapped, ok := x.(wrappedSetTarget); ok {
		return wrapped.Target.FinishSet(wrapped.SetTarget)
	}
	cc := x.(compConvTarget)
	return finishConvert(cc.fin, cc.fill)
}

// wrappedMapTarget is used to implement FinishMap for direct targets.
type wrappedMapTarget struct {
	MapTarget
	Target Target
}

// StartMap implements the Target interface method.
func (c convTarget) StartMap(tt *Type, len int) (MapTarget, error) {
	if target := c.makeDirectTarget(); target != nil {
		mapTarget, err := target.StartMap(tt, len)
		if err != nil {
			return nil, err
		}
		return wrappedMapTarget{mapTarget, target}, nil
	}
	fin, fill, err := startConvert(c, tt)
	return compConvTarget{fin, fill}, err
}

// FinishMap implements the Target interface method.
func (c convTarget) FinishMap(x MapTarget) error {
	if wrapped, ok := x.(wrappedMapTarget); ok {
		return wrapped.Target.FinishMap(wrapped.MapTarget)
	}
	cc := x.(compConvTarget)
	return finishConvert(cc.fin, cc.fill)
}

// wrappedFieldsTarget is used to implement FinishFields for direct targets.
type wrappedFieldsTarget struct {
	FieldsTarget
	Target Target
}

// StartFields implements the Target interface method.
func (c convTarget) StartFields(tt *Type) (FieldsTarget, error) {
	if target := c.makeDirectTarget(); target != nil {
		fieldsTarget, err := target.StartFields(tt)
		if err != nil {
			return nil, err
		}
		return wrappedFieldsTarget{fieldsTarget, target}, nil
	}
	if c.vv == nil && c.rv.Kind() == reflect.Interface {
		// TODO(bprosnitz) Union targets are not used for explicit union field structs
		// It is possible to generate and use targets in this case. Consider
		// if it is useful.
		ri, _, err := deriveReflectInfo(c.rv.Type())
		if err != nil {
			return nil, err
		}
		if ri.UnionTargetFactory != nil {
			if ni := nativeInfoFromWire(c.rv.Type()); ni == nil {
				// Wire types are not supported by the generated union target.
				rv := c.rv.Addr()
				target, err := ri.UnionTargetFactory.VDLMakeUnionTarget(rv.Interface())
				if err != nil {
					return nil, err
				}
				fieldsTarget, err := target.StartFields(tt)
				if err != nil {
					return nil, err
				}
				return wrappedFieldsTarget{fieldsTarget, target}, nil
			}
		}
	}
	fin, fill, err := startConvert(c, tt)
	return compConvTarget{fin, fill}, err
}

// FinishFields implements the Target interface method.
func (c convTarget) FinishFields(x FieldsTarget) error {
	if wrapped, ok := x.(wrappedFieldsTarget); ok {
		return wrapped.Target.FinishFields(wrapped.FieldsTarget)
	}
	cc := x.(compConvTarget)
	return finishConvert(cc.fin, cc.fill)
}

// compConvTarget represents the state and logic for composite value conversion.
type compConvTarget struct {
	fin, fill convTarget // fields returned by startConvert.
}

// TODO(bprosnitz) Remove this -- it exists to get the reflect value for VOM and avoid package dependency cycles.
func (c compConvTarget) HackGetRv() reflect.Value {
	return c.fin.rv
}

// StartElem implements the ListTarget interface method.
func (cc compConvTarget) StartElem(index int) (elem Target, _ error) {
	return cc.fill.startElem(index)
}

// FinishElem implements the ListTarget interface method.
func (cc compConvTarget) FinishElem(elem Target) error {
	return cc.fill.finishElem(elem.(convTarget))
}

// StartKey implements the SetTarget and MapTarget interface method.
func (cc compConvTarget) StartKey() (key Target, _ error) {
	return cc.fill.startKey()
}

// FinishKeyStartField implements the MapTarget interface method.
func (cc compConvTarget) FinishKeyStartField(key Target) (field Target, _ error) {
	return cc.fill.finishKeyStartField(key.(convTarget))
}

// FinishField implements the MapTarget and FieldsTarget interface method.
func (cc compConvTarget) FinishField(key, field Target) error {
	return cc.fill.finishField(key.(convTarget), field.(convTarget))
}

// StartField implements the FieldsTarget interface method.
func (cc compConvTarget) StartField(name string) (key, field Target, _ error) {
	var err error
	if key, err = cc.StartKey(); err != nil {
		return nil, nil, err
	}
	if err = key.FromString(name, StringType); err != nil {
		return nil, nil, err
	}
	if field, err = cc.FinishKeyStartField(key); err != nil {
		return nil, nil, err
	}
	return
}

// ZeroField implements the FieldsTarget interface method.
func (cc compConvTarget) ZeroField(name string) error {
	key, field, err := cc.StartField(name)
	if err != nil {
		return err
	}
	tt := cc.fill.tt
	if tt.Kind() == Optional {
		tt = tt.Elem()
	}
	var ztt *Type
	switch tt.Kind() {
	case Struct, Union:
		fld, index := tt.FieldByName(name)
		if index < 0 {
			return ErrFieldNoExist
		}
		ztt = fld.Type
	case Map:
		ztt = tt.Elem()
	case Set:
		ztt = TypeOf(struct{}{})
	}
	if err := FromValue(field, ZeroValue(ztt)); err != nil {
		return err
	}
	return cc.FinishField(key, field)
}

// FinishKey implements the SetTarget interface method.
func (cc compConvTarget) FinishKey(key Target) error {
	field, err := cc.FinishKeyStartField(key)
	if err != nil {
		return err
	}
	return cc.FinishField(key, field)
}

func (c convTarget) startElem(index int) (convTarget, error) {
	if c.vv == nil {
		tt := removeOptional(c.tt)
		switch c.rv.Kind() {
		case reflect.Array:
			if index >= c.rv.Len() {
				return convTarget{}, errArrayIndex
			}
			return reflectConv(c.rv.Index(index), tt.Elem())
		case reflect.Slice:
			newlen := index + 1
			if newlen < c.rv.Len() {
				newlen = c.rv.Len()
			}
			if newlen > c.rv.Cap() {
				rvNew := reflect.MakeSlice(c.rv.Type(), newlen, newlen*2)
				reflect.Copy(rvNew, c.rv)
				c.rv.Set(rvNew)
			} else {
				c.rv.SetLen(newlen)
			}
			return reflectConv(c.rv.Index(index), tt.Elem())
		}
	} else {
		switch c.vv.Kind() {
		case Array:
			if index >= c.vv.Len() {
				return convTarget{}, errArrayIndex
			}
			return valueConv(c.vv.Index(index)), nil
		case List:
			newlen := index + 1
			if newlen < c.vv.Len() {
				newlen = c.vv.Len()
			}
			c.vv.AssignLen(newlen)
			return valueConv(c.vv.Index(index)), nil
		}
	}
	return convTarget{}, fmt.Errorf("type %v doesn't support StartElem", c.tt)
}

func (c convTarget) finishElem(elem convTarget) error {
	if c.vv == nil {
		switch c.rv.Kind() {
		case reflect.Array, reflect.Slice:
			return nil
		}
	} else {
		switch c.vv.Kind() {
		case Array, List:
			return nil
		}
	}
	return fmt.Errorf("type %v doesn't support FinishElem", c.tt)
}

func (c convTarget) startKey() (convTarget, error) {
	if c.vv == nil {
		tt := removeOptional(c.tt)
		switch c.rv.Kind() {
		case reflect.Map:
			return reflectConv(rvSettableZeroValue(c.rv.Type().Key(), tt.Key()), tt.Key())
		case reflect.Struct, reflect.Interface:
			// The key for struct and union is the field name, which is a string.
			return reflectConv(reflect.New(rtString).Elem(), StringType)
		}
	} else {
		switch c.vv.Kind() {
		case Set, Map:
			return valueConv(ZeroValue(c.vv.Type().Key())), nil
		case Struct, Union:
			// The key for struct and union is the field name, which is a string.
			return valueConv(ZeroValue(StringType)), nil
		}
	}
	return convTarget{}, fmt.Errorf("type %v doesn't support StartKey", c.tt)
}

func (c convTarget) finishKeyStartField(key convTarget) (convTarget, error) {
	// There are various special-cases regarding bool values below.  These are to
	// handle different representations of sets; the following types are all
	// convertible to each other:
	//   set[string], map[string]bool, struct{X, Y, Z bool}
	//
	// To deal with these cases in a uniform manner, we return a bool field for
	// set[string], and initialize bool fields to true.
	if c.vv == nil {
		tt := removeOptional(c.tt)
		switch c.rv.Kind() {
		case reflect.Map:
			var ttField *Type
			var rvField reflect.Value
			switch rtField := c.rv.Type().Elem(); {
			case tt.Kind() == Set:
				// The map actually represents a set
				ttField = BoolType
				rvField = reflect.New(rtBool).Elem()
				rvField.SetBool(true)
			case rtField.Kind() == reflect.Bool:
				ttField = tt.Elem()
				rvField = reflect.New(rtField).Elem()
				rvField.SetBool(true)
			default:
				// TODO(toddw): This doesn't work correctly for map[_]any.
				ttField = tt.Elem()
				rvField = rvSettableZeroValue(rtField, ttField)
			}
			return reflectConv(rvField, ttField)
		case reflect.Struct:
			fieldName := key.rv.String()
			if tt.Kind() == Union {
				// Special-case: the fill target is a union concrete field struct.  This
				// means that we should only return a field if the field name matches.
				existingName := c.rv.Interface().(namer).Name()
				if existingName != fieldName {
					return convTarget{}, ErrFieldNoExist
				}
				ttField, _ := tt.FieldByName(fieldName)
				return reflectConv(c.rv.FieldByName("Value"), ttField.Type)
			}
			// TODO(toddw): How should we handle anonymous (aka embedded) fields?
			// Note that unexported embedded fields may themselves have exported
			// fields.  See https://github.com/golang/go/issues/12367
			rvField := c.rv.FieldByName(fieldName)
			ttField, index := tt.FieldByName(fieldName)
			if !rvField.IsValid() || index < 0 {
				// TODO(toddw): Add a way to track extra and missing fields.
				return convTarget{}, ErrFieldNoExist
			}
			if rvField.Kind() == reflect.Bool {
				rvField.SetBool(true)
			}
			return reflectConv(rvField, ttField.Type)
		case reflect.Interface:
			if tt.Kind() == Union {
				ri, _, err := deriveReflectInfo(c.rv.Type())
				if err != nil {
					return convTarget{}, err
				}
				ttField, index := tt.FieldByName(key.rv.String())
				fld, found := ri.UnionFields[index].RepType.FieldByName("Value")
				if !found {
					return convTarget{}, fmt.Errorf("concrete union type %q missing required Value field", ri.UnionFields[index].RepType)
				}
				rvValue := reflect.New(fld.Type).Elem()
				return reflectConv(rvValue, ttField.Type)
			}
		}
	} else {
		switch c.vv.Kind() {
		case Set:
			return valueConv(BoolValue(nil, true)), nil
		case Map:
			vvField := ZeroValue(c.vv.Type().Elem())
			if vvField.Kind() == Bool {
				vvField.AssignBool(true)
			}
			return valueConv(vvField), nil
		case Struct:
			_, index := c.vv.Type().FieldByName(key.vv.RawString())
			if index < 0 {
				// TODO(toddw): Add a way to track extra and missing fields.
				return convTarget{}, ErrFieldNoExist
			}
			vvField := c.vv.StructField(index)
			if vvField.Kind() == Bool {
				vvField.AssignBool(true)
			}
			return valueConv(vvField), nil
		case Union:
			f, index := c.vv.Type().FieldByName(key.vv.RawString())
			if index < 0 {
				return convTarget{}, ErrFieldNoExist
			}
			vvField := ZeroValue(f.Type)
			return valueConv(vvField), nil
		}
	}
	return convTarget{}, fmt.Errorf("type %v doesn't support FinishKeyStartField", c.tt)
}

func (c convTarget) finishField(key, field convTarget) error {
	// The special-case handling of bool fields matches the special-cases in
	// FinishKeyStartField.
	if c.vv == nil {
		tt := removeOptional(c.tt)
		switch c.rv.Kind() {
		case reflect.Map:
			rvField := field.rv
			if tt.Kind() == Set {
				// The map actually represents a set
				if !field.rv.Bool() {
					return fmt.Errorf("%v can only be converted from true fields", tt)
				}
				rvField = reflect.Zero(c.rv.Type().Elem())
			}
			if c.rv.IsNil() {
				c.rv.Set(reflect.MakeMap(c.rv.Type()))
			}
			c.rv.SetMapIndex(key.rv, rvField)
			return nil
		case reflect.Struct:
			return nil
		case reflect.Interface:
			if tt.Kind() == Union {
				ri, _, err := deriveReflectInfo(c.rv.Type())
				if err != nil {
					return err
				}
				_, index := c.tt.FieldByName(key.rv.String())
				rvField := reflect.New(ri.UnionFields[index].RepType).Elem()
				rvField.FieldByName("Value").Set(field.rv)
				c.rv.Set(rvField)
				return nil
			}
		}
	} else {
		switch c.vv.Kind() {
		case Set:
			if !field.vv.Bool() {
				return fmt.Errorf("%v can only be converted from true fields", c.vv.Type())
			}
			c.vv.AssignSetKey(key.vv)
			return nil
		case Map:
			c.vv.AssignMapIndex(key.vv, field.vv)
			return nil
		case Struct:
			return nil
		case Union:
			_, index := c.vv.Type().FieldByName(key.vv.RawString())
			c.vv.AssignField(index, field.vv)
			return nil
		}
	}
	return fmt.Errorf("type %v doesn't support FinishField", c.tt)
}
