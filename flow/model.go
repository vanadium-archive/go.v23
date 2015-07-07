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
//
// A Acceptor is used to accept flows from the Manager. Each Acceptor has a unique id
// that is in the endpoint for that Acceptor. The Manager routes incoming messages
// to the correct Acceptor based on that id.
package flow

import (
	"io"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
)

// Manager is the interface for managing the creation of Flows.
type Manager interface {
	// Acceptor creates a Acceptor that can be used to accept new flows from addresses
	// that the Manager is listening on.
	//
	// For example:
	//   err := m.Listen(ctx, "tcp", ":0")
	//   a, err := m.Acceptor(ctx, blessings)
	//   for {
	//     flow, err := a.Accept(ctx)
	//     // process flow
	//   }
	//
	// can be used to accept Flows initiated by remote processes to the endpoints returned
	// by Acceptor.Endpoints. All created Acceptors accept Flows from all addresses the
	// Manager is listening on, but are differentiated by a RoutingID unique to each
	// Acceptor.
	//
	// 'blessings' are the Blessings presented to the Client during authentication.
	Acceptor(ctx *context.T, blessings security.Blessings) (Acceptor, error)

	// Listen causes the Manager to accept flows from the provided protocol and address.
	// Listen may be called muliple times.
	Listen(ctx *context.T, protocol, address string) error

	// Dial creates a Flow to the provided remote endpoint. 'auth' is used to authorize
	// the remote end.
	//
	// To maximize re-use of connections, the Manager will also Listen on Dialed
	// connections for the lifetime of the connection.
	//
	// TODO(suharshs): Revisit passing in an authorizer here. Perhaps restrict server
	// authorization to a smaller set of policies, or use a different mechanism to
	// allow the user to specify a policy.
	Dial(ctx *context.T, remote naming.Endpoint, auth security.Authorizer) (Flow, error)

	// Closed returns a channel that remains open for the lifetime of the Manager
	// object. Once the channel is closed any operations on the Manager will
	// necessarily fail.
	Closed() <-chan struct{}
}

// Flow is the interface for a flow-controlled channel multiplexed over underlying network connections.
type Flow interface {
	io.ReadWriter

	// LocalEndpoint returns the local vanadium Endpoint
	LocalEndpoint() naming.Endpoint
	// RemoteEndpoint returns the remote vanadium Endpoint
	RemoteEndpoint() naming.Endpoint
	// LocalBlessings returns the blessings presented by the local end of the flow during authentication.
	LocalBlessings() security.Blessings
	// RemoteBlessings returns the blessings presented by the remote end of the flow during authentication.
	RemoteBlessings() security.Blessings
	// LocalDischarges returns the discharges presented by the local end of the flow during authentication.
	//
	// Discharges are organized in a map keyed by the discharge-identifier.
	LocalDischarges() map[string]security.Discharge
	// RemoteDischarges returns the discharges presented by the remote end of the flow during authentication.
	//
	// Discharges are organized in a map keyed by the discharge-identifier.
	RemoteDischarges() map[string]security.Discharge

	// Closed returns a channel that remains open until the flow has been closed or
	// the ctx to the Dial or Accept call used to create the flow has been cancelled.
	Closed() <-chan struct{}
}

// Acceptor is the interface for accepting Flows created by a remote process.
type Acceptor interface {
	// ListeningEndpoints returns the endpoints that the Manager has explicitly
	// listened on. The Acceptor will accept new flows on these endpoints.
	// Returned endpoints all have a RoutingID unique to the Acceptor.
	ListeningEndpoints() []naming.Endpoint
	// Accept blocks until a new Flow has been initiated by a remote process.
	Accept(ctx *context.T) (Flow, error)
	// Closed returns a channel that remains open until the Acceptor has been closed.
	// i.e. the context provided to Manager.Acceptor() has been cancelled. Once the
	// channel is closed, all calls to Accept will result in an error.
	Closed() <-chan struct{}
}
