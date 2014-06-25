// Package security provides the API for identity, authentication and authorization
// in veyron.
package security

import (
	"crypto/ecdsa"
	"math/big"
	"time"
	"veyron2/naming"
)

// PublicIDStore is an interface for managing PublicIDs. All PublicIDs added
// to the store are required to be blessing the same public key and must be tagged
// with a PrincipalPattern. By default, in IPC, a client uses a PublicID from the
// store to authenticate to servers identified by the pattern tagged on
// the PublicID.
type PublicIDStore interface {
	// Adds a PublicID to the store and tags it with the provided peerPattern.
	// The method fails if the provided PublicID has a different public key from
	// the (common) public key of existing PublicIDs in the store. PublicIDs with
	// multiple Names are broken up into PublicIDs with at most one Name and then
	// added separately to the store.
	Add(PublicID PublicID, peerPattern PrincipalPattern) error

	// ForPeer returns a PublicID by combining all PublicIDs from the store that are
	// tagged with patterns matching the provided peer. The combined PublicID has the
	// same public key as the individual PublicIDs and carries the union of the
	// set of names of the individual PublicIDs. An error is returned if there are no
	// matching PublicIDs.
	ForPeer(peer PublicID) (PublicID, error)

	// DefaultPublicID returns a PublicID from the store based on the default
	// PrincipalPattern. The returned PublicID has the same public key as the common
	// public key of all PublicIDs in the store, and carries the union of the set of
	// names of all PublicIDs that match the default pattern. An error is returned if
	// there are no matching PublicIDs. (Note that it is the PublicIDs that are matched
	// with the default pattern rather than the peer pattern tags on them.)
	DefaultPublicID() (PublicID, error)

	// SetDefaultPrincipalPattern changes the default PrincipalPattern used by subsequent
	// calls to DefaultPublicID to the provided pattern. In the absence of any
	// SetDefaultPrincipalPattern calls, the default PrincipalPattern must be set to
	// "*" which matches all PublicIDs.
	SetDefaultPrincipalPattern(pattern PrincipalPattern)
}

// PublicID is the interface for a non-secret component of a principal's
// unique identity.
type PublicID interface {
	// Names returns a list of human-readable names associated with the principal.
	// The returned names act only as a hint, there is no guarantee that they are
	// globally unique.
	Names() []string

	// Match verifies if the principal has a name matching the provided PrincipalPattern
	// or can obtain a name matching the PrincipalPattern by manipulating its PublicID
	// using PrivateID operations (e.g., <PrivateID>.Bless(<PublicID>, ..., ...)). The
	// provided pattern may be of one of the following forms:
	// - pattern "*" matching all principals regardless of the names they have.
	// - a specific name <name> matching all principals who have a name that can be extended to <name>.
	// - pattern <name>/* matching all principals who have a name that is an extension of the name <name>.
	Match(pattern PrincipalPattern) bool

	// PublicKey returns the public key corresponding to the private key
	// that is held only by the principal represented by this PublicID.
	PublicKey() *ecdsa.PublicKey

	// Authorize determines whether the PublicID has credentials that are valid
	// under the provided context. If so, Authorize returns a new PublicID that
	// carries only the valid credentials. The returned PublicID is always
	// non-nil in the absence of errors and has the same public key as this
	// PublicID.
	Authorize(context Context) (PublicID, error)

	// ThirdPartyCaveats returns the set of third-party restrictions on the scope of the
	// identity. The returned restrictions are wrapped in ServiceCaveats according to the
	// services they are bound to.
	ThirdPartyCaveats() []ServiceCaveat
}

