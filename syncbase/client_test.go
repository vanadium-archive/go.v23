// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/query/syncql"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase"
	"v.io/v23/verror"
	"v.io/v23/vom"
	_ "v.io/x/ref/runtime/factories/generic"
	tu "v.io/x/ref/services/syncbase/testutil"
)

// TODO(rogulenko): Test perms checking for Glob and Exec.

// Tests various Name, FullName, and Key methods.
func TestNameAndKey(t *testing.T) {
	s := syncbase.NewService("s")
	a := s.App("a")
	d := a.Database("d", nil)
	c := d.Collection("c")
	r := c.Row("r")

	if s.FullName() != "s" {
		t.Errorf("Wrong full name: %q", s.FullName())
	}
	if a.Name() != "a" {
		t.Errorf("Wrong name: %q", a.Name())
	}
	if a.FullName() != naming.Join("s", "a") {
		t.Errorf("Wrong name: %q", a.FullName())
	}
	if d.Name() != "d" {
		t.Errorf("Wrong name: %q", d.Name())
	}
	if d.FullName() != naming.Join("s", "a", "d") {
		t.Errorf("Wrong full name: %q", d.FullName())
	}
	if c.Name() != "c" {
		t.Errorf("Wrong name: %q", c.Name())
	}
	if c.FullName() != naming.Join("s", "a", "d", "c") {
		t.Errorf("Wrong full name: %q", c.FullName())
	}
	if r.Key() != "r" {
		t.Errorf("Wrong key: %q", r.Key())
	}
	if r.FullName() != naming.Join("s", "a", "d", "c", "r") {
		t.Errorf("Wrong full name: %q", r.FullName())
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
	tu.TestListChildren(t, ctx, a, tu.OkDbCollectionNames)
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
	d := tu.CreateDatabase(t, ctx, a, "d")
	d2 := tu.CreateDatabase(t, ctx, a, "d2")
	c := tu.CreateCollection(t, ctx, d, "c")
	// Write some data.
	if err := c.Put(ctx, "bar/baz", "A"); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	if err := c.Put(ctx, "foo", "B"); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	// Remove admin and write permissions from "bar" and d2.
	fullPerms := tu.DefaultPerms("root:client").Normalize()
	readPerms := fullPerms.Copy()
	readPerms.Clear("root:client", string(access.Write), string(access.Admin))
	readPerms.Normalize()
	if err := c.SetPrefixPermissions(ctx, syncbase.Prefix("bar"), readPerms); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}
	if err := d2.SetPermissions(ctx, readPerms, ""); err != nil {
		t.Fatalf("d2.SetPermissions() failed: %v", err)
	}
	// Verify we have no write access to "bar" anymore.
	if err := c.Put(ctx, "bar/bat", "C"); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("c.Put() should have failed with ErrNoAccess, got: %v", err)
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
	d = tu.CreateDatabase(t, ctx, a, "d")
	d2 = tu.CreateDatabase(t, ctx, a, "d2")
	c = tu.CreateCollection(t, ctx, d, "c")
	// Verify collection is empty.
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{}, []interface{}{})
	// Verify we again have write access to "bar".
	if err := c.Put(ctx, "bar/bat", "C"); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
}

// Tests that Database.Create works as expected.
func TestDatabaseCreate(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	tu.TestCreate(t, ctx, a)
}

// Tests name-checking on database creation.
func TestDatabaseCreateNameValidation(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	tu.TestCreateNameValidation(t, ctx, a, tu.OkDbCollectionNames, tu.NotOkDbCollectionNames)
}

// Tests that Database.Destroy works as expected.
func TestDatabaseDestroy(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	tu.TestDestroy(t, ctx, a)
}

// Tests that Database.Exec works as expected.
// Note: The Exec method is tested more thoroughly in exec_test.go.
// Also, the query package is tested in its entirety in v23/query/...
func TestExec(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	foo := Foo{I: 4, S: "f"}
	if err := c.Put(ctx, "foo", foo); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}

	bar := Bar{F: 0.5, S: "b"}
	// NOTE: not best practice, but store bar as
	// optional (by passing the address of bar to Put).
	// This tests auto-dereferencing.
	if err := c.Put(ctx, "bar", &bar); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}

	baz := Baz{Name: "John Doe", Active: true}
	if err := c.Put(ctx, "baz", baz); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}

	tu.CheckExec(t, ctx, d, "select k, v.Name from c where Type(v) like \"%.Baz\"",
		[]string{"k", "v.Name"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz.Name)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from c",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz)},
			{vom.RawBytesOf("foo"), vom.RawBytesOf(foo)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from c where k like \"ba%\"",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from c where v.Active = true",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from c where Type(v) like \"%.Bar\"",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from c where v.F = 0.5",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from c where Type(v) like \"%.Baz\"",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz)},
		})

	// Delete baz.
	tu.CheckExec(t, ctx, d, "delete from c where Type(v) like \"%.Baz\"",
		[]string{"Count"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf(1)},
		})

	// Check that baz is no longer in the collection.
	tu.CheckExec(t, ctx, d, "select k, v from c",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
			{vom.RawBytesOf("foo"), vom.RawBytesOf(foo)},
		})

	tu.CheckExecError(t, ctx, d, "select k, v from foo", syncql.ErrTableCantAccess.ID)
}

// Tests that Database.ListCollections works as expected.
func TestListCollections(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "app/a#%b")
	d := tu.CreateDatabase(t, ctx, a, "d")
	tu.TestListChildren(t, ctx, d, tu.OkDbCollectionNames)
}

// Tests that Database.{Set,Get}Permissions work as expected.
func TestDatabasePerms(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
	tu.TestPerms(t, ctx, d)
}

// Tests that Collection.Create works as expected.
func TestCollectionCreate(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
	tu.TestCreate(t, ctx, d)
}

