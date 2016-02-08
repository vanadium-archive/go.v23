// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql_test

import (
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	wire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/nosql"
	"v.io/v23/verror"
	"v.io/v23/vom"
	_ "v.io/x/ref/runtime/factories/generic"
	constants "v.io/x/ref/services/syncbase/server/util"
	"v.io/x/ref/test/v23test"
)

const (
	testTable = "tb"
)

// NOTE(sadovsky): These tests take a very long time to run - nearly 4 minutes
// on my Macbook Pro! Various instances of time.Sleep() below likely contribute
// to the problem.

// TODO(ivanpi): Move to featuretests and deduplicate helpers.

// TestV23SyncbasedJoinSyncgroup tests the creation and joining of a syncgroup.
// Client0 creates a syncgroup at Syncbase0. Client1 requests to join the
// syncgroup at Syncbase1. Syncbase1 in turn requests Syncbase0 to join the
// syncgroup.
func TestV23SyncbasedJoinSyncgroup(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, v23test.Opts{})
	defer sh.Cleanup()
	sh.StartRootMountTable()

	server0Creds := sh.ForkCredentials("s0")
	client0Ctx := sh.ForkContext("c0")
	sh.StartSyncbase(server0Creds, "sync0", "",
		`{"Read": {"In":["root:c0"]}, "Write": {"In":["root:c0"]}}`)

	server1Creds := sh.ForkCredentials("s1")
	client1Ctx := sh.ForkContext("c1")
	sh.StartSyncbase(server1Creds, "sync1", "",
		`{"Read": {"In":["root:c1"]}, "Write": {"In":["root:c1"]}}`)

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")

	ok(t, runSetupAppA(client0Ctx, "sync0"))
	ok(t, runCreateSyncgroup(client0Ctx, "sync0", sgName, "tb:foo", "", "root:s0", "root:s1"))

	ok(t, runSetupAppA(client1Ctx, "sync1"))
	ok(t, runJoinSyncgroup(client1Ctx, "sync1", sgName))
}

// TestV23SyncbasedGetDeltas tests the sending of deltas between two Syncbase
// instances and their clients.  The 1st client creates a syncgroup and puts
// some database entries in it.  The 2nd client joins that syncgroup and reads
// the database entries.  This verifies the end-to-end synchronization of data
// along the path: client0--Syncbase0--Syncbase1--client1.
func TestV23SyncbasedGetDeltas(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, v23test.Opts{})
	defer sh.Cleanup()
	sh.StartRootMountTable()

	server0Creds := sh.ForkCredentials("s0")
	client0Ctx := sh.ForkContext("c0")
	sh.StartSyncbase(server0Creds, "sync0", "",
		`{"Read": {"In":["root:c0"]}, "Write": {"In":["root:c0"]}}`)

	server1Creds := sh.ForkCredentials("s1")
	client1Ctx := sh.ForkContext("c1")
	// TODO(aghassemi): Resolve permission is currently needed for Watch.
	// See https://github.com/vanadium/issues/issues/1110
	sh.StartSyncbase(server1Creds, "sync1", "",
		`{"Resolve": {"In":["root:c1"]}, "Read": {"In":["root:c1"]}, "Write": {"In":["root:c1"]}}`)

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")

	ok(t, runSetupAppA(client0Ctx, "sync0"))
	ok(t, runCreateSyncgroup(client0Ctx, "sync0", sgName, "tb:foo,tb:bar", "", "root:s0", "root:s1"))
	ok(t, runPopulateData(client0Ctx, "sync0", "foo", 0))
	ok(t, runPopulateNonVomData(client0Ctx, "sync0", "bar", 0))

	ok(t, runSetupAppA(client1Ctx, "sync1"))
	beforeSyncMarker, err := getResumeMarker(client1Ctx, "sync1")
	ok(t, err)
	ok(t, runJoinSyncgroup(client1Ctx, "sync1", sgName))
	ok(t, runVerifySyncgroupData(client1Ctx, "sync1", "foo", 0, 10, false))
	ok(t, runVerifySyncgroupDataWithWatch(client1Ctx, "sync1", "foo", 10, false, beforeSyncMarker))
	ok(t, runVerifySyncgroupNonVomData(client1Ctx, "sync1", "bar", 0, 10))
}

// TestV23SyncbasedGetDeltasWithDel tests the sending of deltas between two
// Syncbase instances and their clients. The 1st client creates a syncgroup and
// puts some database entries in it. The 2nd client joins that syncgroup and
// reads the database entries. The 1st client then deletes a portion of this
// data, and adds new entries. The 2nd client verifies that these changes are
// correctly synced. This verifies the end-to-end synchronization of data along
// the path: client0--Syncbase0--Syncbase1--client1 with a workload of puts and
// deletes.
func TestV23SyncbasedGetDeltasWithDel(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, v23test.Opts{})
	defer sh.Cleanup()
	sh.StartRootMountTable()

	server0Creds := sh.ForkCredentials("s0")
	client0Ctx := sh.ForkContext("c0")
	sh.StartSyncbase(server0Creds, "sync0", "",
		`{"Read": {"In":["root:c0"]}, "Write": {"In":["root:c0"]}}`)

	server1Creds := sh.ForkCredentials("s1")
	client1Ctx := sh.ForkContext("c1")
	// TODO(aghassemi): Resolve permission is currently needed for Watch.
	// See https://github.com/vanadium/issues/issues/1110
	sh.StartSyncbase(server1Creds, "sync1", "",
		`{"Resolve": {"In":["root:c1"]}, "Read": {"In":["root:c1"]}, "Write": {"In":["root:c1"]}}`)

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")

	ok(t, runSetupAppA(client0Ctx, "sync0"))
	ok(t, runCreateSyncgroup(client0Ctx, "sync0", sgName, "tb:foo,tb:bar", "", "root:s0", "root:s1"))
	ok(t, runPopulateData(client0Ctx, "sync0", "foo", 0))

	ok(t, runSetupAppA(client1Ctx, "sync1"))
	beforeSyncMarker, err := getResumeMarker(client1Ctx, "sync1")
	ok(t, err)
	ok(t, runJoinSyncgroup(client1Ctx, "sync1", sgName))
	ok(t, runVerifySyncgroupData(client1Ctx, "sync1", "foo", 0, 10, false))
	ok(t, runVerifySyncgroupDataWithWatch(client1Ctx, "sync1", "foo", 10, false, beforeSyncMarker))

	beforeDeleteMarker, err := getResumeMarker(client1Ctx, "sync1")
	ok(t, err)
	ok(t, runDeleteData(client0Ctx, "sync0", 0))
	ok(t, runVerifyDeletedData(client0Ctx, "sync0", "foo"))
	ok(t, runVerifyDeletedData(client1Ctx, "sync1", "foo"))
	ok(t, runVerifySyncgroupDataWithWatch(client1Ctx, "sync1", "foo", 5, true, beforeDeleteMarker))

	ok(t, runPopulateData(client0Ctx, "sync0", "bar", 0))
	ok(t, runVerifySyncgroupData(client1Ctx, "sync1", "bar", 0, 10, false))
}

