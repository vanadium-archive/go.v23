// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"testing"

	"v.io/v23/context"
)

func TestDefaultAuthorizer(t *testing.T) {
	var (
		pali = newPrincipal(t)
		pbob = newPrincipal(t)
		pche = newPrincipal(t)
		pdis = newPrincipal(t) // third-party caveat discharger

		che, _ = pche.BlessSelf("che")
		ali, _ = pali.BlessSelf("ali")
		bob, _ = pbob.BlessSelf("bob")

		tpcav  = mkThirdPartyCaveat(t, pdis.PublicKey(), "someLocation", UnconstrainedUse())
		dis, _ = pdis.MintDischarge(tpcav, UnconstrainedUse())
		dismap = map[string]Discharge{dis.ID(): dis}

		// bless(ali, bob, "friend") will generate a blessing for ali, calling him "bob/friend".
		bless = func(target, extend Blessings, extension string, caveats ...Caveat) Blessings {
			var p Principal
			switch extend.PublicKey() {
			case ali.PublicKey():
				p = pali
			case bob.PublicKey():
				p = pbob
			case che.PublicKey():
				p = pche
			default:
				panic(extend)
			}
			if len(caveats) == 0 {
				caveats = []Caveat{UnconstrainedUse()}
			}
			ret, err := p.Bless(target.PublicKey(), extend, extension, caveats[0], caveats[1:]...)
			if err != nil {
				panic(err)
			}
			return ret
		}

		U = func(blessings ...Blessings) Blessings {
			u, err := UnionOfBlessings(blessings...)
			if err != nil {
				panic(err)
			}
			return u
		}

		// Shorthands for getting blessings for Ali and Bob.
		A = func(as Blessings, extension string, caveats ...Caveat) Blessings {
			return bless(ali, as, extension, caveats...)
		}
		B = func(as Blessings, extension string, caveats ...Caveat) Blessings {
			return bless(bob, as, extension, caveats...)
		}

		authorizer defaultAuthorizer
	)
	// Make ali, bob (the two ends) recognize all three blessings
	for ip, p := range []Principal{pali, pbob} {
		for _, b := range []Blessings{ali, bob, che} {
			if err := p.AddToRoots(b); err != nil {
				t.Fatalf("%d: %v - %v", ip, b, err)
			}
		}
	}
	// All tests are run as if "ali" is the local end and "bob" is the remote.
	tests := []struct {
		local, remote Blessings
		call          CallParams
		authorized    bool
	}{
		{
			local:      ali,
			remote:     ali,
			authorized: true,
		},
		{
			local:      ali,
			remote:     bob,
			authorized: false,
		},
		{
			// ali talking to ali/friend (invalid caveat)
			local:      ali,
			remote:     B(ali, "friend", tpcav),
			authorized: false,
		},
		{
			// ali talking to ali/friend
			local:      ali,
			remote:     B(ali, "friend", tpcav),
			call:       CallParams{RemoteDischarges: dismap},
			authorized: true,
		},
		{
			// bob/friend talking to bob (local blessing has an invalid caveat, but it is not checked)
			local:      A(bob, "friend", tpcav),
			remote:     bob,
			authorized: true,
		},
		{
			// che/friend talking to che/family
			local:      A(che, "friend"),
			remote:     B(che, "family"),
			authorized: false,
		},
		{
			// {ali, bob/friend, che/friend} talking to {bob/friend/spouse, che/family}
			local:      U(ali, A(bob, "friend"), A(che, "friend")),
			remote:     U(B(bob, "friend/spouse", tpcav), B(che, "family")),
			call:       CallParams{RemoteDischarges: dismap},
			authorized: true,
		},
	}
	for _, test := range tests {
		test.call.LocalPrincipal = pali
		test.call.LocalBlessings = test.local
		test.call.RemoteBlessings = test.remote
		ctx, cancel := context.RootContext()
		err := authorizer.Authorize(SetCall(ctx, NewCall(&test.call)))
		if (err == nil) != test.authorized {
			t.Errorf("call: %v. Got %v", test.call, err)
		}
		cancel()
	}
}

func mkThirdPartyCaveat(t *testing.T, discharger PublicKey, location string, c Caveat) Caveat {
	tpc, err := NewPublicKeyCaveat(discharger, location, ThirdPartyRequirements{}, c)
	if err != nil {
		t.Fatal(err)
	}
	return tpc
}