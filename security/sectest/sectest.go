// Package sectest provides utility functions for security testing.
package sectest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"

	"veyron.io/veyron/veyron2/security"
)

// NewKey generates an ECDSA (public, private) key pair..
func NewKey() (security.PublicKey, *ecdsa.PrivateKey, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return security.NewECDSAPublicKey(&priv.PublicKey), priv, nil
}

type roots map[string][]security.BlessingPattern

func (r roots) Add(key security.PublicKey, pattern security.BlessingPattern) error {
	if key == nil {
		return fmt.Errorf("nil key")
	}
	mapkey := fmt.Sprintf("%v", key)
	mapval := append(r[mapkey], pattern)
	r[mapkey] = mapval
	return nil
}

func (r roots) Recognized(key security.PublicKey, blessing string) error {
	mapval := r[fmt.Sprintf("%v", key)]
	for _, p := range mapval {
		if p.MatchedBy(blessing) {
			return nil
		}
	}
	return fmt.Errorf("public key %v is not recognized as an authority on the blessing %q", key, blessing)
}

// NewBlessingRoots returns a security.BlessingRoots implementation that
// keeps state only in memory.
func NewBlessingRoots() security.BlessingRoots {
	return roots(make(map[string][]security.BlessingPattern))
}

// NewPrincipal returns a security.Principal that uses a trivial BlessingRoots and BlessingStore implementation.
func NewPrincipal() (security.Principal, error) {
	_, key, err := NewKey()
	if err != nil {
		return nil, err
	}
	return security.CreatePrincipal(security.NewInMemoryECDSASigner(key), nil, NewBlessingRoots())
}
