// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
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

func BenchmarkBlessingsSingleCertificateEquality(b *testing.B) {
	p, err := CreatePrincipal(newSigner(), nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	caveat, err := NewExpiryCaveat(time.Now().Add(time.Hour))
	if err != nil {
		b.Fatal(err)
	}
	blessings, err := p.BlessSelf("self", caveat)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if equals := blessings.Equivalent(blessings); !equals {
			b.Fatalf("blessings should be equal")
		}
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
