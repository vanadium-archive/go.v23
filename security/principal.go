package security

import (
	"bytes"
	"fmt"
	"reflect"
	"time"

	"veyron.io/veyron/veyron2/vom"
)

// CreatePrincipal returns a Principal that uses 'signer' for all
// private key operations, 'store' for storing blessings bound
// to the Principal and 'roots' for the set of authoritative public
// keys on blessings recognized by this Principal.
//
// It returns an error if 'store' or 'roots' is nil, or store.PublicKey
// does not match signer.PublicKey.
func CreatePrincipal(signer Signer, store BlessingStore, roots BlessingRoots) (Principal, error) {
	if store == nil || roots == nil {
		return nil, fmt.Errorf("store: %v, roots: %v cannot be nil", store, roots)
	}
	if got, want := store.PublicKey(), signer.PublicKey(); !reflect.DeepEqual(got, want) {
		return nil, fmt.Errorf("store's public key: %v does not match signer's public key: %v", got, want)
	}
	return &principal{signer: signer, store: store, roots: roots}, nil
}

// TODO(ataly, ashankar): GET RID OF THIS METHOD
// This method is a hack for getting MintDischarge to work on PrivateIDs.
// It should go away as soon as we replace PrivateIDs with Principals.
func MintDischargeForPrivateID(signer Signer, cav ThirdPartyCaveat, ctx Context, duration time.Duration, dischargeCaveats []Caveat) (Discharge, error) {
	p := &principal{signer: signer}
	expiry, err := ExpiryCaveat(time.Now().Add(duration))
	if err != nil {
		return nil, err
	}
	return p.MintDischarge(cav, ctx, expiry, dischargeCaveats...)
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
)

type principal struct {
	signer Signer
	roots  BlessingRoots
	store  BlessingStore
}

func (p *principal) Bless(key PublicKey, with Blessings, extension string, caveat Caveat, additionalCaveats ...Caveat) (Blessings, error) {
	if !reflect.DeepEqual(with.PublicKey(), p.PublicKey()) {
		return nil, fmt.Errorf("Principal with public key %v cannot extend blessing with public key %v", p.PublicKey(), with.PublicKey())
	}
	caveats := additionalCaveats
	if !isUnconstrainedUseCaveat(caveat) {
		caveats = append(caveats, caveat)
	}
	cert, err := newUnsignedCertificate(extension, key, caveats...)
	if err != nil {
		return nil, err
	}
	chains := with.certificateChains()
	newchains := make([][]Certificate, len(chains))
	for idx, chain := range chains {
		if err := cert.sign(p.signer, chain[len(chain)-1].Signature); err != nil {
			return nil, err
		}
		newchains[idx] = append(chains[idx], *cert)
	}
	return &blessingsImpl{
		chains:    newchains,
		publicKey: key,
	}, nil
}

func (p *principal) BlessSelf(name string, caveats ...Caveat) (Blessings, error) {
	cert, err := newUnsignedCertificate(name, p.PublicKey(), caveats...)
	if err != nil {
		return nil, err
	}
	if err := cert.sign(p.signer, Signature{}); err != nil {
		return nil, err
	}
	return &blessingsImpl{
		chains:    [][]Certificate{[]Certificate{*cert}},
		publicKey: p.PublicKey(),
	}, nil
}

func (p *principal) Sign(message []byte) (Signature, error) {
	return p.signer.Sign(signPurpose, message)
}

func (p *principal) MintDischarge(tp ThirdPartyCaveat, ctx Context, caveat Caveat, additionalCaveats ...Caveat) (Discharge, error) {
	tpcav, ok := tp.(*publicKeyThirdPartyCaveat)
	if !ok {
		return nil, fmt.Errorf("principal implementation cannot create discharges for caveats of type %T", tp)
	}
	// Validate the caveat encoded within the third-party caveat.
	for _, cav := range tpcav.Caveats {
		var validator CaveatValidator
		if err := vom.NewDecoder(bytes.NewReader(cav.ValidatorVOM)).Decode(&validator); err != nil {
			return nil, fmt.Errorf("failed to interpret restriction encoded in ThirdPartyCaveat: %v", err)
		}
		if err := validator.Validate(ctx); err != nil {
			return nil, fmt.Errorf("caveat validation on %T failed: %v", validator, err)
		}
	}
	// Create the discharge
	caveats := additionalCaveats
	if !isUnconstrainedUseCaveat(caveat) {
		caveats = append(additionalCaveats, caveat)
	}
	d := &publicKeyDischarge{ThirdPartyCaveatID: tpcav.ID(), Caveats: caveats}
	if err := d.sign(p.signer); err != nil {
		return nil, fmt.Errorf("failed to sign discharge: %v", err)
	}
	return d, nil
}

func (p *principal) PublicKey() PublicKey {
	return p.signer.PublicKey()
}

func (p *principal) BlessingStore() BlessingStore {
	return p.store
}

func (p *principal) Roots() BlessingRoots {
	return p.roots
}

func (p *principal) AddToRoots(blessings Blessings) error {
	chains := blessings.certificateChains()
	glob := ChainSeparator + AllPrincipals
	for _, chain := range chains {
		root, err := UnmarshalPublicKey(chain[0].PublicKey)
		if err != nil {
			return fmt.Errorf("failed to unmarshal public key in root certificate with Extension: %q: %v", chain[0].Extension, err)
		}
		pattern := BlessingPattern(chain[0].Extension) + glob
		if err := p.roots.Add(root, pattern); err != nil {
			return fmt.Errorf("failed to Add root: %v for pattern: %v to this principal's roots: %v", root, pattern)
		}
	}
	return nil
}
