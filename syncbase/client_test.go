// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase_test

import (
	"reflect"
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

// Tests various Id, Name, FullName, and Key methods.
func TestIdNameAndKey(t *testing.T) {
	s := syncbase.NewService("s")
	d := s.DatabaseForId(wire.Id{"a", "d"}, nil)
	c := d.Collection("c")
	r := c.Row("r")

	if s.FullName() != "s" {
		t.Errorf("Wrong full name: %q", s.FullName())
	}
	if d.Id() != (wire.Id{"a", "d"}) {
		t.Errorf("Wrong id: %q", d.Id())
	}
	if d.FullName() != naming.Join("s", "a,d") {
		t.Errorf("Wrong full name: %q", d.FullName())
	}
	if c.Name() != "c" {
		t.Errorf("Wrong name: %q", c.Name())
	}
	if c.FullName() != naming.Join("s", "a,d", "c") {
		t.Errorf("Wrong full name: %q", c.FullName())
	}
	if r.Key() != "r" {
		t.Errorf("Wrong key: %q", r.Key())
	}
	if r.FullName() != naming.Join("s", "a,d", "c", "r") {
		t.Errorf("Wrong full name: %q", r.FullName())
	}
}

// Tests that Service.ListDatabases works as expected.
func TestListDatabases(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	tu.TestListChildIds(t, ctx, syncbase.NewService(sName), tu.OkAppBlessings, tu.OkDbNames)
}

// Tests that Service.{Set,Get}Permissions work as expected.
func TestServicePerms(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	tu.TestPerms(t, ctx, syncbase.NewService(sName))
}

// Tests that Database.Create works as expected.
func TestDatabaseCreate(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	tu.TestCreate(t, ctx, syncbase.NewService(sName))
}

// Tests name-checking on database creation.
// TODO(sadovsky): Also test blessing validation. We should rewrite some of
// these tests. Let's do this after we update collections to use ids.
func TestDatabaseCreateNameValidation(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	tu.TestCreateNameValidation(t, ctx, syncbase.NewService(sName), tu.OkDbNames, tu.NotOkDbNames)
}

// Tests that Database.Destroy works as expected.
func TestDatabaseDestroy(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	tu.TestDestroy(t, ctx, syncbase.NewService(sName))
}

// Tests that Database.Exec works as expected.
// Note: The Exec method is tested more thoroughly in exec_test.go.
// Also, the query package is tested in its entirety in v23/query/...
func TestExec(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
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
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	tu.TestListChildren(t, ctx, d, tu.OkCollectionNames)
}

// Tests that Database.{Set,Get}Permissions work as expected.
func TestDatabasePerms(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	tu.TestPerms(t, ctx, d)
}

// Tests that Collection.Create works as expected.
func TestCollectionCreate(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	tu.TestCreate(t, ctx, d)
}

// Tests name-checking on collection creation.
func TestCollectionCreateNameValidation(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	tu.TestCreateNameValidation(t, ctx, d, tu.OkCollectionNames, tu.NotOkCollectionNames)
}

// Tests that Collection.Destroy works as expected.
func TestCollectionDestroy(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	tu.TestDestroy(t, ctx, d)
}

// Tests that Collection.Destroy deletes all rows in the collection.
func TestCollectionDestroyAndRecreate(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")
	// Write some data.
	if err := c.Put(ctx, "bar/baz", "A"); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	if err := c.Put(ctx, "foo", "B"); err != nil {
		t.Fatalf("c.Put() failed: %v", err)
	}
	/*
		// Remove write permissions from collection.
		fullPerms := tu.DefaultPerms("root:client").Normalize()
		readPerms := fullPerms.Copy()
		readPerms.Clear("root:client", string(access.Write))
		readPerms.Normalize()
		if err := c.SetPermissions(ctx, readPerms); err != nil {
			t.Fatalf("c.SetPermissions() failed: %v", err)
		}
		// Verify we have no write access to "bar" anymore.
		if err := c.Put(ctx, "bar/bat", "C"); verror.ErrorID(err) != verror.ErrNoAccess.ID {
			t.Fatalf("c.Put() should have failed with ErrNoAccess, got: %v", err)
		}
	*/
	// Destroy collection. Destroy needs only admin permissions on the collection,
	// so it shouldn't be affected by the lack of Write access.
	// TODO(ivanpi): Destroy currently needs Write, reenable code above when fixed.
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
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
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
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
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
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
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
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
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
func TestRowKeyValidation(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")
	tu.TestCreateNameValidation(t, ctx, c, tu.OkRowKeys, tu.NotOkRowKeys)
}

// Test permission checking in Row.{Get,Put,Delete} and
// Collection.{Scan, DeleteRange}.
// TODO(ivanpi): Redundant with permissions_test?
func TestRowPermissions(t *testing.T) {
	_, clientACtx, sName, _, cleanup := tu.SetupOrDieCustom("clientA", "server", nil)
	defer cleanup()
	d := tu.CreateDatabase(t, clientACtx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, clientACtx, d, "c")

	// Add some key-value pairs.
	r := c.Row("foo")
	if err := r.Put(clientACtx, Foo{}); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}

	// Lock A out of c.
	bOnly := tu.DefaultPerms("root:clientB")
	if err := c.SetPermissions(clientACtx, bOnly); err != nil {
		t.Fatalf("c.SetPermissions() failed: %v", err)
	}

	// Check A doesn't have access.
	if err := r.Get(clientACtx, &Foo{}); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("rb.Get() should have failed: %v", err)
	}
	if err := r.Put(clientACtx, Foo{}); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("rb.Put() should have failed: %v", err)
	}
	if err := r.Delete(clientACtx); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("rb.Delete() should have failed: %v", err)
	}
	// Test Collection.DeleteRange and Scan.
	if err := c.DeleteRange(clientACtx, syncbase.Prefix("")); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("c.DeleteRange should have failed: %v", err)
	}
	s := c.Scan(clientACtx, syncbase.Prefix(""))
	if s.Advance() {
		t.Fatalf("Stream advanced unexpectedly")
	}
	if err := s.Err(); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("Unexpected stream error: %v", err)
	}
}

