package security

import (
	"bytes"
	"fmt"
	"reflect"

	"veyron.io/veyron/veyron2/vom"
)

// CreatePrincipal returns a Principal with the provided blessings and uses
// signer to Sign further operations. It requires that the provided roots are
// recognized and that blessings.PublicKey be exactly the same as signer.PublicKey.
//
// TODO(ataly): Add "BlessingStore" and "BlessingRoots" as arguments after adding
// methods to the Principal interface that have been specified in the new API
// but not added yet.
func CreatePrincipal(signer Signer) (Principal, error) {
	return &principal{signer: signer}, nil
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
