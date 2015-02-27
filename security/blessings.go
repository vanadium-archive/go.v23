package security

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"v.io/v23/vlog"
)

var errEmptyChain = errors.New("empty certificate chain found")

// Blessings encapsulates all the cryptographic operations required to
// prove that a set of blessings (human-readable strings) have been bound
// to a principal in a specific context.
//
// Blessings objects are meant to be presented to other principals to authenticate
// and authorize actions.
//
// Blessings objects are immutable and multiple goroutines may invoke methods
// on them simultaneously.
type Blessings struct {
	chains    [][]Certificate
	publicKey PublicKey
}

const chainValidatorKey = "customChainValidator"

// ForContext returns a validated set of (human-readable string) blessings
// presented by the principal. These returned blessings (strings) are
// guaranteed to:
//
// (1) Satisfy all the caveats given context
// (2) Be rooted in context.LocalPrincipal.Roots.
//
// Caveats are considered satisfied in the given context if the CaveatValidator
// implementation can be found in the address space of the caller and Validate
// returns nil.
//
// ForContext also returns the RejectedBlessings for each blessing that cannot
// be validated.
func (b Blessings) ForContext(ctx Context) (ret []string, info []RejectedBlessing) {
	if b.IsZero() {
		return nil, nil
	}
	validator := defaultChainCaveatValidator
	if customValidator := ctx.Context().Value(chainValidatorKey); customValidator != nil {
		validator = customValidator.(func(ctx Context, chains [][]Caveat) []error)
	}

	blessings := []string{}
	chainCaveats := [][]Caveat{}
	for _, chain := range b.chains {
		blessing, err := verifyChainSignature(ctx, chain)
		if err != nil {
			info = append(info, RejectedBlessing{blessing, err})
			continue
		}

		cavs := []Caveat{}
		for _, cert := range chain {
			cavs = append(cavs, cert.Caveats...)
		}

		if len(cavs) == 0 {
			ret = append(ret, blessing) // No caveats to validate, add it to blessing list.
		} else {
			chainCaveats = append(chainCaveats, cavs)
			blessings = append(blessings, blessing)
		}
	}

	if len(chainCaveats) == 0 {
		return // Skip the validation call (is high-overhead for javascript).
	}

	validationResult := validator(ctx, chainCaveats)
	if len(validationResult) != len(blessings) {
		panic(fmt.Sprintf("Got wrong number of validation results. Got %d, expected %d.", len(validationResult), len(blessings)))
	}

	for i, resultErr := range validationResult {
		if resultErr == nil {
			ret = append(ret, blessings[i])
		} else {
			info = append(info, RejectedBlessing{blessings[i], resultErr})
		}
	}
	return
}

// PublicKey returns the public key of the principal to which
// blessings obtained from this object are bound.
//
// Can return nil if b is the zero value.
func (b Blessings) PublicKey() PublicKey { return b.publicKey }

// ThirdPartyCaveats returns the set of third-party restrictions on the
// scope of the blessings (i.e., the subset of Caveats for which
// ThirdPartyDetails will be non-nil).
func (b Blessings) ThirdPartyCaveats() []Caveat {
	var ret []Caveat
	for _, chain := range b.chains {
		for _, cert := range chain {
			for _, cav := range cert.Caveats {
				if tp := cav.ThirdPartyDetails(); tp != nil {
					ret = append(ret, cav)
				}
			}
		}
	}
	return ret
}

// IsZero returns true if b represents the zero value of blessings (an empty
// set).
func (b Blessings) IsZero() bool {
	// b.publicKey == nil <=> len(b.chains) == 0
	return b.publicKey == nil
}

func (b Blessings) publicKeyDER() []byte {
	chain := b.chains[0]
	return chain[len(chain)-1].PublicKey
}
func (b Blessings) certificateChains() [][]Certificate { return b.chains }
func (b Blessings) blessingsByNameForPrincipal(p Principal, pattern BlessingPattern) Blessings {
	ret := Blessings{publicKey: b.publicKey}
	for _, chain := range b.chains {
		blessing := nameForPrincipal(p, chain)
		if len(blessing) > 0 && pattern.MatchedBy(blessing) {
			ret.chains = append(ret.chains, chain)
		}
	}
	if len(ret.chains) == 0 {
		return Blessings{}
	}
	return ret
}
func (b Blessings) String() string {
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

// Verifies that the chain signatures are correct, without handling caveat validation
func verifyChainSignature(ctx Context, chain []Certificate) (string, error) {
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
		return blessing, err
	}
	local := ctx.LocalPrincipal()
	if local == nil {
		vlog.VI(2).Infof("could not extract blessing as provided Context %v has LocalPrincipal nil", ctx)
		return blessing, NewErrUntrustedRoot(nil, blessing)
	}
	if local.Roots() == nil {
		vlog.VI(4).Info("could not extract blessing as no keys are recgonized as valid roots")
		return blessing, NewErrUntrustedRoot(nil, blessing)
	}
	if err := local.Roots().Recognized(root, blessing); err != nil {
		vlog.VI(4).Infof("ignoring blessing %v because %v", blessing, err)
		return blessing, NewErrUntrustedRoot(nil, blessing)
	}

	return blessing, nil
}

