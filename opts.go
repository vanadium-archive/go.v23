package veyron2

import (
	"time"

	"veyron.io/veyron/veyron2/config"
	"veyron.io/veyron/veyron2/ipc/stream"
	"veyron.io/veyron/veyron2/naming"
	"veyron.io/veyron/veyron2/security"
)

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
//   type FooOption int
//   func (FooOption) DemoMyFuncAOpt() {}
//   func (FooOption) DemoMyFuncBOpt() {}
//
//   type BarOption string
//   func (FooOption) DemoMyFuncBOpt() {}
//
// FooOption is accepted by both demo.MyFuncA and demo.MyFuncB, while BarOption
// is only accepted by demo.MyFuncB.  The methods defined for each option
// essentially act as annotations telling us which functions will accept them.
//
// Go stipulates that methods may only be attached to named types, and the type
// may not be an interface.  E.g.
//
//   // BAD: can't attach methods to named interfaces.
//   type BadOption interface{}
//   func (BadOption) DemoMyFuncAOpt() {}
//
//   // GOOD: wrap the interface in a named struct with a helper function.
//   func GoodOption(val interface{}) GoodOptionOpt {
//     return GoodOptionOpt{val}
//   }
//   type GoodOptionOpt struct {
//     val interface{}
//   }
//   func (GoodOption) DemoMyFuncAOpt() {}
//
// The helper function ensures that the caller ends up with the same syntax for
// creating options, e.g. here's some examples of using the options:
//
//   demo.MyFuncA(1, 2, 3, FooOption(5), GoodOption("good"))
//   demo.MyFuncB(FooOption(9), BarOption("bar"))

// RuntimeIDOpt represents the identity to be used by the runtime.
//
// It wraps the security.PrivateID interface so that functions representing
// option annotations can be added.
type RuntimeIDOpt struct{ security.PrivateID }

func (RuntimeIDOpt) ROpt() {}

// RuntimeID returns an option specifiying the PrivateID to be used by the
// local process.
func RuntimeID(id security.PrivateID) RuntimeIDOpt { return RuntimeIDOpt{id} }

// RuntimePublicIDStoreOpt represents the PublicIDStore to be used the local
// process.
type RuntimePublicIDStoreOpt struct{ security.PublicIDStore }

func (RuntimePublicIDStoreOpt) ROpt() {}

// RuntimePublicIDStore returns an option specifying the PublicIDStore to be used
// by the local process.
func RuntimePublicIDStore(s security.PublicIDStore) RuntimePublicIDStoreOpt {
	return RuntimePublicIDStoreOpt{s}
}

// LocalIDOpt represents the PublicID to be used by the local end of an IPC.
//
// It wraps the security.PrivateID interface so that functions representing
// option annotations can be added.
type LocalIDOpt struct{ security.PublicID }

func (LocalIDOpt) IPCClientOpt() {}
func (LocalIDOpt) IPCServerOpt() {}

// LocalID returns an option specifying the PublicID to be used by the local end
// of an IPC.
func LocalID(id security.PublicID) LocalIDOpt { return LocalIDOpt{id} }

// RemoteID specifies a pattern identifying the set of valid remote identities for
// a call.
type RemoteID security.BlessingPattern

func (RemoteID) IPCCallOpt() {}

// VCSecurityLevel represents the level of confidentiality of data transmitted
// and received over a VC.
type VCSecurityLevel int

func (VCSecurityLevel) IPCClientOpt()         {}
func (VCSecurityLevel) IPCServerOpt()         {}
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

// DischargeOpt wraps the security.Discharge interface so that we can
// add functions representing the option annotations.
type DischargeOpt struct{ security.Discharge }

func (DischargeOpt) IPCCallOpt() {}

// RetryTimeoutOpt is the duration during which we will retry starting
// an RPC call.  Zero means don't retry.
type RetryTimeoutOpt time.Duration

func (RetryTimeoutOpt) IPCCallOpt() {}

// StreamManager specifies an explicit stream.Manager.
func StreamManager(sm stream.Manager) StreamManagerOpt {
	return StreamManagerOpt{sm}
}

// StreamManagerOpt wraps the stream.Manager interface so that we can add
// functions representing the option annotations.
type StreamManagerOpt struct{ stream.Manager }

func (StreamManagerOpt) IPCClientOpt() {}

// Namespace specifies an explicit naming.Namespace.
func Namespace(mt naming.Namespace) NamespaceOpt {
	return NamespaceOpt{mt}
}

// NamespaceOpt wraps the naming.Namespace interface so that we can add
// functions representing the option annotations.
type NamespaceOpt struct{ naming.Namespace }

func (NamespaceOpt) IPCClientOpt() {}
func (NamespaceOpt) IPCServerOpt() {}

// NamespaceRoots wraps an array of strings so that we specify the root
// of the Namespace when initializing the runtime.
type NamespaceRoots []string

func (NamespaceRoots) ROpt() {}

// ProfileOpt wraps the Profile interface so that we can add
// functions representing the option annotations
type ProfileOpt struct{ Profile }

func (ProfileOpt) ROpt() {}

// RuntimeOpt wraps the name of the runtime so that we can add
// functions representing the option annotations
type RuntimeOpt struct{ Name string }

func (RuntimeOpt) ROpt() {}

// HTTPDebugOpt specifies the address on which an HTTP server will be run for
// debugging the process.
type HTTPDebugOpt string

func (HTTPDebugOpt) ROpt() {}

// Create a server that will be used to serve a MountTable. This server
// cannot be used for any other purpose.
type ServesMountTableOpt bool

func (ServesMountTableOpt) IPCServerOpt() {}

// AddressChooserOpt is a function that can be used to select
// the preferred address to use for publishing to the mount table when
// the default policy is inappropriate.
type AddressChooserOpt struct{ AddressChooser }

func (AddressChooserOpt) IPCServerOpt() {}

// RoamingPublisherOpt wraps a publisher name and associated stream to be used
// for receiving dynamically changing network settings over.
type RoamingPublisherOpt struct {
	Publisher  *config.Publisher
	StreamName string
}

func (RoamingPublisherOpt) IPCServerOpt() {}
