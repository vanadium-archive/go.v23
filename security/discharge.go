package security

import (
	"time"

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
		return v.Value.ThirdPartyCaveatId
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

func wireDischargeToNative(wire WireDischarge, native *Discharge) error {
	native.wire = wire
	return nil
}

func wireDischargeFromNative(wire *WireDischarge, native Discharge) error {
	*wire = native.wire
	return nil
}

func expiryTime(cav Caveat) time.Time {
	switch cav.Id {
	case ExpiryCaveatX.Id:
		var t time.Time
		if err := vom.Decode(cav.ParamVom, &t); err != nil {
			// TODO(jsimsa): Decide what (if any) logging mechanism to use.
			// vlog.Errorf("Failed to decode ParamVOM for cav(%v): %v", cav, err)
			return time.Time{}
		}
		return t
	}
	return time.Time{}
}
