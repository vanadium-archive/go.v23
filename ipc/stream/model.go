package stream

import (
	"net"

	"veyron2/naming"
	"veyron2/security"
)

// Flow is the interface for a flow-controlled channel multiplexed on a Virtual
// Circuit (VC) (and its underlying network connections).
//
// This allows for a single level of multiplexing and flow-control over
// multiple concurrent streams (that may be used for RPCs) over multiple
// VCs over a single underlying network connection.
type Flow interface {
	// Flow objects implement the net.Conn interface.
	net.Conn

	// Cancel, like Close, closes the Flow but unlike Close discards any queued writes.
	Cancel()

	// LocalID returns the identity of the local end of a Flow.
	LocalID() security.PublicID

	// RemoteID returns the identity of the remote end of a Flow.
	RemoteID() security.PublicID

	// Closed returns true if the flow has been closed or cancelled.
	IsClosed() bool

	// Closed returns a channel that remains open until the flow has been closed.
	Closed() <-chan struct{}
}

// FlowOpt is the interface for all Flow options.
type FlowOpt interface {
	IPCStreamFlowOpt()
}

// Listener is the interface for accepting Flows created by a remote process.
type Listener interface {
	// Accept blocks until a new Flow has been initiated by a remote process.
	// TODO(toddw): This should be:
	//   Accept() (Flow, Connector, error)
	Accept() (Flow, error)

	// Close prevents new Flows from being accepted on this Listener.
	// Previously accepted Flows are not closed down.
	Close() error
}

// ListenerOpt is the interface for all options that control the creation of a
// Listener.
type ListenerOpt interface {
	IPCStreamListenerOpt()
}

// Connector is the interface for initiating Flows to a remote process over a
// Virtual Circuit (VC).
type Connector interface {
	Connect(opts ...FlowOpt) (Flow, error)
}

// VC is the interface for creating authenticated and secure end-to-end
// streams.
//
// VCs are multiplexed onto underlying network conections and can span
// multiple hops. Authentication and encryption are end-to-end, even though
// underlying network connections span a single hop.
type VC interface {
	Connector
	Listen() (Listener, error)
}

// VCOpt is the interface for all VC options.
type VCOpt interface {
	IPCStreamVCOpt()
}

// Manager is the interface for managing the creation of VCs.
type Manager interface {
	// Listen creates a Listener that can be used to accept Flows initiated
	// with the provided network address.
	//
	// For example:
	//   ln, ep, err := Listen("tcp", ":0")
	//   for {
	//     flow, err := ln.Accept()
	//     // process flow
	//   }
	// can be used to accept Flows initiated by remote processes to the endpoint
	// identified by the returned Endpoint.
	//
	// Typical options accepted:
	// veyron2.TLSConfig
	// veyron2.ServerID
	Listen(protocol, address string, opts ...ListenerOpt) (Listener, naming.Endpoint, error)

	// Dial creates a VC to the provided remote endpoint.
	//
	// Typical options accepted:
	// TODO(ashankar): Update comment as these option types will change
	// once we finalize the "public" and "private" ids in veyron2.
	// veyron2.ClientID - Identity of the caller
	// veyron2.ServerID - Expected identity of the server
	// veyron2.TLSConfig
	//
	// TODO: Should any of these security related options be made explicit
	// positional arguments?
	Dial(remote naming.Endpoint, opts ...VCOpt) (VC, error)

	// ShutdownEndpoint closes all VCs (and Flows and Listeners over it)
	// involving the provided remote endpoint.
	ShutdownEndpoint(remote naming.Endpoint)

	// Shutdown closes all VCs and Listeners (and Flows over them) and
	// frees up internal data structures.
	// The Manager is not usable after Shutdown has been called.
	Shutdown()
}

// ManagerOpt is the interface for all Manager related options provided to Runtime.NewStreamManager.
type ManagerOpt interface {
	IPCStreamManagerOpt()
}

// Runtime defines the factory and accessor methods that any runtime must provide.
type Runtime interface {
	NewStreamManager(opts ...ManagerOpt) (Manager, error)

	// StreamManager returns the pre-configured StreamManager that is created
	// when the Runtime is initialized.
	StreamManager() Manager
}
