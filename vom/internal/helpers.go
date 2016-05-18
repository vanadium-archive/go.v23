// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/gob"
	"strings"
	"testing"

	"v.io/v23/security"
	"v.io/v23/vom"
)

func vomEncode(b *testing.B, value interface{}) {
	// Try encoding once to make sure it succeeds.
	if _, err := vom.Encode(value); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vom.Encode(value)
	}
}

func vomDecode(b *testing.B, tofill, value interface{}) {
	// Encode once to get the data, and decode once to make sure it succeeds.
	data, err := vom.Encode(value)
	if err != nil {
		b.Fatal(err)
	}
	if err := vom.Decode(data, tofill); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vom.Decode(data, tofill)
	}
}

func vomEncodeMany(b *testing.B, value interface{}) {
	var buf bytes.Buffer
	enc := vom.NewEncoder(&buf)
	// Encode once first to write the type and value.
	if err := enc.Encode(value); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		enc.Encode(value)
	}
}

func vomDecodeMany(b *testing.B, tofill, value interface{}) {
	var buf bytes.Buffer
	enc := vom.NewEncoder(&buf)
	// Encode once first to write the type and value.
	if err := enc.Encode(value); err != nil {
		b.Fatal(err)
	}
	// Capture the offset, and encode again to write just the value.
	valueOffset := int64(buf.Len())
	if err := enc.Encode(value); err != nil {
		b.Fatal(err)
	}
	// Decode the type and value once first.
	reader := bytes.NewReader(buf.Bytes())
	dec := vom.NewDecoder(reader)
	if err := dec.Decode(tofill); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Seek(valueOffset, 0)
		dec.Decode(tofill)
	}
}

func gobEncode(b *testing.B, value interface{}) {
	// Try encoding once to make sure it succeeds.
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(value); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		gob.NewEncoder(&buf).Encode(value)
	}
}

func gobDecode(b *testing.B, tofill, value interface{}) {
	// Encode once to get the data, and decode once to make sure it succeeds.
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(value); err != nil {
		b.Fatal(err)
	}
	reader := bytes.NewReader(buf.Bytes())
	if err := gob.NewDecoder(reader).Decode(tofill); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Seek(0, 0)
		gob.NewDecoder(reader).Decode(tofill)
	}
}

func gobEncodeMany(b *testing.B, value interface{}) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	// Encode once first to write the type.
	if err := enc.Encode(value); err != nil {
		b.Fatal(err)
	}
	enc.Encode(value)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		enc.Encode(value)
	}
}

func gobDecodeMany(b *testing.B, tofill, value interface{}) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	// Encode once first to write the type and value.
	if err := enc.Encode(value); err != nil {
		b.Fatal(err)
	}
	// Capture the offset, and encode again to write just the value.
	valueOffset := int64(buf.Len())
	if err := enc.Encode(value); err != nil {
		b.Fatal(err)
	}
	// Decode the type and value once first.
	reader := bytes.NewReader(buf.Bytes())
	dec := gob.NewDecoder(reader)
	if err := dec.Decode(tofill); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Seek(valueOffset, 0)
		dec.Decode(tofill)
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
