// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"time"

	"v.io/v23/context"
	"v.io/v23/naming"
	wire "v.io/v23/services/syncbase"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/syncbase/server/util"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test/v23tests"
)

//go:generate jiri test generate

const (
	openPerms = `{"Read": {"In":["..."]}, "Write": {"In":["..."]}, "Resolve": {"In":["..."]}, "Admin": {"In":["..."]}}`
)

var (
	jan2015  = time.Date(2015, time.January, 1, 0, 0, 0, 0, time.UTC)
	feb2015  = time.Date(2015, time.February, 1, 0, 0, 0, 0, time.UTC)
	mar2015  = time.Date(2015, time.March, 1, 0, 0, 0, 0, time.UTC)
	fiveSecs = 5 * time.Second
)

func testInit(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
}

////////////////////////////////////////////////////////////////////////////////
// Tests for local vclock updates

// Tests that the virtual clock moves forward.
func V23TestVClockMovesForward(t *v23tests.T) {
	testInit(t)
	ctx := forkContext(t, "c0")
	s0Creds := forkCredentials(t, "s0")
	cleanup := tu.StartSyncbased(t, s0Creds, "s0", "", openPerms)
	defer cleanup()

	t0, err := sc("s0").DevModeGetTime(ctx)
	ok(t, err)
	t1, err := sc("s0").DevModeGetTime(ctx)
	ok(t, err)

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

	// Initialize system time.
	ok(t, sc("s0").DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		Now:           jan2015,
		ElapsedTime:   0,
		DoLocalUpdate: true,
	}))
	checkSbTimeApproxEq(t, "s0", ctx, jan2015)

	// Move time and elapsed time forward by equal amounts. Syncbase should not
	// detect any anomalies, and should reflect the new time.
	ok(t, sc("s0").DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		Now:           jan2015.Add(time.Hour),
		ElapsedTime:   time.Hour,
		DoLocalUpdate: true,
	}))
	checkSbTimeApproxEq(t, "s0", ctx, jan2015.Add(time.Hour))

	// Move time forward, and reset elapsed time to one second. Syncbase should
	// detect a reboot, and should reflect the new time.
	ok(t, sc("s0").DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		Now:           jan2015.Add(8 * time.Hour),
		ElapsedTime:   time.Second,
		DoLocalUpdate: true,
	}))
	checkSbTimeApproxEq(t, "s0", ctx, jan2015.Add(8*time.Hour))

	// Move time backward, and reset elapsed time to 0. Syncbase should detect a
	// reboot, and should reflect the new time.
	ok(t, sc("s0").DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		Now:           jan2015.Add(-time.Hour),
		ElapsedTime:   0,
		DoLocalUpdate: true,
	}))
	checkSbTimeApproxEq(t, "s0", ctx, jan2015.Add(-time.Hour))

	// Move time forward 5 hours and set elapsed time to 3 hours. Syncbase should
	// detect this as a system clock change (since the system clock time is not
	// where it's expected to be) and adjust skew so that the new reported time is
	// equal to the previous reported time plus the change in elapsed time.
	ok(t, sc("s0").DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		Now:           jan2015.Add(5 * time.Hour),
		ElapsedTime:   3 * time.Hour,
		DoLocalUpdate: true,
	}))
	checkSbTimeApproxEq(t, "s0", ctx, jan2015.Add(2*time.Hour))
}

// Tests that the virtual clock daemon checks for system clock updates at the
// expected frequency (loosely speaking).
func V23TestVClockSystemClockFrequency(t *v23tests.T) {
	testInit(t)
	ctx := forkContext(t, "c0")
	s0Creds := forkCredentials(t, "s0")
	cleanup := tu.StartSyncbased(t, s0Creds, "s0", "", openPerms)
	defer cleanup()

	checkSbTimeNotEq(t, "s0", ctx, jan2015)

	// Set system time but do not force a local update.
	ok(t, sc("s0").DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		Now:           jan2015,
		ElapsedTime:   0,
		DoLocalUpdate: false,
	}))

	// Sleep for two seconds. Note, VClockD checks for sysclock changes once per
	// second.
	time.Sleep(2 * time.Second)

	// Virtual clock should reflect the sysclock change.
	checkSbTimeApproxEq(t, "s0", ctx, jan2015)
}

////////////////////////////////////////////////////////////////////////////////
// Tests for NTP vclock updates

