// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"v.io/v23"
	"v.io/v23/naming"
	wire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/nosql"
	"v.io/v23/verror"
	"v.io/x/ref"
	_ "v.io/x/ref/runtime/factories/generic"
	constants "v.io/x/ref/services/syncbase/server/util"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test/modules"
	"v.io/x/ref/test/v23tests"
)

const (
	testTable = "tb"
)

// NOTE(sadovsky): These tests take a very long time to run - nearly 4 minutes
// on my Macbook Pro! Various instances of time.Sleep() below likely contribute
// to the problem.

//go:generate jiri test generate

// V23TestSyncbasedJoinSyncgroup tests the creation and joining of a syncgroup.
// Client0 creates a syncgroup at Syncbase0. Client1 requests to join the
// syncgroup at Syncbase1. Syncbase1 in turn requests Syncbase0 to join the
// syncgroup.
func V23TestSyncbasedJoinSyncgroup(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	server0Creds, _ := t.Shell().NewChildCredentials("s0")
	client0Creds, _ := t.Shell().NewChildCredentials("c0")
	cleanSync0 := tu.StartSyncbased(t, server0Creds, "sync0", "",
		`{"Read": {"In":["root/c0"]}, "Write": {"In":["root/c0"]}}`)
	defer cleanSync0()

	server1Creds, _ := t.Shell().NewChildCredentials("s1")
	client1Creds, _ := t.Shell().NewChildCredentials("c1")
	cleanSync1 := tu.StartSyncbased(t, server1Creds, "sync1", "",
		`{"Read": {"In":["root/c1"]}, "Write": {"In":["root/c1"]}}`)
	defer cleanSync1()

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")

	tu.RunClient(t, client0Creds, runSetupAppA, "sync0")
	tu.RunClient(t, client0Creds, runCreateSyncgroup, "sync0", sgName, "tb:foo", "", "root/s0", "root/s1")

	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncgroup, "sync1", sgName)
}

// V23TestSyncbasedGetDeltas tests the sending of deltas between two Syncbase
// instances and their clients.  The 1st client creates a syncgroup and puts
// some database entries in it.  The 2nd client joins that syncgroup and reads
// the database entries.  This verifies the end-to-end synchronization of data
// along the path: client0--Syncbase0--Syncbase1--client1.
func V23TestSyncbasedGetDeltas(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	server0Creds, _ := t.Shell().NewChildCredentials("s0")
	client0Creds, _ := t.Shell().NewChildCredentials("c0")
	cleanSync0 := tu.StartSyncbased(t, server0Creds, "sync0", "",
		`{"Read": {"In":["root/c0"]}, "Write": {"In":["root/c0"]}}`)
	defer cleanSync0()

	server1Creds, _ := t.Shell().NewChildCredentials("s1")
	client1Creds, _ := t.Shell().NewChildCredentials("c1")
	cleanSync1 := tu.StartSyncbased(t, server1Creds, "sync1", "",
		`{"Read": {"In":["root/c1"]}, "Write": {"In":["root/c1"]}}`)
	defer cleanSync1()

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")

	tu.RunClient(t, client0Creds, runSetupAppA, "sync0")
	tu.RunClient(t, client0Creds, runCreateSyncgroup, "sync0", sgName, "tb:foo,tb:bar", "", "root/s0", "root/s1")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foo", "0")
	tu.RunClient(t, client0Creds, runPopulateNonVomData, "sync0", "bar", "0")

	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncgroup, "sync1", sgName)
	tu.RunClient(t, client1Creds, runVerifySyncgroupData, "sync1", "foo", "0", "10", "false")
	tu.RunClient(t, client1Creds, runVerifySyncgroupNonVomData, "sync1", "bar", "0", "10")
}

// V23TestSyncbasedExchangeDeltasNeighborhood tests the sending of deltas
// between two Syncbase instances and their clients via neighborhood discovery,
// without the syncgroup mount table being available.
func V23TestSyncbasedExchangeDeltasNeighborhood(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	server0Creds, _ := t.Shell().NewChildCredentials("s0")
	client0Creds, _ := t.Shell().NewChildCredentials("c0")
	cleanSync0 := tu.StartSyncbased(t, server0Creds, "sync0", "",
		`{"Read": {"In":["root/c0"]}, "Write": {"In":["root/c0"]}}`)
	defer cleanSync0()

	server1Creds, _ := t.Shell().NewChildCredentials("s1")
	client1Creds, _ := t.Shell().NewChildCredentials("c1")
	cleanSync1 := tu.StartSyncbased(t, server1Creds, "sync1", "",
		`{"Read": {"In":["root/c1"]}, "Write": {"In":["root/c1"]}}`)
	defer cleanSync1()

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")

	tu.RunClient(t, client0Creds, runSetupAppA, "sync0")
	// We are using a fake mount table so that the peers are forced to find
	// each other over neighborhood.
	tu.RunClient(t, client0Creds, runCreateSyncgroup, "sync0", sgName, "tb:foo", "/mttable", "root/s0", "root/s1")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foo", "0")

	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncgroup, "sync1", sgName)
	tu.RunClient(t, client1Creds, runVerifySyncgroupData, "sync1", "foo", "0", "10", "false")

	tu.RunClient(t, client1Creds, runPopulateData, "sync1", "foo", "10")
	tu.RunClient(t, client0Creds, runVerifySyncgroupData, "sync0", "foo", "0", "20", "true")
}

