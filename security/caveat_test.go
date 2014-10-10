package security

import (
	"bytes"
	"testing"
	"time"

	"veyron.io/veyron/veyron2/vom"
)

func TestCaveats(t *testing.T) {
	var (
		self  = newPrincipal(t)
		ctx   = &context{method: "Foo", local: self, localBlessings: blessSelf(t, self, "alice/phone/friend")}
		C     = newCaveat
		tests = []struct {
			cav Caveat
			ok  bool
		}{
			// NewCaveat
			{C(NewCaveat(unixTimeExpiryCaveat(time.Now().Add(time.Hour).Unix()))), true},
			// ExpiryCaveat
			{C(ExpiryCaveat(time.Now().Add(time.Hour))), true},
			{C(ExpiryCaveat(time.Now().Add(-1 * time.Hour))), false},
			// MethodCaveat
			{C(MethodCaveat("Foo")), true},
			{C(MethodCaveat("Bar")), false},
			{C(MethodCaveat("Foo", "Bar")), true},
			{C(MethodCaveat("Bar", "Baz")), false},
			/*
				// PeerBlessingCaveat
				{C(PeerBlessingsCaveat("bob/...")), false},
				{C(PeerBlessingsCaveat("alice/...")), true},
				{C(PeerBlessingsCaveat("alice/phone")), false},
				{C(PeerBlessingsCaveat("alice/phone/...")), true},
				{C(PeerBlessingsCaveat("alice/phone/friend")), true},
				{C(PeerBlessingsCaveat("alice/phone/friend/...")), true},
				{C(PeerBlessingsCaveat("alice/phone/friend/delegate")), true},
				{C(PeerBlessingsCaveat("alice/desktop/friend")), false},
				{C(PeerBlessingsCaveat("alice/desktop/friend", "alice/phone/...")), true},
			*/
		}
	)
	self.AddToRoots(ctx.localBlessings)
	for idx, test := range tests {
		var validator CaveatValidator
		if err := vom.NewDecoder(bytes.NewReader(test.cav.ValidatorVOM)).Decode(&validator); err != nil {
			t.Errorf("Failed to decode validator(%v) for test #%d", err, idx)
			continue
		}
		if err := validator.Validate(ctx); (err == nil) != test.ok {
			t.Errorf("(%T=%v).Validate(...) returned '%v', expected validation? %v", validator, validator, err, test.ok)
		}
	}
}

func TestPublicKeyThirdPartyCaveat(t *testing.T) {
	var (
		valid        = newCaveat(ExpiryCaveat(time.Now().Add(24 * time.Hour)))
		expired      = newCaveat(ExpiryCaveat(time.Now().Add(-24 * time.Hour)))
		discharger   = newPrincipal(t)
		randomserver = newPrincipal(t)
		ctx          = func(method string, discharges ...Discharge) Context {
			ctx := &context{method: method, discharges: make(map[string]Discharge)}
			for _, d := range discharges {
				ctx.discharges[d.ID()] = d
			}
			return ctx
		}
	)

	tpc, err := NewPublicKeyCaveat(discharger.PublicKey(), "location", ThirdPartyRequirements{}, valid)
	if err != nil {
		t.Fatal(err)
	}
	// Caveat should fail validation without a discharge
	if err := matchesError(tpc.Validate(ctx("Method1")), "missing discharge"); err != nil {
		t.Fatal(err)
	}
	// Should validate when the discharge is present (and caveats on the discharge are met).
	d, err := discharger.MintDischarge(tpc, newCaveat(MethodCaveat("Method1")))
	if err != nil {
		t.Fatal(err)
	}
	if err := tpc.Validate(ctx("Method1", d)); err != nil {
		t.Fatal(err)
	}
	// Should fail validation when caveats on the discharge are not met.
	if err := matchesError(tpc.Validate(ctx("Method2", d)), "discharge failed to validate"); err != nil {
		t.Fatal(err)
	}
	// A discharge minted by another principal should not be respected.
	if d, err = randomserver.MintDischarge(tpc, UnconstrainedUse()); d != nil {
		if err := matchesError(tpc.Validate(ctx("Method1", d)), "signature verification on discharge"); err != nil {
			t.Fatal(err)
		}
	}
	// And ThirdPartyCaveat should not be dischargeable if caveats encoded within it fail validation.
	tpc, err = NewPublicKeyCaveat(discharger.PublicKey(), "location", ThirdPartyRequirements{}, expired)
	if err != nil {
		t.Fatal(err)
	}
	if merr := matchesError(tpc.Dischargeable(&context{}), "could not validate embedded restriction security.unixTimeExpiryCaveat"); merr != nil {
		t.Fatal(merr)
	}
}
