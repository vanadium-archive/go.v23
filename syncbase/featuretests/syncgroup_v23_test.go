// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains feature tests that test the life cycle of a syncgroup
// under various application, topology and network scenarios.

package featuretests_test

import (
	"fmt"
	"strings"
	"time"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/security/access"
	_ "v.io/x/ref/runtime/factories/generic"
	constants "v.io/x/ref/services/syncbase/server/util"
	"v.io/x/ref/test/v23tests"
)

//go:generate jiri test generate

// V23TestSyncgroupRendezvousOnline tests that Syncbases can join a syncgroup
// when: all Syncbases are online and a creator creates the syncgroup and shares
// the syncgroup name with all the joiners.
func V23TestSyncgroupRendezvousOnline(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")

	N := 5
	// Setup N Syncbases.
	sbs, cleanup := setupSyncbases(t, N)
	defer cleanup()

	// Syncbase s0 is the creator.
	sgName := naming.Join(sbs[0].sbName, constants.SyncbaseSuffix, "SG1")

	ok(t, createSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sgName, "tb:foo", "", sbBlessings(sbs), nil))

	// Remaining syncbases run the specified workload concurrently.
	for i := 1; i < len(sbs); i++ {
		go func(i int) {
			ok(t, runSyncWorkload(sbs[i].clientCtx, sbs[i].sbName, sgName, "foo"))
		}(i)
	}

	// Populate data on creator as well.
	keypfx := "foo==" + sbs[0].sbName + "=="
	ok(t, populateData(sbs[0].clientCtx, sbs[0].sbName, keypfx, 0, 5))

	// Verify steady state sequentially.
	for _, sb := range sbs {
		ok(t, verifySync(sb.clientCtx, sb.sbName, N, "foo"))
		ok(t, verifySyncgroupMembers(sb.clientCtx, sb.sbName, sgName, N))
	}

	fmt.Println("V23TestSyncgroupRendezvousOnline=====Phase 1 Done")
}

// V23TestSyncgroupRendezvousOnlineCloud tests that Syncbases can join a
// syncgroup when: all Syncbases are online and a creator creates the syncgroup
// and nominates a cloud syncbase for the other joiners to join at.
func V23TestSyncgroupRendezvousOnlineCloud(t *v23tests.T) {
	// TODO(hpucha): There is a potential bug that is currently preventing
	// this test from succeeding.
	t.Skip()

	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")

	N := 5
	// Setup N+1 Syncbases (1 for the cloud instance).
	sbs, cleanup := setupSyncbases(t, N+1)
	defer cleanup()

	// Syncbase s0 is the creator, and sN is the cloud.
	sgName := naming.Join(sbs[N].sbName, constants.SyncbaseSuffix, "SG1")

	ok(t, createSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sgName, "tb:foo", "", sbBlessings(sbs), nil))

	// Remaining N-1 syncbases run the specified workload concurrently.
	for i := 1; i < N; i++ {
		go func(i int) {
			ok(t, runSyncWorkload(sbs[i].clientCtx, sbs[i].sbName, sgName, "foo"))
		}(i)
	}

	// Populate data on creator as well.
	keypfx := "foo==" + sbs[0].sbName + "=="
	ok(t, populateData(sbs[0].clientCtx, sbs[0].sbName, keypfx, 0, 5))

	// Verify steady state sequentially.
	for i := 0; i < N; i++ {
		ok(t, verifySync(sbs[i].clientCtx, sbs[i].sbName, N, "foo"))
		// ok(t, verifySyncgroupMembers(sbs[i].clientCtx, sbs[i].sbName, sgName, N+1))
	}

	fmt.Println("V23TestSyncgroupRendezvousOnlineCloud=====Phase 1 Done")
}

