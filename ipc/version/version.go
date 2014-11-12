package version

// IPCVersion represents a version of the IPC protocol.
type IPCVersion uint32

const (
	// UnknownIPCVersion is used for Min/MaxIPCVersion in an Endpoint when
	// we don't know the relevant version numbers.  In this case the IPC
	// implementation will have to guess the correct values.
	UnknownIPCVersion IPCVersion = iota
	// Deprecated versions
	ipcVersion2
	ipcVersion3
	ipcDummyVersion3 // So that the numeric value of IPCVersion4 is 4
	IPCVersion4

	// IPCVersion5 uses the new security model (Principal and Blessings objects),
	// and sends discharges for third-party caveats on the server's blessings
	// during authentication.
	IPCVersion5
)

// IPCVersionRange allows you to optionally specify a range of versions to
// use when calling FormatEndpoint
type IPCVersionRange struct {
	Min, Max IPCVersion
}

// EndpointOpt implents the EndpointOpt interface so an IPCVersionRange
// can be used as an option to FormatEndpoint.
func (IPCVersionRange) EndpointOpt() {}
