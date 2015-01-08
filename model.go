/*
Package veyron2 defines the Runtime interface of the public Veyron API and its subdirectories define the entire Veyron public API.

Once we reach a '1.0' version these public APIs will be stable over
an extended period and changes to them will be carefully managed to ensure backward compatibility. The same solicy as used for go (http://golang.org/doc/go1compat) will be used for them.

The current release is 0.1 and although we will do our best to maintain
backwards compatibility we can't guarantee that until we reach the 1.0 milestone.

*/
package veyron2

import (
	"fmt"
	"sync"

	"v.io/core/veyron2/config"
	"v.io/core/veyron2/context"
	"v.io/core/veyron2/ipc"
	"v.io/core/veyron2/ipc/stream"
	"v.io/core/veyron2/naming"
	"v.io/core/veyron2/security"
	"v.io/core/veyron2/vlog"
	"v.io/core/veyron2/vtrace"
)

const (
	// LocalStop is the message received on WaitForStop when the stop was
	// initiated by the process itself.
	LocalStop = "localstop"
	// RemoteStop is the message received on WaitForStop when the stop was
	// initiated via an RPC call (AppCycle.Stop).
	RemoteStop            = "remotestop"
	UnhandledStopExitCode = 1
	ForceStopExitCode     = 1
)

// Task is streamed to channels registered using TrackTask to provide a sense of
// the progress of the application's shutdown sequence.  For a description of
// the fields, see the Task struct in the veyron2/services/mgmt/appcycle
// package, which it mirrors.
type Task struct {
	Progress, Goal int32
}

// Platform describes the hardware and software environment that this
// process is running on. It is modeled on the Unix uname call.
type Platform struct {
	// Vendor is the manufacturer of the system. It must be possible
	// to crypographically verify the authenticity of the Vendor
	// using the value of Vendor. The test must fail if the underlying
	// hardware does not provide appropriate support for this.
	// TODO(ashankar, ataly): provide an API for verifying vendor authenticity.
	Vendor string

	// AIKCertificate attests that the platform contains a TPM that is trusted
	// and has valid EK (Endorsement Key) and platform credentials. The
	// AIKCertificate is bound to an AIK public key (attestation identity key)
	// that is secure on the TPM and can be used for signing operations.
	// TODO(gauthamt): provide an implementation of how a device gets this
	// certificate and how a remote process uses it to verify device identity.
	AIKCertificate *security.Certificate

	// Model is the model description, including version information.
	Model string

	// System is the name of the operating system.
	// E.g. 'Linux', 'Darwin'
	System string

	// Release is the specific release of System if known, the empty
	// string otherwise.
	// E.g. 3.12.24-1-ARCH on a Raspberry pi running Arch Linux
	Release string

	// Version is the version of System if known, the empty string otherwise.
	// E.g. #1 PREEMPT Thu Jul 10 23:57:15 MDT 2014 on a Raspberry Pi B
	// running Arch Linux
	Version string

	// Machine is the hardware identifier
	// E.g. armv6l on a Raspberry Pi B
	Machine string

	// Node is the name of the name of the node that the
	// the platform is running on. This is not necessarily unique.
	Node string
}

func (p *Platform) String() string {
	return fmt.Sprintf("%s/%s node %s running %s (%s %s) on machine %s", p.Vendor, p.Model, p.Node, p.System, p.Release, p.Version, p.Machine)
}

