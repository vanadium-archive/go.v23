// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"bytes"
	"strings"

	"v.io/v23/verror"
)

var (
	errCantMarshalKey             = verror.Register(pkgPath+".errCantMarshalKey", verror.NoRetry, "{1:}{2:}failed to marshal PublicKey into a Certificate{:_}")
	errInapproriateCertSignature  = verror.Register(pkgPath+".errInapproriateCertSignature", verror.NoRetry, "{1:}{2:}signature on certificate(for {3}) was not intended for certification (purpose={4}){:_}")
	errBadBlessingExtensionInCert = verror.Register(pkgPath+".errBadBlessingExtensionInCert", verror.NoRetry, "{1:}{2:}invalid blessing extension in certificate(for {3}){:_}")
	errBadCertSignature           = verror.Register(pkgPath+".errBadCertSignature", verror.NoRetry, "{1:}{2:}invalid Signature in certificate(for \"{3}\"), signing key{:_}")
	errBadBlessingEmptyExtension  = verror.Register(pkgPath+".errBadBlessingEmptyExtension", verror.NoRetry, "{1:}{2:}invalid blessing extension(empty string){:_}")
	errBadBlessingBadStart        = verror.Register(pkgPath+".errBadBlessingBadStart", verror.NoRetry, "{1:}{2:}invalid blessing extension(starts with {3}){:_}")
	errBadBlessingBadEnd          = verror.Register(pkgPath+".errBadBlessingBadEnd", verror.NoRetry, "{1:}{2:}invalid blessing extension(ends with {3}){:_}")
	errBadBlessingExtension       = verror.Register(pkgPath+".errBadBlessingExtension", verror.NoRetry, "{1:}{2:}invalid blessing extension({3}){:_}")
	errBadBlessingBadSubstring    = verror.Register(pkgPath+".errBadBlessingBadSubstring", verror.NoRetry, "{1:}{2:}invalid blessing extension({3} has {4} as a substring){:_}")
)

var (
	// invalidBlessingSubStrings are strings that a blessing extension cannot have as a substring.
	invalidBlessingSubStrings = []string{string(AllPrincipals), ChainSeparator + ChainSeparator /* double slash not allowed */, ",", "@@", "(", ")", "<", ">"}
	// invalidBlessingExtensions are strings that are disallowed as blessing extensions.
	invalidBlessingExtensions = []string{string(NoExtension)}
)

func newUnsignedCertificate(extension string, key PublicKey, caveats ...Caveat) (*Certificate, error) {
	err := validateExtension(extension)
	if err != nil {
		return nil, err
	}
	cert := &Certificate{Extension: extension, Caveats: caveats}
	if cert.PublicKey, err = key.MarshalBinary(); err != nil {
		return nil, verror.New(errCantMarshalKey, nil, err)
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
		return verror.New(errInapproriateCertSignature, nil, c.Extension, c.Signature.Purpose)
	}
	if err := validateExtension(c.Extension); err != nil {
		return verror.New(errBadBlessingExtensionInCert, nil, c.Extension, err)
	}
	if !c.Signature.Verify(parentKey, c.digest(c.Signature.Hash, parentSignature)) {
		return verror.New(errBadCertSignature, nil, c.Extension, parentKey)
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
		return verror.New(errBadBlessingEmptyExtension, nil)
	}
	if strings.HasPrefix(extension, ChainSeparator) {
		return verror.New(errBadBlessingBadStart, nil, ChainSeparator)
	}
	if strings.HasSuffix(extension, ChainSeparator) {
		return verror.New(errBadBlessingBadEnd, nil, ChainSeparator)
	}
	for _, n := range invalidBlessingExtensions {
		if extension == n {
			return verror.New(errBadBlessingExtension, nil, extension)
		}
	}
	for _, n := range invalidBlessingSubStrings {
		if strings.Contains(extension, n) {
			return verror.New(errBadBlessingBadSubstring, nil, extension, n)
		}
	}
	return nil
}
