package java

import (
	"bytes"
	"log"
	"unicode"
	"unicode/utf8"
)

// toConstCase converts ThisString to THIS_STRING.
func toConstCase(s string) string {
	// Extract all characters.
	chars := []rune{}
	idx := 0
	for idx < len(s) {
		r, size := utf8.DecodeRuneInString(s[idx:])
		if r == utf8.RuneError {
			log.Fatalf("Invalid UTF8 string: %s", s)
		}
		chars = append(chars, r)
		idx += size
	}
	// Case.
	var buf bytes.Buffer
	for i, r := range chars {
		if i > 0 && unicode.IsUpper(r) && i < len(chars)-1 && !unicode.IsUpper(chars[i+1]) {
			buf.WriteRune('_')
		}
		buf.WriteRune(unicode.ToUpper(r))
	}
	return buf.String()
}

// toUpperCamelCase converts thisString to ThisString.
func toUpperCamelCase(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}