// TestV23SyncbasedCompEval is a comprehensive sniff test for core sync
// functionality. It tests the exchange of deltas between two Syncbase instances
// and their clients. The 1st client creates a syncgroup and puts some database
// entries in it. The 2nd client joins that syncgroup and reads the database
// entries. The 2nd client then updates a subset of existing keys and adds more
// keys and the 1st client verifies that it can read these keys. This verifies
// the end-to-end bi-directional synchronization of data along the path:
// client0--Syncbase0--Syncbase1--client1. In addition, this test also verifies
// the bi-directional exchange of syncgroup deltas. After the first phase is
// done, both Syncbase instances are shutdown and restarted, and new data is
// synced once again.
func TestV23SyncbasedCompEval(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, v23test.Opts{})
	defer sh.Cleanup()
	sh.StartRootMountTable()

	server0Creds := sh.ForkCredentials("s0")
	client0Ctx := sh.ForkContext("c0")
	server0RDir := sh.MakeTempDir()
	cleanSync0 := sh.StartSyncbase(server0Creds, "sync0", server0RDir,
		`{"Read": {"In":["root:c0"]}, "Write": {"In":["root:c0"]}}`)

	server1Creds := sh.ForkCredentials("s1")
	client1Ctx := sh.ForkContext("c1")
	server1RDir := sh.MakeTempDir()
	cleanSync1 := sh.StartSyncbase(server1Creds, "sync1", server1RDir,
		`{"Read": {"In":["root:c1"]}, "Write": {"In":["root:c1"]}}`)

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")

	ok(t, runSetupAppA(client0Ctx, "sync0"))
	ok(t, runCreateSyncgroup(client0Ctx, "sync0", sgName, "tb:foo", "", "root:s0", "root:s1"))
	ok(t, runPopulateData(client0Ctx, "sync0", "foo", 0))

	// This is a decoy syncgroup that no other Syncbase joins, but is on the
	// same database as the first syncgroup. Populating it after the first
	// syncgroup causes the database generations to go up, but the joiners
	// on the first syncgroup do not get any data belonging to this
	// syncgroup. This triggers the handling of filtered log records in the
	// restartability code.
	sgName1 := naming.Join("sync0", constants.SyncbaseSuffix, "SG2")
	ok(t, runCreateSyncgroup(client0Ctx, "sync0", sgName1, "tb:bar", "", "root:s0", "root:s1"))
	ok(t, runPopulateData(client0Ctx, "sync0", "bar", 0))

	ok(t, runSetupAppA(client1Ctx, "sync1"))
	ok(t, runJoinSyncgroup(client1Ctx, "sync1", sgName))
	ok(t, runVerifySyncgroupData(client1Ctx, "sync1", "foo", 0, 10, false))

	// Shutdown and restart Syncbase instances.
	cleanSync0(os.Interrupt)
	cleanSync1(os.Interrupt)

	cleanSync0 = sh.StartSyncbase(server0Creds, "sync0", server0RDir,
		`{"Read": {"In":["root:c0"]}, "Write": {"In":["root:c0"]}}`)
	cleanSync1 = sh.StartSyncbase(server1Creds, "sync1", server1RDir,
		`{"Read": {"In":["root:c1"]}, "Write": {"In":["root:c1"]}}`)

	ok(t, runSetSyncgroupSpec(client0Ctx, "sync0", sgName, "v2", "tb:foo", "root:s0", "root:s1", "root:s3"))
	ok(t, runGetSyncgroupSpec(client1Ctx, "sync1", sgName, "v2", "tb:foo", "root:s0", "root:s1", "root:s3"))

	ok(t, runUpdateData(client1Ctx, "sync1", 5))
	ok(t, runPopulateData(client1Ctx, "sync1", "foo", 10))
	ok(t, runSetSyncgroupSpec(client1Ctx, "sync1", sgName, "v3", "tb:foo", "root:s0", "root:s1", "root:s4"))

	ok(t, runVerifyLocalAndRemoteData(client0Ctx, "sync0"))
	ok(t, runGetSyncgroupSpec(client0Ctx, "sync0", sgName, "v3", "tb:foo", "root:s0", "root:s1", "root:s4"))

	// Shutdown and restart Syncbase instances.
	cleanSync0(os.Interrupt)
	cleanSync1(os.Interrupt)

	_ = sh.StartSyncbase(server0Creds, "sync0", server0RDir,
		`{"Read": {"In":["root:c0"]}, "Write": {"In":["root:c0"]}}`)
	_ = sh.StartSyncbase(server1Creds, "sync1", server1RDir,
		`{"Read": {"In":["root:c1"]}, "Write": {"In":["root:c1"]}}`)

	ok(t, runGetSyncgroupSpec(client0Ctx, "sync0", sgName, "v3", "tb:foo", "root:s0", "root:s1", "root:s4"))
	ok(t, runGetSyncgroupSpec(client1Ctx, "sync1", sgName, "v3", "tb:foo", "root:s0", "root:s1", "root:s4"))
	ok(t, runPopulateData(client0Ctx, "sync0", "foo", 20))
	ok(t, runVerifySyncgroupData(client1Ctx, "sync1", "foo", 20, 10, true))
}

