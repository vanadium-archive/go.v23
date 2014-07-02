package vdl

import (
	"unicode"
	"unicode/utf8"
)

// ToCamelCase converts ThisString to thisString.
func ToCamelCase(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}
