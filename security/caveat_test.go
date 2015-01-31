package security

import (
	"reflect"
	"testing"
	"time"

	"v.io/core/veyron2/vom"
)

func TestCaveats(t *testing.T) {
	var (
		self = newPrincipal(t)
		now  = time.Now()
		ctx  = NewContext(&ContextParams{
			Timestamp:      now,
			Method:         "Foo",
			LocalPrincipal: self,
			LocalBlessings: blessSelf(t, self, "alice/phone/friend"),
		})
		C     = newCaveat
		tests = []struct {
			cav Caveat
			ok  bool
		}{
			// NewCaveat
			{C(NewCaveat(unixTimeExpiryCaveat(now.Add(time.Second).Unix()))), true},
			// ExpiryCaveat
			{C(ExpiryCaveat(now.Add(time.Second))), true},
			{C(ExpiryCaveat(now.Add(-1 * time.Second))), false},
			// MethodCaveat
			{C(MethodCaveat("Foo")), true},
			{C(MethodCaveat("Bar")), false},
			{C(MethodCaveat("Foo", "Bar")), true},
			{C(MethodCaveat("Bar", "Baz")), false},
		}
	)
	self.AddToRoots(ctx.LocalBlessings())
	for idx, test := range tests {
		var validator CaveatValidator
		if err := vom.Decode(test.cav.ValidatorVOM, &validator); err != nil {
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
		now          = time.Now()
		valid        = newCaveat(ExpiryCaveat(now.Add(time.Second)))
		expired      = newCaveat(ExpiryCaveat(now.Add(-1 * time.Second)))
		discharger   = newPrincipal(t)
		randomserver = newPrincipal(t)
		ctx          = func(method string, discharges ...Discharge) Context {
			params := &ContextParams{
				Timestamp:        now,
				Method:           method,
				RemoteDischarges: make(map[string]Discharge),
			}
			for _, d := range discharges {
				params.RemoteDischarges[d.ID()] = d
			}
			return NewContext(params)
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
	if merr := matchesError(tpc.Dischargeable(NewContext(&ContextParams{Timestamp: now})), "could not validate embedded restriction security.unixTimeExpiryCaveat"); merr != nil {
		t.Fatal(merr)
	}
}

func TestThirdPartyDetails(t *testing.T) {
	niltests := []Caveat{
		newCaveat(ExpiryCaveat(time.Now())),
		newCaveat(MethodCaveat("Foo", "Bar")),
	}
	for _, c := range niltests {
		if tp := c.ThirdPartyDetails(); tp != nil {
			t.Errorf("Caveat [%v] recognized as a third-party caveat: %v", c, tp)
		}
	}
	req := ThirdPartyRequirements{ReportMethod: true}
	tp, err := NewPublicKeyCaveat(newPrincipal(t).PublicKey(), "location", req, newCaveat(ExpiryCaveat(time.Now())))
	if err != nil {
		t.Fatal(err)
	}
	c := newCaveat(NewCaveat(tp))
	if got := c.ThirdPartyDetails(); !reflect.DeepEqual(got, tp) {
		t.Errorf("Got %v, want %v", got, tp)
	}
}
