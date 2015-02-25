package security

import (
	"time"

	"v.io/v23/vlog"
	"v.io/v23/vom"
)

// Discharge represents a "proof" required for satisfying a ThirdPartyCaveat.
//
// A discharge may have caveats of its own (including ThirdPartyCaveats) that
// restrict the context in which the discharge is usable.
//
// Discharge objects are immutable and multiple goroutines may invoke methods
// on a Discharge simultaneously.
type Discharge struct {
	wire WireDischarge
}

// ID returns the identifier for the third-party caveat that d is a discharge
// for.
func (d Discharge) ID() string {
	switch v := d.wire.(type) {
	case WireDischargePublicKey:
		return v.Value.ThirdPartyCaveatID
	default:
		return ""
	}
}

// ThirdPartyCaveats returns the set of third-party caveats on the scope of the
// discharge.
func (d Discharge) ThirdPartyCaveats() []ThirdPartyCaveat {
	var ret []ThirdPartyCaveat
	switch v := d.wire.(type) {
	case WireDischargePublicKey:
		for _, cav := range v.Value.Caveats {
			if tp := cav.ThirdPartyDetails(); tp != nil {
				ret = append(ret, tp)
			}
		}
	}
	return ret
}

// Expiry returns the time at which d will no longer be valid, or the zero
// value of time.Time if the discharge does not expire.
func (d Discharge) Expiry() time.Time {
	var min time.Time
	switch v := d.wire.(type) {
	case WireDischargePublicKey:
		for _, cav := range v.Value.Caveats {
			t := expiryTime(cav)
			if !t.IsZero() && (min.IsZero() || t.Before(min)) {
				min = t
			}
		}
	}
	return min
}

func expiryTime(cav Caveat) time.Time {
	switch cav.Id {
	case ExpiryCaveatX.Id:
		var t time.Time
		if err := vom.Decode(cav.ParamVom, &t); err != nil {
			vlog.Errorf("Failed to decode ParamVOM for cav(%v): %v", cav, err)
			return time.Time{}
		}
		return t
	case UnixTimeExpiryCaveatX.Id:
		// TODO(suharshs): Remove this after we only use ExpiryCaveatX.
		var unix int64
		if err := vom.Decode(cav.ParamVom, &unix); err != nil {
			vlog.Errorf("Failed to decode ParamVOM for cav(%v): %v", cav, err)
			return time.Time{}
		}
		return time.Unix(unix, 0)
	}
	return time.Time{}
}

// NewDischarge creates a Discharge object from the provided wire representation.
//
// TODO(ashankar,toddw): Once native-to-wire conversion support is ready for
// union types, then remove this and instead use VDL support for wire/native
// type conversions.
func NewDischarge(wire WireDischarge) Discharge {
	return Discharge{wire}
}

// MarshalDischarge is the inverse of NewDischarge, converting an in-memory
// representation of d to the VDL-defined wire format.
//
// TODO(ashankar,toddw): Once native-to-wire conversion support is ready for
// union types, then remove this and instead use VDL support for wire/native
// type conversions.
func MarshalDischarge(d Discharge) WireDischarge { return d.wire }