// A Profile represents the combination of hardware, operating system,
// compiler and libraries available to the application. The Profile
// interface is the runtime representation of the Profiles used in the
// Veyron Management and Build System to manage and build applications.
// The Profile interface is thus intended to abstract out the hardware,
// operating system and library specific implementation so that they
// can be integrated with the rest of the Veyron runtime. The implementations
// of the Profile interface are intended to capture all of the dependencies
// implied by that Profile. For example, if a Profile requires a particular
// hardware specific library (say Bluetooth support), then the implementation
// of the Profile should include that dependency; the package implementing
// the Profile may should expose the additional APIs needed to use the
// functionality.
//
// The Profile interface provides a description of the Profile and the
// platform it is running on, an initialization hook into the rest of
// the Veyron runtime and a means of sharing configuration information with
// the rest of the application as follows:
//
// - the Name methods returns the name of the Profile used to build the binary
// - the Platform method returns a description of the platform the binary is
//   running on.
// - the Init method is called by the Runtime after it has fully
//   initialized itself. Henceforth, the Profile implementation can safely
//   use the supplied Runtime. This a convenient hook for initialization.
// - configuration information can be provided to the rest of the
//   application via the config.Publisher passed to the Init method.
//   This allows a common set of configuration information to be made
//   available to all applications without burdening them with understanding
//   the implementations of how to obtain that information (e.g. dhcp,
//   network configuration differs).
//
// Profiles range from the generic to the very specific (e.g. "linux" or
// "my-sprinkler-controller-v2". Applications should, in general, use
// as generic a Profile as possbile.
// Profiles are registered using veyron2/rt.RegisterProfile and subsequent
// registrations override prior ones. Profile implementations will generally
// call RegisterProfile in their init functions and hence importing a
// profile will be sufficient to register it. Applications are also free
// to explicitly register profile directly by calling RegisterProfile themselves
// prior to calling rt.Init.
//
// This scheme allows applications to use a pre-supplied Profile as well
// as for developers to create their own Profiles (to represent their
// hardware and software system). The Veyron Build System, once fully
// developed will likely insert generated that uses one of the above schemes
// to configure Profiles.
//
// See the veyron/profiles package for a complete description of the
// precanned Profiles and how to use them; see veyron2/rt for the registration
// function used to register new Profiles.
type Profile interface {
	// Name returns the name of the Profile used to build this binary.
	Name() string

	// Runtime returns the name of the Runtime that this Profile requires,
	// and options to be passed to the Runtime factory.
	Runtime() (string, []ROpt)

	// Description returns the Description of the Platform.
	Platform() *Platform

	// Init is called by the Runtime once it has been initialized and
	// command line flags have been parsed. It returns an instance of
	// the AppCycle interface to be used for managing this runtime.
	// Init will be called once and only once by the Runtime.
	// TODO(suharshs, mattr): Once we switch to the new profile init func, we can remove this.
	Init(rt Runtime, p *config.Publisher) (AppCycle, error)

	String() string

	// Cleanup shuts down any internal state maintained by the Profile.
	// It will be called by the Runtime from its Cleanup method.
	// TODO(suharshs, mattr): Once we switch to the new profile init func, we can remove this.
	Cleanup()
}

// AppCycle is the interface for managing the shutdown of a runtime
// remotely and locally. An appropriate instance of this is provided by
// the Profile to the runtime implementation which in turn arranges to
// serve it on an appropriate network address.
type AppCycle interface {
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
	//
	// The channel is assumed to remain open while messages could be sent on
	// it.  The channel will be automatically closed during the call to
	// Cleanup.
	WaitForStop(chan<- string)

	// AdvanceGoal extends the goal value in the shutdown task tracker.
	// Non-positive delta is ignored.
	AdvanceGoal(delta int32)
	// AdvanceProgress advances the progress value in the shutdown task
	// tracker.  Non-positive delta is ignored.
	AdvanceProgress(delta int32)
	// TrackTask registers a channel to receive task updates (a Task will be
	// sent on the channel if either the goal or progress values of the
	// task have changed).  If the channel is not being received on, or is
	// full, no Task is sent on it.
	//
	// The channel is assumed to remain open while Tasks could be sent on
	// it.
	TrackTask(chan<- Task)

	// Remote returns an object to serve the remotely accessible AppCycle
	// interface (as defined in veyron2/services/mgmt/appcycle)
	Remote() interface{}
}

