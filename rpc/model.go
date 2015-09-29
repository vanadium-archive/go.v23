// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/vdl"
	"v.io/v23/vdlroot/signature"
)

// Client represents the interface for making RPC calls.  There may be multiple
// outstanding Calls associated with a single Client, and a Client may be
// used by multiple goroutines concurrently.
type Client interface {
	// StartCall starts an asynchronous call of the method on the server instance
	// identified by name, with the given input args (of any arity).  The returned
	// Call object manages streaming args and results, and finishes the call.
	//
	// StartCall accepts at least the following options:
	// v23.CallTimeout.
	StartCall(ctx *context.T, name, method string, args []interface{}, opts ...CallOpt) (ClientCall, error)

	// Call makes a synchronous call that will retry application level
	// verrors that have verror.ActionCode RetryBackoff.
	Call(ctx *context.T, name, method string, inArgs, outArgs []interface{}, callOpts ...CallOpt) error

	// Close discards all state associated with this Client.  In-flight calls may
	// be terminated with an error.
	// TODO(mattr): This method is deprecated with the new RPC system.
	Close()

	// Closed returns a channel that will be closed after the client is shut down.
	Closed() <-chan struct{}
}

// Call defines the interface for each in-flight call on the client.
// Finish must be called to finish the call; all other methods are optional.
type ClientCall interface {
	Stream

	// CloseSend indicates to the server that no more items will be sent; server
	// Recv calls will receive io.EOF after all sent items.  Subsequent calls to
	// Send on the client will fail.  This is an optional call - it's used by
	// streaming clients that need the server to receive the io.EOF terminator.
	CloseSend() error

	// Finish blocks until the server has finished the call, and fills resultptrs
	// with the positional output results (of any arity).
	Finish(resultptrs ...interface{}) error

	// RemoteBlesesings returns the blessings that the server provided to authenticate
	// with the client.
	//
	// It returns both the string blessings and a handle to the object that contains
	// cryptographic proof of the validity of those blessings.
	RemoteBlessings() ([]string, security.Blessings)
}

// Stream defines the interface for a bidirectional FIFO stream of typed values.
type Stream interface {
	// Send places the item onto the output stream, blocking if there is no buffer
	// space available.
	Send(item interface{}) error

	// Recv fills itemptr with the next item in the input stream, blocking until
	// an item is available.  Returns io.EOF to indicate graceful end of input.
	Recv(itemptr interface{}) error
}

// ListenAddrs is the set of protocol, address pairs to listen on.
// An anonymous struct is used to to more easily initialize a ListenSpec
// from a different package.
//
// For TCP, the address must be in <ip>:<port> format. The <ip> may be
// omitted, but the <port> can not (choose a port of 0 to have the system
// allocate one).
//
// TODO(toddw): Using an anonymous struct leads to having to use the
// following ugly syntax to work around "go vet":
//
// localhost := []struct{ Protocol, Address string }{{"tcp", "127.0.0.1:0"}}
// addrs := rpc.ListenAddrs(localhost)
type ListenAddrs []struct {
	Protocol, Address string
}

// AddressChooser determines the preferred address to publish with the mount
// table when one is not otherwise specified.
type AddressChooser interface {
	ChooseAddress(protocol string, candidates []net.Addr) ([]net.Addr, error)
}

// AddressChooserFunc is a convenience for implementations that wish to supply
// a function literal implementation of AddressChooser.
type AddressChooserFunc func(protocol string, candidates []net.Addr) ([]net.Addr, error)

func (f AddressChooserFunc) ChooseAddress(protocol string, candidates []net.Addr) ([]net.Addr, error) {
	return f(protocol, candidates)
}

// ListenSpec specifies the information required to create a set of listening
// network endpoints for a server and, optionally, the name of a proxy
// to use in conjunction with that listener.
type ListenSpec struct {
	// The addresses to listen on.
	Addrs ListenAddrs

	// The name of a proxy to be used to proxy connections to this listener.
	Proxy string

	// The address chooser to use for determining preferred publishing
	// addresses.
	AddressChooser
}

func (l ListenSpec) String() string {
	s := ""
	for _, a := range l.Addrs {
		s += fmt.Sprintf("(%q, %q) ", a.Protocol, a.Address)
	}
	if len(l.Proxy) > 0 {
		s += "proxy(" + l.Proxy + ") "
	}
	return strings.TrimSpace(s)
}

