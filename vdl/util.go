// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"fmt"
	"reflect"
	"unsafe"
)

var (
	bitlenReflect = [...]uintptr{
		reflect.Uint8:   8,
		reflect.Uint16:  16,
		reflect.Uint32:  32,
		reflect.Uint64:  64,
		reflect.Uint:    8 * unsafe.Sizeof(uint(0)),
		reflect.Uintptr: 8 * unsafe.Sizeof(uintptr(0)),
		reflect.Int8:    8,
		reflect.Int16:   16,
		reflect.Int32:   32,
		reflect.Int64:   64,
		reflect.Int:     8 * unsafe.Sizeof(int(0)),
		reflect.Float32: 32,
		reflect.Float64: 64,
	}

	bitlenVDL = [...]uintptr{
		Byte:    8,
		Uint16:  16,
		Uint32:  32,
		Uint64:  64,
		Int8:    8,
		Int16:   16,
		Int32:   32,
		Int64:   64,
		Float32: 32,
		Float64: 64,
	}
)

// bitlen{R,V} enforce static type safety on kind.
func bitlenR(kind reflect.Kind) uintptr { return bitlenReflect[kind] }
func bitlenV(kind Kind) uintptr         { return bitlenVDL[kind] }

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

// IsZeroer is the interface that wraps the VDLIsZero method.
//
// VDLIsZero returns true iff the receiver that implements this method is the
// VDL zero value.
type IsZeroer interface {
	VDLIsZero() (bool, error)
}

type stringer interface {
	String() string
}
type namer interface {
	Name() string
}
type indexer interface {
	Index() int
}

var (
	rvAnyType            = reflect.ValueOf(AnyType)
	kkTypeObjectOrUnion  = []Kind{TypeObject, Union}
	kkZeroValueNotUnique = []Kind{Any, TypeObject, Union, List, Set, Map}
)

// rvZeroValue returns the zero value of rt, using the vdl zero rules.
//
// VDL and Go define zero values differently.  According to VDL:
//    TypeObject: AnyType
//    Union:      zero value of the type at index 0
// But according to go:
//    TypeObject: (*Type)(nil)
//    Union:      UnionInterface(nil)
//
// Thus we must special-case values of these types, or any types that contain
// these types inline.  E.g. if an array, struct, or union contains one of these
// types, it will show up in the zero value, and needs special-casing.
//
// TODO(toddw): Cache the generated zero values, if it's too expensive to
// generate them each time.
func rvZeroValue(rt reflect.Type, tt *Type) (reflect.Value, error) {
	// Easy fastpath; if the type doesn't contain inline typeobject or union, the
	// regular Go zero value is sufficient.
	if !tt.ContainsKind(WalkInline, kkTypeObjectOrUnion...) {
		return reflect.Zero(rt), nil
	}
	// Handle typeobject, which has the AnyType zero value.
	if rt == rtPtrToType {
		return rvAnyType, nil
	}
	// Handle native types by returning the native value filled in with a zero
	// value of the wire type.
	if ni := nativeInfoFromNative(rt); ni != nil {
		rvWire := reflect.New(ni.WireType).Elem()
		ttWire, err := TypeFromReflect(ni.WireType)
		if err != nil {
			return reflect.Value{}, err
		}
		switch zero, err := rvZeroValue(ni.WireType, ttWire); {
		case err != nil:
			return reflect.Value{}, err
		default:
			rvWire.Set(zero)
		}
		rvNativePtr := reflect.New(rt)
		if err := ni.ToNative(rvWire, rvNativePtr); err != nil {
			return reflect.Value{}, err
		}
		return rvNativePtr.Elem(), nil
	}
	// Handle composite types with inline subtypes.
	rv := reflect.New(rt).Elem()
	switch {
	case tt.Kind() == Union:
		// Set the union interface with the zero value of the type at index 0.
		ri, _, err := deriveReflectInfo(rt)
		if err != nil {
			return reflect.Value{}, err
		}
		switch zero, err := rvZeroValue(ri.UnionFields[0].RepType, tt.Field(0).Type); {
		case err != nil:
			return reflect.Value{}, err
		default:
			rv.Set(zero)
		}
	case rt.Kind() == reflect.Array:
		for ix := 0; ix < rt.Len(); ix++ {
			switch zero, err := rvZeroValue(rt.Elem(), tt.Elem()); {
			case err != nil:
				return reflect.Value{}, err
			default:
				rv.Index(ix).Set(zero)
			}
		}
	case rt.Kind() == reflect.Struct:
		for ix := 0; ix < tt.NumField(); ix++ {
			field := tt.Field(ix)
			rvField := rv.FieldByName(field.Name)
			switch zero, err := rvZeroValue(rvField.Type(), field.Type); {
			case err != nil:
				return reflect.Value{}, err
			default:
				rvField.Set(zero)
			}
		}
	default:
		return reflect.Value{}, fmt.Errorf("vdl: rvZeroValue unhandled rt: %v tt: %v", rt, tt)
	}
	return rv, nil
}

