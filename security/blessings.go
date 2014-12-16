package security

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"

	"veyron.io/veyron/veyron2/vlog"
	"veyron.io/veyron/veyron2/vom"
)

var errEmptyChain = errors.New("empty certificate chain found")

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
func (b *blessingsImpl) blessingsByNameForPrincipal(p Principal, pattern BlessingPattern) Blessings {
	var ret blessingsImpl
	for _, chain := range b.chains {
		blessing := nameForPrincipal(p, chain)
		if len(blessing) > 0 && pattern.MatchedBy(blessing) {
			ret.chains = append(ret.chains, chain)
		}
	}
	if len(ret.chains) > 0 {
		ret.publicKey = b.publicKey
		return &ret
	}
	return nil
}
func (b *blessingsImpl) String() string {
	blessings := make([]string, len(b.chains))
	for chainidx, chain := range b.chains {
		onechain := make([]string, len(chain))
		for certidx, cert := range chain {
			onechain[certidx] = cert.Extension
		}
		blessings[chainidx] = fmt.Sprintf("%v", strings.Join(onechain, ChainSeparator))
	}
	return strings.Join(blessings, "#")
}

func nameForPrincipal(p Principal, chain []Certificate) string {
	// Verify the chain belongs to this principal
	pKey, err := p.PublicKey().MarshalBinary()
	if err != nil || !bytes.Equal(chain[len(chain)-1].PublicKey, pKey) {
		return ""
	}
	blessing := chain[0].Extension
	for i := 1; i < len(chain); i++ {
		blessing += ChainSeparator
		blessing += chain[i].Extension
	}
	// Verify that the root of the chain is recognized as an authority
	// on blessing.
	rootKey, err := UnmarshalPublicKey(chain[0].PublicKey)
	if err != nil {
		return ""
	}
	if p.Roots() == nil {
		return ""
	}
	if err := p.Roots().Recognized(rootKey, blessing); err != nil {
		return ""
	}
	return blessing
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
	blessing := chain[0].Extension
	for i := 1; i < len(chain); i++ {
		blessing += ChainSeparator
		blessing += chain[i].Extension
	}

	// Verify that the root of the chain is recognized as an authority
	// on blessing.
	root, err := UnmarshalPublicKey(chain[0].PublicKey)
	if err != nil {
		vlog.VI(2).Infof("could not extract blessing as PublicKey from root certificate with Extension: %v could not be unmarshaled: %v", chain[0].Extension, err)
		return ""
	}
	local := ctx.LocalPrincipal()
	if local == nil {
		vlog.VI(2).Infof("could not extract blessing as provided Context %v has LocalPrincipal nil", ctx)
		return ""
	}
	if local.Roots() == nil {
		vlog.VI(4).Info("could not extract blessing as no keys are recgonized as valid roots")
		return ""
	}
	if err := local.Roots().Recognized(root, blessing); err != nil {
		vlog.VI(4).Infof("ignoring blessing %v because %v", blessing, err)
		return ""
	}

	// Validate all caveats embedded in the chain.
	for _, cert := range chain {
		for _, cav := range cert.Caveats {
			var validator CaveatValidator
			if err := vom.NewDecoder(bytes.NewBuffer(cav.ValidatorVOM)).Decode(&validator); err != nil {
				vlog.VI(4).Infof("ignoring blessing %v because CaveatValidator decoding failed: %v", blessing, err)
				return ""
			}
			if err := validator.Validate(ctx); err != nil {
				vlog.VI(4).Infof("ignoring blessing %v because caveat %T failed valdiation: %v", blessing, validator, err)
				return ""
			}
		}
	}
	return blessing
}

// NewBlessings creates a Blessings object from the provided wire representation.
func NewBlessings(wire WireBlessings) (Blessings, error) {
	if len(wire.CertificateChains) == 0 {
		return nil, nil
	}
	var b blessingsImpl
	certchains := wire.CertificateChains
	if len(certchains) == 0 || len(certchains[0]) == 0 {
		return nil, errEmptyChain
	}
	// Public keys should match for all chains.
	marshaledkey := certchains[0][len(certchains[0])-1].PublicKey
	key, err := validateCertificateChain(certchains[0])
	if err != nil {
		return nil, err
	}
	for i := 1; i < len(certchains); i++ {
		chain := certchains[i]
		if len(chain) == 0 {
			return nil, errEmptyChain
		}
		cert := chain[len(chain)-1]
		if !bytes.Equal(marshaledkey, cert.PublicKey) {
			return nil, errors.New("invalid blessings: two certificate chains that bind to different public keys")
		}
		if _, err := validateCertificateChain(chain); err != nil {
			return nil, err
		}
	}
	b.chains = certchains
	b.publicKey = key
	return &b, nil
}

// MarshalBlessings is the inverse of NewBlessings, converting an in-memory
// representation of Blessings to the wire format.
func MarshalBlessings(b Blessings) WireBlessings {
	if b == nil {
		return WireBlessings{}
	}
	return WireBlessings{CertificateChains: b.certificateChains()}
}

// UnionOfBlessings returns a Blessings object that carries the union of the
// provided blessings.
//
// All provided Blessings must have the same PublicKey.
//
// UnionOfBlessings with no arguments returns (nil, nil).
func UnionOfBlessings(blessings ...Blessings) (Blessings, error) {
	for len(blessings) > 0 && blessings[0] == nil {
		blessings = blessings[1:]
	}
	switch len(blessings) {
	case 0:
		return nil, nil
	case 1:
		return blessings[0], nil
	}
	key0 := blessings[0].publicKeyDER()
	var ret blessingsImpl
	for idx, b := range blessings {
		if b == nil {
			continue
		}
		if idx > 0 && !bytes.Equal(key0, b.publicKeyDER()) {
			return nil, errors.New("mismatched public keys")
		}
		ret.chains = append(ret.chains, b.certificateChains()...)
	}
	var err error
	if ret.publicKey, err = UnmarshalPublicKey(key0); err != nil {
		return nil, err
	}
	// For pretty printing, sort the certificate chains so that there is a consistent
	// ordering, irrespective of the ordering of arugments to UnionOfBlessings.
	sort.Stable(certificateChainsSorter(ret.chains))
	return &ret, nil
}

type certificateChainsSorter [][]Certificate

func (c certificateChainsSorter) Len() int      { return len(c) }
func (c certificateChainsSorter) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c certificateChainsSorter) Less(i, j int) bool {
	ci := c[i]
	cj := c[j]
	if len(ci) < len(cj) {
		return true
	}
	if len(ci) > len(cj) {
		return false
	}
	// Equal size chains, order by the names in the certificates.
	N := len(ci)
	for idx := 0; idx < N; idx++ {
		ie := ci[idx].Extension
		je := cj[idx].Extension
		if ie < je {
			return true
		}
		if ie > je {
			return false
		}
	}
	return false
}
