/*
Package veyron2 defines the Runtime interface of the public Veyron API and its subdirectories define the entire Veyron public API.

Once we reach a '1.0' version these public APIs will be stable over
an extended period and changes to them will be carefully managed to ensure backward compatibility. The same solicy as used for go (http://golang.org/doc/go1compat) will be used for them.

The current release is 0.1 and although we will do our best to maintain
backwards compatibility we can't guarantee that until we reach the 1.0 milestone.

*/
package veyron2

import (
	"bytes"
	"fmt"
	"runtime"
	"sync"

	"v.io/core/veyron2/config"
	"v.io/core/veyron2/context"
	"v.io/core/veyron2/ipc"
	"v.io/core/veyron2/ipc/stream"
	"v.io/core/veyron2/naming"
	"v.io/core/veyron2/security"
	"v.io/core/veyron2/vlog"
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
	// NewContext creates a new root context.
	// This should be used when you are doing a new operation that isn't related
	// to ongoing RPCs.
	NewContext() *context.T

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

	// GetProfile gets the current profile.
	// TODO(suharshs, mattr): Again not completely sure yet if we actually need this.
	GetProfile(ctx *context.T) Profile

	// GetAppCycle gets the current AppCycle.
	GetAppCycle(ctx *context.T) AppCycle

	// GetListenSpec gets the ListenSpec.
	// TODO(suharshs, mattr): Determine if setting this should also be allowed, or if
	// only the Profiles should be able to this.
	GetListenSpec(ctx *context.T) ipc.ListenSpec

	// SetBackgroundContext creates a new context derived from the given context
	// with the given context set as the background context.
	SetBackgroundContext(ctx *context.T) *context.T

	// BackgroundContext retrieves a background context.  This context can
	// be used for general background activities.
	GetBackgroundContext(ctx *context.T) *context.T
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
	return initState.currentRuntime().NewEndpoint(ep)
}

// NewServer creates a new Server instance.
//
// It accepts at least the following options:
// ServesMountTable and ServerBlessings.
func NewServer(ctx *context.T, opts ...ipc.ServerOpt) (ipc.Server, error) {
	return initState.currentRuntime().NewServer(ctx, opts...)
}

// SetNewStreamManager creates a new stream manager and context
// with that StreamManager attached.
func SetNewStreamManager(ctx *context.T, opts ...stream.ManagerOpt) (*context.T, stream.Manager, error) {
	return initState.currentRuntime().SetNewStreamManager(ctx, opts...)
}

// GetStreamManager returns the current stream manager.
func GetStreamManager(ctx *context.T) stream.Manager {
	return initState.currentRuntime().GetStreamManager(ctx)
}

// SetPrincipal attaches a principal to the returned context.
func SetPrincipal(ctx *context.T, principal security.Principal) (*context.T, error) {
	return initState.currentRuntime().SetPrincipal(ctx, principal)
}

// GetPrincipal returns the current Principal.
func GetPrincipal(ctx *context.T) security.Principal {
	return initState.currentRuntime().GetPrincipal(ctx)
}

// SetNewClient creates a new Client instance and attaches it to a
// new context.
func SetNewClient(ctx *context.T, opts ...ipc.ClientOpt) (*context.T, ipc.Client, error) {
	return initState.currentRuntime().SetNewClient(ctx, opts...)
}

// GetClient returns the current Client.
func GetClient(ctx *context.T) ipc.Client {
	return initState.currentRuntime().GetClient(ctx)
}

// SetNewNamespace creates a new Namespace and attaches it to the
// returned context.
func SetNewNamespace(ctx *context.T, roots ...string) (*context.T, naming.Namespace, error) {
	return initState.currentRuntime().SetNewNamespace(ctx, roots...)
}

// GetNamespace returns the current namespace.
func GetNamespace(ctx *context.T) naming.Namespace {
	return initState.currentRuntime().GetNamespace(ctx)
}

// SetNewLogger creates a new Logger and attaches it to the
// returned context.
func SetNewLogger(ctx *context.T, name string, opts ...vlog.LoggingOpts) (*context.T, vlog.Logger, error) {
	return initState.currentRuntime().SetNewLogger(ctx, name, opts...)
}