// Copy clones a ListenSpec. The cloned spec has its own copy of the array of
// addresses to listen on.
func (l ListenSpec) Copy() ListenSpec {
	l.Addrs = append(ListenAddrs{}, l.Addrs...)
	return l
}

// Server defines the interface for managing a server that receives RPC calls.
type Server interface {
	// AddName adds the specified name to the mount table for this server.
	// AddName may be called multiple times.
	AddName(name string) error

	// RemoveName removes the specified name from the mount table.  RemoveName may
	// be called multiple times.
	RemoveName(name string)

	// Status returns the current status of the server, see ServerStatus for
	// details.
	Status() ServerStatus

	// WatchNetwork registers a channel over which NetworkChange's will be
	// sent. The Server will not block sending data over this channel and hence
	// change events may be lost if the caller doesn't ensure there is sufficient
	// buffering in the channel.
	WatchNetwork(ch chan<- NetworkChange)

	// UnwatchNetwork unregisters a channel previously registered using
	// WatchNetwork.
	UnwatchNetwork(ch chan<- NetworkChange)

	// Stop gracefully stops all services on this Server.  New calls are rejected,
	// but any in-flight calls are allowed to complete.  All published mountpoints
	// are unmounted.  This call waits for this process to complete, and returns
	// once the server has been shut down.
	// TODO(mattr): This method is deprecated in the new RPC system.
	Stop() error

	// Closed returns a channel that will be closed after the server is shut down.
	Closed() <-chan struct{}
}

type ProxyStatus struct {
	// Name of the proxy.
	Proxy string
	// Endpoint is the Endpoint that this server is using to receive proxied
	// requests on. The Endpoint of the proxy itself can be obtained by
	// resolving its name.
	Endpoint naming.Endpoint
	// The error status of the connection to the proxy, nil if the connection
	// is currently correctly established, the most recent error otherwise.
	Error error
}

// ServerState represents the 'state' of the Server.
type ServerState int

const (
	// ServerActive indicates that the server is 'active'.
	ServerActive ServerState = iota
	// ServerStopping indicates that the server has been asked to stop and is
	// in the process of doing so. It may take a while for the server to
	// complete this process since it will wait for outstanding operations
	// to complete gracefully.
	ServerStopping
	// ServerStopped indicates that the server has stopped. It can no longer be
	// used.
	ServerStopped
)

func (i ServerState) String() string {
	switch i {
	case ServerActive:
		return "Active"
	case ServerStopping:
		return "Stopping"
	case ServerStopped:
		return "Stopped"
	default:
		return fmt.Sprintf("%s(%T=%d)", "%!s", i, i)
	}
}

// MountStatus contains the status of a given mount operation.
type MountStatus struct {
	// The Name and Server 'address' of this mount table request.
	Name, Server string
	// LastMount records the time of the last attempted mount request.
	LastMount time.Time
	// LastMountErr records any error reported by the last attempted mount.
	LastMountErr error
	// TTL is the TTL supplied for the last mount request.
	TTL time.Duration
	// LastUnount records the time of the last attempted unmount request.
	LastUnmount time.Time
	// LastUnmountErr records any error reported by the last attempted unmount.
	LastUnmountErr error
}

func (ms MountStatus) String() string {
	r := ""
	if !ms.LastMount.IsZero() {
		r += "Mounted @ " + ms.LastMount.String() + " TTL " + ms.TTL.String()
		if ms.LastMountErr != nil {
			r += " " + ms.LastMountErr.Error()
		}
	}
	if !ms.LastUnmount.IsZero() {
		r += "Unmounted @ " + ms.LastUnmount.String()
		if ms.LastUnmountErr != nil {
			r += " " + ms.LastUnmountErr.Error()
		}
	}
	return r
}

type MountState []MountStatus

// NetworkChange represents the changes made in response to a network
// Setting change being received.
type NetworkChange struct {
	Time         time.Time         // Time of last change.
	State        ServerState       // The current state of the server.
	AddedAddrs   []net.Addr        // Addresses added sinced the last change.
	RemovedAddrs []net.Addr        // Addresses removed since the last change.
	Changed      []naming.Endpoint // The set of endpoints added/removed as a result of this change.
	Error        error             // Any error that encountered.
}

func (nc NetworkChange) DebugString() string {
	s := fmt.Sprintf("NetworkChange @ %s\n", nc.Time)
	s += fmt.Sprintf("  Added: %s\n", nc.AddedAddrs)
	s += fmt.Sprintf("  Removed: %s\n", nc.RemovedAddrs)
	if nc.Error != nil {
		s += fmt.Sprintf("  Error: %s", nc.Error)
	}
	if len(nc.Changed) > 0 {
		s += fmt.Sprintf("  Endpoints: %d changes\n", len(nc.Changed))
		for i, ep := range nc.Changed {
			s += fmt.Sprintf("  %d:%s\n", i, ep)
		}
	}
	return s
}

