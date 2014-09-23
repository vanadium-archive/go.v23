// Package security provides the API for identity, authentication and authorization
// in veyron.
package security

import (
	"time"
	"veyron.io/veyron/veyron2/naming"
)

// PublicIDStore is an interface for managing PublicIDs. All PublicIDs added
// to the store are required to be blessing the same public key and must be tagged
// with a BlessingPattern. By default, in IPC, a client uses a PublicID from the
// store to authenticate to servers identified by the pattern tagged on
// the PublicID.
type PublicIDStore interface {
	// Adds a PublicID to the store and tags it with the provided peerPattern.
	// The method fails if the provided PublicID has a different public key from
	// the (common) public key of existing PublicIDs in the store. PublicIDs with
	// multiple Names are broken up into PublicIDs with at most one Name and then
	// added separately to the store.
	Add(id PublicID, peerPattern BlessingPattern) error

	// ForPeer returns a PublicID by combining all PublicIDs from the store that are
	// tagged with patterns matching the provided peer. The combined PublicID has the
	// same public key as the individual PublicIDs and carries the union of the
	// set of names of the individual PublicIDs. An error is returned if there are no
	// matching PublicIDs.
	ForPeer(peer PublicID) (PublicID, error)

	// DefaultPublicID returns a PublicID from the store based on the default
	// BlessingPattern. The returned PublicID has the same public key as the common
	// public key of all PublicIDs in the store, and carries the union of the set of
	// names of all PublicIDs that match the default pattern. An error is returned if
	// there are no matching PublicIDs. (Note that it is the PublicIDs that are matched
	// with the default pattern rather than the peer pattern tags on them.)
	DefaultPublicID() (PublicID, error)

	// SetDefaultBlessingPattern changes the default BlessingPattern used by subsequent
	// calls to DefaultPublicID to the provided pattern. In the absence of any
	// SetDefaultBlessingPattern calls, the default BlessingPattern must be set to
	// "..." which matches all PublicIDs.
	SetDefaultBlessingPattern(pattern BlessingPattern) error
}

// PublicID is the interface for a non-secret component of a principal's
// unique identity.
type PublicID interface {
	// Names returns a list of human-readable names associated with the principal.
	// The returned names act only as a hint, there is no guarantee that they are
	// globally unique.
	Names() []string

	// PublicKey returns the public key corresponding to the private key
	// that is held only by the principal represented by this PublicID.
	PublicKey() PublicKey

	// Authorize determines whether the PublicID has credentials that are valid
	// under the provided context. If so, Authorize returns a new PublicID that
	// carries only the valid credentials. The returned PublicID is always
	// non-nil in the absence of errors and has the same public key as this
	// PublicID.
	Authorize(context Context) (PublicID, error)

	// ThirdPartyCaveats returns the set of third-party restrictions for this PublicID.
	ThirdPartyCaveats() []ThirdPartyCaveat
}

// Principal represents an entity capable of making or receiving RPCs.
// Principals have a unique (public, private) key pair, have blessings bound
// to them and can bless other principals.
//
// TODO(ashankar, ataly): REMOVE THIS TODO :)
// IF YOU ARE READING THIS FILE AND YOU SEE THE TYPES PrivateID and
// PublicID, THEN IGNORE THIS Blessings TYPE. Blessings is part of a
// newer security API (see https://veyron-review.googlesource.com/#/c/4102)
// AND WILL EVENTUALLY REPLACE PublicID.
type Principal interface {
	// Bless binds extensions of blessings held by this principal to
	// another principal (represented by its public key).
	//
	// For example, a principal with the blessings "google/alice"
	// and "veyron/alice" can bind the blessings "google/alice/friend"
	// and "veyron/alice/friend" to another principal using:
	//   Bless(<other principal>, <google/alice, veyron/alice>, "friend", ...)
	//
	// To discourage   delegation of authority, the interface
	// requires at least one caveat to be provided. If unconstrained delegation
	// is desired, the UnconstrainedUse function can be used to produce
	// this argument.
	//
	// with.PublicKey must be the same as the principal's public key.
	Bless(key PublicKey, with Blessings, extension string, caveat Caveat, additionalCaveats ...Caveat) (Blessings, error)

	// BlessSelf creates a blessing with the provided name for this principal.
	BlessSelf(name string, caveats ...Caveat) (Blessings, error)

	// Sign uses the private key of the principal to sign message.
	Sign(message []byte) (Signature, error)

	// MintDischarge generates a discharge for 'tp' after validating any
	// restrictions specified in it under context.
	//
	// The returned discharge will be usable only if the provided caveats
	// are met when using the discharge.
	MintDischarge(tp ThirdPartyCaveat, context Context, caveat Caveat, additionalCaveats ...Caveat) (Discharge, error)

	// PublicKey returns the public key counterpart of the private key held
	// by the Principal.
	PublicKey() PublicKey
}