// Tests that NTP sync affects virtual clock state (e.g. clock can move
// backward).
func V23TestVClockNtpUpdate(t *v23tests.T) {
	testInit(t)
	ctx := forkContext(t, "c0")
	s0Creds := forkCredentials(t, "s0")
	cleanup := tu.StartSyncbased(t, s0Creds, "s0", "", openPerms)
	defer cleanup()

	checkSbTimeNotEq(t, "s0", ctx, jan2015)

	// Use NTP to set the clock to jan2015.
	ok(t, sc("s0").DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		NtpHost:     startFakeNtpServer(t, jan2015),
		DoNtpUpdate: true,
	}))
	checkSbTimeApproxEq(t, "s0", ctx, jan2015)

	// Use NTP to move the clock forward to feb2015.
	ok(t, sc("s0").DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		NtpHost:     startFakeNtpServer(t, feb2015),
		DoNtpUpdate: true,
	}))
	checkSbTimeApproxEq(t, "s0", ctx, feb2015)
}

// Tests that NTP skew persists across reboots.
func V23TestVClockNtpSkewAfterReboot(t *v23tests.T) {
	testInit(t)
	ctx := forkContext(t, "c0")
	s0Creds := forkCredentials(t, "s0")
	cleanup := tu.StartSyncbased(t, s0Creds, "s0", "", openPerms)
	defer cleanup()

	// Set s0's local clock.
	ok(t, sc("s0").DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		Now:           jan2015.Add(-time.Hour),
		ElapsedTime:   0,
		DoLocalUpdate: true,
	}))
	checkSbTimeApproxEq(t, "s0", ctx, jan2015.Add(-time.Hour))

	// Do NTP at s0. As a result, s0 will think it has a one hour NTP skew, i.e.
	// NTP time minus system clock time equals one hour.
	ok(t, sc("s0").DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		NtpHost:     startFakeNtpServer(t, jan2015),
		DoNtpUpdate: true,
	}))
	checkSbTimeApproxEq(t, "s0", ctx, jan2015)

	// Move time forward at s0, and reset its elapsed time to 0. Syncbase should
	// detect a reboot, and should reflect the new time. Note that the time
	// reported by s0.GetTime should reflect its one hour NTP skew.
	ok(t, sc("s0").DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		Now:           jan2015.Add(8 * time.Hour),
		ElapsedTime:   0,
		DoLocalUpdate: true,
	}))
	checkSbTimeApproxEq(t, "s0", ctx, jan2015.Add(9*time.Hour))
}

// Tests that the virtual clock daemon checks in with NTP at the expected
// frequency (loosely speaking).
func V23TestVClockNtpFrequency(t *v23tests.T) {
	testInit(t)
	ctx := forkContext(t, "c0")
	s0Creds := forkCredentials(t, "s0")
	cleanup := tu.StartSyncbased(t, s0Creds, "s0", "", openPerms)
	defer cleanup()

	t0, err := sc("s0").DevModeGetTime(ctx)
	ok(t, err)
	if t0.Sub(jan2015) < time.Hour {
		t.Fatalf("unexpected time: %v", t0)
	}

	// Use NTP to set the clock to jan2015.
	ok(t, sc("s0").DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		NtpHost:     startFakeNtpServer(t, jan2015),
		DoNtpUpdate: true,
	}))
	checkSbTimeApproxEq(t, "s0", ctx, jan2015)

	// Use NTP to move the clock forward to feb2015, but do not run
	// vclockd.DoNtpUpdate(). Since NTP sync happens only once per hour, the
	// virtual clock should continue reporting the old time, even after we sleep
	// for several seconds.
	ok(t, sc("s0").DevModeUpdateVClock(ctx, wire.DevModeUpdateVClockOpts{
		NtpHost:     startFakeNtpServer(t, feb2015),
		DoNtpUpdate: false,
	}))

	time.Sleep(fiveSecs)
	checkSbTimeApproxEq(t, "s0", ctx, jan2015.Add(fiveSecs))
}

////////////////////////////////////////////////////////////////////////////////
// Tests for p2p vclock updates

