package security

import (
	"bytes"
	"crypto/elliptic"
	"fmt"
	"reflect"
	"testing"

	"veyron.io/veyron/veyron2/vom"
)

func TestBlessSelf(t *testing.T) {
	var (
		tp = newPrincipalWithRoots(t) // principal where blessings are tested
		p  = newPrincipal(t)
	)

	alice, err := p.BlessSelf("alice", newCaveat(MethodCaveat("Method")))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(alice.PublicKey(), p.PublicKey()) {
		t.Errorf("Public key mismatch. Principal: %v, Blessing: %v", p.PublicKey(), alice.PublicKey())
	}
	if err := checkBlessings(alice, &context{local: tp, method: "Foo"}); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(alice, &context{local: tp, method: "Method"}); err != nil {
		t.Error(err)
	}
	addToRoots(t, tp, alice)
	if err := checkBlessings(alice, &context{local: tp, method: "Foo"}); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(alice, &context{local: tp, method: "Method"}, "alice"); err != nil {
		t.Error(err)
	}
}

func TestBless(t *testing.T) {
	var (
		tp = newPrincipalWithRoots(t) // principal where blessings are tested

		p1    = newPrincipal(t)
		p2    = newPrincipal(t)
		p3    = newPrincipal(t)
		alice = blessSelf(t, p1, "alice")
	)
	addToRoots(t, tp, alice)
	// p1 blessing p2 as "alice/friend" for "Suffix.Method"
	friend, err := p1.Bless(p2.PublicKey(), alice, "friend", newCaveat(MethodCaveat("Method")), newSuffixCaveat("Suffix"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(friend.PublicKey(), p2.PublicKey()) {
		t.Errorf("Public key mismatch. Principal: %v, Blessing: %v", p2.PublicKey(), friend.PublicKey())
	}
	if err := checkBlessings(friend, &context{local: tp, method: "Method"}); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, &context{local: tp, suffix: "Suffix"}); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, &context{local: tp, method: "Method", suffix: "Suffix"}, "alice/friend"); err != nil {
		t.Error(err)
	}
	// p1.Bless should not mess with the certificate chains of "alice" itself.
	if err := checkBlessings(alice, &context{local: tp}, "alice"); err != nil {
		t.Error(err)
	}

	// p2 should not be able to bless p3 as "alice/friend"
	blessing, err := p2.Bless(p3.PublicKey(), alice, "friend", UnconstrainedUse())
	if blessing != nil {
		t.Errorf("p2 was able to extend a blessing bound to p1 to produce: %v", blessing)
	}
	if err := matchesError(err, "cannot extend blessing with public key"); err != nil {
		t.Fatal(err)
	}
}