// TestV23SyncbasedExchangeDeltasWithAcls tests the exchange of deltas including
// acls between two Syncbase instances and their clients.  The 1st client
// creates a syncgroup at "foo", sets an acl, and puts some database entries in
// it.  The 2nd client joins that syncgroup and reads the database entries.  The
// 2nd client then adds a prefix acl with access to only itself at "foobar".
// The 1st client should be unable to access the subset of keys under
// "foobar". The 2nd client then modifies the prefix acl at "foobar" with access
// to both clients. The 1st client should regain access.
func TestV23SyncbasedExchangeDeltasWithAcls(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, v23test.Opts{})
	defer sh.Cleanup()
	sh.StartRootMountTable()

	server0Creds := sh.ForkCredentials("s0")
	client0Ctx := sh.ForkContext("c0")
	sh.StartSyncbase(server0Creds, "sync0", "",
		`{"Read": {"In":["root:c0"]}, "Write": {"In":["root:c0"]}, "Admin": {"In":["root:c0"]}}`)

	server1Creds := sh.ForkCredentials("s1")
	client1Ctx := sh.ForkContext("c1")
	sh.StartSyncbase(server1Creds, "sync1", "",
		`{"Read": {"In":["root:c1"]}, "Write": {"In":["root:c1"]}, "Admin": {"In":["root:c1"]}}`)

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")

	ok(t, runSetupAppA(client0Ctx, "sync0"))
	ok(t, runCreateSyncgroup(client0Ctx, "sync0", sgName, "tb:foo", "", "root:s0", "root:s1"))
	ok(t, runPopulateData(client0Ctx, "sync0", "foobarbaz", 0))
	ok(t, runPopulateData(client0Ctx, "sync0", "foo", 0))
	ok(t, runSetPrefixPermissions(client0Ctx, "sync0", "foo", "root:c0", "root:c1"))

	ok(t, runSetupAppA(client1Ctx, "sync1"))
	ok(t, runJoinSyncgroup(client1Ctx, "sync1", sgName))
	ok(t, runVerifySyncgroupData(client1Ctx, "sync1", "foobarbaz", 0, 10, false))
	ok(t, runVerifySyncgroupData(client1Ctx, "sync1", "foo", 0, 10, true))

	ok(t, runSetPrefixPermissions(client1Ctx, "sync1", "foobar", "root:c1"))
	ok(t, runVerifyLostAccess(client0Ctx, "sync0", "foobarbaz", 0, 10))
	ok(t, runVerifySyncgroupData(client0Ctx, "sync0", "foo", 0, 10, true))

	ok(t, runSetPrefixPermissions(client1Ctx, "sync1", "foobar", "root:c0", "root:c1"))
	ok(t, runVerifySyncgroupData(client0Ctx, "sync0", "foobarbaz", 0, 10, false))
}

// TestV23SyncbasedExchangeDeltasWithConflicts tests the exchange of deltas
// between two Syncbase instances and their clients in the presence of
// conflicting updates. The 1st client creates a syncgroup and puts some
// database entries in it. The 2nd client joins that syncgroup and reads the
// database entries. Both clients then update a subset of existing keys
// concurrently, and sync with each other. During sync, the following
// possibilities arise: (1) Both clients make their local updates first, sync
// with each other to detect conflicts. Resolution will cause one of the clients
// to see a new value based on the timestamp. (2) One client's update is synced
// before the other client has a chance to commit its update. The other client's
// update will then not be a conflict but a valid update building on the first
// one's change.
//
// Note that the verification done from the client side can have false positives
// re. the test's success. Since we cannot accurately predict which client's
// updates win, the test passes if we find either outcome. However, this could
// also imply that the sync failed, and each client is merely reading its own
// local value. The verification step mainly verifies that syncbased is still
// responsive and that the data is not corrupt.
//
// TODO(hpucha): We could diff the states of the two clients and ensure they are
// identical. Optionally we could expose inner state of syncbased via some
// debug methods.
func TestV23SyncbasedExchangeDeltasWithConflicts(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)

	// Run it multiple times to exercise different interactions between sync
	// and local updates that change every run due to timing.
	for i := 0; i < 10; i++ {
		testSyncbasedExchangeDeltasWithConflicts(t)
	}
}

func testSyncbasedExchangeDeltasWithConflicts(t *testing.T) {
	sh := v23test.NewShell(t, v23test.Opts{})
	defer sh.Cleanup()
	sh.StartRootMountTable()

	server0Creds := sh.ForkCredentials("s0")
	client0Ctx := sh.ForkContext("c0")
	sh.StartSyncbase(server0Creds, "sync0", "",
		`{"Read": {"In":["root:c0"]}, "Write": {"In":["root:c0"]}}`)

	server1Creds := sh.ForkCredentials("s1")
	client1Ctx := sh.ForkContext("c1")
	sh.StartSyncbase(server1Creds, "sync1", "",
		`{"Read": {"In":["root:c1"]}, "Write": {"In":["root:c1"]}}`)

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")

	ok(t, runSetupAppA(client0Ctx, "sync0"))
	ok(t, runCreateSyncgroup(client0Ctx, "sync0", sgName, "tb:foo", "", "root:s0", "root:s1"))
	ok(t, runPopulateData(client0Ctx, "sync0", "foo", 0))

	ok(t, runSetupAppA(client1Ctx, "sync1"))
	ok(t, runJoinSyncgroup(client1Ctx, "sync1", sgName))
	ok(t, runVerifySyncgroupData(client1Ctx, "sync1", "foo", 0, 10, false))

	go func() { ok(t, runUpdateData(client0Ctx, "sync0", 5)) }()
	d := time.Duration(rand.Int63n(50)) * time.Millisecond
	time.Sleep(d)
	ok(t, runUpdateData(client1Ctx, "sync1", 5))

	time.Sleep(10 * time.Second)

	ok(t, runVerifyConflictResolution(client0Ctx, "sync0"))
	ok(t, runVerifyConflictResolution(client1Ctx, "sync1"))
}

// TestV23NestedSyncgroups tests the exchange of deltas between two Syncbase
// instances and their clients with nested syncgroups. The 1st client creates
// two syncgroups at prefixes "f" and "foo" and puts some database entries in
// both of them.  The 2nd client first joins the syncgroup with prefix "foo" and
// verifies that it reads the corresponding database entries.  The 2nd client
// then joins the syncgroup with prefix "f" and verifies that it can read the
// "f" keys.
func TestV23NestedSyncgroups(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, v23test.Opts{})
	defer sh.Cleanup()
	sh.StartRootMountTable()

	server0Creds := sh.ForkCredentials("s0")
	client0Ctx := sh.ForkContext("c0")
	sh.StartSyncbase(server0Creds, "sync0", "",
		`{"Read": {"In":["root:c0"]}, "Write": {"In":["root:c0"]}}`)

	server1Creds := sh.ForkCredentials("s1")
	client1Ctx := sh.ForkContext("c1")
	sh.StartSyncbase(server1Creds, "sync1", "",
		`{"Read": {"In":["root:c1"]}, "Write": {"In":["root:c1"]}}`)

	sg1Name := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")
	sg2Name := naming.Join("sync0", constants.SyncbaseSuffix, "SG2")

	ok(t, runSetupAppA(client0Ctx, "sync0"))
	ok(t, runCreateSyncgroup(client0Ctx, "sync0", sg1Name, "tb:foo", "", "root:s0", "root:s1"))
	ok(t, runCreateSyncgroup(client0Ctx, "sync0", sg2Name, "tb:f", "", "root:s0", "root:s1"))
	ok(t, runPopulateData(client0Ctx, "sync0", "f", 0))
	ok(t, runPopulateData(client0Ctx, "sync0", "foo", 0))

	ok(t, runSetupAppA(client1Ctx, "sync1"))
	ok(t, runJoinSyncgroup(client1Ctx, "sync1", sg1Name))
	ok(t, runVerifySyncgroupData(client1Ctx, "sync1", "foo", 0, 10, false))
	ok(t, runJoinSyncgroup(client1Ctx, "sync1", sg2Name))
	ok(t, runVerifyNestedSyncgroupData(client1Ctx, "sync1"))
}