// Tests name-checking on collection creation.
func TestCollectionCreateNameValidation(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
	tu.TestCreateNameValidation(t, ctx, d, tu.OkDbCollectionNames, tu.NotOkDbCollectionNames)
}

// Tests that Collection.Destroy works as expected.
func TestCollectionDestroy(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
	tu.TestDestroy(t, ctx, d)
}

// Tests that Collection.{Set,Get,Delete}Permissions methods work as expected.
func TestCollectionPerms(t *testing.T) {
	ctx, clientACtx, sName, rootp, cleanup := tu.SetupOrDieCustom("clientA", "server", nil)
	defer cleanup()
	clientBCtx := tu.NewCtx(ctx, rootp, "clientB")
	a := tu.CreateApp(t, clientACtx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, clientACtx, a, "d")
	c := tu.CreateCollection(t, clientACtx, d, "c")

	// Permission objects.
	aAndB := tu.DefaultPerms("root:clientA", "root:clientB")
	aOnly := tu.DefaultPerms("root:clientA")
	bOnly := tu.DefaultPerms("root:clientB")

	// Set initial permissions.
	if err := c.SetPermissions(clientACtx, aAndB); err != nil {
		t.Fatalf("c.SetPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientACtx, syncbase.Prefix(""), aAndB); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientACtx, syncbase.Prefix("prefix"), aAndB); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientBCtx, syncbase.Prefix("prefix"), aAndB); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientACtx, syncbase.Prefix("prefix_a"), aOnly); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientBCtx, syncbase.Prefix("prefix_b"), bOnly); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}

	// Checks A has no access to 'prefix_b' and vice versa.
	if err := c.SetPrefixPermissions(clientACtx, syncbase.Prefix("prefix_b"), aOnly); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("c.SetPrefixPermissions() should have failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientACtx, syncbase.Prefix("prefix_b_suffix"), aOnly); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("c.SetPrefixPermissions() should have failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientBCtx, syncbase.Prefix("prefix_a"), bOnly); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("c.SetPrefixPermissions() should have failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientBCtx, syncbase.Prefix("prefix_a_suffix"), bOnly); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("c.SetPrefixPermissions() should have failed: %v", err)
	}

	// Check GetPrefixPermissions.
	wantPerms := []syncbase.PrefixPermissions{
		syncbase.PrefixPermissions{Prefix: syncbase.Prefix(""), Perms: aAndB},
	}
	if got, _ := c.GetPrefixPermissions(clientACtx, ""); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := c.GetPrefixPermissions(clientACtx, "abc"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	wantPerms = []syncbase.PrefixPermissions{
		syncbase.PrefixPermissions{Prefix: syncbase.Prefix("prefix"), Perms: aAndB},
		syncbase.PrefixPermissions{Prefix: syncbase.Prefix(""), Perms: aAndB},
	}
	if got, _ := c.GetPrefixPermissions(clientACtx, "prefix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := c.GetPrefixPermissions(clientACtx, "prefix_c"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	wantPerms = []syncbase.PrefixPermissions{
		syncbase.PrefixPermissions{Prefix: syncbase.Prefix("prefix_a"), Perms: aOnly},
		syncbase.PrefixPermissions{Prefix: syncbase.Prefix("prefix"), Perms: aAndB},
		syncbase.PrefixPermissions{Prefix: syncbase.Prefix(""), Perms: aAndB},
	}
	if got, _ := c.GetPrefixPermissions(clientACtx, "prefix_a"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := c.GetPrefixPermissions(clientACtx, "prefix_a_suffix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	wantPerms = []syncbase.PrefixPermissions{
		syncbase.PrefixPermissions{Prefix: syncbase.Prefix("prefix_b"), Perms: bOnly},
		syncbase.PrefixPermissions{Prefix: syncbase.Prefix("prefix"), Perms: aAndB},
		syncbase.PrefixPermissions{Prefix: syncbase.Prefix(""), Perms: aAndB},
	}
	if got, _ := c.GetPrefixPermissions(clientACtx, "prefix_b"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := c.GetPrefixPermissions(clientACtx, "prefix_b_suffix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}

	// Delete some prefix permissions and check again.
	// Check that A can't delete permissions of B.
	if err := c.DeletePrefixPermissions(clientACtx, syncbase.Prefix("prefix_b")); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("c.DeletePrefixPermissions() should have failed: %v", err)
	}
	if err := c.DeletePrefixPermissions(clientBCtx, syncbase.Prefix("prefix_a")); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("c.DeletePrefixPermissions() should have failed: %v", err)
	}
	// Delete 'prefix' and 'prefix_a'
	if err := c.DeletePrefixPermissions(clientACtx, syncbase.Prefix("prefix")); err != nil {
		t.Fatalf("c.DeletePrefixPermissions() failed: %v", err)
	}
	if err := c.DeletePrefixPermissions(clientACtx, syncbase.Prefix("prefix_a")); err != nil {
		t.Fatalf("c.DeletePrefixPermissions() failed: %v", err)
	}
	// Check DeletePrefixPermissions is idempotent.
	if err := c.DeletePrefixPermissions(clientACtx, syncbase.Prefix("prefix")); err != nil {
		t.Fatalf("c.DeletePrefixPermissions() failed: %v", err)
	}

	// Check GetPrefixPermissions again.
	wantPerms = []syncbase.PrefixPermissions{
		syncbase.PrefixPermissions{Prefix: syncbase.Prefix(""), Perms: aAndB},
	}
	if got, _ := c.GetPrefixPermissions(clientACtx, ""); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := c.GetPrefixPermissions(clientACtx, "prefix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := c.GetPrefixPermissions(clientACtx, "prefix_a"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := c.GetPrefixPermissions(clientACtx, "prefix_a_suffix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	wantPerms = []syncbase.PrefixPermissions{
		syncbase.PrefixPermissions{Prefix: syncbase.Prefix("prefix_b"), Perms: bOnly},
		syncbase.PrefixPermissions{Prefix: syncbase.Prefix(""), Perms: aAndB},
	}
	if got, _ := c.GetPrefixPermissions(clientACtx, "prefix_b"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := c.GetPrefixPermissions(clientACtx, "prefix_b_suffix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}

	// Remove B from collection-level permissions and check B has no access.
	if err := c.SetPermissions(clientACtx, aOnly); err != nil {
		t.Fatalf("c.SetPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientACtx, syncbase.Prefix(""), aOnly); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}
	if _, err := c.GetPrefixPermissions(clientBCtx, ""); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("c.GetPrefixPermissions() should have failed: %v", err)
	}
	if _, err := c.GetPrefixPermissions(clientBCtx, "prefix_b"); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("c.GetPrefixPermissions() should have failed: %v", err)
	}
	if err := c.SetPermissions(clientBCtx, bOnly); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("c.SetPermissions() should have failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientBCtx, syncbase.Prefix("prefix_b"), bOnly); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("c.SetPrefixPermissions() should have failed: %v", err)
	}
}

// Tests that Collection.{Set,Get}Permissions methods work as expected.
func TestCollectionPermsDifferentOrder(t *testing.T) {
	ctx, clientACtx, sName, rootp, cleanup := tu.SetupOrDieCustom("clientA", "server", nil)
	defer cleanup()
	clientBCtx := tu.NewCtx(ctx, rootp, "clientB")
	a := tu.CreateApp(t, clientACtx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, clientACtx, a, "d")
	c := tu.CreateCollection(t, clientACtx, d, "c")

	// Permission objects.
	aAndB := tu.DefaultPerms("root:clientA", "root:clientB")
	aOnly := tu.DefaultPerms("root:clientA")
	bOnly := tu.DefaultPerms("root:clientB")

	// Set initial permissions.
	if err := c.SetPermissions(clientACtx, aAndB); err != nil {
		t.Fatalf("c.SetPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientACtx, syncbase.Prefix(""), aAndB); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientACtx, syncbase.Prefix("prefix"), aAndB); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientBCtx, syncbase.Prefix("prefix_b"), bOnly); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientACtx, syncbase.Prefix("prefix_a"), aOnly); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}

	wantPerms := []syncbase.PrefixPermissions{
		syncbase.PrefixPermissions{Prefix: syncbase.Prefix("prefix_a"), Perms: aOnly},
		syncbase.PrefixPermissions{Prefix: syncbase.Prefix("prefix"), Perms: aAndB},
		syncbase.PrefixPermissions{Prefix: syncbase.Prefix(""), Perms: aAndB},
	}
	if got, _ := c.GetPrefixPermissions(clientACtx, "prefix_a"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := c.GetPrefixPermissions(clientACtx, "prefix_a_suffix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	wantPerms = []syncbase.PrefixPermissions{
		syncbase.PrefixPermissions{Prefix: syncbase.Prefix("prefix_b"), Perms: bOnly},
		syncbase.PrefixPermissions{Prefix: syncbase.Prefix("prefix"), Perms: aAndB},
		syncbase.PrefixPermissions{Prefix: syncbase.Prefix(""), Perms: aAndB},
	}
	if got, _ := c.GetPrefixPermissions(clientACtx, "prefix_b"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := c.GetPrefixPermissions(clientACtx, "prefix_b_suffix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
}

// bitmaskToPrefix converts first length bits of bitmask to a string.
// Bits are converted the following way: 0 -> 'b', 1 -> 'a'.
// The lowest bit becomes the first character in the string.
// For example, (0x8, 4) -> "bbba".
func bitmaskToPrefix(bitmask, length int) string {
	prefix := ""
	for k := 0; k < length; k++ {
		prefix += string('b' - ((bitmask >> uint32(k)) & 1))
	}
	return prefix
}

func checkGetPermissions(t *testing.T, ctx *context.T, c syncbase.Collection, prefix string, max, min int) {
	perms := tu.DefaultPerms("root:client")
	for len(prefix) < max {
		prefix += "a"
	}
	var wantPerms []syncbase.PrefixPermissions
	for k := max; k >= min; k-- {
		wantPerms = append(wantPerms, syncbase.PrefixPermissions{Prefix: syncbase.Prefix(prefix[:k]), Perms: perms})
	}
	wantPerms = append(wantPerms, syncbase.PrefixPermissions{Prefix: syncbase.Prefix(""), Perms: perms})
	if gotPerms, _ := c.GetPrefixPermissions(ctx, prefix); !reflect.DeepEqual(gotPerms, wantPerms) {
		tu.Fatalf(t, "Unexpected permissions for %q: got %v, want %v", prefix, gotPerms, wantPerms)
	}
}

// Tests that Collection.{Set,Get,Delete}Permissions methods work as expected
// for nested prefixes.
func TestCollectionPermsNested(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
	c := tu.CreateCollection(t, ctx, d, "c")
	// The permission object.
	perms := tu.DefaultPerms("root:client")
	depth := 4
	// Set permissions in order b, a, bb, ba, ab, aa, ...
	for i := 1; i <= depth; i++ {
		for j := 0; j < 1<<uint32(i); j++ {
			prefix := bitmaskToPrefix(j, i)
			if err := c.SetPrefixPermissions(ctx, syncbase.Prefix(prefix), perms); err != nil {
				t.Fatalf("c.SetPrefixPermissions() failed for %q: %v", prefix, err)
			}
			checkGetPermissions(t, ctx, c, prefix, i, 1)
		}
	}
	// Delete permissions in the reverse order.
	for i := depth; i >= 1; i-- {
		for j := 0; j < 1<<uint32(i); j++ {
			prefix := bitmaskToPrefix(j, i)
			if err := c.DeletePrefixPermissions(ctx, syncbase.Prefix(prefix)); err != nil {
				t.Fatalf("c.DeletePrefixPermissions() failed for %q: %v", prefix, err)
			}
			checkGetPermissions(t, ctx, c, prefix, i-1, 1)
		}
	}
	// Do again the first two steps in the reverse order.
	for i := depth; i >= 1; i-- {
		for j := 0; j < 1<<uint32(i); j++ {
			prefix := bitmaskToPrefix(j, i)
			if err := c.SetPrefixPermissions(ctx, syncbase.Prefix(prefix), perms); err != nil {
				t.Fatalf("c.SetPrefixPermissions() failed for %q: %v", prefix, err)
			}
			checkGetPermissions(t, ctx, c, prefix, depth, i)
		}
	}
	for i := 1; i <= depth; i++ {
		for j := 0; j < 1<<uint32(i); j++ {
			prefix := bitmaskToPrefix(j, i)
			if err := c.DeletePrefixPermissions(ctx, syncbase.Prefix(prefix)); err != nil {
				t.Fatalf("c.DeletePrefixPermissions() failed for %q: %v", prefix, err)
			}
			checkGetPermissions(t, ctx, c, prefix, depth, i+1)
		}
	}
}

// Tests that Collection.Destroy deletes all rows and ACLs in the collection.
func TestCollectionDestroyAndRecreate(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
	c := tu.CreateCollection(t, ctx, d, "c")
	// Write some data.
	if err := c.Put(ctx, "bar/baz", "A"); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	if err := c.Put(ctx, "foo", "B"); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	// Remove admin and write permissions from "bar".
	fullPerms := tu.DefaultPerms("root:client").Normalize()
	readPerms := fullPerms.Copy()
	readPerms.Clear("root:client", string(access.Write), string(access.Admin))
	readPerms.Normalize()
	if err := c.SetPrefixPermissions(ctx, syncbase.Prefix("bar"), readPerms); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}
	// Verify we have no write access to "bar" anymore.
	if err := c.Put(ctx, "bar/bat", "C"); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("c.Put() should have failed with ErrNoAccess, got: %v", err)
	}
	// Destroy collection. Destroy needs only admin permissions on the collection,
	// so it shouldn't be affected by the read-only prefix ACL.
	if err := c.Destroy(ctx); err != nil {
		t.Fatalf("c.Destroy() failed: %v", err)
	}
	// Recreate the collection.
	if err := c.Create(ctx, nil); err != nil {
		t.Fatalf("c.Create() (recreate) failed: %v", err)
	}
	// Verify collection is empty.
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{}, []interface{}{})
	// Verify we again have write access to "bar".
	if err := c.Put(ctx, "bar/bat", "C"); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
}

////////////////////////////////////////
// Tests involving rows

type Foo struct {
	I int
	S string
}

type Bar struct {
	F float32
	S string
}

type Baz struct {
	Name   string
	Active bool
}

// Tests that Collection.Scan works as expected.
func TestCollectionScan(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{}, []interface{}{})

	fooWant := Foo{I: 4, S: "f"}
	if err := c.Put(ctx, "foo", &fooWant); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	barWant := Bar{F: 0.5, S: "b"}
	if err := c.Put(ctx, "bar", &barWant); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}

	// Match all keys.
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})
	tu.CheckScan(t, ctx, c, syncbase.Range("", ""), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})
	tu.CheckScan(t, ctx, c, syncbase.Range("", "z"), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})
	tu.CheckScan(t, ctx, c, syncbase.Range("a", ""), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})
	tu.CheckScan(t, ctx, c, syncbase.Range("a", "z"), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})

	// Match "bar" only.
	tu.CheckScan(t, ctx, c, syncbase.Prefix("b"), []string{"bar"}, []interface{}{&barWant})
	tu.CheckScan(t, ctx, c, syncbase.Prefix("bar"), []string{"bar"}, []interface{}{&barWant})
	tu.CheckScan(t, ctx, c, syncbase.Range("bar", "baz"), []string{"bar"}, []interface{}{&barWant})
	tu.CheckScan(t, ctx, c, syncbase.Range("bar", "foo"), []string{"bar"}, []interface{}{&barWant})
	tu.CheckScan(t, ctx, c, syncbase.Range("", "foo"), []string{"bar"}, []interface{}{&barWant})

	// Match "foo" only.
	tu.CheckScan(t, ctx, c, syncbase.Prefix("f"), []string{"foo"}, []interface{}{&fooWant})
	tu.CheckScan(t, ctx, c, syncbase.Prefix("foo"), []string{"foo"}, []interface{}{&fooWant})
	tu.CheckScan(t, ctx, c, syncbase.Range("foo", "fox"), []string{"foo"}, []interface{}{&fooWant})
	tu.CheckScan(t, ctx, c, syncbase.Range("foo", ""), []string{"foo"}, []interface{}{&fooWant})

	// Match nothing.
	tu.CheckScan(t, ctx, c, syncbase.Range("a", "bar"), []string{}, []interface{}{})
	tu.CheckScan(t, ctx, c, syncbase.Range("bar", "bar"), []string{}, []interface{}{})
	tu.CheckScan(t, ctx, c, syncbase.Prefix("z"), []string{}, []interface{}{})
}

// Tests that Collection.DeleteRange works as expected.
func TestCollectionDeleteRange(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{}, []interface{}{})

	// Put foo and bar.
	fooWant := Foo{I: 4, S: "f"}
	if err := c.Put(ctx, "foo", &fooWant); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	barWant := Bar{F: 0.5, S: "b"}
	if err := c.Put(ctx, "bar", &barWant); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})

	// Delete foo.
	if err := c.DeleteRange(ctx, syncbase.Prefix("f")); err != nil {
		t.Fatalf("c.DeleteRange() failed: %v", err)
	}
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{"bar"}, []interface{}{&barWant})

	// Restore foo.
	if err := c.Put(ctx, "foo", &fooWant); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})

	// Delete everything.
	if err := c.DeleteRange(ctx, syncbase.Prefix("")); err != nil {
		t.Fatalf("c.DeleteRange() failed: %v", err)
	}
	tu.CheckScan(t, ctx, c, syncbase.Prefix(""), []string{}, []interface{}{})
}

