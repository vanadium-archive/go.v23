// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdltest

import (
	"strconv"
	"strings"
)

// The VNativeWire* types are the native types associated with wire types.  The
// wire types are defined in vtype_manual.vdl.
//
// The naming is such that VWire* <-> VNativeWire*, e.g.  VWireBoolNString is
// the wire type associated with the VNativeWireBoolNString native type, where
// the wire type is bool, and the native type is string.
type (
	VNativeWireBoolNBool   bool
	VNativeWireBoolNString string
	VNativeWireBoolNStruct struct{ X string }

	VNativeWireIntNInt    int
	VNativeWireIntNString string
	VNativeWireIntNStruct struct{ X string }

	VNativeWireStringNString string
	VNativeWireStringNStruct struct{ X string }

	VNativeWireArrayNString string
	VNativeWireArrayNStruct struct{ X string }

	VNativeWireListNString string
	VNativeWireListNStruct struct{ X string }

	VNativeWireStructNString string
	VNativeWireStructNStruct struct{ X string }
	VNativeWireStructNArray  [1]string
	VNativeWireStructNSlice  []string
	// TODO(toddw): Add tests for native pointer and interface.
	//   VNativeWireStructNPointer *string
	//   VNativeWireStructNIface   interface {
	//   	Value() string
	//   }

	VNativeWireUnionNString string
	VNativeWireUnionNStruct struct{ X string }
	VNativeWireUnionNArray  [1]string
	VNativeWireUnionNSlice  []string
	// TODO(toddw): Add tests for native pointer and interface.
	//   VNativeWireUnionNPointer *string
	//   VNativeWireUnionNIface   interface {
	//   	Value() string
	//   }
)

// VWireBoolN{Bool,String,Struct}

func VWireBoolNBoolToNative(wire VWireBoolNBool, native *VNativeWireBoolNBool) error {
	*native = VNativeWireBoolNBool(wire)
	return nil
}
func VWireBoolNBoolFromNative(wire *VWireBoolNBool, native VNativeWireBoolNBool) error {
	*wire = VWireBoolNBool(native)
	return nil
}
func VWireBoolNStringToNative(wire VWireBoolNString, native *VNativeWireBoolNString) error {
	*native = VNativeWireBoolNString(strconv.FormatBool(bool(wire)))
	return nil
}
func VWireBoolNStringFromNative(wire *VWireBoolNString, native VNativeWireBoolNString) error {
	*wire = native != ""
	return nil
}
func VWireBoolNStructToNative(wire VWireBoolNStruct, native *VNativeWireBoolNStruct) error {
	native.X = strconv.FormatBool(bool(wire))
	return nil
}
func VWireBoolNStructFromNative(wire *VWireBoolNStruct, native VNativeWireBoolNStruct) error {
	*wire = native.X != ""
	return nil
}

// VWireIntN{Int,String,Struct}

func VWireIntNIntToNative(wire VWireIntNInt, native *VNativeWireIntNInt) error {
	*native = VNativeWireIntNInt(wire)
	return nil
}
func VWireIntNIntFromNative(wire *VWireIntNInt, native VNativeWireIntNInt) error {
	*wire = VWireIntNInt(native)
	return nil
}
func VWireIntNStringToNative(wire VWireIntNString, native *VNativeWireIntNString) error {
	*native = VNativeWireIntNString(strconv.Itoa(int(wire)))
	return nil
}
func VWireIntNStringFromNative(wire *VWireIntNString, native VNativeWireIntNString) error {
	x, err := strconv.Atoi(string(native))
	if err != nil {
		x = 0 // Ignore errors for this test type.
	}
	*wire = VWireIntNString(x)
	return nil
}
func VWireIntNStructToNative(wire VWireIntNStruct, native *VNativeWireIntNStruct) error {
	native.X = strconv.Itoa(int(wire))
	return nil
}
func VWireIntNStructFromNative(wire *VWireIntNStruct, native VNativeWireIntNStruct) error {
	x, err := strconv.Atoi(native.X)
	if err != nil {
		x = 0 // Ignore errors for this test type.
	}
	*wire = VWireIntNStruct(x)
	return nil
}

// VWireStringN{String,Struct}

func VWireStringNStringToNative(wire VWireStringNString, native *VNativeWireStringNString) error {
	*native = VNativeWireStringNString(wire)
	return nil
}
func VWireStringNStringFromNative(wire *VWireStringNString, native VNativeWireStringNString) error {
	*wire = VWireStringNString(native)
	return nil
}
func VWireStringNStructToNative(wire VWireStringNStruct, native *VNativeWireStringNStruct) error {
	native.X = string(wire)
	return nil
}
func VWireStringNStructFromNative(wire *VWireStringNStruct, native VNativeWireStringNStruct) error {
	*wire = VWireStringNStruct(native.X)
	return nil
}

// VWireArrayN{String,Struct}

func VWireArrayNStringToNative(wire VWireArrayNString, native *VNativeWireArrayNString) error {
	*native = VNativeWireArrayNString(wire[0])
	return nil
}
func VWireArrayNStringFromNative(wire *VWireArrayNString, native VNativeWireArrayNString) error {
	wire[0] = string(native)
	return nil
}
func VWireArrayNStructToNative(wire VWireArrayNStruct, native *VNativeWireArrayNStruct) error {
	native.X = wire[0]
	return nil
}
func VWireArrayNStructFromNative(wire *VWireArrayNStruct, native VNativeWireArrayNStruct) error {
	wire[0] = native.X
	return nil
}