// PrivateID is the interface for the secret component of a principal's unique
// identity.
//
// Each principal has a unique (private, public) key pair. The private key
// is known only to the principal and is not expected to be shared.
type PrivateID interface {
	// PublicID returns the non-secret component of principal's identity
	// (which can be encoded and transmitted across the network perhaps).
	// TODO(ataly): Replace this method with one that returns the PublicIDStore.
	PublicID() PublicID

	// Sign signs an arbitrary length message (often the hash of a larger message)
	// using the private key associated with this identity.
	Sign(message []byte) (Signature, error)

	// Bless creates a constrained PublicID from the provided one.
	// The returned PublicID:
	// - Has the same PublicKey as the provided one.
	// - Has a new name which is an extension of PrivateID's name with the
	//   provided blessingName string.
	// - Is valid for the provided duration only if both the constraints on the
	//   PrivateID and the provided service caveats are met.
	//
	// Bless assumes that the blessee is in posession of the private key correponding
	// to the blessee.PublicKey. Failure to ensure this property may result in
	// impersonation attacks.
	Bless(blessee PublicID, blessingName string, duration time.Duration, caveats []ServiceCaveat) (PublicID, error)

	// Derive returns a new PrivateID that has the same secret component
	// as a existing PrivateID but with the provided public component (PublicID).
	// The provided PublicID must have the same public key as the existing
	// PublicID for this operation to succeed.
	// TODO(ataly, ashankar): Replace this method with one that derives from a PublicIDStore.
	//
	Derive(publicID PublicID) (PrivateID, error)

	// MintDischarge returns a discharge for the provided third-party caveat if
	// the caveat's restrictions on minting discharges are satisfied.
	// Otherwise, it returns nil and an error.
	// The discharge is valid for duration if caveats are met.
	//
	// TODO(ataly, ashankar): Should we get rid of the duration argument
	// and simply have a list of discharge caveats?
	MintDischarge(caveat ThirdPartyCaveat, context Context, duration time.Duration, caveats []ServiceCaveat) (ThirdPartyDischarge, error)
}

// Signature represents a digital signature created by a PrivateID.
type Signature struct {
	R, S *big.Int
}

// Caveat is the interface for restrictions on the scope of an identity. These
// restrictions are validated by the remote end when a connection is made using the
// identity.
type Caveat interface {
	// Validate tests the restrictions specified in the Caveat under the
	// provided context, returning nil if they are satisfied or an error if
	// not.
	Validate(context Context) error
}

// ServiceCaveat binds a caveat to a specific set of services.
type ServiceCaveat struct {
	// Service is a pattern identifying the services this caveat is bound to.
	Service PrincipalPattern

	// Caveat represents the underlying restriction embedded in this ServiceCaveat.
	Caveat
}

// ThirdPartyCaveatID is the string identifier for ThirdPartyCaveats and Discharges.
type ThirdPartyCaveatID string

// ThirdPartyCaveat is the interface for third-party restrictions on the scope
// of an identity. A request made using an identity carrying such a third-party
// restriction must be accompanied with a "discharge" for the restriction obtained
// from the third-party.
type ThirdPartyCaveat interface {
	Caveat

	// ID returns a cryptographically unique identity for the caveat.
	ID() ThirdPartyCaveatID

	// Location returns a global Veyron name for the discharging third-party.
	Location() string
}

// ThirdPartyDischarge is the interface for discharges for third-party restrictions on PublicIDs.
type ThirdPartyDischarge interface {
	// CaveatID returns a cryptographically unique identity for the discharge.
	// This identity must match the identity of the corresponding caveat.
	CaveatID() ThirdPartyCaveatID

	// ThirdPartyCaveats returns the set of third-party restrictions on the scope of the
	// discharge. The returned restrictions are wrapped in ServiceCaveats according to the
	// services they are bound to.
	ThirdPartyCaveats() []ServiceCaveat
}

// CaveatDischargeMap is map from third-party caveat identities to the corresponding
// discharges.
type CaveatDischargeMap map[ThirdPartyCaveatID]ThirdPartyDischarge

// Context defines the state available for authorizing a principal.
type Context interface {
	// Method returns the method being invoked.
	Method() string
	// Name returns the veyron name on which the method is being invoked.
	Name() string
	// Suffix returns the veyron name suffix for the request.
	Suffix() string
	// Label returns the method's security label.
	Label() Label
	// CaveatDischarges returns a map of Discharges indexed by their CaveatIDs.
	CaveatDischarges() CaveatDischargeMap
	// LocalID returns the PublicID of the principal at the local end of the request.
	LocalID() PublicID
	// RemoteID returns the PublicID of the principal at the remote end of the request.
	RemoteID() PublicID
	// LocalEndpoint() returns the Endpoint of the principal at the local end of the request
	LocalEndpoint() naming.Endpoint
	// RemoteAddr() returns the Endpoint of the principal at the remote end of the request
	RemoteEndpoint() naming.Endpoint
}

// Authorizer is the interface for performing authorization checks.
type Authorizer interface {
	Authorize(context Context) error
}
