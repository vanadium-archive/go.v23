package veyron2

import (
	"time"

	"veyron2/ipc/stream"
	"veyron2/naming"
	"veyron2/product"
	"veyron2/security"
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

// LocalIDOpt represents the identity of the local process.
//
// It wraps the security.PrivateID interface so that functions representing
// option annotations can be added.
type LocalIDOpt struct{ security.PrivateID }

func (LocalIDOpt) IPCClientOpt()         {}
func (LocalIDOpt) IPCServerOpt()         {}
func (LocalIDOpt) IPCStreamVCOpt()       {}
func (LocalIDOpt) IPCStreamListenerOpt() {}
func (LocalIDOpt) ROpt()                 {}

// LocalID specifies the identity of the local process.
func LocalID(id security.PrivateID) LocalIDOpt { return LocalIDOpt{id} }

// RemoteID specifies a pattern identifying the set of valid remote identities for
// a call.
type RemoteID security.PrincipalPattern

func (RemoteID) IPCClientCallOpt() {}

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

// CallTimeout specifies the timeout for Call.
type CallTimeout time.Duration

func (CallTimeout) IPCClientCallOpt() {}
func (CallTimeout) IPCClientOpt()     {}

// StreamManager specifies an explicit stream.Manager.
func StreamManager(sm stream.Manager) StreamManagerOpt {
	return StreamManagerOpt{sm}
}

// StreamManagerOpt wraps the stream.Manager interface so that we can add
// functions representing the option annotations.
type StreamManagerOpt struct{ stream.Manager }

func (StreamManagerOpt) IPCClientOpt() {}
func (StreamManagerOpt) IPCServerOpt() {}

// MountTable specifies an explicit naming.MountTable.
func MountTable(mt naming.MountTable) MountTableOpt {
	return MountTableOpt{mt}
}

// MountTableOpt wraps the naming.MountTable interface so that we can add
// functions representing the option annotations.
type MountTableOpt struct{ naming.MountTable }

func (MountTableOpt) IPCClientOpt() {}
func (MountTableOpt) IPCServerOpt() {}

// MountTableRoots wraps an array of strings so that we specify the root
// of the mounttable when initializing the runtime.
type MountTableRoots []string

func (MountTableRoots) ROpt() {}

// RuntimeOpt wraps the Runtime interface so that we can add
// functions representing the option annotations.
type RuntimeOpt struct{ Runtime }

func (RuntimeOpt) IPCBindOpt() {}

// ProductOpt wraps the product.T interface so that we can add
// functions representing the option annotations
type ProductOpt struct{ product.T }

func (ProductOpt) ROpt() {}

// HTTPDebugOpt specifies the address on which an HTTP server will be run for
// debugging the process.
type HTTPDebugOpt string

func (HTTPDebugOpt) ROpt() {}
