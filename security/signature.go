package security

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"math/big"
)

// Verify returns true iff sig is a valid signature for a message.
func (sig Signature) Verify(key PublicKey, message []byte) bool {
	if message = cryptoHash(sig.Hash, message); message == nil {
		return false
	}
	switch v := key.(type) {
	case *ecdsaPublicKey:
		var r, s big.Int
		return ecdsa.Verify(v.key, message, r.SetBytes(sig.R), s.SetBytes(sig.S))
	default:
		return false
	}
}

// NewInMemoryECDSASigner creates a Signer that uses the provided ECDSA private
// key to sign messages.  This private key is kept in the clear in the memory
// of the running process.
func NewInMemoryECDSASigner(key *ecdsa.PrivateKey) Signer {
	var hash Hash
	if nbits := key.Curve.Params().BitSize; nbits <= 160 {
		hash = SHA1Hash
	} else if nbits <= 256 {
		hash = SHA256Hash
	} else if nbits <= 384 {
		hash = SHA384Hash
	} else {
		hash = SHA512Hash
	}
	return &clearSigner{key, NewECDSAPublicKey(&key.PublicKey), hash}
}

type clearSigner struct {
	key    *ecdsa.PrivateKey
	pubkey PublicKey
	hash   Hash
}

func (c *clearSigner) Sign(message []byte) (sig Signature, err error) {
	r, s, err := ecdsa.Sign(rand.Reader, c.key, cryptoHash(c.hash, message))
	if err != nil {
		return
	}
	sig.R, sig.S, sig.Hash = r.Bytes(), s.Bytes(), c.hash
	return
}

func (c *clearSigner) PublicKey() PublicKey {
	return c.pubkey
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