func TestBlessings(t *testing.T) {
	type s []string

	var (
		tp = newPrincipalWithRoots(t) // principal where blessings are tested

		p     = newPrincipal(t)
		p2    = newPrincipal(t).PublicKey()
		alice = blessSelf(t, p, "alice")
		valid = s{
			"a",
			"john.doe",
			"bruce@wayne.com",
			"bugs..bunny",
			"trusted/friends",
			"friends/colleagues/work",
		}
		invalid = s{
			"",
			"...",
			"/",
			"bugs...bunny",
			"/bruce",
			"bruce/",
			"trusted//friends",
		}
	)
	addToRoots(t, tp, alice)
	for _, test := range valid {
		self, err := p.BlessSelf(test)
		if err != nil {
			t.Errorf("BlessSelf(%q) failed: %v", test, err)
			continue
		}
		addToRoots(t, tp, self)
		if err := checkBlessings(self, &context{local: tp}, test); err != nil {
			t.Errorf("BlessSelf(%q): %v)", test, err)
		}
		other, err := p.Bless(p2, alice, test, UnconstrainedUse())
		if err != nil {
			t.Errorf("Bless(%q) failed: %v", test, err)
			continue
		}
		if err := checkBlessings(other, &context{local: tp}, fmt.Sprintf("alice%v%v", ChainSeparator, test)); err != nil {
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
		other, err := p.Bless(p2, alice, test, UnconstrainedUse())
		if merr := matchesError(err, "invalid blessing extension"); merr != nil {
			t.Errorf("Bless(%q): %v", test, merr)
		} else if other != nil {
			t.Errorf("Bless(%q) returned %q", test, other)
		}
	}
}

func TestCreatePrincipalWithNilStoreAndRoots(t *testing.T) {
	p, err := CreatePrincipal(newECDSASigner(t, elliptic.P256()), nil, nil)
	if err != nil {
		t.Fatalf("CreatePrincipal failed: %v", err)
	}

	// Test Roots.
	r := p.Roots()
	if r == nil {
		t.Fatal("Roots() returned nil")
	}
	wantErr := "BlessingRoots object is nil"
	if err := matchesError(r.Add(nil, ""), wantErr); err != nil {
		t.Error(err)
	}
	if err := matchesError(r.Recognized(nil, ""), wantErr); err != nil {
		t.Error(err)
	}

	// Test Store.
	s := p.BlessingStore()
	if r == nil {
		t.Fatal("BlessingStore() returned nil")
	}
	wantErr = "BlessingStore object is nil"
	if err := matchesError(s.Add(nil, ""), wantErr); err != nil {
		t.Error(err)
	}
	if err := matchesError(s.SetDefault(nil), wantErr); err != nil {
		t.Error(err)
	}
	if got := s.ForPeer(); got != nil {
		t.Errorf("BlessingStore.ForPeer: got %v want nil", got)
	}
	if got := s.Default(); got != nil {
		t.Errorf("BlessingStore.Default: got %v want nil", got)
	}
	if got, want := s.PublicKey(), p.PublicKey(); !reflect.DeepEqual(got, want) {
		t.Errorf("BlessingStore.PublicKey: got %v want %v", got, want)
	}

	// Test that no blessings are trusted by the principal.
	if err := checkBlessings(blessSelf(t, p, "alice"), &context{local: p}); err != nil {
		t.Error(err)
	}
}

func TestAddToRoots(t *testing.T) {
	type s []string
	var (
		p1          = newPrincipal(t)
		aliceFriend = blessSelf(t, p1, "alice/friend")

		p2      = newPrincipal(t)
		charlie = blessSelf(t, p2, "charlie")

		p3 = newPrincipal(t).PublicKey()
	)
	aliceFriendSpouse, err := p1.Bless(p3, aliceFriend, "spouse", UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	charlieFamilyDaughter, err := p2.Bless(p3, charlie, "family/daughter", UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		add           Blessings
		root          PublicKey
		recognized    []string
		notRecognized []string
	}{
		{
			add:           aliceFriendSpouse,
			root:          p1.PublicKey(),
			recognized:    s{"alice", "alice/friend", "alice/friend/device", "alice/friend/device/app", "alice/friend/spouse", "alice/friend/spouse/friend"},
			notRecognized: s{"alice/device", "bob", "bob/friend", "bob/friend/spouse"},
		},
		{
			add:           charlieFamilyDaughter,
			root:          p2.PublicKey(),
			recognized:    s{"charlie", "charlie/friend", "charlie/friend/device", "charlie/family", "charlie/family/daughter", "charlie/family/friend", "charlie/family/friend/device"},
			notRecognized: s{"alice", "bob", "alice/family", "alice/family/daughter"},
		},
	}
	for _, test := range tests {
		tp := newPrincipalWithRoots(t) // principal where roots are tested.
		if err := tp.AddToRoots(test.add); err != nil {
			t.Error(err)
			continue
		}
		for _, b := range test.recognized {
			if tp.Roots().Recognized(test.root, b) != nil {
				t.Errorf("added roots for: %v but did not recognize blessing: %v", test.add, b)
			}
		}
		for _, b := range test.notRecognized {
			if tp.Roots().Recognized(test.root, b) == nil {
				t.Errorf("added roots for: %v but recognized blessing: %v", test.add, b)
			}
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
	otherBlessing, err := p.Bless(newPrincipal(t).PublicKey(), selfBlessing, "bar", UnconstrainedUse())
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
	principalTrustingRootsOf := func(roots ...Blessings) Principal {
		if roots == nil {
			return newPrincipal(t)
		}
		p := newPrincipalWithRoots(t)
		for _, r := range roots {
			addToRoots(t, p, r)
		}
		return p
	}
	// A bunch of principals bless p
	var (
		p1    = newPrincipal(t)
		p2    = newPrincipal(t)
		alice = blessSelf(t, p1, "alice")
		bob   = blessSelf(t, p2, "bob")
		p     = newPrincipal(t)
		carol = blessSelf(t, p, "carol")
	)
	alicefriend, err := p1.Bless(p.PublicKey(), alice, "friend", newCaveat(MethodCaveat("Method")))
	if err != nil {
		t.Fatal(err)
	}

	bobfriend, err := p2.Bless(p.PublicKey(), bob, "friend", newSuffixCaveat("Suffix"))
	if err != nil {
		t.Fatal(err)
	}
	friend, err := UnionOfBlessings(alicefriend, bobfriend, carol)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkBlessings(friend, &context{local: principalTrustingRootsOf()}); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, &context{local: principalTrustingRootsOf(alice, bob)}); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, &context{local: principalTrustingRootsOf(carol)}, "carol"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, &context{local: principalTrustingRootsOf(alice), method: "Method"}, "alice/friend"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, &context{local: principalTrustingRootsOf(alice, carol), method: "Method"}, "alice/friend", "carol"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, &context{local: principalTrustingRootsOf(bob), suffix: "Suffix"}, "bob/friend"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, &context{local: principalTrustingRootsOf(bob, carol), suffix: "Suffix"}, "bob/friend", "carol"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, &context{local: principalTrustingRootsOf(alice, bob, carol), method: "Method", suffix: "Suffix"}, "alice/friend", "bob/friend", "carol"); err != nil {
		t.Error(err)
	}

	// p can bless p3 further
	spouse, err := p.Bless(newPrincipal(t).PublicKey(), friend, "spouse", newCaveat(PeerBlessingsCaveat("fake/peer")))
	if err != nil {
		t.Fatal(err)
	}
	server := FakePublicID("peer")
	if err := checkBlessings(spouse, &context{local: principalTrustingRootsOf()}); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(spouse, &context{local: principalTrustingRootsOf(carol), localID: server}, "carol/spouse"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(spouse, &context{local: principalTrustingRootsOf(alice, carol), method: "Method", localID: server}, "alice/friend/spouse", "carol/spouse"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(spouse, &context{local: principalTrustingRootsOf(bob, carol), suffix: "Suffix", localID: server}, "bob/friend/spouse", "carol/spouse"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(spouse, &context{local: principalTrustingRootsOf(alice, bob, carol), suffix: "Suffix", method: "Method", localID: server}, "alice/friend/spouse", "bob/friend/spouse", "carol/spouse"); err != nil {
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
		tp = newPrincipalWithRoots(t) // principal for testing blessings.

		p1    = newPrincipal(t)
		alice = blessSelf(t, p1, "alice")
		p2    = newPrincipal(t)
		bob   = blessSelf(t, p2, "bob")
		p3    = newPrincipal(t)
		p4    = newPrincipal(t)
		ctx   = &context{method: "Foo", local: tp}
	)
	addToRoots(t, tp, alice)
	addToRoots(t, tp, bob)
	// p3 has the blessings "alice/friend" and "bob/family" (from p1 and p2 respectively).
	// It then blesses p4 as "alice/friend/spouse" with no caveat and as "bob/family/spouse"
	// with a caveat.
	alicefriend, err := p1.Bless(p3.PublicKey(), alice, "friend", UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	bobfamily, err := p2.Bless(p3.PublicKey(), bob, "family", UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}

	alicefriendspouse, err := p3.Bless(p4.PublicKey(), alicefriend, "spouse", UnconstrainedUse())
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
		tp = newPrincipalWithRoots(t) // principal for testing blessings.

		p1 = newPrincipal(t)
		p2 = newPrincipal(t)
		p3 = newPrincipal(t)

		alice = blessSelf(t, p1, "alice")
	)
	addToRoots(t, tp, alice)

	alicefriend, err := p1.Bless(p2.PublicKey(), alice, "friend", UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	if err := checkBlessings(alicefriend, &context{local: tp}, "alice/friend"); err != nil {
		t.Fatal(err)
	}
	// p3 attempts to "steal" the blessing by constructing his own certificate.
	cert := &alicefriend.(*blessingsImpl).chains[0][1]
	if cert.PublicKey, err = p3.PublicKey().MarshalBinary(); err != nil {
		t.Fatal(err)
	}
	if err := matchesError(checkBlessings(alicefriend, &context{local: tp}, "alice/friend"), "invalid Signature in certificate(for \"friend\")"); err != nil {
		t.Error(err)
	}
}

func TestCertificateChainsTamperingAttack(t *testing.T) {
	var (
		tp = newPrincipalWithRoots(t) // principal for testing blessings.

		p1    = newPrincipal(t)
		p2    = newPrincipal(t)
		alice = blessSelf(t, p1, "alice")
		bob   = blessSelf(t, p2, "bob")
	)
	addToRoots(t, tp, alice)
	addToRoots(t, tp, bob)

	if err := checkBlessings(alice, &context{local: tp}, "alice"); err != nil {
		t.Fatal(err)
	}
	// Act as if alice tried to package bob's chain with her existing chains and ship it over the network.
	alice.(*blessingsImpl).chains = append(alice.(*blessingsImpl).chains, bob.(*blessingsImpl).chains...)
	if err := matchesError(checkBlessings(alice, &context{local: tp}, "alice", "bob"), "two certificate chains that bind to different public keys"); err != nil {
		t.Error(err)
	}
}

func TestBlessingsOnWire(t *testing.T) {
	b, err := newPrincipal(t).BlessSelf("self")
	if err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	if err := vom.NewEncoder(buf).Encode(b); err != nil {
		t.Fatal(err)
	}
	// Even though the Blessings object was encoded "directly", should be
	// able to decode into the concrete type of the wire representation.
	var wire WireBlessings
	if err := vom.NewDecoder(buf).Decode(&wire); err != nil {
		t.Fatal(err)
	}
	got, err := NewBlessings(wire)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b, got) {
		t.Fatalf("Got %#v, want %#v", got, b)
	}
	// Putzing around with the wire representation should break the factory function.
	otherkey, err := newPrincipal(t).PublicKey().MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	wire.CertificateChains[0][len(wire.CertificateChains[0])-1].PublicKey = otherkey
	got, err = NewBlessings(wire)
	if merr := matchesError(err, "invalid Signature in certificate"); merr != nil || got != nil {
		t.Errorf("Got (%v, %v): %v", got, err, merr)
	}
}

func TestBlessingsOnWireWithMissingCertificates(t *testing.T) {
	var (
		B = func(b Blessings, err error) Blessings {
			if err != nil {
				t.Fatal(err)
			}
			return b
		}

		// Create "leaf", a blessing involving three certificates that bind the name
		// root/middleman/leaf to the leaf principal.
		rootP      = newPrincipal(t)
		middlemanP = newPrincipal(t)
		leafP      = newPrincipal(t)
		root       = B(rootP.BlessSelf("root"))
		middleman  = B(rootP.Bless(middlemanP.PublicKey(), root, "middleman", UnconstrainedUse()))
		leaf       = B(middlemanP.Bless(leafP.PublicKey(), middleman, "leaf", UnconstrainedUse()))

		buf  = new(bytes.Buffer)
		wire WireBlessings
	)
	if err := vom.NewEncoder(buf).Encode(leaf); err != nil {
		t.Fatal(err)
	}
	if err := vom.NewDecoder(buf).Decode(&wire); err != nil {
		t.Fatal(err)
	}
	// Phew! We should have a certificate chain of size 3.
	chain := wire.CertificateChains[0]
	if len(chain) != 3 {
		t.Fatalf("Got a chain of %d certificates, want 3", len(chain))
	}

	C1, C2, C3 := chain[0], chain[1], chain[2]
	var CX Certificate
	// The following combinations should fail because a certificate is missing
	type C []Certificate
	tests := []struct {
		Chain []Certificate
		Err   string
	}{
		{C{}, "empty certificate chain"}, // Empty chain
		{C{C1, C3}, "invalid Signature"}, // Missing link in the chain
		{C{C2, C3}, "invalid Signature"},
		{C{CX, C2, C3}, "syntax error"},
		{C{C1, CX, C3}, "signature"},
		{C{C1, C2, CX}, "signature"},
		{C{C1, C2, C3}, ""}, // Valid chain
	}
	for idx, test := range tests {
		wire.CertificateChains[0] = test.Chain
		_, err := NewBlessings(wire)
		if merr := matchesError(err, test.Err); merr != nil {
			t.Errorf("(%d) %v [%v]", idx, merr, test.Chain)
		}
	}

	// Mulitple chains, certifying different keys should fail
	wire.CertificateChains = [][]Certificate{
		C{C1},
		C{C1, C2},
		C{C1, C2, C3},
	}
	_, err := NewBlessings(wire)
	if merr := matchesError(err, "bind to different public keys"); merr != nil {
		t.Error(err)
	}

	// Multiple chains certifying the same key are okay
	wire.CertificateChains = [][]Certificate{chain, chain, chain}
	if _, err := NewBlessings(wire); err != nil {
		t.Error(err)
	}
	// But leaving any empty chains is not okay
	for idx := 0; idx < len(wire.CertificateChains); idx++ {
		wire.CertificateChains[idx] = []Certificate{}
		_, err := NewBlessings(wire)
		if merr := matchesError(err, "empty certificate chain"); merr != nil {
			t.Errorf("%d: %v", idx, merr)
		}
		wire.CertificateChains[idx] = chain
	}
}
