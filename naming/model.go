package naming

import (
	"net"
	"time"

	"veyron2/verror"
)

var (
	ErrNameExists              = verror.Make(verror.Exists, "Name already exists")
	ErrNoSuchName              = verror.Make(verror.NotFound, "Name doesn't exist")
	ErrNoSuchNameRoot          = verror.Make(verror.NotFound, "Name doesn't exist: root of namespace")
	ErrResolutionDepthExceeded = verror.Make(verror.Aborted, "Resolution depth exceeded")
	ErrNoMountTable            = verror.Make(verror.Internal, "No mount table available")
)

// Endpoint represents unique identifiers for entities communicating over a
// network.  End users don't use endpoints - they deal solely with veyron names,
// with the MountTable providing translation of veyron names to endpoints.
type Endpoint interface {
	// Network returns "veyron" so that Endpoint can implement net.Addr.
	Network() string

	// String returns a string representation of the endpoint.
	//
	// The String method formats the endpoint as:
	//   @<version>@<version specific fields>@@
	// Where version is an unsigned integer.
	//
	// Version 1 is the current version for network address information:
	//   @1@<protocol>@<address>@<routingid>@@
	// Where protocol is the underlying network protocol (tcp, bluetooth etc.)
	// and address is the address specific to that protocol (host:port for
	// tcp, MAC address for bluetooth etc.)
	//
	// Version 2 is the current version for RPC:
	//   @2@<protocol>@<address>@<routingid>@<rpc version>@<rpc codec>@@
	//
	// Along with Network, this method ensures that Endpoint implements net.Addr.
	String() string

	// RoutingID returns the RoutingID associated with this Endpoint.
	RoutingID() RoutingID

	// Addrs returns a net.Addr whose String method will return the
	// the underlying network address encoded in the endpoint rather than
	// the endpoint string itself.
	// For example, for TCP based endpoints it will return a net.Addr
	// whose network is "tcp" and string representation is <host>:<port>,
	// than the full Veyron endpoint as per the String method above.
	Addr() net.Addr
}

// MountedServer represents a server mounted under a veyron name.
type MountedServer struct {
	Server string        // Server is an object address (OA): endpoint + suffix
	TTL    time.Duration // Time-To-Live, after which the mount expires.
}

// MountEntry represents a name mounted in the mounttable.
type MountEntry struct {
	// Name is the mounted name.
	Name string
	// Servers (if present) specifies the mounted names (Link is empty).
	Servers []MountedServer
	// An error occurred fulfilling the request.
	Error error
}

// MountTable provides translation from veyron names to server object addresses.
type MountTable interface {
	// Mount the server OA under the veyron name, expiring after the ttl.
	// ttl of zero implies an implementation-specific high value
	// (essentially, forever).
	Mount(name, server string, ttl time.Duration) error

	// Unmount the server OA from the veyron name, or if server is empty, unmount
	// all server OAs from the veyron name.
	Unmount(name, server string) error

	// Resolve the veyron name into its mounted servers.
	Resolve(name string) (names []string, err error)

	// ResolveToMountTable resolves the veyron name into the mounttables
	// directly responsible for the name.
	ResolveToMountTable(name string) (names []string, err error)

	// TODO(caprita): consider adding a version of Unresolve to the
	// IDL-generated stub (in addition to UnresolveStep).

	// Unresolve returns the veyron name that resolves to the given name.
	// It can be the given name itself, though typically the service at the
	// given name will return the name of a mount table, which is then
	// followed up the namespace ancestry to obtain a name rooted at a
	// 'global' (i.e., widely accessible) mount table.
	Unresolve(name string) (names []string, err error)

	// Glob returns all names matching pattern.  If recursive is true, it also
	// returns all names below the matching ones.
	Glob(pattern string) (chan MountEntry, error)
}

type Runtime interface {
	// NewEndpoint returns an Endpoint by parsing the supplied endpoint
	// string as per the format described above. It can be used to test
	// a string to see if it's in valid endpoint format.
	//
	// NewEndpoint will accept srings both in the @ format described
	// above and in internet host:port format.
	//
	// All implementations of NewEndpoint should provide appropriate
	// defaults for any endpoint subfields not explicitly provided as
	// follows:
	// - a missing protocol will default to a protocol appropriate for the
	//   implementation hosting NewEndpoint
	// - a missing host:port will default to :0 - i.e. any port on all
	//   interfaces
	// - a missing routing id should default to the null routing id
	// - a missing codec version should default to AnyCodec
	// - a missing RPC version should default to the highest version
	//   supported by the runtime implementation hosting NewEndpoint
	NewEndpoint(ep string) (Endpoint, error)

	// MountTable returns the pre-configured MountTable that is created
	// when the Runtime is initialized.
	MountTable() MountTable
}
