// Package options implements common options recognized by veyron implementations.
//
// Below are the common options required of all veyron implementations.  Let's
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

func (ServerBlessings) IPCServerOpt()         {}
func (ServerBlessings) IPCStreamListenerOpt() {}

// VCSecurityLevel represents the level of confidentiality of data transmitted
// and received over a VC.
type VCSecurityLevel int

func (VCSecurityLevel) IPCServerOpt()         {}
func (VCSecurityLevel) IPCCallOpt()           {}
func (VCSecurityLevel) IPCStreamVCOpt()       {}
func (VCSecurityLevel) IPCStreamListenerOpt() {}

const (
	// All user data transmitted over the VC is encrypted and can be interpreted only
	// by processes at the two ends of the VC.
	// This is the default level.
	VCSecurityConfidential VCSecurityLevel = 0
	// Data is transmitted over the VC in plain text and there is no authentication.
	VCSecurityNone VCSecurityLevel = 1
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

func (AllowedServersPolicy) IPCCallOpt() {}

// When ServerPublicKey is specified, the client will refuse to connect to
// servers with a different PublicKey.
type ServerPublicKey struct{ security.PublicKey }

func (ServerPublicKey) IPCCallOpt() {}

// SkipResolveAuthorization causes clients to ignore the expected server
// blessings provided during namespace resolution. In other words, it causes
// clients to skip authorization of servers when resolving names to addresses
// (endpoints).
//
// When providing this option, secure clients will typically specify an
// authorization policy via other options like AllowedServersPolicy or
// ServerPublicKey.
type SkipResolveAuthorization struct{}

func (SkipResolveAuthorization) IPCCallOpt()   {}
func (SkipResolveAuthorization) NSResolveOpt() {}

// Discharge wraps the security.Discharge interface so that we can
// add functions representing the option annotations.
type Discharge struct{ security.Discharge }

func (Discharge) IPCCallOpt() {}

// RetryTimeout is the duration during which we will retry starting
// an RPC call.  Zero means don't retry.
type RetryTimeout time.Duration

func (RetryTimeout) IPCCallOpt() {}

// NoResolve specifies that the RPC call should not further Resolve the name.
type NoResolve struct{}

func (NoResolve) IPCCallOpt()   {}
func (NoResolve) NSResolveOpt() {}

// GoogleRuntime is the name of the Google runtime implementation.
const GoogleRuntime = "google"

// Create a server that will be used to serve a MountTable. This server
// cannot be used for any other purpose.
type ServesMountTable bool

func (ServesMountTable) IPCServerOpt() {}

// When NoRetry is specified, the client will not retry calls that fail but would
// normally be retried.
type NoRetry struct{}

func (NoRetry) IPCCallOpt() {}
