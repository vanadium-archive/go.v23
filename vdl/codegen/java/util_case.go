package java

import (
	"unicode"
	"unicode/utf8"
)

// toUpperCamelCase converts thisString to ThisString.
func toUpperCamelCase(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}
