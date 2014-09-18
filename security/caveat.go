package security

import (
	"bytes"
	"fmt"
	"time"

	"veyron.io/veyron/veyron2/vom"
)

// NewCaveat returns a Caveat that requires validation by validator.
func NewCaveat(validator CaveatValidator) (Caveat, error) {
	var buf bytes.Buffer
	if err := vom.NewEncoder(&buf).Encode(validator); err != nil {
		return Caveat{}, err
	}
	return Caveat{buf.Bytes()}, nil
}

// ExpiryCaveat returns a Caveat that validates iff the current time is before t.
func ExpiryCaveat(t time.Time) (Caveat, error) {
	return NewCaveat(unixTimeExpiryCaveat(t.Unix()))
}

// MethodCaveat returns a Caveat that validates iff the method being invoked by
// the peer is listed in an argument to this function.
func MethodCaveat(method string, additionalMethods ...string) (Caveat, error) {
	return NewCaveat(methodCaveat(append(additionalMethods, method)))
}

// PeerBlessingsCaveat returns a Caveat that validates iff the peer has a blessing
// that matches one of the patterns provided as an argument to this function.
//
// For example, creating a blessing "alice/friend" with a PeerBlessingsCaveat("bob")
// will allow the blessing "alice/friend" to be used only when communicating with
// a principal that has the blessing "bob".
func PeerBlessingsCaveat(pattern BlessingPattern, additionalPatterns ...BlessingPattern) (Caveat, error) {
	return NewCaveat(peerBlessingsCaveat(append(additionalPatterns, pattern)))
}

func (c unixTimeExpiryCaveat) Validate(ctx Context) error {
	now := time.Now()
	expiry := time.Unix(int64(c), 0)
	if now.After(expiry) {
		return fmt.Errorf("%T(%v=%v) fails validation at %v", c, c, expiry, now)
	}
	return nil
}

func (c methodCaveat) Validate(ctx Context) error {
	methods := []string(c)
	if ctx.Method() == "" && len(methods) == 0 {
		return nil
	}
	for _, m := range methods {
		if ctx.Method() == m {
			return nil
		}
	}
	return fmt.Errorf("%T=%v fails validation for method %q", c, c, ctx.Method())
}

func (c peerBlessingsCaveat) Validate(ctx Context) error {
	patterns := []BlessingPattern(c)
	if ctx.LocalID() == nil {
		return fmt.Errorf("%T=%v fails validation since ctx.LocalID is nil", c, c)
	}
	peerblessings := ctx.LocalID().Names()
	for _, p := range patterns {
		if p.MatchedBy(peerblessings...) {
			return nil
		}
	}
	return fmt.Errorf("%T=%v fails validation for peer with blessings %v", c, c, peerblessings)
}

// TODO(ashankar): This is kept around only for backward compatibility with the
// "old" security API. Remove this when switching to the new API (i.e., when there
// are no concerns about persisted blessings with this caveat).
type Expiry struct {
	// TODO(ataly,ashankar): Get rid of IssueTime from this caveat.
	IssueTime  time.Time
	ExpiryTime time.Time
}

func (v *Expiry) Validate(context Context) error {
	now := time.Now()
	if now.Before(v.IssueTime) || now.After(v.ExpiryTime) {
		return fmt.Errorf("%#v forbids credential from being used at this time(%v)", v, now)
	}
	return nil
}

// UnconstrainedDelegation returns a Caveat implementation that never fails to
// validate. This is useful only for providing unconstrained blessings to
// another principal.
func UnconstrainedDelegation() Caveat { return Caveat{} }
