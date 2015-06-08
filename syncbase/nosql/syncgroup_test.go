// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql_test

import (
	"testing"

	wire "v.io/syncbase/v23/services/syncbase/nosql"
	"v.io/syncbase/v23/syncbase"
	"v.io/syncbase/v23/syncbase/nosql"
	tu "v.io/syncbase/v23/syncbase/testutil"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/verror"
)

// Tests that SyncGroup.Create works as expected.
func TestCreateSyncGroup(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(perms("server/client"))
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")

	// Check if create fails with empty spec.
	spec := wire.SyncGroupSpec{}
	createSyncGroup(t, ctx, d, "sg1", spec, verror.ErrBadArg.ID)

	// Create successfully.
	spec = wire.SyncGroupSpec{
		Description: "test syncgroup sg1",
		Perms:       nil,
		Prefixes:    []string{"t1/foo"},
	}
	createSyncGroup(t, ctx, d, "sg1", spec, verror.ID(""))

	// Check if creating an already existing syncgroup fails.
	createSyncGroup(t, ctx, d, "sg1", spec, verror.ErrExist.ID)

	// Create a peer syncgroup.
	spec.Description = "test syncgroup sg2"
	createSyncGroup(t, ctx, d, "sg2", spec, verror.ID(""))

	// Create a nested syncgroup.
	spec.Description = "test syncgroup sg3"
	spec.Prefixes = []string{"t1/foobar"}
	createSyncGroup(t, ctx, d, "sg3", spec, verror.ID(""))

	// Check that create fails if the perms disallow access.
	perms := perms("server/client")
	perms.Blacklist("server/client", string(access.Read))
	if err := d.SetPermissions(ctx, perms, ""); err != nil {
		t.Fatalf("d.SetPermissions() failed: %v", err)
	}
	spec.Description = "test syncgroup sg4"
	createSyncGroup(t, ctx, d, "sg4", spec, verror.ErrNoAccess.ID)
}

// Tests that SyncGroup.Join works as expected for the case with one Syncbase
// and 2 clients. One client creates the SyncGroup, while the other attempts to
// join it.
//
// TODO(hpucha): Add more integration-style testing with 2 syncbases.
func TestJoinSyncGroup(t *testing.T) {
	// Create client1-server pair.
	ctx1, sName, cleanup, sp, ctx := tu.SetupOrDieCustom("client1", "server", perms("server/client1"))
	defer cleanup()

	a1 := tu.CreateApp(t, ctx1, syncbase.NewService(sName), "a")
	d1 := tu.CreateNoSQLDatabase(t, ctx1, a1, "d")
	specA := wire.SyncGroupSpec{
		Description: "test syncgroup sgA",
		Perms:       perms("server/client1"),
		Prefixes:    []string{"t1/foo"},
	}
	sgNameA := sName + "sgA"
	createSyncGroup(t, ctx1, d1, sgNameA, specA, verror.ID(""))

	// Check that creator can call join successfully.
	joinSyncGroup(t, ctx1, d1, sgNameA, verror.ID(""))

	// Create client2.
	ctx2 := tu.NewClient("client2", "server", ctx, sp)
	a2 := syncbase.NewService(sName).App("a")
	d2 := a2.NoSQLDatabase("d")

	// Check that client2's join fails if the perms disallow access.
	joinSyncGroup(t, ctx2, d2, sgNameA, verror.ErrNoAccess.ID)

	// Client1 gives access to client2.
	if err := d1.SetPermissions(ctx1, perms("server/client1", "server/client2"), ""); err != nil {
		t.Fatalf("d.SetPermissions() failed: %v", err)
	}

	// Verify client2 has access.
	if err := d2.SetPermissions(ctx2, perms("server/client1", "server/client2"), ""); err != nil {
		t.Fatalf("d.SetPermissions() failed: %v", err)
	}

	// Check that client2's join still fails since the SG ACL disallows access.
	joinSyncGroup(t, ctx2, d2, sgNameA, verror.ErrNoAccess.ID)

	// Create a different SyncGroup.
	specB := wire.SyncGroupSpec{
		Description: "test syncgroup sgB",
		Perms:       perms("server/client1", "server/client2"),
		Prefixes:    []string{"t1/foo"},
	}
	sgNameB := sName + "sgB"
	createSyncGroup(t, ctx1, d1, sgNameB, specB, verror.ID(""))

	// Check that client2's join now succeeds.
	joinSyncGroup(t, ctx2, d2, sgNameB, verror.ID(""))
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

func joinSyncGroup(t *testing.T, ctx *context.T, d nosql.Database, sgName string, errID verror.ID) nosql.SyncGroup {
	sg := d.SyncGroup(sgName)
	info := wire.SyncGroupMemberInfo{10}
	if _, err := sg.Join(ctx, info); verror.ErrorID(err) != errID {
		tu.Fatalf(t, "Join SG %v failed: %v", sgName, err)
	}
	return sg
}

func perms(bps ...string) access.Permissions {
	perms := access.Permissions{}
	for _, bp := range bps {
		for _, tag := range access.AllTypicalTags() {
			perms.Add(security.BlessingPattern(bp), string(tag))
		}
	}
	return perms
}
