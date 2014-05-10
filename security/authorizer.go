package security

// CaveatDischargeMap is map from third-party caveat identities to the corresponding
// discharges.
type CaveatDischargeMap map[ThirdPartyCaveatID]ThirdPartyDischarge

// Context defines the state available for verifying whether a request
// is authorized.
type Context interface {
	// Method returns the method name for the request.
	Method() string
	// Name returns the undispatched name for the request.  It consists of
	// the dispatch prefix + the suffix (the latter is returned by the
	// Suffix method).
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
}

// Authorizer defines the interface that a server must implement in order to
// perform an authorization check.
type Authorizer interface {
	// Authorize checks if the provided request context is authorized.
	Authorize(context Context) error
}
