package stream

import (
	"io"

	"v.io/core/veyron2/naming"
	"v.io/core/veyron2/security"
)

// Flow is the interface for a flow-controlled channel multiplexed on a Virtual
// Circuit (VC) (and its underlying network connections).
//
// This allows for a single level of multiplexing and flow-control over
// multiple concurrent streams (that may be used for RPCs) over multiple
// VCs over a single underlying network connection.
type Flow interface {
	io.ReadWriteCloser

	// LocalEndpoint returns the local veyron Endpoint
	LocalEndpoint() naming.Endpoint
	// RemoteEndpoint returns the remote veyron Endpoint
	RemoteEndpoint() naming.Endpoint
	// LocalPrincipal returns the Principal at the local end of the flow that has authenticated with the remote end.
	LocalPrincipal() security.Principal
	// LocalBlessings returns the blessings presented by the local end of the flow during authentication.
	LocalBlessings() security.Blessings
	// RemoteBlessings returns the blessings presented by the remote end of the flow during authentication.
	RemoteBlessings() security.Blessings
	// RemoteDischarges() returns the discharges presented by the remote end of the flow during authentication.
	//
	// The discharges are organized in a map keyed by the discharge-identifier.
	RemoteDischarges() map[string]security.Discharge
	// Cancel, like Close, closes the Flow but unlike Close discards any queued writes.
	Cancel()
	// Closed returns true if the flow has been closed or cancelled.
	IsClosed() bool
	// Closed returns a channel that remains open until the flow has been closed.
	Closed() <-chan struct{}

	// SetDeadline causes reads and writes to the flow to be
	// cancelled when the given channel is closed.
	SetDeadline(deadline <-chan struct{})

	// VCDataCache returns the stream.VCDataCache object that allows information to be
	// shared across the Flow's parent VC.
	VCDataCache() VCDataCache
}

// VCDataCache is a thread-safe store that allows data to be shared across a VC,
// with the intention of caching data that reappears over multiple flows.
type VCDataCache interface {
	// GetOrInsert returns the 'value' associated with 'key'. If an entry already exists in the
	// cache with the 'key', the 'value' is returned, otherwise 'create' is called to create a new
	// value N, the cache is updated, and N is returned.  GetOrInsert may be called from
	// multiple goroutines concurrently.
	GetOrInsert(key interface{}, create func() interface{}) interface{}
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
	Listen(protocol, address string, opts ...ListenerOpt) (Listener, naming.Endpoint, error)

	// Dial creates a VC to the provided remote endpoint.
	Dial(remote naming.Endpoint, opts ...VCOpt) (VC, error)

	// ShutdownEndpoint closes all VCs (and Flows and Listeners over it)
	// involving the provided remote endpoint.
	ShutdownEndpoint(remote naming.Endpoint)

	// Shutdown closes all VCs and Listeners (and Flows over them) and
	// frees up internal data structures.
	// The Manager is not usable after Shutdown has been called.
	Shutdown()

	// RoutingID returns the Routing ID associated with the VC.
	RoutingID() naming.RoutingID
}

// ManagerOpt is the interface for all Manager related options provided to Runtime.NewStreamManager.
type ManagerOpt interface {
	IPCStreamManagerOpt()
}
