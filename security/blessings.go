package security

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var errEmptyChain = errors.New("empty certificate chain found")

// Blessings encapsulates all the cryptographic operations required to
// prove that a set of (human-readable) blessing names have been bound
// to a principal in a specific call.
//
// Blessings objects are meant to be presented to other principals to authenticate
// and authorize actions. The 'BlessingNames' function in this package can be
// used to uncover the blessing names encapsulated in these objects.
//
// Blessings objects are immutable and multiple goroutines may invoke methods
// on them simultaneously.
type Blessings struct {
	chains    [][]Certificate
	publicKey PublicKey
}

const chainValidatorKey = "customChainValidator"

// HACK, REMOVE BEFORE LAUNCH
type hackCall struct {
	Call
	b Blessings
}

func (c hackCall) RemoteBlessings() Blessings {
	return c.b
}

// DEPECREATED: Use BlessingNames instead
// TODO(ataly, ashankar): Get rid of this method.
func (b Blessings) ForCall(call Call) (ret []string, info []RejectedBlessing) {
	return BlessingNames(hackCall{Call: call, b: b}, CallSideRemote)
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
func verifyChainSignature(call Call, side CallSide, chain []Certificate) (string, error) {
	blessing := chain[0].Extension
	for i := 1; i < len(chain); i++ {
		blessing += ChainSeparator
		blessing += chain[i].Extension
	}

	// Verify that the root of the chain is recognized as an authority
	// on blessing.
	root, err := UnmarshalPublicKey(chain[0].PublicKey)
	if err != nil {
		// TODO(jsimsa): Decide what (if any) logging mechanism to use.
		// vlog.VI(2).Infof("could not extract blessing as PublicKey from root certificate with Extension: %v could not be unmarshaled: %v", chain[0].Extension, err)
		return blessing, err
	}
	local := call.LocalPrincipal()
	if local == nil {
		// TODO(jsimsa): Decide what (if any) logging mechanism to use.
		// vlog.VI(2).Infof("could not extract blessing as provided Context %v has LocalPrincipal nil", call)
		return blessing, NewErrUntrustedRoot(nil, blessing)
	}
	if local.Roots() == nil {
		// TODO(jsimsa): Decide what (if any) logging mechanism to use.
		// vlog.VI(4).Info("could not extract blessing as no keys are recgonized as valid roots")
		return blessing, NewErrUntrustedRoot(nil, blessing)
	}
	if err := local.Roots().Recognized(root, blessing); err != nil {
		// TODO(jsimsa): Decide what (if any) logging mechanism to use.
		// vlog.VI(4).Infof("ignoring blessing %v because %v", blessing, err)
		return blessing, NewErrUntrustedRoot(nil, blessing)
	}

	return blessing, nil
}

func defaultChainCaveatValidator(call Call, side CallSide, chains [][]Caveat) []error {
	results := make([]error, len(chains))
	for i, chain := range chains {
		for _, cav := range chain {
			if err := validateCaveat(call, side, cav); err != nil {
				// TODO(jsimsa): Decide what (if any) logging mechanism to use.
				// vlog.VI(4).Infof("Ignoring chain %v: %v", chain, err)
				results[i] = err
				break
			}
		}
	}

	return results
}

// validateCaveat is pretty much the same as cav.Validate(call, side), but it defers
// to any validation scheme overrides via SetCaveatValidator.
func validateCaveat(call Call, side CallSide, cav Caveat) error {
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
		return fn(call, side, cav)
	}
	return cav.Validate(call, side)
}

// TODO(ashankar): Get rid of this function? It allows users to mess
// with the integrity of 'b'.
func MarshalBlessings(b Blessings) WireBlessings {
	return WireBlessings{b.chains}
}