// Tests that Collection.{Get,Put,Delete} work as expected.
func TestCollectionRowMethods(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	got, want := Foo{}, Foo{I: 4, S: "foo"}
	if err := c.Get(ctx, "f", &got); verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("c.Get() should have failed: %v", err)
	}
	if err := c.Put(ctx, "f", &want); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	if err := c.Get(ctx, "f", &got); err != nil {
		t.Fatalf("c.Get() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Values do not match: got %v, want %v", got, want)
	}
	// Overwrite value.
	want.I = 6
	if err := c.Put(ctx, "f", &want); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	if err := c.Get(ctx, "f", &got); err != nil {
		t.Fatalf("c.Get() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Values do not match: got %v, want %v", got, want)
	}
	if err := c.Delete(ctx, "f"); err != nil {
		t.Fatalf("c.Delete() failed: %v", err)
	}
	if err := c.Get(ctx, "f", &got); verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("r.Get() should have failed: %v", err)
	}
}

// Tests that Row.{Get,Put,Delete} work as expected.
func TestRowMethods(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	r := c.Row("f")
	got, want := Foo{}, Foo{I: 4, S: "foo"}
	if err := r.Get(ctx, &got); verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("r.Get() should have failed: %v", err)
	}
	if err := r.Put(ctx, &want); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}
	if err := r.Get(ctx, &got); err != nil {
		t.Fatalf("r.Get() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Values do not match: got %v, want %v", got, want)
	}
	// Overwrite value.
	want.I = 6
	if err := r.Put(ctx, &want); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}
	if err := r.Get(ctx, &got); err != nil {
		t.Fatalf("r.Get() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Values do not match: got %v, want %v", got, want)
	}
	if err := r.Delete(ctx); err != nil {
		t.Fatalf("r.Delete() failed: %v", err)
	}
	if err := r.Get(ctx, &got); verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("r.Get() should have failed: %v", err)
	}
}