// BlessingStore is the interface for storing blessings bound to a
// principal and managing the subset of blessings to be presented to
// particular peers.
type BlessingStore interface {
	// Add adds a set of blessings to the store.
	//
	// These blessings are intended to be shared with peers whose
	// blessings match the provided pattern.
	//
	// It is an error to call Add with "blessings" whose public key does
	// not match the PublicKey of the principal for which this store hosts
	// blessings.
	Add(blessings PublicID, forPeers BlessingPattern) error

	// ForPeer returns the set of blessings that have been previously
	// Add-ed to the store with an intent of being shared with peers
	// that have at least one of the provided blessings.
	//
	// If no peerBlessings are provided then blessings marked for all peers
	// (i.e., Add-ed with the AllPrincipals pattern) is returned.
	//
	// Returns nil if there are no matching blessings in the store.
	ForPeer(peerBlessings ...string) PublicID

	// SetDefault sets up the Blessings made available on a subsequent call
	// to Default.
	//
	// It is an error to call SetDefault with Blesssings whose public key
	// does not match the PublicKey of the principal for which this store
	// hosts blessings.
	SetDefault(blessings PublicID) error

	// Default returns the blessings to be shared with peers for which
	// no other information is available in order to select blessings
	// from the store.
	//
	// For example, Default can be used by servers to identify themselves
	// to clients before the client has identified itself.
	//
	// Default returns the blessings provided to the last call to
	// SetDefault, or if no such call was made it is equivalent to ForPeer
	// with no arguments.
	//
	// Returns nil if there is no usable blessing.
	Default() PublicID

	// PublicKey returns the public key of the Principal for which
	// this store hosts blessings.
	PublicKey() PublicKey

	// TODO(ataly,ashankar): Might add methods so that discharges
	// can be persisted. e.g,
	// // AddDischarge adds a Discharge to the store iff there is a
	// // blessing that requires it.
	// AddDischarge(d Discharge) error
	// // Discharges returns all the Discharges that have been
	// // added to the store and are required by "blessings".
	// Discharges(blessings PublicID) []Discharge
}

// BlessingRoots hosts the set of authoritative public keys for roots
// of blessings.
type BlessingRoots interface {
	// Add marks 'root' as an authoritative key for blessings that
	// match 'pattern'.
	//
	// Multiple keys can be added for the same pattern, in which
	// case all those keys are considered authoritative for
	// blessings that match the pattern.
	Add(root PublicKey, pattern BlessingPattern) error

	// Recognized returns nil iff the provided key is recognized
	// as an authority on a pattern that is matched by blessing.
	Recognized(key PublicKey, blessing string) error
}