// V23TestSyncbasedGetDeltasWithDel tests the sending of deltas between two
// Syncbase instances and their clients. The 1st client creates a syncgroup and
// puts some database entries in it. The 2nd client joins that syncgroup and
// reads the database entries. The 1st client then deletes a portion of this
// data, and adds new entries. The 2nd client verifies that these changes are
// correctly synced. This verifies the end-to-end synchronization of data along
// the path: client0--Syncbase0--Syncbase1--client1 with a workload of puts and
// deletes.
func V23TestSyncbasedGetDeltasWithDel(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	server0Creds, _ := t.Shell().NewChildCredentials("s0")
	client0Creds, _ := t.Shell().NewChildCredentials("c0")
	cleanSync0 := tu.StartSyncbased(t, server0Creds, "sync0", "",
		`{"Read": {"In":["root/c0"]}, "Write": {"In":["root/c0"]}}`)
	defer cleanSync0()

	server1Creds, _ := t.Shell().NewChildCredentials("s1")
	client1Creds, _ := t.Shell().NewChildCredentials("c1")
	cleanSync1 := tu.StartSyncbased(t, server1Creds, "sync1", "",
		`{"Read": {"In":["root/c1"]}, "Write": {"In":["root/c1"]}}`)
	defer cleanSync1()

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")

	tu.RunClient(t, client0Creds, runSetupAppA, "sync0")
	tu.RunClient(t, client0Creds, runCreateSyncgroup, "sync0", sgName, "tb:foo,tb:bar", "", "root/s0", "root/s1")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foo", "0")

	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncgroup, "sync1", sgName)
	tu.RunClient(t, client1Creds, runVerifySyncgroupData, "sync1", "foo", "0", "10", "false")

	tu.RunClient(t, client0Creds, runDeleteData, "sync0", "foo", "0")
	tu.RunClient(t, client0Creds, runVerifyDeletedData, "sync0", "foo")
	tu.RunClient(t, client1Creds, runVerifyDeletedData, "sync1", "foo")

	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "bar", "0")
	tu.RunClient(t, client1Creds, runVerifySyncgroupData, "sync1", "bar", "0", "10", "false")
}

// V23TestSyncbasedCompEval is a comprehensive sniff test for core sync
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
func V23TestSyncbasedCompEval(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	server0Creds, _ := t.Shell().NewChildCredentials("s0")
	client0Creds, _ := t.Shell().NewChildCredentials("c0")
	server0RDir, err := ioutil.TempDir("", "sync0")
	if err != nil {
		tu.V23Fatalf(t, "can't create temp dir: %v", err)
	}
	cleanSync0 := tu.StartSyncbased(t, server0Creds, "sync0", server0RDir,
		`{"Read": {"In":["root/c0"]}, "Write": {"In":["root/c0"]}}`)

	server1Creds, _ := t.Shell().NewChildCredentials("s1")
	client1Creds, _ := t.Shell().NewChildCredentials("c1")
	server1RDir, err := ioutil.TempDir("", "sync1")
	if err != nil {
		tu.V23Fatalf(t, "can't create temp dir: %v", err)
	}
	cleanSync1 := tu.StartSyncbased(t, server1Creds, "sync1", server1RDir,
		`{"Read": {"In":["root/c1"]}, "Write": {"In":["root/c1"]}}`)

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")

	tu.RunClient(t, client0Creds, runSetupAppA, "sync0")
	tu.RunClient(t, client0Creds, runCreateSyncgroup, "sync0", sgName, "tb:foo", "", "root/s0", "root/s1")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foo", "0")

	// This is a decoy syncgroup that no other Syncbase joins, but is on the
	// same database as the first syncgroup. Populating it after the first
	// syncgroup causes the database generations to go up, but the joiners
	// on the first syncgroup do not get any data belonging to this
	// syncgroup. This triggers the handling of filtered log records in the
	// restartability code.
	sgName1 := naming.Join("sync0", constants.SyncbaseSuffix, "SG2")
	tu.RunClient(t, client0Creds, runCreateSyncgroup, "sync0", sgName1, "tb:bar", "", "root/s0", "root/s1")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "bar", "0")

	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncgroup, "sync1", sgName)
	tu.RunClient(t, client1Creds, runVerifySyncgroupData, "sync1", "foo", "0", "10", "false")

	// Shutdown and restart Syncbase instances.
	cleanSync0()
	cleanSync1()

	cleanSync0 = tu.StartSyncbased(t, server0Creds, "sync0", server0RDir,
		`{"Read": {"In":["root/c0"]}, "Write": {"In":["root/c0"]}}`)
	cleanSync1 = tu.StartSyncbased(t, server1Creds, "sync1", server1RDir,
		`{"Read": {"In":["root/c1"]}, "Write": {"In":["root/c1"]}}`)

	tu.RunClient(t, client0Creds, runSetSyncgroupSpec, "sync0", sgName, "v2", "tb:foo", "root/s0", "root/s1", "root/s3")
	tu.RunClient(t, client1Creds, runGetSyncgroupSpec, "sync1", sgName, "v2", "tb:foo", "root/s0", "root/s1", "root/s3")

	tu.RunClient(t, client1Creds, runUpdateData, "sync1", "5")
	tu.RunClient(t, client1Creds, runPopulateData, "sync1", "foo", "10")
	tu.RunClient(t, client1Creds, runSetSyncgroupSpec, "sync1", sgName, "v3", "tb:foo", "root/s0", "root/s1", "root/s4")

	tu.RunClient(t, client0Creds, runVerifyLocalAndRemoteData, "sync0")
	tu.RunClient(t, client0Creds, runGetSyncgroupSpec, "sync0", sgName, "v3", "tb:foo", "root/s0", "root/s1", "root/s4")

	// Shutdown and restart Syncbase instances.
	cleanSync0()
	cleanSync1()

	cleanSync0 = tu.StartSyncbased(t, server0Creds, "sync0", server0RDir,
		`{"Read": {"In":["root/c0"]}, "Write": {"In":["root/c0"]}}`)
	cleanSync1 = tu.StartSyncbased(t, server1Creds, "sync1", server1RDir,
		`{"Read": {"In":["root/c1"]}, "Write": {"In":["root/c1"]}}`)

	tu.RunClient(t, client0Creds, runGetSyncgroupSpec, "sync0", sgName, "v3", "tb:foo", "root/s0", "root/s1", "root/s4")
	tu.RunClient(t, client1Creds, runGetSyncgroupSpec, "sync1", sgName, "v3", "tb:foo", "root/s0", "root/s1", "root/s4")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foo", "20")
	tu.RunClient(t, client1Creds, runVerifySyncgroupData, "sync1", "foo", "20", "10", "true")

	cleanSync0()
	cleanSync1()

	rdirs := []string{server0RDir, server1RDir}
	for _, r := range rdirs {
		if err := os.RemoveAll(r); err != nil {
			tu.V23Fatalf(t, "can't remove dir %v: %v", r, err)
		}
	}
}

