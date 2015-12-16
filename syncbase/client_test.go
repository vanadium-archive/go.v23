// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase_test

import (
	"testing"

	"v.io/v23/naming"
	"v.io/v23/security/access"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/nosql"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/generic"
	tu "v.io/x/ref/services/syncbase/testutil"
)

// Tests various Name and FullName methods.
func TestName(t *testing.T) {
	s := syncbase.NewService("s")
	a := s.App("a")

	if s.FullName() != "s" {
		t.Errorf("Wrong full name: %q", s.FullName())
	}
	if a.Name() != "a" {
		t.Errorf("Wrong name: %q", a.Name())
	}
	if a.FullName() != naming.Join("s", "a") {
		t.Errorf("Wrong name: %q", a.FullName())
	}
}

// Tests that Service.ListApps works as expected.
func TestListApps(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	s := syncbase.NewService(sName)
	tu.TestListChildren(t, ctx, s, tu.OkAppNames)
}

// Tests that Service.{Set,Get}Permissions work as expected.
func TestServicePerms(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	tu.TestPerms(t, ctx, syncbase.NewService(sName))
}

// Tests that App.Create works as expected.
func TestAppCreate(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	tu.TestCreate(t, ctx, syncbase.NewService(sName))
}

// Tests that App.Create checks names as expected.
func TestAppCreateNameValidation(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	tu.TestCreateNameValidation(t, ctx, syncbase.NewService(sName), tu.OkAppNames, tu.NotOkAppNames)
}

// Tests that App.Destroy works as expected.
func TestAppDestroy(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	tu.TestDestroy(t, ctx, syncbase.NewService(sName))
}

// Tests that App.ListDatabases works as expected.
func TestListDatabases(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "app/a#%b")
	tu.TestListChildren(t, ctx, a, tu.OkDbTableNames)
}

// Tests that App.{Set,Get}Permissions work as expected.
func TestAppPerms(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	tu.TestPerms(t, ctx, a)
}

// Tests that App.Destroy destroys all databases in the app.
func TestAppDestroyAndRecreate(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	// Create the hierarchy.
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	d2 := tu.CreateNoSQLDatabase(t, ctx, a, "d2")
	tb := tu.CreateTable(t, ctx, d, "tb")
	// Write some data.
	if err := tb.Put(ctx, "bar/baz", "A"); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}
	if err := tb.Put(ctx, "foo", "B"); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}
	// Remove admin and write permissions from "bar" and d2.
	fullPerms := tu.DefaultPerms("root:client").Normalize()
	readPerms := fullPerms.Copy()
	readPerms.Clear("root:client", string(access.Write), string(access.Admin))
	readPerms.Normalize()
	if err := tb.SetPrefixPermissions(ctx, nosql.Prefix("bar"), readPerms); err != nil {
		t.Fatalf("tb.SetPrefixPermissions() failed: %v", err)
	}
	if err := d2.SetPermissions(ctx, readPerms, ""); err != nil {
		t.Fatalf("d2.SetPermissions() failed: %v", err)
	}
	// Verify we have no write access to "bar" anymore.
	if err := tb.Put(ctx, "bar/bat", "C"); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.Put() should have failed with ErrNoAccess, got: %v", err)
	}
	// Verify we cannot destroy d2.
	if err := d2.Destroy(ctx); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("d2.Destroy() should have failed with ErrNoAccess, got: %v", err)
	}
	// Destroy app. Destroy needs only admin permissions on the app, so it
	// shouldn't be affected by the read-only prefix or database ACL.
	if err := a.Destroy(ctx); err != nil {
		t.Fatalf("a.Destroy() failed: %v", err)
	}
	// Recreate the hierarchy.
	a = tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d = tu.CreateNoSQLDatabase(t, ctx, a, "d")
	d2 = tu.CreateNoSQLDatabase(t, ctx, a, "d2")
	tb = tu.CreateTable(t, ctx, d, "tb")
	// Verify table is empty.
	tu.CheckScan(t, ctx, tb, nosql.Prefix(""), []string{}, []interface{}{})
	// Verify we again have write access to "bar".
	if err := tb.Put(ctx, "bar/bat", "C"); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}
}
