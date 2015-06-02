// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/verror"
)

var (
	errEmptyChain         = verror.Register(pkgPath+".errEmptyChain", verror.NoRetry, "empty certificate chain in blessings")
	errMisconfiguredRoots = verror.Register(pkgPath+".errMisconfiguredRoots", verror.NoRetry, "recognized root certificates not configured")
	errMultiplePublicKeys = verror.Register(pkgPath+".errMultiplePublicKeys", verror.NoRetry, "invalid blessings: two certificate chains that bind to different public keys")
	errInvalidUnion       = verror.Register(pkgPath+".errInvalidUnion", verror.NoRetry, "cannot create union of blessings bound to different public keys")
)

// Blessings encapsulates all cryptographic operations required to
// prove that a set of (human-readable) blessing names are bound
// to a principal in a specific call.
//
// Blessings objects are meant to be presented to other principals to
// authenticate and authorize actions. The functions 'LocalBlessingNames'
// and 'RemoteBlessingNames' defined in this package can be used to uncover
// the blessing names encapsulated in these objects.
//
// Blessings objects are immutable and multiple goroutines may invoke methods
// on them simultaneously.
//
// See also: https://v.io/glossary.html#blessing
type Blessings struct {
	chains    [][]Certificate
	publicKey PublicKey
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

// Equivalent returns true if 'b' and 'blessings' can be used interchangeably,
// i.e., 'b' will be authorized wherever 'blessings' is and vice-versa.
func (b Blessings) Equivalent(blessings Blessings) bool {
	return reflect.DeepEqual(b, blessings)
}

// Expiry returns the time at which b will no longer be valid, or the zero
// value of time.Time if the blessing does not expire.
func (b Blessings) Expiry() time.Time {
	var min time.Time
	for _, chain := range b.chains {
		for _, cert := range chain {
			for _, cav := range cert.Caveats {
				if t := expiryTime(cav); !t.IsZero() && (min.IsZero() || t.Before(min)) {
					min = t
				}
			}
		}
	}
	return min
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
	return strings.Join(blessings, ",")
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

func validateCertificateChains(chains [][]Certificate, ch chan error) {
	for _, chain := range chains {
		pMarshaledKey := chain[0].PublicKey
		pSignature := Signature{}
		for _, cert := range chain {
			go func(cert Certificate, parent Signature, marshaledParentKey []byte) {
				parentKey, err := UnmarshalPublicKey(marshaledParentKey)
				if err != nil {
					ch <- err
					return
				}
				ch <- cert.validate(parent, parentKey)
			}(cert, pSignature, pMarshaledKey)
			pMarshaledKey = cert.PublicKey
			pSignature = cert.Signature
		}
	}
}

// Verifies that the chain signatures are correct, without handling caveat validation
func verifyChainSignature(ctx *context.T, call Call, chain []Certificate) (string, error) {
	blessing := chain[0].Extension
	for i := 1; i < len(chain); i++ {
		blessing += ChainSeparator
		blessing += chain[i].Extension
	}

	// Verify that the root of the chain is recognized as an authority
	// on blessing.
	root, err := UnmarshalPublicKey(chain[0].PublicKey)
	if err != nil {
		return blessing, err
	}
	local := call.LocalPrincipal()
	if local == nil {
		return blessing, NewErrUnrecognizedRoot(ctx, root.String(), verror.New(errMisconfiguredRoots, ctx))
	}
	if local.Roots() == nil {
		return blessing, NewErrUnrecognizedRoot(ctx, root.String(), verror.New(errMisconfiguredRoots, ctx))
	}
	if err := local.Roots().Recognized(root, blessing); err != nil {
		return blessing, err
	}

	return blessing, nil
}

func defaultCaveatValidation(ctx *context.T, call Call, chains [][]Caveat) []error {
	results := make([]error, len(chains))
	for i, chain := range chains {
		for _, cav := range chain {
			if err := cav.Validate(ctx, call); err != nil {
				results[i] = err
				break
			}
		}
	}
	return results
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
		return verror.New(errEmptyChain, nil)
	}
	marshaledKey := certchains[0][len(certchains[0])-1].PublicKey
	key, err := UnmarshalPublicKey(marshaledKey)
	if err != nil {
		return err
	}

	// Do error checking up-front to avoid doing unnecessary work.
	ncerts := 0
	for _, chain := range certchains {
		if len(chain) == 0 {
			return verror.New(errEmptyChain, nil)
		}
		if !bytes.Equal(marshaledKey, chain[len(chain)-1].PublicKey) {
			return verror.New(errMultiplePublicKeys, nil)
		}
		ncerts += len(chain)
	}

	// Now validate all the certs in parallel.
	var firstErr error
	ch := make(chan error, ncerts)
	validateCertificateChains(certchains, ch)
	for i := 0; i < ncerts; i++ {
		if err = <-ch; err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return firstErr
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
			return Blessings{}, verror.New(errInvalidUnion, nil)
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

var (
	caveatValidationMu         sync.RWMutex
	caveatValidation           = defaultCaveatValidation
	caveatValidationOverridden = false
)

func overrideCaveatValidation(fn func(ctx *context.T, call Call, sets [][]Caveat) []error) {
	caveatValidationMu.Lock()
	if caveatValidationOverridden {
		panic("security: OverrideCaveatValidation may only be called once")
	}
	caveatValidationOverridden = true
	caveatValidation = fn
	caveatValidationMu.Unlock()
}

func setCaveatValidationForTest(fn func(ctx *context.T, call Call, sets [][]Caveat) []error) {
	// For tests we skip the panic on multiple calls, so that we can easily revert
	// to the default validator.
	caveatValidationMu.Lock()
	caveatValidation = fn
	caveatValidationMu.Unlock()
}

func getCaveatValidation() func(ctx *context.T, call Call, sets [][]Caveat) []error {
	caveatValidationMu.RLock()
	fn := caveatValidation
	caveatValidationMu.RUnlock()
	return fn
}

// ReturnBlessingNames returns the validated set of human-readable blessing names
// encapsulated in the blessings object presented by the remote end of a call.
//
// The blessing names are guaranteed to:
//
// (1) Satisfy all the caveats associated with them, in the context of the call.
// (2) Be rooted in call.LocalPrincipal.Roots.
//
// Caveats are considered satisfied for the 'call' if the CaveatValidator implementation
// can be found in the address space of the caller and Validate returns nil.
//
// RemoteBlessingNames also returns the RejectedBlessings for each blessing name that cannot
// be validated.
func RemoteBlessingNames(ctx *context.T, call Call) ([]string, []RejectedBlessing) {
	if ctx == nil || call == nil {
		return nil, nil
	}
	b := call.RemoteBlessings()
	if b.IsZero() {
		return nil, nil
	}
	var (
		validatedNames    []string
		rejected          []RejectedBlessing
		pendingNames      []string
		pendingCaveatSets [][]Caveat
	)
	for _, chain := range b.chains {
		name, err := verifyChainSignature(ctx, call, chain)
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
			pendingCaveatSets = append(pendingCaveatSets, cavs)
		}
	}
	if len(pendingCaveatSets) == 0 {
		return validatedNames, rejected
	}

	validationResults := getCaveatValidation()(ctx, call, pendingCaveatSets)
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

// LocalBlessingNames returns the set of human-readable blessing names encapsulated
// in the blessings object presented by the local end of the call.
//
// The blessing names are guaranteed to be rooted in call.LocalPrincipal.Roots.
//
// LocalBlessingNames does not validate caveats on the blessing names.
func LocalBlessingNames(ctx *context.T, call Call) []string {
	if ctx == nil || call == nil {
		return nil
	}
	var names []string
	for n, _ := range call.LocalPrincipal().BlessingsInfo(call.LocalBlessings()) {
		names = append(names, n)
	}
	return names
}
