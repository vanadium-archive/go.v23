// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
	"time"

	"v.io/v23/vom"
)

func newSigner() Signer {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	return NewInMemoryECDSASigner(key)
}

// Log the "on-the-wire" sizes for blessings (which are shipped during the
// authentication protocol).
// As of February 27, 2015, the numbers were:
//   Marshaled P256 ECDSA key                   :   91 bytes
//   Major components of an ECDSA signature     :   64 bytes
//   VOM type information overhead for blessings:  354 bytes
//   Blessing with 1 certificates               :  536 bytes (a)
//   Blessing with 2 certificates               :  741 bytes (a/a)
//   Blessing with 3 certificates               :  945 bytes (a/a/a)
//   Blessing with 4 certificates               : 1149 bytes (a/a/a/a)
//   Marshaled caveat                           :   55 bytes (0xa64c2d0119fba3348071feeb2f308000(time.Time=0001-01-01 00:00:00 +0000 UTC))
//   Marshaled caveat                           :    6 bytes (0x54a676398137187ecdb26d2d69ba0003([]string=[m]))
func TestByteSize(t *testing.T) {
	blessingsize := func(b Blessings) int {
		buf, err := vom.Encode(b)
		if err != nil {
			t.Fatal(err)
		}
		return len(buf)
	}
	key, err := newSigner().PublicKey().MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var sigbytes int
	if sig, err := newSigner().Sign([]byte("purpose"), []byte("message")); err != nil {
		t.Fatal(err)
	} else {
		sigbytes = len(sig.R) + len(sig.S)
	}
	t.Logf("Marshaled P256 ECDSA key                   : %4d bytes", len(key))
	t.Logf("Major components of an ECDSA signature     : %4d bytes", sigbytes)
	// Byte sizes of blessings (with no caveats in any certificates).
	t.Logf("VOM type information overhead for blessings: %4d bytes", blessingsize(Blessings{}))
	for ncerts := 1; ncerts < 5; ncerts++ {
		b := makeBlessings(t, ncerts)
		t.Logf("Blessing with %d certificates               : %4d bytes (%v)", ncerts, blessingsize(b), b)
	}
	// Byte size of framework caveats.
	logCaveatSize := func(c Caveat, err error) {
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Marshaled caveat                           : %4d bytes (%v)", len(c.ParamVom), &c)
	}
	logCaveatSize(NewExpiryCaveat(time.Now()))
	logCaveatSize(NewMethodCaveat("m"))
}