// V23TestSyncgroupNeighborhoodOnly tests that Syncbases can join a syncgroup
// when: all Syncbases do not have general connectivity but can reach each other
// over neighborhood, and a creator creates the syncgroup and shares the
// syncgroup name with all the joiners. Restricted connectivity is simulated by
// picking a syncgroup name that is not reachable and a syncgroup mount table
// that doesn't exist.
func V23TestSyncgroupNeighborhoodOnly(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")

	N := 5

	// Setup N Syncbases.
	sbs, cleanup := setupSyncbases(t, N)
	defer cleanup()

	// Syncbase s0 is the creator, but the syncgroup refers to non-existent
	// Syncbase "s6".
	sgName := naming.Join("s6", constants.SyncbaseSuffix, "SG1")

	// For now, we set the permissions to have a single admin. Once we have
	// syncgroup conflict resolution in place, we should be able to have
	// multiple admins on the neighborhood.
	//
	// TODO(hpucha): Change it to multi-admin scenario.
	principals := sbBlessings(sbs)
	perms := access.Permissions{}
	for _, tag := range []access.Tag{access.Read, access.Write} {
		for _, pattern := range strings.Split(principals, ";") {
			perms.Add(security.BlessingPattern(pattern), string(tag))
		}
	}
	perms.Add(security.BlessingPattern("root:"+sbs[0].sbName), string(access.Admin))

	ok(t, createSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sgName, "tb:foo", "/mttable", "", perms))

	// Remaining syncbases run the specified workload concurrently.
	for i := 1; i < len(sbs); i++ {
		go func(i int) {
			ok(t, runSyncWorkload(sbs[i].clientCtx, sbs[i].sbName, sgName, "foo"))
		}(i)
	}

	// Populate data on creator as well.
	keypfx := "foo==" + sbs[0].sbName + "=="
	ok(t, populateData(sbs[0].clientCtx, sbs[0].sbName, keypfx, 0, 5))

	// Verify steady state sequentially.
	for _, sb := range sbs {
		ok(t, verifySync(sb.clientCtx, sb.sbName, N, "foo"))
		ok(t, verifySyncgroupMembers(sb.clientCtx, sb.sbName, sgName, N))
	}

	fmt.Println("V23TestSyncgroupNeighborhoodOnly=====Phase 1 Done")
}

// V23TestSyncgroupPreknownStaggered tests that Syncbases can join a syncgroup
// when: all Syncbases come online in a staggered fashion. Each Syncbase always
// tries to join a syncgroup with a predetermined name, and if join fails,
// creates the syncgroup.
func V23TestSyncgroupPreknownStaggered(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")

	N := 5
	// Setup N Syncbases.
	sbs, cleanup := setupSyncbases(t, N)
	defer cleanup()

	// Syncbase s0 is the first to join or create. Run s0 separately to
	// stagger the process.
	sgName := naming.Join(sbs[0].sbName, constants.SyncbaseSuffix, "SG1")
	ok(t, joinOrCreateSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sgName, "tb:foo", "", sbBlessings(sbs)))

	// Remaining syncbases run the specified workload concurrently.
	for i := 1; i < len(sbs); i++ {
		go func(i int) {
			ok(t, joinOrCreateSyncgroup(sbs[i].clientCtx, sbs[i].sbName, sgName, "tb:foo", "", sbBlessings(sbs)))
		}(i)
	}

	// Populate and join occur concurrently.
	for _, sb := range sbs {
		go func(sb *testSyncbase) {
			keypfx := "foo==" + sb.sbName + "=="
			ok(t, populateData(sb.clientCtx, sb.sbName, keypfx, 0, 5))
		}(sb)
	}

	// Verify steady state sequentially.
	for _, sb := range sbs {
		ok(t, verifySync(sb.clientCtx, sb.sbName, N, "foo"))
		ok(t, verifySyncgroupMembers(sb.clientCtx, sb.sbName, sgName, N))
	}

	fmt.Println("V23TestSyncgroupPreknownStaggered=====Phase 1 Done")
}

////////////////////////////////////////
// Helpers.

func runSyncWorkload(ctx *context.T, syncbaseName, sgName, prefix string) error {
	var err error
	for i := 0; i < 8; i++ {
		time.Sleep(500 * time.Millisecond)
		if err = joinSyncgroup(ctx, syncbaseName, sgName); err == nil {
			break
		}
	}
	if err != nil {
		return err
	}

	// Populate some data without colliding with data from other Syncbases.
	keypfx := prefix + "==" + syncbaseName + "=="
	return populateData(ctx, syncbaseName, keypfx, 0, 5)
}

func verifySync(ctx *context.T, syncbaseName string, numSyncbases int, prefix string) error {
	for i := numSyncbases - 1; i >= 0; i-- {
		keypfx := fmt.Sprintf("%s==s%d==", prefix, i)
		if err := verifySyncgroupData(ctx, syncbaseName, keypfx, 0, 5); err != nil {
			return err
		}
	}
	return nil
}

func joinOrCreateSyncgroup(ctx *context.T, syncbaseName, sgName, sgPrefixes, mtName, bps string) error {
	if err := joinSyncgroup(ctx, syncbaseName, sgName); err == nil {
		return nil
	}
	return createSyncgroup(ctx, syncbaseName, sgName, sgPrefixes, mtName, bps, nil)
}
