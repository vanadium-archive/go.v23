package security

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"math/big"
)

// Verify returns true iff sig is a valid signature for a message.
func (sig *Signature) Verify(key PublicKey, message []byte) bool {
	if message = messageToSign(sig.Hash, sig.Purpose, message); message == nil {
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
	return &ecdsaSigner{key, NewECDSAPublicKey(&key.PublicKey), hash}
}

type ecdsaSigner struct {
	key    *ecdsa.PrivateKey
	pubkey PublicKey
	hash   Hash
}

func (c *ecdsaSigner) Sign(purpose, message []byte) (Signature, error) {
	if message = messageToSign(c.hash, purpose, message); message == nil {
		return Signature{}, fmt.Errorf("unable to create bytes to sign from message with hashing function %q", c.hash)
	}
	r, s, err := ecdsa.Sign(rand.Reader, c.key, message)
	if err != nil {
		return Signature{}, err
	}
	return Signature{
		Purpose: purpose,
		Hash:    c.hash,
		R:       r.Bytes(),
		S:       s.Bytes(),
	}, nil
}

func (c *ecdsaSigner) PublicKey() PublicKey {
	return c.pubkey
}

func messageToSign(hash Hash, purpose, message []byte) []byte {
	if message = hash.sum(message); message == nil {
		return nil
	}
	// TODO(ashankar): Remove this when switching to the new security API that actually sets the Purpose field.
	// Kept for backward compatibility till the switch is made.
	if purpose == nil {
		return hash.sum(message)
	}
	if purpose = hash.sum(purpose); purpose == nil {
		return nil
	}
	return hash.sum(append(message, purpose...))
}

// sum returns the hash of data using hash as the cryptographic hash function.
// Returns nil if data is nil or hash is not recognized.
func (hash Hash) sum(data []byte) []byte {
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