// Runtime is the interface that concrete Veyron implementations must
// implement.
type Runtime interface {
	// Profile returns the current processes' Profile.
	Profile() Profile

	// Publisher returns a configuration Publisher that
	// can be used to access configuration information.
	Publisher() *config.Publisher

	// Principal returns the principal that represents this runtime.
	Principal() security.Principal

	// NewClient creates a new Client instance.
	//
	// It accepts at least the following options:
	// StreamManager, and Namespace
	//
	// In particular, if the options include a Client, then NewClient
	// just returns that.
	NewClient(opts ...ipc.ClientOpt) (ipc.Client, error)

	// NewServer creates a new Server instance.
	//
	// It accepts at least the following options:
	// ServesMountTable, ServerBlessings, and Namespace
	NewServer(opts ...ipc.ServerOpt) (ipc.Server, error)

	// Client returns the pre-configured Client that is created when the
	// Runtime is initialized.
	Client() ipc.Client

	// NewContext creates a new root context.
	// This should be used when you are doing a new operation that isn't related
	// to ongoing RPCs.
	NewContext() *context.T

	// WithNewSpan derives a context with a new Span that can be used to
	// trace and annotate operations across process boundaries.
	WithNewSpan(ctx *context.T, name string) (*context.T, vtrace.Span)

	// SpanFromContext finds the currently active span.
	SpanFromContext(ctx *context.T) vtrace.Span

	// NewStreamManager creates a new stream manager.  The returned stream
	// manager will be shutdown by the runtime on Cleanup.
	NewStreamManager(opts ...stream.ManagerOpt) (stream.Manager, error)

	// NewEndpoint returns an Endpoint by parsing the supplied endpoint
	// string as per the format described above. It can be used to test
	// a string to see if it's in valid endpoint format.
	//
	// NewEndpoint will accept strings both in the @ format described
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

	// Namespace returns the pre-configured Namespace that is created
	// when the Runtime is initialized.
	Namespace() naming.Namespace

	// Logger returns the current logger in use by the Runtime.
	Logger() vlog.Logger

	// NewLogger creates a new instance of the logging interface that is
	// separate from the one provided by Runtime.
	NewLogger(name string, opts ...vlog.LoggingOpts) (vlog.Logger, error)

	// Cleanup cleanly shuts down any internal state, logging, goroutines
	// etc spawned and managed by the runtime. It is useful for cases where
	// an application or library wants to be sure that it cleans up after
	// itself, and should typically be the last thing the program does.
	// Cleanup does not wait for any inflight requests to complete on
	// existing servers and clients in the runtime -- these need to be shut
	// down cleanly in advance if desired.  It does, however, drain the
	// network connections.
	Cleanup()

	// ConfigureReservedName sets and configures a dispatcher for the
	// reserved portion of the name space, i.e. any path starting with '__'
	// Only one dispatcher may be registered, with subsequent calls
	// overriding previous settings.
	ConfigureReservedName(server ipc.Dispatcher, opts ...ipc.ServerOpt)

	// VtraceStore gets the vtrace store for this runtime.
	VtraceStore() vtrace.Store
}

// RuntimeFromContext returns the runtime used to generate a given context.
// The result will always be a non-nil Runtime instance.
func RuntimeFromContext(ctx *context.T) Runtime {
	return ctx.Runtime().(Runtime)
}

// The name for the google runtime implementation
const GoogleRuntimeName = "google"

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