// V23TestSyncbasedExchangeDeltasWithAcls tests the exchange of deltas including
// acls between two Syncbase instances and their clients.  The 1st client
// creates a syncgroup at "foo", sets an acl, and puts some database entries in
// it.  The 2nd client joins that syncgroup and reads the database entries.  The
// 2nd client then adds a prefix acl with access to only itself at "foobar".
// The 1st client should be unable to access the subset of keys under
// "foobar". The 2nd client then modifies the prefix acl at "foobar" with access
// to both clients. The 1st client should regain access.
func V23TestSyncbasedExchangeDeltasWithAcls(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	server0Creds, _ := t.Shell().NewChildCredentials("s0")
	client0Creds, _ := t.Shell().NewChildCredentials("c0")
	cleanSync0 := tu.StartSyncbased(t, server0Creds, "sync0", "",
		`{"Read": {"In":["root/c0"]}, "Write": {"In":["root/c0"]}, "Admin": {"In":["root/c0"]}}`)
	defer cleanSync0()

	server1Creds, _ := t.Shell().NewChildCredentials("s1")
	client1Creds, _ := t.Shell().NewChildCredentials("c1")
	cleanSync1 := tu.StartSyncbased(t, server1Creds, "sync1", "",
		`{"Read": {"In":["root/c1"]}, "Write": {"In":["root/c1"]}, "Admin": {"In":["root/c1"]}}`)
	defer cleanSync1()

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")

	tu.RunClient(t, client0Creds, runSetupAppA, "sync0")
	tu.RunClient(t, client0Creds, runCreateSyncgroup, "sync0", sgName, "tb:foo", "", "root/s0", "root/s1")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foobarbaz", "0")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foo", "0")
	tu.RunClient(t, client0Creds, runSetPrefixPermissions, "sync0", "foo", "root/c0", "root/c1")

	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncgroup, "sync1", sgName)
	tu.RunClient(t, client1Creds, runVerifySyncgroupData, "sync1", "foobarbaz", "0", "10", "false")
	tu.RunClient(t, client1Creds, runVerifySyncgroupData, "sync1", "foo", "0", "10", "true")

	tu.RunClient(t, client1Creds, runSetPrefixPermissions, "sync1", "foobar", "root/c1")
	tu.RunClient(t, client0Creds, runVerifyLostAccess, "sync0", "foobarbaz", "0", "10")
	tu.RunClient(t, client0Creds, runVerifySyncgroupData, "sync0", "foo", "0", "10", "true")

	tu.RunClient(t, client1Creds, runSetPrefixPermissions, "sync1", "foobar", "root/c0", "root/c1")
	tu.RunClient(t, client0Creds, runVerifySyncgroupData, "sync0", "foobarbaz", "0", "10", "false")
}

// V23 TestSyncbasedExchangeDeltasWithConflicts tests the exchange of deltas
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
func V23TestSyncbasedExchangeDeltasWithConflicts(t *v23tests.T) {
	// Run it multiple times to exercise different interactions between sync
	// and local updates that change every run due to timing.
	for i := 0; i < 10; i++ {
		testSyncbasedExchangeDeltasWithConflicts(t)
	}
}