// Tests name-checking on row creation.
func TestRowNameValidation(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
	c := tu.CreateCollection(t, ctx, d, "c")
	tu.TestCreateNameValidation(t, ctx, c, tu.OkRowNames, tu.NotOkRowNames)
}

// Test permission checking in Row.{Get,Put,Delete} and
// Collection.{Scan, DeleteRange}.
func TestRowPermissions(t *testing.T) {
	ctx, clientACtx, sName, rootp, cleanup := tu.SetupOrDieCustom("clientA", "server", nil)
	defer cleanup()
	clientBCtx := tu.NewCtx(ctx, rootp, "clientB")
	a := tu.CreateApp(t, clientACtx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, clientACtx, a, "d")
	c := tu.CreateCollection(t, clientACtx, d, "c")

	// Permission objects.
	aAndB := tu.DefaultPerms("root:clientA", "root:clientB")
	aOnly := tu.DefaultPerms("root:clientA")
	bOnly := tu.DefaultPerms("root:clientB")

	// Set initial permissions.
	if err := c.SetPermissions(clientACtx, aAndB); err != nil {
		t.Fatalf("c.SetPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientACtx, syncbase.Prefix(""), aAndB); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientACtx, syncbase.Prefix("a"), aOnly); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientBCtx, syncbase.Prefix("b"), bOnly); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}

	// Add some key-value pairs.
	ra := c.Row("afoo")
	rb := c.Row("bfoo")
	if err := ra.Put(clientACtx, Foo{}); err != nil {
		t.Fatalf("ra.Put() failed: %v", err)
	}
	if err := rb.Put(clientBCtx, Foo{}); err != nil {
		t.Fatalf("rb.Put() failed: %v", err)
	}

	// Check A doesn't have access to 'b'.
	if err := rb.Get(clientACtx, &Foo{}); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("rb.Get() should have failed: %v", err)
	}
	if err := rb.Put(clientACtx, Foo{}); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("rb.Put() should have failed: %v", err)
	}
	if err := rb.Delete(clientACtx); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("rb.Delete() should have failed: %v", err)
	}
	// Test Collection.DeleteRange and Scan.
	if err := c.DeleteRange(clientACtx, syncbase.Prefix("")); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("c.DeleteRange should have failed: %v", err)
	}
	s := c.Scan(clientACtx, syncbase.Prefix(""))
	if !s.Advance() {
		t.Fatalf("Stream should have advanced: %v", s.Err())
	}
	if s.Key() != "afoo" {
		t.Fatalf("Unexpected key: got %q, want %q", s.Key(), "afoo")
	}
	if s.Advance() {
		t.Fatalf("Stream advanced unexpectedly")
	}
	if err := s.Err(); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("Unexpected stream error: %v", err)
	}
}

