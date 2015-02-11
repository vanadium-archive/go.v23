package naming

import (
	"net"
	"time"

	"v.io/core/veyron2/context"

	"v.io/core/veyron2/verror"
)

const (
	pkgPath         = "v.io/core/veyron2/naming"
	UnknownProtocol = ""
)

var (
	ErrNameExists              = verror.Register(pkgPath+".nameExists", verror.NoRetry, "{1} {2} Name exists {_}")
	ErrNoSuchName              = verror.Register(pkgPath+".nameDoesntExist", verror.NoRetry, "{1} {2} Name {3} doesn't exist {_}")
	ErrNoSuchNameRoot          = verror.Register(pkgPath+".rootNameDoesntExist", verror.NoRetry, "{1} {2} Namespace root name {3} doesn't exist {_}")
	ErrResolutionDepthExceeded = verror.Register(pkgPath+".resolutionDepthExceeded", verror.NoRetry, "{1} {2} Resolution depth exceeded {_}")
	ErrNoMountTable            = verror.Register(pkgPath+".noMounttable", verror.NoRetry, "{1} {2} No mounttable {_}")
)

// Endpoint represents unique identifiers for entities communicating over a
// network.  End users don't use endpoints - they deal solely with object names,
// with the MountTable providing translation of object names to endpoints.
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
	// Version 2 is the previous version for RPC:
	//   @2@<protocol>@<address>@<routingid>@<rpc version>@<rpc codec>@@
	//
	// Version 3 is the current version for RPC:
	//   @3@<protocol>@<address>@<routingid>@<rpc version>@<rpc codec>@m|s@@
	//
	// Along with Network, this method ensures that Endpoint implements net.Addr.
	String() string

	// Name returns a string reprsentation of this Endpoint that can
	// be used as a name with ipc.StartCall.
	Name() string

	// VersionedString returns a string in the specified format. If the version
	// number is unsupported, the current 'default' version will be used.
	VersionedString(version int) string

	// RoutingID returns the RoutingID associated with this Endpoint.
	RoutingID() RoutingID

	// Addrs returns a net.Addr whose String method will return the
	// the underlying network address encoded in the endpoint rather than
	// the endpoint string itself.
	// For example, for TCP based endpoints it will return a net.Addr
	// whose network is "tcp" and string representation is <host>:<port>,
	// than the full Veyron endpoint as per the String method above.
	Addr() net.Addr

	// ServesMountTable returns true if this endpoint serves a mount table.
	ServesMountTable() bool
}

// MountedServer represents a server mounted under an object name.
//
// TODO(toddw): Consolidate with VDLMountedServer once vdl supports time.
type MountedServer struct {
	Server           string    // Server is an object address (OA): endpoint + suffix
	Expires          time.Time // Absolute time after which the mount expires.
	BlessingPatterns []string  // Patterns matching blessings presented by Server.
}

// MountEntry represents a name mounted in the mounttable.
//
// TODO(toddw): Consolidate with VDLMountEntry once vdl supports time.
type MountEntry struct {
	// Name is the mounted name.
	Name string
	// Servers (if present) specifies the mounted names.
	Servers []MountedServer
	// mt is true if servers refer to another mount table.
	mt bool
	// Pattern is a security.BlessingPattern that should match the servers.
	Pattern string
}

// GlobError is returned by namespace.Glob to indicate a piece of the namespace
// that could not be traversed.
type GlobError struct {
	// Name is the mounted name.
	Name string
	// An error occurred fulfilling the request.
	Error error
}

// ServesMountTable returns true if the mount entry represents servers that are
// mount tables.
// TODO(p): When the endpoint actually has this fact encoded in, use that.
func (e *MountEntry) ServesMountTable() bool { return e.mt }

// SetServesMountTable sets whether or not this is a mount table.
func (e *MountEntry) SetServesMountTable(v bool) { e.mt = v }

// Names returns the servers represented by MountEntry as names, including
// the MountedName suffix.
func (e *MountEntry) Names() []string {
	var names []string
	for _, s := range e.Servers {
		names = append(names, JoinAddressName(s.Server, e.Name))
	}
	return names
}