func testSyncbasedExchangeDeltasWithConflicts(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	server0Creds, _ := t.Shell().NewChildCredentials("s0")
	client0Creds, _ := t.Shell().NewChildCredentials("c0")
	cleanSync0 := tu.StartSyncbased(t, server0Creds, "sync0", "",
		`{"Read": {"In":["root/c0"]}, "Write": {"In":["root/c0"]}}`)
	defer cleanSync0()

	server1Creds, _ := t.Shell().NewChildCredentials("s1")
	client1Creds, _ := t.Shell().NewChildCredentials("c1")
	cleanSync1 := tu.StartSyncbased(t, server1Creds, "sync1", "",
		`{"Read": {"In":["root/c1"]}, "Write": {"In":["root/c1"]}}`)
	defer cleanSync1()

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")

	tu.RunClient(t, client0Creds, runSetupAppA, "sync0")
	tu.RunClient(t, client0Creds, runCreateSyncgroup, "sync0", sgName, "tb:foo", "", "root/s0", "root/s1")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foo", "0")

	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncgroup, "sync1", sgName)
	tu.RunClient(t, client1Creds, runVerifySyncgroupData, "sync1", "foo", "0", "10", "false")

	go tu.RunClient(t, client0Creds, runUpdateData, "sync0", "5")
	d := time.Duration(rand.Int63n(50)) * time.Millisecond
	time.Sleep(d)
	tu.RunClient(t, client1Creds, runUpdateData, "sync1", "5")

	time.Sleep(10 * time.Second)

	tu.RunClient(t, client0Creds, runVerifyConflictResolution, "sync0")
	tu.RunClient(t, client1Creds, runVerifyConflictResolution, "sync1")
}

// V23TestNestedSyncgroups tests the exchange of deltas between two Syncbase
// instances and their clients with nested syncgroups. The 1st client creates
// two syncgroups at prefixes "f" and "foo" and puts some database entries in
// both of them.  The 2nd client first joins the syncgroup with prefix "foo" and
// verifies that it reads the corresponding database entries.  The 2nd client
// then joins the syncgroup with prefix "f" and verifies that it can read the
// "f" keys.
func V23TestNestedSyncgroups(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	server0Creds, _ := t.Shell().NewChildCredentials("s0")
	client0Creds, _ := t.Shell().NewChildCredentials("c0")
	cleanSync0 := tu.StartSyncbased(t, server0Creds, "sync0", "",
		`{"Read": {"In":["root/c0"]}, "Write": {"In":["root/c0"]}}`)
	defer cleanSync0()

	server1Creds, _ := t.Shell().NewChildCredentials("s1")
	client1Creds, _ := t.Shell().NewChildCredentials("c1")
	cleanSync1 := tu.StartSyncbased(t, server1Creds, "sync1", "",
		`{"Read": {"In":["root/c1"]}, "Write": {"In":["root/c1"]}}`)
	defer cleanSync1()

	sg1Name := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")
	sg2Name := naming.Join("sync0", constants.SyncbaseSuffix, "SG2")

	tu.RunClient(t, client0Creds, runSetupAppA, "sync0")
	tu.RunClient(t, client0Creds, runCreateSyncgroup, "sync0", sg1Name, "tb:foo", "", "root/s0", "root/s1")
	tu.RunClient(t, client0Creds, runCreateSyncgroup, "sync0", sg2Name, "tb:f", "", "root/s0", "root/s1")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "f", "0")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foo", "0")

	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncgroup, "sync1", sg1Name)
	tu.RunClient(t, client1Creds, runVerifySyncgroupData, "sync1", "foo", "0", "10", "false")
	tu.RunClient(t, client1Creds, runJoinSyncgroup, "sync1", sg2Name)
	tu.RunClient(t, client1Creds, runVerifyNestedSyncgroupData, "sync1")
}

