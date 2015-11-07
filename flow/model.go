// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package flow defines interfaces for the management of authenticated bidirectional byte Flows.
// TODO(suharshs): This is a work in progress and can change without notice.
//
// A Flow represents a flow-controlled authenticated byte stream between two endpoints.
//
// A Manager manages the creation of Flows and the re-use of network connections.
// A Manager can Dial out to a specific remote end to receive a Flow to that end.
// A Manager can Listen on multiple protocols and addresses. Listening
// causes the Manager to accept flows from any of the specified protocols and addresses.
// Additionally a Manager will accept incoming Dialed out connections for their lifetime.
package flow

import (
	"io"
	"net"
	"time"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc/version"
	"v.io/v23/security"
)

// Manager is the interface for managing the creation of Flows.
type Manager interface {
	// Listen causes the Manager to accept flows from the provided protocol and address.
	// Listen may be called muliple times.
	Listen(ctx *context.T, protocol, address string) error

	// ProxyListen causes the Manager to accept flows from the specified endpoint.
	// The endpoint must correspond to a vanadium proxy.
	// The call will block until the connection to the proxy endpoint fails. The
	// caller may then choose to retry the connection.
	ProxyListen(ctx *context.T, endpoint naming.Endpoint) error

	// ListeningEndpoints returns the endpoints that the Manager has explicitly
	// called Listen on. The Manager will accept new flows on these endpoints.
	// Proxied endpoints are included in the results.
	// If the Manager is not listening on any endpoints, an endpoint with the
	// Manager's RoutingID will be returned for use in bidirectional RPC.
	// Returned endpoints all have the Manager's unique RoutingID.
	// Closing of the returned channel indicates that the endpoints have changed
	// and ListeningEndpoints must be called to receive the fresh endpoints.
	// Once the manager is closed the returned channel will be nil.
	ListeningEndpoints() ([]naming.Endpoint, <-chan struct{})

	// StopListening stops listening on all currently listening addresses and proxies.
	// All outstanding calls to Accept will return an error.
	// It is safe to begin listening again.
	StopListening(ctx *context.T)

	// Accept blocks until a new Flow has been initiated by a remote process.
	// Flows are accepted from addresses that the Manager is listening on,
	// including outgoing dialed connections.
	//
	// For example:
	//   err := m.Listen(ctx, "tcp", ":0")
	//   for {
	//     flow, err := m.Accept(ctx)
	//     // process flow
	//   }
	//
	// can be used to accept Flows initiated by remote processes.
	Accept(ctx *context.T) (Flow, error)

	// Dial creates a Flow to the provided remote endpoint, using 'auth' to
	// determine the blessings that will be sent to the remote end.
	//
	// If the manager has a non-null RoutingID, the Manager will re-use connections
	// by Listening on Dialed connections for the lifetime of the Dialed connection.
	//
	// channelTimeout specifies the duration we are willing to wait before determining
	// that connections managed by this FlowManager are unhealthy and should be
	// closed.
	Dial(ctx *context.T, remote naming.Endpoint, auth PeerAuthorizer, channelTimeout time.Duration) (Flow, error)

	// RoutingID returns the naming.Routing of the flow.Manager.
	// If the RoutingID of the manager is naming.NullRoutingID, the manager can
	// only be used to Dial outgoing calls.
	RoutingID() naming.RoutingID

	// Closed returns a channel that remains open for the lifetime of the Manager
	// object. Once the channel is closed any operations on the Manager will
	// necessarily fail.
	Closed() <-chan struct{}
}

// PeerAuthorizer is the interface used in performing security authorization.
type PeerAuthorizer interface {
	// AuthorizePeer authorizes the remote blessings and returns the remote
	// blessing names, and those names rejected.
	AuthorizePeer(ctx *context.T,
		localEndpoint naming.Endpoint,
		remoteEndpoint naming.Endpoint,
		remoteBlessings security.Blessings,
		remoteDischarges map[string]security.Discharge,
	) (peerBlessingNames []string, rejectedPeerNames []security.RejectedBlessing, _ error)

	// BlessingsForPeer returns the blessings and discharges that should be
	// presented to the remote end with peerBlessingNames.
	BlessingsForPeer(ctx *context.T, peerBlessingNames []string) (
		security.Blessings, map[string]security.Discharge, error)
}