// Blessings encapsulates all the cryptographic operations required to
// prove that a set of blessings (human-readable strings) have been bound
// to a principal in a specific context.
//
// Blessings objects are meant to be presented to other principals to authenticate
// and authorize actions.
//
// TODO(ashankar, ataly): REMOVE THIS TODO :)
// IF YOU ARE READING THIS FILE AND YOU SEE THE TYPES PrivateID and
// PublicID, THEN IGNORE THIS Blessings TYPE. Blessings is part of a
// newer security API (see https://veyron-review.googlesource.com/#/c/4102)
// AND WILL EVENTUALLY REPLACE PublicID.
type Blessings interface {
	// ForContext returns a validated set of (human-readable string) blessings
	// presented by the principal. These returned blessings (strings) are guaranteed to:
	//
	// (1) Satisfy all the caveats given context
	// (2) Rooted in context.LocalPrincipal.Roots.
	//
	// Caveats are considered satisfied in the given context if the CaveatValidator
	// implementation can be found in the address space of the caller and
	// Validate returns nil.
	ForContext(context Context) []string

	// PublicKey returns the public key of the principal to which
	// blessings obtained from this object are bound.
	PublicKey() PublicKey

	// ThirdPartyCaveats returns the set of third-party restrictions on the
	// scope of the blessings.
	ThirdPartyCaveats() []ThirdPartyCaveat

	// unexported methods that prevent implementations of this interface
	// outside this package.
	certificateChains() [][]Certificate
	publicKeyDER() []byte
}

// Signer is the interface for signing arbitrary length messages using private keys.
type Signer interface {
	// Sign signs an arbitrary length message using the private key associated
	// with this Signer.
	//
	// The provided purpose is used to avoid "type attacks", wherein an honest
	// entity is cheated into interpreting a field in a message as one with a
	// type other than the intended one.
	Sign(purpose, message []byte) (Signature, error)

	// PublicKey returns the public key corresponding to the Signer's private key.
	PublicKey() PublicKey
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
	Bless(blessee PublicID, blessingName string, duration time.Duration, caveats []Caveat) (PublicID, error)

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
	MintDischarge(caveat ThirdPartyCaveat, context Context, duration time.Duration, caveats []Caveat) (Discharge, error)

	// Sign signs an arbitrary length message (often the hash of a larger message)
	// using the private key associated with this PrivateID.
	Sign(message []byte) (Signature, error)

	// PublicKey returns the public key corresponding to the principal.
	PublicKey() PublicKey
}

// CaveatValidator is the interface for validating the restrictions specified
// in a caveat.
type CaveatValidator interface {
	// Validate returns nil iff the restriction encapsulated in the
	// corresponding caveat has been satisfied by the provided context.
	Validate(context Context) error
}

// ThirdPartyCaveat is a restriction on the applicability of a blessing that is
// considered satisfied only when accompanied with a specific "discharge" from
// the third-party specified in the caveat.
type ThirdPartyCaveat interface {
	// ThidPartyCaveat implements CaveatValidator, where Validate
	// succeeds iff a discharge for the caveat is available in the Context.
	CaveatValidator

	// ID returns a cryptographically unique identifier for the third-party
	// caveat.
	ID() string

	// Location returns the Veyron object name of the discharging third-party.
	Location() string

	// Requirements lists the information that the third-party requires
	// in order to issue a discharge.
	Requirements() ThirdPartyRequirements

	// TODO(andreser, ashankar): require the discharger to have a specific
	// identity so that the private information below is not exposed to
	// anybody who can accept an ipc call.
}

// Discharge represents a "proof" required for satisfying a ThirdPartyCaveat.
//
// A discharge may have caveats of its own (including ThirdPartyCaveats) that
// restrict the context in which the discharge is usable.
type Discharge interface {
	// ID returns the identifier for the ThirdPartyCaveat this discharge is
	// associated with.
	ID() string

	// ThirdPartyCaveats returns the set of third-party restrictions on the
	// scope of the discharge.
	ThirdPartyCaveats() []ThirdPartyCaveat
}

// Context defines the state available for authorizing a principal.
type Context interface {
	// Method returns the method being invoked.
	Method() string
	// Name returns the Veyron object name on which the method is being invoked.
	Name() string
	// Suffix returns the Veyron object name suffix for the request.
	Suffix() string
	// Label returns the method's security label.
	Label() Label
	// CaveatDischarges returns a map of Discharges indexed by their IDs.
	Discharges() map[string]Discharge
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