// V23TestNestedAndPeerSyncgroups tests the exchange of deltas between three
// Syncbase instances and their clients consisting of nested/peer
// syncgroups. The 1st client creates two syncgroups: SG1 at prefix "foo" and
// SG2 at "f" and puts some database entries in both of them.  The 2nd client
// joins the syncgroup SG1 and verifies that it reads the corresponding database
// entries. Client 2 then creates SG3 at prefix "f". The 3rd client joins the
// syncgroups SG2 and SG3 and verifies that it can read all the "f" and "foo"
// keys created by client 1. Client 2 also verifies that it can read all the "f"
// and "foo" keys created by client 1.
func V23TestNestedAndPeerSyncgroups(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	server0Creds, _ := t.Shell().NewChildCredentials("s0")
	client0Creds, _ := t.Shell().NewChildCredentials("c0")
	cleanSync0 := tu.StartSyncbased(t, server0Creds, "sync0", "",
		`{"Read": {"In":["root/c0"]}, "Write": {"In":["root/c0"]}}`)
	defer cleanSync0()

	server1Creds, _ := t.Shell().NewChildCredentials("s1")
	client1Creds, _ := t.Shell().NewChildCredentials("c1")
	cleanSync1 := tu.StartSyncbased(t, server1Creds, "sync1", "",
		`{"Read": {"In":["root/c1"]}, "Write": {"In":["root/c1"]}}`)
	defer cleanSync1()

	server2Creds, _ := t.Shell().NewChildCredentials("s2")
	client2Creds, _ := t.Shell().NewChildCredentials("c2")
	cleanSync2 := tu.StartSyncbased(t, server2Creds, "sync2", "",
		`{"Read": {"In":["root/c2"]}, "Write": {"In":["root/c2"]}}`)
	defer cleanSync2()

	sg1Name := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")
	sg2Name := naming.Join("sync0", constants.SyncbaseSuffix, "SG2")
	sg3Name := naming.Join("sync1", constants.SyncbaseSuffix, "SG3")

	tu.RunClient(t, client0Creds, runSetupAppA, "sync0")
	tu.RunClient(t, client0Creds, runCreateSyncgroup, "sync0", sg1Name, "tb:foo", "", "root/s0", "root/s1", "root/s2")
	tu.RunClient(t, client0Creds, runCreateSyncgroup, "sync0", sg2Name, "tb:f", "", "root/s0", "root/s2")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "f", "0")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foo", "0")

	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncgroup, "sync1", sg1Name)
	tu.RunClient(t, client1Creds, runVerifySyncgroupData, "sync1", "foo", "0", "10", "false")
	tu.RunClient(t, client1Creds, runCreateSyncgroup, "sync1", sg3Name, "tb:f", "", "root/s1", "root/s2")

	tu.RunClient(t, client2Creds, runSetupAppA, "sync2")
	tu.RunClient(t, client2Creds, runJoinSyncgroup, "sync2", sg2Name)
	tu.RunClient(t, client2Creds, runJoinSyncgroup, "sync2", sg3Name)
	tu.RunClient(t, client2Creds, runVerifyNestedSyncgroupData, "sync2")

	tu.RunClient(t, client1Creds, runVerifyNestedSyncgroupData, "sync1")
}

// V23TestSyncbasedGetDeltasPrePopulate tests the sending of deltas between two
// Syncbase instances and their clients with data existing before the creation
// of a syncgroup.  The 1st client puts entries in a database then creates a
// syncgroup over that data.  The 2nd client joins that syncgroup and reads the
// database entries.
func V23TestSyncbasedGetDeltasPrePopulate(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	server0Creds, _ := t.Shell().NewChildCredentials("s0")
	client0Creds, _ := t.Shell().NewChildCredentials("c0")
	cleanSync0 := tu.StartSyncbased(t, server0Creds, "sync0", "",
		`{"Read": {"In":["root/c0"]}, "Write": {"In":["root/c0"]}}`)
	defer cleanSync0()

	server1Creds, _ := t.Shell().NewChildCredentials("s1")
	client1Creds, _ := t.Shell().NewChildCredentials("c1")
	cleanSync1 := tu.StartSyncbased(t, server1Creds, "sync1", "",
		`{"Read": {"In":["root/c1"]}, "Write": {"In":["root/c1"]}}`)
	defer cleanSync1()

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")

	// Populate table data before creating the syncgroup.  Also populate
	// with data that is not part of the syncgroup to verify filtering.
	tu.RunClient(t, client0Creds, runSetupAppA, "sync0")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foo", "0")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "bar", "0")
	tu.RunClient(t, client0Creds, runCreateSyncgroup, "sync0", sgName, "tb:foo", "", "root/s0", "root/s1")

	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncgroup, "sync1", sgName)
	tu.RunClient(t, client1Creds, runVerifySyncgroupData, "sync1", "foo", "0", "10", "false")
	tu.RunClient(t, client1Creds, runVerifyNonSyncgroupData, "sync1", "bar")
}

// V23TestSyncbasedGetDeltasMultiApp tests the sending of deltas between two
// Syncbase instances and their clients across multiple apps, databases, and
// tables.  The 1st client puts entries in multiple tables across multiple
// app databases then creates multiple syncgroups (one per database) over that
// data.  The 2nd client joins these syncgroups and reads all the data.
func V23TestSyncbasedGetDeltasMultiApp(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	server0Creds, _ := t.Shell().NewChildCredentials("s0")
	client0Creds, _ := t.Shell().NewChildCredentials("c0")
	cleanSync0 := tu.StartSyncbased(t, server0Creds, "sync0", "",
		`{"Read": {"In":["root/c0"]}, "Write": {"In":["root/c0"]}}`)
	defer cleanSync0()

	server1Creds, _ := t.Shell().NewChildCredentials("s1")
	client1Creds, _ := t.Shell().NewChildCredentials("c1")
	cleanSync1 := tu.StartSyncbased(t, server1Creds, "sync1", "",
		`{"Read": {"In":["root/c1"]}, "Write": {"In":["root/c1"]}}`)
	defer cleanSync1()

	sgNamePrefix := naming.Join("sync0", constants.SyncbaseSuffix)
	na, nd, nt := "2", "2", "2" // number of apps, dbs, tables

	tu.RunClient(t, client0Creds, runSetupAppMulti, "sync0", na, nd, nt)
	tu.RunClient(t, client0Creds, runPopulateSyncgroupMulti, "sync0", sgNamePrefix, na, nd, nt, "foo", "bar")

	tu.RunClient(t, client1Creds, runSetupAppMulti, "sync1", na, nd, nt)
	tu.RunClient(t, client1Creds, runJoinSyncgroupMulti, "sync1", sgNamePrefix, na, nd)
	tu.RunClient(t, client1Creds, runVerifySyncgroupDataMulti, "sync1", na, nd, nt, "foo", "bar")
}