// TestV23NestedAndPeerSyncgroups tests the exchange of deltas between three
// Syncbase instances and their clients consisting of nested/peer
// syncgroups. The 1st client creates two syncgroups: SG1 at prefix "foo" and
// SG2 at "f" and puts some database entries in both of them.  The 2nd client
// joins the syncgroup SG1 and verifies that it reads the corresponding database
// entries. Client 2 then creates SG3 at prefix "f". The 3rd client joins the
// syncgroups SG2 and SG3 and verifies that it can read all the "f" and "foo"
// keys created by client 1. Client 2 also verifies that it can read all the "f"
// and "foo" keys created by client 1.
func TestV23NestedAndPeerSyncgroups(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, v23test.Opts{})
	defer sh.Cleanup()
	sh.StartRootMountTable()

	server0Creds := sh.ForkCredentials("s0")
	client0Ctx := sh.ForkContext("c0")
	sh.StartSyncbase(server0Creds, "sync0", "",
		`{"Read": {"In":["root:c0"]}, "Write": {"In":["root:c0"]}}`)

	server1Creds := sh.ForkCredentials("s1")
	client1Ctx := sh.ForkContext("c1")
	sh.StartSyncbase(server1Creds, "sync1", "",
		`{"Read": {"In":["root:c1"]}, "Write": {"In":["root:c1"]}}`)

	server2Creds := sh.ForkCredentials("s2")
	client2Ctx := sh.ForkContext("c2")
	sh.StartSyncbase(server2Creds, "sync2", "",
		`{"Read": {"In":["root:c2"]}, "Write": {"In":["root:c2"]}}`)

	sg1Name := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")
	sg2Name := naming.Join("sync0", constants.SyncbaseSuffix, "SG2")
	sg3Name := naming.Join("sync1", constants.SyncbaseSuffix, "SG3")

	ok(t, runSetupAppA(client0Ctx, "sync0"))
	ok(t, runCreateSyncgroup(client0Ctx, "sync0", sg1Name, "tb:foo", "", "root:s0", "root:s1", "root:s2"))
	ok(t, runCreateSyncgroup(client0Ctx, "sync0", sg2Name, "tb:f", "", "root:s0", "root:s2"))
	ok(t, runPopulateData(client0Ctx, "sync0", "f", 0))
	ok(t, runPopulateData(client0Ctx, "sync0", "foo", 0))

	ok(t, runSetupAppA(client1Ctx, "sync1"))
	ok(t, runJoinSyncgroup(client1Ctx, "sync1", sg1Name))
	ok(t, runVerifySyncgroupData(client1Ctx, "sync1", "foo", 0, 10, false))
	ok(t, runCreateSyncgroup(client1Ctx, "sync1", sg3Name, "tb:f", "", "root:s1", "root:s2"))

	ok(t, runSetupAppA(client2Ctx, "sync2"))
	ok(t, runJoinSyncgroup(client2Ctx, "sync2", sg2Name))
	ok(t, runJoinSyncgroup(client2Ctx, "sync2", sg3Name))
	ok(t, runVerifyNestedSyncgroupData(client2Ctx, "sync2"))

	ok(t, runVerifyNestedSyncgroupData(client1Ctx, "sync1"))
}

// TestV23SyncbasedGetDeltasPrePopulate tests the sending of deltas between two
// Syncbase instances and their clients with data existing before the creation
// of a syncgroup.  The 1st client puts entries in a database then creates a
// syncgroup over that data.  The 2nd client joins that syncgroup and reads the
// database entries.
func TestV23SyncbasedGetDeltasPrePopulate(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, v23test.Opts{})
	defer sh.Cleanup()
	sh.StartRootMountTable()

	server0Creds := sh.ForkCredentials("s0")
	client0Ctx := sh.ForkContext("c0")
	sh.StartSyncbase(server0Creds, "sync0", "",
		`{"Read": {"In":["root:c0"]}, "Write": {"In":["root:c0"]}}`)

	server1Creds := sh.ForkCredentials("s1")
	client1Ctx := sh.ForkContext("c1")
	sh.StartSyncbase(server1Creds, "sync1", "",
		`{"Read": {"In":["root:c1"]}, "Write": {"In":["root:c1"]}}`)

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")

	// Populate table data before creating the syncgroup.  Also populate
	// with data that is not part of the syncgroup to verify filtering.
	ok(t, runSetupAppA(client0Ctx, "sync0"))
	ok(t, runPopulateData(client0Ctx, "sync0", "foo", 0))
	ok(t, runPopulateData(client0Ctx, "sync0", "bar", 0))
	ok(t, runCreateSyncgroup(client0Ctx, "sync0", sgName, "tb:foo", "", "root:s0", "root:s1"))

	ok(t, runSetupAppA(client1Ctx, "sync1"))
	ok(t, runJoinSyncgroup(client1Ctx, "sync1", sgName))
	ok(t, runVerifySyncgroupData(client1Ctx, "sync1", "foo", 0, 10, false))
	ok(t, runVerifyNonSyncgroupData(client1Ctx, "sync1", "bar"))
}

// TestV23SyncbasedGetDeltasMultiApp tests the sending of deltas between two
// Syncbase instances and their clients across multiple apps, databases, and
// tables.  The 1st client puts entries in multiple tables across multiple
// app databases then creates multiple syncgroups (one per database) over that
// data.  The 2nd client joins these syncgroups and reads all the data.
func TestV23SyncbasedGetDeltasMultiApp(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, v23test.Opts{})
	defer sh.Cleanup()
	sh.StartRootMountTable()

	server0Creds := sh.ForkCredentials("s0")
	client0Ctx := sh.ForkContext("c0")
	sh.StartSyncbase(server0Creds, "sync0", "",
		`{"Read": {"In":["root:c0"]}, "Write": {"In":["root:c0"]}}`)

	server1Creds := sh.ForkCredentials("s1")
	client1Ctx := sh.ForkContext("c1")
	sh.StartSyncbase(server1Creds, "sync1", "",
		`{"Read": {"In":["root:c1"]}, "Write": {"In":["root:c1"]}}`)

	sgNamePrefix := naming.Join("sync0", constants.SyncbaseSuffix)
	na, nd, nt := 2, 2, 2 // number of apps, dbs, tables

	ok(t, runSetupAppMulti(client0Ctx, "sync0", na, nd, nt))
	ok(t, runPopulateSyncgroupMulti(client0Ctx, "sync0", sgNamePrefix, na, nd, nt, "foo", "bar"))

	ok(t, runSetupAppMulti(client1Ctx, "sync1", na, nd, nt))
	ok(t, runJoinSyncgroupMulti(client1Ctx, "sync1", sgNamePrefix, na, nd))
	ok(t, runVerifySyncgroupDataMulti(client1Ctx, "sync1", na, nd, nt, "foo", "bar"))
}

