// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"reflect"
	"v.io/v23/verror"
)

// CreatePrincipal returns a Principal that uses 'signer' for all
// private key operations, 'store' for storing blessings bound
// to the Principal and 'roots' for the set of authoritative public
// keys on blessings recognized by this Principal.
//
// If provided 'roots' is nil then the Principal does not trust any
// public keys and all subsequent 'AddToRoots' operations fail.
//
// It returns an error if store.PublicKey does not match signer.PublicKey.
//
// NOTE: v.io/x/ref/lib/testutil/security provides utility methods for creating
// principals for testing purposes.
func CreatePrincipal(signer Signer, store BlessingStore, roots BlessingRoots) (Principal, error) {
	if store == nil {
		store = errStore{signer.PublicKey()}
	}
	if roots == nil {
		roots = errRoots{}
	}
	if got, want := store.PublicKey(), signer.PublicKey(); !reflect.DeepEqual(got, want) {
		return nil, verror.New(errBadStoreKey, nil, got, want)
	}
	return &principal{signer: signer, store: store, roots: roots}, nil
}

var (
	// Every Signer.Sign operation conducted by the principal fills in a
	// "purpose" before signing to prevent "type attacks" so that a signature obtained
	// for one operation (e.g. Principal.Sign) cannot be re-purposed for another
	// operation (e.g. Principal.Bless). If the "Principal" object lives in an
	// external "agent" process, this works out well since this other process
	// can confidently audit all private key operations.
	// For this to work, ALL private key operations by the Principal must
	// use a distinct "purpose" and no two "purpose"s should share a prefix
	// or suffix.
	blessPurpose     = []byte(SignatureForBlessingCertificates)
	signPurpose      = []byte(SignatureForMessageSigning)
	dischargePurpose = []byte(SignatureForDischarge)

	errNilStore  = verror.Register(pkgPath+".errNilStore", verror.NoRetry, "{1:}{2:}underlying BlessingStore object is nil{:_}")
	errNilRoots  = verror.Register(pkgPath+".errNilRoots", verror.NoRetry, "{1:}{2:}underlying BlessingRoots object is nil{:_}")
	errNeedCert  = verror.Register(pkgPath+".errNeedCert", verror.NoRetry, "{1:}{2:}the Blessings to bless 'with' must have at least one certificate{:_}")
	errNeedRoots = verror.Register(pkgPath+".errNeedRoots", verror.NoRetry, "{1:}{2:}principal does not have any BlessingRoots{:_}")

	errBadStoreKey        = verror.Register(pkgPath+".errBadStoreKey", verror.NoRetry, "{1:}{2:}store's public key: {3} does not match signer's public key: {4}{:_}")
	errCantExtendBlessing = verror.Register(pkgPath+".errCantExtendBlessing", verror.NoRetry, "{1:}{2:}Principal with public key {3} cannot extend blessing with public key {4}{:_}")
	errCantMintDischarges = verror.Register(pkgPath+".errCantMintDischarges", verror.NoRetry, "{1:}{2:}cannot mint discharges for {3}{:_}")
	errCantSignDischarge  = verror.Register(pkgPath+".errCantSignDischarge", verror.NoRetry, "{1:}{2:}failed to sign discharge: {3}{:_}")
	errCantUnmarshalKey   = verror.Register(pkgPath+".errCantUnmarshalKey", verror.NoRetry, "{1:}{2:}failed to unmarshal public key in root certificate with Extension: {3}: {4}{:_}")
	errCantAddRoot        = verror.Register(pkgPath+".errCantAddRoot", verror.NoRetry, "{1:}{2:}failed to Add root: {3} for pattern: {4} to this principal's roots: {5}{:_}")
)

type errStore struct {
	key PublicKey
}

func (errStore) Set(Blessings, BlessingPattern) (Blessings, error) {
	return Blessings{}, verror.New(errNilStore, nil)
}
func (errStore) ForPeer(peerBlessings ...string) Blessings    { return Blessings{} }
func (errStore) SetDefault(blessings Blessings) error         { return verror.New(errNilStore, nil) }
func (errStore) Default() Blessings                           { return Blessings{} }
func (errStore) PeerBlessings() map[BlessingPattern]Blessings { return nil }
func (errStore) DebugString() string                          { return verror.New(errNilStore, nil).Error() }
func (s errStore) PublicKey() PublicKey                       { return s.key }

type errRoots struct{}

