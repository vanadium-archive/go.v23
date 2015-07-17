// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql_test

import (
	"fmt"
	"strconv"
	"time"

	wire "v.io/syncbase/v23/services/syncbase/nosql"
	"v.io/syncbase/v23/syncbase"
	"v.io/syncbase/v23/syncbase/nosql"
	tu "v.io/syncbase/v23/syncbase/testutil"
	constants "v.io/syncbase/x/ref/services/syncbase/server/util"
	"v.io/v23"
	"v.io/v23/naming"
	"v.io/x/ref"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test/modules"
	"v.io/x/ref/test/v23tests"
)

//go:generate v23 test generate

// V23TestSyncbasedJoinSyncGroup tests the creation and joining of a
// SyncGroup. Client0 creates a SyncGroup at Syncbase0. Client1 requests to join
// the SyncGroup at Syncbase1. Syncbase1 in turn requests Syncbase0 to join the
// SyncGroup.
func V23TestSyncbasedJoinSyncGroup(t *v23tests.T) {
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
	tu.RunClient(t, client0Creds, runCreateSyncGroup, "sync0", sgName, "tb:foo", "root/s0", "root/s1")

	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncGroup, "sync1", sgName)
}

// V23TestSyncbasedGetDeltas tests the sending of deltas between two Syncbase
// instances and their clients.  The 1st client creates a SyncGroup and puts
// some database entries in it.  The 2nd client joins that SyncGroup and reads
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
	tu.RunClient(t, client0Creds, runCreateSyncGroup, "sync0", sgName, "tb:foo", "root/s0", "root/s1")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foo", "0")

	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncGroup, "sync1", sgName)
	tu.RunClient(t, client1Creds, runVerifySyncGroupData, "sync1", "foo", "0", "10")
}

// V23TestSyncbasedExchangeDeltas tests the exchange of deltas between two
// Syncbase instances and their clients.  The 1st client creates a SyncGroup and
// puts some database entries in it.  The 2nd client joins that SyncGroup and
// reads the database entries.  The 2nd client then updates a subset of existing
// keys and adds more keys and the 1st client verifies that it can read these
// keys. This verifies the end-to-end bi-directional synchronization of data
// along the path: client0--Syncbase0--Syncbase1--client1.
func V23TestSyncbasedExchangeDeltas(t *v23tests.T) {
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
	tu.RunClient(t, client0Creds, runCreateSyncGroup, "sync0", sgName, "tb:foo", "root/s0", "root/s1")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foo", "0")

	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncGroup, "sync1", sgName)
	tu.RunClient(t, client1Creds, runVerifySyncGroupData, "sync1", "foo", "0", "10")

	tu.RunClient(t, client1Creds, runUpdateData, "sync1", "5")
	tu.RunClient(t, client1Creds, runPopulateData, "sync1", "foo", "10")

	tu.RunClient(t, client0Creds, runVerifyLocalAndRemoteData, "sync0")
}

// V23TestNestedSyncGroups tests the exchange of deltas between two Syncbase
// instances and their clients with nested SyncGroups. The 1st client creates
// two SyncGroups at prefixes "f" and "foo" and puts some database entries in
// both of them.  The 2nd client first joins the SyncGroup with prefix "foo" and
// verifies that it reads the corresponding database entries.  The 2nd client
// then joins the SyncGroup with prefix "f" and verifies that it can read the
// "f" keys.
func V23TestNestedSyncGroups(t *v23tests.T) {
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
	tu.RunClient(t, client0Creds, runCreateSyncGroup, "sync0", sg1Name, "tb:foo", "root/s0", "root/s1")
	tu.RunClient(t, client0Creds, runCreateSyncGroup, "sync0", sg2Name, "tb:f", "root/s0", "root/s1")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "f", "0")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foo", "0")

	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncGroup, "sync1", sg1Name)
	tu.RunClient(t, client1Creds, runVerifySyncGroupData, "sync1", "foo", "0", "10")
	tu.RunClient(t, client1Creds, runJoinSyncGroup, "sync1", sg2Name)
	tu.RunClient(t, client1Creds, runVerifyNestedSyncGroupData, "sync1")
}

