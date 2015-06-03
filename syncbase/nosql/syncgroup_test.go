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
	"v.io/v23/security/access"
	"v.io/v23/verror"
)

// Tests that SyncGroup.Create works as expected.
func TestCreateSyncGroup(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")

	// Check if create fails with empty spec.
	sg1 := d.SyncGroup("sg1")
	spec := wire.SyncGroupSpec{}
	info := wire.SyncGroupMemberInfo{8}
	if err := sg1.Create(ctx, spec, info); verror.ErrorID(err) != verror.ErrBadArg.ID {
		t.Fatalf("Create with empty spec didn't fail: %v", err)
	}

	// Create successfully.
	spec = wire.SyncGroupSpec{
		Description: "test syncgroup sg1",
		Perms:       nil,
		Prefixes:    []string{"t1/foo"},
	}
	createSyncGroup(t, ctx, d, "sg1", spec)

	// Check if creating an already existing syncgroup fails.
	if err := sg1.Create(ctx, spec, info); verror.ErrorID(err) != verror.ErrExist.ID {
		t.Fatalf("Create already existing sg didn't fail: %v", err)
	}

	// Create a peer syncgroup.
	spec.Description = "test syncgroup sg2"
	createSyncGroup(t, ctx, d, "sg2", spec)

	// Create a nested syncgroup.
	spec.Description = "test syncgroup sg3"
	spec.Prefixes = []string{"t1/foobar"}
	createSyncGroup(t, ctx, d, "sg3", spec)

	// Check that create fails if the perms disallow access.
	perms := tu.DefaultPerms()
	perms.Blacklist("server/client", string(access.Read))
	if err := d.SetPermissions(ctx, perms, ""); err != nil {
		t.Fatalf("d.SetPermissions() failed: %v", err)
	}
	sg4 := d.SyncGroup("sg4")
	spec.Description = "test syncgroup sg4"
	if err := sg4.Create(ctx, spec, info); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("Create without permissions didn't fail: %v", err)
	}
}

// TODO(hpucha): Consider using this helper function for both success and
// failure cases.
func createSyncGroup(t *testing.T, ctx *context.T, d nosql.Database, sgName string, spec wire.SyncGroupSpec) nosql.SyncGroup {
	sg := d.SyncGroup(sgName)
	info := wire.SyncGroupMemberInfo{8}
	if err := sg.Create(ctx, spec, info); err != nil {
		tu.Fatalf(t, "Create SG %q failed: %v", sgName, err)
	}
	return sg
}
