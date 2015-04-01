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

	"v.io/v23/config"
	"v.io/v23/context"
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

	// Close discards all state associated with this Client.  In-flight calls may
	// be terminated with an error.
	Close()
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

// NewAddAddrsSetting creates the Setting to be sent to Listen to inform
// it of new addresses that have become available since the last change.
func NewAddAddrsSetting(a []Address) config.Setting {
	return config.NewAny(NewAddrsSetting, NewAddrsSettingDesc, a)
}

// NewRmAddrsSetting creates the Setting to be sent to Listen to inform
// it of addresses that are no longer available.
func NewRmAddrsSetting(a []Address) config.Setting {
	return config.NewAny(RmAddrsSetting, RmAddrsSettingDesc, a)
}

const (
	NewAddrsSetting     = "NewAddrs"
	NewAddrsSettingDesc = "New Addresses discovered since last change"
	RmAddrsSetting      = "RmAddrs"
	RmAddrsSettingDesc  = "Addresses that have been removed since last change"
)

// ListenAddrs is the set of protocol, address pairs to listen on.
// An anonymous struct is used to to more easily initialize a ListenSpec
// from a different package.
//
// For TCP, the address must be in <ip>:<port> format. The <ip> may be
// omitted, but the <port> can not (choose a port of 0 to have the system
// allocate one).
type ListenAddrs []struct {
	Protocol, Address string
}

// ListenSpec specifies the information required to create a set of listening
// network endpoints for a server and, optionally, the name of a proxy
// to use in conjunction with that listener.
type ListenSpec struct {
	// The addresses to listen on.
	Addrs ListenAddrs

	// The name of a proxy to be used to proxy connections to this listener.
	Proxy string

	// A publisher, which if non-nil, can be used to receive updated
	// network Settings via the stream named StreamName.
	StreamPublisher *config.Publisher
	// The name of the Stream to Fork on the StreamPublisher.
	StreamName string

	// AddressChooser returns a function that can be used to
	// choose the preferred address to publish with the mount table
	// when one is not otherwise specified.
	AddressChooser AddressChooser
}

// NetworkInterface represents a network interface.
type NetworkInterface interface {
	// Networks returns the set of networks accessible over this interface.
	Networks() []net.Addr
	// InterfaceIndex returns the index of this interface.
	InterfaceIndex() int
	// InterfaceName returns the name of this interface.
	InterfaceName() string
}

// Address represents a network address and the interface that hosts it.
type Address interface {
	// Address returns the network address this instance represents.
	Address() net.Addr
	NetworkInterface
}

func (l ListenSpec) String() string {
	s := ""
	for _, a := range l.Addrs {
		s += fmt.Sprintf("(%q, %q) ", a.Protocol, a.Address)
	}
	if len(l.Proxy) > 0 {
		s += "proxy(" + l.Proxy + ") "
	}
	if l.StreamPublisher != nil {
		s += "publisher(" + l.StreamName + ")"
	}
	return strings.TrimSpace(s)
}

// Copy clones a ListenSpec. The cloned spec has its own copy of the array of
// addresses to listen on.
func (l ListenSpec) Copy() ListenSpec {
	l.Addrs = append(ListenAddrs{}, l.Addrs...)
	return l
}

// AddressChooser returns the address it prefers out of the set passed to it
// for the specified network.
type AddressChooser func(network string, addrs []Address) ([]Address, error)

