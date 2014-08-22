package vdlutil

import (
	"unicode"
	"unicode/utf8"
)

// ToCamelCase converts ThisString to thisString.
// TODO(toddw): Remove this function, replace calls with FirstRuneToLower.
func ToCamelCase(s string) string {
	return FirstRuneToLower(s)
}

// FirstRuneToLower returns s with its first rune in lowercase.
func FirstRuneToLower(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}

// FirstRuneToUpper returns s with its first rune in uppercase.
func FirstRuneToUpper(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}

// FirstRuneToExportCase returns s with its first rune in uppercase if export is
// true, otherwise in lowercase.
func FirstRuneToExportCase(s string, export bool) string {
	if export {
		return FirstRuneToUpper(s)
	}
	return FirstRuneToLower(s)
}