// Tests collection perms where get is allowed but put is not.
// TODO(ivanpi): Redundant with permissions_test?
func TestMixedCollectionPerms(t *testing.T) {
	ctx, clientACtx, sName, rootp, cleanup := tu.SetupOrDieCustom("clientA", "server", nil)
	defer cleanup()
	clientBCtx := tu.NewCtx(ctx, rootp, "clientB")
	d := tu.CreateDatabase(t, clientACtx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, clientACtx, d, "c")

	// Set permissions.
	aAllBRead := tu.DefaultPerms("root:clientA")
	aAllBRead.Add(security.BlessingPattern("root:clientB"), string(access.Read))
	if err := c.SetPermissions(clientACtx, aAllBRead); err != nil {
		t.Fatalf("c.SetPermissions() failed: %v", err)
	}

	// Test GetPermissions.
	if got, err := c.GetPermissions(clientACtx); err != nil {
		t.Fatalf("c.GetPermissions() failed: %v", err)
	} else if want := aAllBRead; !reflect.DeepEqual(got, want) {
		t.Fatalf("Unexpected collection permissions: got %v, want %v", got, want)
	}

	// Both A and B can read row key "a", but only A can write it.
	r := c.Row("a")
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

// TestWatchBasic test the basic client watch functionality: no perms,
// no batches.
func TestWatchBasic(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
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

	valueBytes, _ := vom.RawBytesFromValue("value")
	allChanges := []tu.WatchChangeTest{
		tu.WatchChangeTest{
			WatchChange: syncbase.WatchChange{
				Collection:   "c",
				Row:          "abc",
				ChangeType:   syncbase.PutChange,
				ResumeMarker: resumeMarkers[1],
			},
			ValueBytes: valueBytes,
		},
		tu.WatchChangeTest{
			WatchChange: syncbase.WatchChange{
				Collection:   "c",
				Row:          "abc",
				ChangeType:   syncbase.DeleteChange,
				ResumeMarker: resumeMarkers[2],
			},
		},
		tu.WatchChangeTest{
			WatchChange: syncbase.WatchChange{
				Collection:   "c",
				Row:          "a",
				ChangeType:   syncbase.PutChange,
				ResumeMarker: resumeMarkers[3],
			},
			ValueBytes: valueBytes,
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

// TestWatchWithBatchAndInitialState test that the client watch correctly
// handles batches and fetching initial state on empty resume marker.
func TestWatchWithBatchAndInitialState(t *testing.T) {
	ctx, adminCtx, sName, rootp, cleanup := tu.SetupOrDieCustom("admin", "server", nil)
	defer cleanup()
	clientCtx := tu.NewCtx(ctx, rootp, "client")
	d := tu.CreateDatabase(t, adminCtx, syncbase.NewService(sName), "d")
	cp := tu.CreateCollection(t, adminCtx, d, "cpublic")
	ch := tu.CreateCollection(t, adminCtx, d, "chidden")

	// Set permissions. Lock client out of ch.
	openAcl := tu.DefaultPerms("root:admin", "root:client")
	adminAcl := tu.DefaultPerms("root:admin")
	if err := cp.SetPermissions(adminCtx, openAcl); err != nil {
		t.Fatalf("cp.SetPermissions() failed: %v", err)
	}
	if err := ch.SetPermissions(adminCtx, adminAcl); err != nil {
		t.Fatalf("ch.SetPermissions() failed: %v", err)
	}

	// Put cp:"a/1" and ch:"b/1" in a batch.
	if err := syncbase.RunInBatch(adminCtx, d, wire.BatchOptions{}, func(b syncbase.BatchDatabase) error {
		cp := b.Collection("cpublic")
		if err := cp.Put(adminCtx, "a/1", "value"); err != nil {
			return err
		}
		ch := b.Collection("chidden")
		return ch.Put(adminCtx, "b/1", "value")
	}); err != nil {
		t.Fatalf("RunInBatch failed: %v", err)
	}
	// Put cp:"c/1".
	if err := cp.Put(adminCtx, "c/1", "value"); err != nil {
		t.Fatalf("cp.Put() failed: %v", err)
	}

	ctxWithTimeout, cancel := context.WithTimeout(clientCtx, 10*time.Second)
	defer cancel()
	// Start watches with empty resume marker.
	// TODO(ivanpi): Empty prefix watch should watch both collections using both
	// admin and client contexts, checking that chidden updates are visible only
	// to admin.
	wstreamAll, _ := d.Watch(ctxWithTimeout, "cpublic", "", nil)
	wstreamD, _ := d.Watch(ctxWithTimeout, "cpublic", "d", nil)

	resumeMarkerInitial, err := d.GetResumeMarker(clientCtx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	valueBytes, _ := vom.RawBytesFromValue("value")
	initialChanges := []tu.WatchChangeTest{
		tu.WatchChangeTest{
			WatchChange: syncbase.WatchChange{
				Collection:   "cpublic",
				Row:          "a/1",
				ChangeType:   syncbase.PutChange,
				ResumeMarker: nil,
				Continued:    true,
			},
			ValueBytes: valueBytes,
		},
		tu.WatchChangeTest{
			WatchChange: syncbase.WatchChange{
				Collection:   "cpublic",
				Row:          "c/1",
				ChangeType:   syncbase.PutChange,
				ResumeMarker: resumeMarkerInitial,
			},
			ValueBytes: valueBytes,
		},
	}
	// Watch with empty prefix should have seen the initial state as one batch,
	// omitting the row in the secret collection.
	tu.CheckWatch(t, wstreamAll, initialChanges)

	// More writes.
	// Put cp:"a/2" and cs:"b/2" in a batch.
	if err := syncbase.RunInBatch(adminCtx, d, wire.BatchOptions{}, func(b syncbase.BatchDatabase) error {
		cp := b.Collection("cpublic")
		if err := cp.Put(adminCtx, "a/2", "value"); err != nil {
			return err
		}
		ch := b.Collection("chidden")
		return ch.Put(adminCtx, "b/2", "value")
	}); err != nil {
		t.Fatalf("RunInBatch failed: %v", err)
	}
	resumeMarkerAfterA2B2, err := d.GetResumeMarker(clientCtx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	// Put cp:"a/3" and cp:"d/1" in a batch.
	if err := syncbase.RunInBatch(adminCtx, d, wire.BatchOptions{}, func(b syncbase.BatchDatabase) error {
		cp := b.Collection("cpublic")
		if err := cp.Put(adminCtx, "a/3", "value"); err != nil {
			return err
		}
		return cp.Put(adminCtx, "d/1", "value")
	}); err != nil {
		t.Fatalf("RunInBatch failed: %v", err)
	}
	resumeMarkerAfterA3D1, err := d.GetResumeMarker(clientCtx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}

	continuedChanges := []tu.WatchChangeTest{
		tu.WatchChangeTest{
			WatchChange: syncbase.WatchChange{
				Collection:   "cpublic",
				Row:          "a/2",
				ChangeType:   syncbase.PutChange,
				ResumeMarker: resumeMarkerAfterA2B2,
			},
			ValueBytes: valueBytes,
		},
		tu.WatchChangeTest{
			WatchChange: syncbase.WatchChange{
				Collection:   "cpublic",
				Row:          "a/3",
				ChangeType:   syncbase.PutChange,
				ResumeMarker: nil,
				Continued:    true,
			},
			ValueBytes: valueBytes,
		},
		tu.WatchChangeTest{
			WatchChange: syncbase.WatchChange{
				Collection:   "cpublic",
				Row:          "d/1",
				ChangeType:   syncbase.PutChange,
				ResumeMarker: resumeMarkerAfterA3D1,
			},
			ValueBytes: valueBytes,
		},
	}
	// Watch with empty prefix should have seen the continued changes as separate
	// batches, omitting rows in the secret collection.
	tu.CheckWatch(t, wstreamAll, continuedChanges)
	// Watch with prefix "d" should have seen only the last change; its initial
	// state was empty.
	tu.CheckWatch(t, wstreamD, continuedChanges[2:])
}

// TestBlockingWatch tests that the server side of the client watch correctly
// blocks until new updates to the database arrive.
func TestBlockingWatch(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	c := tu.CreateCollection(t, ctx, d, "c")

	resumeMarker, err := d.GetResumeMarker(ctx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	wstream, _ := d.Watch(ctxWithTimeout, "c", "a", resumeMarker)
	valueBytes, _ := vom.RawBytesFromValue("value")
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
		want := tu.WatchChangeTest{
			WatchChange: syncbase.WatchChange{
				Collection:   "c",
				Row:          "abc",
				ChangeType:   syncbase.PutChange,
				ResumeMarker: resumeMarker,
			},
			ValueBytes: valueBytes,
		}
		if got := wstream.Change(); !tu.WatchChangeEq(&got, &want) {
			t.Fatalf("Unexpected watch change: got %v, want %v", got, want)
		}
	}
}

// TestBlockedWatchCancel tests that the watch call blocked on the server side
// can be successfully canceled from the client.
func TestBlockedWatchCancel(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	d := tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")

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
		t.Errorf("Unexpected wstream error ID: got %v, want %v", got, want)
	}
}