// RuntimeX is the interface that concrete Veyron implementations must
// implement.  It will not be used directly by application builders.
// They will instead use the package level functions that mirror these
// factories.
// TODO(mattr): Some opts might not make sense now.  For example, the
// namespace is currently a ServerOpt, but it probably makes more sense
// to just use the current namespace in the context that is passed
// to NewServer.  The same for Profile and StreamManager.
type RuntimeX interface {
	// Init initilizes the runtime.  It will be called only once.  It
	// must be called before any other Runtime methods are called.  Its
	// primary responsability is to populate the initial context with
	// all required state.
	Init(ctx *context.T) *context.T

	// NewEndpoint returns an Endpoint by parsing the supplied endpoint
	// string as per the format described above. It can be used to test
	// a string to see if it's in valid endpoint format.
	//
	// NewEndpoint will accept strings both in the @ format described
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

	// NewServer creates a new Server instance.
	//
	// It accepts at least the following options:
	// ServesMountTable and ServerBlessings.
	NewServer(ctx *context.T, opts ...ipc.ServerOpt) (ipc.Server, error)

	// SetNewStreamManager creates a new stream manager and context
	// with that StreamManager attached.
	SetNewStreamManager(ctx *context.T, opts ...stream.ManagerOpt) (*context.T, stream.Manager, error)

	// StreamManager returns the current stream manager.
	GetStreamManager(ctx *context.T) stream.Manager

	// SetPrincipal attaches a principal to the returned context.
	SetPrincipal(ctx *context.T, principal security.Principal) (*context.T, error)

	// GetPrincipal returns the current Principal.
	GetPrincipal(ctx *context.T) security.Principal

	// SetNewClient creates a new Client instance and attaches it to a
	// new context.
	SetNewClient(ctx *context.T, opts ...ipc.ClientOpt) (*context.T, ipc.Client, error)

	// GetClient returns the current Client.
	GetClient(ctx *context.T) ipc.Client

	// SetNewSpan derives a context with a new Span that can be used to
	// trace and annotate operations across process boundaries.
	SetNewSpan(ctx *context.T, name string) (*context.T, vtrace.Span)

	// GetSpan finds the currently active span.
	GetSpan(ctx *context.T) vtrace.Span

	// SetNewNamespace creates a new Namespace and attaches it to the
	// returned context.
	SetNewNamespace(ctx *context.T, roots ...string) (*context.T, naming.Namespace, error)

	// GetNamespace returns the current namespace
	GetNamespace(ctx *context.T) naming.Namespace

	// SetNewLogger creates a new Logger and attaches it to the
	// returned context.
	SetNewLogger(ctx *context.T, name string, opts ...vlog.LoggingOpts) (*context.T, vlog.Logger, error)

	// GetLogger returns the current logger.
	GetLogger(ctx *context.T) vlog.Logger

	// GetVtraceStore returns the current vtrace.Store.
	GetVtraceStore(ctx *context.T) vtrace.Store

	// SetReservedNameDispatcher sets and configures a dispatcher for the
	// reserved portion of the name space, i.e. any path starting with '__'.
	SetReservedNameDispatcher(ctx *context.T, server ipc.Dispatcher, opts ...ipc.ServerOpt) *context.T

	// GetProfile gets the current profile.
	// TODO(suharshs, mattr): Again not completely sure yet if we actually need this.
	GetProfile(ctx *context.T) Profile

	// GetAppCycle gets the current AppCycle.
	GetAppCycle(ctx *context.T) AppCycle

	// GetListenSpec gets the ListenSpec.
	// TODO(suharshs, mattr): Determine if setting this should also be allowed, or if
	// only the Profiles should be able to this.
	GetListenSpec(ctx *context.T) ipc.ListenSpec
}

var (
	runtimeConfig struct {
		sync.Mutex
		runtime RuntimeX
	}
)

// TODO(mattr): this is a short term hack to put us into a situation
// where we'll have backward compatibility during the transition from
// Runtime to RuntimeX.  This will be replaced by a real registration
// mechanism as in the current rt package.
// For now, this simply must be called before main begins (in an init).
func RegisterRuntime(name string, r RuntimeX) {
	runtimeConfig.Lock()
	if runtimeConfig.runtime == nil {
		runtimeConfig.runtime = r
	}
	runtimeConfig.Unlock()
}

// NewEndpoint returns an Endpoint by parsing the supplied endpoint
// string as per the format described above. It can be used to test
// a string to see if it's in valid endpoint format.
//
// NewEndpoint will accept strings both in the @ format described
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
func NewEndpoint(ep string) (naming.Endpoint, error) {
	return runtimeConfig.runtime.NewEndpoint(ep)
}

