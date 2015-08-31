// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql_test

import (
	"reflect"
	"testing"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/nosql"
	tu "v.io/v23/syncbase/testutil"
	"v.io/v23/verror"
	"v.io/x/ref/services/syncbase/server/util"
)

// Tests that SyncGroup.Create works as expected.
func TestCreateSyncGroup(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(perms("root/client"))
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")

	// Check if create fails with empty spec.
	spec := wire.SyncGroupSpec{}
	sg1 := naming.Join(sName, util.SyncbaseSuffix, "sg1")

	createSyncGroup(t, ctx, d, sg1, spec, verror.ErrBadArg.ID)

	var wantNames []string
	verifySyncGroupNames(t, ctx, d, wantNames, verror.ID(""))

	// Prefill entries before creating a SyncGroup to exercise the bootstrap
	// of a SyncGroup through Snapshot operations to the watcher.
	t1 := tu.CreateTable(t, ctx, d, "t1")
	for _, k := range []string{"foo123", "foobar123", "xyz"} {
		if err := t1.Put(ctx, k, "value@"+k); err != nil {
			t.Fatalf("t1.Put() of %s failed: %v", k, err)
		}
	}

	// Create successfully.
	// TODO(rdaoud): switch prefixes to (table, prefix) tuples.
	spec = wire.SyncGroupSpec{
		Description: "test syncgroup sg1",
		Perms:       nil,
		Prefixes:    []string{"t1:foo"},
	}
	createSyncGroup(t, ctx, d, sg1, spec, verror.ID(""))

	// Verify SyncGroup is created.
	wantNames = []string{sg1}
	verifySyncGroupNames(t, ctx, d, wantNames, verror.ID(""))
	verifySyncGroupInfo(t, ctx, d, sg1, spec, 1)

	// Check if creating an already existing syncgroup fails.
	createSyncGroup(t, ctx, d, sg1, spec, verror.ErrExist.ID)
	verifySyncGroupNames(t, ctx, d, wantNames, verror.ID(""))

	// Create a peer syncgroup.
	spec.Description = "test syncgroup sg2"
	sg2 := naming.Join(sName, util.SyncbaseSuffix, "sg2")
	createSyncGroup(t, ctx, d, sg2, spec, verror.ID(""))

	wantNames = []string{sg1, sg2}
	verifySyncGroupNames(t, ctx, d, wantNames, verror.ID(""))
	verifySyncGroupInfo(t, ctx, d, sg2, spec, 1)

	// Create a nested syncgroup.
	spec.Description = "test syncgroup sg3"
	spec.Prefixes = []string{"t1:foobar"}
	sg3 := naming.Join(sName, util.SyncbaseSuffix, "sg3")
	createSyncGroup(t, ctx, d, sg3, spec, verror.ID(""))

	wantNames = []string{sg1, sg2, sg3}
	verifySyncGroupNames(t, ctx, d, wantNames, verror.ID(""))
	verifySyncGroupInfo(t, ctx, d, sg3, spec, 1)

	// Check that create fails if the perms disallow access.
	perms := perms("root/client")
	perms.Blacklist("root/client", string(access.Read))
	if err := d.SetPermissions(ctx, perms, ""); err != nil {
		t.Fatalf("d.SetPermissions() failed: %v", err)
	}
	spec.Description = "test syncgroup sg4"
	sg4 := naming.Join(sName, util.SyncbaseSuffix, "sg4")
	createSyncGroup(t, ctx, d, sg4, spec, verror.ErrNoAccess.ID)
	verifySyncGroupNames(t, ctx, d, nil, verror.ErrNoAccess.ID)
}