func wireBlessingsToNative(wire WireBlessings, native *Blessings) error {
	if len(wire.CertificateChains) == 0 {
		return nil
	}
	certchains := wire.CertificateChains
	if len(certchains) == 0 || len(certchains[0]) == 0 {
		return errEmptyChain
	}
	// Public keys should match for all chains.
	marshaledkey := certchains[0][len(certchains[0])-1].PublicKey
	key, err := validateCertificateChain(certchains[0])
	if err != nil {
		return err
	}
	for i := 1; i < len(certchains); i++ {
		chain := certchains[i]
		if len(chain) == 0 {
			return errEmptyChain
		}
		cert := chain[len(chain)-1]
		if !bytes.Equal(marshaledkey, cert.PublicKey) {
			return errors.New("invalid blessings: two certificate chains that bind to different public keys")
		}
		if _, err := validateCertificateChain(chain); err != nil {
			return err
		}
	}
	native.chains = certchains
	native.publicKey = key
	return nil
}

func wireBlessingsFromNative(wire *WireBlessings, native Blessings) error {
	wire.CertificateChains = native.chains
	return nil
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
		ret.chains = append(ret.chains, b.chains...)
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
	fn        func(Call, CallSide, Caveat) error
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
func SetCaveatValidator(fn func(Call, CallSide, Caveat) error) {
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

// DefaultBlessingPatterns returns the BlessingsPatterns of the Default Blessings
// of the provided Principal.
func DefaultBlessingPatterns(p Principal) (patterns []BlessingPattern) {
	for b, _ := range p.BlessingsInfo(p.BlessingStore().Default()) {
		patterns = append(patterns, BlessingPattern(b))
	}
	return
}

// BlessingNames returns a validated set of human-readable blessing names encapsulated
// in the blessings object presented by a principal. These blessing names are uncovered
// from the blessings object presented by the local end of the 'call' if 'side' is
// CallSideLocal, and from the blessings object presented by the remote end if 'side' is
// CallSideRemote.
//
// The blessing names are guaranteed to:
//
// (1) Satisfy all the caveats associated with them, in the context of the call.
// (2) Be rooted in call.LocalPrincipal.Roots.
//
// Caveats are considered satisfied for the 'call' and 'side' if the CaveatValidator
// implementation can be found in the address space of the caller and Validate
// returns nil.
//
// BlessingNames also returns the RejectedBlessings for each blessing name that cannot
// be validated.
func BlessingNames(call Call, side CallSide) ([]string, []RejectedBlessing) {
	var b Blessings
	switch side {
	case CallSideLocal:
		b = call.LocalBlessings()
	case CallSideRemote:
		b = call.RemoteBlessings()
	}

	if b.IsZero() {
		return nil, nil
	}
	validator := defaultChainCaveatValidator
	if customValidator := call.Context().Value(chainValidatorKey); customValidator != nil {
		validator = customValidator.(func(call Call, side CallSide, chains [][]Caveat) []error)
	}

	var (
		validatedNames      []string
		rejected            []RejectedBlessing
		pendingNames        []string
		pendingChainCaveats [][]Caveat
	)
	for _, chain := range b.chains {
		name, err := verifyChainSignature(call, side, chain)
		if err != nil {
			rejected = append(rejected, RejectedBlessing{name, err})
			continue
		}

		cavs := []Caveat{}
		for _, cert := range chain {
			cavs = append(cavs, cert.Caveats...)
		}

		if len(cavs) == 0 {
			validatedNames = append(validatedNames, name) // No caveats to validate, add it to blessingNames.
		} else {
			pendingNames = append(pendingNames, name)
			pendingChainCaveats = append(pendingChainCaveats, cavs)
		}
	}

	if len(pendingChainCaveats) == 0 {
		return validatedNames, rejected
	}

	validationResults := validator(call, side, pendingChainCaveats)
	if g, w := len(validationResults), len(pendingNames); g != w {
		panic(fmt.Sprintf("Got wrong number of validation results. Got %d, expected %d.", g, w))
	}

	for i, resultErr := range validationResults {
		if resultErr == nil {
			validatedNames = append(validatedNames, pendingNames[i])
		} else {
			rejected = append(rejected, RejectedBlessing{pendingNames[i], resultErr})
		}
	}
	return validatedNames, rejected
}
