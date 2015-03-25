// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"math/big"
	"v.io/v23/verror"
)

var (
	errSignCantHash = verror.Register(pkgPath+".errSignCantHash", verror.NoRetry, "{1:}{2:}unable to create bytes to sign from message with hashing function {3}{:_}")
)

// Verify returns true iff sig is a valid signature for a message.
func (sig *Signature) Verify(key PublicKey, message []byte) bool {
	if message = messageDigest(sig.Hash, sig.Purpose, message); message == nil {
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
	return &ecdsaSigner{key, NewECDSAPublicKey(&key.PublicKey)}
}

type ecdsaSigner struct {
	key    *ecdsa.PrivateKey
	pubkey PublicKey
}

func (c *ecdsaSigner) Sign(purpose, message []byte) (Signature, error) {
	hash := c.pubkey.hash()
	if message = messageDigest(hash, purpose, message); message == nil {
		return Signature{}, verror.New(errSignCantHash, nil, hash)
	}
	r, s, err := ecdsa.Sign(rand.Reader, c.key, message)
	if err != nil {
		return Signature{}, err
	}
	return Signature{
		Purpose: purpose,
		Hash:    hash,
		R:       r.Bytes(),
		S:       s.Bytes(),
	}, nil
}

func (c *ecdsaSigner) PublicKey() PublicKey {
	return c.pubkey
}

func messageDigest(hash Hash, purpose, message []byte) []byte {
	if message = hash.sum(message); message == nil {
		return nil
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
