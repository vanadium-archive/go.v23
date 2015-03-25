// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"reflect"
	"testing"
)

func TestDischargeToAndFromWire(t *testing.T) {
	var (
		p   = newPrincipal(t)
		cav = newCaveat(NewPublicKeyCaveat(p.PublicKey(), "peoria", ThirdPartyRequirements{}, UnconstrainedUse()))
	)
	discharge, err := p.MintDischarge(cav, UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	// Should be able to round-trip from native type to a copy
	var d2 Discharge
	if err := roundTrip(discharge, &d2); err != nil || !reflect.DeepEqual(discharge, d2) {
		t.Errorf("Got (%#v, %v) want (%#v, nil)", d2, err, discharge)
	}
	// And from native type to a wire type
	var wire1, wire2 WireDischarge
	if err := roundTrip(discharge, &wire1); err != nil {
		t.Fatal(err)
	}
	// And from wire to another wire type
	if err := roundTrip(wire1, &wire2); err != nil || !reflect.DeepEqual(wire1, wire2) {
		t.Fatalf("Got (%#v, %v) want (%#v, nil)", wire2, err, wire1)
	}
	// And from wire to a native type
	var d3 Discharge
	if err := roundTrip(wire2, &d3); err != nil || !reflect.DeepEqual(discharge, d3) {
		t.Errorf("Got (%#v, %v) want (%#v, nil)", d3, err, discharge)
	}
}