// Tests prefix perms where get is allowed but put is not.
func TestMixedPrefixPerms(t *testing.T) {
	ctx, clientACtx, sName, rootp, cleanup := tu.SetupOrDieCustom("clientA", "server", nil)
	defer cleanup()
	clientBCtx := tu.NewCtx(ctx, rootp, "clientB")
	a := tu.CreateApp(t, clientACtx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, clientACtx, a, "d")
	c := tu.CreateCollection(t, clientACtx, d, "c")

	// Permission objects.
	aAndB := tu.DefaultPerms("root:clientA", "root:clientB")
	aAllBRead := tu.DefaultPerms("root:clientA")
	aAllBRead.Add(security.BlessingPattern("root:clientB"), string(access.Read))

	// Set initial permissions.
	if err := c.SetPermissions(clientACtx, aAndB); err != nil {
		t.Fatalf("c.SetPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientACtx, syncbase.Prefix(""), aAndB); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientACtx, syncbase.Prefix("a"), aAllBRead); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}

	// Both A and B can read and write row key "z".
	r := c.Row("z")
	if err := r.Put(clientACtx, Foo{}); err != nil {
		t.Fatalf("client A r.Put() failed: %v", err)
	}
	if err := r.Put(clientBCtx, Foo{}); err != nil {
		t.Fatalf("client B r.Put() failed: %v", err)
	}
	if err := r.Get(clientACtx, &Foo{}); err != nil {
		t.Fatalf("client A r.Get() failed: %v", err)
	}
	if err := r.Get(clientBCtx, &Foo{}); err != nil {
		t.Fatalf("client B r.Get() failed: %v", err)
	}

	// Both A and B can read row key "a", but only A can write it.
	r = c.Row("a")
	if err := r.Put(clientACtx, Foo{}); err != nil {
		t.Fatalf("client A r.Put() failed: %v", err)
	}
	if err := r.Put(clientBCtx, Foo{}); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("client B r.Put() should have failed: %v", err)
	}
	if err := r.Get(clientACtx, &Foo{}); err != nil {
		t.Fatalf("client A r.Get() failed: %v", err)
	}
	if err := r.Get(clientBCtx, &Foo{}); err != nil {
		t.Fatalf("client B r.Get() failed: %v", err)
	}

	// Same for a:foo.
	r = c.Row("a:foo")
	if err := r.Put(clientACtx, Foo{}); err != nil {
		t.Fatalf("client A r.Put() failed: %v", err)
	}
	if err := r.Put(clientBCtx, Foo{}); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("client B r.Put() should have failed: %v", err)
	}
	if err := r.Get(clientACtx, &Foo{}); err != nil {
		t.Fatalf("client A r.Get() failed: %v", err)
	}
	if err := r.Get(clientBCtx, &Foo{}); err != nil {
		t.Fatalf("client B r.Get() failed: %v", err)
	}
}

