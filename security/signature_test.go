package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
)

func TestSignature(t *testing.T) {
	curves := []elliptic.Curve{
		elliptic.P224(),
		elliptic.P256(),
		elliptic.P384(),
		elliptic.P521(),
	}
	for _, curve := range curves {
		nbits := curve.Params().BitSize
		key, err := ecdsa.GenerateKey(curve, rand.Reader)
		if err != nil {
			t.Errorf("Failed to generate key for curve of %d bits: %v", nbits, err)
			continue
		}
		// Sign a message nbits long and then ensure that a larger message does not have the same signature.
		message := make([]byte, nbits>>3)
		signer := NewInMemoryECDSASigner(key)
		sig, err := signer.Sign(message)
		if err != nil {
			t.Errorf("Failed to generate signature with curve of %d bits: %v", nbits, err)
			continue
		}
		if !sig.Verify(signer.PublicKey(), message) {
			t.Errorf("Signature verification failed with curve of %d bits", nbits)
			continue
		}
		message = append(message, 1)
		if sig.Verify(signer.PublicKey(), message) {
			t.Errorf("Signature of message of %d bytes incorrectly verified with curve of %d bits", len(message), nbits)
			continue
		}
		// Sign a message larger than nbits and verify that switching the last byte does not end up with the same signature.
		if sig, err = signer.Sign(message); err != nil {
			t.Errorf("Failed to generate signature with curve of %d bits: %v", nbits, err)
			continue
		}
		if !sig.Verify(signer.PublicKey(), message) {
			t.Errorf("Signature verification failed with curve of %d bits", nbits)
			continue
		}
		message[len(message)-1] = message[len(message)-1] + 1
		if sig.Verify(signer.PublicKey(), message) {
			t.Errorf("Signature of modified message incorrectly verified with curve of %d bits", len(message), nbits)
			continue
		}
		// Signing small messages and then extending them.
		nbytes := (nbits + 7) / 3
		message = make([]byte, 1, nbytes)
		if sig, err = signer.Sign(message); err != nil {
			t.Errorf("Failed to generate signature with curve of %d bits: %v", nbits, err)
			continue
		}
		if !sig.Verify(signer.PublicKey(), message) {
			t.Errorf("Failed to verify signature of small message with curve of %d bits", nbits)
			continue
		}
		for len(message) < nbytes {
			// Extended message should fail verification
			message = append(message, message...)
			if sig.Verify(signer.PublicKey(), message) {
				t.Errorf("Signature of extended message (%d bytes) incorrectly verified with curve of %d bits", len(message), nbits)
				break
			}
		}
	}
}
