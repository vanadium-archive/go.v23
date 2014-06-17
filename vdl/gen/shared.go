package gen

import (
	"unicode"
	"unicode/utf8"
)

// toCamelCase converts ThisString to thisString.
func toCamelCase(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}