// V23TestNestedAndPeerSyncGroups tests the exchange of deltas between three
// Syncbase instances and their clients consisting of nested/peer
// SyncGroups. The 1st client creates two SyncGroups: SG1 at prefix "foo" and
// SG2 at "f" and puts some database entries in both of them.  The 2nd client
// joins the SyncGroup SG1 and verifies that it reads the corresponding database
// entries. Client 2 then creates SG3 at prefix "f". The 3rd client joins the
// SyncGroups SG2 and SG3 and verifies that it can read all the "f" and "foo"
// keys created by client 1. Client 2 also verifies that it can read all the "f"
// and "foo" keys created by client 1.
func V23TestNestedAndPeerSyncGroups(t *v23tests.T) {
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
	tu.RunClient(t, client0Creds, runCreateSyncGroup, "sync0", sg1Name, "tb:foo", "root/s0", "root/s1", "root/s2")
	tu.RunClient(t, client0Creds, runCreateSyncGroup, "sync0", sg2Name, "tb:f", "root/s0", "root/s2")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "f", "0")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foo", "0")

	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncGroup, "sync1", sg1Name)
	tu.RunClient(t, client1Creds, runVerifySyncGroupData, "sync1", "foo", "0", "10")
	tu.RunClient(t, client1Creds, runCreateSyncGroup, "sync1", sg3Name, "tb:f", "root/s1", "root/s2")

	tu.RunClient(t, client2Creds, runSetupAppA, "sync2")
	tu.RunClient(t, client2Creds, runJoinSyncGroup, "sync2", sg2Name)
	tu.RunClient(t, client2Creds, runJoinSyncGroup, "sync2", sg3Name)
	tu.RunClient(t, client2Creds, runVerifyNestedSyncGroupData, "sync2")

	tu.RunClient(t, client1Creds, runVerifyNestedSyncGroupData, "sync1")
}

// V23TestSyncbasedGetDeltasPrePopulate tests the sending of deltas between two
// Syncbase instances and their clients with data existing before the creation
// of a SyncGroup.  The 1st client puts entries in a database then creates a
// SyncGroup over that data.  The 2nd client joins that SyncGroup and reads the
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

	// Populate table data before creating the SyncGroup.  Also populate
	// with data that is not part of the SyncGroup to verify filtering.
	tu.RunClient(t, client0Creds, runSetupAppA, "sync0")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foo", "0")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "bar", "0")
	tu.RunClient(t, client0Creds, runCreateSyncGroup, "sync0", sgName, "tb:foo", "root/s0", "root/s1")

	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncGroup, "sync1", sgName)
	tu.RunClient(t, client1Creds, runVerifySyncGroupData, "sync1", "foo", "0", "10")
	tu.RunClient(t, client1Creds, runVerifyNonSyncGroupData, "sync1", "bar")
}

// V23TestSyncbasedGetDeltasMultiApp tests the sending of deltas between two
// Syncbase instances and their clients across multiple apps, databases, and
// tables.  The 1st client puts entries in multiple tables across multiple
// app databases then creates multiple SyncGroups (one per database) over that
// data.  The 2nd client joins these SyncGroups and reads all the data.
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
	tu.RunClient(t, client0Creds, runPopulateSyncGroupMulti, "sync0", sgNamePrefix, na, nd, nt, "foo", "bar")

	tu.RunClient(t, client1Creds, runSetupAppMulti, "sync1", na, nd, nt)
	tu.RunClient(t, client1Creds, runJoinSyncGroupMulti, "sync1", sgNamePrefix, na, nd)
	tu.RunClient(t, client1Creds, runVerifySyncGroupDataMulti, "sync1", na, nd, nt, "foo", "bar")
}

////////////////////////////////////
// Helpers.

var runSetupAppA = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	a.Create(ctx, nil)
	d := a.NoSQLDatabase("d", nil)
	d.Create(ctx, nil)
	d.CreateTable(ctx, "tb", nil)

	return nil
}, "runSetupAppA")

