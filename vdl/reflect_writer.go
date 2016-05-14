// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"errors"
	"fmt"
	"reflect"
)

var (
	errWriteMustReflect = errors.New("vdl: write must be handled via reflection")
)

// Write uses enc to encode value v, calling VDLWrite methods and fast compiled
// writers when available, and using reflection otherwise.  This is basically an
// all-purpose VDLWrite implementation.
func Write(enc Encoder, v interface{}) error {
	if v == nil {
		return enc.NilValue(AnyType)
	}
	rv := reflect.ValueOf(v)
	// Fastpath check for non-reflect support.  Unfortunately we must use
	// reflection to detect the case where v is a pointer, which is handled by the
	// more complicated optional-checking logic in writeReflect.
	//
	// TODO(toddw): *vom.RawBytes is 50% faster if we could special-case it here,
	// without breaking support for optional types.
	if rv.Kind() != reflect.Ptr {
		if err := writeNonReflect(enc, v); err != errWriteMustReflect {
			return err
		}
	}
	tt, err := TypeFromReflect(rv.Type())
	if err != nil {
		return err
	}
	return writeReflect(enc, rv, tt)
}

var ttBytes = ListType(ByteType)

func writeNonReflect(enc Encoder, v interface{}) error {
	switch x := v.(type) {
	case Writer:
		// Writer handles the case where x has a code-generated decoder, and
		// special-cases such as vdl.Value and vom.RawBytes.
		return x.VDLWrite(enc)

		// Cases after this point are purely performance optimizations.
		// TODO(toddw): Handle other common cases.
	case []byte:
		if err := enc.StartValue(ttBytes); err != nil {
			return err
		}
		if err := enc.EncodeBytes(x); err != nil {
			return err
		}
		return enc.FinishValue()
	}
	return errWriteMustReflect
}

// WriteReflect is like Write, but takes a reflect.Value argument.
func WriteReflect(enc Encoder, rv reflect.Value) error {
	if !rv.IsValid() {
		return enc.NilValue(AnyType)
	}
	tt, err := TypeFromReflect(rv.Type())
	if err != nil {
		return err
	}
	return writeReflect(enc, rv, tt)
}

func writeReflect(enc Encoder, rv reflect.Value, tt *Type) error {
	// Fastpath check for non-reflect support.  Optional types are tricky, since
	// they may be nil, and need SetNextStartValueIsOptional() to be set, so they
	// can't use this fastpath.  This handles the non-nil *vom.RawBytes and
	// *vdl.Value cases, and avoids an expensive copy of all their fields.
	if tt.Kind() != Optional && (rv.Kind() != reflect.Ptr || !rv.IsNil()) {
		if err := writeNonReflect(enc, rv.Interface()); err != errWriteMustReflect {
			return err
		}
	}
	// Walk pointers and interfaces in rv, and handle nil values.
	for {
		isPtr, isIface := rv.Kind() == reflect.Ptr, rv.Kind() == reflect.Interface
		if !isPtr && !isIface {
			break
		}
		if rv.IsNil() {
			switch {
			case tt.Kind() == TypeObject:
				// Treat nil *Type as AnyType.
				return AnyType.VDLWrite(enc)
			case tt.Kind() == Union && isIface:
				// Treat nil Union interface as the zero value of the type at index 0.
				return ZeroValue(tt).VDLWrite(enc)
			case tt.Kind() == Optional:
				enc.SetNextStartValueIsOptional()
				return enc.NilValue(tt)
			case tt == AnyType:
				return enc.NilValue(tt)
			}
			return fmt.Errorf("vdl: can't encode nil from non-any non-optional %v", tt)
		}
		rv = rv.Elem()
		// Recompute tt as we pass interface boundaries.  There's no need to
		// recompute as we traverse pointers, since tt won't change.
		if isIface {
			var err error
			if tt, err = TypeFromReflect(rv.Type()); err != nil {
				return err
			}
		}
	}
	if tt.Kind() == Optional {
		enc.SetNextStartValueIsOptional()
	}
	// Check for faster non-reflect support, which also handles vdl.Value and
	// vom.RawBytes, and any other special-cases.
	if err := writeNonReflect(enc, rv.Interface()); err != errWriteMustReflect {
		return err
	}
	if reflect.PtrTo(rv.Type()).Implements(rtVDLWriter) {
		if rv.CanAddr() {
			return writeNonReflect(enc, rv.Addr().Interface())
		} else {
			// This handles the case where rv implements VDLWrite with a pointer
			// receiver, but we can't address rv to get a pointer.  E.g.
			//    type Foo string
			//    func (x *Foo) VDLWrite(enc vdl.Encoder) error {...}
			//    rv := Foo{}
			//
			// TODO(toddw): Do we need to handle this case?
			rvPtr := reflect.New(rv.Type())
			rvPtr.Elem().Set(rv)
			return writeNonReflect(enc, rvPtr.Interface())
		}
	}
	// Handle marshaling from native type to wire type.
	if ni := nativeInfoFromNative(rv.Type()); ni != nil {
		rvWirePtr := reflect.New(ni.WireType)
		if err := ni.FromNative(rvWirePtr, rv); err != nil {
			return err
		}
		return writeReflect(enc, rvWirePtr.Elem(), tt)
	}
	// Handle errors that are implemented by arbitrary rv values.  E.g. the Go
	// standard errors.errorString implements the error interface, but is an
	// invalid vdl type since it doesn't have any exported fields.
	//
	// See corresponding special-case in reflect_type.go
	if tt == ErrorType {
		if rv.Type().Implements(rtError) {
			return writeNonNilError(enc, rv)
		}
		if rv.CanAddr() && rv.Addr().Type().Implements(rtError) {
			return writeNonNilError(enc, rv.Addr())
		}
	}
	// Handle regular non-nil values.
	tt = tt.NonOptional()
	if err := enc.StartValue(tt); err != nil {
		return err
	}
	if err := writeNonNilValue(enc, rv, tt); err != nil {
		return err
	}
	return enc.FinishValue()
}

