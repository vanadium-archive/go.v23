package version

// IPCVersion represents a version of the IPC protocol.
type IPCVersion uint32

const (
	// UnknownIPCVersion is used for Min/MaxIPCVersion in an Endpoint when
	// we don't know the relevant version numbers.  In this case the IPC
	// implementation will have to guess the correct values.
	UnknownIPCVersion IPCVersion = iota

	// IPCVersion2 uses VOM for encoding signatures.
	IPCVersion2

	// IPCVersion3 uses channel-binding for authentication.
	// Versions prior to this have broken authentication.
	IPCVersion3

	// dummy version so that the numeric value of IPCVersion4 is 4
	dummyVersion3

	// IPCVersion4 uses the new security model (Principal and Blessings objects).
	// TODO(ashankar,ataly): Remove this comment and all versions prior to version
	// 3 when the transition to the new model is complete.
	IPCVersion4
)

// IPCVersionRange allows you to optionally specify a range of versions to
// use when calling FormatEndpoint
type IPCVersionRange struct {
	Min, Max IPCVersion
}

// EndpointOpt implents the EndpointOpt interface so an IPCVersionRange
// can be used as an option to FormatEndpoint.
func (IPCVersionRange) EndpointOpt() {}
