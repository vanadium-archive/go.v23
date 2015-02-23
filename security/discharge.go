package security

import (
	"fmt"
)

// NewDischarge creates a Discharge object from the provided wire representation.
func NewDischarge(wire WireDischarge) (Discharge, error) {
	switch v := wire.(type) {
	case WireDischargePublicKey:
		return &v.Value, nil
	default:
		return nil, fmt.Errorf("this binary cannot interpret WireDischarge.%v(%T) as a Discharge object", wire.Name(), v)
	}
}

// MarshalDischarge is the inverse of NewDischarge, converting an in-memory
// representation of d to the VDL-defined wire format.
func MarshalDischarge(d Discharge) WireDischarge { return d.toWire() }
