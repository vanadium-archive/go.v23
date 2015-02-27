package security

import (
	"bytes"
	"fmt"
	"strings"
)

var (
	// invalidBlessingSubStrings are strings that a blessing extension cannot have as a substring.
	invalidBlessingSubStrings = []string{string(AllPrincipals), ChainSeparator + ChainSeparator /* double slash not allowed */, ","}
	// invalidBlessingExtensions are strings that are disallowed as blessing extensions.
	// TODO(ataly, ashankar): Add more reserved characters/strings to this list. Note that
	// we need to be careful that none of these invalid strings are valid email addresses.
	invalidBlessingExtensions = []string{string(NoExtension)}
)

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

func (c *Certificate) digest(hash Hash, parent Signature) []byte {
	var fields []byte
	w := func(data []byte) {
		fields = append(fields, hash.sum(data)...)
	}
	// Fields of the Certificate type.
	w([]byte(c.Extension))
	w(c.PublicKey)
	for _, cav := range c.Caveats {
		fields = append(fields, cav.digest(hash)...)
	}
	// Bind to the parent Certificate by including parent certificate's signature.
	w([]byte(parent.Hash))
	w(parent.Purpose)
	w([]byte("ECDSA")) // Type of signature
	w(parent.R)
	w(parent.S)
	return hash.sum(fields)
}

func (c *Certificate) validate(parentSignature Signature, parentKey PublicKey) error {
	if !bytes.Equal(c.Signature.Purpose, blessPurpose) {
		return fmt.Errorf("signature on certificate(for %q) was not intended for certification (purpose=%v)", c.Extension, c.Signature.Purpose)
	}
	if err := validateExtension(c.Extension); err != nil {
		return fmt.Errorf("invalid blessing extension in certificate(for %q): %v", c.Extension, err)
	}
	if !c.Signature.Verify(parentKey, c.digest(c.Signature.Hash, parentSignature)) {
		return fmt.Errorf("invalid Signature in certificate(for %q), signing key: %v", c.Extension, parentKey)
	}
	return nil
}

func (c *Certificate) sign(signer Signer, parentSignature Signature) error {
	var err error
	c.Signature, err = signer.Sign(blessPurpose, c.digest(signer.PublicKey().hash(), parentSignature))
	return err
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
	for _, n := range invalidBlessingExtensions {
		if extension == n {
			return fmt.Errorf("invalid blessing extension(%q)", extension)
		}
	}
	for _, n := range invalidBlessingSubStrings {
		if strings.Contains(extension, n) {
			return fmt.Errorf("invalid blessing extension(%q has %q as a substring)", extension, n)
		}
	}
	return nil
}