// rvIsZeroValue returns true iff rv represents the VDL zero value.  Here are
// the types with multiple VDL zero value representations:
//   Any:            nil, or VDLIsZero on vdl.Value/vom.RawBytes
//   TypeObject:     nil, or AnyType
//   Union:          nil, or zero value of field 0
//   List, Set, Map: nil, or empty
func rvIsZeroValue(rv reflect.Value, tt *Type) (bool, error) {
	// Walk pointers and interfaces, and handle nil values.
	for rv.Kind() == reflect.Ptr || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			// All nil pointers and nil interfaces are considered to be zero.  Note
			// that we may have a non-optional type that happens to be represented by
			// a pointer; technically nil might be considered an error, but it's
			// easier for the user (and for us) to treat it as zero.
			return true, nil
		}
		rv = rv.Elem()
	}
	rt := rv.Type()
	// Now we know that rv isn't a pointer or interface, and also can't be nil.
	// Call VDLIsZero if it exists.  This handles the vdl.Value/vom.RawBytes
	// cases, as well generated code and user-implemented VDLIsZero methods.
	if rt.Implements(rtIsZeroer) {
		return rv.Interface().(IsZeroer).VDLIsZero()
	}
	if reflect.PtrTo(rt).Implements(rtIsZeroer) {
		if rv.CanAddr() {
			return rv.Addr().Interface().(IsZeroer).VDLIsZero()
		}
		// Handle the harder case where *T implements IsZeroer, but we can't take
		// the address of rv to turn it into *T.  Create a new *T value and fill it
		// in with rv, so that we can call VDLIsZero.  This is conceptually similar
		// to storing rv in a temporary variable, so that we can take the address.
		rvPtr := reflect.New(rt)
		rvPtr.Elem().Set(rv)
		return rvPtr.Interface().(IsZeroer).VDLIsZero()
	}
	// Handle native types, by converting and checking the wire value for zero.
	if ni := nativeInfoFromNative(rt); ni != nil {
		rvWirePtr := reflect.New(ni.WireType)
		if err := ni.FromNative(rvWirePtr, rv); err != nil {
			return false, err
		}
		return rvIsZeroValue(rvWirePtr.Elem(), tt)
	}
	// Optional is only zero if it is a nil pointer, which was handled above.  The
	// interface form of any was also handled above, while the non-interface forms
	// were handled via VDLIsZero.
	if tt.Kind() == Optional || tt.Kind() == Any {
		return false, nil
	}
	// TODO(toddw): We could consider adding a "fastpath" here to check against
	// the go zero value, or the zero value created by rvZeroValue, and possibly
	// returning early.  This is tricky though; we can't use this fastpath if rt
	// contains any native types, but the only way to know whether rt contains any
	// native types is to look through the entire type, which might actually be
	// slower than the benefit of this "fastpath".  The cases where it'll help are
	// large arrays or structs.
	//
	// Handle all reflect cases.
	switch rv.Kind() {
	case reflect.Bool:
		return !rv.Bool(), nil
	case reflect.String:
		return rv.String() == "", nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint() == 0, nil
	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0, nil
	case reflect.Complex64, reflect.Complex128:
		return rv.Complex() == 0, nil
	case reflect.UnsafePointer:
		return rv.Pointer() == 0, nil
	case reflect.Slice, reflect.Map:
		return rv.Len() == 0, nil
	case reflect.Array:
		for ix := 0; ix < rv.Len(); ix++ {
			if z, err := rvIsZeroValue(rv.Index(ix), tt.Elem()); err != nil || !z {
				return false, err
			}
		}
		return true, nil
	case reflect.Struct:
		switch tt.Kind() {
		case Struct:
			for ix := 0; ix < tt.NumField(); ix++ {
				field := tt.Field(ix)
				if z, err := rvIsZeroValue(rv.FieldByName(field.Name), field.Type); err != nil || !z {
					return false, err
				}
			}
			return true, nil
		case Union:
			// We already handled the nil union interface case above in the regular
			// pointer/interface walking.  Here we check to make sure the union field
			// struct represents field 0, and is set to its zero value.
			//
			// TypeFromReflect already validated Index(); call without error checking.
			if index := rv.Interface().(indexer).Index(); index != 0 {
				return false, nil
			}
			return rvIsZeroValue(rv.Field(0), tt.Field(0).Type)
		}
	}
	return false, fmt.Errorf("vdl: rvIsZeroValue unhandled rt: %v tt: %v", rt, tt)
}