// Tests that SyncGroup.Join works as expected for the case with one Syncbase
// and 2 clients. One client creates the SyncGroup, while the other attempts to
// join it.
func TestJoinSyncGroup(t *testing.T) {
	// Create client1-server pair.
	ctx, ctx1, sName, rootp, cleanup := tu.SetupOrDieCustom("client1", "server", perms("root/client1"))
	defer cleanup()

	a1 := tu.CreateApp(t, ctx1, syncbase.NewService(sName), "a")
	d1 := tu.CreateNoSQLDatabase(t, ctx1, a1, "d")
	specA := wire.SyncGroupSpec{
		Description: "test syncgroup sgA",
		Perms:       perms("root/client1"),
		Prefixes:    []string{"t1:foo"},
	}
	sgNameA := naming.Join(sName, util.SyncbaseSuffix, "sgA")
	createSyncGroup(t, ctx1, d1, sgNameA, specA, verror.ID(""))

	// Check that creator can call join successfully.
	joinSyncGroup(t, ctx1, d1, sgNameA, verror.ID(""))

	// Create client2.
	ctx2 := tu.NewCtx(ctx, rootp, "client2")
	a2 := syncbase.NewService(sName).App("a")
	d2 := a2.NoSQLDatabase("d", nil)

	// Check that client2's join fails if the perms disallow access.
	joinSyncGroup(t, ctx2, d2, sgNameA, verror.ErrNoAccess.ID)

	verifySyncGroupNames(t, ctx2, d2, nil, verror.ErrNoAccess.ID)

	// Client1 gives access to client2.
	if err := d1.SetPermissions(ctx1, perms("root/client1", "root/client2"), ""); err != nil {
		t.Fatalf("d.SetPermissions() failed: %v", err)
	}

	// Verify client2 has access.
	if err := d2.SetPermissions(ctx2, perms("root/client1", "root/client2"), ""); err != nil {
		t.Fatalf("d.SetPermissions() failed: %v", err)
	}

	// Check that client2's join still fails since the SG ACL disallows access.
	joinSyncGroup(t, ctx2, d2, sgNameA, verror.ErrNoAccess.ID)

	// Create a different SyncGroup.
	specB := wire.SyncGroupSpec{
		Description: "test syncgroup sgB",
		Perms:       perms("root/client1", "root/client2"),
		Prefixes:    []string{"t1:foo"},
	}
	sgNameB := naming.Join(sName, util.SyncbaseSuffix, "sgB")
	createSyncGroup(t, ctx1, d1, sgNameB, specB, verror.ID(""))

	// Check that client2's join now succeeds.
	joinSyncGroup(t, ctx2, d2, sgNameB, verror.ID(""))

	// Verify SyncGroup state.
	wantNames := []string{sgNameA, sgNameB}
	verifySyncGroupNames(t, ctx1, d1, wantNames, verror.ID(""))
	verifySyncGroupNames(t, ctx2, d2, wantNames, verror.ID(""))
	verifySyncGroupInfo(t, ctx1, d1, sgNameA, specA, 1)
	verifySyncGroupInfo(t, ctx1, d1, sgNameB, specB, 1)
	verifySyncGroupInfo(t, ctx2, d2, sgNameB, specB, 1)
}

// Tests that SyncGroup.SetSpec works as expected.
func TestSetSpecSyncGroup(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(perms("root/client"))
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")

	// Create successfully.
	sgName := naming.Join(sName, util.SyncbaseSuffix, "sg1")
	spec := wire.SyncGroupSpec{
		Description: "test syncgroup sg1",
		Perms:       nil,
		Prefixes:    []string{"t1:foo"},
	}
	createSyncGroup(t, ctx, d, sgName, spec, verror.ID(""))

	// Verify SyncGroup is created.
	wantNames := []string{sgName}
	verifySyncGroupNames(t, ctx, d, wantNames, verror.ID(""))
	verifySyncGroupInfo(t, ctx, d, sgName, spec, 1)

	spec.Prefixes = []string{"t1:foo", "t2:bar"}
	spec.Description = "test syncgroup sg1 update"
	spec.Perms = perms("root/client1")

	sg := d.SyncGroup(sgName)
	if err := sg.SetSpec(ctx, spec, ""); err != nil {
		t.Fatalf("sg.SetSpec failed: %v", err)
	}
	verifySyncGroupInfo(t, ctx, d, sgName, spec, 1)
}

///////////////////
// Helpers.

func createSyncGroup(t *testing.T, ctx *context.T, d nosql.Database, sgName string, spec wire.SyncGroupSpec, errID verror.ID) nosql.SyncGroup {
	sg := d.SyncGroup(sgName)
	info := wire.SyncGroupMemberInfo{8}
	if err := sg.Create(ctx, spec, info); verror.ErrorID(err) != errID {
		tu.Fatalf(t, "Create SG %q failed: %v", sgName, err)
	}
	return sg
}

func joinSyncGroup(t *testing.T, ctx *context.T, d nosql.Database, sgName string, wantErr verror.ID) nosql.SyncGroup {
	sg := d.SyncGroup(sgName)
	info := wire.SyncGroupMemberInfo{10}
	if _, err := sg.Join(ctx, info); verror.ErrorID(err) != wantErr {
		tu.Fatalf(t, "Join SG %v failed: %v", sgName, err)
	}
	return sg
}

func verifySyncGroupNames(t *testing.T, ctx *context.T, d nosql.Database, wantNames []string, wantErr verror.ID) {
	gotNames, gotErr := d.GetSyncGroupNames(ctx)
	if verror.ErrorID(gotErr) != wantErr || !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("d.GetSyncGroupNames() failed, got %v, want %v, err %v", gotNames, wantNames, gotErr)
	}
}

func verifySyncGroupInfo(t *testing.T, ctx *context.T, d nosql.Database, sgName string, wantSpec wire.SyncGroupSpec, wantMembers int) {
	sg := d.SyncGroup(sgName)
	gotSpec, _, err := sg.GetSpec(ctx)
	if err != nil || !reflect.DeepEqual(gotSpec, wantSpec) {
		t.Fatalf("sg.GetSpec() failed, got %v, want %v, err %v", gotSpec, wantSpec, err)
	}

	members, err := sg.GetMembers(ctx)
	if err != nil || len(members) != wantMembers {
		t.Fatalf("sg.GetMembers() failed, got %v, want %v, err %v", members, wantMembers, err)
	}
}

// TODO(sadovsky): This appears to be identical to tu.DefaultPerms(). We should
// just use that.
func perms(bps ...string) access.Permissions {
	perms := access.Permissions{}
	for _, bp := range bps {
		for _, tag := range access.AllTypicalTags() {
			perms.Add(security.BlessingPattern(bp), string(tag))
		}
	}
	return perms
}
