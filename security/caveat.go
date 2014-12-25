package security

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"v.io/veyron/veyron2/vlog"
	"v.io/veyron/veyron2/vom"
)

// NewCaveat returns a Caveat that requires validation by validator.
func NewCaveat(validator CaveatValidator) (Caveat, error) {
	var buf bytes.Buffer
	if err := vom.NewEncoder(&buf).Encode(validator); err != nil {
		return Caveat{}, err
	}
	return Caveat{buf.Bytes()}, nil
}

// ExpiryCaveat returns a Caveat that validates iff the current time is before t.
func ExpiryCaveat(t time.Time) (Caveat, error) {
	return NewCaveat(unixTimeExpiryCaveat(t.Unix()))
}

// MethodCaveat returns a Caveat that validates iff the method being invoked by
// the peer is listed in an argument to this function.
func MethodCaveat(method string, additionalMethods ...string) (Caveat, error) {
	return NewCaveat(methodCaveat(append(additionalMethods, method)))
}

/*
// WARNING: Please do not use this caveat just yet. There is a possible "infinite loop"
// problem when both security.Context.LocalBlessings and security.Context.RemoteBlessings
// have a peer-blessings caveat in them.
//
// TODO(ashankar,ataly): Fix the infinite loop, or remove this caveat.
//
// PeerBlessingsCaveat returns a Caveat that validates iff the peer has a blessing
// that matches one of the patterns provided as an argument to this function.
//
// For example, creating a blessing "alice/friend" with a PeerBlessingsCaveat("bob")
// will allow the blessing "alice/friend" to be used only when communicating with
// a principal that has the blessing "bob".
func PeerBlessingsCaveat(pattern BlessingPattern, additionalPatterns ...BlessingPattern) (Caveat, error) {
	return NewCaveat(peerBlessingsCaveat(append(additionalPatterns, pattern)))
}
*/

// digest returns a hash of the contents of c.
func (c *Caveat) digest(hash Hash) []byte { return hash.sum(c.ValidatorVOM) }

func (c Caveat) String() string {
	var validator CaveatValidator
	if err := vom.NewDecoder(bytes.NewReader(c.ValidatorVOM)).Decode(&validator); err == nil {
		return fmt.Sprintf("%T(%v)", validator, validator)
	}
	// If we could "peek" the type of the encoded object via the VOM-API, that may be a better message?
	return fmt.Sprintf("{Caveat(%d bytes) with the corresponding CaveatValidator not compiled into this binary}", len(c.ValidatorVOM))
}

func (c unixTimeExpiryCaveat) Validate(ctx Context) error {
	now := ctx.Timestamp()
	expiry := time.Unix(int64(c), 0)
	if now.After(expiry) {
		return fmt.Errorf("%T(%v=%v) fails validation at %v", c, c, expiry, now)
	}
	return nil
}

func (c unixTimeExpiryCaveat) String() string {
	return fmt.Sprintf("%v = %v", int64(c), time.Unix(int64(c), 0))
}

func (c methodCaveat) Validate(ctx Context) error {
	methods := []string(c)
	if ctx.Method() == "" && len(methods) == 0 {
		return nil
	}
	for _, m := range methods {
		if ctx.Method() == m {
			return nil
		}
	}
	return fmt.Errorf("%T=%v fails validation for method %q", c, c, ctx.Method())
}

func (c peerBlessingsCaveat) Validate(ctx Context) error {
	patterns := []BlessingPattern(c)
	var self []string
	switch {
	case ctx.LocalBlessings() != nil:
		self = ctx.LocalBlessings().ForContext(ctx)
	default:
		return fmt.Errorf("%T=%v failed validation since ctx.LocalBlessings is nil", c, c)
	}
	for _, p := range patterns {
		if p.MatchedBy(self...) {
			return nil
		}
	}
	return fmt.Errorf("%T=%v fails validation for peer with blessings %v", c, c, self)
}

// UnconstrainedUse returns a Caveat implementation that never fails to
// validate. This is useful only for providing unconstrained blessings/discharges
// to another principal.
func UnconstrainedUse() Caveat { return Caveat{} }

func isUnconstrainedUseCaveat(c Caveat) bool { return len(c.ValidatorVOM) == 0 }

// NewPublicKeyCaveat returns a security.ThirdPartyCaveat which requires a
// discharge from a principal identified by the public key 'key' and present
// at the object name 'location'. This discharging principal is expected to
// validate all provided 'caveats' before issuing a discharge.
func NewPublicKeyCaveat(discharger PublicKey, location string, requirements ThirdPartyRequirements, caveat Caveat, additionalCaveats ...Caveat) (ThirdPartyCaveat, error) {
	cav := &publicKeyThirdPartyCaveat{
		Caveats:                append(additionalCaveats, caveat),
		DischargerLocation:     location,
		DischargerRequirements: requirements,
	}
	if _, err := rand.Read(cav.Nonce[:]); err != nil {
		return nil, err
	}
	var err error
	if cav.DischargerKey, err = discharger.MarshalBinary(); err != nil {
		return nil, err
	}
	return cav, nil
}

