package security

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
	"time"

	"v.io/veyron/veyron2/vom"
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
// As of October 26, 2014, the numbers were:
//   Marshaled P256 ECDSA key                   :   91 bytes
//   VOM type information overhead for blessings:  409 bytes
//   Blessing with 1 certificates               :  591 bytes (a)
//   Blessing with 2 certificates               :  770 bytes (a/a)
//   Blessing with 3 certificates               :  948 bytes (a/a/a)
//   Blessing with 4 certificates               : 1126 bytes (a/a/a/a)
//   Marshaled caveat                           :   71 bytes (security.unixTimeExpiryCaveat(1414312924 = 2014-10-26 01:42:04 -0700 PDT))
//   Marshaled caveat                           :   61 bytes (security.methodCaveat([m]))
func TestByteSize(t *testing.T) {
	blessingsize := func(b Blessings) int {
		var buf bytes.Buffer
		if err := vom.NewEncoder(&buf).Encode(MarshalBlessings(b)); err != nil {
			t.Fatal(err)
		}
		return buf.Len()
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
	t.Logf("VOM type information overhead for blessings: %4d bytes", blessingsize(nil))
	for ncerts := 1; ncerts < 5; ncerts++ {
		b := makeBlessings(t, ncerts)
		t.Logf("Blessing with %d certificates               : %4d bytes (%v)", ncerts, blessingsize(b), b)
	}
	// Byte size of framework caveats.
	logCaveatSize := func(c Caveat, err error) {
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Marshaled caveat                           : %4d bytes (%v)", len(c.ValidatorVOM), &c)
	}
	logCaveatSize(ExpiryCaveat(time.Now()))
	logCaveatSize(MethodCaveat("m"))
}

func BenchmarkBless(b *testing.B) {
	p, err := CreatePrincipal(newSigner(), nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	self, err := p.BlessSelf("self")
	if err != nil {
		b.Fatal(err)
	}
	// Include at least one caveat as having caveats should be the common case.
	caveat, err := ExpiryCaveat(time.Now().Add(time.Hour))
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

func BenchmarkVerifyCertificateIntegrity(b *testing.B) {
	wire := MarshalBlessings(makeBlessings(b, 1))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := NewBlessings(wire); err != nil {
			b.Fatal(err)
		}
	}
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
