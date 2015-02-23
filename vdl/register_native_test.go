package vdl

import (
	"fmt"
	"reflect"
	"testing"
)

type (
	wireA   struct{}
	nativeA struct{}
)

func TestRegisterNative(t *testing.T) {
	tests := []*nativeInfo{
		{
			reflect.TypeOf(wireA{}),
			reflect.TypeOf(nativeA{}),
			reflect.ValueOf(func(wireA, *nativeA) error { return nil }),
			reflect.ValueOf(func(*wireA, nativeA) error { return nil }),
		},
		// TODO(toddw): Add tests where the wire type is a VDL union.
	}
	for _, test := range tests {
		name := fmt.Sprint("[%v %v]", test.WireType, test.NativeType)
		RegisterNative(test.toNativeFunc.Interface(), test.fromNativeFunc.Interface())
		if got, want := nativeInfoFromWire(test.WireType), test; !reflect.DeepEqual(got, want) {
			t.Errorf("%s nativeInfoFromWire got %#v, want %#v", name, got, want)
		}
		if got, want := nativeInfoFromNative(test.NativeType), test; !reflect.DeepEqual(got, want) {
			t.Errorf("%s nativeInfoFromNative got %#v, want %#v", name, got, want)
		}
	}
}

func TestDeriveNativeInfoError(t *testing.T) {
	const (
		errTo   = "toFn must have signature ToNative(wire W, native *N) error"
		errFrom = "fromFn must have signature FromNative(wire *W, native N) error"
		errMis  = "mismatched wire/native types"
	)
	var (
		goodTo   = func(wireA, *nativeA) error { return nil }
		goodFrom = func(*wireA, nativeA) error { return nil }
	)

	tests := []struct {
		Name                 string
		ToNative, FromNative interface{}
		ErrStr               string
	}{
		{"NilFuncs", nil, nil, "nil arguments"},
		{"NotFuncs", "abc", "abc", "arguments must be functions"},
		{"BadTo1", func() {}, goodFrom, errTo},
		{"BadTo2", func(wireA) {}, goodFrom, errTo},
		{"BadTo3", func(wireA, nativeA) {}, goodFrom, errTo},
		{"BadTo3", func(wireA, *nativeA) {}, goodFrom, errTo},
		{"BadTo3", func(wireA, nativeA) error { return nil }, goodFrom, errTo},
		{"BadFrom1", goodTo, func() {}, errFrom},
		{"BadFrom2", goodTo, func(wireA) {}, errFrom},
		{"BadFrom3", goodTo, func(wireA, nativeA) {}, errFrom},
		{"BadFrom3", goodTo, func(*wireA, nativeA) {}, errFrom},
		{"BadFrom3", goodTo, func(wireA, nativeA) error { return nil }, errFrom},
		{
			"Mismatch1",
			func(string, *nativeA) error { return nil },
			func(*wireA, nativeA) error { return nil },
			errMis,
		},
		{
			"Mismatch2",
			func(wireA, *string) error { return nil },
			func(*wireA, nativeA) error { return nil },
			errMis,
		},
		{
			"Mismatch3",
			func(wireA, *nativeA) error { return nil },
			func(*string, nativeA) error { return nil },
			errMis,
		},
		{
			"Mismatch4",
			func(wireA, *nativeA) error { return nil },
			func(*wireA, string) error { return nil },
			errMis,
		},
		{
			"MismatchPtr1",
			func(*wireA, *nativeA) error { return nil },
			func(*wireA, nativeA) error { return nil },
			errMis,
		},
		{
			"MismatchPtr2",
			func(wireA, *nativeA) error { return nil },
			func(*wireA, *nativeA) error { return nil },
			errMis,
		},
		{
			"SameType",
			func(wireA, *wireA) error { return nil },
			func(*wireA, wireA) error { return nil },
			"wire type == native type",
		},
	}
	for _, test := range tests {
		ni, err := deriveNativeInfo(test.ToNative, test.FromNative)
		if ni != nil {
			t.Errorf("%s got %#v, want nil", test.Name, ni)
		}
		ExpectErr(t, err, test.ErrStr, test.Name)
	}
}
