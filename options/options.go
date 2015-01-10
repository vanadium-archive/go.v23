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

	"v.io/core/veyron2"
	"v.io/core/veyron2/ipc"
	"v.io/core/veyron2/ipc/stream"
	"v.io/core/veyron2/naming"

	"v.io/core/veyron2/security"
)

// TODO(suharshs, mattr): Remove the ROpts.

// RuntimePrincipal represents the principal to be used by the runtime.
//
// It wraps the security.Principal interface so that functions representing
// option annotations can be added.
type RuntimePrincipal struct{ security.Principal }

func (RuntimePrincipal) ROpt() {}

// ServerBlessings represents the blessings presented by a process (a "server")
// accepting network connections from other processes ("clients").
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

// StreamManager wraps the stream.Manager interface so that we can add
// functions representing the option annotations.
type StreamManager struct{ stream.Manager }

func (StreamManager) IPCClientOpt() {}

// Namespace wraps the naming.Namespace interface so that we can add
// functions representing the option annotations.
type Namespace struct{ naming.Namespace }

func (Namespace) IPCClientOpt() {}
func (Namespace) IPCServerOpt() {}

// Profile wraps the veyron2.Profile interface so that we can add
// functions representing the option annotations
type Profile struct{ veyron2.Profile }

func (Profile) ROpt() {}

// PreferredProtocolOrder instructs the Runtime implementation to select
// endpoints with the specified protocols and to order them in the
// specified order.
type PreferredProtocols []string

func (PreferredProtocols) ROpt()         {}
func (PreferredProtocols) IPCClientOpt() {}

// GoogleRuntime is the name of the Google runtime implementation.
const GoogleRuntime = "google"

// Create a server that will be used to serve a MountTable. This server
// cannot be used for any other purpose.
type ServesMountTable bool

func (ServesMountTable) IPCServerOpt() {}

// ReservedNameDispatcher specifies the dispatcher that controls access
// to framework managed portion of the namespace.
type ReservedNameDispatcher struct {
	Dispatcher ipc.Dispatcher
}

func (ReservedNameDispatcher) IPCServerOpt() {}
