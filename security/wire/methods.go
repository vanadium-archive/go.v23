package wire

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"

	"veyron2/security"
)

var (
	// ErrNoIntegrity is the error returned when bytes of an object seem to have been tampered with.
	ErrNoIntegrity = errors.New("signature does not match bytes, possible tampering")

	// TODO(ataly, ashankar): Make sure we add all reserved characters to this list.
	invalidBlessingStrs = []string{string(security.AllPrincipals), security.ChainSeparator}
)

// errInvalidBlessingName returns an error specifying that the provided blessing name is invalid.
func errInvalidBlessingName(blessingName string) error {
	return fmt.Errorf("invalid blessing name:%q", blessingName)
}

// errInvalidPattern returns an error specifying that the provided BlessingPattern is invalid.
func errInvalidPattern(pattern security.BlessingPattern) error {
	return fmt.Errorf("invalid blessing pattern:%q", pattern)
}

// WriteString writes the length and contents of the provided string to the provided Writer.
func WriteString(w io.Writer, tmp []byte, s string) {
	w.Write(tmp[:binary.PutVarint(tmp, int64(len(s)))])
	io.WriteString(w, s)
}

// WriteBytes writes the length and contents of the provided byte slice to the provided Writer.
func WriteBytes(w io.Writer, tmp, b []byte) {
	w.Write(tmp[:binary.PutVarint(tmp, int64(len(b)))])
	w.Write(b)
}

func ellipticCurve(t KeyCurve) (elliptic.Curve, error) {
	switch t {
	case KeyCurveP256:
		return elliptic.P256(), nil
	default:
		return nil, fmt.Errorf("unrecognized elliptic curve %v", t)
	}
}

// Decode unmarshals the contents of the PublicKey object and returns
// a security.PublicKey object.
func (p *PublicKey) Decode() (security.PublicKey, error) {
	curve, err := ellipticCurve(p.Curve)
	if err != nil {
		return nil, err
	}
	x, y := elliptic.Unmarshal(curve, p.XY)
	return security.NewECDSAPublicKey(&ecdsa.PublicKey{Curve: curve, X: x, Y: y}), nil
}

// Encode takes a security.PublicKey object, marshals its contents
// and populates the PublicKey object with them.
func (p *PublicKey) Encode(pk security.PublicKey) error {
	key := pk.DO_NOT_USE()
	if key.Curve != elliptic.P256() {
		return fmt.Errorf("unrecognized elliptic curve %T", p.Curve)
	}
	p.Curve = KeyCurveP256
	p.XY = elliptic.Marshal(key.Curve, key.X, key.Y)
	return nil
}

// -- Helper methods on the wire format for the chain implementation of Identity --

// contentHash returns a SHA256 hash of the contents of the certificate along with the
// provided signature.
func (c *Certificate) contentHash(issuerSignature security.Signature) []byte {
	h := sha256.New()
	tmp := make([]byte, binary.MaxVarintLen64)
	if issuerSignature.Hash != security.NoHash {
		WriteBytes(h, tmp, []byte(issuerSignature.Hash))
	}
	WriteBytes(h, tmp, issuerSignature.R)
	WriteBytes(h, tmp, issuerSignature.S)
	WriteString(h, tmp, c.Name)
	h.Write([]byte{byte(c.PublicKey.Curve)})
	WriteBytes(h, tmp, c.PublicKey.XY)
	binary.Write(h, binary.BigEndian, uint32(len(c.Caveats)))
	for _, cav := range c.Caveats {
		WriteString(h, tmp, string(cav.Service))
		WriteBytes(h, tmp, cav.Bytes)
	}
	return h.Sum(nil)
}

// Sign uses the given Signer to sign the signature of the last certificate in
// the provided PublicID, storing the new signature in the current certificate.
func (c *Certificate) Sign(signer security.PrivateID, pubID *ChainPublicID) error {
	numCerts := len(pubID.Certificates)
	if numCerts == 0 {
		return errors.New("cannot sign a ChainPublicID with no certificates")
	}
	var err error
	c.Signature, err = signer.Sign(c.contentHash(pubID.Certificates[numCerts-1].Signature))
	return err
}

func (c *Certificate) verify(issuerSignature security.Signature, key security.PublicKey) bool {
	return c.Signature.Verify(key, c.contentHash(issuerSignature))
}

// Name returns the chained name obtained by joining all names along the ChainPublicID's
// certificate chain.
func (id *ChainPublicID) Name() string {
	var buf bytes.Buffer
	for i, c := range id.Certificates {
		if i > 0 {
			buf.WriteString(security.ChainSeparator)
		}
		buf.WriteString(c.Name)
	}
	return buf.String()
}

// VerifyIntegrity verifies that the ChainPublicID has a valid certificate chain, i.e,
// (1) each certificate on the chain has a signature that can be verified using the
// public key specified in the previous certificate, (2) the first certificate's
// signature can be verified using its own public key, and (3) all certificate names
// are valid blessing names.
func (id *ChainPublicID) VerifyIntegrity() error {
	nCerts := len(id.Certificates)
	if nCerts == 0 {
		return ErrNoIntegrity
	}
	verificationKey, err := id.Certificates[0].PublicKey.Decode()
	if err != nil {
		return ErrNoIntegrity
	}
	issuerSignature := security.Signature{}
	for _, c := range id.Certificates {
		if err := ValidateBlessingName(c.Name); err != nil {
			return err
		}
		// TODO(ashankar, ataly): Do we worry about timing attacks by
		// early exiting on invalid certificate?
		if !c.verify(issuerSignature, verificationKey) {
			return ErrNoIntegrity
		}
		if verificationKey, err = c.PublicKey.Decode(); err != nil {
			return ErrNoIntegrity
		}
		issuerSignature = c.Signature
	}
	return nil
}

// ValidateBlessingName verifies if the provided name is fit to be the name of a blessing.
func ValidateBlessingName(name string) error {
	if name == "" {
		return errInvalidBlessingName(name)
	}
	for _, s := range invalidBlessingStrs {
		if strings.Contains(name, s) {
			return errInvalidBlessingName(name)
		}
	}
	return nil
}

// ValidateBlessingPattern verifies if the provided security.BlessingPattern is valid.
func ValidateBlessingPattern(pattern security.BlessingPattern) error {
	patternParts := strings.Split(string(pattern), security.ChainSeparator)
	for i, p := range patternParts {
		if (p == string(security.AllPrincipals)) && (i == len(patternParts)-1) {
			break
		}
		if ValidateBlessingName(p) != nil {
			return errInvalidPattern(pattern)
		}
	}
	return nil
}
