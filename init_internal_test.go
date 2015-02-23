package veyron2

import (
	"strings"
	"testing"

	"v.io/core/veyron2/config"
	"v.io/core/veyron2/context"
	"v.io/core/veyron2/ipc"
	"v.io/core/veyron2/naming"
	"v.io/core/veyron2/security"
)

// Create a mock profile initialization function.
func MockInit(ctx *context.T) (Runtime, *context.T, Shutdown, error) {
	return &mockRuntime{}, nil, func() {}, nil
}

type mockRuntime struct{}

func (*mockRuntime) Init(ctx *context.T) error                      { return nil }
func (*mockRuntime) NewEndpoint(ep string) (naming.Endpoint, error) { return nil, nil }
func (*mockRuntime) NewServer(ctx *context.T, opts ...ipc.ServerOpt) (ipc.Server, error) {
	return nil, nil
}
func (*mockRuntime) SetNewStreamManager(ctx *context.T) (*context.T, error) {
	return nil, nil
}
func (*mockRuntime) SetPrincipal(ctx *context.T, principal security.Principal) (*context.T, error) {
	return nil, nil
}
func (*mockRuntime) GetPrincipal(ctx *context.T) security.Principal { return nil }
func (*mockRuntime) SetNewClient(ctx *context.T, opts ...ipc.ClientOpt) (*context.T, ipc.Client, error) {
	return nil, nil, nil
}
func (*mockRuntime) GetClient(ctx *context.T) ipc.Client { return nil }
func (*mockRuntime) SetNewNamespace(ctx *context.T, roots ...string) (*context.T, naming.Namespace, error) {
	return nil, nil, nil
}
func (*mockRuntime) GetNamespace(ctx *context.T) naming.Namespace { return nil }
func (*mockRuntime) SetReservedNameDispatcher(ctx *context.T, server ipc.Dispatcher, opts ...ipc.ServerOpt) *context.T {
	return nil
}
func (*mockRuntime) GetAppCycle(ctx *context.T) AppCycle            { return nil }
func (*mockRuntime) GetListenSpec(ctx *context.T) ipc.ListenSpec    { return ipc.ListenSpec{} }
func (*mockRuntime) GetPublisher(ctx *context.T) *config.Publisher  { return nil }
func (*mockRuntime) SetBackgroundContext(ctx *context.T) *context.T { return nil }
func (*mockRuntime) GetBackgroundContext(ctx *context.T) *context.T { return nil }

func TestPanicOnInitWithNoProfile(t *testing.T) {
	clear()
	// Calling Init without a registered profile should panic.
	catcher := func() {
		r := recover()
		if r == nil {
			t.Fatalf("recover returned nil")
		}
		str := r.(string)
		if !strings.Contains(str, "No profile has been registered") {
			t.Fatalf("unexpected error: %s", str)
		}
	}
	defer catcher()
	Init()
}

func TestInitWithRegisteredProfile(t *testing.T) {
	clear()
	// Calling veyron2.Init with a profile should succeed.
	RegisterProfileInit(MockInit)
	Init()
}

func TestPanicOnSecondProfileRegistration(t *testing.T) {
	clear()
	// Registering multiple profiles should fail.
	RegisterProfileInit(MockInit)
	catcher := func() {
		r := recover()
		if r == nil {
			t.Fatalf("recover returned nil")
		}
		str := r.(string)
		if !strings.Contains(str, "A profile has already been registered") {
			t.Fatalf("unexpected error: %s", str)
		}
	}
	defer catcher()
	RegisterProfileInit(MockInit)
}

func TestPanicOnSecondInit(t *testing.T) {
	clear()
	// Calling Init again before calling shutdown should fail.
	RegisterProfileInit(MockInit)
	Init()
	catcher := func() {
		r := recover()
		if r == nil {
			t.Fatalf("recover returned nil")
		}
		str := r.(string)
		if !strings.Contains(str, "A runtime has already been initialized") {
			t.Fatalf("unexpected error: %s", str)
		}
	}
	defer catcher()
	Init()
}

func TestSecondInitAfterShutdown(t *testing.T) {
	clear()
	// Calling Init, shutting down, and then calling Init again should succeed.
	RegisterProfileInit(MockInit)
	_, shutdown := Init()
	shutdown()
	Init()
}

// for tests only
func clear() {
	initState.mu.Lock()
	initState.runtime = nil
	initState.runtimeStack = ""
	initState.profile = nil
	initState.profileStack = ""
	initState.mu.Unlock()
}
