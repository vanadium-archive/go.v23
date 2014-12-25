package rt_test

import (
	"testing"

	"v.io/veyron/veyron2"
	"v.io/veyron/veyron2/config"
	"v.io/veyron/veyron2/context"
	"v.io/veyron/veyron2/ipc"
	"v.io/veyron/veyron2/ipc/stream"
	"v.io/veyron/veyron2/naming"
	"v.io/veyron/veyron2/options"
	"v.io/veyron/veyron2/rt"
	"v.io/veyron/veyron2/security"
	"v.io/veyron/veyron2/vlog"
	"v.io/veyron/veyron2/vtrace"
)

func ExampleInit() {
	r, err := rt.New()
	if err != nil {
		panic(err)
	}

	// Go ahead and use the runtime.
	log := r.Logger()
	log.Infof("hello world")
}

type myprofile struct {
	called int
}

func (mp *myprofile) Name() string {
	return "test"
}

func (mp *myprofile) Runtime() (string, []veyron2.ROpt) {
	return "mock", nil
}

func (mp *myprofile) Platform() *veyron2.Platform {
	return &veyron2.Platform{"google", nil, "v1", "any", "rel1", ".2", "who knows", "this host"}
}

func (mp *myprofile) String() string {
	return "myprofile on " + mp.Platform().String()
}

func (mp *myprofile) Init(veyron2.Runtime, *config.Publisher) (veyron2.AppCycle, error) {
	mp.called++
	return nil, nil
}

func (mp *myprofile) Cleanup() {}

func ExampleInitWithProfile() {
	r, err := rt.New(options.Profile{&myprofile{}})
	if err != nil {
		panic(err)
	}

	// Go ahead and use the runtime.
	log := r.Logger()
	log.Infof("hello world from my product: %s", r.Profile())
}

// TODO(cnicolaou): add tests to:
//  - catch mismatched profile and runtimes - e.g. profile asks for "foo"
// runtime, but only bar is available.
//  - tests to catch multiple calls to init with different options

func TestRTNew(t *testing.T) {
	profile := &myprofile{}
	rt.RegisterProfile(profile)
	runtime := &mockRuntime{}
	factory := func(opts ...veyron2.ROpt) (veyron2.Runtime, error) {
		for _, o := range opts {
			if profile, ok := o.(veyron2.Profile); ok {
				profile.Init(runtime, nil)
			}
		}
		return runtime, nil
	}
	rt.RegisterRuntime("mock", factory)
	rt.New()
	rt.New()
	if got, want := profile.called, 2; got != want {
		t.Errorf("profile called %d times, want %d", got, want)
	}
}

type mockRuntime struct{}

func (*mockRuntime) Profile() veyron2.Profile   { return nil }
func (*mockRuntime) AppCycle() veyron2.AppCycle { return nil }

func (*mockRuntime) Publisher() *config.Publisher                           { return nil }
func (*mockRuntime) Principal() security.Principal                          { return nil }
func (*mockRuntime) NewClient(opts ...ipc.ClientOpt) (ipc.Client, error)    { return nil, nil }
func (*mockRuntime) NewServer(opts ...ipc.ServerOpt) (ipc.Server, error)    { return nil, nil }
func (*mockRuntime) Client() ipc.Client                                     { return nil }
func (*mockRuntime) NewContext() context.T                                  { return nil }
func (*mockRuntime) WithNewSpan(context.T, string) (context.T, vtrace.Span) { return nil, nil }
func (*mockRuntime) SpanFromContext(context.T) vtrace.Span                  { return nil }
func (*mockRuntime) NewStreamManager(opts ...stream.ManagerOpt) (stream.Manager, error) {
	return nil, nil
}
func (*mockRuntime) NewEndpoint(ep string) (naming.Endpoint, error) { return nil, nil }
func (*mockRuntime) Namespace() naming.Namespace                    { return nil }
func (*mockRuntime) Logger() vlog.Logger                            { return nil }
func (*mockRuntime) NewLogger(name string, opts ...vlog.LoggingOpts) (vlog.Logger, error) {
	return nil, nil
}
func (*mockRuntime) ConfigureReservedName(ipc.Dispatcher, ...ipc.ServerOpt) {}
func (*mockRuntime) VtraceStore() vtrace.Store                              { return nil }
func (*mockRuntime) Cleanup()                                               {}
