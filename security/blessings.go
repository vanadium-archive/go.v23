package security

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"veyron.io/veyron/veyron2/vom"
)

type blessingsImpl struct {
	chains    [][]Certificate
	publicKey PublicKey
}

func (b *blessingsImpl) ForContext(ctx Context) []string {
	var ret []string
	for _, chain := range b.chains {
		if blessing := blessingForCertificateChain(ctx, chain); blessing != "" {
			ret = append(ret, blessing)
		}
	}
	return ret
}
func (b *blessingsImpl) PublicKey() PublicKey { return b.publicKey }
func (b *blessingsImpl) ThirdPartyCaveats() []ThirdPartyCaveat {
	var ret []ThirdPartyCaveat
	var tpc ThirdPartyCaveat
	for _, chain := range b.chains {
		for _, cert := range chain {
			for _, cav := range cert.Caveats {
				if err := vom.NewDecoder(bytes.NewBuffer(cav.ValidatorVOM)).Decode(&tpc); err == nil && tpc != nil {
					ret = append(ret, tpc)
				}
			}
		}
	}
	return ret
}
func (b *blessingsImpl) publicKeyDER() []byte {
	chain := b.chains[0]
	return chain[len(chain)-1].PublicKey
}
func (b *blessingsImpl) certificateChains() [][]Certificate { return b.chains }

func (b *blessingsImpl) String() string {
	blessings := make([]string, len(b.chains))
	for chainidx, chain := range b.chains {
		onechain := make([]string, len(chain))
		ncaveats := 0
		for certidx, cert := range chain {
			ncaveats += len(cert.Caveats)
			onechain[certidx] = cert.Extension
		}
		blessings[chainidx] = fmt.Sprintf("%v(%d caveats)", strings.Join(onechain, ChainSeparator), ncaveats)
	}
	return strings.Join(blessings, "#")
}

// TODO(toddw,ashankar): See comment for VomDecode
func (b *blessingsImpl) VomEncode() ([][]Certificate, error) {
	return b.chains, nil
}

// TODO(toddw,ashankar): When vom2 and config file support for VDL is
// in place, then get rid of VomEncode and VomDecode from here and instead
// in the config file for types.vdl specify a factory function that will
// convert between the "wire" type ([][]Certificate) and the "in-memory"
// type (Blessings=blessingsImpl) via factory functions that will do the
// integrity checks.
func (b *blessingsImpl) VomDecode(certchains [][]Certificate) error {
	if len(certchains) == 0 {
		return errors.New("empty certificate chain")
	}
	// Public keys should match for all chains.
	marshaledkey := certchains[0][len(certchains[0])-1].PublicKey
	key, err := validateCertificateChain(certchains[0])
	if err != nil {
		return err
	}
	for i := 1; i < len(certchains); i++ {
		chain := certchains[i]
		cert := chain[len(chain)-1]
		if !bytes.Equal(marshaledkey, cert.PublicKey) {
			return errors.New("invalid blessings: two certificate chains that bind to different public keys")
		}
		if _, err := validateCertificateChain(chain); err != nil {
			return err
		}
	}
	b.chains = certchains
	b.publicKey = key
	return nil
}

func validateCertificateChain(chain []Certificate) (PublicKey, error) {
	parent := &Signature{}
	key, err := UnmarshalPublicKey(chain[0].PublicKey)
	if err != nil {
		return nil, err
	}
	for idx, cert := range chain {
		if err := cert.validate(*parent, key); err != nil {
			return nil, err
		}
		if key, err = UnmarshalPublicKey(cert.PublicKey); err != nil {
			return nil, err
		}
		parent = &(chain[idx].Signature)
	}
	return key, nil
}

func blessingForCertificateChain(ctx Context, chain []Certificate) string {
	// TODO(ashankar,ataly): Incorporate trust in the root certificate
	if chain[0].Extension == "" {
		return ""
	}
	for _, cert := range chain {
		for _, cav := range cert.Caveats {
			var validator CaveatValidator
			if err := vom.NewDecoder(bytes.NewBuffer(cav.ValidatorVOM)).Decode(&validator); err != nil {
				// Failed to decode CaveatValidator, cannot validate caveat, so this chain means nothing.
				return ""
			}
			if err := validator.Validate(ctx); err != nil {
				return ""
			}
		}
	}
	// All caveats have been validated, construct the blessing name.
	ret := chain[0].Extension
	for i := 1; i < len(chain); i++ {
		ret += ChainSeparator
		ret += chain[i].Extension
	}
	return ret
}

// UnionOfBlessings returns a Blessings object that carries the union of the
// provided blessings.
//
// All provided Blessings must have the same PublicKey.
//
// UnionOfBlessings with no arguments returns (nil, nil).
func UnionOfBlessings(blessings ...Blessings) (Blessings, error) {
	switch len(blessings) {
	case 0:
		return nil, nil
	case 1:
		return blessings[0], nil
	}
	key0 := blessings[0].publicKeyDER()
	var ret blessingsImpl
	for idx, b := range blessings {
		if idx > 0 && !bytes.Equal(key0, b.publicKeyDER()) {
			return nil, errors.New("mismatched public keys")
		}
		ret.chains = append(ret.chains, b.certificateChains()...)
	}
	var err error
	if ret.publicKey, err = UnmarshalPublicKey(key0); err != nil {
		return nil, err
	}
	return &ret, nil
}