// Tests p2p clock sync where local is not NTP-synced and is 1, 2, or 3 hops
// away from an NTP-synced device.
func V23TestVClockSyncBasic(t *v23tests.T) {
	testInit(t)
	sbs, cleanup := setupSyncbases(t, 4)
	defer cleanup()

	checkSbTimeNotEq(t, "s0", sbs[0].clientCtx, jan2015)

	// Do NTP at s0.
	ok(t, sc("s0").DevModeUpdateVClock(sbs[0].clientCtx, wire.DevModeUpdateVClockOpts{
		NtpHost:     startFakeNtpServer(t, jan2015),
		DoNtpUpdate: true,
	}))
	checkSbTimeApproxEq(t, "s0", sbs[0].clientCtx, jan2015)

	// Set up a chain of syncgroups, then wait for a few seconds to allow the
	// Syncbases to sync.
	// TODO(sadovsky): Maybe add some sort of a "wait for sync" helper. Without
	// that, we risk flakiness.
	setupChain(t, sbs)
	time.Sleep(fiveSecs)

	// s1 and s2 should sync s0's clock; s3 should not.
	checkSbTimeApproxEq(t, "s1", sbs[1].clientCtx, jan2015.Add(fiveSecs))
	checkSbTimeApproxEq(t, "s2", sbs[2].clientCtx, jan2015.Add(fiveSecs))
	checkSbTimeNotEq(t, "s3", sbs[3].clientCtx, jan2015.Add(fiveSecs))
}

// Tests p2p clock sync where multiple devices are NTP-synced.
func V23TestVClockSyncWithLocalNtp(t *v23tests.T) {
	testInit(t)
	sbs, cleanup := setupSyncbases(t, 3)
	defer cleanup()

	// Do NTP at s0 and s2.
	ok(t, sc("s0").DevModeUpdateVClock(sbs[0].clientCtx, wire.DevModeUpdateVClockOpts{
		NtpHost:     startFakeNtpServer(t, jan2015),
		DoNtpUpdate: true,
	}))
	checkSbTimeApproxEq(t, "s0", sbs[0].clientCtx, jan2015)

	ok(t, sc("s2").DevModeUpdateVClock(sbs[2].clientCtx, wire.DevModeUpdateVClockOpts{
		NtpHost:     startFakeNtpServer(t, feb2015),
		DoNtpUpdate: true,
	}))
	checkSbTimeApproxEq(t, "s2", sbs[2].clientCtx, feb2015)

	// Set up a chain of syncgroups, then wait for a few seconds to allow the
	// Syncbases to sync.
	// TODO(sadovsky): Maybe add some sort of a "wait for sync" helper. Without
	// that, we risk flakiness.
	setupChain(t, sbs)
	time.Sleep(fiveSecs)

	// All Syncbases should now think it's feb2015, because that NTP sync happened
	// most recently.
	// NOTE(sadovsky): Writing this test made me realize our current scheme loses
	// information. Suppose we have Syncbases {A,B,C,D} in a chain, where C and A
	// have NTP'ed at times t0 and t1 respectively. If C transitively hears from A
	// before talking to D, D will not pick up C's NTP time even though C is just
	// one hop away. This probably doesn't matter much in practice.
	checkSbTimeApproxEq(t, "s0", sbs[0].clientCtx, feb2015.Add(fiveSecs))
	checkSbTimeApproxEq(t, "s1", sbs[1].clientCtx, feb2015.Add(fiveSecs))
	checkSbTimeApproxEq(t, "s2", sbs[2].clientCtx, feb2015.Add(fiveSecs))

	// Do NTP at s0 again; the update should propagate through the existing
	// syncgroups.
	ok(t, sc("s0").DevModeUpdateVClock(sbs[0].clientCtx, wire.DevModeUpdateVClockOpts{
		NtpHost:     startFakeNtpServer(t, mar2015),
		DoNtpUpdate: true,
	}))

	time.Sleep(fiveSecs)
	checkSbTimeApproxEq(t, "s0", sbs[0].clientCtx, mar2015.Add(fiveSecs))
	checkSbTimeApproxEq(t, "s1", sbs[1].clientCtx, mar2015.Add(fiveSecs))
	checkSbTimeApproxEq(t, "s2", sbs[2].clientCtx, mar2015.Add(fiveSecs))
}