// NewServer creates a new Server instance.
//
// It accepts at least the following options:
// ServesMountTable and ServerBlessings.
func NewServer(ctx *context.T, opts ...ipc.ServerOpt) (ipc.Server, error) {
	return runtimeConfig.runtime.NewServer(ctx, opts...)
}

// SetNewStreamManager creates a new stream manager and context
// with that StreamManager attached.
func SetNewStreamManager(ctx *context.T, opts ...stream.ManagerOpt) (*context.T, stream.Manager, error) {
	return runtimeConfig.runtime.SetNewStreamManager(ctx, opts...)
}

// GetStreamManager returns the current stream manager.
func GetStreamManager(ctx *context.T) stream.Manager {
	return runtimeConfig.runtime.GetStreamManager(ctx)
}

// SetPrincipal attaches a principal to the returned context.
func SetPrincipal(ctx *context.T, principal security.Principal) (*context.T, error) {
	return runtimeConfig.runtime.SetPrincipal(ctx, principal)
}

// GetPrincipal returns the current Principal.
func GetPrincipal(ctx *context.T) security.Principal {
	return runtimeConfig.runtime.GetPrincipal(ctx)
}

// SetNewClient creates a new Client instance and attaches it to a
// new context.
func SetNewClient(ctx *context.T, opts ...ipc.ClientOpt) (*context.T, ipc.Client, error) {
	return runtimeConfig.runtime.SetNewClient(ctx, opts...)
}

// GetClient returns the current Client.
func GetClient(ctx *context.T) ipc.Client {
	return runtimeConfig.runtime.GetClient(ctx)
}

// SetNewSpan derives a context with a new Span that can be used to
// trace and annotate operations across process boundaries.
func SetNewSpan(ctx *context.T, name string) (*context.T, vtrace.Span) {
	return runtimeConfig.runtime.SetNewSpan(ctx, name)
}

// GetSpan finds the currently active span.
func GetSpan(ctx *context.T) vtrace.Span {
	return runtimeConfig.runtime.GetSpan(ctx)
}

// SetNewNamespace creates a new Namespace and attaches it to the
// returned context.
func SetNewNamespace(ctx *context.T, roots ...string) (*context.T, naming.Namespace, error) {
	return runtimeConfig.runtime.SetNewNamespace(ctx, roots...)
}

// GetNamespace returns the current namespace.
func GetNamespace(ctx *context.T) naming.Namespace {
	return runtimeConfig.runtime.GetNamespace(ctx)
}

// SetNewLogger creates a new Logger and attaches it to the
// returned context.
func SetNewLogger(ctx *context.T, name string, opts ...vlog.LoggingOpts) (*context.T, vlog.Logger, error) {
	return runtimeConfig.runtime.SetNewLogger(ctx, name, opts...)
}

// GetLogger returns the current logger.
func GetLogger(ctx *context.T) vlog.Logger {
	return runtimeConfig.runtime.GetLogger(ctx)
}

// GetVtraceStore gets the current vtrace.Store.
func GetVtraceStore(ctx *context.T) vtrace.Store {
	return runtimeConfig.runtime.GetVtraceStore(ctx)
}

// GetProfile gets the current Profile.
func GetProfile(ctx *context.T) Profile {
	return runtimeConfig.runtime.GetProfile(ctx)
}

// GetAppCycle gets the current AppCycle.
func GetAppCycle(ctx *context.T) AppCycle {
	return runtimeConfig.runtime.GetAppCycle(ctx)
}

// GetListenSpec gets the current ListenSpec.
func GetListenSpec(ctx *context.T) ipc.ListenSpec {
	return runtimeConfig.runtime.GetListenSpec(ctx)
}

var (
	profileInitFunc ProfileInitFunc
	profileInitMu   sync.Mutex
)

type ProfileInitFunc func(ctx *context.T) (RuntimeX, *context.T)

func RegisterProfileInit(f ProfileInitFunc) {
	profileInitMu.Lock()
	if profileInitFunc != nil {
		panic("Profile intialization functions can only be registered one time.")
	}
	profileInitFunc = f
	profileInitMu.Unlock()
}