func (c *publicKeyThirdPartyCaveat) Validate(ctx Context) error {
	discharge, ok := ctx.RemoteDischarges()[c.ID()]
	if !ok {
		return fmt.Errorf("missing discharge for caveat(id=%v)", c.ID())
	}
	// Must be of the valid type.
	d, ok := discharge.(*publicKeyDischarge)
	if !ok {
		return fmt.Errorf("invalid discharge type(%T) for caveat(%T)", d, c)
	}
	// Must be signed by the principal designated by c.DischargerKey
	key, err := c.discharger()
	if err != nil {
		return err
	}
	if err := d.verify(key); err != nil {
		return err
	}
	// And all caveats on the discharge must be met.
	for _, cav := range d.Caveats {
		var validator CaveatValidator
		if err := vom.NewDecoder(bytes.NewReader(cav.ValidatorVOM)).Decode(&validator); err != nil {
			return fmt.Errorf("failed to interpret a caveat on the discharge: %v", err)
		}
		if err := validator.Validate(ctx); err != nil {
			return fmt.Errorf("a caveat(%T) on the discharge failed to validate: %v", validator, err)
		}
	}
	return nil
}

func (c *publicKeyThirdPartyCaveat) ID() string {
	key, err := c.discharger()
	if err != nil {
		vlog.Error(err)
		return ""
	}
	hash := key.hash()
	bytes := append(hash.sum(c.Nonce[:]), hash.sum(c.DischargerKey)...)
	for _, cav := range c.Caveats {
		bytes = append(bytes, cav.digest(hash)...)
	}
	return base64.StdEncoding.EncodeToString(hash.sum(bytes))
}

func (c *publicKeyThirdPartyCaveat) Location() string { return c.DischargerLocation }
func (c *publicKeyThirdPartyCaveat) Requirements() ThirdPartyRequirements {
	return c.DischargerRequirements
}

func (c *publicKeyThirdPartyCaveat) Dischargeable(context Context) error {
	// Validate the caveats embedded within this third-party caveat.
	for _, cav := range c.Caveats {
		if isUnconstrainedUseCaveat(cav) {
			continue
		}
		var validator CaveatValidator
		if err := vom.NewDecoder(bytes.NewReader(cav.ValidatorVOM)).Decode(&validator); err != nil {
			return fmt.Errorf("failed to interpret restriction embedded in ThirdPartyCaveat: %v", err)
		}
		if err := validator.Validate(context); err != nil {
			return fmt.Errorf("could not validate embedded restriction %T: %v", validator, err)
		}
	}
	return nil
}

func (c *publicKeyThirdPartyCaveat) discharger() (PublicKey, error) {
	key, err := UnmarshalPublicKey(c.DischargerKey)
	if err != nil {
		return nil, fmt.Errorf("invalid %T: failed to unmarshal discharger's public key: %v", *c, err)
	}
	return key, nil
}

func (c publicKeyThirdPartyCaveat) String() string {
	return fmt.Sprintf("%v@%v [%+v]", c.ID(), c.Location(), c.Requirements())
}

func (d *publicKeyDischarge) ID() string { return d.ThirdPartyCaveatID }
func (d *publicKeyDischarge) ThirdPartyCaveats() []ThirdPartyCaveat {
	var ret []ThirdPartyCaveat
	for _, cav := range d.Caveats {
		var tpcav ThirdPartyCaveat
		if err := vom.NewDecoder(bytes.NewReader(cav.ValidatorVOM)).Decode(&tpcav); err == nil {
			ret = append(ret, tpcav)
		}
	}
	return ret
}

func (d *publicKeyDischarge) digest(hash Hash) []byte {
	msg := hash.sum([]byte(d.ThirdPartyCaveatID))
	for _, cav := range d.Caveats {
		msg = append(msg, cav.digest(hash)...)
	}
	return hash.sum(msg)
}

func (d *publicKeyDischarge) verify(key PublicKey) error {
	if !bytes.Equal(d.Signature.Purpose, dischargePurpose) {
		return fmt.Errorf("signature on discharge for caveat %v was not intended for discharges(purpose=%q)", d.ThirdPartyCaveatID, d.Signature.Purpose)
	}
	if !d.Signature.Verify(key, d.digest(key.hash())) {
		return fmt.Errorf("signature verification on discharge for caveat %v failed", d.ThirdPartyCaveatID)
	}
	return nil
}

func (d *publicKeyDischarge) sign(signer Signer) error {
	var err error
	d.Signature, err = signer.Sign(dischargePurpose, d.digest(signer.PublicKey().hash()))
	return err
}
