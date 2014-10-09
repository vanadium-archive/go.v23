// This file contains utility functions and types for tests for the security package.

package security

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"veyron.io/veyron/veyron2/naming"
	"veyron.io/veyron/veyron2/vom"
)

type markedRoot struct {
	root    PublicKey
	pattern BlessingPattern
}

type roots struct {
	data []markedRoot
}

func (r *roots) Add(root PublicKey, pattern BlessingPattern) error {
	if !pattern.IsValid() {
		return fmt.Errorf("pattern %q is invalid", pattern)
	}
	r.data = append(r.data, markedRoot{root, pattern})
	return nil
}

func (r *roots) Recognized(root PublicKey, blessing string) error {
	for _, mr := range r.data {
		if reflect.DeepEqual(root, mr.root) && mr.pattern.MatchedBy(blessing) {
			return nil
		}
	}
	return fmt.Errorf("root %v not recognized for blessing %q", root, blessing)
}

func (*roots) DebugString() string {
	return "BlessingRoots implementation for testing purposes only"
}

type context struct {
	method, name, suffix string
	label                Label
	localID, remoteID    PublicID
	local                Principal
	discharges           map[string]Discharge
}

func (c *context) Method() string                   { return c.method }
func (c *context) Name() string                     { return c.name }
func (c *context) Suffix() string                   { return c.suffix }
func (c *context) Label() Label                     { return c.label }
func (c *context) Discharges() map[string]Discharge { return c.discharges }
func (c *context) LocalID() PublicID                { return c.localID }
func (c *context) RemoteID() PublicID               { return c.remoteID }
func (c *context) LocalPrincipal() Principal        { return c.local }
func (c *context) LocalBlessings() Blessings        { return nil }
func (c *context) RemoteBlessings() Blessings       { return nil }
func (c *context) LocalEndpoint() naming.Endpoint   { return nil }
func (c *context) RemoteEndpoint() naming.Endpoint  { return nil }

func newCaveat(c Caveat, err error) Caveat {
	if err != nil {
		panic(err)
	}
	return c
}

// Caveat that validates iff Context.Suffix matches the string.
//
// Since at the time of this writing, it was not clear that we want to make caveats on
// suffixes generally available, this type is implemented in this test file.
// If there is a general need for such a caveat, it should be defined similar to
// other caveats (like methodCaveat) in caveat.vdl and removed from this test file.
type suffixCaveat string

func (c suffixCaveat) Validate(ctx Context) error {
	if string(c) != ctx.Suffix() {
		return fmt.Errorf("suffixCaveat not met")
	}
	return nil
}

func newSuffixCaveat(suffix string) Caveat { return newCaveat(NewCaveat(suffixCaveat(suffix))) }

func newECDSASigner(t *testing.T, curve elliptic.Curve) Signer {
	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate ECDSA key: %v", err)
	}
	return NewInMemoryECDSASigner(key)
}

func newPrincipal(t *testing.T) Principal {
	p, err := CreatePrincipal(newECDSASigner(t, elliptic.P256()), nil, nil)
	if err != nil {
		t.Fatalf("CreatePrincipal failed: %v", err)
	}
	return p
}

func newPrincipalWithRoots(t *testing.T) Principal {
	p, err := CreatePrincipal(newECDSASigner(t, elliptic.P256()), nil, &roots{})
	if err != nil {
		t.Fatalf("CreatePrincipal failed: %v", err)
	}
	return p
}

func blessSelf(t *testing.T, p Principal, name string, caveats ...Caveat) Blessings {
	b, err := p.BlessSelf(name, caveats...)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func addToRoots(t *testing.T, p Principal, b Blessings) {
	if err := p.AddToRoots(b); err != nil {
		t.Fatal(err)
	}
}

func checkBlessings(b Blessings, c Context, want ...string) error {
	// Validate the integrity of the bits.
	buf := new(bytes.Buffer)
	if err := vom.NewEncoder(buf).Encode(b); err != nil {
		return err
	}
	var decoded Blessings
	if err := vom.NewDecoder(buf).Decode(&decoded); err != nil {
		return err
	}
	if !reflect.DeepEqual(decoded, b) {
		return fmt.Errorf("reflect.DeepEqual(%v, %v) failed after validBlessing", decoded, b)
	}
	// And now check them under the right context
	got := b.ForContext(c)
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("Got blessings %v, want %v", got, want)
	}
	return nil
}

func matchesError(got error, want string) error {
	if (got == nil) && len(want) == 0 {
		return nil
	}
	if got == nil {
		return fmt.Errorf("Got nil error, wanted to match %q", want)
	}
	if !strings.Contains(got.Error(), want) {
		return fmt.Errorf("Got error %q, wanted to match %q", got, want)
	}
	return nil
}

func init() {
	vom.Register(suffixCaveat(""))
}
