package storage

import (
	"bytes"
	"strings"
)

// PathName is a slash-separated path.
type PathName []string

// ParsePath splits the string into its list of components.
func ParsePath(p string) PathName {
	var pll PathName
	parts := strings.Split(p, "/")
	for _, c := range parts {
		switch c {
		case "", ".":
			// do nothing
		default:
			pll = append(pll, c)
		}
	}
	return pll
}

// IsRoot returns true iff the path refers to the root directory.
func (p PathName) IsRoot() bool {
	return len(p) == 0
}

// Split splits the path into a directory and file part.
func (p PathName) Split() (PathName, string) {
	if len(p) == 0 {
		return PathName{}, ""
	}
	return p[:len(p)-1], p[len(p)-1]
}

func (p PathName) String() string {
	var buf bytes.Buffer
	for i, s := range p {
		if i != 0 {
			buf.WriteRune('/')
		}
		buf.WriteString(s)
	}
	return buf.String()
}
