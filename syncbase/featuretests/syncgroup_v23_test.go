// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains feature tests that test the life cycle of a syncgroup
// under various application, topology and network scenarios.

package featuretests_test

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"v.io/v23/naming"
	_ "v.io/x/ref/runtime/factories/generic"
	constants "v.io/x/ref/services/syncbase/server/util"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test/modules"
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
	sbs, principals := setupSyncbase(t, N)
	for _, sb := range sbs {
		defer sb.cleanup()
	}

	// Setup one app and one db at each of the Syncbases.
	for _, sb := range sbs {
		tu.RunClient(t, sb.client, runSetupAppA, sb.sbName)
	}

	// Syncbase s0 is the creator.
	sgName := naming.Join(sbs[0].sbName, constants.SyncbaseSuffix, "SG1")
	tu.RunClient(t, sbs[0].client, runCreateSyncgroup, sbs[0].sbName, sgName, "tb:foo", "", principals)

	// Remaining syncbases run the specified workload concurrently.
	for i := 1; i < len(sbs); i++ {
		go tu.RunClient(t, sbs[i].client, runSyncWorkload, sbs[i].sbName, sgName, "foo")
	}

	// Populate data on creator as well.
	keypfx := "foo==" + sbs[0].sbName + "=="
	tu.RunClient(t, sbs[0].client, runPopulateData, sbs[0].sbName, keypfx, "0", "5")

	// Verify steady state sequentially.
	for _, sb := range sbs {
		tu.RunClient(t, sb.client, runVerifySync, sb.sbName, strconv.Itoa(N), "foo")
		tu.RunClient(t, sb.client, runVerifySyncgroupMembers, sb.sbName, sgName, strconv.Itoa(N))
	}

	fmt.Println("V23TestSyncgroupRendezvousOnline=====Phase 1 Done")
}

// V23TestSyncgroupRendezvousOnlineCloud tests that Syncbases can join a
// syncgroup when: all Syncbases are online and a creator creates the syncgroup
// and nominates a cloud syncbase for the other joiners to join at.
func V23TestSyncgroupRendezvousOnlineCloud(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")

	N := 5

	// Setup N+1 Syncbases (1 for the cloud instance).
	sbs, principals := setupSyncbase(t, N+1)
	for _, sb := range sbs {
		defer sb.cleanup()
	}

	// Setup one app and one db at each of the Syncbases.
	for _, sb := range sbs {
		tu.RunClient(t, sb.client, runSetupAppA, sb.sbName)
	}

	// Syncbase s0 is the creator, and sN is the cloud.
	sgName := naming.Join(sbs[N].sbName, constants.SyncbaseSuffix, "SG1")
	tu.RunClient(t, sbs[0].client, runCreateSyncgroup, sbs[0].sbName, sgName, "tb:foo", "", principals)

	// Remaining N-1 syncbases run the specified workload concurrently.
	for i := 1; i < N; i++ {
		go tu.RunClient(t, sbs[i].client, runSyncWorkload, sbs[i].sbName, sgName, "foo")
	}

	// Populate data on creator as well.
	keypfx := "foo==" + sbs[0].sbName + "=="
	tu.RunClient(t, sbs[0].client, runPopulateData, sbs[0].sbName, keypfx, "0", "5")

	// Verify steady state sequentially.
	for i := 0; i < N; i++ {
		tu.RunClient(t, sbs[i].client, runVerifySync, sbs[i].sbName, strconv.Itoa(N), "foo")
		// TODO(hpucha): There is a bug that is currently preventing this verification.
		// tu.RunClient(t, sbs[i].client, runVerifySyncgroupMembers, sbs[i].sbName, sgName, strconv.Itoa(N+1))
	}

	fmt.Println("V23TestSyncgroupRendezvousOnlineCloud=====Phase 1 Done")
}