// TestV23SyncgroupSync tests the syncing of syncgroup metadata. The 1st client
// creates the syncgroup SG1, and clients 2 and 3 join this syncgroup. All three
// clients must learn of the remaining two. Note that client 2 relies on
// syncgroup metadata syncing to learn of client 3 .
func TestV23SyncgroupSync(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, v23test.Opts{})
	defer sh.Cleanup()
	sh.StartRootMountTable()

	server0Creds := sh.ForkCredentials("s0")
	client0Ctx := sh.ForkContext("c0")
	sh.StartSyncbase(server0Creds, "sync0", "",
		`{"Read": {"In":["root:c0"]}, "Write": {"In":["root:c0"]}}`)

	server1Creds := sh.ForkCredentials("s1")
	client1Ctx := sh.ForkContext("c1")
	sh.StartSyncbase(server1Creds, "sync1", "",
		`{"Read": {"In":["root:c1"]}, "Write": {"In":["root:c1"]}}`)

	server2Creds := sh.ForkCredentials("s2")
	client2Ctx := sh.ForkContext("c2")
	sh.StartSyncbase(server2Creds, "sync2", "",
		`{"Read": {"In":["root:c2"]}, "Write": {"In":["root:c2"]}}`)

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")

	ok(t, runSetupAppA(client0Ctx, "sync0"))
	ok(t, runCreateSyncgroup(client0Ctx, "sync0", sgName, "tb:foo", "", "root:s0", "root:s1", "root:s2"))
	ok(t, runPopulateData(client0Ctx, "sync0", "foo", 0))

	ok(t, runSetupAppA(client1Ctx, "sync1"))
	ok(t, runJoinSyncgroup(client1Ctx, "sync1", sgName))

	ok(t, runSetupAppA(client2Ctx, "sync2"))
	ok(t, runJoinSyncgroup(client2Ctx, "sync2", sgName))

	ok(t, runGetSyncgroupMembers(client1Ctx, "sync1", sgName, 3))
	ok(t, runGetSyncgroupMembers(client2Ctx, "sync2", sgName, 3))

	ok(t, runVerifySyncgroupData(client1Ctx, "sync1", "foo", 0, 10, false))
	ok(t, runVerifySyncgroupData(client2Ctx, "sync2", "foo", 0, 10, false))
}

////////////////////////////////////
// Helpers.

// toSgPrefixes converts, for example, "a:b,c:" to
// [{TableName: "a", Row: "b"}, {TableName: "c", Row: ""}].
func toSgPrefixes(csv string) []wire.TableRow {
	strs := strings.Split(csv, ",")
	res := make([]wire.TableRow, len(strs))
	for i, v := range strs {
		parts := strings.SplitN(v, ":", 2)
		if len(parts) != 2 {
			panic(fmt.Sprintf("invalid prefix string: %q", v))
		}
		res[i] = wire.TableRow{TableName: parts[0], Row: parts[1]}
	}
	return res
}

// TODO(hpucha): Look into refactoring scan logic out of the helpers, and
// avoiding gets when we can scan.

func runSetupAppA(ctx *context.T, serviceName string) error {
	a := syncbase.NewService(serviceName).App("a")
	if err := a.Create(ctx, nil); err != nil {
		return err
	}
	d := a.NoSQLDatabase("d", nil)
	if err := d.Create(ctx, nil); err != nil {
		return err
	}
	tb := d.Table(testTable)
	if err := tb.Create(ctx, nil); err != nil {
		return err
	}

	return nil
}

func runCreateSyncgroup(ctx *context.T, serviceName, sgName, sgPrefixes, mtName string, sgBlessings ...string) error {
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)

	mtNames := v23.GetNamespace(ctx).Roots()
	if mtName != "" {
		mtNames = []string{mtName}
	}

	spec := wire.SyncgroupSpec{
		Description: "test syncgroup sg",
		Perms:       perms(sgBlessings...),
		Prefixes:    toSgPrefixes(sgPrefixes),
		MountTables: mtNames,
	}

	sg := d.Syncgroup(sgName)
	info := wire.SyncgroupMemberInfo{SyncPriority: 8}
	if err := sg.Create(ctx, spec, info); err != nil {
		return fmt.Errorf("Create SG %q failed: %v\n", sgName, err)
	}
	return nil
}

func runJoinSyncgroup(ctx *context.T, serviceName, sgName string) error {
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)
	sg := d.Syncgroup(sgName)
	info := wire.SyncgroupMemberInfo{SyncPriority: 10}
	if _, err := sg.Join(ctx, info); err != nil {
		return fmt.Errorf("Join SG %q failed: %v\n", sgName, err)
	}
	return nil
}

func runSetSyncgroupSpec(ctx *context.T, serviceName, sgName, sgDesc, sgPrefixes string, sgBlessings ...string) error {
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)

	sg := d.Syncgroup(sgName)

	mtNames := v23.GetNamespace(ctx).Roots()
	spec := wire.SyncgroupSpec{
		Description: sgDesc,
		Prefixes:    toSgPrefixes(sgPrefixes),
		Perms:       perms(sgBlessings...),
		MountTables: mtNames,
	}

	if err := sg.SetSpec(ctx, spec, ""); err != nil {
		return fmt.Errorf("SetSpec SG %q failed: %v\n", sgName, err)
	}
	return nil
}

func runGetSyncgroupSpec(ctx *context.T, serviceName, sgName, wantDesc, wantPrefixes string, wantBlessings ...string) error {
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)

	sg := d.Syncgroup(sgName)

	wantPfxs := toSgPrefixes(wantPrefixes)
	wantPerms := perms(wantBlessings...)

	var spec wire.SyncgroupSpec
	var err error
	for i := 0; i < 8; i++ {
		time.Sleep(500 * time.Millisecond)
		spec, _, err = sg.GetSpec(ctx)
		if err != nil {
			return fmt.Errorf("GetSpec SG %q failed: %v\n", sgName, err)
		}
		if spec.Description == wantDesc {
			break
		}
	}
	if spec.Description != wantDesc || !reflect.DeepEqual(spec.Prefixes, wantPfxs) || !reflect.DeepEqual(spec.Perms, wantPerms) {
		return fmt.Errorf("GetSpec SG %q failed: description got %v, want %v, prefixes got %v, want %v, perms got %v, want %v\n", sgName, spec.Description, wantDesc, spec.Prefixes, wantPfxs, spec.Perms, wantPerms)
	}
	return nil
}

