// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"strings"
	"testing"
	"v.io/v23/security"
	"v.io/v23/vom"
)

func benchmarkSingleShotEncode(b *testing.B, value interface{}) {
	for i := 0; i < b.N; i++ {
		vom.Encode(value)
	}
}

func benchmarkRepeatedEncode(b *testing.B, value interface{}) {
	var buf bytes.Buffer
	enc := vom.NewEncoder(&buf)
	enc.Encode(value) // encode the type
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enc.Encode(value)
	}
}

func benchmarkSingleShotDecode(b *testing.B, tofill, value interface{}) {
	bytes, _ := vom.Encode(value)
	for i := 0; i < b.N; i++ {
		vom.Decode(bytes, &tofill)
	}
}

func benchmarkRepeatedDecode(b *testing.B, tofill, value interface{}) {
	var buf bytes.Buffer
	enc := vom.NewEncoder(&buf)
	for i := 0; i <= b.N; i++ {
		enc.Encode(value)
	}
	dec := vom.NewDecoder(&buf)
	dec.Decode(&tofill)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dec.Decode(&tofill)
	}
}

func createString(length int) string {
	return strings.Repeat("a", length)
}

func createByteList(length int) []byte {
	return []byte(strings.Repeat("a", length))
}

func createList(length int) []int32 {
	l := make([]int32, length)
	for i := range l {
		l[i] = int32(i)
	}
	return l
}

func createListAny(length int) []*vom.RawBytes {
	l := make([]*vom.RawBytes, length)
	for i := range l {
		l[i] = vom.RawBytesOf(i)
	}
	return l
}

func newSigner() security.Signer {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	return security.NewInMemoryECDSASigner(key)
}

// createTypicalBlessings creates a blessings with certificate structures and
// caveats that are "common": a chain of 3 certificates for the user, with a
// third-party caveat used for revocation.
func createTypicalBlessings() security.Blessings {
	var (
		p1, _  = security.CreatePrincipal(newSigner(), nil, nil)
		p2, _  = security.CreatePrincipal(newSigner(), nil, nil)
		p3, _  = security.CreatePrincipal(newSigner(), nil, nil)
		tpc, _ = security.NewPublicKeyCaveat(p2.PublicKey(), "some_location", security.ThirdPartyRequirements{}, security.UnconstrainedUse())

		b1, _ = p1.BlessSelf("root")
		b2, _ = p1.Bless(p2.PublicKey(), b1, "u", security.UnconstrainedUse())
		b3, _ = p2.Bless(p3.PublicKey(), b2, "user", tpc)
	)
	return b3
}