// Tests p2p clock sync where local is not NTP-synced and is 1 hop away from an
// NTP-synced device with >0 reboots.
func V23TestVClockSyncWithReboots(t *v23tests.T) {
	testInit(t)
	sbs, cleanup := setupSyncbases(t, 2)
	defer cleanup()

	// Set s0's local clock.
	ok(t, sc("s0").DevModeUpdateVClock(sbs[0].clientCtx, wire.DevModeUpdateVClockOpts{
		Now:           jan2015.Add(-time.Hour),
		ElapsedTime:   0,
		DoLocalUpdate: true,
	}))
	checkSbTimeApproxEq(t, "s0", sbs[0].clientCtx, jan2015.Add(-time.Hour))

	// Do NTP at s0. As a result, s0 will think it has a one hour NTP skew, i.e.
	// NTP time minus system clock time equals one hour.
	ok(t, sc("s0").DevModeUpdateVClock(sbs[0].clientCtx, wire.DevModeUpdateVClockOpts{
		NtpHost:     startFakeNtpServer(t, jan2015),
		DoNtpUpdate: true,
	}))
	checkSbTimeApproxEq(t, "s0", sbs[0].clientCtx, jan2015)

	// Set s1's local clock.
	ok(t, sc("s1").DevModeUpdateVClock(sbs[1].clientCtx, wire.DevModeUpdateVClockOpts{
		Now:           feb2015,
		ElapsedTime:   0,
		DoLocalUpdate: true,
	}))
	checkSbTimeApproxEq(t, "s1", sbs[1].clientCtx, feb2015)

	// Move time forward at s0, and reset its elapsed time to 0. Syncbase should
	// detect a reboot, and should reflect the new time. Note that the time
	// reported by s0.GetTime should reflect its one hour NTP skew.
	ok(t, sc("s0").DevModeUpdateVClock(sbs[0].clientCtx, wire.DevModeUpdateVClockOpts{
		Now:           jan2015.Add(8 * time.Hour),
		ElapsedTime:   0,
		DoLocalUpdate: true,
	}))
	checkSbTimeApproxEq(t, "s0", sbs[0].clientCtx, jan2015.Add(9*time.Hour))

	// Since s0 thinks it has rebooted, s1 should not get s0's clock.
	setupChain(t, sbs)
	time.Sleep(fiveSecs)
	checkSbTimeApproxEq(t, "s0", sbs[0].clientCtx, jan2015.Add(9*time.Hour).Add(fiveSecs))
	checkSbTimeApproxEq(t, "s1", sbs[1].clientCtx, feb2015.Add(fiveSecs))
}

////////////////////////////////////////////////////////////////////////////////
// Helper functions

func sc(name string) wire.ServiceClientStub {
	return wire.ServiceClient(name)
}

// Creates a "chain" of syncgroups, where each adjacent pair of Syncbases {A,B}
// share a syncgroup with key prefix "AB".
func setupChain(t *v23tests.T, sbs []*testSyncbase) {
	for i, _ := range sbs {
		if i == len(sbs)-1 {
			break
		}
		a, b := sbs[i], sbs[i+1]
		sgName := naming.Join(a.sbName, util.SyncbaseSuffix, "syncgroup")
		ok(t, createSyncgroup(a.clientCtx, a.sbName, sgName, testTable+":"+a.sbName+b.sbName, "", "root", nil))
		ok(t, joinSyncgroup(b.clientCtx, b.sbName, sgName))
		// Wait for a to see b.
		ok(t, verifySyncgroupMembers(a.clientCtx, a.sbName, sgName, 2))
	}
}

func startFakeNtpServer(t *v23tests.T, now time.Time) string {
	nowBuf, err := now.MarshalText()
	ok(t, err)
	ntpd := t.BuildV23Pkg("v.io/x/ref/services/syncbase/testutil/fake_ntp_server")
	invocation := ntpd.WithStartOpts(ntpd.StartOpts().NoExecProgram().WithSessions(t, time.Second)).Start("--now=" + string(nowBuf))
	host := invocation.ExpectVar("HOST")
	if host == "" {
		t.Fatalf("fake_ntp_server failed to start")
	}
	return host
}

// sbTimeDelta returns sbTime, abs(target-sbTime).
func sbTimeDelta(t *v23tests.T, sbName string, ctx *context.T, target time.Time) (time.Time, time.Duration) {
	sbTime, err := sc(sbName).DevModeGetTime(ctx)
	ok(t, err)
	delta := target.Sub(sbTime)
	if delta < 0 {
		delta = -delta
	}
	return sbTime, delta
}

// checkSbTimeApproxEq checks that the given Syncbase's virtual clock time is
// within 10 seconds of target.
func checkSbTimeApproxEq(t *v23tests.T, sbName string, ctx *context.T, target time.Time) {
	sbTime, delta := sbTimeDelta(t, sbName, ctx, target)
	if delta > 10*time.Second {
		fatalf(t, "unexpected time: got %v, target %v", sbTime, target)
	}
}

// checkSbTimeNotEq checks that the given Syncbase's virtual clock time is not
// within 1 minute of target.
func checkSbTimeNotEq(t *v23tests.T, sbName string, ctx *context.T, target time.Time) {
	sbTime, delta := sbTimeDelta(t, sbName, ctx, target)
	if delta < time.Minute {
		fatalf(t, "expected different times: got %v, target %v", sbTime, target)
	}
}
