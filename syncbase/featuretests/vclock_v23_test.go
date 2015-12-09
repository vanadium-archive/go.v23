// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"time"

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

func testInit(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
}

////////////////////////////////////////
// Tests for local vclock updates

// Tests that the virtual clock moves forward.
func V23TestVClockMovesForward(t *v23tests.T) {
	testInit(t)
	ctx := forkContext(t, "c0")
	s0Creds := forkCredentials(t, "s0")
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

// Tests that system clock updates affect the virtual clock.
func V23TestVClockSystemClockUpdate(t *v23tests.T) {
	testInit(t)
	ctx := forkContext(t, "c0")
	s0Creds := forkCredentials(t, "s0")
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

// Tests that the virtual clock daemon checks for system clock updates at the
// expected frequency (loosely speaking).
func V23TestVClockSystemClockFrequency(t *v23tests.T) {
	testInit(t)
	ctx := forkContext(t, "c0")
	s0Creds := forkCredentials(t, "s0")
	cleanup := tu.StartSyncbased(t, s0Creds, "s0", "", openPerms)
	defer cleanup()

	sc := wire.ServiceClient("s0")

	t0, err := sc.DevModeGetTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if t0.Sub(jan2015) < time.Hour {
		t.Fatalf("unexpected time: %v", t0)
	}

	// Set system time but do not force a local update.
	if err := sc.DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		Now:           jan2015,
		ElapsedTime:   0,
		DoLocalUpdate: false,
	}); err != nil {
		t.Fatal(err)
	}

	// Sleep for two seconds. Note, VClockD checks for sysclock changes once per
	// second.
	time.Sleep(2 * time.Second)

	// t1 should reflect the sysclock change.
	t1, err := sc.DevModeGetTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !isBarelyAfter(t1, jan2015) {
		t.Fatalf("unexpected time: %v", t1)
	}
}

// Tests that NTP sync affects virtual clock state (e.g. clock can move
// backward).
func V23TestVClockNtpUpdate(t *v23tests.T) {
	testInit(t)
	ctx := forkContext(t, "c0")
	s0Creds := forkCredentials(t, "s0")
	cleanup := tu.StartSyncbased(t, s0Creds, "s0", "", openPerms)
	defer cleanup()

	sc := wire.ServiceClient("s0")

	t0, err := sc.DevModeGetTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if t0.Sub(jan2015) < time.Hour {
		t.Fatalf("unexpected time: %v", t0)
	}

	// Use NTP to set the clock to jan2015.
	if err := sc.DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		NtpHost:     startFakeNtpServer(t, jan2015),
		DoNtpUpdate: true,
	}); err != nil {
		t.Fatal(err)
	}

	t1, err := sc.DevModeGetTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !isBarelyAfter(t1, jan2015) {
		t.Fatalf("unexpected time: %v", t1)
	}

	// Use NTP to move the clock forward by 5 hours.
	if err := sc.DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		NtpHost:     startFakeNtpServer(t, jan2015.Add(5*time.Hour)),
		DoNtpUpdate: true,
	}); err != nil {
		t.Fatal(err)
	}

	t2, err := sc.DevModeGetTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !isBarelyAfter(t2, jan2015.Add(5*time.Hour)) {
		t.Fatalf("unexpected time: %v", t2)
	}
}

// Tests that the virtual clock daemon checks in with NTP at the expected
// frequency (loosely speaking).
func V23TestVClockNtpFrequency(t *v23tests.T) {
	testInit(t)
	ctx := forkContext(t, "c0")
	s0Creds := forkCredentials(t, "s0")
	cleanup := tu.StartSyncbased(t, s0Creds, "s0", "", openPerms)
	defer cleanup()

	sc := wire.ServiceClient("s0")

	t0, err := sc.DevModeGetTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if t0.Sub(jan2015) < time.Hour {
		t.Fatalf("unexpected time: %v", t0)
	}

	// Use NTP to set the clock to jan2015.
	if err := sc.DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		NtpHost:     startFakeNtpServer(t, jan2015),
		DoNtpUpdate: true,
	}); err != nil {
		t.Fatal(err)
	}

	t1, err := sc.DevModeGetTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !isBarelyAfter(t1, jan2015) {
		t.Fatalf("unexpected time: %v", t1)
	}

	// Use NTP to move the clock forward by 5 hours, but do not run
	// vclockd.DoNtpUpdate(). Since NTP sync happens only once per hour, the
	// virtual clock should continue reporting the old time, even after we sleep
	// for several seconds.
	if err := sc.DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		NtpHost:     startFakeNtpServer(t, jan2015.Add(5*time.Hour)),
		DoNtpUpdate: false,
	}); err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Second)
	t2, err := sc.DevModeGetTime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !isBarelyAfter(t2, jan2015.Add(5*time.Second)) {
		t.Fatalf("unexpected time: %v", t2)
	}
}

////////////////////////////////////////
// Tests for p2p vclock updates

// TODO(sadovsky): Implement the following tests (from the test plan):
// - Peer is 0, 1, or 2 hops away from NTP-synced device, local is not
//   NTP-synced
// - Same, but local is NTP-synced (with varying times since sync, and varying
//   number of hops away from NTP-synced device)
// - Same, but peer has experienced 0, 1, or 2 reboots since obtaining its
//   NTP-synced time

////////////////////////////////////////////////////////////////////////////////
// Helper functions

func startFakeNtpServer(t *v23tests.T, now time.Time) string {
	nowBuf, err := now.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	ntpd := t.BuildV23Pkg("v.io/x/ref/services/syncbase/testutil/fake_ntp_server")
	invocation := ntpd.WithStartOpts(ntpd.StartOpts().NoExecProgram().WithSessions(t, time.Second)).Start("--now=" + string(nowBuf))
	host := invocation.ExpectVar("HOST")
	if host == "" {
		t.Fatalf("fake_ntp_server failed to start")
	}
	return host
}

// isBarelyAfter returns true if t is after target but before target + 10s.
func isBarelyAfter(t time.Time, target time.Time) bool {
	return t.After(target) && t.Before(target.Add(10*time.Second))
}
