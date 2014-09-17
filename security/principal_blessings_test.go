package security

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

func TestBlessSelf(t *testing.T) {
	p := newPrincipal(t)
	alice, err := p.BlessSelf("alice", newCaveat(MethodCaveat("Method")))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(alice.PublicKey(), p.PublicKey()) {
		t.Errorf("Public key mismatch. Principal: %v, Blessing: %v", p.PublicKey(), alice.PublicKey())
	}
	if err := checkBlessings(alice, &context{method: "Method"}, "alice"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(alice, &context{method: "Foo"}); err != nil {
		t.Error(err)
	}
}

func TestBless(t *testing.T) {
	var (
		p1    = newPrincipal(t)
		p2    = newPrincipal(t)
		p3    = newPrincipal(t)
		alice = blessSelf(t, p1, "alice")
	)
	// p1 blessing p2 as "alice/friend" for "Suffix.Method"
	friend, err := p1.Bless(p2.PublicKey(), alice, "friend", newCaveat(MethodCaveat("Method")), newSuffixCaveat("Suffix"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(friend.PublicKey(), p2.PublicKey()) {
		t.Errorf("Public key mismatch. Principal: %v, Blessing: %v", p2.PublicKey(), friend.PublicKey())
	}
	if err := checkBlessings(friend, &context{method: "Method"}); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, &context{suffix: "Suffix"}); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, &context{method: "Method", suffix: "Suffix"}, "alice/friend"); err != nil {
		t.Error(err)
	}
	// p1.Bless should not mess with the certificate chains of "alice" itself.
	if err := checkBlessings(alice, &context{}, "alice"); err != nil {
		t.Error(err)
	}

	// p2 should not be able to bless p3 as "alice/friend"
	blessing, err := p2.Bless(p3.PublicKey(), alice, "friend", UnconstrainedDelegation())
	if blessing != nil {
		t.Errorf("p2 was able to extend a blessing bound to p1 to produce: %v", blessing)
	}
	if err := matchesError(err, "cannot extend blessing with public key"); err != nil {
		t.Fatal(err)
	}
}

func TestBlessings(t *testing.T) {
	var (
		p     = newPrincipal(t)
		p2    = newPrincipal(t).PublicKey()
		alice = blessSelf(t, p, "alice")
		valid = []string{
			"a",
			"john.doe",
			"bruce@wayne.com",
			"bugs..bunny",
			"trusted/friends",
			"friends/colleagues/work",
		}
		invalid = []string{
			"",
			"...",
			"/",
			"bugs...bunny",
			"/bruce",
			"bruce/",
			"trusted//friends",
		}
	)

	for _, test := range valid {
		self, err := p.BlessSelf(test)
		if err != nil {
			t.Errorf("BlessSelf(%q) failed: %v", test, err)
			continue
		}
		if err := checkBlessings(self, &context{}, test); err != nil {
			t.Errorf("BlessSelf(%q): %v)", test, err)
		}
		other, err := p.Bless(p2, alice, test, UnconstrainedDelegation())
		if err != nil {
			t.Errorf("Bless(%q) failed: %v", test, err)
			continue
		}
		if err := checkBlessings(other, &context{}, fmt.Sprintf("alice%v%v", ChainSeparator, test)); err != nil {
			t.Errorf("Bless(%q): %v", test, err)
		}
	}

	for _, test := range invalid {
		self, err := p.BlessSelf(test)
		if merr := matchesError(err, "invalid blessing extension"); merr != nil {
			t.Errorf("BlessSelf(%q): %v", test, merr)
		} else if self != nil {
			t.Errorf("BlessSelf(%q) returned %q", test, self)
		}
		other, err := p.Bless(p2, alice, test, UnconstrainedDelegation())
		if merr := matchesError(err, "invalid blessing extension"); merr != nil {
			t.Errorf("Bless(%q): %v", test, merr)
		} else if other != nil {
			t.Errorf("Bless(%q) returned %q", test, other)
		}
	}
}

func TestPrincipalSign(t *testing.T) {
	var (
		p       = newPrincipal(t)
		message = make([]byte, 10)
	)
	if sig, err := p.Sign(message); err != nil {
		t.Error(err)
	} else if !sig.Verify(p.PublicKey(), message) {
		t.Errorf("Signature is not valid for message that was signed")
	}
}

func TestPrincipalSignaturePurpose(t *testing.T) {
	// Ensure that logically different private key operations result in different purposes in the signatures.
	p := newPrincipal(t)

	// signPurpose for Sign
	if sig, err := p.Sign(make([]byte, 1)); err != nil {
		t.Error(err)
	} else if !bytes.Equal(sig.Purpose, signPurpose) {
		t.Errorf("Sign returned signature with purpose %q, want %q", sig.Purpose, signPurpose)
	}

	// blessPurpose for Bless (and BlessSelf)
	selfBlessing, err := p.BlessSelf("foo")
	if err != nil {
		t.Fatal(err)
	}
	if sig := selfBlessing.(*blessingsImpl).chains[0][0].Signature; !bytes.Equal(sig.Purpose, blessPurpose) {
		t.Errorf("BlessSelf used signature with purpose %q, want %q", sig.Purpose, blessPurpose)
	}
	otherBlessing, err := p.Bless(newPrincipal(t).PublicKey(), selfBlessing, "bar", UnconstrainedDelegation())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // Should be precisely 2 certificates in "otherBlessing"
		cert := otherBlessing.(*blessingsImpl).chains[0][i]
		if !bytes.Equal(cert.Signature.Purpose, blessPurpose) {
			t.Errorf("Certificate with purpose %q, want %q", cert.Signature.Purpose, blessPurpose)
		}
	}
}

