// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v23

import (
	"strings"
	"testing"

	"v.io/x/lib/pubsub"

	"v.io/v23/context"
	"v.io/v23/namespace"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
)

// Create a mock profile initialization function.
func MockInit(ctx *context.T) (Runtime, *context.T, Shutdown, error) {
	return &mockRuntime{}, nil, func() {}, nil
}

type mockRuntime struct{}

func (*mockRuntime) Init(ctx *context.T) error                      { return nil }
func (*mockRuntime) NewEndpoint(ep string) (naming.Endpoint, error) { return nil, nil }
func (*mockRuntime) NewServer(ctx *context.T, opts ...rpc.ServerOpt) (rpc.Server, error) {
	return nil, nil
}
func (*mockRuntime) WithNewStreamManager(ctx *context.T) (*context.T, error) {
	return nil, nil
}
func (*mockRuntime) WithPrincipal(ctx *context.T, principal security.Principal) (*context.T, error) {
	return nil, nil
}
func (*mockRuntime) GetPrincipal(ctx *context.T) security.Principal { return nil }
func (*mockRuntime) WithNewClient(ctx *context.T, opts ...rpc.ClientOpt) (*context.T, rpc.Client, error) {
	return nil, nil, nil
}
func (*mockRuntime) GetClient(ctx *context.T) rpc.Client { return nil }
func (*mockRuntime) WithNewNamespace(ctx *context.T, roots ...string) (*context.T, namespace.T, error) {
	return nil, nil, nil
}
func (*mockRuntime) GetNamespace(ctx *context.T) namespace.T         { return nil }
func (*mockRuntime) GetAppCycle(ctx *context.T) AppCycle             { return nil }
func (*mockRuntime) GetListenSpec(ctx *context.T) rpc.ListenSpec     { return rpc.ListenSpec{} }
func (*mockRuntime) GetPublisher(ctx *context.T) *pubsub.Publisher   { return nil }
func (*mockRuntime) WithBackgroundContext(ctx *context.T) *context.T { return nil }
func (*mockRuntime) GetBackgroundContext(ctx *context.T) *context.T  { return nil }

func (*mockRuntime) WithReservedNameDispatcher(ctx *context.T, d rpc.Dispatcher) *context.T {
	return nil
}

func (*mockRuntime) GetReservedNameDispatcher(ctx *context.T) rpc.Dispatcher { return nil }

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
	// Calling v23.Init with a profile should succeed.
	RegisterProfile(MockInit)
	Init()
}

func TestPanicOnSecondProfileRegistration(t *testing.T) {
	clear()
	// Registering multiple profiles should fail.
	RegisterProfile(MockInit)
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
	RegisterProfile(MockInit)
}

func TestPanicOnSecondInit(t *testing.T) {
	clear()
	// Calling Init again before calling shutdown should fail.
	RegisterProfile(MockInit)
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
	RegisterProfile(MockInit)
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
