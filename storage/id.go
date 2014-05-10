package storage

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
)

// NewID returns a new ID.  This implementation is based on crypto/rand.
func NewID() ID {
	var id ID
	if _, err := rand.Read(id[:]); err != nil {
		panic("NewID: rand error")
	}
	return id
}

// Print IDs in hex.
func (id ID) String() string {
	return hex.EncodeToString(id[:])
}

// Returns true iff the ID is not the zero id.
func (id ID) IsValid() bool {
	return id != ID{}
}

// compareIDs compares two storage.ID values lexicographically.
func CompareIDs(id1, id2 ID) int {
	return bytes.Compare(id1[:], id2[:])
}
