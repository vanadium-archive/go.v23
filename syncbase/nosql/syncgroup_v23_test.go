// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql_test

import (
	"fmt"

	wire "v.io/syncbase/v23/services/syncbase/nosql"
	"v.io/syncbase/v23/syncbase"
	tu "v.io/syncbase/v23/syncbase/testutil"
	constants "v.io/syncbase/x/ref/services/syncbase/server/util"
	"v.io/v23"
	"v.io/v23/naming"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test/modules"
	"v.io/x/ref/test/v23tests"
)

//go:generate v23 test generate

func V23TestSyncbasedJoinSyncGroup(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	server0Creds, _ := t.Shell().NewChildCredentials("s0")
	client0Creds, _ := t.Shell().NewChildCredentials("s0/c0")
	cleanSync0 := tu.StartSyncbased(t, server0Creds, "sync0", "",
		`{"Read": {"In":["root/s0/c0"]}, "Write": {"In":["root/s0/c0"]}}`)
	defer cleanSync0()

	server1Creds, _ := t.Shell().NewChildCredentials("s0/s1")
	client1Creds, _ := t.Shell().NewChildCredentials("s0/s1/c1")
	cleanSync1 := tu.StartSyncbased(t, server1Creds, "sync1", "",
		`{"Read": {"In":["root/s0/s1/c1"]}, "Write": {"In":["root/s0/s1/c1"]}}`)
	defer cleanSync1()

	tu.RunClient(t, client0Creds, runCreateSyncGroup)
	tu.RunClient(t, client1Creds, runJoinSyncGroup)
}

var runCreateSyncGroup = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService("sync0").App("a")
	a.Create(ctx, nil)
	d := a.NoSQLDatabase("d")
	d.Create(ctx, nil)

	spec := wire.SyncGroupSpec{
		Description: "test syncgroup sg",
		Perms:       perms("root/s0", "root/s0/s1"),
		Prefixes:    []string{"t1/foo"},
	}
	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "foo")
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

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "foo")
	sg := d.SyncGroup(sgName)
	info := wire.SyncGroupMemberInfo{10}
	if _, err := sg.Join(ctx, info); err != nil {
		return fmt.Errorf("Join SG %v failed: %v", sgName, err)
	}
	return nil
}, "runJoinSyncGroup")
