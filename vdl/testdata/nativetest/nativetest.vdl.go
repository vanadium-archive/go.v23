// This file was auto-generated by the veyron vdl tool.
// Source: nativetest.vdl

// Package nativetest tests a package with native type conversions.
package nativetest

import (
	// VDL system imports
	"v.io/core/veyron2/vdl"

	// VDL user imports
	"time"
)

type WireString int32

func (WireString) __VDLReflect(struct {
	Name string "v.io/core/veyron2/vdl/testdata/nativetest.WireString"
}) {
}

type WireMapStringInt int32

func (WireMapStringInt) __VDLReflect(struct {
	Name string "v.io/core/veyron2/vdl/testdata/nativetest.WireMapStringInt"
}) {
}

type WireTime int32

func (WireTime) __VDLReflect(struct {
	Name string "v.io/core/veyron2/vdl/testdata/nativetest.WireTime"
}) {
}

type WireSamePkg int32

func (WireSamePkg) __VDLReflect(struct {
	Name string "v.io/core/veyron2/vdl/testdata/nativetest.WireSamePkg"
}) {
}

type WireMultiImport int32

func (WireMultiImport) __VDLReflect(struct {
	Name string "v.io/core/veyron2/vdl/testdata/nativetest.WireMultiImport"
}) {
}

type WireAll struct {
	A string
	B map[string]int
	C time.Time
	D NativeSamePkg
	E map[NativeSamePkg]time.Time
}

func (WireAll) __VDLReflect(struct {
	Name string "v.io/core/veyron2/vdl/testdata/nativetest.WireAll"
}) {
}

func init() {
	vdl.RegisterNative(wireMapStringIntToNative, wireMapStringIntFromNative)
	vdl.RegisterNative(wireMultiImportToNative, wireMultiImportFromNative)
	vdl.RegisterNative(wireSamePkgToNative, wireSamePkgFromNative)
	vdl.RegisterNative(wireStringToNative, wireStringFromNative)
	vdl.RegisterNative(wireTimeToNative, wireTimeFromNative)
	vdl.Register((*WireString)(nil))
	vdl.Register((*WireMapStringInt)(nil))
	vdl.Register((*WireTime)(nil))
	vdl.Register((*WireSamePkg)(nil))
	vdl.Register((*WireMultiImport)(nil))
	vdl.Register((*WireAll)(nil))
}

// Type-check WireMapStringInt conversion functions.
var _ func(WireMapStringInt, *map[string]int) error = wireMapStringIntToNative
var _ func(*WireMapStringInt, map[string]int) error = wireMapStringIntFromNative

// Type-check WireMultiImport conversion functions.
var _ func(WireMultiImport, *map[NativeSamePkg]time.Time) error = wireMultiImportToNative
var _ func(*WireMultiImport, map[NativeSamePkg]time.Time) error = wireMultiImportFromNative

// Type-check WireSamePkg conversion functions.
var _ func(WireSamePkg, *NativeSamePkg) error = wireSamePkgToNative
var _ func(*WireSamePkg, NativeSamePkg) error = wireSamePkgFromNative

// Type-check WireString conversion functions.
var _ func(WireString, *string) error = wireStringToNative
var _ func(*WireString, string) error = wireStringFromNative

// Type-check WireTime conversion functions.
var _ func(WireTime, *time.Time) error = wireTimeToNative
var _ func(*WireTime, time.Time) error = wireTimeFromNative
