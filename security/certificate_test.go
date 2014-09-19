package security

import (
	"bytes"
	"crypto/elliptic"
	"reflect"
	"testing"
)

func TestCertificateContentHash(t *testing.T) {
	// This test generates a bunch of Certificates and Signatures using the reflect package
	// to ensure that ever single field of these two is excercised.
	//
	// Then with this "comprehensive" set of certificates and signatures, it ensures that:
	// (1) No two certificates with different fields have the same content hash
	// (2) No two certificates when hashed with distinct parent signatures have the same content hash
	// (3) Except, the "Signature" field in the certificates should not be included in the content hash.
	var (
		// Array of Certificate and Signature where the i-th element differs from the (i-1)th in exactly one field.
		certificates = make([]Certificate, 1)
		signatures   = make([]Signature, 1)
		numtested    = 0

		v = func(item interface{}) reflect.Value { return reflect.ValueOf(item) }
		// type of field in Certificate/Signature to a set of values to test against.
		type2values = map[reflect.Type][]reflect.Value{
			reflect.TypeOf(""):         []reflect.Value{v("a"), v("b")},
			reflect.TypeOf(Hash("")):   []reflect.Value{v(SHA256Hash), v(SHA384Hash)},
			reflect.TypeOf([]byte{}):   []reflect.Value{v([]byte{1}), v([]byte{2})},
			reflect.TypeOf([]Caveat{}): []reflect.Value{v([]Caveat{newCaveat(MethodCaveat("Method"))}), v([]Caveat{newCaveat(PeerBlessingsCaveat("peer"))})},
		}
		hashfn = SHA256Hash // hash function used to compute the content hash in tests.
	)
	defer func() {
		// Paranoia: Most of the tests are gated by loops on the size of "certificates" and "signatures",
		// so a small bug might cause many loops to be skipped. This sanity check tries to detect such test
		// bugs by counting the expected number of content hashes that were generated and tested.
		// - len(certificates) = 3 fields * 2 values + empty cert = 7
		//   Thus, number of certificate pairs = 7C2 = 21
		// - len(signatures) = 4 fields * 2 values each + empty = 9
		//   Thus, number of signature pairs = 9C2 = 36
		//
		// Tests:
		// - content hashes should be different for each Certificate:      21 hash comparisons
		// - content hashes should be different for each parent Singature: 36 hash comparisons
		// - content hashes should not depend on Certificate.Signature:     8 hash comparisons (9 signatures)
		if got, want := numtested, 21+36+8; got != want {
			t.Fatalf("Executed %d tests, expected %d", got, want)
		}
	}()

	// Generate a bunch of certificates (adding them to certs), each with one field
	// different from the previous one. No two certificates should have the same
	// content hash (since they differ in content). Exclude the Signature field since
	// that does not affect the content hash.
	for typ, idx := reflect.TypeOf(Certificate{}), 0; idx < typ.NumField(); idx++ {
		field := typ.Field(idx)
		if field.Name == "Signature" {
			continue
		}
		values := type2values[field.Type]
		if len(values) == 0 {
			t.Fatalf("No sample values for field %q of type %v", field.Name, field.Type)
		}
		cert := certificates[len(certificates)-1] // copy of the last certificate
		for _, v := range values {
			reflect.ValueOf(&cert).Elem().Field(idx).Set(v)
			certificates = append(certificates, cert)
		}
	}
	// Similarly, generate a bunch of signatures.
	for typ, idx := reflect.TypeOf(Signature{}), 0; idx < typ.NumField(); idx++ {
		field := typ.Field(idx)
		values := type2values[field.Type]
		if len(values) == 0 {
			t.Fatalf("No sample values for field %q of type %v", field.Name, field.Type)
		}
		sig := signatures[len(signatures)-1]
		for _, v := range values {
			reflect.ValueOf(&sig).Elem().Field(idx).Set(v)
			signatures = append(signatures, sig)
		}
	}

	// Alright, now we have generated a bunch of test data: Certificates with all fields, Signatures with all fields.
	// TEST: No two certificates should have the same contenthash, even when the parent signature is the same.
	hashes := make([][]byte, len(certificates))
	for i, cert := range certificates {
		hashes[i] = cert.contentHash(hashfn, Signature{})
	}
	for i := 0; i < len(hashes); i++ {
		for j := i + 1; j < len(hashes); j++ {
			numtested++
			if bytes.Equal(hashes[i], hashes[j]) {
				t.Errorf("Certificates:{%+v} and {%+v} have the same content hash", certificates[i], certificates[j])
			}
		}
	}

	// TEST: The content hash should change with parent signatures
	hashes = make([][]byte, len(signatures))
	for i, sig := range signatures {
		var cert Certificate
		hashes[i] = cert.contentHash(hashfn, sig)
	}
	for i := 0; i < len(hashes); i++ {
		for j := i + 1; j < len(hashes); j++ {
			numtested++
			if bytes.Equal(hashes[i], hashes[j]) {
				t.Errorf("Certificate content hash is the same for two different parent signatures {%v} and {%v}", signatures[i], signatures[j])
			}
		}
	}

	// TEST: The Signature field within a certificate itself should not affect the hash.
	hashes = make([][]byte, len(signatures))
	for i, sig := range signatures {
		cert := Certificate{Signature: sig}
		hashes[i] = cert.contentHash(hashfn, Signature{})
	}
	for i := 1; i < len(hashes); i++ {
		numtested++
		if !bytes.Equal(hashes[i], hashes[i-1]) {
			cert1 := Certificate{Signature: signatures[i]}
			cert2 := Certificate{Signature: signatures[i-1]}
			t.Errorf("Certificate{%v} and {%v} which only differ in their Signature field seem to have different content hashes", cert1, cert2)
		}
	}
}

func TestCertificateSignUsesContentHashWithStrengthComparableToSigningKey(t *testing.T) {
	tests := []struct {
		curve  elliptic.Curve
		hash   Hash
		nBytes int
	}{
		{elliptic.P224(), SHA256Hash, 32},
		{elliptic.P256(), SHA256Hash, 32},
		{elliptic.P384(), SHA384Hash, 48},
		{elliptic.P521(), SHA512Hash, 64},
	}
	for idx, test := range tests {
		var cert Certificate
		wanthash := cert.contentHash(test.hash, Signature{})
		if got, want := len(wanthash), test.nBytes; got != want {
			t.Errorf("Got content hash of %d bytes, want %d for hash function %q", got, want, test.hash)
			continue
		}
		signer := newECDSASigner(t, test.curve)
		if err := cert.sign(signer, Signature{}); err != nil {
			t.Errorf("cert.sign for test #%d (hash:%q) failed: %v", idx, test.hash, err)
			continue
		}
		if !cert.Signature.Verify(signer.PublicKey(), wanthash) {
			t.Errorf("Incorrect hash function used by sign. Test #%d, expected hash:%q", idx, test.hash)
			continue
		}
	}
}
