// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux,amd64,cgo openssl,cgo

// The purpose of this file is to improve performance, as demonstrated by
// benchmarks when linked against openssl-1.0.1f (with further improvements in
// openssl-1.0.2, at the time this comment was written).
//   go test -bench ECDSA v.io/v23/security
//
// See https://go-review.googlesource.com/#/c/8968/ for why the Go standard
// library (as of Go 1.5) does not have performance on par with OpenSSL.
//
// By default (without an explicit build tag), this is disabled for darwin
// since OpenSSL has been marked deprecated on OS X since version 10.7.  The
// last openssl release on OS X was version 0.9.8 and compiling this file will
// show these deprecation warnings.  Those sensitive to performance are
// encouraged to compile a later version of OpenSSL and set the build tag.
//
// Currently, this file is disabled for linux/arm as a temporary hack.  In
// practice, linux/arm binaries are often cross-compiled on a linux/amd64 host.
// The author of this comment hadn't changed the vanadium devtools to download
// and build a cross-compiled OpenSSL library at the time this comment was
// written.
//
// TODO(ashankar): Figure out how to remove the "amd64" tag hack.

package security

// #cgo pkg-config: libcrypto
// #include <openssl/bn.h>
// #include <openssl/ec.h>
// #include <openssl/ecdsa.h>
// #include <openssl/err.h>
// #include <openssl/opensslv.h>
// #include <openssl/x509.h>
//
// void openssl_init_locks();
// EC_KEY* openssl_d2i_EC_PUBKEY(const unsigned char** data, long len, unsigned long* e);
// EC_KEY* openssl_d2i_ECPrivateKey(const unsigned char** data, long len, unsigned long* e);
// ECDSA_SIG* openssl_ECDSA_do_sign(const unsigned char* data, int len, EC_KEY* key, unsigned long *e);
import "C"

import (
	"crypto/ecdsa"
	"crypto/x509"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"unsafe"

	"v.io/v23/verror"
)

var (
	errOpenSSL   = verror.Register(pkgPath+".errOpenSSL", verror.NoRetry, "{1:}{2:} OpenSSL error ({3}): {4} in {5}:{6}")
	opensslLocks []sync.RWMutex
)

func init() {
	C.ERR_load_crypto_strings()
	opensslLocks = make([]sync.RWMutex, C.CRYPTO_num_locks())
	C.openssl_init_locks()
}

//export openssl_lock
func openssl_lock(mode, n C.int) {
	l := &opensslLocks[int(n)]
	if (mode & C.CRYPTO_LOCK) == 0 {
		if (mode & C.CRYPTO_READ) == 0 {
			l.Unlock()
		} else {
			l.RUnlock()
		}
	} else {
		if (mode & C.CRYPTO_READ) == 0 {
			l.Lock()
		} else {
			l.RLock()
		}
	}
}

type opensslECPublicKey struct {
	k   *C.EC_KEY
	h   Hash
	goK ecdsa.PublicKey
}

func (k *opensslECPublicKey) MarshalBinary() ([]byte, error) { return x509.MarshalPKIXPublicKey(&k.goK) }
func (k *opensslECPublicKey) String() string                 { return publicKeyString(k) }
func (k *opensslECPublicKey) hash() Hash                     { return k.h }
func (k *opensslECPublicKey) verify(digest []byte, signature *Signature) bool {
	sig := C.ECDSA_SIG_new()
	sig.r = C.BN_bin2bn(uchar(signature.R), C.int(len(signature.R)), sig.r)
	sig.s = C.BN_bin2bn(uchar(signature.S), C.int(len(signature.S)), sig.s)
	status := C.ECDSA_do_verify(uchar(digest), C.int(len(digest)), sig, k.k)
	C.ECDSA_SIG_free(sig)
	return status == 1
}

func newOpenSSLPublicKey(golang *ecdsa.PublicKey) (PublicKey, error) {
	der, err := x509.MarshalPKIXPublicKey(golang)
	if err != nil {
		return nil, err
	}
	var errno C.ulong
	in := uchar(der)
	k := C.openssl_d2i_EC_PUBKEY(&in, C.long(len(der)), &errno)
	if k == nil {
		return nil, opensslMakeError(errno)
	}
	ret := &opensslECPublicKey{
		k:   k,
		h:   (&ecdsaPublicKey{golang}).hash(),
		goK: *golang,
	}
	runtime.SetFinalizer(ret, func(k *opensslECPublicKey) { C.EC_KEY_free(k.k) })
	return ret, nil
}

type opensslSigner struct {
	k *C.EC_KEY
}

func (k *opensslSigner) sign(data []byte) (r, s *big.Int, err error) {
	var errno C.ulong
	sig := C.openssl_ECDSA_do_sign(uchar(data), C.int(len(data)), k.k, &errno)
	if sig == nil {
		return nil, nil, opensslMakeError(errno)
	}
	var (
		rlen = (int(C.BN_num_bits(sig.r)) + 7) / 8
		slen = (int(C.BN_num_bits(sig.s)) + 7) / 8
		buf  []byte
	)
	if rlen > slen {
		buf = make([]byte, rlen)
	} else {
		buf = make([]byte, slen)
	}
	r = big.NewInt(0).SetBytes(buf[0:int(C.BN_bn2bin(sig.r, uchar(buf)))])
	s = big.NewInt(0).SetBytes(buf[0:int(C.BN_bn2bin(sig.s, uchar(buf)))])
	C.ECDSA_SIG_free(sig)
	return r, s, nil
}

func newOpenSSLSigner(golang *ecdsa.PrivateKey) (Signer, error) {
	der, err := x509.MarshalECPrivateKey(golang)
	if err != nil {
		return nil, err
	}
	pubkey, err := newOpenSSLPublicKey(&golang.PublicKey)
	if err != nil {
		return nil, err
	}
	var errno C.ulong
	in := uchar(der)
	k := C.openssl_d2i_ECPrivateKey(&in, C.long(len(der)), &errno)
	if k == nil {
		return nil, opensslMakeError(errno)
	}
	impl := &opensslSigner{k}
	runtime.SetFinalizer(impl, func(k *opensslSigner) { C.EC_KEY_free(k.k) })
	return &ecdsaSigner{
		sign: func(data []byte) (r, s *big.Int, err error) {
			return impl.sign(data)
		},
		pubkey: pubkey,
		impl:   impl,
	}, nil
}

func opensslMakeError(errno C.ulong) error {
	return verror.New(errOpenSSL, nil, errno, C.GoString(C.ERR_func_error_string(errno)), C.GoString(C.ERR_lib_error_string(errno)), C.GoString(C.ERR_reason_error_string(errno)))
}

func uchar(b []byte) *C.uchar {
	return (*C.uchar)(unsafe.Pointer(&b[0]))
}

func newInMemoryECDSASignerImpl(key *ecdsa.PrivateKey) (Signer, error) {
	return newOpenSSLSigner(key)
}

func newECDSAPublicKeyImpl(key *ecdsa.PublicKey) PublicKey {
	if key, err := newOpenSSLPublicKey(key); err == nil {
		return key
	}
	return newGoStdlibPublicKey(key)
}

func openssl_version() string {
	return fmt.Sprintf("%v (CFLAGS:%v)", C.GoString(C.SSLeay_version(C.SSLEAY_VERSION)), C.GoString(C.SSLeay_version(C.SSLEAY_CFLAGS)))
}
