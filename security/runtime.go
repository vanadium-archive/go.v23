package security

type Runtime interface {
	// NewIdentity creates a new PrivateID with the provided name.
	NewIdentity(name string) (PrivateID, error)

	// Identity returns the default identity used by the runtime.
	Identity() PrivateID
}