func (errRoots) Add(PublicKey, BlessingPattern) error { return verror.New(errNilRoots, nil) }
func (errRoots) Recognized(PublicKey, string) error   { return verror.New(errNilRoots, nil) }
func (errRoots) DebugString() string                  { return verror.New(errNilRoots, nil).Error() }

type principal struct {
	signer Signer
	roots  BlessingRoots
	store  BlessingStore
}

func (p *principal) Bless(key PublicKey, with Blessings, extension string, caveat Caveat, additionalCaveats ...Caveat) (Blessings, error) {
	if with.IsZero() {
		return Blessings{}, verror.New(errNeedCert, nil)
	}
	if !reflect.DeepEqual(with.PublicKey(), p.PublicKey()) {
		return Blessings{}, verror.New(errCantExtendBlessing, nil, p.PublicKey(), with.PublicKey())
	}
	caveats := append(additionalCaveats, caveat)
	cert, err := newUnsignedCertificate(extension, key, caveats...)
	if err != nil {
		return Blessings{}, err
	}
	chains := with.chains
	newchains := make([][]Certificate, len(chains))
	for idx, chain := range chains {
		if err := cert.sign(p.signer, chain[len(chain)-1].Signature); err != nil {
			return Blessings{}, err
		}
		cpy := make([]Certificate, len(chain)+1)
		copy(cpy, chain)
		cpy[len(cpy)-1] = *cert
		newchains[idx] = cpy
	}
	return Blessings{
		chains:    newchains,
		publicKey: key,
	}, nil
}

func (p *principal) BlessSelf(name string, caveats ...Caveat) (Blessings, error) {
	cert, err := newUnsignedCertificate(name, p.PublicKey(), caveats...)
	if err != nil {
		return Blessings{}, err
	}
	if err := cert.sign(p.signer, Signature{}); err != nil {
		return Blessings{}, err
	}
	return Blessings{
		chains:    [][]Certificate{[]Certificate{*cert}},
		publicKey: p.PublicKey(),
	}, nil
}

func (p *principal) Sign(message []byte) (Signature, error) {
	return p.signer.Sign(signPurpose, message)
}

func (p *principal) MintDischarge(forCaveat, caveatOnDischarge Caveat, additionalCaveatsOnDischarge ...Caveat) (Discharge, error) {
	if forCaveat.Id != PublicKeyThirdPartyCaveat.Id {
		return Discharge{}, verror.New(errCantMintDischarges, nil, forCaveat)
	}
	id := forCaveat.ThirdPartyDetails().ID()
	dischargeCaveats := append(additionalCaveatsOnDischarge, caveatOnDischarge)
	d := publicKeyDischarge{ThirdPartyCaveatId: id, Caveats: dischargeCaveats}
	if err := d.sign(p.signer); err != nil {
		return Discharge{}, verror.New(errCantSignDischarge, nil, err)
	}
	return Discharge{WireDischargePublicKey{d}}, nil
}

func (p *principal) PublicKey() PublicKey {
	return p.signer.PublicKey()
}

func (p *principal) BlessingsInfo(b Blessings) map[string][]Caveat {
	var bInfo map[string][]Caveat
	for _, chain := range b.chains {
		name := nameForPrincipal(p, chain)
		if len(name) > 0 {
			if bInfo == nil {
				bInfo = make(map[string][]Caveat)
			}
			bInfo[name] = nil
			for _, cert := range chain {
				bInfo[name] = append(bInfo[name], cert.Caveats...)
			}
		}
	}
	return bInfo
}

func (p *principal) BlessingsByName(name BlessingPattern) []Blessings {
	var matched []Blessings
	for _, b := range p.store.PeerBlessings() {
		if m := b.blessingsByNameForPrincipal(p, name); !m.IsZero() {
			matched = append(matched, b)
		}
	}
	return matched
}

func (p *principal) BlessingStore() BlessingStore {
	return p.store
}

func (p *principal) Roots() BlessingRoots {
	return p.roots
}

func (p *principal) AddToRoots(blessings Blessings) error {
	if p.roots == nil {
		return verror.New(errNeedRoots, nil)
	}
	chains := blessings.chains
	for _, chain := range chains {
		root, err := UnmarshalPublicKey(chain[0].PublicKey)
		if err != nil {
			return verror.New(errCantUnmarshalKey, nil, chain[0].Extension, err)
		}
		pattern := BlessingPattern(chain[0].Extension)
		if err := p.roots.Add(root, pattern); err != nil {
			return verror.New(errCantAddRoot, nil, root, pattern, err)
		}
	}
	return nil
}
