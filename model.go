/*
Package veyron2 defines the Runtime interface of the public Veyron API and its subdirectories define the entire Veyron public API.

These public APIs will be stable over an extended period and changes to
them will be carefully managed to ensure backward compatibility. The same
policy as used for go (http://golang.org/doc/go1compat) will be used for these
APIs.
*/
package veyron2

import (
	"veyron2/ipc"
	"veyron2/ipc/stream"
	"veyron2/naming"
	"veyron2/product"
	"veyron2/security"
	"veyron2/vlog"
)

// LocalStop is the message received on WaitForStop when the stop was initiated
// by the process itself.
const (
	LocalStop             = "localstop"
	UnhandledStopExitCode = 1
	ForceStopExitCode     = 1
)

// TODO(caprita): make Stop return chan Tick and expose the ability to convey
// shutdown progress to application layer.
// type Tick struct {
// 	Processed, Goal int32
// }

// Runtime is the interface that concrete Veyron implementations must
// implement.
type Runtime interface {
	// Product returns the Product that the current process is running on.
	Product() product.T

	// NewIdentity creates a new PrivateID with the provided name.
	NewIdentity(name string) (security.PrivateID, error)

	// Identity returns the default identity used by the runtime.
	Identity() security.PrivateID

	// NewClient creates a new Client instance.
	//
	// It accepts at least the following options:
	// Client, ClientID and StreamManager
	//
	// In particular, if the options include a Client, then NewClient
	// just returns that.
	NewClient(opts ...ipc.ClientOpt) (ipc.Client, error)

	// NewServer creates a new Server instance.
	//
	// It accepts at least the following options:
	// ServerID and StreamManager
	NewServer(opts ...ipc.ServerOpt) (ipc.Server, error)

	// Client returns the pre-configured Client that is created when the
	// Runtime is initialized.
	Client() ipc.Client

	// Background is a factory function to generate a background client context.
	// This should be used when you are doing a new operation that isn't related
	// to another ongoing RPC.
	NewContext() ipc.Context

	// TODO is a factory function to generate a new client context.
	// This should be used when some context should be supplied but you aren't yet
	// ready to fill in the correct value.  The idea is that no TODO context
	// should remain in the codebase long-term.
	// TODO(mattr): Remove this method entirely once the whole tree has Context
	// piped through it.
	TODOContext() ipc.Context

	// NewStreamManager creates a new stream manager.
	NewStreamManager(opts ...stream.ManagerOpt) (stream.Manager, error)

	// StreamManager returns the pre-configured StreamManager that is created
	// when the Runtime is initialized.
	StreamManager() stream.Manager

	// NewEndpoint returns an Endpoint by parsing the supplied endpoint
	// string as per the format described above. It can be used to test
	// a string to see if it's in valid endpoint format.
	//
	// NewEndpoint will accept srings both in the @ format described
	// above and in internet host:port format.
	//
	// All implementations of NewEndpoint should provide appropriate
	// defaults for any endpoint subfields not explicitly provided as
	// follows:
	// - a missing protocol will default to a protocol appropriate for the
	//   implementation hosting NewEndpoint
	// - a missing host:port will default to :0 - i.e. any port on all
	//   interfaces
	// - a missing routing id should default to the null routing id
	// - a missing codec version should default to AnyCodec
	// - a missing RPC version should default to the highest version
	//   supported by the runtime implementation hosting NewEndpoint
	NewEndpoint(ep string) (naming.Endpoint, error)

	// MountTable returns the pre-configured MountTable that is created
	// when the Runtime is initialized.
	MountTable() naming.MountTable

	// Logger returns the current logger in use by the Runtime.
	Logger() vlog.Logger

	// NewLogger creates a new instance of the logging interface that is
	// separate from the one provided by Runtime.
	NewLogger(name string, opts ...vlog.LoggingOpts) (vlog.Logger, error)

	// Stop causes all the channels returned by WaitForStop to return the
	// LocalStop message, to give the application a chance to shut down.
	// Stop does not block.  If any of the channels are not receiving,
	// the message is not sent on them.
	// If WaitForStop had never been called, Stop acts like ForceStop.
	Stop()

	// ForceStop causes the application to exit immediately with an error
	// code.
	ForceStop()

	// WaitForStop takes in a channel on which a stop event will be
	// conveyed.  The stop event is represented by a string identifying the
	// source of the event.  For example, when Stop is called locally, the
	// LocalStop message will be received on the channel.  If the channel is
	// not being received on, or is full, no message is sent on it.
	WaitForStop(chan<- string)

	// Shutdown cleanly shuts down any internal state, logging, goroutines
	// etc spawned and managed by the runtime. It is useful for cases where
	// an application or library wants to be sure that it cleans up after
	// itself, and should typically be the last thing the program does.
	// Shutdown does not wait for any inflight requests to complete on
	// existing servers and clients in the runtime -- these need to be shut
	// down cleanly in advance if desired.
	Shutdown()
}

// The runtime must provide two package level functions, R and NewR.
// R returns the initialized global instance of the Runtime. NewR will
// create and initialiaze a new instance of the Runtime; it will typically
// be used from within unit tests.
//
// Their signatures are:
// <package>.R(opts ...NewROpt{}) (Runtime, error)
// <package>.NewR(opts ...NewROpt{}) (Runtime, error)
type ROpt interface {
	ROpt()
}