// Server defines the interface for managing a collection of services.
type Server interface {
	// Listen creates a listening network endpoint for the Server
	// as specified by its ListenSpec parameter. If any of the listen
	// addresses passed in the ListenSpec are 'unspecified' (e.g. don't
	// include a fixed address such as in ":0") and the ListenSpec includes
	// a Publisher, then 'roaming' support will be enabled. In this mode
	// the server will listen for changes in the network configuration
	// using a Stream created on the supplied Publisher and change the
	// set of Endpoints it publishes to the mount table accordingly.
	// The set of expected Settings received over the Stream is defined
	// by the New<setting>Functions above. The Publisher is ignored if
	// all of the addresses are specified.
	//
	// Listen may be called multiple times, but it must be called before
	// Serve or ServeDispatcher.
	//
	// Listen returns the set of endpoints that can be used to reach
	// this server. A single listen address in the ListenSpec can lead
	// to multiple such endpoints (e.g. :0 on a device with multiple interfaces
	// or that is being proxied). In the case where multiple listen addresses
	// are used it is not possible to tell which listen address supports which
	// Endpoint. If there is need to associate endpoints with specific
	// listen addresses then Listen should be called separately for each one.
	//
	// Any non-nil value of error can be converted to a verror.E.  If
	// error is nil and at least one address was supplied in the ListenSpec
	// then ListenEndpoints will include at least one Endpoint.
	Listen(spec ListenSpec) ([]naming.Endpoint, error)

	// Serve associates object with name by publishing the address of this
	// server with the mount table under the supplied name and using
	// authorizer to authorize access to it. RPCs invoked on the supplied
	// name will be delivered to methods implemented by the supplied object.
	//
	// Reflection is used to match requests to the object's method set.  As
	// a special-case, if the object implements the Invoker interface, the
	// Invoker is used to invoke methods directly, without reflection.
	//
	// If name is an empty string, no attempt will made to publish that
	// name to a mount table.
	//
	// It is an error to call Serve if ServeDispatcher has already been
	// called. It is also an error to call Serve multiple times.
	// It is considered an error to call Listen after Serve.
	Serve(name string, object interface{}, auth security.Authorizer) error

	// ServeDispatcher associates dispatcher with the portion of the mount
	// table's name space for which name is a prefix, by publishing the
	// address of this dispatcher with the mount table under the supplied
	// name.
	//
	// If name is an empty string, no attempt will made to publish that name
	// to a mount table.
	//
	// RPCs invoked on the supplied name will be delivered to the supplied
	// Dispatcher's Lookup method which will in turn return the object
	// and security.Authorizer used to serve the actual RPC call.
	// If name is an empty string, no attempt will made to publish that
	// name to a mount table.
	//
	// It is an error to call ServeDispatcher if Serve has already been
	// called. It is also an error to call ServeDispatcher multiple times.
	// It is considered an error to call Listen after ServeDispatcher.
	ServeDispatcher(name string, disp Dispatcher) error

	// AddName adds the specified name to the mount table for the object or
	// Dispatcher served by this server.
	// AddName may be called multiple times, but only after Serve or
	// ServeDispatcher have been called.
	AddName(name string) error

	// RemoveName removes the specified name from the mount table.
	// RemoveName may be called multiple times, but only after Serve or
	// ServeDispatcher have been called.
	RemoveName(name string)

	// Status returns the current status of the server, see ServerStatus
	// for details.
	Status() ServerStatus

	// WatchNetwork registers a channel over which NetworkChange's will
	// be sent. The Server will not block sending data over this channel
	// and hence change events may be lost if the caller doesn't ensure
	// there is sufficient buffering in the channel.
	WatchNetwork(ch chan<- NetworkChange)

	// UnwatchNetwork unregisters a channel previously registered using
	// WatchNetwork.
	UnwatchNetwork(ch chan<- NetworkChange)

	// Stop gracefully stops all services on this Server.  New calls are
	// rejected, but any in-flight calls are allowed to complete.  All
	// published mountpoints are unmounted.  This call waits for this
	// process to complete, and returns once the server has been shut down.
	Stop() error
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
	// ServerInit is the initial state of the server.
	ServerInit ServerState = iota
	// ServerActive indicates that the server is 'active' - i.e. its
	// Listen, Serve, ServeDispatcher, AddName or RemoveName methods
	// have been called.
	ServerActive
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
	case ServerInit:
		return "Initialized"
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
	Time    time.Time         // Time of last change.
	State   ServerState       // The current state of the server.
	Setting config.Setting    // The Setting sent for the last change.
	Changed []naming.Endpoint // The set of endpoints added/removed as a result of this change.
	Error   error             // Any error that encountered.
}

func (nc NetworkChange) DebugString() string {
	s := fmt.Sprintf("NetworkChange @ %s\n", nc.Time)
	s += fmt.Sprintf("  Setting: %s\n", nc.Setting)
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
	Lookup(suffix string) (interface{}, security.Authorizer, error)
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
	Prepare(method string, numArgs int) (argptrs []interface{}, tags []*vdl.Value, _ error)

	// Invoke is the second stage of method invocation.  It is passed the method
	// name, the in-flight call context, and the argptrs returned by Prepare,
	// filled in with decoded arguments.  It returns the results from the
	// invocation, and any errors in invoking the method.
	//
	// Note that argptrs is a slice of pointers to the argument objects; each
	// pointer must be dereferenced to obtain the actual arg value.
	Invoke(method string, call StreamServerCall, argptrs []interface{}) (results []interface{}, _ error)

	// Signature corresponds to the reserved __Signature method; it returns the
	// signatures of the interfaces the underlying object implements.
	Signature(call ServerCall) ([]signature.Interface, error)

	// MethodSignature corresponds to the reserved __MethodSignature method; it
	// returns the signature of the given method.
	MethodSignature(call ServerCall, method string) (signature.Method, error)

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
	// Glob__ returns a MountEntry for the objects that match the given
	// pattern in the namespace below the receiver object. All the names
	// returned are relative to the receiver.
	Glob__(call ServerCall, pattern string) (<-chan naming.GlobReply, error)
}

// ChildrenGlobber is a simple interface to publish the relationship between
// nodes in the namespace graph.
type ChildrenGlobber interface {
	// GlobChildren__ returns the names of the receiver's immediate children
	// on a channel.  It should return an error if the receiver doesn't
	// exist.
	GlobChildren__(call ServerCall) (<-chan string, error)
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

	// Context returns the current context.T.
	Context() *context.T
}

// TODO(ashankar): Remove this once VDL-generated files stop referencing it.
type BindOpt interface {
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
type Granter interface {
	// Grant grants a blessing to the provided server.
	Grant(server security.Blessings) (blessing security.Blessings, err error)

	// Granter implements the CallOpt interface so that
	// Granters can be provided as options to an RPC invocation.
	CallOpt
}
