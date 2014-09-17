package security

import (
	"fmt"
	"reflect"
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
	blessPurpose = []byte(SignatureForBlessingCertificates)
	signPurpose  = []byte(SignatureForMessageSigning)
)

type principal struct {
	signer Signer
}

func (p *principal) Bless(key PublicKey, with Blessings, extension string, caveat Caveat, additionalCaveats ...Caveat) (Blessings, error) {
	if !reflect.DeepEqual(with.PublicKey(), p.PublicKey()) {
		return nil, fmt.Errorf("Principal with public key %v cannot extend blessing with public key %v", p.PublicKey(), with.PublicKey())
	}
	caveats := additionalCaveats
	if caveat.ValidatorVOM != nil { // nil would imply "unconstrained delegation"
		caveats = append(caveats, caveat)
	}
	cert, err := newUnsignedCertificate(extension, key, caveats...)
	if err != nil {
		return nil, err
	}
	chains := with.certificateChains()
	newchains := make([][]Certificate, len(chains))
	for idx, chain := range chains {
		msg, err := cert.contentHash(chain[len(chain)-1].Signature)
		if err != nil {
			return nil, err
		}
		if cert.Signature, err = p.signer.Sign(blessPurpose, msg); err != nil {
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
	msg, err := cert.contentHash(Signature{})
	if err != nil {
		return nil, err
	}
	if cert.Signature, err = p.signer.Sign(blessPurpose, msg); err != nil {
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

func (p *principal) PublicKey() PublicKey {
	return p.signer.PublicKey()
}
