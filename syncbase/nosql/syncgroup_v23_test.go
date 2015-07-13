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

	tu.RunClient(t, client0Creds, runCreateSyncGroup)
	tu.RunClient(t, client1Creds, runJoinSyncGroup)
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

	tu.RunClient(t, client0Creds, runCreateSyncGroup)
	tu.RunClient(t, client0Creds, runPopulateSyncGroup, "sync0", "0")
	tu.RunClient(t, client1Creds, runJoinSyncGroup)
	tu.RunClient(t, client1Creds, runVerifySyncGroupData, "sync1", "0", "10")
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

	tu.RunClient(t, client0Creds, runCreateSyncGroup)
	tu.RunClient(t, client0Creds, runPopulateSyncGroup, "sync0", "0")

	tu.RunClient(t, client1Creds, runJoinSyncGroup)
	tu.RunClient(t, client1Creds, runVerifySyncGroupData, "sync1", "0", "10")

	tu.RunClient(t, client1Creds, runUpdateData, "sync1", "5")
	tu.RunClient(t, client1Creds, runPopulateSyncGroup, "sync1", "10")

	tu.RunClient(t, client0Creds, runVerifyLocalAndRemoteData, "sync0")
}

////////////////////////////////////
// Helpers.

var runCreateSyncGroup = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService("sync0").App("a")
	a.Create(ctx, nil)
	d := a.NoSQLDatabase("d")
	d.Create(ctx, nil)
	d.CreateTable(ctx, "tb", nil)

	mtName := env.Vars[ref.EnvNamespacePrefix]
	spec := wire.SyncGroupSpec{
		Description: "test syncgroup sg",
		Perms:       perms("root/s0", "root/s1"),
		Prefixes:    []string{"tb:foo"},
		MountTables: []string{mtName},
	}

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")
	sg := d.SyncGroup(sgName)
	info := wire.SyncGroupMemberInfo{8}
	if err := sg.Create(ctx, spec, info); err != nil {
		return fmt.Errorf("Create SG %q failed: %v", sgName, err)
	}
	return nil
}, "runCreateSyncGroup")

var runJoinSyncGroup = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService("sync1").App("a")
	a.Create(ctx, nil)
	d := a.NoSQLDatabase("d")
	d.Create(ctx, nil)
	d.CreateTable(ctx, "tb", nil)

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")
	sg := d.SyncGroup(sgName)
	info := wire.SyncGroupMemberInfo{10}
	if _, err := sg.Join(ctx, info); err != nil {
		return fmt.Errorf("Join SG %q failed: %v", sgName, err)
	}
	return nil
}, "runJoinSyncGroup")

var runPopulateSyncGroup = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d")

	// Do Puts.
	tb := d.Table("tb")
	start, _ := strconv.ParseUint(args[1], 10, 64)

	for i := start; i < start+10; i++ {
		key := fmt.Sprintf("foo%d", i)
		r := tb.Row(key)
		if err := r.Put(ctx, "testkey"+key); err != nil {
			return fmt.Errorf("r.Put() failed: %v", err)
		}
	}

	return nil
}, "runPopulateSyncGroup")

var runUpdateData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d")

	// Do Puts.
	tb := d.Table("tb")
	start, _ := strconv.ParseUint(args[1], 10, 64)

	for i := start; i < start+5; i++ {
		key := fmt.Sprintf("foo%d", i)
		r := tb.Row(key)
		if err := r.Put(ctx, "testkey1"+key); err != nil {
			return fmt.Errorf("r.Put() failed: %v", err)
		}
	}

	return nil
}, "runUpdateData")

var runVerifySyncGroupData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d")

	// Wait for a bit (up to 4 sec) until the last key appears.
	tb := d.Table("tb")

	start, _ := strconv.ParseUint(args[1], 10, 64)
	count, _ := strconv.ParseUint(args[2], 10, 64)
	lastKey := fmt.Sprintf("foo%d", start+count-1)

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
		key := fmt.Sprintf("foo%d", i)
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
	stream := tb.Scan(ctx, nosql.Prefix("foo"))
	for i := 0; stream.Advance(); i++ {
		want := fmt.Sprintf("foo%d", i)
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

var runVerifyLocalAndRemoteData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d")
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