func TestUnionOfBlessings(t *testing.T) {
	// A bunch of principals bless p
	var (
		p1    = newPrincipal(t)
		p2    = newPrincipal(t)
		alice = blessSelf(t, p1, "alice")
		bob   = blessSelf(t, p2, "bob")
		p     = newPrincipal(t)
	)
	alicefriend, err := p1.Bless(p.PublicKey(), alice, "friend", newCaveat(MethodCaveat("Method")))
	if err != nil {
		t.Fatal(err)
	}

	bobfriend, err := p2.Bless(p.PublicKey(), bob, "friend", newSuffixCaveat("Suffix"))
	if err != nil {
		t.Fatal(err)
	}
	friend, err := UnionOfBlessings(alicefriend, bobfriend, blessSelf(t, p, "carol"))
	if err != nil {
		t.Fatal(err)
	}
	if err := checkBlessings(friend, &context{}, "carol"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, &context{method: "Method"}, "alice/friend", "carol"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, &context{suffix: "Suffix"}, "bob/friend", "carol"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, &context{method: "Method", suffix: "Suffix"}, "alice/friend", "bob/friend", "carol"); err != nil {
		t.Error(err)
	}

	// p can bless p3 further
	spouse, err := p.Bless(newPrincipal(t).PublicKey(), friend, "spouse", newCaveat(PeerBlessingsCaveat("fake/peer")))
	if err != nil {
		t.Fatal(err)
	}
	server := FakePublicID("peer")
	if err := checkBlessings(spouse, &context{}); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(spouse, &context{localID: server}, "carol/spouse"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(spouse, &context{method: "Method", localID: server}, "alice/friend/spouse", "carol/spouse"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(spouse, &context{suffix: "Suffix", localID: server}, "bob/friend/spouse", "carol/spouse"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(spouse, &context{suffix: "Suffix", method: "Method", localID: server}, "alice/friend/spouse", "bob/friend/spouse", "carol/spouse"); err != nil {
		t.Error(err)
	}

	// However, UnionOfBlessings must not mix up public keys
	mixed, err := UnionOfBlessings(alice, bob)
	if berr := matchesError(err, "mismatched public keys"); berr != nil || mixed != nil {
		t.Errorf("%v(%v)", berr, mixed)
	}
}

func TestCertificateCompositionAttack(t *testing.T) {
	var (
		p1    = newPrincipal(t)
		alice = blessSelf(t, p1, "alice")
		p2    = newPrincipal(t)
		bob   = blessSelf(t, p2, "bob")
		p3    = newPrincipal(t)
		p4    = newPrincipal(t)
		ctx   = &context{method: "Foo"}
	)
	// p3 has the blessings "alice/friend" and "bob/family" (from p1 and p2 respectively).
	// It then blesses p4 as "alice/friend/spouse" with no caveat and as "bob/family/spouse"
	// with a caveat.
	alicefriend, err := p1.Bless(p3.PublicKey(), alice, "friend", UnconstrainedDelegation())
	if err != nil {
		t.Fatal(err)
	}
	bobfamily, err := p2.Bless(p3.PublicKey(), bob, "family", UnconstrainedDelegation())
	if err != nil {
		t.Fatal(err)
	}

	alicefriendspouse, err := p3.Bless(p4.PublicKey(), alicefriend, "spouse", UnconstrainedDelegation())
	if err != nil {
		t.Fatal(err)
	}
	bobfamilyspouse, err := p3.Bless(p4.PublicKey(), bobfamily, "spouse", newCaveat(MethodCaveat("Foo")))
	if err != nil {
		t.Fatal(err)
	}
	// p4's blessings should be valid.
	if err := checkBlessings(alicefriendspouse, ctx, "alice/friend/spouse"); err != nil {
		t.Fatal(err)
	}
	if err := checkBlessings(bobfamilyspouse, ctx, "bob/family/spouse"); err != nil {
		t.Fatal(err)
	}

	// p4 should be not to construct a valid "bob/family/spouse" blessing by
	// using the "spouse" certificate from "alice/friend/spouse" (that has no caveats)
	// and replacing the "spouse" certificate from "bob/family/spouse".
	spousecert := alicefriendspouse.(*blessingsImpl).chains[0][2]
	// sanity check
	if spousecert.Extension != "spouse" || len(spousecert.Caveats) != 0 {
		t.Fatalf("Invalid test data. Certificate: %+v", spousecert)
	}
	// Replace the certificate in bobfamilyspouse
	bobfamilyspouse.(*blessingsImpl).chains[0][2] = spousecert
	if err := matchesError(checkBlessings(bobfamilyspouse, ctx), "invalid Signature in certificate(for \"spouse\")"); err != nil {
		t.Fatal(err)
	}
}

func TestCertificateTamperingAttack(t *testing.T) {
	var (
		p1 = newPrincipal(t)
		p2 = newPrincipal(t)
		p3 = newPrincipal(t)

		alice = blessSelf(t, p1, "alice")
	)

	alicefriend, err := p1.Bless(p2.PublicKey(), alice, "friend", UnconstrainedDelegation())
	if err != nil {
		t.Fatal(err)
	}
	if err := checkBlessings(alicefriend, &context{}, "alice/friend"); err != nil {
		t.Fatal(err)
	}
	// p3 attempts to "steal" the blessing by constructing his own certificate.
	cert := &alicefriend.(*blessingsImpl).chains[0][1]
	if cert.PublicKey, err = p3.PublicKey().MarshalBinary(); err != nil {
		t.Fatal(err)
	}
	if err := matchesError(checkBlessings(alicefriend, &context{}, "alice/friend"), "invalid Signature in certificate(for \"friend\")"); err != nil {
		t.Error(err)
	}
}

func TestCertificateChainsTamperingAttack(t *testing.T) {
	var (
		p1    = newPrincipal(t)
		p2    = newPrincipal(t)
		alice = blessSelf(t, p1, "alice")
		bob   = blessSelf(t, p2, "bob")
	)
	if err := checkBlessings(alice, &context{}, "alice"); err != nil {
		t.Fatal(err)
	}
	// Act as if alice tried to package bob's chain with her existing chains and ship it over the network.
	alice.(*blessingsImpl).chains = append(alice.(*blessingsImpl).chains, bob.(*blessingsImpl).chains...)
	if err := matchesError(checkBlessings(alice, &context{}, "alice", "bob"), "two certificate chains that bind to different public keys"); err != nil {
		t.Error(err)
	}
}

func TestThirdPartyCaveats(t *testing.T) {
	// TODO(ashankar,ataly): Implement this once we've figured out what aspects of third-party caveats go into veyron2/security
}
