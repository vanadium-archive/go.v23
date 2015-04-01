// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package options implements common options recognized by vanadium implementations.
//
// Below are the common options required of all vanadium implementations.  Let's
// say we have functions MyFuncA and MyFuncB in package demo:
//
//   package demo
//   func MyFuncA(a, b, c int, opts ...MyFuncAOpt)
//   func MyFuncB(opts ...MyFuncBOpt)
//
//   type MyFuncAOpt interface {
//     DemoMyFuncAOpt()
//   }
//   type MyFuncBOpt interface {
//     DemoMyFuncBOpt()
//   }
//
// The MyFuncAOpt interface is used solely to constrain the types of options
// that MyFuncA accepts, and ditto for MyFuncBOpt and MyFuncB.  In order to
// enable an option to be accepted by a particular function, you simply add a
// no-op function definition with the appropriate name.  An example:
//
//   type Foo int
//   func (Foo) DemoMyFuncAOpt() {}
//   func (Foo) DemoMyFuncBOpt() {}
//
//   type Bar string
//   func (Bar) DemoMyFuncBOpt() {}
//
// Foo is accepted by both demo.MyFuncA and demo.MyFuncB, while Bar is only
// accepted by demo.MyFuncB.  The methods defined for each option essentially
// act as annotations telling us which functions will accept them.
//
// Go stipulates that methods may only be attached to named types, and the type
// may not be an interface.  E.g.
//
//   // BAD: can't attach methods to named interfaces.
//   type Bad interface{}
//   func (Bad) DemoMyFuncAOpt() {}
//
//   // GOOD: wrap the interface in a named struct.
//   type Good struct { val interface{} }
//
//   func (Good) DemoMyFuncAOpt() {}
//
// These options can then be passed to the function as so:
//   MyFuncA(a, b, c, Foo(1), Good{object})
package options

import (
	"time"

	"v.io/v23/security"
)

// ServerBlessings is the blessings that should be presented by a process (a
// "server") accepting network connections from other processes ("clients").
//
// If none is provided, implementations typically use the "default" blessings
// from the BlessingStore of the Principal.
type ServerBlessings struct{ security.Blessings }

func (ServerBlessings) RPCServerOpt() {}

// SecurityLevel represents the level of confidentiality of data transmitted
// and received over a connection.
type SecurityLevel int

func (SecurityLevel) RPCServerOpt() {}
func (SecurityLevel) RPCCallOpt()   {}

const (
	// All user data transmitted over the connection is encrypted and can be
	// interpreted only by processes at the two ends of the connection.
	// This is the default level.
	SecurityConfidential SecurityLevel = 0
	// Data is transmitted over the connection in plain text and there is no authentication.
	SecurityNone SecurityLevel = 1
)

// AllowedServersPolicy specifies a policy used by clients to authenticate
// servers when making an RPC.
//
// This policy is specified as a set of patterns. At least one of these
// patterns must be matched by the blessings presented by the server
// before the client sends the contents of the RPC request (which
// include the method name, the arguments etc.).
//
// Not providing this option to StartCall is equivalent to providing
// AllowedServersPolicy{security.AllPrincipals}.
type AllowedServersPolicy []security.BlessingPattern

func (AllowedServersPolicy) RPCCallOpt() {}

// When ServerPublicKey is specified, the client will refuse to connect to
// servers with a different PublicKey.
type ServerPublicKey struct{ security.PublicKey }

func (ServerPublicKey) RPCCallOpt() {}

// SkipServerEndpointAuthorization causes clients to ignore the blessings in
// remote (server) endpoint during authorization. With this option enabled,
// clients are susceptible to man-in-the-middle attacks where an imposter
// server has taken over the network address of a real server.
//
// Thus, use of this option should typically be accompanied by an
// alternative policy for server authorization like AllowedServersPolicy
// or ServerPublicKey.
type SkipServerEndpointAuthorization struct{}

func (SkipServerEndpointAuthorization) RPCCallOpt()   {}
func (SkipServerEndpointAuthorization) NSResolveOpt() {}

// RetryTimeout is the duration during which we will retry starting
// an RPC call.  Zero means don't retry.
type RetryTimeout time.Duration

func (RetryTimeout) RPCCallOpt() {}

// NoResolve specifies that the RPC call should not further Resolve the name.
type NoResolve struct{}

func (NoResolve) RPCCallOpt()   {}
func (NoResolve) NSResolveOpt() {}

// Create a server that will be used to serve a MountTable. This server
// cannot be used for any other purpose.
type ServesMountTable bool

func (ServesMountTable) RPCServerOpt() {}

// When NoRetry is specified, the client will not retry calls that fail but would
// normally be retried.
type NoRetry struct{}

func (NoRetry) RPCCallOpt() {}
