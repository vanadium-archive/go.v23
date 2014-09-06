package security

import (
	"bytes"

	"veyron2/vom"
)

// NewCaveat returns a Caveat that requires validation by validator.
func NewCaveat(validator CaveatValidator) (Caveat, error) {
	var buf bytes.Buffer
	if err := vom.NewEncoder(&buf).Encode(validator); err != nil {
		return Caveat{}, err
	}
	return Caveat{buf.Bytes()}, nil
}

func (c Caveat) Bytes() []byte {
	return c.ValidatorVOM
}

// TODO(ataly, ashankar): Define UnconstrainedDelegationCaveat.
