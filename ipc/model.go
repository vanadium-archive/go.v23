package ipc

import (
	"time"

	"veyron2/naming"
	"veyron2/security"
)

// Client represents the interface for making RPC calls.  There may be multiple
// outstanding ClientCalls associated with a single Client, and a Client may be
// used by multiple goroutines concurrently.
type Client interface {
	// Client can be provided to Bind<Service> calls.
	BindOpt

	// StartCall starts an asynchronous call of the method on the server instance
	// identified by name, with the given input args (of any arity).  The returned
	// Call object manages streaming args and results, and finishes the call.
	//
	// StartCall accepts at least the following options:
	// veyron2.CallTimeout.
	StartCall(name, method string, args []interface{}, opts ...ClientCallOpt) (ClientCall, error)

	// Close discards all state associated with this Client.  In-flight calls may
	// be terminated with an error.
	Close()
}

// ClientCall defines the interface for each in-flight call on the client.
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

	// Cancel the call.  The server will stop processing, if possible.  Calls to
	// Finish will return immediately with an error indicating the cancellation.
	// It is safe to call Cancel concurrently with any other ClientCall method.
	Cancel()
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

// Server defines the interface for managing a collection of services.
type Server interface {
	// Register associates a Dispatcher with a name prefix.  Dispatchers are used
	// in order of the longest prefix matching the name specified in the incoming
	// request.  Multiple dispatchers may be associated with the same prefix
	// (which may be the empty string), in which case they will be invoked in the
	// same order as the invocations of Register.  Path components (the substring
	// between /'s) are not partially matched.
	//
	//   Register("media/video", videoSvc)
	//   Register("media", mediaSvc)
	// and
	//   Register("media", mediaSvc)
	//   Register("media/video", videoSvc)
	// will both result in videoSvc being invoked for names of the form
	// "media/video/*"
	Register(prefix string, disp Dispatcher) error

	// Listen creates a listening network endpoint for the Server.  The
	// meaning of the arguments are similar to what Go's net.Listen accepts.
	// For the special protocol "veyron", the address can also be:
	// - a formatted Veyron endpoint
	// - a Veyron name which resolves to an endpoint
	//
	// Listen may be called multiple times to listen on multiple endpoints.
	// The returned endpoint represents an address that will be published
	// with the mount table when Publish (below) is called.
	Listen(protocol, address string) (naming.Endpoint, error)

	// Publish enables the services registered thus far to service RPCs.  It
	// will register them with the mount table and maintain that
	// registration so long as Stop has not been called.  The name
	// determines where in the mount table's name tree the new services will
	// appear.  The name is applied as a prefix to the prefixes specified in
	// Register.
	//
	// To serve names of the form "mymedia/media/*" make the calls:
	//   Register("media", mediaSvc)
	//   Publish("mymedia")
	//
	// Publish may be called multiple times to publish the same server under
	// multiple names.
	//
	// TODO(toddw): If Listen hasn't been called yet, it will be called using a
	// default protocol/address?
	Publish(name string) error

	// Published returns the rooted names that this server's services have
	// been published as.
	//
	// For example, if the mount table's rooted name is mtep/mt, and
	// Publish("a") and Publish("b") were called on this server, then
	// Published would return ["mtep/mt/a", "mtep/mt/b"].
	Published() ([]string, error)

	// Stop gracefully stops all services on this Server.  New calls are
	// rejected, but any in-flight calls are allowed to complete.  All
	// published mountpoints are unmounted.  This call waits for this
	// process to complete, and returns once the server has been shut down.
	Stop() error
}

// Dispatcher defines the interface that a server must implement to handle
// method invocations on named objects.
type Dispatcher interface {
	// Lookup returns an Invoker that serves methods for the object identified by
	// the given suffix.  Returning a nil Invoker with a nil error indicates this
	// Dispatcher does not handle the object - the framework will try other
	// Dispatchers to serve the method.
	//
	// An Authorizer is also returned to allow control over authorization checks.
	// Returning a nil Authorizer indicates the default authorization checks
	// should be used.
	//
	// Returning any non-nil error indicates the dispatch lookup has failed.  The
	// error will be delivered back to the client, and no further dispatch lookups
	// will be performed.
	//
	// Lookup may be invoked concurrently by the underlying RPC system, and hence
	// must be thread-safe.
	Lookup(suffix string) (Invoker, security.Authorizer, error)
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
	// Prepare is the first stage of method invocation.  It returns a slice of
	// pointers to argument objects, which will be used by the framework to decode
	// the arguments sent by the client.  If the arg types are known, argptrs
	// should contain pointers to args of the appropriate type.  Otherwise argptrs
	// may contain vom.Value objects to support generic arg decoding.
	//
	// numArgs specifies the number of input arguments sent by the client - it may
	// be used to support method overloading, e.g. for different versions of a
	// method with the same name.  The returned security label is used in the
	// subsequent authorization check.
	Prepare(method string, numArgs int) (argptrs []interface{}, label security.Label, err error)

	// Invoke is the second stage of method invocation.  It is passed the method
	// name, the in-flight call context, and the argptrs returned by Prepare,
	// filled in with decoded arguments.  It returns the results from the
	// invocation, and any errors in invoking the method.
	//
	// Note that argptrs is a slice of pointers to the argument objects; each
	// pointer must be dereferenced to obtain the actual arg value.
	Invoke(method string, call ServerCall, argptrs []interface{}) (results []interface{}, err error)
}

// Unresolver defines the interface to be implemented by service objects
// that want to define their custom UnresolveStep functionality.
type Unresolver interface {
	UnresolveStep(context Context) ([]string, error)
}

// ServerCall defines the interface for each in-flight call on the server.
type ServerCall interface {
	Stream
	Context
}

// Context defines the in-flight call state on the server, not including methods
// to stream args and results.
type Context interface {
	security.Context
	// Deadline returns the deadline for this call.
	Deadline() time.Time
	// IsClosed returns true iff the call has been cancelled or otherwise closed.
	IsClosed() bool
	// Closed returns a channel that remains open until the call has been
	// cancelled or otherwise closed.
	Closed() <-chan struct{}
	// Server returns the Server that this context is associated with.
	Server() Server
}

// BindOpt is the interface for options provided to Bind<Service> calls in IPC
// clients.
type BindOpt interface {
	IPCBindOpt()
}

// ClientCallOpt is the interface for all ClientCall options.
type ClientCallOpt interface {
	IPCClientCallOpt()
}

// ClientOpt is the interface for all Client options.
type ClientOpt interface {
	IPCClientOpt()
}

// ServerOpt is the interface for all Server options.
type ServerOpt interface {
	IPCServerOpt()
}