// Tests prefix perms where client A has full access to everything except for a
// particular prefix, while client B can only read/write that prefix.
// Originally inspired by syncslides example app.
func TestNestedMixedPrefixPerms(t *testing.T) {
	ctx, clientACtx, sName, rootp, cleanup := tu.SetupOrDieCustom("clientA", "server", nil)
	defer cleanup()
	clientBCtx := tu.NewCtx(ctx, rootp, "clientB")
	a := tu.CreateApp(t, clientACtx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, clientACtx, a, "d")
	c := tu.CreateCollection(t, clientACtx, d, "c")

	// On "", resolve should be open, and only A should have read/write/admin.
	// On "b", read/resolve should be open, and only B should have write/admin.
	prefixEmpty, err := access.ReadPermissions(strings.NewReader(`{"Read":{"In":["root:clientA"],"NotIn":[]},"Admin":{"In":["root:clientA"],"NotIn":[]},"Resolve":{"In":["..."],"NotIn":[]},"Write":{"In":["root:clientA"],"NotIn":[]}}`))
	if err != nil {
		t.Fatalf("ReadPermissions failed: %v", err)
	}
	prefixB, err := access.ReadPermissions(strings.NewReader(`{"Admin":{"In":["root:clientB"],"NotIn":[]},"Read":{"In":["..."],"NotIn":[]},"Write":{"In":["root:clientB"],"NotIn":[]},"Resolve":{"In":["..."],"NotIn":[]}}}`))
	if err != nil {
		t.Fatalf("ReadPermissions failed: %v", err)
	}

	// Set initial permissions.
	aAndB := tu.DefaultPerms("root:clientA", "root:clientB")
	if err := c.SetPermissions(clientACtx, aAndB); err != nil {
		t.Fatalf("c.SetPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientACtx, syncbase.Prefix(""), prefixEmpty); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}
	// Note that with the current perms, A must write these perms on B's behalf.
	if err := c.SetPrefixPermissions(clientACtx, syncbase.Prefix("b"), prefixB); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}

	// A should be able to read/write "a".
	// B should be able to to read/write "b".
	// B should not be able to read/write "a".
	// A should not be able to write "b", but should be able to read it.

	r := c.Row("a")
	if err := r.Put(clientACtx, Foo{}); err != nil {
		t.Fatalf("client A r.Put() failed: %v", err)
	}
	if err := r.Get(clientACtx, &Foo{}); err != nil {
		t.Fatalf("client A r.Get() failed: %v", err)
	}

	r = c.Row("b")
	if err := r.Put(clientBCtx, Foo{}); err != nil {
		t.Fatalf("client B r.Put() failed: %v", err)
	}
	if err := r.Get(clientBCtx, &Foo{}); err != nil {
		t.Fatalf("client B r.Get() failed: %v", err)
	}

	r = c.Row("a")
	if err := r.Put(clientBCtx, Foo{}); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("client B r.Put() should have failed: %v", err)
	}
	if err := r.Get(clientBCtx, &Foo{}); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("client B r.Get() should have failed: %v", err)
	}

	r = c.Row("b")
	if err := r.Put(clientACtx, Foo{}); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("client A r.Put() should have failed: %v", err)
	}
	if err := r.Get(clientACtx, &Foo{}); err != nil {
		t.Fatalf("client A r.Get() failed: %v", err)
	}
}