// CacheCtl is a cache control for the resolution cache.
type CacheCtl interface {
	CacheCtl()
}

// DisbleCache disables the resolution cache when set to true and enables if false.
// As a side effect one can flush the cache by disabling and then reenabling it.
type DisableCache bool

func (DisableCache) CacheCtl() {}

// MountOpt is the interface for all Mount options.
type MountOpt interface {
	NSMountOpt()
}

// ReplaceMountOpt requests the mount to replace the previous mount.
type ReplaceMountOpt bool

func (ReplaceMountOpt) NSMountOpt() {}

// ServesMountTableOpt means the target is a mount table.
type ServesMountTableOpt bool

func (ServesMountTableOpt) NSMountOpt()  {}
func (ServesMountTableOpt) EndpointOpt() {}

// ResolveOpt is the interface for all Mount options.
type ResolveOpt interface {
	NSResolveOpt()
}

// RootBlessingPatternOpt specifies a blessing pattern that the root
// mount table must match.
type RootBlessingPatternOpt string

func (RootBlessingPatternOpt) NSResolveOpt() {}

// TODO(p): Perhaps add an ACL Opt.

// Namespace provides translation from object names to server object addresses.
// It represents the interface to a client side library for the MountTable
// service
type Namespace interface {
	// Mount the server object address under the object name, expiring after
	// the ttl. ttl of zero implies an implementation-specific high value
	// (essentially, forever).
	Mount(ctx *context.T, name, server string, ttl time.Duration, opts ...MountOpt) error

	// Unmount the server object address from the object name, or if server
	// is empty, unmount all server OAs from the object name.
	Unmount(ctx *context.T, name, server string) error

	// Resolve the object name into its mounted servers.
	Resolve(ctx *context.T, name string, opts ...ResolveOpt) (entry *MountEntry, err error)

	// ResolveToMountTable resolves the object name into the mounttables
	// directly responsible for the name.
	ResolveToMountTable(ctx *context.T, name string, opts ...ResolveOpt) (entry *MountEntry, err error)

	// FlushCacheEntry flushes resolution information cached for the name.  If
	// anything was flushed it returns true.
	FlushCacheEntry(name string) bool

	// CacheCtl sets controls and returns the current control values.
	CacheCtl(ctls ...CacheCtl) []CacheCtl

	// Glob returns MountEntry's whose name matches the pattern and GlobError's
	// for any piece of the space that can't be traversed.
	//
	// Two special patterns:
	//   prefix/... means all names below prefix.
	//   prefix/*** is like prefix/... but doesn't traverse into non-mounttable servers.
	//
	// Example:
	//	rc, err := ns.Glob(ctx, pattern)
	//	if err != nil {
	//		boom(t, "Glob(%s): %s", pattern, err)
	//	}
	//	for s := range rc {
	//		switch v := s.(type) {
	//		case *naming.MountEntry:
	//			fmt.Printf("%s: %v\n", v.Name, v.Servers)
	//		case *naming.GlobError:
	//			fmt.Fprintf(stderr, "%s can't be traversed: %s\n", v.Name, v.Error)
	//		}
	//	}
	Glob(ctx *context.T, pattern string) (chan interface{}, error)

	// SetRoots sets the roots that the local Namespace is
	// relative to. All relative names passed to the methods above
	// will be interpreted as relative to these roots. The roots
	// will be tried in the order that they are specified in the parameter
	// list for SetRoots. Calling SetRoots with no arguments will clear the
	// currently configured set of roots.
	SetRoots(roots ...string) error

	// Roots returns the currently configured roots. An empty slice is
	// returned if no roots are configured.
	Roots() []string
}

// When this prefix is present at the beginning of an object name suffix, the
// server may intercept the request and handle it internally. This is used to
// provide debugging, monitoring and other common functionality across all
// servers. Applications cannot use any name component that starts with this
// prefix.
const ReservedNamePrefix = "__"
