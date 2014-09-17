package security

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
)

var invalidNames = []string{string(AllPrincipals), ChainSeparator + ChainSeparator /* double slash not allowed */}

func newUnsignedCertificate(extension string, key PublicKey, caveats ...Caveat) (*Certificate, error) {
	err := validateExtension(extension)
	if err != nil {
		return nil, err
	}
	cert := &Certificate{Extension: extension, Caveats: caveats}
	if cert.PublicKey, err = key.MarshalBinary(); err != nil {
		return nil, fmt.Errorf("failed to marshal PublicKey into a Certificate: %v", err)
	}
	return cert, nil
}

// TODO(ashankar): Use a hash function that does not reduce security of the signature.
// in other words, use a hash function that is appropriate given the number of bits
// on the key.
func (c *Certificate) contentHash(parent Signature) ([]byte, error) {
	h := sha256.New()
	var err error
	w := func(data []byte) {
		if err != nil {
			return
		}
		tmp := sha256.Sum256(data)
		_, err = h.Write(tmp[:])
	}
	// Fields of the Certificate type.
	w([]byte(c.Extension))
	w(c.PublicKey)
	for _, cav := range c.Caveats {
		w(cav.ValidatorVOM)
	}
	// Bind to the parent Certificate by including parent certificate's signature.
	w([]byte(parent.Hash))
	w(parent.Purpose)
	w([]byte("ECDSA")) // Type of signature
	w(parent.R)
	w(parent.S)
	if err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func (c *Certificate) validate(parentSignature Signature, parentKey PublicKey) error {
	if !bytes.Equal(c.Signature.Purpose, blessPurpose) {
		return fmt.Errorf("signature on certificate(for %q) was not intended for certification (purpose=%v)", c.Extension, c.Signature.Purpose)
	}
	if err := validateExtension(c.Extension); err != nil {
		return fmt.Errorf("invalid blessing extension in certificate(for %q): %v", c.Extension, err)
	}
	hash, err := c.contentHash(parentSignature)
	if err != nil {
		return fmt.Errorf("unable to compute content hash of certificate(for %q): %v", c.Extension, err)
	}
	if !c.Signature.Verify(parentKey, hash) {
		return fmt.Errorf("invalid Signature in certificate(for %q), signing key: %v", c.Extension, parentKey)
	}
	return nil
}

func validateExtension(extension string) error {
	if len(extension) == 0 {
		return fmt.Errorf("invalid blessing extension(empty string)")
	}
	if strings.HasPrefix(extension, ChainSeparator) {
		return fmt.Errorf("invalid blessing extension(starts with %q)", ChainSeparator)
	}
	if strings.HasSuffix(extension, ChainSeparator) {
		return fmt.Errorf("invalid blessing extension(ends with %q)", ChainSeparator)
	}
	for _, n := range invalidNames {
		if strings.Contains(extension, n) {
			return fmt.Errorf("invalid blessing extension(%q has %q as a substring)", extension, n)
		}
	}
	return nil
}
