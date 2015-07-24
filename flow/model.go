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

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
)

// Manager is the interface for managing the creation of Flows.
type Manager interface {
	// Listen causes the Manager to accept flows from the provided protocol and address.
	// Listen may be called muliple times.
	//
	// The flow.Manager associated with ctx must be the receiver of the method,
	// otherwise an error is returned.
	Listen(ctx *context.T, protocol, address string) error

	// ListeningEndpoints returns the endpoints that the Manager has explicitly
	// listened on. The Manager will accept new flows on these endpoints.
	// Returned endpoints all have a RoutingID unique to the Acceptor.
	ListeningEndpoints() []naming.Endpoint

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
	//
	// The flow.Manager associated with ctx must be the receiver of the method,
	// otherwise an error is returned.
	Accept(ctx *context.T) (Flow, error)

	// Dial creates a Flow to the provided remote endpoint, using 'fn' to
	// determine the blessings that will be sent to the remote end.
	//
	// To maximize re-use of connections, the Manager will also Listen on Dialed
	// connections for the lifetime of the connection.
	//
	// The flow.Manager associated with ctx must be the receiver of the method,
	// otherwise an error is returned.
	Dial(ctx *context.T, remote naming.Endpoint, fn BlessingsForPeer) (Flow, error)

	// Closed returns a channel that remains open for the lifetime of the Manager
	// object. Once the channel is closed any operations on the Manager will
	// necessarily fail.
	Closed() <-chan struct{}
}

// BlessingsForPeer is the type of a callback used in performing security
// authorization.  The remote end reveals its blessings first in the
// authorization protocol, so the callback should first authorize the remote
// blessings in the call.  Once authorized, the callback should return the
// blessings that will be revealed to the remote end.
type BlessingsForPeer func(ctx *context.T, call security.Call) (security.Blessings, error)

// Conn represents the connection onto which this flow is multiplexed.
// Since this Conn may be shared between many flows it wouldn't be safe
// to read and write to it directly.  We just provide some metadata.
type Conn interface {
	// LocalEndpoint returns the local vanadium Endpoint
	LocalEndpoint() naming.Endpoint
	// RemoteEndpoint returns the remote vanadium Endpoint
	RemoteEndpoint() naming.Endpoint
	// Closed returns a channel that remains open until the connection has been closed.
	Closed() <-chan struct{}
}

// MsgWriter defines and interface for writing messages.
type MsgWriter interface {
	// WriteMsg is like Write, but allows writing more than one buffer at a time.
	// The data in each buffer is written sequentially onto the flow.  Returns the
	// number of bytes written.  WriteV must return a non-nil error if it writes
	// less than the total number of bytes from all buffers.
	WriteMsg(data ...[]byte) (int, error)
}

// MsgReader defines an interface for reading messages.
type MsgReader interface {
	// ReadMsg is like read, but it reads bytes in chunks.  Depending on the
	// implementation the batch boundaries might or might not be significant.
	ReadMsg() ([]byte, error)
}

// MsgReadWriter combines the MsgReader and MsgWriter interfaces
type MsgReadWriter interface {
	MsgWriter
	MsgReader
}

// Flow is the interface for a flow-controlled channel multiplexed over a Conn.
type Flow interface {
	io.ReadWriter
	MsgReadWriter

	// WriteVAndClose performs WriteMsg and then closes the flow.
	WriteMsgAndClose(data ...[]byte) (int, error)

	// SetContext sets the context associated with the flow.  Typically this is
	// used to set state that is only available after the flow is connected, such
	// as a more restricted flow timeout, or the language of the request.
	//
	// The flow.Manager associated with ctx must be the same flow.Manager that the
	// flow was dialed or accepted from, otherwise an error is returned.
	SetContext(ctx *context.T) error

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
	Conn() Conn

	// Closed returns a channel that remains open until the flow has been closed or
	// the ctx to the Dial or Accept call used to create the flow has been cancelled.
	Closed() <-chan struct{}
}
