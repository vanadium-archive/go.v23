package security

import (
	"bytes"
	"testing"
	"time"

	"veyron.io/veyron/veyron2/vom"
)

func TestCaveats(t *testing.T) {
	var (
		ctx = &context{method: "Foo", name: "myobj", localID: FakePublicID("alice/phone/friend")}
		C   = func(caveat Caveat, err error) Caveat {
			if err != nil {
				t.Fatal(err)
			}
			return caveat
		}
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
