package security

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"v.io/core/veyron2/uniqueid"
	"v.io/core/veyron2/vdl"
	"v.io/core/veyron2/verror"
)

func TestStandardCaveatFactories(t *testing.T) {
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
		err := test.cav.Validate(ctx)
		if test.ok && err != nil {
			t.Errorf("#%d: %v.Validate(...) failed validation: %v", idx, test.cav, err)
		} else if !test.ok && !verror.Is(err, ErrCaveatValidation.ID) {
			t.Errorf("#%d: %v.Validate(...) returned error='%v' (errorid=%v), want errorid=%v", idx, test.cav, err, verror.ErrorID(err), ErrCaveatValidation.ID)
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
	// Discharge can be converted to and from wire format:
	if d2, err := NewDischarge(MarshalDischarge(d)); err != nil {
		t.Error(err)
	} else if !reflect.DeepEqual(d, d2) {
		t.Errorf("Got %#v, want %#v", d2, d)
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
	if merr := matchesError(tpc.ThirdPartyDetails().Dischargeable(NewContext(&ContextParams{Timestamp: now})), "could not validate embedded restriction"); merr != nil {
		t.Fatal(merr)
	}
}

func TestCaveat(t *testing.T) {
	uid, err := uniqueid.Random()
	if err != nil {
		t.Fatal(err)
	}
	cd := CaveatDescriptor{
		Id:        uid,
		ParamType: vdl.TypeOf(string("")),
	}
	if _, err := NewCaveat(cd, nil); !verror.Is(err, ErrCaveatParamTypeMismatch.ID) {
		t.Errorf("Got '%v' (errorid=%v), want errorid=%v", err, verror.ErrorID(err), ErrCaveatParamTypeMismatch.ID)
	}
	// A param of type *vdl.Value with underlying type string should succeed.
	if _, err := NewCaveat(cd, vdl.StringValue("")); err != nil {
		t.Errorf("vdl value should have succeeded: %v", err)
	}
	ctx := NewContext(&ContextParams{
		Method: "Foo",
	})
	c1, err := NewCaveat(cd, "Foo")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := NewCaveat(cd, "Bar")
	if err != nil {
		t.Fatal(err)
	}
	// Validation will fail when the validation function isn't registered.
	if err := c1.Validate(ctx); !verror.Is(err, ErrCaveatNotRegistered.ID) {
		t.Errorf("Got '%v' (errorid=%v), want errorid=%v", err, verror.ErrorID(err), ErrCaveatNotRegistered.ID)
	}
	// Once registered, then it should be invoked
	RegisterCaveatValidator(cd, func(ctx Context, param string) error {
		if ctx.Method() == param {
			return nil
		}
		return fmt.Errorf("na na na")
	})
	if err := c1.Validate(ctx); err != nil {
		t.Error(err)
	}
	if err := c2.Validate(ctx); !verror.Is(err, ErrCaveatValidation.ID) {
		t.Errorf("Got '%v' (errorid=%v), want errorid=%v", err, verror.ErrorID(err), ErrCaveatValidation.ID)
	}
	// If a malformed caveat was received, then validation should fail
	c3 := Caveat{Id: cd.Id, ParamVom: nil}
	if err := c3.Validate(ctx); !verror.Is(err, ErrCaveatParamCoding.ID) {
		t.Errorf("Got '%v' (errorid=%v), want errorid=%v", err, verror.ErrorID(err), ErrCaveatParamCoding.ID)
	}
}

func TestRegisterCaveat(t *testing.T) {
	uid, err := uniqueid.Random()
	if err != nil {
		t.Fatal(err)
	}
	var (
		cd = CaveatDescriptor{
			Id:        uid,
			ParamType: vdl.TypeOf(string("")),
		}
		npanics     int
		expectPanic = func(details string) {
			npanics++
			if err := recover(); err == nil {
				t.Errorf("%s: expected a panic", details)
			}
		}
	)
	func() {
		defer expectPanic("not a function")
		RegisterCaveatValidator(cd, "not a function")
	}()
	func() {
		defer expectPanic("wrong #outputs")
		RegisterCaveatValidator(cd, func(Context, string) (error, error) { return nil, nil })
	}()
	func() {
		defer expectPanic("bad output type")
		RegisterCaveatValidator(cd, func(Context, string) int { return 0 })
	}()
	func() {
		defer expectPanic("wrong #inputs")
		RegisterCaveatValidator(cd, func(Context, string, string) error { return nil })
	}()
	func() {
		defer expectPanic("bad input arg 0")
		RegisterCaveatValidator(cd, func(int, string) error { return nil })
	}()
	func() {
		defer expectPanic("bad input arg 1")
		RegisterCaveatValidator(cd, func(Context, int) error { return nil })
	}()
	func() {
		// Successful registration: No panic:
		RegisterCaveatValidator(cd, func(Context, string) error { return nil })
	}()
	func() {
		defer expectPanic("Duplication registration")
		RegisterCaveatValidator(cd, func(Context, string) error { return nil })
	}()
	if got, want := npanics, 7; got != want {
		t.Errorf("Got %d panics, want %d", got, want)
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
	c, err := NewPublicKeyCaveat(newPrincipal(t).PublicKey(), "location", req, newCaveat(ExpiryCaveat(time.Now())))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.ThirdPartyDetails(); got.Location() != "location" {
		t.Errorf("Got location %q, want %q", got.Location(), "location")
	} else if !reflect.DeepEqual(got.Requirements(), req) {
		t.Errorf("Got requirements %+v, want %+v", got.Requirements(), req)
	}
}

// Benchmark creation of a new caveat using one of the simplest caveats
// (expiry)
func BenchmarkNewCaveat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := NewCaveat(UnixTimeExpiryCaveatX, 0); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark caveat valdation using one of the simplest caveats (expiry).
func BenchmarkValidateCaveat(b *testing.B) {
	cav, err := NewCaveat(UnixTimeExpiryCaveatX, time.Now().Unix())
	if err != nil {
		b.Fatal(err)
	}
	ctx := NewContext(&ContextParams{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cav.Validate(ctx)
	}
}