// writeNonNilError writes rvNative, which must be a non-nil implementation of
// the Go error interface, out to enc.
func writeNonNilError(enc Encoder, rvNative reflect.Value) error {
	ni := nativeInfoFromNative(rtError)
	if ni == nil {
		return errNoRegisterNativeError
	}
	rvWirePtr := reflect.New(ni.WireType)
	if err := ni.FromNative(rvWirePtr, rvNative); err != nil {
		return err
	}
	return writeReflect(enc, rvWirePtr.Elem(), ErrorType)
}

func writeNonNilValue(enc Encoder, rv reflect.Value, tt *Type) error {
	// Handle named and unnamed []byte and [N]byte, where the element type is the
	// unnamed byte type.  Cases like []MyByte fall through and are handled as
	// regular lists, since we can't easily convert []MyByte to []byte.
	switch {
	case tt.Kind() == Array && tt.Elem() == ByteType:
		var bytes []byte
		if rv.CanAddr() {
			bytes = rv.Slice(0, tt.Len()).Interface().([]byte)
		} else {
			bytes = make([]byte, tt.Len())
			reflect.Copy(reflect.ValueOf(bytes), rv)
		}
		return enc.EncodeBytes(bytes)
	case tt.Kind() == List && tt.Elem() == ByteType:
		bytes := rv.Convert(rtByteList).Interface().([]byte)
		return enc.EncodeBytes(bytes)
	}
	// Handle regular non-nil values.
	switch tt.Kind() {
	case Bool:
		return enc.EncodeBool(rv.Bool())
	case String:
		return enc.EncodeString(rv.String())
	case Enum:
		// TypeFromReflect already validated String(); call without error checking.
		label := rv.Interface().(stringer).String()
		return enc.EncodeString(label)
	case Byte, Uint16, Uint32, Uint64:
		return enc.EncodeUint(rv.Uint())
	case Int8, Int16, Int32, Int64:
		return enc.EncodeInt(rv.Int())
	case Float32, Float64:
		return enc.EncodeFloat(rv.Float())
	case Array, List:
		return writeArrayOrList(enc, rv, tt)
	case Set, Map:
		return writeSetOrMap(enc, rv, tt)
	case Struct:
		return writeStruct(enc, rv, tt)
	case Union:
		return writeUnion(enc, rv, tt)
	}
	// Special representations like vdl.Type, vdl.Value and vom.RawBytes all
	// implement VDLWrite, and should have been handled by writeNonReflect.  Nil
	// optional and any should have been handled by the pointer-flattening loop in
	// writeReflect.  Non-nil optional should have been flattened after the loop,
	// while non-nil any should have flattened itself down to a non-any type.
	return fmt.Errorf("vdl: Write unhandled type %v %v", rv.Type(), tt)
}

func writeArrayOrList(enc Encoder, rv reflect.Value, tt *Type) error {
	if tt.Kind() == List {
		if err := enc.SetLenHint(rv.Len()); err != nil {
			return err
		}
	}
	for ix := 0; ix < rv.Len(); ix++ {
		if err := enc.NextEntry(false); err != nil {
			return err
		}
		if err := writeReflect(enc, rv.Index(ix), tt.Elem()); err != nil {
			return err
		}
	}
	return enc.NextEntry(true)
}

func writeSetOrMap(enc Encoder, rv reflect.Value, tt *Type) error {
	if err := enc.SetLenHint(rv.Len()); err != nil {
		return err
	}
	for _, rvKey := range rv.MapKeys() {
		if err := enc.NextEntry(false); err != nil {
			return err
		}
		if err := writeReflect(enc, rvKey, tt.Key()); err != nil {
			return err
		}
		if tt.Kind() == Map {
			if err := writeReflect(enc, rv.MapIndex(rvKey), tt.Elem()); err != nil {
				return err
			}
		}
	}
	return enc.NextEntry(true)
}

func writeStruct(enc Encoder, rv reflect.Value, tt *Type) error {
	// Loop through tt fields rather than rt fields, since the VDL type tt might
	// have ignored some of the fields in rt, e.g. unexported fields.
	for ix := 0; ix < tt.NumField(); ix++ {
		field := tt.Field(ix)
		rvField := rv.FieldByName(field.Name)
		if !rvField.IsValid() {
			panic(fmt.Errorf("vdl: reflect type %v doesn't have field %q, vdl field %d of type %v", rv.Type(), field.Name, ix, tt))
		}
		switch isZero, err := rvIsZeroValue(rvField, field.Type); {
		case err != nil:
			return err
		case isZero:
			continue // skip zero-valued fields
		}
		if err := enc.NextField(field.Name); err != nil {
			return err
		}
		if err := writeReflect(enc, rvField, field.Type); err != nil {
			return err
		}
	}
	return enc.NextField("")
}

func writeUnion(enc Encoder, rv reflect.Value, tt *Type) error {
	// TypeFromReflect already validated Name() and Index().
	iface := rv.Interface()
	name, index := iface.(namer).Name(), iface.(indexer).Index()
	if err := enc.NextField(name); err != nil {
		return err
	}
	// Since this is a non-nil union, we're guaranteed rv is the concrete field
	// struct, so we can just grab the "Value" field.
	rvField := rv.Field(0)
	if err := writeReflect(enc, rvField, tt.Field(index).Type); err != nil {
		return err
	}
	return enc.NextField("")
}
