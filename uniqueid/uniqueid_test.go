package uniqueid

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestNewID(t *testing.T) {
	g := RandomGenerator{}
	numHitMaxCount := 2
	maxCount := 0x7fff
	firstRandomValue := []byte{}
	var atLeastOneDiffersFromFirst bool
	for i := 0; i < numHitMaxCount*maxCount; i++ {
		id, err := g.NewID()
		if err != nil {
			t.Fatal("Error generating new id: ", err)
		}

		if firstRandomValue == nil {
			firstRandomValue = id[:14]
		}
		if !bytes.Equal(firstRandomValue, []byte(id[:14])) {
			atLeastOneDiffersFromFirst = true
		}

		if id[14]&0x80 != 0x80 {
			t.Errorf("Expected high bit to be 1, but containing byte was: %x", id[14])
		}
		if binary.BigEndian.Uint16(id[14:])&0x7fff != uint16(i)&0x7fff {
			t.Errorf("Counts don't match. Got: %d, Expected: %d", binary.BigEndian.Uint16(id[14:])&0x7fff, i&0x7fff)
		}
	}
	if !atLeastOneDiffersFromFirst {
		t.Errorf("Expected at least two of the randomly generated numbers to be different")
	}
}

func BenchmarkNewIDParallel(b *testing.B) {
	g := RandomGenerator{}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			g.NewID()
		}
	})
}
