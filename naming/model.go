// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package naming

import (
	"net"

	"v.io/v23/rpc/version"
	"v.io/v23/verror"
)

const (
	pkgPath         = "v.io/v23/naming"
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
	// Network returns "v23" so that Endpoint can implement net.Addr.
	Network() string

	// String returns a string representation of the endpoint.
	//
	// The String method formats the endpoint as:
	//   @<version>@<version specific fields>@@
	// Where version is an unsigned integer.
	//
	// TODO(ashankar,cnicolaou): Remove versions 2 & 3 before release?
	//
	// Version 1 is the current version for network address information:
	//   @1@<protocol>@<address>@<routingid>@@
	// Where protocol is the underlying network protocol (tcp, bluetooth etc.)
	// and address is the address specific to that protocol (host:port for
	// tcp, MAC address for bluetooth etc.)
	//
	// Version 2 is an old version for RPC:
	//   @2@<protocol>@<address>@<routingid>@<rpc version>@<rpc codec>@@
	//
	// Version 3 is an old version for RPC:
	//   @3@<protocol>@<address>@<routingid>@<rpc version>@<rpc codec>@m|s@@
	//
	// Version 4 is the previous version for RPC:
	//   @4@<protocol>@<address>@<routingid>@<min RPC version>@<max RPC version>@m|s@[<blessing>[,<blessing>]...]@@
	// Version 5 is the current version for RPC:
	//   @5@<protocol>@<address>@<routingid>@m|s@[<blessing>[,<blessing>]...]@@
	//
	// Along with Network, this method ensures that Endpoint implements net.Addr.
	String() string

	// Name returns a string reprsentation of this Endpoint that can
	// be used as a name with rpc.StartCall.
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
	// than the full Vanadium endpoint as per the String method above.
	Addr() net.Addr

	// ServesMountTable returns true if this endpoint serves a mount table.
	ServesMountTable() bool

	// ServesLeaf returns true if this endpoint serves a leaf server.
	ServesLeaf() bool

	// BlessingNames returns the blessings that the process associated with
	// this Endpoint will present.
	BlessingNames() []string

	// RPCVersionRange returns the range of protocol versions supported by
	// the process at this endpoint.
	RPCVersionRange() version.RPCVersionRange
}

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

// NamespaceOpt is the interface for all Namespace options.
type NamespaceOpt interface {
	NSOpt()
}

// ReplaceMount requests the mount to replace the previous mount.
type ReplaceMount bool

func (ReplaceMount) NSOpt() {}

// ServesMountTable means the target is a mount table.
type ServesMountTable bool

func (ServesMountTable) NSOpt()       {}
func (ServesMountTable) EndpointOpt() {}

// IsLeaf means the target is a leaf
type IsLeaf bool

func (IsLeaf) NSOpt() {}

// BlessingOpt is used to add a blessing name to the endpoint.
type BlessingOpt string

func (BlessingOpt) EndpointOpt() {}

// When this prefix is present at the beginning of an object name suffix, the
// server may intercept the request and handle it internally. This is used to
// provide debugging, monitoring and other common functionality across all
// servers. Applications cannot use any name component that starts with this
// prefix.
const ReservedNamePrefix = "__"