var runCreateSyncGroup = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	mtName := env.Vars[ref.EnvNamespacePrefix]
	spec := wire.SyncGroupSpec{
		Description: "test syncgroup sg",
		Perms:       perms(args[3:]...),
		Prefixes:    []string{args[2]},
		MountTables: []string{mtName},
	}

	sg := d.SyncGroup(args[1])
	info := wire.SyncGroupMemberInfo{8}
	if err := sg.Create(ctx, spec, info); err != nil {
		return fmt.Errorf("Create SG %q failed: %v\n", args[1], err)
	}
	return nil
}, "runCreateSyncGroup")

var runJoinSyncGroup = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	sg := d.SyncGroup(args[1])
	info := wire.SyncGroupMemberInfo{10}
	if _, err := sg.Join(ctx, info); err != nil {
		return fmt.Errorf("Join SG %q failed: %v\n", args[1], err)
	}
	return nil
}, "runJoinSyncGroup")

var runPopulateData = modules.Register(func(env *modules.Env, args ...string) error {
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
		if err := r.Put(ctx, "testkey"+key); err != nil {
			return fmt.Errorf("r.Put() failed: %v\n", err)
		}
	}
	return nil
}, "runPopulateData")

var runUpdateData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Do Puts.
	tb := d.Table("tb")
	start, _ := strconv.ParseUint(args[1], 10, 64)

	for i := start; i < start+5; i++ {
		key := fmt.Sprintf("foo%d", i)
		r := tb.Row(key)
		if err := r.Put(ctx, "testkey1"+key); err != nil {
			return fmt.Errorf("r.Put() failed: %v\n", err)
		}
	}

	return nil
}, "runUpdateData")

var runVerifySyncGroupData = modules.Register(func(env *modules.Env, args ...string) error {
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
	return nil
}, "runVerifySyncGroupData")

var runVerifyNonSyncGroupData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)
	tb := d.Table("tb")

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
}, "runVerifyNonSyncGroupData")

var runVerifyLocalAndRemoteData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)
	tb := d.Table("tb")

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
		{5, 5, "testkey1"},
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

var runVerifyNestedSyncGroupData = modules.Register(func(env *modules.Env, args ...string) error {
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
	tb := d.Table("tb")

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
}, "runVerifyNestedSyncGroupData")

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
				d.CreateTable(ctx, tbName, nil)
			}
		}
	}

	return nil
}, "runSetupAppMulti")

var runPopulateSyncGroupMulti = modules.Register(func(env *modules.Env, args ...string) error {
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
			// Also determine the SyncGroup prefixes.
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

			// Create one SyncGroup per database across all tables
			// and prefixes.
			sgName := naming.Join(sgNamePrefix, appName, dbName)
			spec := wire.SyncGroupSpec{
				Description: fmt.Sprintf("test sg %s/%s", appName, dbName),
				Perms:       perms("root/s0", "root/s1"),
				Prefixes:    sgPrefixes,
				MountTables: []string{mtName},
			}

			sg := d.SyncGroup(sgName)
			info := wire.SyncGroupMemberInfo{8}
			if err := sg.Create(ctx, spec, info); err != nil {
				return fmt.Errorf("Create SG %q failed: %v\n", sgName, err)
			}
		}
	}

	return nil
}, "runPopulateSyncGroupMulti")

var runJoinSyncGroupMulti = modules.Register(func(env *modules.Env, args ...string) error {
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
			sg := d.SyncGroup(sgName)
			info := wire.SyncGroupMemberInfo{10}
			if _, err := sg.Join(ctx, info); err != nil {
				return fmt.Errorf("Join SG %q failed: %v\n", sgName, err)
			}
		}
	}

	return nil
}, "runJoinSyncGroupMulti")

var runVerifySyncGroupDataMulti = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	svc := syncbase.NewService(args[0])
	numApps, _ := strconv.Atoi(args[1])
	numDbs, _ := strconv.Atoi(args[2])
	numTbs, _ := strconv.Atoi(args[3])
	prefixes := args[4:]

	time.Sleep(15 * time.Second)

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
}, "runVerifySyncGroupDataMulti")
