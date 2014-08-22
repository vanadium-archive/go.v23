package vdlutil

import (
	"testing"
)

func TestFirstRuneToLower(t *testing.T) {
	tests := []struct {
		arg, want string
	}{
		{"foo", "foo"},
		{"Foo", "foo"},
		{"FOO", "fOO"},
		{"foobar", "foobar"},
		{"fooBar", "fooBar"},
		{"FooBar", "fooBar"},
		{"FOOBAR", "fOOBAR"},
	}
	for _, test := range tests {
		if got, want := FirstRuneToLower(test.arg), test.want; got != want {
			t.Errorf("FirstRuneToLower(%s) got %s, want %s", test.arg, got, want)
		}
	}
}

func TestFirstRuneToUpper(t *testing.T) {
	tests := []struct {
		arg, want string
	}{
		{"foo", "Foo"},
		{"Foo", "Foo"},
		{"FOO", "FOO"},
		{"foobar", "Foobar"},
		{"fooBar", "FooBar"},
		{"FooBar", "FooBar"},
		{"FOOBAR", "FOOBAR"},
	}
	for _, test := range tests {
		if got, want := FirstRuneToUpper(test.arg), test.want; got != want {
			t.Errorf("FirstRuneToUpper(%s) got %s, want %s", test.arg, got, want)
		}
	}
}
