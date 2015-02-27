package security

import (
	"errors"
	"fmt"
	"reflect"
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
// NOTE: veyron/lib/testutil/security provides utility methods for creating
// principals for testing purposes.
func CreatePrincipal(signer Signer, store BlessingStore, roots BlessingRoots) (Principal, error) {
	if store == nil {
		store = errStore{signer.PublicKey()}
	}
	if roots == nil {
		roots = errRoots{}
	}
	if got, want := store.PublicKey(), signer.PublicKey(); !reflect.DeepEqual(got, want) {
		return nil, fmt.Errorf("store's public key: %v does not match signer's public key: %v", got, want)
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

	errNilStore = errors.New("underlying BlessingStore object is nil")
	errNilRoots = errors.New("underlying BlessingRoots object is nil")
)

type errStore struct {
	key PublicKey
}

func (errStore) Set(Blessings, BlessingPattern) (Blessings, error) { return Blessings{}, errNilStore }
func (errStore) ForPeer(peerBlessings ...string) Blessings         { return Blessings{} }
func (errStore) SetDefault(blessings Blessings) error              { return errNilStore }
func (errStore) Default() Blessings                                { return Blessings{} }
func (errStore) PeerBlessings() map[BlessingPattern]Blessings      { return nil }
func (errStore) DebugString() string                               { return errNilStore.Error() }
func (s errStore) PublicKey() PublicKey                            { return s.key }

type errRoots struct{}

func (errRoots) Add(PublicKey, BlessingPattern) error { return errNilRoots }
func (errRoots) Recognized(PublicKey, string) error   { return errNilRoots }
func (errRoots) DebugString() string                  { return errNilRoots.Error() }

type principal struct {
	signer Signer
	roots  BlessingRoots
	store  BlessingStore
}

func (p *principal) Bless(key PublicKey, with Blessings, extension string, caveat Caveat, additionalCaveats ...Caveat) (Blessings, error) {
	if with.IsZero() {
		return Blessings{}, errors.New("the Blessings to bless 'with' must have at least one certificate")
	}
	if !reflect.DeepEqual(with.PublicKey(), p.PublicKey()) {
		return Blessings{}, fmt.Errorf("Principal with public key %v cannot extend blessing with public key %v", p.PublicKey(), with.PublicKey())
	}
	caveats := append(additionalCaveats, caveat)
	cert, err := newUnsignedCertificate(extension, key, caveats...)
	if err != nil {
		return Blessings{}, err
	}
	chains := with.certificateChains()
	newchains := make([][]Certificate, len(chains))
	for idx, chain := range chains {
		if err := cert.sign(p.signer, chain[len(chain)-1].Signature); err != nil {
			return Blessings{}, err
		}
		newchains[idx] = append(chains[idx], *cert)
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
	if forCaveat.Id != PublicKeyThirdPartyCaveatX.Id {
		return Discharge{}, fmt.Errorf("cannot mint discharges for %v", forCaveat)
	}
	id := forCaveat.ThirdPartyDetails().ID()
	dischargeCaveats := append(additionalCaveatsOnDischarge, caveatOnDischarge)
	d := publicKeyDischarge{ThirdPartyCaveatID: id, Caveats: dischargeCaveats}
	if err := d.sign(p.signer); err != nil {
		return Discharge{}, fmt.Errorf("failed to sign discharge: %v", err)
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
		return errors.New("principal does not have any BlessingRoots")
	}
	chains := blessings.certificateChains()
	for _, chain := range chains {
		root, err := UnmarshalPublicKey(chain[0].PublicKey)
		if err != nil {
			return fmt.Errorf("failed to unmarshal public key in root certificate with Extension: %q: %v", chain[0].Extension, err)
		}
		pattern := BlessingPattern(chain[0].Extension)
		if err := p.roots.Add(root, pattern); err != nil {
			return fmt.Errorf("failed to Add root: %v for pattern: %v to this principal's roots: %v", root, pattern, err)
		}
	}
	return nil
}