func runGetSyncgroupMembers(ctx *context.T, serviceName, sgName string, wantMembers uint64) error {
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)

	sg := d.Syncgroup(sgName)

	var gotMembers uint64

	for i := 0; i < 8; i++ {
		time.Sleep(500 * time.Millisecond)
		members, err := sg.GetMembers(ctx)
		if err != nil {
			return fmt.Errorf("GetMembers SG %q failed: %v\n", sgName, err)
		}
		gotMembers = uint64(len(members))
		if wantMembers == gotMembers {
			break
		}
	}
	if wantMembers != gotMembers {
		return fmt.Errorf("GetMembers SG %q failed: members got %v, want %v\n", sgName, gotMembers, wantMembers)
	}
	return nil
}

func runPopulateData(ctx *context.T, serviceName, keyPrefix string, start uint64) error {
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Do Puts.
	tb := d.Table(testTable)

	for i := start; i < start+10; i++ {
		key := fmt.Sprintf("%s%d", keyPrefix, i)
		r := tb.Row(key)
		if err := r.Put(ctx, "testkey"+key); err != nil {
			return fmt.Errorf("r.Put() failed: %v\n", err)
		}
	}
	return nil
}

func runPopulateNonVomData(ctx *context.T, serviceName, keyPrefix string, start uint64) error {
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Do Puts.
	tb := d.Table("tb")

	for i := start; i < start+10; i++ {
		key := fmt.Sprintf("%s%d", keyPrefix, i)
		r := tb.Row(key)
		c := wire.RowClient(r.FullName())
		val := []byte("nonvomtestkey" + key)
		if err := c.Put(ctx, -1, val); err != nil {
			return fmt.Errorf("c.Put() failed: %v\n", err)
		}
	}
	return nil
}

func runUpdateData(ctx *context.T, serviceName string, start uint64) error {
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Do Puts.
	tb := d.Table(testTable)

	for i := start; i < start+5; i++ {
		key := fmt.Sprintf("foo%d", i)
		r := tb.Row(key)
		if err := r.Put(ctx, "testkey"+serviceName+key); err != nil {
			return fmt.Errorf("r.Put() failed: %v\n", err)
		}
	}

	return nil
}

func runDeleteData(ctx *context.T, serviceName string, start uint64) error {
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Do Puts.
	tb := d.Table(testTable)

	for i := start; i < start+5; i++ {
		key := fmt.Sprintf("foo%d", i)
		r := tb.Row(key)
		if err := r.Delete(ctx); err != nil {
			return fmt.Errorf("r.Delete() failed: %v\n", err)
		}
	}

	return nil
}

func runSetPrefixPermissions(ctx *context.T, serviceName, keyPrefix string, aclBlessings ...string) error {
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Set acl.
	tb := d.Table(testTable)

	if err := tb.SetPrefixPermissions(ctx, nosql.Prefix(keyPrefix), perms(aclBlessings...)); err != nil {
		return fmt.Errorf("tb.SetPrefixPermissions() failed: %v\n", err)
	}

	return nil
}

func runVerifySyncgroupData(ctx *context.T, serviceName, keyPrefix string, start, count uint64, skipScan bool) error {
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Wait for a bit (up to 4 sec) until the last key appears.
	tb := d.Table(testTable)

	lastKey := fmt.Sprintf("%s%d", keyPrefix, start+count-1)

	r := tb.Row(lastKey)
	for i := 0; i < 8; i++ {
		time.Sleep(500 * time.Millisecond)
		var value string
		if err := r.Get(ctx, &value); err == nil {
			break
		}
	}

	// Verify that all keys and values made it correctly.
	for i := start; i < start+count; i++ {
		key := fmt.Sprintf("%s%d", keyPrefix, i)
		r := tb.Row(key)
		var got string
		if err := r.Get(ctx, &got); err != nil {
			return fmt.Errorf("r.Get() failed: %v\n", err)
		}
		want := "testkey" + key
		if got != want {
			return fmt.Errorf("unexpected value: got %q, want %q\n", got, want)
		}
	}

	if !skipScan {
		// Re-verify using a scan operation.
		stream := tb.Scan(ctx, nosql.Prefix(keyPrefix))
		for i := 0; stream.Advance(); i++ {
			want := fmt.Sprintf("%s%d", keyPrefix, i)
			got := stream.Key()
			if got != want {
				return fmt.Errorf("unexpected key in scan: got %q, want %q\n", got, want)
			}
			want = "testkey" + want
			if err := stream.Value(&got); err != nil {
				return fmt.Errorf("cannot fetch value in scan: %v\n", err)
			}
			if got != want {
				return fmt.Errorf("unexpected value in scan: got %q, want %q\n", got, want)
			}
		}

		if err := stream.Err(); err != nil {
			return fmt.Errorf("scan stream error: %v\n", err)
		}
	}
	return nil
}

func runVerifySyncgroupDataWithWatch(ctx *context.T, serviceName, keyPrefix string, count int, expectDelete bool, beforeSyncMarker watch.ResumeMarker) error {
	if count == 0 {
		return fmt.Errorf("count cannot be 0: got %d", count)
	}
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	stream, err := d.Watch(ctxWithTimeout, testTable, keyPrefix, beforeSyncMarker)
	if err != nil {
		return fmt.Errorf("watch error: %v\n", err)
	}

	var changes []nosql.WatchChange
	for i := 0; stream.Advance() && i < count; i++ {
		if err := stream.Err(); err != nil {
			return fmt.Errorf("watch stream error: %v\n", err)
		}
		changes = append(changes, stream.Change())
	}

	sort.Sort(ByRow(changes))

	if got, want := len(changes), count; got != want {
		return fmt.Errorf("unexpected number of changes: got %d, want %d", got, want)
	}

	for i, change := range changes {
		if got, want := change.Table, "tb"; got != want {
			return fmt.Errorf("unexpected watch table: got %q, want %q", got, want)
		}
		if got, want := change.Row, fmt.Sprintf("%s%d", keyPrefix, i); got != want {
			return fmt.Errorf("unexpected watch row: got %q, want %q", got, want)
		}
		if got, want := change.FromSync, true; got != want {
			return fmt.Errorf("unexpected FromSync value: got %t, want %t", got, want)
		}
		if expectDelete {
			if got, want := change.ChangeType, nosql.DeleteChange; got != want {
				return fmt.Errorf("unexpected watch change type: got %q, want %q", got, want)
			}
			return nil
		}
		var result string
		if got, want := change.ChangeType, nosql.PutChange; got != want {
			return fmt.Errorf("unexpected watch change type: got %q, want %q", got, want)
		}
		if err := vom.Decode(change.ValueBytes, &result); err != nil {
			return fmt.Errorf("couldn't decode watch value: %v", err)
		}
		if got, want := result, fmt.Sprintf("testkey%s%d", keyPrefix, i); got != want {
			return fmt.Errorf("unexpected watch value: got %q, want %q", got, want)
		}
	}
	return nil
}