// V23TestSyncgroupPreknownStaggered tests that Syncbases can join a syncgroup
// when: all Syncbases come online in a staggered fashion. Each Syncbase always
// tries to join a syncgroup with a predetermined name, and if join fails,
// creates the syncgroup.
func V23TestSyncgroupPreknownStaggered(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")

	N := 5

	// Setup N Syncbases.
	sbs, principals := setupSyncbase(t, N)
	for _, sb := range sbs {
		defer sb.cleanup()
	}

	// Setup one app and one db at each of the Syncbases.
	for _, sb := range sbs {
		tu.RunClient(t, sb.client, runSetupAppA, sb.sbName)
	}

	// Syncbase s0 is the first to join or create. Run s0 separately to
	// stagger the process.
	sgName := naming.Join(sbs[0].sbName, constants.SyncbaseSuffix, "SG1")
	tu.RunClient(t, sbs[0].client, runJoinOrCreateSyncgroup, sbs[0].sbName, sgName, "tb:foo", "", principals)

	// Remaining syncbases run the specified workload concurrently.
	for i := 1; i < len(sbs); i++ {
		go tu.RunClient(t, sbs[i].client, runJoinOrCreateSyncgroup, sbs[i].sbName, sgName, "tb:foo", "", principals)
	}

	// Populate and join occur concurrently.
	for _, sb := range sbs {
		keypfx := "foo==" + sb.sbName + "=="
		go tu.RunClient(t, sb.client, runPopulateData, sb.sbName, keypfx, "0", "5")
	}

	// Verify steady state sequentially.
	for _, sb := range sbs {
		tu.RunClient(t, sb.client, runVerifySync, sb.sbName, strconv.Itoa(N), "foo")
		tu.RunClient(t, sb.client, runVerifySyncgroupMembers, sb.sbName, sgName, strconv.Itoa(N))
	}

	fmt.Println("V23TestSyncgroupPreknownStaggered=====Phase 1 Done")
}

////////////////////////////////////////
// Helpers.

type testSyncbase struct {
	sb      *modules.CustomCredentials
	sbName  string
	client  *modules.CustomCredentials
	cleanup func()
}

// Spawns "num" syncbases. Returns handles to the Syncbases, and a ";" separated
// list of Syncbase principals.
func setupSyncbase(t *v23tests.T, num int) ([]*testSyncbase, string) {
	sbs := make([]*testSyncbase, num)
	var principals []string

	for i := 0; i < num; i++ {
		sbs[i] = &testSyncbase{}

		sbs[i].sbName = fmt.Sprintf("s%d", i)
		sbs[i].sb, _ = t.Shell().NewChildCredentials(sbs[i].sbName)

		cName := fmt.Sprintf("c%d", i)
		sbs[i].client, _ = t.Shell().NewChildCredentials(cName)

		// Give Read and Write permissions to the client at its
		// respective Syncbase.
		aclStr := fmt.Sprintf("{\"Read\": {\"In\":[\"root:%s\"]}, \"Write\": {\"In\":[\"root:%s\"]}}", cName, cName)
		sbs[i].cleanup = tu.StartSyncbased(t, sbs[i].sb, sbs[i].sbName, "", aclStr)

		principals = append(principals, "root:"+sbs[i].sbName)
	}
	return sbs, strings.Join(principals, ";")
}

// Arguments: 0: Syncbase name, 1: syncgroup name, 2: data prefix.
var runSyncWorkload = modules.Register(func(env *modules.Env, args ...string) error {
	// Join the syncgroup. Retry for 4 seconds since the server may be in
	// join pending state.
	var err error
	for i := 0; i < 8; i++ {
		time.Sleep(500 * time.Millisecond)
		if err = runJoinSyncGroupInternal(args[0], args[1]); err == nil {
			break
		}
	}
	if err != nil {
		return err
	}

	// Populate some data without colliding with other Syncbases.
	keypfx := args[2] + "==" + args[0] + "=="
	return runPopulateDataInternal(args[0], keypfx, "0", "5")
}, "runSyncWorkload")

// Arguments: 0: Syncbase name, 1: number of syncbases, 2: data prefix.
var runVerifySync = modules.Register(func(env *modules.Env, args ...string) error {
	N, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return err
	}

	for i := N - 1; i >= 0; i-- {
		keypfx := fmt.Sprintf("%s==s%d==", args[2], i)
		if err := runVerifySyncgroupDataInternal(args[0], keypfx, "0", "5", "true"); err != nil {
			return err
		}
	}

	return nil
}, "runVerifySync")

// Arguments: 0: Syncbase name, 1: syncgroup name, 2: prefixes, 3: mount table,
// 4: syncgroup permission blessings (; separated)
var runJoinOrCreateSyncgroup = modules.Register(func(env *modules.Env, args ...string) error {
	if err := runJoinSyncGroupInternal(args[0], args[1]); err == nil {
		return nil
	}
	return runCreateSyncgroupInternal(env, args...)
}, "runJoinOrCreateSyncgroup")
