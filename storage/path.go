package storage

import (
	"bytes"
	"encoding/hex"
	"strings"
)

// PathName is a slash-separated path.
type PathName []string

const (
	UIDDirName = "uid"
)

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

// IsID returns true iff the path refers to a value by id: /uid/<ID>/...
func (p PathName) IsID() bool {
	return len(p) >= 2 && p[0] == UIDDirName
}

// IsStrictID returns true iff the path refers to a valud by ID, with no suffix: /uid/<ID>.
func (p PathName) IsStrictID() bool {
	return p.IsID() && len(p) == 2
}

// GetID returns the ID for an id path.
func (p PathName) GetID() (ID, PathName, bool) {
	var id ID
	if !p.IsID() {
		return id, nil, false
	}
	_, err := hex.Decode(id[:], []byte(p[1]))
	if err != nil {
		return id, nil, false
	}
	return id, p[2:], true
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
