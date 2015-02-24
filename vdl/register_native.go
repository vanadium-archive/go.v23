package vdl

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// RegisterNative registers conversion functions between a VDL wire type and a
// Go native type.  This is typically used when there is a more idiomatic native
// representation for a given wire type; e.g. the VDL standard Time type is
// converted into the Go standard time.Time.
//
// The signatures of the conversion functions is expected to be:
//   func ToNative(wire W, native *N) error
//   func FromNative(wire *W, native N) error
//
// The VDL conversion routines automatically apply these conversion functions,
// to avoid manual marshaling by the user.  The "dst" values (i.e. native in
// ToNative, and wire in FromNative) are guaranteed to be an allocated zero
// value of the respective type.
//
// As a special-case, RegisterNative is also called by the verror package to
// register conversions between vdl.WireError and the standard Go error type.
// This is required for error conversions to work correctly.
//
// RegisterNative is not intended to be called by end users; calls are
// auto-generated for types with native conversions in *.vdl files.
func RegisterNative(toFn, fromFn interface{}) {
	ni, err := deriveNativeInfo(toFn, fromFn)
	if err != nil {
		panic(fmt.Errorf("vdl: RegisterNative invalid (%v)", err))
	}
	if err := niReg.addNativeInfo(ni); err != nil {
		panic(fmt.Errorf("vdl: RegisterNative invalid (%v)", err))
	}
}

// nativeType holds the mapping between a VDL wire type and a Go native type,
// along with functions to convert between values of these types.
type nativeInfo struct {
	WireType       reflect.Type  // Wire type from the conversion funcs.
	NativeType     reflect.Type  // Native type from the conversion funcs.
	toNativeFunc   reflect.Value // ToNative conversion func.
	fromNativeFunc reflect.Value // FromNative conversion func.
}

func (ni *nativeInfo) ToNative(wire, native reflect.Value) error {
	if ierr := ni.toNativeFunc.Call([]reflect.Value{wire, native})[0].Interface(); ierr != nil {
		return ierr.(error)
	}
	return nil
}

func (ni *nativeInfo) FromNative(wire, native reflect.Value) error {
	if ierr := ni.fromNativeFunc.Call([]reflect.Value{wire, native})[0].Interface(); ierr != nil {
		return ierr.(error)
	}
	return nil
}

// niRegistry holds the nativeInfo registry.  Unlike rtRegister (used for the
// rtCache), this information cannot be regenerated at will.  We expect a small
// number of native types to be registered within a single address space.
type niRegistry struct {
	sync.RWMutex
	fromWire   map[reflect.Type]*nativeInfo
	fromNative map[reflect.Type]*nativeInfo
	forError   *nativeInfo
}

var niReg = &niRegistry{
	fromWire:   make(map[reflect.Type]*nativeInfo),
	fromNative: make(map[reflect.Type]*nativeInfo),
}

func (reg *niRegistry) addNativeInfo(ni *nativeInfo) error {
	reg.Lock()
	defer reg.Unlock()
	// Require a bijective mapping.  Also reject chains of A <-> B <-> C..., which
	// would add unnecessary complexity to the already complex conversion logic.
	dup1 := reg.fromWire[ni.WireType]
	dup2 := reg.fromWire[ni.NativeType]
	dup3 := reg.fromNative[ni.WireType]
	dup4 := reg.fromNative[ni.NativeType]
	if dup1 != nil || dup2 != nil || dup3 != nil || dup4 != nil {
		return fmt.Errorf("non-bijective mapping, or chaining, %#v duplicates: %#v %#v %#v %#v", ni, dup1, dup2, dup3, dup4)
	}
	if ni.WireType == rtWireError && ni.NativeType == rtError {
		// Special-case the WireError<->error conversions.  These need to be tracked
		// separately, since the native type is an interface, and needs special
		// handling in our conversion routines.
		if reg.forError != nil {
			return fmt.Errorf("WireError<->error conversion already registered")
		}
		reg.forError = ni
		return nil
	}
	reg.fromWire[ni.WireType] = ni
	reg.fromNative[ni.NativeType] = ni
	return nil
}

// nativeInfoFromWire returns the nativeInfo for the given wire type, or nil if
// RegisterNative has not been called for the wire type.
func nativeInfoFromWire(wire reflect.Type) *nativeInfo {
	niReg.RLock()
	ni := niReg.fromWire[wire]
	niReg.RUnlock()
	return ni
}

// nativeInfoFromNative returns the nativeInfo for the given native type, or nil
// if RegisterNative has not been called for the native type.
func nativeInfoFromNative(native reflect.Type) *nativeInfo {
	niReg.RLock()
	ni := niReg.fromNative[native]
	niReg.RUnlock()
	return ni
}

// nativeInfoForError returns the non-nil nativeInfo holding WireError<->error
// conversions.  Returns an error if RegisterNative has not been called for
// these error conversions.
func nativeInfoForError() (*nativeInfo, error) {
	niReg.RLock()
	ni := niReg.forError
	niReg.RUnlock()
	if ni == nil {
		return nil, errNoRegisterNativeError
	}
	return ni, nil
}

var errNoRegisterNativeError = errors.New(`vdl: RegisterNative must be called to register error<->WireError conversions.  Import the "v.io/v23/verror" package in your program.`)

// deriveNativeInfo returns the nativeInfo corresponding to toFn and fromFn,
// which are expected to have the following signatures:
//   func ToNative(wire W, native *N) error
//   func FromNative(wire *W, native N) error
func deriveNativeInfo(toFn, fromFn interface{}) (*nativeInfo, error) {
	if toFn == nil || fromFn == nil {
		return nil, fmt.Errorf("nil arguments")
	}
	rvT, rvF := reflect.ValueOf(toFn), reflect.ValueOf(fromFn)
	rtT, rtF := rvT.Type(), rvF.Type()
	if rtT.Kind() != reflect.Func || rtF.Kind() != reflect.Func {
		return nil, fmt.Errorf("arguments must be functions")
	}
	if rtT.NumIn() != 2 || rtT.In(1).Kind() != reflect.Ptr || rtT.NumOut() != 1 || rtT.Out(0) != rtError {
		return nil, fmt.Errorf("toFn must have signature ToNative(wire W, native *N) error")
	}
	if rtF.NumIn() != 2 || rtF.In(0).Kind() != reflect.Ptr || rtF.NumOut() != 1 || rtF.Out(0) != rtError {
		return nil, fmt.Errorf("fromFn must have signature FromNative(wire *W, native N) error")
	}
	rtWire, rtNative := rtT.In(0), rtF.In(1)
	if rtWire != rtF.In(0).Elem() || rtNative != rtT.In(1).Elem() {
		return nil, fmt.Errorf("mismatched wire/native types, want signatures ToNative(wire W, native *N) error, FromNative(wire *W, native N) error")
	}
	if rtWire == rtNative {
		return nil, fmt.Errorf("wire type == native type: %v", rtWire)
	}
	return &nativeInfo{rtWire, rtNative, rvT, rvF}, nil
}