// V23TestSyncgroupSync tests the syncing of syncgroup metadata. The 1st client
// creates the syncgroup SG1, and clients 2 and 3 join this syncgroup. All three
// clients must learn of the remaining two. Note that client 2 relies on
// syncgroup metadata syncing to learn of client 3 .
func V23TestSyncgroupSync(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	server0Creds, _ := t.Shell().NewChildCredentials("s0")
	client0Creds, _ := t.Shell().NewChildCredentials("c0")
	cleanSync0 := tu.StartSyncbased(t, server0Creds, "sync0", "",
		`{"Read": {"In":["root/c0"]}, "Write": {"In":["root/c0"]}}`)
	defer cleanSync0()

	server1Creds, _ := t.Shell().NewChildCredentials("s1")
	client1Creds, _ := t.Shell().NewChildCredentials("c1")
	cleanSync1 := tu.StartSyncbased(t, server1Creds, "sync1", "",
		`{"Read": {"In":["root/c1"]}, "Write": {"In":["root/c1"]}}`)
	defer cleanSync1()

	server2Creds, _ := t.Shell().NewChildCredentials("s2")
	client2Creds, _ := t.Shell().NewChildCredentials("c2")
	cleanSync2 := tu.StartSyncbased(t, server2Creds, "sync2", "",
		`{"Read": {"In":["root/c2"]}, "Write": {"In":["root/c2"]}}`)
	defer cleanSync2()

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")

	tu.RunClient(t, client0Creds, runSetupAppA, "sync0")
	tu.RunClient(t, client0Creds, runCreateSyncgroup, "sync0", sgName, "tb:foo", "", "root/s0", "root/s1", "root/s2")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foo", "0")

	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncgroup, "sync1", sgName)

	tu.RunClient(t, client2Creds, runSetupAppA, "sync2")
	tu.RunClient(t, client2Creds, runJoinSyncgroup, "sync2", sgName)

	tu.RunClient(t, client1Creds, runGetSyncgroupMembers, "sync1", sgName, "3")
	tu.RunClient(t, client2Creds, runGetSyncgroupMembers, "sync2", sgName, "3")

	tu.RunClient(t, client1Creds, runVerifySyncgroupData, "sync1", "foo", "0", "10", "false")
	tu.RunClient(t, client2Creds, runVerifySyncgroupData, "sync2", "foo", "0", "10", "false")
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

var runSetupAppA = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	a.Create(ctx, nil)
	d := a.NoSQLDatabase("d", nil)
	d.Create(ctx, nil)
	d.Table(testTable).Create(ctx, nil)

	return nil
}, "runSetupAppA")

// Arguments: 0: Syncbase name, 1: syncgroup name, 2: prefixes, 3: mount table,
// 4 onwards: syncgroup permission blessings.
var runCreateSyncgroup = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	mtName := args[3]
	if mtName == "" {
		mtName = env.Vars[ref.EnvNamespacePrefix]
	}

	spec := wire.SyncgroupSpec{
		Description: "test syncgroup sg",
		Perms:       perms(args[4:]...),
		Prefixes:    toSgPrefixes(args[2]),
		MountTables: []string{mtName},
	}

	sg := d.Syncgroup(args[1])
	info := wire.SyncgroupMemberInfo{SyncPriority: 8}
	if err := sg.Create(ctx, spec, info); err != nil {
		return fmt.Errorf("Create SG %q failed: %v\n", args[1], err)
	}
	return nil
}, "runCreateSyncgroup")

var runJoinSyncgroup = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	sg := d.Syncgroup(args[1])
	info := wire.SyncgroupMemberInfo{SyncPriority: 10}
	if _, err := sg.Join(ctx, info); err != nil {
		return fmt.Errorf("Join SG %q failed: %v\n", args[1], err)
	}
	return nil
}, "runJoinSyncgroup")

// Arguments: 0: Syncbase name, 1: syncgroup name, 2: syncgroup description, 3:
// prefixes, 4 onwards: syncgroup permission blessings.
var runSetSyncgroupSpec = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	sg := d.Syncgroup(args[1])

	mtName := env.Vars[ref.EnvNamespacePrefix]
	spec := wire.SyncgroupSpec{
		Description: args[2],
		Prefixes:    toSgPrefixes(args[3]),
		Perms:       perms(args[4:]...),
		MountTables: []string{mtName},
	}

	if err := sg.SetSpec(ctx, spec, ""); err != nil {
		return fmt.Errorf("SetSpec SG %q failed: %v\n", args[1], err)
	}
	return nil
}, "runSetSyncgroupSpec")