type ServerStatus struct {
	// The current state of the server.
	State ServerState

	// ServesMountTable is true if this server serves a mount table.
	ServesMountTable bool

	// Mounts returns the status of the last mount or unmount
	// operation for every combination of name and server address being
	// published by this Server
	Mounts MountState

	// Endpoints contains the set of endpoints currently registered with the
	// mount table for the names published using this server but excluding
	// those used for serving proxied requests.
	Endpoints []naming.Endpoint

	// Errors contains any errors encountered when creating new listeners
	// or endpoints.
	Errors []error

	// Proxies contains the status of any proxy connections maintained by
	// this server.
	Proxies []ProxyStatus
}

func (st MountState) dedup(str func(MountStatus) string) []string {
	m := map[string]bool{}
	var r []string
	for _, v := range st {
		if s := str(v); !m[s] {
			m[s] = true
			r = append(r, s)
		}
	}
	sort.Strings(r)
	return r
}

// Names returns the current set of names being published by the publisher,
// these names not rooted at the mounttable.
func (st MountState) Names() []string {
	return st.dedup(func(v MountStatus) string { return v.Name })
}

// Servers retyuns the current set of server addresses being published by
// the publisher.
func (st MountState) Servers() []string {
	return st.dedup(func(v MountStatus) string { return v.Server })
}

// Dispatcher defines the interface that a server must implement to handle
// method invocations on named objects.
type Dispatcher interface {
	// Lookup returns the service implementation for the object identified
	// by the given suffix.
	//
	// Reflection is used to match requests to the service object's method
	// set.  As a special-case, if the object returned by Lookup implements
	// the Invoker interface, the Invoker is used to invoke methods
	// directly, without reflection.
	//
	// Returning a nil object indicates that this Dispatcher does not
	// support the requested suffix.
	//
	// An Authorizer is also returned to allow control over authorization
	// checks. Returning a nil Authorizer indicates the default
	// authorization checks should be used.
	//
	// Returning any non-nil error indicates the dispatch lookup has failed.
	// The error will be delivered back to the client.
	//
	// Lookup may be called concurrently by the underlying RPC system, and
	// hence must be thread-safe.
	Lookup(ctx *context.T, suffix string) (interface{}, security.Authorizer, error)
}

// Invoker defines the interface used by the server for invoking methods on
// named objects.  Typically ReflectInvoker(object) is used, which makes all
// exported methods on the given object invocable.
//
// Advanced users may implement this interface themselves for finer-grained
// control.  E.g. an RPC gateway that enables bindings for other languages (like
// javascript) may use this interface to support serving methods without an
// explicit intermediate object.
type Invoker interface {
	// Prepare is the first stage of method invocation, based on the given method
	// name.  The given numArgs specifies the number of input arguments sent by
	// the client, which may be used to support method overloading or generic
	// processing.
	//
	// Returns argptrs which will be filled in by the caller; e.g. the server
	// framework calls Prepare, and decodes the input arguments sent by the client
	// into argptrs.
	//
	// If the Invoker has access to the underlying Go values, it should return
	// argptrs containing pointers to the Go values that will receive the
	// arguments.  This is the typical case, e.g. the ReflectInvoker.
	//
	// If the Invoker doesn't have access to the underlying Go values, but knows
	// the expected types, it should return argptrs containing *vdl.Value objects
	// initialized to each expected type.  For purely generic decoding each
	// *vdl.Value may be initialized to vdl.AnyType.
	//
	// The returned method tags provide additional information associated with the
	// method.  E.g. the security system uses tags to configure AccessLists.  The tags
	// are typically configured in the VDL specification of the method.
	Prepare(ctx *context.T, method string, numArgs int) (argptrs []interface{}, tags []*vdl.Value, _ error)

	// Invoke is the second stage of method invocation.  It is passed the the
	// in-flight context and call, the method name, and the argptrs returned by
	// Prepare, filled in with decoded arguments.  It returns the results from the
	// invocation, and any errors in invoking the method.
	//
	// Note that argptrs is a slice of pointers to the argument objects; each
	// pointer must be dereferenced to obtain the actual arg value.
	Invoke(ctx *context.T, call StreamServerCall, method string, argptrs []interface{}) (results []interface{}, _ error)

	// Signature corresponds to the reserved __Signature method; it returns the
	// signatures of the interfaces the underlying object implements.
	Signature(ctx *context.T, call ServerCall) ([]signature.Interface, error)

	// MethodSignature corresponds to the reserved __MethodSignature method; it
	// returns the signature of the given method.
	MethodSignature(ctx *context.T, call ServerCall, method string) (signature.Method, error)

	// Globber allows objects to take part in the namespace.
	Globber
}

