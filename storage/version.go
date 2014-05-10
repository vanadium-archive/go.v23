package storage

import (
	"math/rand"
)

// NewVersion returns a new version number.
//
// TODO(jyh): Choose a better version generator.
func NewVersion() Version {
	for {
		if v := Version(rand.Int63()); v != 0 {
			return v
		}
	}
}
