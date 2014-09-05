package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"testing"
)

func newSigner(hash Hash) *signer {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic("couldn't generate private ECDSA key")
	}
	return &signer{
		hash: hash,
		key:  key,
	}
}

type signer struct {
	hash Hash
	key  *ecdsa.PrivateKey
}

func (sn *signer) Sign(message []byte) (Signature, error) {
	if message = cryptoHash(sn.hash, message); message == nil {
		return Signature{}, fmt.Errorf("couldn't sign message")
	}
	r, s, err := ecdsa.Sign(rand.Reader, sn.key, message)
	if err != nil {
		return Signature{}, err
	}
	return Signature{
		Hash: sn.hash,
		R:    r.Bytes(),
		S:    s.Bytes(),
	}, nil
}

func (sn *signer) PublicKey() PublicKey {
	return &ecdsaPublicKey{&sn.key.PublicKey}
}

func TestVerify(t *testing.T) {
	testcases := [][]byte{
		[]byte{},
		[]byte("iwannaruniwannhide"),
	}
	signers := []*signer{
		newSigner(NoHash),
		newSigner(SHA1Hash),
		newSigner(SHA256Hash),
		newSigner(SHA384Hash),
		newSigner(SHA512Hash),
	}
	for _, testcase := range testcases {
		for _, s := range signers {
			sig, err := s.Sign(testcase)
			if err != nil {
				t.Errorf("couldn't sign message %q using signer %q: %v", testcase, s.hash, err)
				continue
			}
			if !sig.Verify(s.PublicKey(), testcase) {
				t.Errorf("couldn't verify signature for testcase %s", testcase)
			}
		}
	}
}