// Globber allows objects to take part in the namespace. Service objects may
// choose to implement either the AllGlobber interface, or the ChildrenGlobber
// interface.
//
// The AllGlobber interface lets the object handle complex glob requests for
// the entire namespace below the receiver object, i.e. "a/b".Glob__("...")
// must return the name of all the objects under "a/b".
//
// The ChildrenGlobber interface is simpler. Each object only has to return
// a list of the objects immediately below itself in the namespace graph.
type Globber interface {
	// Globber returns a GlobState with references to the interface that the
	// object implements. Only one implementation is needed to participate
	// in the namespace.
	Globber() *GlobState
}

// GlobState indicates which Glob interface the object implements.
type GlobState struct {
	AllGlobber      AllGlobber
	ChildrenGlobber ChildrenGlobber
}

// AllGlobber is a powerful interface that allows the object to enumerate the
// the entire namespace below the receiver object. Every object that implements
// it must be able to handle glob requests that could match any object below
// itself. E.g. "a/b".Glob__("*/*"), "a/b".Glob__("c/..."), etc.
type AllGlobber interface {
	// Glob__ returns a GlobReply for the objects that match the given
	// glob pattern in the namespace below the receiver object. All the
	// names returned are relative to the receiver.
	Glob__(ctx *context.T, call GlobServerCall, g *glob.Glob) error
}

// GlobServerCall defines the in-flight context for a Glob__ call, including the
// method to stream the results.
type GlobServerCall interface {
	SendStream() interface {
		Send(reply naming.GlobReply) error
	}
	ServerCall
}

// ChildrenGlobber allows the object to enumerate the namespace immediately
// below the receiver object.
type ChildrenGlobber interface {
	// GlobChildren__ returns a GlobChildrenReply for the receiver's
	// immediate children that match the glob pattern element.
	// It should return an error if the receiver doesn't exist.
	GlobChildren__(ctx *context.T, call GlobChildrenServerCall, matcher *glob.Element) error
}

// GlobChildrenServerCall defines the in-flight context for a GlobChildren__
// call, including the method to stream the results.
type GlobChildrenServerCall interface {
	SendStream() interface {
		Send(reply naming.GlobChildrenReply) error
	}
	ServerCall
}

// StreamServerCall defines the in-flight context for a server method
// call, including methods to stream args and results.
type StreamServerCall interface {
	Stream
	ServerCall
}

// ServerCall defines the in-flight context for a server method call, not
// including methods to stream args and results.
type ServerCall interface {
	// Security returns the security-related state associated with the call.
	Security() security.Call
	// Suffix returns the object name suffix for the request.
	Suffix() string
	// LocalEndpoint returns the Endpoint at the local end of
	// communication.
	LocalEndpoint() naming.Endpoint
	// RemoteEndpoint returns the Endpoint at the remote end of
	// communication.
	RemoteEndpoint() naming.Endpoint
	// GrantedBlessings are blessings granted by the client to the server
	// (bound to the server).  Typically provided by a client to delegate
	// to the server, allowed the server to use the client's authority to
	// pursue some task.
	//
	// Can be nil, indicating that the client did not delegate any
	// authority to the server for this request.
	//
	// This is distinct from the blessings used by the client and
	// server to authenticate with each other (RemoteBlessings
	// and LocalBlessings respectively).
	GrantedBlessings() security.Blessings
	// Server returns the Server that this context is associated with.
	Server() Server
}

// CallOpt is the interface for all Call options.
type CallOpt interface {
	RPCCallOpt()
}

// ClientOpt is the interface for all Client options.
type ClientOpt interface {
	RPCClientOpt()
}

// ServerOpt is the interface for all Server options.
type ServerOpt interface {
	RPCServerOpt()
}

// Granter is a ClientCallOpt that is used to provide blessings to
// the server when making an RPC.
//
// It gets passed a context.T with parameters of the RPC call set on
// it.
type Granter interface {
	Grant(ctx *context.T, call security.Call) (security.Blessings, error)

	CallOpt
}