// Arguments: 0: Syncbase name, 1: syncgroup name, 2: syncgroup description, 3:
// prefixes, 4 onwards: syncgroup permission blessings.
var runGetSyncgroupSpec = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	sg := d.Syncgroup(args[1])

	wantDesc := args[2]
	wantPfxs := toSgPrefixes(args[3])
	wantPerms := perms(args[4:]...)

	var spec wire.SyncgroupSpec
	var err error
	for i := 0; i < 8; i++ {
		time.Sleep(500 * time.Millisecond)
		spec, _, err = sg.GetSpec(ctx)
		if err != nil {
			return fmt.Errorf("GetSpec SG %q failed: %v\n", args[1], err)
		}
		if spec.Description == wantDesc {
			break
		}
	}
	if spec.Description != wantDesc || !reflect.DeepEqual(spec.Prefixes, wantPfxs) || !reflect.DeepEqual(spec.Perms, wantPerms) {
		return fmt.Errorf("GetSpec SG %q failed: description got %v, want %v, prefixes got %v, want %v, perms got %v, want %v\n", args[1], spec.Description, wantDesc, spec.Prefixes, wantPfxs, spec.Perms, wantPerms)
	}
	return nil
}, "runGetSyncgroupSpec")

// Arguments: 0: Syncbase name, 1: syncgroup name, 2: number of syncgroup
// members.
var runGetSyncgroupMembers = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	sg := d.Syncgroup(args[1])

	wantMembers, _ := strconv.ParseUint(args[2], 10, 64)
	var gotMembers uint64

	for i := 0; i < 8; i++ {
		time.Sleep(500 * time.Millisecond)
		members, err := sg.GetMembers(ctx)
		if err != nil {
			return fmt.Errorf("GetMembers SG %q failed: %v\n", args[1], err)
		}
		gotMembers = uint64(len(members))
		if wantMembers == gotMembers {
			break
		}
	}
	if wantMembers != gotMembers {
		return fmt.Errorf("GetMembers SG %q failed: members got %v, want %v\n", args[1], gotMembers, wantMembers)
	}
	return nil
}, "runGetSyncgroupMembers")

// Arguments: 0: Syncbase name, 1: key prefix, 2: start index.
var runPopulateData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Do Puts.
	tb := d.Table(testTable)
	start, _ := strconv.ParseUint(args[2], 10, 64)

	for i := start; i < start+10; i++ {
		key := fmt.Sprintf("%s%d", args[1], i)
		r := tb.Row(key)
		if err := r.Put(ctx, "testkey"+key); err != nil {
			return fmt.Errorf("r.Put() failed: %v\n", err)
		}
	}
	return nil
}, "runPopulateData")

// Arguments: 0: Syncbase name, 1: key prefix, 2: start index.
var runPopulateNonVomData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Do Puts.
	tb := d.Table("tb")
	start, _ := strconv.ParseUint(args[2], 10, 64)

	for i := start; i < start+10; i++ {
		key := fmt.Sprintf("%s%d", args[1], i)
		r := tb.Row(key)
		c := wire.RowClient(r.FullName())
		val := []byte("nonvomtestkey" + key)
		if err := c.Put(ctx, -1, val); err != nil {
			return fmt.Errorf("c.Put() failed: %v\n", err)
		}
	}
	return nil
}, "runPopulateNonVomData")

// Arguments: 0: Syncbase name, 1: start index.
// Optional args: 2: end index, 3: value prefix.
// Values are from [start,end) or [start, start+5) depending on whether end
// param was provided.
var runUpdateData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	serviceName, startStr := args[0], args[1]
	start, _ := strconv.ParseUint(startStr, 10, 64)
	end, prefix := start+5, "testkey"
	if len(args) > 2 {
		end, _ = strconv.ParseUint(args[2], 10, 64)
		if end <= start {
			return fmt.Errorf("Test error: end <= start. start: %d, end: %d", start, end)
		}
	}
	if len(args) > 3 {
		prefix = args[3]
	}

	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Do Puts.
	tb := d.Table(testTable)

	for i := start; i < end; i++ {
		key := fmt.Sprintf("foo%d", i)
		r := tb.Row(key)
		if err := r.Put(ctx, prefix+serviceName+key); err != nil {
			return fmt.Errorf("r.Put() failed: %v\n", err)
		}
	}

	return nil
}, "runUpdateData")

var runDeleteData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Do Puts.
	tb := d.Table(testTable)
	start, _ := strconv.ParseUint(args[1], 10, 64)

	for i := start; i < start+5; i++ {
		key := fmt.Sprintf("foo%d", i)
		r := tb.Row(key)
		if err := r.Delete(ctx); err != nil {
			return fmt.Errorf("r.Delete() failed: %v\n", err)
		}
	}

	return nil
}, "runDeleteData")

// Arguments: 0: syncbase name, 1: key prefix, 2: blessing for acl.
var runSetPrefixPermissions = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Set acl.
	tb := d.Table(testTable)

	if err := tb.SetPrefixPermissions(ctx, nosql.Prefix(args[1]), perms(args[2:]...)); err != nil {
		return fmt.Errorf("tb.SetPrefixPermissions() failed: %v\n", err)
	}

	return nil
}, "runSetPrefixPermissions")