func TestBlessingCouldHaveNames(t *testing.T) {
	falseCaveat, err := NewCaveat(ConstCaveat, false)
	if err != nil {
		t.Fatal(err)
	}

	bless := func(p Principal, key PublicKey, with Blessings, extension string) Blessings {
		b, err := p.Bless(key, with, extension, falseCaveat)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	var (
		alice = newPrincipal(t)
		bob   = newPrincipal(t)

		bbob = blessSelf(t, bob, "bob/tablet")

		balice1   = blessSelf(t, alice, "alice")
		balice2   = blessSelf(t, alice, "alice/phone/youtube", falseCaveat)
		balice3   = bless(bob, alice.PublicKey(), bbob, "friend")
		balice, _ = UnionOfBlessings(balice1, balice2, balice3)
	)

	tests := []struct {
		names  []string
		result bool
	}{
		{[]string{"alice", "alice/phone/youtube", "bob/tablet/friend"}, true},
		{[]string{"alice", "alice/phone/youtube"}, true},
		{[]string{"alice/phone/youtube", "bob/tablet/friend"}, true},
		{[]string{"alice", "bob/tablet/friend"}, true},
		{[]string{"alice"}, true},
		{[]string{"alice/phone/youtube"}, true},
		{[]string{"bob/tablet/friend"}, true},
		{[]string{"alice/tablet"}, false},
		{[]string{"alice/phone"}, false},
		{[]string{"bob/tablet"}, false},
		{[]string{"bob/tablet/friend/spouse"}, false},
		{[]string{"carol/phone"}, false},
	}
	for _, test := range tests {
		if got, want := balice.CouldHaveNames(test.names), test.result; got != want {
			t.Errorf("%v.CouldHaveNames(%v): got %v, want %v", balice, test.names, got, want)
		}
	}
}

func TestBlessingsExpiry(t *testing.T) {
	p, err := CreatePrincipal(newSigner(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	oneHour := now.Add(time.Hour)
	twoHour := now.Add(2 * time.Hour)
	oneHourCav, err := NewExpiryCaveat(oneHour)
	if err != nil {
		t.Fatal(err)
	}
	twoHourCav, err := NewExpiryCaveat(twoHour)
	if err != nil {
		t.Fatal(err)
	}
	// twoHourB should expiry in two hours.
	twoHourB, err := p.BlessSelf("self", twoHourCav)
	if err != nil {
		t.Fatal(err)
	}
	// oneHourB should expiry in one hour.
	oneHourB, err := p.BlessSelf("self", oneHourCav, twoHourCav)
	if err != nil {
		t.Fatal(err)
	}
	// noExpiryB should never expiry.
	noExpiryB, err := p.BlessSelf("self", UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	if exp := noExpiryB.Expiry(); !exp.IsZero() {
		t.Errorf("got %v, want %v", exp, time.Time{})
	}
	if got, want := oneHourB.Expiry().UTC(), oneHour.UTC(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := twoHourB.Expiry().UTC(), twoHour.UTC(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBlessingsUniqueID(t *testing.T) {
	var (
		palice = newPrincipal(t)
		pbob   = newPrincipal(t)

		// Create blessings using all the methods available to create
		// them: Bless, BlessSelf, UnionOfBlessings.
		alice        = blessSelf(t, palice, "alice")
		bob          = blessSelf(t, pbob, "bob")
		bobfriend, _ = pbob.Bless(alice.PublicKey(), bob, "friend", UnconstrainedUse())
		bobspouse, _ = pbob.Bless(alice.PublicKey(), bob, "spouse", UnconstrainedUse())

		u1, _ = UnionOfBlessings(alice, bobfriend, bobspouse)
		u2, _ = UnionOfBlessings(bobfriend, bobspouse, alice)

		all = []Blessings{alice, bob, bobfriend, bobspouse, u1}
	)
	// Each individual blessing should have a different UniqueID, and different from u1
	for i := 0; i < len(all); i++ {
		b1 := all[i]
		for j := i + 1; j < len(all); j++ {
			if b2 := all[j]; bytes.Equal(b1.UniqueID(), b2.UniqueID()) {
				t.Errorf("%q and %q have the same UniqueID!", b1, b2)
			}
		}
		// Each blessings object must have a unique ID (whether created
		// by blessing self, blessed by another principal, or
		// roundtripped through VOM)
		if len(b1.UniqueID()) == 0 {
			t.Errorf("%q has no UniqueID", b1)
		}
		serialized, err := vom.Encode(b1)
		if err != nil {
			t.Errorf("%q failed VOM encoding: %v", b1, err)
		}
		var deserialized Blessings
		if err := vom.Decode(serialized, &deserialized); err != nil || !bytes.Equal(b1.UniqueID(), deserialized.UniqueID()) {
			t.Errorf("%q: UniqueID mismatch after VOM round-tripping. VOM decode error: %v", b1, err)
		}
	}
	// u1 and u2 should have the same UniqueID
	if !bytes.Equal(u1.UniqueID(), u2.UniqueID()) {
		t.Errorf("%q and %q have different UniqueIDs", u1, u2)
	}
}

// TODO(ashankar): Remove to fully resolve https://github.com/vanadium/issues/issues/543
func runWithSignatureScheme(newscheme bool, f func(*testing.B), b *testing.B) {
	old := useNewCertificateSigningScheme
	useNewCertificateSigningScheme = newscheme
	f(b)
	useNewCertificateSigningScheme = old
}

func benchmarkBless(b *testing.B) {
	p, err := CreatePrincipal(newSigner(), nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	self, err := p.BlessSelf("self")
	if err != nil {
		b.Fatal(err)
	}
	// Include at least one caveat as having caveats should be the common case.
	caveat, err := NewExpiryCaveat(time.Now().Add(time.Hour))
	if err != nil {
		b.Fatal(err)
	}
	blessee := newSigner().PublicKey()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p.Bless(blessee, self, "friend", caveat); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBless_Deprecated(b *testing.B) {
	runWithSignatureScheme(false, benchmarkBless, b)
}

func BenchmarkBless(b *testing.B) {
	runWithSignatureScheme(true, benchmarkBless, b)
}

func benchmarkVerifyCertificateIntegrity(b *testing.B) {
	native := makeBlessings(b, 1)
	var wire WireBlessings
	if err := wireBlessingsFromNative(&wire, native); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := wireBlessingsToNative(wire, &native); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVerifyCertificateIntegrity_Deprecated(b *testing.B) {
	runWithSignatureScheme(false, benchmarkVerifyCertificateIntegrity, b)
}

func BenchmarkVerifyCertificateIntegrity(b *testing.B) {
	runWithSignatureScheme(true, benchmarkVerifyCertificateIntegrity, b)
}

func BenchmarkVerifyCertificateIntegrity_NoCaching(b *testing.B) {
	signatureCache.disable()
	defer signatureCache.enable()
	runWithSignatureScheme(true, benchmarkVerifyCertificateIntegrity, b)
}

func makeBlessings(t testing.TB, ncerts int) Blessings {
	p, err := CreatePrincipal(newSigner(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.BlessSelf("a")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < ncerts; i++ {
		p2, err := CreatePrincipal(newSigner(), nil, nil)
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		b2, err := p.Bless(p2.PublicKey(), b, "a", UnconstrainedUse())
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		p = p2
		b = b2
	}
	return b
}
