package security

import (
	"crypto/ecdsa"
	"crypto/md5"
	"crypto/x509"
	"encoding"
	"fmt"
)

// PublicKey represents a public key using an unspecified algorithm.
//
// MarshalBinary returns the DER-encoded PKIX representation of the public key,
// while UnmarshalPublicKey creates a PublicKey object from the marshaled bytes.
//
// String returns a human-readable representation of the public key.
type PublicKey interface {
	encoding.BinaryMarshaler
	fmt.Stringer

	implementationsOnlyInThisPackage()

	// TODO(ashankar): Transitional method that should be removed once the "wire"
	// package is removed prior to the 0.1 release.
	// At the time this comment was written, the only clients for this method
	// were in packages/types that are intended to be removed.
	DO_NOT_USE() *ecdsa.PublicKey
}

type ecdsaPublicKey struct {
	key *ecdsa.PublicKey
}

func (pk *ecdsaPublicKey) MarshalBinary() ([]byte, error)    { return x509.MarshalPKIXPublicKey(pk.key) }
func (pk *ecdsaPublicKey) String() string                    { return publicKeyString(pk) }
func (pk *ecdsaPublicKey) DO_NOT_USE() *ecdsa.PublicKey      { return pk.key }
func (pk *ecdsaPublicKey) implementationsOnlyInThisPackage() {}

func publicKeyString(pk PublicKey) string {
	bytes, err := pk.MarshalBinary()
	if err != nil {
		return fmt.Sprintf("<invalid public key: %v>", err)
	}
	const hextable = "0123456789abcdef"
	hash := md5.Sum(bytes)
	var repr [md5.Size * 3]byte
	for i, v := range hash {
		repr[i*3] = hextable[v>>4]
		repr[i*3+1] = hextable[v&0x0f]
		repr[i*3+2] = ':'
	}
	return string(repr[:len(repr)-1])
}

// UnmarshalPublicKey returns a PublicKey object from the DER-encoded PKIX represntation of it
// (typically obtianed via PublicKey.MarshalBinary).
func UnmarshalPublicKey(bytes []byte) (PublicKey, error) {
	key, err := x509.ParsePKIXPublicKey(bytes)
	if err != nil {
		return nil, err
	}
	switch v := key.(type) {
	case *ecdsa.PublicKey:
		return &ecdsaPublicKey{v}, nil
	default:
		return nil, fmt.Errorf("unrecognized PublicKey type(%T)", key)
	}
}

// NewECDSAPublicKey creates a PublicKey object that uses the ECDSA algorithm and the provided ECDSA public key.
func NewECDSAPublicKey(key *ecdsa.PublicKey) PublicKey {
	return &ecdsaPublicKey{key}
}
