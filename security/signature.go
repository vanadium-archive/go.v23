package security

import (
	"crypto/ecdsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"math/big"
)

// Verify returns true iff sig is a valid signature for a message.
func (sig Signature) Verify(key *ecdsa.PublicKey, message []byte) bool {
	if message = cryptoHash(sig.Hash, message); message == nil {
		return false
	}
	var r, s big.Int
	return ecdsa.Verify(key, message, r.SetBytes(sig.R), s.SetBytes(sig.S))
}

// cryptoHash hashes the provided data using the given cryptographic hash
// function, returning nil iff the hash couldn't be applied.
func cryptoHash(hash Hash, data []byte) []byte {
	switch hash {
	case NoHash:
		return data
	case SHA1Hash:
		h := sha1.Sum(data)
		return h[:]
	case SHA256Hash:
		h := sha256.Sum256(data)
		return h[:]
	case SHA384Hash:
		h := sha512.Sum384(data)
		return h[:]
	case SHA512Hash:
		h := sha512.Sum512(data)
		return h[:]
	}
	return nil
}
