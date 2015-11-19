// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"testing"

	"v.io/v23/naming"
	_ "v.io/x/ref/runtime/factories/generic"
	constants "v.io/x/ref/services/syncbase/server/util"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test/v23tests"
)

//go:generate jiri test generate

// TODO(hpucha): The rest of this file is placeholder only, copied from
// nosql/syncgroup_v23_test.go. Will be modifying it.
func V23TestSyncbasedJoinSyncgroup(t *v23tests.T) {
	if testing.Short() {
		t.Skip("skipping V23TestSyncbasedJoinSyncgroup in short mode")
	}

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
