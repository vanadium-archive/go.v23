// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !cgo !linux,!openssl !amd64,!openssl

// See comments in ecdsa_openssl.go for an explanation of the choice of
// build tags.

package security

import "crypto/ecdsa"

func newInMemoryECDSASignerImpl(key *ecdsa.PrivateKey) (Signer, error) {
	return newGoStdlibSigner(key)
}

func newECDSAPublicKeyImpl(key *ecdsa.PublicKey) PublicKey {
	return newGoStdlibPublicKey(key)
}