func runVerifySyncgroupNonVomData(ctx *context.T, serviceName, keyPrefix string, start, count uint64) error {
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Wait for a bit (up to 4 sec) until the last key appears.
	tb := d.Table("tb")

	lastKey := fmt.Sprintf("%s%d", keyPrefix, start+count-1)

	r := tb.Row(lastKey)
	c := wire.RowClient(r.FullName())
	for i := 0; i < 8; i++ {
		time.Sleep(500 * time.Millisecond)
		if _, err := c.Get(ctx, -1); err == nil {
			break
		}
	}

	// Verify that all keys and values made it correctly.
	for i := start; i < start+count; i++ {
		key := fmt.Sprintf("%s%d", keyPrefix, i)
		r := tb.Row(key)
		c := wire.RowClient(r.FullName())
		var got string
		val, err := c.Get(ctx, -1)
		if err != nil {
			return fmt.Errorf("c.Get() failed: %v\n", err)
		}
		got = string(val)
		want := "nonvomtestkey" + key
		if got != want {
			return fmt.Errorf("unexpected value: got %q, want %q\n", got, want)
		}
	}
	return nil
}

func runVerifyDeletedData(ctx *context.T, serviceName, keyPrefix string) error {
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Wait for a bit for deletions to propagate.
	tb := d.Table(testTable)

	r := tb.Row("foo4")
	for i := 0; i < 8; i++ {
		time.Sleep(500 * time.Millisecond)
		var value string
		if err := r.Get(ctx, &value); verror.ErrorID(err) == verror.ErrNoExist.ID {
			break
		}
	}

	// Verify using a scan operation.
	stream := tb.Scan(ctx, nosql.Prefix(keyPrefix))
	count := 0
	for i := 5; stream.Advance(); i++ {
		want := fmt.Sprintf("%s%d", keyPrefix, i)
		got := stream.Key()
		if got != want {
			return fmt.Errorf("unexpected key in scan: got %q, want %q\n", got, want)
		}
		want = "testkey" + want
		if err := stream.Value(&got); err != nil {
			return fmt.Errorf("cannot fetch value in scan: %v\n", err)
		}
		if got != want {
			return fmt.Errorf("unexpected value in scan: got %q, want %q\n", got, want)
		}
		count++
	}

	if err := stream.Err(); err != nil {
		return fmt.Errorf("scan stream error: %v\n", err)
	}

	if count != 5 {
		return fmt.Errorf("scan stream count error: %v\n", count)
	}

	return nil
}

func runVerifyConflictResolution(ctx *context.T, serviceName string) error {
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)
	tb := d.Table(testTable)

	wantData := []struct {
		start  uint64
		count  uint64
		valPfx []string
	}{
		{0, 5, []string{"testkey"}},
		{5, 5, []string{"testkeysync0", "testkeysync1"}},
	}

	// Verify that all keys and values made it correctly.
	for _, d := range wantData {
		for i := d.start; i < d.start+d.count; i++ {
			key := fmt.Sprintf("foo%d", i)
			r := tb.Row(key)
			var got string
			if err := r.Get(ctx, &got); err != nil {
				return fmt.Errorf("r.Get() failed: %v\n", err)
			}
			match := 0
			for _, p := range d.valPfx {
				want := p + key
				if got == want {
					match++
				}
			}
			if match != 1 {
				return fmt.Errorf("unexpected value: got %q, match %v, want %v\n", got, match, d.valPfx)
			}
		}
	}
	return nil
}

func runVerifyNonSyncgroupData(ctx *context.T, serviceName, keyPrefix string) error {
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)
	tb := d.Table(testTable)

	// Verify through a scan that none of that data exists.
	count := 0
	stream := tb.Scan(ctx, nosql.Prefix(keyPrefix))
	for stream.Advance() {
		count++
	}

	if err := stream.Err(); err != nil {
		return fmt.Errorf("scan stream error: %v\n", err)
	}
	if count > 0 {
		return fmt.Errorf("found %d entries in %s prefix that should not be there\n", count, keyPrefix)
	}

	return nil
}

func runVerifyLocalAndRemoteData(ctx *context.T, serviceName string) error {
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)
	tb := d.Table(testTable)

	// Wait for a bit (up to 4 sec) until the last key appears.
	r := tb.Row("foo19")
	for i := 0; i < 8; i++ {
		time.Sleep(500 * time.Millisecond)
		var value string
		if err := r.Get(ctx, &value); err == nil {
			break
		}
	}

	wantData := []struct {
		start  uint64
		count  uint64
		valPfx string
	}{
		{0, 5, "testkey"},
		{5, 5, "testkeysync1"},
		{10, 10, "testkey"},
	}

	// Verify that all keys and values made it correctly.
	for _, d := range wantData {
		for i := d.start; i < d.start+d.count; i++ {
			key := fmt.Sprintf("foo%d", i)
			r := tb.Row(key)
			var got string
			if err := r.Get(ctx, &got); err != nil {
				return fmt.Errorf("r.Get() failed: %v\n", err)
			}
			want := d.valPfx + key
			if got != want {
				return fmt.Errorf("unexpected value: got %q, want %q\n", got, want)
			}
		}
	}
	return nil
}

func runVerifyLostAccess(ctx *context.T, serviceName, keyPrefix string, start, count uint64) error {
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Wait for a bit (up to 4 sec) until the last key disappears.
	tb := d.Table(testTable)

	lastKey := fmt.Sprintf("%s%d", keyPrefix, start+count-1)

	r := tb.Row(lastKey)
	for i := 0; i < 8; i++ {
		time.Sleep(500 * time.Millisecond)
		var value string
		if err := r.Get(ctx, &value); verror.ErrorID(err) == verror.ErrNoAccess.ID {
			break
		}
	}

	// Verify that all keys and values have lost access.
	for i := start; i < start+count; i++ {
		key := fmt.Sprintf("%s%d", keyPrefix, i)
		r := tb.Row(key)
		var got string
		if err := r.Get(ctx, &got); verror.ErrorID(err) != verror.ErrNoAccess.ID {
			return fmt.Errorf("r.Get() didn't fail: %v\n", err)
		}
	}

	return nil
}

