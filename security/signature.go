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
	if message = messageDigest(sig.Hash, sig.Purpose, message, key); message == nil {
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

func (sig *Signature) digest(hashfn Hash) []byte {
	var fields []byte
	w := func(data []byte) {
		fields = append(fields, hashfn.sum(data)...)
	}
	w([]byte(sig.Hash))
	w(sig.Purpose)
	w([]byte("ECDSA")) // The signing algorithm
	w(sig.R)
	w(sig.S)
	return hashfn.sum(fields)
}

// NewInMemoryECDSASigner creates a Signer that uses the provided ECDSA private
// key to sign messages.  This private key is kept in the clear in the memory
// of the running process.
func NewInMemoryECDSASigner(key *ecdsa.PrivateKey) Signer {
	sign := func(data []byte) (r, s *big.Int, err error) {
		return ecdsa.Sign(rand.Reader, key, data)
	}
	return NewECDSASigner(&key.PublicKey, sign)
}

// NewECDSASigner creates a Signer that uses the provided function to sign
// messages.
func NewECDSASigner(key *ecdsa.PublicKey, sign func(data []byte) (r, s *big.Int, err error)) Signer {
	return &ecdsaSigner{sign: sign, pubkey: NewECDSAPublicKey(key)}
}

type ecdsaSigner struct {
	sign   func(data []byte) (r, s *big.Int, err error)
	pubkey PublicKey
}

func (c *ecdsaSigner) Sign(purpose, message []byte) (Signature, error) {
	hash := c.pubkey.hash()
	if message = messageDigest(hash, purpose, message, c.pubkey); message == nil {
		return Signature{}, verror.New(errSignCantHash, nil, hash)
	}
	r, s, err := c.sign(message)
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

func messageDigest(hash Hash, purpose, message []byte, key PublicKey) []byte {
	var fields []byte
	w := func(data []byte) bool {
		h := hash.sum(data)
		if h == nil {
			return false
		}
		fields = append(fields, h...)
		return true
	}
	if isV1Purpose(purpose) {
		// In order to defend agains "Duplicate Signature Key Selection (DSKS)" attacks
		// as defined in the paper "Another look at Security Definition" by Neal Koblitz
		// and Alfred Menzes, we also include the public key of the signer in the message
		// being signed.
		keybytes, err := key.MarshalBinary()
		if err != nil {
			return nil
		}
		if !w(keybytes) {
			return nil
		}
	}
	if !w(message) {
		return nil
	}
	if !w(purpose) {
		return nil
	}
	return hash.sum(fields)
}

// sum returns the hash of data using hash as the cryptographic hash function.
// Returns nil if the hash function is not recognized.
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

func isV1Purpose(purpose []byte) bool {
	switch string(purpose) {
	case SignatureForMessageSigningV0, SignatureForBlessingCertificatesV0, SignatureForDischargeV0:
		return false
	default:
		return true
	}
}