// VWireListN{String,Struct}

func VWireListNStringToNative(wire VWireListNString, native *VNativeWireListNString) error {
	*native = ""
	for i, w := range wire {
		if i > 0 {
			*native += ","
		}
		*native += VNativeWireListNString(w)
	}
	return nil
}
func VWireListNStringFromNative(wire *VWireListNString, native VNativeWireListNString) error {
	*wire = nil
	if native != "" {
		for _, n := range strings.Split(string(native), ",") {
			*wire = append(*wire, n)
		}
	}
	return nil
}
func VWireListNStructToNative(wire VWireListNStruct, native *VNativeWireListNStruct) error {
	native.X = ""
	for i, w := range wire {
		if i > 0 {
			native.X += ","
		}
		native.X += w
	}
	return nil
}
func VWireListNStructFromNative(wire *VWireListNStruct, native VNativeWireListNStruct) error {
	*wire = nil
	if native.X != "" {
		for _, n := range strings.Split(native.X, ",") {
			*wire = append(*wire, n)
		}
	}
	return nil
}

// VWireStructN{String,Struct,Array,Slice,Pointer,Iface}

func VWireStructNStringToNative(wire VWireStructNString, native *VNativeWireStructNString) error {
	*native = VNativeWireStructNString(wire.X)
	return nil
}
func VWireStructNStringFromNative(wire *VWireStructNString, native VNativeWireStructNString) error {
	wire.X = string(native)
	return nil
}
func VWireStructNStructToNative(wire VWireStructNStruct, native *VNativeWireStructNStruct) error {
	native.X = wire.X
	return nil
}
func VWireStructNStructFromNative(wire *VWireStructNStruct, native VNativeWireStructNStruct) error {
	wire.X = native.X
	return nil
}
func VWireStructNArrayToNative(wire VWireStructNArray, native *VNativeWireStructNArray) error {
	native[0] = wire.X
	return nil
}
func VWireStructNArrayFromNative(wire *VWireStructNArray, native VNativeWireStructNArray) error {
	wire.X = native[0]
	return nil
}
func VWireStructNSliceToNative(wire VWireStructNSlice, native *VNativeWireStructNSlice) error {
	*native = nil
	if wire.X != "" {
		for _, w := range strings.Split(wire.X, ",") {
			*native = append(*native, w)
		}
	}
	return nil
}
func VWireStructNSliceFromNative(wire *VWireStructNSlice, native VNativeWireStructNSlice) error {
	wire.X = ""
	for i, n := range native {
		if i > 0 {
			wire.X += ","
		}
		wire.X += n
	}
	return nil
}
func (x VNativeWireStructNSlice) IsZero() bool {
	return len(x) == 0
}

// VWireUnionN{String,Struct,Array,Slice,Pointer,Iface}

func VWireUnionNStringToNative(wire VWireUnionNString, native *VNativeWireUnionNString) error {
	switch wt := wire.(type) {
	case VWireUnionNStringX:
		*native = VNativeWireUnionNString(wt.Value)
	default:
		*native = ""
	}
	return nil
}
func VWireUnionNStringFromNative(wire *VWireUnionNString, native VNativeWireUnionNString) error {
	*wire = VWireUnionNStringX{Value: string(native)}
	return nil
}
func VWireUnionNStructToNative(wire VWireUnionNStruct, native *VNativeWireUnionNStruct) error {
	switch wt := wire.(type) {
	case VWireUnionNStructX:
		native.X = wt.Value
	default:
		native.X = ""
	}
	return nil
}
func VWireUnionNStructFromNative(wire *VWireUnionNStruct, native VNativeWireUnionNStruct) error {
	*wire = VWireUnionNStructX{Value: native.X}
	return nil
}
func VWireUnionNArrayToNative(wire VWireUnionNArray, native *VNativeWireUnionNArray) error {
	switch wt := wire.(type) {
	case VWireUnionNArrayX:
		native[0] = wt.Value
	default:
		native[0] = ""
	}
	return nil
}
func VWireUnionNArrayFromNative(wire *VWireUnionNArray, native VNativeWireUnionNArray) error {
	*wire = VWireUnionNArrayX{Value: native[0]}
	return nil
}
func VWireUnionNSliceToNative(wire VWireUnionNSlice, native *VNativeWireUnionNSlice) error {
	*native = nil
	switch wt := wire.(type) {
	case VWireUnionNSliceX:
		if wt.Value != "" {
			for _, w := range strings.Split(wt.Value, ",") {
				*native = append(*native, w)
			}
		}
	}
	return nil
}
func VWireUnionNSliceFromNative(wire *VWireUnionNSlice, native VNativeWireUnionNSlice) error {
	x := ""
	for i, n := range native {
		if i > 0 {
			x += ","
		}
		x += n
	}
	*wire = VWireUnionNSliceX{Value: x}
	return nil
}
func (x VNativeWireUnionNSlice) IsZero() bool {
	return len(x) == 0
}
