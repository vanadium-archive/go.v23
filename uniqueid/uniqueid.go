// Package uniqueid helps generate identifiers that are likely to be
// globally unique.  We want to be able to generate many IDs quickly,
// so we make a time/space tradeoff.  We reuse the same random data
// many times with a counter appended.  Note: these IDs are NOT useful
// as a security mechanism as they will be predictable.
package uniqueid

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
)

var random = RandomGenerator{}

func (id ID) String() string {
	return fmt.Sprintf("0x%x", [16]byte(id))
}

// Valid returns true if the given ID is valid.
func Valid(id ID) bool {
	return id != ID{}
}

func FromHexString(s string) (ID, error) {
	var id ID
	var slice []byte
	if strings.HasPrefix(s, "0x") {
		s = s[2:]
	}
	if _, err := fmt.Sscanf(s, "%x", &slice); err != nil {
		return id, err
	}
	if len(slice) != len(id) {
		return id, fmt.Errorf("Cannot convert %s to ID, size mismatch.", s)
	}
	copy(id[:], slice)
	return id, nil
}

// A RandomGenerator can generate random IDs.
// The zero value of RandomGenerator is ready to use.
type RandomGenerator struct {
	mu    sync.Mutex
	id    ID
	count uint16
}

// NewID produces a new probably unique identifier.
func (g *RandomGenerator) NewID() (ID, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.count > 0x7fff || g.count == uint16(0) {
		g.count = 0

		// Either the generator is uninitialized or the counter
		// has wrapped.  We need a new random prefix.
		if _, err := rand.Read(g.id[:14]); err != nil {
			return ID{}, err
		}
	}
	binary.BigEndian.PutUint16(g.id[14:], g.count)
	g.count++
	g.id[14] |= 0x80 // Use this bit as a reserved bit (set to 1) to support future format changes.
	return g.id, nil
}

// Random produces a new probably unique identifier using the RandomGenerator.
func Random() (ID, error) {
	return random.NewID()
}