// TestWatchBasic test the basic client watch functionality: no perms,
// no batches.
func TestWatchBasic(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
	c := tu.CreateCollection(t, ctx, d, "c")
	var resumeMarkers []watch.ResumeMarker

	// Generate the data and resume markers.
	// Initial state.
	resumeMarker, err := d.GetResumeMarker(ctx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	resumeMarkers = append(resumeMarkers, resumeMarker)
	// Put "abc".
	r := c.Row("abc")
	if err := r.Put(ctx, "value"); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}
	if resumeMarker, err = d.GetResumeMarker(ctx); err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	resumeMarkers = append(resumeMarkers, resumeMarker)
	// Delete "abc".
	if err := r.Delete(ctx); err != nil {
		t.Fatalf("r.Delete() failed: %v", err)
	}
	if resumeMarker, err = d.GetResumeMarker(ctx); err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	resumeMarkers = append(resumeMarkers, resumeMarker)
	// Put "a".
	r = c.Row("a")
	if err := r.Put(ctx, "value"); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}
	if resumeMarker, err = d.GetResumeMarker(ctx); err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	resumeMarkers = append(resumeMarkers, resumeMarker)

	vomValue, _ := vom.Encode("value")
	allChanges := []syncbase.WatchChange{
		syncbase.WatchChange{
			Collection:   "c",
			Row:          "abc",
			ChangeType:   syncbase.PutChange,
			ValueBytes:   vomValue,
			ResumeMarker: resumeMarkers[1],
		},
		syncbase.WatchChange{
			Collection:   "c",
			Row:          "abc",
			ChangeType:   syncbase.DeleteChange,
			ResumeMarker: resumeMarkers[2],
		},
		syncbase.WatchChange{
			Collection:   "c",
			Row:          "a",
			ChangeType:   syncbase.PutChange,
			ValueBytes:   vomValue,
			ResumeMarker: resumeMarkers[3],
		},
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	wstream, _ := d.Watch(ctxWithTimeout, "c", "a", resumeMarkers[0])
	tu.CheckWatch(t, wstream, allChanges)
	wstream, _ = d.Watch(ctxWithTimeout, "c", "a", resumeMarkers[1])
	tu.CheckWatch(t, wstream, allChanges[1:])
	wstream, _ = d.Watch(ctxWithTimeout, "c", "a", resumeMarkers[2])
	tu.CheckWatch(t, wstream, allChanges[2:])

	wstream, _ = d.Watch(ctxWithTimeout, "c", "abc", resumeMarkers[0])
	tu.CheckWatch(t, wstream, allChanges[:2])
	wstream, _ = d.Watch(ctxWithTimeout, "c", "abc", resumeMarkers[1])
	tu.CheckWatch(t, wstream, allChanges[1:2])
}

