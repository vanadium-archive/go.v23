// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

// To update v23_test.go:
// jiri test generate v.io/v23/syncbase/featuretests

// To run just the VClock tests:
// jiri go test v.io/v23/syncbase/featuretests --v23.tests -test.run=TestV23VClock

// TODO(sadovsky): Add a way to stop vclockd, or to instruct Syncbase not to
// start vclockd in the first place. Otherwise, some of these tests can have
// racy interactions with vclockd.

// TODO(sadovsky): Implement the following tests (from the test plan).
//
// Single-machine
// - Clock moves forward (barring system clock events and NTP sync events)
// - System clock events affect virtual clock state
// - System clock is checked at the expected frequency
// - NTP sync affects virtual clock state (e.g. clock can move backward)
// - NTP sync occurs at the expected frequency
//
// Multi-machine (sync)
// - Peer is 0, 1, or 2 hops away from NTP-synced device, local is not
//   NTP-synced
// - Same, but local is NTP-synced (with varying times since sync, and varying
//   number of hops away from NTP-synced device)
// - Same, but peer has experienced 0, 1, or 2 reboots since obtaining its
//   NTP-synced time

import (
	"time"

	"v.io/v23"
	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	_ "v.io/x/ref/runtime/factories/generic"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test/v23tests"
)

//go:generate jiri test generate

const (
	openPerms = `{"Read": {"In":["..."]}, "Write": {"In":["..."]}, "Resolve": {"In":["..."]}, "Admin": {"In":["..."]}}`
)

var (
	jan2015 = time.Date(2015, time.January, 1, 0, 0, 0, 0, time.UTC)
)

func testInit(t *v23tests.T) *context.T {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	return tu.NewCtx(t.Context(), v23.GetPrincipal(t.Context()), "u:client")
}

////////////////////////////////////////
// Tests for local vclock updates

func V23TestVClockMovesForward(t *v23tests.T) {
	ctx := testInit(t)
	s0Creds, _ := t.Shell().NewChildCredentials("r:s0")
	cleanup := tu.StartSyncbased(t, s0Creds, "s0", "", openPerms)
	defer cleanup()

	sc := wire.ServiceClient("s0")

	t0, err := sc.DevModeGetTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t1, err := sc.DevModeGetTime(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !t0.Before(t1) {
		t.Fatalf("expected t0 < t1: %v, %v", t0, t1)
	}
}

func V23TestVClockSystemClock(t *v23tests.T) {
	ctx := testInit(t)
	s0Creds, _ := t.Shell().NewChildCredentials("s0")
	cleanup := tu.StartSyncbased(t, s0Creds, "s0", "", openPerms)
	defer cleanup()

	sc := wire.ServiceClient("s0")

	// Initialize system time.
	if err := sc.DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		Now:           jan2015,
		ElapsedTime:   0,
		DoLocalUpdate: true,
	}); err != nil {
		t.Fatal(err)
	}

	t0, err := sc.DevModeGetTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !isBarelyAfter(t0, jan2015) {
		t.Fatalf("unexpected time: %v", t0)
	}

	// Move time and elapsed time forward by equal amounts. Syncbase should not
	// detect any anomalies, and should reflect the new time.
	if err := sc.DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		Now:           jan2015.Add(time.Hour),
		ElapsedTime:   time.Hour,
		DoLocalUpdate: true,
	}); err != nil {
		t.Fatal(err)
	}

	t0, err = sc.DevModeGetTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !isBarelyAfter(t0, jan2015.Add(time.Hour)) {
		t.Fatalf("unexpected time: %v", t0)
	}

	// Move time backward, and reset elapsed time to 0. Syncbase should detect a
	// reboot, and should reflect the new time.
	if err := sc.DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		Now:           jan2015.Add(-time.Hour),
		ElapsedTime:   0,
		DoLocalUpdate: true,
	}); err != nil {
		t.Fatal(err)
	}

	t0, err = sc.DevModeGetTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !isBarelyAfter(t0, jan2015.Add(-time.Hour)) {
		t.Fatalf("unexpected time: %v", t0)
	}

	// Move time forward 5 hours and set elapsed time to 1 hour. Syncbase should
	// detect this as a system clock change (since the system clock time is not
	// where it's expected to be) and adjust skew so that the new reported time is
	// equal to the previous reported time plus the change in elapsed time.
	if err := sc.DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		Now:           jan2015.Add(5 * time.Hour),
		ElapsedTime:   time.Hour,
		DoLocalUpdate: true,
	}); err != nil {
		t.Fatal(err)
	}

	t0, err = sc.DevModeGetTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !isBarelyAfter(t0, jan2015) {
		t.Fatalf("unexpected time: %v", t0)
	}
}

func V23TestVClockSystemClockFrequency(t *v23tests.T) {
	//clientCtx := testInit(t)
	server0Creds, _ := t.Shell().NewChildCredentials("s0")
	cleanup := tu.StartSyncbased(t, server0Creds, "s0", "", openPerms)
	defer cleanup()
}

func V23TestVClockNtp(t *v23tests.T) {
	//clientCtx := testInit(t)
	server0Creds, _ := t.Shell().NewChildCredentials("s0")
	cleanup := tu.StartSyncbased(t, server0Creds, "s0", "", openPerms)
	defer cleanup()
}

func V23TestVClockNtpFrequency(t *v23tests.T) {
	//clientCtx := testInit(t)
	server0Creds, _ := t.Shell().NewChildCredentials("s0")
	cleanup := tu.StartSyncbased(t, server0Creds, "s0", "", openPerms)
	defer cleanup()
}

////////////////////////////////////////
// Tests for p2p vclock updates

////////////////////////////////////////////////////////////////////////////////
// Helper functions

// isBarelyAfter returns true if t is after target but before target + 10s.
func isBarelyAfter(t time.Time, target time.Time) bool {
	return t.After(target) && t.Before(target.Add(10*time.Second))
}