// Arguments: 0: syncbase name, 1: key prefix, 2: start index, 3: number of keys, 4: skip scan.
var runVerifySyncgroupData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Wait for a bit (up to 4 sec) until the last key appears.
	tb := d.Table(testTable)

	start, _ := strconv.ParseUint(args[2], 10, 64)
	count, _ := strconv.ParseUint(args[3], 10, 64)
	skipScan, _ := strconv.ParseBool(args[4])
	lastKey := fmt.Sprintf("%s%d", args[1], start+count-1)

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
		key := fmt.Sprintf("%s%d", args[1], i)
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
		stream := tb.Scan(ctx, nosql.Prefix(args[1]))
		for i := 0; stream.Advance(); i++ {
			want := fmt.Sprintf("%s%d", args[1], i)
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
}, "runVerifySyncgroupData")

// Arguments: 0: syncbase name, 1: key prefix, 2: start index, 3: number of keys.
var runVerifySyncgroupNonVomData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Wait for a bit (up to 4 sec) until the last key appears.
	tb := d.Table("tb")

	start, _ := strconv.ParseUint(args[2], 10, 64)
	count, _ := strconv.ParseUint(args[3], 10, 64)
	lastKey := fmt.Sprintf("%s%d", args[1], start+count-1)

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
		key := fmt.Sprintf("%s%d", args[1], i)
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
}, "runVerifySyncgroupNonVomData")

var runVerifyDeletedData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
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
	stream := tb.Scan(ctx, nosql.Prefix(args[1]))
	count := 0
	for i := 5; stream.Advance(); i++ {
		want := fmt.Sprintf("%s%d", args[1], i)
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
}, "runVerifyDeletedData")

var runVerifyConflictResolution = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
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
}, "runVerifyConflictResolution")

var runVerifyNonSyncgroupData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)
	tb := d.Table(testTable)

	// Verify through a scan that none of that data exists.
	count := 0
	stream := tb.Scan(ctx, nosql.Prefix(args[1]))
	for stream.Advance() {
		count++
	}

	if err := stream.Err(); err != nil {
		return fmt.Errorf("scan stream error: %v\n", err)
	}
	if count > 0 {
		return fmt.Errorf("found %d entries in %s prefix that should not be there\n", count, args[1])
	}

	return nil
}, "runVerifyNonSyncgroupData")

var runVerifyLocalAndRemoteData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
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
}, "runVerifyLocalAndRemoteData")

// Arguments: 0: syncbase name, 1: key prefix, 2: start pos for key, 3: number of keys.
var runVerifyLostAccess = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Wait for a bit (up to 4 sec) until the last key disappears.
	tb := d.Table(testTable)

	start, _ := strconv.ParseUint(args[2], 10, 64)
	count, _ := strconv.ParseUint(args[3], 10, 64)
	lastKey := fmt.Sprintf("%s%d", args[1], start+count-1)

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
		key := fmt.Sprintf("%s%d", args[1], i)
		r := tb.Row(key)
		var got string
		if err := r.Get(ctx, &got); verror.ErrorID(err) != verror.ErrNoAccess.ID {
			return fmt.Errorf("r.Get() didn't fail: %v\n", err)
		}
	}

	return nil
}, "runVerifyLostAccess")

var runVerifyNestedSyncgroupData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
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
}, "runVerifyNestedSyncgroupData")

var runSetupAppMulti = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	svc := syncbase.NewService(args[0])
	numApps, _ := strconv.Atoi(args[1])
	numDbs, _ := strconv.Atoi(args[2])
	numTbs, _ := strconv.Atoi(args[3])

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
}, "runSetupAppMulti")

var runPopulateSyncgroupMulti = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	mtName := env.Vars[ref.EnvNamespacePrefix]

	svc := syncbase.NewService(args[0])
	sgNamePrefix := args[1]
	numApps, _ := strconv.Atoi(args[2])
	numDbs, _ := strconv.Atoi(args[3])
	numTbs, _ := strconv.Atoi(args[4])
	prefixes := args[5:]

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
				Perms:       perms("root/s0", "root/s1"),
				Prefixes:    toSgPrefixes(strings.Join(sgPrefixes, ",")),
				MountTables: []string{mtName},
			}

			sg := d.Syncgroup(sgName)
			info := wire.SyncgroupMemberInfo{SyncPriority: 8}
			if err := sg.Create(ctx, spec, info); err != nil {
				return fmt.Errorf("Create SG %q failed: %v\n", sgName, err)
			}
		}
	}

	return nil
}, "runPopulateSyncgroupMulti")

var runJoinSyncgroupMulti = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	svc := syncbase.NewService(args[0])
	sgNamePrefix := args[1]
	numApps, _ := strconv.Atoi(args[2])
	numDbs, _ := strconv.Atoi(args[3])

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
}, "runJoinSyncgroupMulti")

var runVerifySyncgroupDataMulti = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	svc := syncbase.NewService(args[0])
	numApps, _ := strconv.Atoi(args[1])
	numDbs, _ := strconv.Atoi(args[2])
	numTbs, _ := strconv.Atoi(args[3])
	prefixes := args[4:]

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
}, "runVerifySyncgroupDataMulti")