// TestWatchWithBatchAndPerms test that the client watch correctly handles
// batches and prefix perms.
func TestWatchWithBatchAndPerms(t *testing.T) {
	ctx, clientACtx, sName, rootp, cleanup := tu.SetupOrDieCustom("clientA", "server", nil)
	defer cleanup()
	clientBCtx := tu.NewCtx(ctx, rootp, "clientB")
	a := tu.CreateApp(t, clientACtx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, clientACtx, a, "d")
	c := tu.CreateCollection(t, clientACtx, d, "c")

	// Set initial permissions.
	aAndB := tu.DefaultPerms("root:clientA", "root:clientB")
	aOnly := tu.DefaultPerms("root:clientA")
	if err := c.SetPermissions(clientACtx, aAndB); err != nil {
		t.Fatalf("c.SetPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientACtx, syncbase.Prefix(""), aAndB); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(clientACtx, syncbase.Prefix("a"), aOnly); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}
	// Get the initial resume marker.
	resumeMarker, err := d.GetResumeMarker(clientACtx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	initMarker := resumeMarker
	// Do two puts in a batch.
	if err := syncbase.RunInBatch(clientACtx, d, wire.BatchOptions{}, func(b syncbase.BatchDatabase) error {
		c := b.Collection("c")
		if err := c.Put(clientACtx, "a", "value"); err != nil {
			return err
		}
		return c.Put(clientACtx, "b", "value")
	}); err != nil {
		t.Fatalf("RunInBatch failed: %v", err)
	}

	if resumeMarker, err = d.GetResumeMarker(clientACtx); err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	vomValue, _ := vom.Encode("value")
	allChanges := []syncbase.WatchChange{
		syncbase.WatchChange{
			Collection:   "c",
			Row:          "a",
			ChangeType:   syncbase.PutChange,
			ValueBytes:   vomValue,
			ResumeMarker: nil,
			Continued:    true,
		},
		syncbase.WatchChange{
			Collection:   "c",
			Row:          "b",
			ChangeType:   syncbase.PutChange,
			ValueBytes:   vomValue,
			ResumeMarker: resumeMarker,
		},
	}

	ctxAWithTimeout, cancelA := context.WithTimeout(clientACtx, 10*time.Second)
	defer cancelA()
	ctxBWithTimeout, cancelB := context.WithTimeout(clientBCtx, 10*time.Second)
	defer cancelB()
	// ClientA should see both changes as one batch.
	wstream, _ := d.Watch(ctxAWithTimeout, "c", "", initMarker)
	tu.CheckWatch(t, wstream, allChanges)
	// ClientB should see only one change.
	wstream, _ = d.Watch(ctxBWithTimeout, "c", "", initMarker)
	tu.CheckWatch(t, wstream, allChanges[1:])
}

// TestWatchWithInitialState test that the client watch correctly handles
// fetching initial state on empty resume marker, including batches and
// prefix perms.
func TestWatchWithInitialState(t *testing.T) {
	ctx, adminCtx, sName, rootp, cleanup := tu.SetupOrDieCustom("admin", "server", nil)
	defer cleanup()
	clientCtx := tu.NewCtx(ctx, rootp, "client")
	a := tu.CreateApp(t, adminCtx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, adminCtx, a, "d")
	c := tu.CreateCollection(t, adminCtx, d, "c")

	// Set permissions. Lock client out of "b".
	openAcl := tu.DefaultPerms("root:admin", "root:client")
	adminAcl := tu.DefaultPerms("root:admin")
	if err := c.SetPermissions(adminCtx, openAcl); err != nil {
		t.Fatalf("c.SetPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(adminCtx, syncbase.Prefix(""), openAcl); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}
	if err := c.SetPrefixPermissions(adminCtx, syncbase.Prefix("b"), adminAcl); err != nil {
		t.Fatalf("c.SetPrefixPermissions() failed: %v", err)
	}

	// Put "a/1" and "b/1" in a batch.
	if err := syncbase.RunInBatch(adminCtx, d, wire.BatchOptions{}, func(b syncbase.BatchDatabase) error {
		c := b.Collection("c")
		if err := c.Put(adminCtx, "a/1", "value"); err != nil {
			return err
		}
		return c.Put(adminCtx, "b/1", "value")
	}); err != nil {
		t.Fatalf("RunInBatch failed: %v", err)
	}
	// Put "c/1".
	if err := c.Put(adminCtx, "c/1", "value"); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}

	ctxWithTimeout, cancel := context.WithTimeout(clientCtx, 10*time.Second)
	defer cancel()
	// Start watches with empty resume marker.
	wstreamAll, _ := d.Watch(ctxWithTimeout, "c", "", nil)
	wstreamD, _ := d.Watch(ctxWithTimeout, "c", "d", nil)

	resumeMarkerInitial, err := d.GetResumeMarker(clientCtx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	vomValue, _ := vom.Encode("value")
	initialChanges := []syncbase.WatchChange{
		syncbase.WatchChange{
			Collection:   "c",
			Row:          "a/1",
			ChangeType:   syncbase.PutChange,
			ValueBytes:   vomValue,
			ResumeMarker: nil,
			Continued:    true,
		},
		syncbase.WatchChange{
			Collection:   "c",
			Row:          "c/1",
			ChangeType:   syncbase.PutChange,
			ValueBytes:   vomValue,
			ResumeMarker: resumeMarkerInitial,
		},
	}
	// Watch with empty prefix should have seen the initial state as one batch,
	// omitting the inaccessible row.
	tu.CheckWatch(t, wstreamAll, initialChanges)

	// More writes.
	// Put "a/2" and "b/2" in a batch.
	if err := syncbase.RunInBatch(adminCtx, d, wire.BatchOptions{}, func(b syncbase.BatchDatabase) error {
		c := b.Collection("c")
		if err := c.Put(adminCtx, "a/2", "value"); err != nil {
			return err
		}
		return c.Put(adminCtx, "b/2", "value")
	}); err != nil {
		t.Fatalf("RunInBatch failed: %v", err)
	}
	resumeMarkerAfterA2B2, err := d.GetResumeMarker(clientCtx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	// Put "d/1".
	if err := c.Put(adminCtx, "d/1", "value"); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	resumeMarkerAfterD1, err := d.GetResumeMarker(clientCtx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}

	continuedChanges := []syncbase.WatchChange{
		syncbase.WatchChange{
			Collection:   "c",
			Row:          "a/2",
			ChangeType:   syncbase.PutChange,
			ValueBytes:   vomValue,
			ResumeMarker: resumeMarkerAfterA2B2,
		},
		syncbase.WatchChange{
			Collection:   "c",
			Row:          "d/1",
			ChangeType:   syncbase.PutChange,
			ValueBytes:   vomValue,
			ResumeMarker: resumeMarkerAfterD1,
		},
	}
	// Watch with empty prefix should have seen the continued changes as separate
	// batches, omitting inaccessible rows.
	tu.CheckWatch(t, wstreamAll, continuedChanges)
	// Watch with prefix "d" should have seen only the last change; its initial
	// state was empty.
	tu.CheckWatch(t, wstreamD, continuedChanges[1:])
}

// TestBlockingWatch tests that the server side of the client watch correctly
// blocks until new updates to the database arrive.
func TestBlockingWatch(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	resumeMarker, err := d.GetResumeMarker(ctx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	wstream, _ := d.Watch(ctxWithTimeout, "c", "a", resumeMarker)
	vomValue, _ := vom.Encode("value")
	for i := 0; i < 10; i++ {
		// Put "abc".
		r := c.Row("abc")
		if err := r.Put(ctx, "value"); err != nil {
			t.Fatalf("r.Put() failed: %v", err)
		}
		if resumeMarker, err = d.GetResumeMarker(ctx); err != nil {
			t.Fatalf("d.GetResumeMarker() failed: %v", err)
		}
		if !wstream.Advance() {
			t.Fatalf("wstream.Advance() reached the end: %v", wstream.Err())
		}
		want := syncbase.WatchChange{
			Collection:   "c",
			Row:          "abc",
			ChangeType:   syncbase.PutChange,
			ValueBytes:   vomValue,
			ResumeMarker: resumeMarker,
		}
		if got := wstream.Change(); !reflect.DeepEqual(got, want) {
			t.Fatalf("unexpected watch change: got %v, want %v", got, want)
		}
	}
}

// TestBlockedWatchCancel tests that the watch call blocked on the server side
// can be successfully canceled from the client.
func TestBlockedWatchCancel(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")

	resumeMarker, err := d.GetResumeMarker(ctx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	ctxCancel, cancel := context.WithCancel(ctx)
	wstream, err := d.Watch(ctxCancel, "c", "a", resumeMarker)
	if err != nil {
		t.Fatalf("d.Watch() failed: %v", err)
	}
	time.AfterFunc(500*time.Millisecond, cancel)
	if wstream.Advance() {
		t.Fatal("wstream should not have advanced")
	}
	if got, want := verror.ErrorID(wstream.Err()), verror.ErrCanceled.ID; got != want {
		t.Errorf("unexpected wstream error ID: got %v, want %v", got, want)
	}
}
