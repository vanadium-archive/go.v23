package nativetest

import (
	"strconv"
	"time"
)

func (x WireString) VDLToNative(native *string) error {
	*native = strconv.Itoa(int(x))
	return nil
}
func (x *WireString) VDLFromNative(native string) error {
	v, err := strconv.Atoi(native)
	*x = WireString(v)
	return err
}

func (x WireMapStringInt) VDLToNative(native *map[string]int) error   { return nil }
func (x *WireMapStringInt) VDLFromNative(native map[string]int) error { return nil }

func (x WireTime) VDLToNative(native *time.Time) error   { return nil }
func (x *WireTime) VDLFromNative(native time.Time) error { return nil }

func (x WireSamePkg) VDLToNative(native *NativeSamePkg) error   { return nil }
func (x *WireSamePkg) VDLFromNative(native NativeSamePkg) error { return nil }

func (x WireMultiImport) VDLToNative(native *map[NativeSamePkg]time.Time) error   { return nil }
func (x *WireMultiImport) VDLFromNative(native map[NativeSamePkg]time.Time) error { return nil }

type NativeSamePkg string
