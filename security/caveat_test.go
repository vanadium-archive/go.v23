package security

import (
	"bytes"
	"testing"
	"time"

	"veyron.io/veyron/veyron2/vom"
)

func TestCaveats(t *testing.T) {
	var (
		ctx   = &context{method: "Foo", localID: FakePublicID("alice/phone/friend")}
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
			// PeerBlessingCaveat
			{C(PeerBlessingsCaveat("fake/bob/...")), false},
			{C(PeerBlessingsCaveat("fake/alice/...")), true},
			{C(PeerBlessingsCaveat("fake/alice/phone")), false},
			{C(PeerBlessingsCaveat("fake/alice/phone/...")), true},
			{C(PeerBlessingsCaveat("fake/alice/phone/friend")), true},
			{C(PeerBlessingsCaveat("fake/alice/phone/friend/...")), true},
			{C(PeerBlessingsCaveat("fake/alice/phone/friend/delegate")), true},
			{C(PeerBlessingsCaveat("fake/alice/desktop/friend")), false},
			{C(PeerBlessingsCaveat("fake/alice/desktop/friend", "fake/alice/phone/...")), true},
		}
	)
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
	d, err := discharger.MintDischarge(tpc, &context{}, newCaveat(MethodCaveat("Method1")))
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
	if d, err = randomserver.MintDischarge(tpc, &context{}, UnconstrainedUse()); d != nil {
		if err := matchesError(tpc.Validate(ctx("Method1", d)), "signature verification on discharge"); err != nil {
			t.Fatal(err)
		}
	}
	// And discharges should not be minted if the caveat encoded within the ThirdPartyCaveat fails validation.
	tpc, err = NewPublicKeyCaveat(discharger.PublicKey(), "location", ThirdPartyRequirements{}, expired)
	if err != nil {
		t.Fatal(err)
	}
	d, err = discharger.MintDischarge(tpc, &context{}, UnconstrainedUse())
	if merr := matchesError(err, "caveat validation on security.unixTimeExpiryCaveat failed"); merr != nil {
		t.Fatal(err)
	}
	if d != nil {
		t.Fatalf("MintDischarge should not have returned a discharge")
	}
}