func runVerifyNestedSyncgroupData(ctx *context.T, serviceName string) error {
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Wait for a bit (up to 8 sec) until the last key appears. This chosen
	// time interval is dependent on how fast the membership view is
	// refreshed (currently 2 seconds) and how frequently we sync with peers
	// (every 50 ms), and then adding a substantial safeguard to it to
	// ensure that the test is not flaky even in somewhat abnormal
	// conditions. Note that we wait longer than the 2 node tests since more
	// nodes implies more pair-wise communication before achieving steady
	// state.
	tb := d.Table(testTable)

	r := tb.Row("f9")
	for i := 0; i < 8; i++ {
		time.Sleep(1 * time.Second)
		var value string
		if err := r.Get(ctx, &value); err == nil {
			break
		}
	}

	// Verify that all keys and values made it correctly.
	pfxs := []string{"foo", "f"}
	for _, p := range pfxs {
		for i := 0; i < 10; i++ {
			key := fmt.Sprintf("%s%d", p, i)
			r := tb.Row(key)
			var got string
			if err := r.Get(ctx, &got); err != nil {
				return fmt.Errorf("r.Get() failed: %v\n", err)
			}
			want := "testkey" + key
			if got != want {
				return fmt.Errorf("unexpected value: got %q, want %q\n", got, want)
			}
		}
	}
	return nil
}

func runSetupAppMulti(ctx *context.T, serviceName string, numApps, numDbs, numTbs int) error {
	svc := syncbase.NewService(serviceName)

	for i := 0; i < numApps; i++ {
		appName := fmt.Sprintf("a%d", i)
		a := svc.App(appName)
		a.Create(ctx, nil)

		for j := 0; j < numDbs; j++ {
			dbName := fmt.Sprintf("d%d", j)
			d := a.NoSQLDatabase(dbName, nil)
			d.Create(ctx, nil)

			for k := 0; k < numTbs; k++ {
				tbName := fmt.Sprintf("tb%d", k)
				d.Table(tbName).Create(ctx, nil)
			}
		}
	}

	return nil
}

func runPopulateSyncgroupMulti(ctx *context.T, serviceName, sgNamePrefix string, numApps, numDbs, numTbs int, prefixes ...string) error {
	mtNames := v23.GetNamespace(ctx).Roots()

	svc := syncbase.NewService(serviceName)

	// For each app...
	for i := 0; i < numApps; i++ {
		appName := fmt.Sprintf("a%d", i)
		a := svc.App(appName)

		// For each database...
		for j := 0; j < numDbs; j++ {
			dbName := fmt.Sprintf("d%d", j)
			d := a.NoSQLDatabase(dbName, nil)

			// For each table, pre-populate entries on each prefix.
			// Also determine the syncgroup prefixes.
			var sgPrefixes []string
			for k := 0; k < numTbs; k++ {
				tbName := fmt.Sprintf("tb%d", k)
				tb := d.Table(tbName)

				for _, pfx := range prefixes {
					p := fmt.Sprintf("%s:%s", tbName, pfx)
					sgPrefixes = append(sgPrefixes, p)

					for n := 0; n < 10; n++ {
						key := fmt.Sprintf("%s%d", pfx, n)
						r := tb.Row(key)
						if err := r.Put(ctx, "testkey"+key); err != nil {
							return fmt.Errorf("r.Put() failed: %v\n", err)
						}
					}
				}
			}

			// Create one syncgroup per database across all tables
			// and prefixes.
			sgName := naming.Join(sgNamePrefix, appName, dbName)
			spec := wire.SyncgroupSpec{
				Description: fmt.Sprintf("test sg %s/%s", appName, dbName),
				Perms:       perms("root:s0", "root:s1"),
				Prefixes:    toSgPrefixes(strings.Join(sgPrefixes, ",")),
				MountTables: mtNames,
			}

			sg := d.Syncgroup(sgName)
			info := wire.SyncgroupMemberInfo{SyncPriority: 8}
			if err := sg.Create(ctx, spec, info); err != nil {
				return fmt.Errorf("Create SG %q failed: %v\n", sgName, err)
			}
		}
	}

	return nil
}

func runJoinSyncgroupMulti(ctx *context.T, serviceName, sgNamePrefix string, numApps, numDbs int) error {
	svc := syncbase.NewService(serviceName)

	for i := 0; i < numApps; i++ {
		appName := fmt.Sprintf("a%d", i)
		a := svc.App(appName)

		for j := 0; j < numDbs; j++ {
			dbName := fmt.Sprintf("d%d", j)
			d := a.NoSQLDatabase(dbName, nil)

			sgName := naming.Join(sgNamePrefix, appName, dbName)
			sg := d.Syncgroup(sgName)
			info := wire.SyncgroupMemberInfo{SyncPriority: 10}
			if _, err := sg.Join(ctx, info); err != nil {
				return fmt.Errorf("Join SG %q failed: %v\n", sgName, err)
			}
		}
	}

	return nil
}

func runVerifySyncgroupDataMulti(ctx *context.T, serviceName string, numApps, numDbs, numTbs int, prefixes ...string) error {
	svc := syncbase.NewService(serviceName)

	time.Sleep(20 * time.Second)

	for i := 0; i < numApps; i++ {
		appName := fmt.Sprintf("a%d", i)
		a := svc.App(appName)

		for j := 0; j < numDbs; j++ {
			dbName := fmt.Sprintf("d%d", j)
			d := a.NoSQLDatabase(dbName, nil)

			for k := 0; k < numTbs; k++ {
				tbName := fmt.Sprintf("tb%d", k)
				tb := d.Table(tbName)

				for _, pfx := range prefixes {
					for n := 0; n < 10; n++ {
						key := fmt.Sprintf("%s%d", pfx, n)
						r := tb.Row(key)
						var got string
						if err := r.Get(ctx, &got); err != nil {
							return fmt.Errorf("r.Get() failed: %v\n", err)
						}
						want := "testkey" + key
						if got != want {
							return fmt.Errorf("unexpected value: got %q, want %q\n",
								got, want)
						}
					}
				}
			}
		}
	}

	return nil
}

func getResumeMarker(ctx *context.T, serviceName string) (watch.ResumeMarker, error) {
	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)
	return d.GetResumeMarker(ctx)
}

func ok(t *testing.T, err error) {
	if err != nil {
		debug.PrintStack()
		t.Fatal(err)
	}
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}

// ByRow implements sort.Interface for []nosql.WatchChange based on the Row field.
type ByRow []nosql.WatchChange

func (c ByRow) Len() int           { return len(c) }
func (c ByRow) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c ByRow) Less(i, j int) bool { return c[i].Row < c[j].Row }
