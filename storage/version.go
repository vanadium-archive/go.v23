package storage

import (
	"math/rand"
	"time"
)

var rng *rand.Rand

func init() {
	rng = rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
}

// NewVersion returns a new version number.
//
// TODO(jyh): Choose a better version generator.
func NewVersion() Version {
	for {
		if v := Version(rng.Int63()); v != 0 {
			return v
		}
	}
}