func defaultChainCaveatValidator(ctx Context, chains [][]Caveat) []error {
	results := make([]error, len(chains))
	for i, chain := range chains {
		for _, cav := range chain {
			if err := validateCaveat(ctx, cav); err != nil {
				vlog.VI(4).Infof("Ignoring chain %v: %v", chain, err)
				results[i] = err
				break
			}
		}
	}

	return results
}

// validateCaveat is pretty much the same as cav.Validate(ctx), but it defers
// to any validation scheme overrides via SetCaveatValidator.
func validateCaveat(ctx Context, cav Caveat) error {
	caveatValidationSetup.mu.RLock()
	fn := caveatValidationSetup.fn
	finalized := caveatValidationSetup.finalized
	caveatValidationSetup.mu.RUnlock()
	if !finalized {
		caveatValidationSetup.mu.Lock()
		caveatValidationSetup.finalized = true
		fn = caveatValidationSetup.fn
		caveatValidationSetup.mu.Unlock()
	}
	if fn != nil {
		return fn(ctx, cav)
	}
	return cav.Validate(ctx)
}

// NewBlessings creates a Blessings object from the provided wire representation.
//
// TODO(ashankar): Remove NewBlessings and MarshalBlessings and instead use the
// VOM/VDL native<->wire conversion mechanism.
func NewBlessings(wire WireBlessings) (Blessings, error) {
	var b Blessings
	if len(wire.CertificateChains) == 0 {
		return b, nil
	}
	certchains := wire.CertificateChains
	if len(certchains) == 0 || len(certchains[0]) == 0 {
		return b, errEmptyChain
	}
	// Public keys should match for all chains.
	marshaledkey := certchains[0][len(certchains[0])-1].PublicKey
	key, err := validateCertificateChain(certchains[0])
	if err != nil {
		return b, err
	}
	for i := 1; i < len(certchains); i++ {
		chain := certchains[i]
		if len(chain) == 0 {
			return b, errEmptyChain
		}
		cert := chain[len(chain)-1]
		if !bytes.Equal(marshaledkey, cert.PublicKey) {
			return b, errors.New("invalid blessings: two certificate chains that bind to different public keys")
		}
		if _, err := validateCertificateChain(chain); err != nil {
			return b, err
		}
	}
	b.chains = certchains
	b.publicKey = key
	return b, nil
}

// MarshalBlessings is the inverse of NewBlessings, converting an in-memory
// representation of Blessings to the wire format.
func MarshalBlessings(b Blessings) WireBlessings {
	return WireBlessings{CertificateChains: b.certificateChains()}
}

// UnionOfBlessings returns a Blessings object that carries the union of the
// provided blessings.
//
// All provided Blessings must have the same PublicKey.
//
// UnionOfBlessings with no arguments returns (nil, nil).
func UnionOfBlessings(blessings ...Blessings) (Blessings, error) {
	for len(blessings) > 0 && blessings[0].IsZero() {
		blessings = blessings[1:]
	}
	switch len(blessings) {
	case 0:
		return Blessings{}, nil
	case 1:
		return blessings[0], nil
	}
	key0 := blessings[0].publicKeyDER()
	var ret Blessings
	for idx, b := range blessings {
		if b.IsZero() {
			continue
		}
		if idx > 0 && !bytes.Equal(key0, b.publicKeyDER()) {
			return Blessings{}, errors.New("mismatched public keys")
		}
		ret.chains = append(ret.chains, b.certificateChains()...)
	}
	var err error
	if ret.publicKey, err = UnmarshalPublicKey(key0); err != nil {
		return Blessings{}, err
	}
	// For pretty printing, sort the certificate chains so that there is a consistent
	// ordering, irrespective of the ordering of arugments to UnionOfBlessings.
	sort.Stable(certificateChainsSorter(ret.chains))
	return ret, nil
}

var caveatValidationSetup struct {
	mu        sync.RWMutex
	fn        func(Context, Caveat) error
	finalized bool
}

// SetGlobalCaveatValidator sets the mechanism used for validating Caveats.
//
// This is normally needed only when using this Go library as a library for
// security privimitives and validating Caveats defined in the native language
// (e.g., Java, JavaScript). In such cases, the address space of the Go process
// cannot interpret the caveat data or execute the validation checks, so this
// function is used as a bridge to defer the validation check to the native
// language.
//
// This function can be called at most once, before any caveats have ever been
// validated.
//
// If never invoked, the default Go API is used to associate validation
// functions with caveats.
func SetCaveatValidator(fn func(Context, Caveat) error) {
	caveatValidationSetup.mu.Lock()
	defer caveatValidationSetup.mu.Unlock()
	if caveatValidationSetup.finalized {
		panic("SetCaveatValidator cannot be called multiple times or after this process has checked at least some caveats")
	}
	caveatValidationSetup.finalized = true
	caveatValidationSetup.fn = fn
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

func (i RejectedBlessing) String() string {
	return fmt.Sprintf("{%q: %v}", i.Blessing, i.Err)
}