// ManagedConn represents the connection onto which this flow is multiplexed.
// Since this ManagedConn may be shared between many flows it wouldn't be safe
// to read and write to it directly.  We just provide some metadata.
type ManagedConn interface {
	// CommonVersion returns the RPCVersion negotiated between the local and remote endpoints.
	CommonVersion() version.RPCVersion
	// Closed returns a channel that remains open until the connection has been closed.
	Closed() <-chan struct{}
}

// MsgWriter defines and interface for writing messages.
type MsgWriter interface {
	// WriteMsg is like Write, but allows writing more than one buffer at a time.
	// The data in each buffer is written sequentially onto the flow.  Returns the
	// number of bytes written.  WriteMsg must return a non-nil error if it writes
	// less than the total number of bytes from all buffers.
	WriteMsg(data ...[]byte) (int, error)
}

// MsgReader defines an interface for reading messages.
type MsgReader interface {
	// ReadMsg is like read, but it reads bytes in chunks.  Depending on the
	// implementation the batch boundaries might or might not be significant.
	ReadMsg() ([]byte, error)
}

// MsgReadWriteCloser combines the MsgReader and MsgWriter interfaces and
// adds the Close method.
type MsgReadWriteCloser interface {
	MsgWriter
	MsgReader

	// Close closes the MsgReadWriteCloser. After Close is called all writes will
	// return an error, but reads of already queued data may succeed.
	Close() error
}

// Flow is the interface for a flow-controlled channel multiplexed over a Conn.
type Flow interface {
	io.ReadWriter
	MsgReadWriteCloser

	// WriteMsgAndClose performs WriteMsg and then closes the flow.
	WriteMsgAndClose(data ...[]byte) (int, error)

	// SetDeadlineContext sets the context associated with the flow.
	// It derives a context from the passed in context and the passed in deadline.
	// Typically this is used to set state that is only available after
	// the flow is connected, such as the language of the request.
	SetDeadlineContext(ctx *context.T, deadline time.Time) *context.T

	// LocalEndpoint returns the local vanadium Endpoint.
	LocalEndpoint() naming.Endpoint
	// RemoteEndpoint returns the remote vanadium Endpoint.
	RemoteEndpoint() naming.Endpoint
	// LocalBlessings returns the blessings presented by the local end of the flow
	// during authentication.
	LocalBlessings() security.Blessings
	// RemoteBlessings returns the blessings presented by the remote end of the
	// flow during authentication.
	RemoteBlessings() security.Blessings
	// LocalDischarges returns the discharges presented by the local end of the
	// flow during authentication.
	//
	// Discharges are organized in a map keyed by the discharge-identifier.
	LocalDischarges() map[string]security.Discharge
	// RemoteDischarges returns the discharges presented by the remote end of the
	// flow during authentication.
	//
	// Discharges are organized in a map keyed by the discharge-identifier.
	RemoteDischarges() map[string]security.Discharge

	// Conn returns the connection the flow is multiplexed on.
	Conn() ManagedConn

	// Closed returns a channel that remains open until the flow has been closed or
	// the ctx to the Dial or Accept call used to create the flow has been cancelled.
	Closed() <-chan struct{}
}

// Conn is the connection onto which flows are mulitplexed.
// It contains information of the Conn's local network address. Other infomation
// is not available until the authentication handshake is complete.
type Conn interface {
	MsgReadWriteCloser

	// LocalAddr returns the Conn's network address.
	LocalAddr() net.Addr
}

// Listener provides methods for accepting new Conns.
type Listener interface {
	// Accept waits for and are returns new Conns.
	Accept(ctx *context.T) (Conn, error)
	// Addr returns Listener's network address.
	Addr() net.Addr
	// Close closes the Listener. After Close is called all Accept calls will fail.
	Close() error
}
