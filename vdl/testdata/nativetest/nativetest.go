package nativetest

import (
	"strconv"
	"time"
)

func wireStringToNative(x WireString, native *string) error {
	*native = strconv.Itoa(int(x))
	return nil
}
func wireStringFromNative(x *WireString, native string) error {
	v, err := strconv.Atoi(native)
	*x = WireString(v)
	return err
}

func wireMapStringIntToNative(WireMapStringInt, *map[string]int) error   { return nil }
func wireMapStringIntFromNative(*WireMapStringInt, map[string]int) error { return nil }

func wireTimeToNative(WireTime, *time.Time) error   { return nil }
func wireTimeFromNative(*WireTime, time.Time) error { return nil }

func wireSamePkgToNative(WireSamePkg, native *NativeSamePkg) error { return nil }
func wireSamePkgFromNative(*WireSamePkg, NativeSamePkg) error      { return nil }

func wireMultiImportToNative(WireMultiImport, *map[NativeSamePkg]time.Time) error   { return nil }
func wireMultiImportFromNative(*WireMultiImport, map[NativeSamePkg]time.Time) error { return nil }

type NativeSamePkg string