// GetLogger returns the current logger.
func GetLogger(ctx *context.T) vlog.Logger {
	return initState.currentRuntime().GetLogger(ctx)
}

// GetProfile gets the current Profile.
func GetProfile(ctx *context.T) Profile {
	return initState.currentRuntime().GetProfile(ctx)
}

// GetAppCycle gets the current AppCycle.
func GetAppCycle(ctx *context.T) AppCycle {
	return initState.currentRuntime().GetAppCycle(ctx)
}

// GetListenSpec gets the current ListenSpec.
func GetListenSpec(ctx *context.T) ipc.ListenSpec {
	return initState.currentRuntime().GetListenSpec(ctx)
}

// SetBackgroundContext creates a new context derived from the given context
// with the given context set as the background context.
func SetBackgroundContext(ctx *context.T) *context.T {
	return initState.runtime.SetBackgroundContext(ctx)
}

// BackgroundContext retrieves a background context.  This context can
// be used for general background activities.
func GetBackgroundContext(ctx *context.T) *context.T {
	return initState.runtime.GetBackgroundContext(ctx)
}

var initState = &initStateData{}

type initStateData struct {
	mu           sync.RWMutex
	runtime      RuntimeX
	runtimeStack string
	profile      ProfileInitFunc
	profileStack string
}

func (i *initStateData) currentRuntime() RuntimeX {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if i.runtime == nil {
		panic(`Calling veyron2 method before initializing the runtime with Init().
You should call Init from your main or test function before calling
other veyron operations.`)
	}

	return i.runtime
}

// TODO(mattr, suharshs): Write thorough documentation for this type.
type ProfileInitFunc func(ctx *context.T) (RuntimeX, *context.T, Shutdown, error)

// TODO(mattr, suharshs): Write thorough documentation for this type.
func RegisterProfileInit(f ProfileInitFunc) {
	// Skip 3 frames: runtime.Callers, getStack, RegisterProfileInit.
	stack := getStack(3)
	initState.mu.Lock()
	defer initState.mu.Unlock()
	if initState.profile != nil {
		format := `A profile has already been registered.
This is most likely because a library package is
importing a profile.  Look for imports of the form
'v.io/core/veyron/profiles/...' and remove them.  Profiles should only be
imported in your main package.  Previous registration was from:
%s
Current registration is from:
%s
`
		panic(fmt.Sprintf(format, initState.profileStack, stack))
	}
	initState.profile = f
	initState.profileStack = stack
}

type Shutdown func()

func getStack(skip int) string {
	var buf bytes.Buffer
	stack := make([]uintptr, 16)
	stack = stack[:runtime.Callers(skip, stack)]
	for _, pc := range stack {
		fnc := runtime.FuncForPC(pc)
		file, line := fnc.FileLine(pc)
		fmt.Fprintf(&buf, "%s:%d: %s\n", file, line, fnc.Name())
	}
	return buf.String()
}

// Init should be called once for each vanadium executable, providing
// the setup of the initial context.T and a Shutdown function that can
// be used to clean up the runtime.  We allow calling Init multiple
// times (useful in tests), but only as long as you call the Shutdown
// returned previously before calling Init the second time.
func Init() (*context.T, Shutdown) {
	initState.mu.Lock()
	defer initState.mu.Unlock()

	// Skip 3 stack frames: runtime.Callers, getStack, Init
	stack := getStack(3)
	if initState.runtime != nil {
		format := `A runtime has already been initialized."
The previous initialization was from:
%s
This registration is from:
%s
`
		panic(fmt.Sprintf(format, initState.runtimeStack, stack))
	}

	ctx, cancel := context.WithCancel(nil)
	if initState.profile == nil {
		panic("No profile has been registered nor specified. This is most" +
			" likely because your main package has not imported a profile")
	}
	rt, ctx, shutdown, err := initState.profile(ctx)

	if err != nil {
		panic(err)
	}

	initState.runtime = rt
	initState.runtimeStack = stack

	return ctx, func() {
		// Note we call our own cancel here to ensure that the
		// runtime/profile implementor has not attached anything to a
		// non-cancellable context.
		cancel()
		shutdown()

		initState.mu.Lock()
		initState.runtime = nil
		initState.runtimeStack = ""
		initState.mu.Unlock()
	}
}
