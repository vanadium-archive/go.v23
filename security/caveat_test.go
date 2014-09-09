package security

import (
	"bytes"
	"testing"

	"veyron2/vom"
)

// alwaysValidCaveat implements CaveatValidator.
type alwaysValidCaveat struct{}

func (alwaysValidCaveat) Validate(Context) error { return nil }

func TestNewCaveat(t *testing.T) {
	validator := alwaysValidCaveat{}
	caveat, err := NewCaveat(validator)
	if err != nil {
		t.Fatalf("NewCaveat(%#v) failed:  %s", err)
	}

	var decodedValidator interface{}
	if err := vom.NewDecoder(bytes.NewReader(caveat.ValidatorVOM)).Decode(&decodedValidator); err != nil {
		t.Fatalf("Could not decode caveat Bytes: %s", err)
	}
	if validator != decodedValidator {
		t.Fatalf("Caveat from CaveatValidator: %#v decoded to CaveatValidator: %#v", validator, decodedValidator)
	}
}
