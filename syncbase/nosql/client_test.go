// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql_test

import (
	"reflect"
	"testing"

	"v.io/syncbase/v23/syncbase"
	"v.io/syncbase/v23/syncbase/nosql"
	"v.io/syncbase/v23/syncbase/nosql/syncql"
	tu "v.io/syncbase/v23/syncbase/testutil"
	"v.io/v23/naming"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/generic"
)

// TODO(sadovsky): Finish writing tests.
// TODO(rogulenko): Test perms checking for Glob and Exec.

// Tests various Name, FullName, and Key methods.
func TestNameAndKey(t *testing.T) {
	d := syncbase.NewService("s").App("a").NoSQLDatabase("d", nil)
	tb := d.Table("tb")
	r := tb.Row("r")

	if d.Name() != "d" {
		t.Errorf("Wrong name: %q", d.Name())
	}
	if d.FullName() != naming.Join("s", "a", "d") {
		t.Errorf("Wrong full name: %q", d.FullName())
	}
	if tb.Name() != "tb" {
		t.Errorf("Wrong name: %q", tb.Name())
	}
	if tb.FullName() != naming.Join("s", "a", "d", "tb") {
		t.Errorf("Wrong full name: %q", tb.FullName())
	}
	if r.Key() != "r" {
		t.Errorf("Wrong key: %q", r.Key())
	}
	if r.FullName() != naming.Join("s", "a", "d", "tb", "r") {
		t.Errorf("Wrong full name: %q", r.FullName())
	}
}

// Tests that Database.Create works as expected.
func TestDatabaseCreate(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	tu.TestCreate(t, ctx, a)
}

// Tests that Database.Exec works as expected.
// Note: More comprehensive client/server tests are in the exec_test
// directory.  Also, exec is tested in its entirety in
// v23/syncbase/nosql/internal/query/...
func TestExec(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tb := tu.CreateTable(t, ctx, d, "tb")

	foo := Foo{I: 4, S: "f"}
	if err := tb.Put(ctx, "foo", foo); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}

	bar := Bar{F: 0.5, S: "b"}
	// NOTE: not best practice, but store bar as
	// optional (by passing the address of bar to Put).
	// This tests auto-dereferencing.
	if err := tb.Put(ctx, "bar", &bar); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}

	baz := Baz{Name: "John Doe", Active: true}
	if err := tb.Put(ctx, "baz", baz); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}

	tu.CheckExec(t, ctx, d, "select k, v.Name from tb where t = \"Baz\"",
		[]string{"k", "v.Name"},
		[][]*vdl.Value{
			[]*vdl.Value{vdl.ValueOf("baz"), vdl.ValueOf(baz.Name)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from tb",
		[]string{"k", "v"},
		[][]*vdl.Value{
			[]*vdl.Value{vdl.ValueOf("bar"), vdl.ValueOf(bar)},
			[]*vdl.Value{vdl.ValueOf("baz"), vdl.ValueOf(baz)},
			[]*vdl.Value{vdl.ValueOf("foo"), vdl.ValueOf(foo)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from tb where k like \"ba%\"",
		[]string{"k", "v"},
		[][]*vdl.Value{
			[]*vdl.Value{vdl.ValueOf("bar"), vdl.ValueOf(bar)},
			[]*vdl.Value{vdl.ValueOf("baz"), vdl.ValueOf(baz)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from tb where v.Active = true",
		[]string{"k", "v"},
		[][]*vdl.Value{
			[]*vdl.Value{vdl.ValueOf("baz"), vdl.ValueOf(baz)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from tb where t = \"Bar\"",
		[]string{"k", "v"},
		[][]*vdl.Value{
			[]*vdl.Value{vdl.ValueOf("bar"), vdl.ValueOf(bar)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from tb where v.F = 0.5",
		[]string{"k", "v"},
		[][]*vdl.Value{
			[]*vdl.Value{vdl.ValueOf("bar"), vdl.ValueOf(bar)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from tb where t = \"Baz\"",
		[]string{"k", "v"},
		[][]*vdl.Value{
			[]*vdl.Value{vdl.ValueOf("baz"), vdl.ValueOf(baz)},
		})

	tu.CheckExecError(t, ctx, d, "select k, v from foo", syncql.ErrTableCantAccess.ID)
}

// Tests that Database.Delete works as expected.
func TestDatabaseDelete(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	tu.TestDelete(t, ctx, a)
}

// Tests that Database.ListTables works as expected.
func TestListTables(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tu.TestListChildren(t, ctx, d)
}

// Tests that Database.{Set,Get}Permissions work as expected.
func TestDatabasePerms(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tu.TestPerms(t, ctx, d)
}

// Tests that Database.CreateTable works as expected.
func TestTableCreate(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tu.TestCreate(t, ctx, d)
}

// Tests that Database.DeleteTable works as expected.
func TestTableDelete(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tu.TestDelete(t, ctx, d)
}

// Tests that Table.{Set,Get,Delete}Permissions methods work as expected.
func TestTablePerms(t *testing.T) {
	ctx, clientACtx, sName, rootp, cleanup := tu.SetupOrDieCustom("clientA", "server", nil)
	defer cleanup()
	clientBCtx := tu.NewCtx(ctx, rootp, "clientB")
	a := tu.CreateApp(t, clientACtx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, clientACtx, a, "d")
	tb := tu.CreateTable(t, clientACtx, d, "tb")

	// Permission objects.
	aAndB := tu.DefaultPerms("root/clientA", "root/clientB")
	aOnly := tu.DefaultPerms("root/clientA")
	bOnly := tu.DefaultPerms("root/clientB")

	// Set initial permissions.
	if err := tb.SetPermissions(clientACtx, nosql.Prefix(""), aAndB); err != nil {
		t.Fatalf("tb.SetPermissions() failed: %v", err)
	}
	if err := tb.SetPermissions(clientACtx, nosql.Prefix("prefix"), aAndB); err != nil {
		t.Fatalf("tb.SetPermissions() failed: %v", err)
	}
	if err := tb.SetPermissions(clientBCtx, nosql.Prefix("prefix"), aAndB); err != nil {
		t.Fatalf("tb.SetPermissions() failed: %v", err)
	}
	if err := tb.SetPermissions(clientACtx, nosql.Prefix("prefix_a"), aOnly); err != nil {
		t.Fatalf("tb.SetPermissions() failed: %v", err)
	}
	if err := tb.SetPermissions(clientBCtx, nosql.Prefix("prefix_b"), bOnly); err != nil {
		t.Fatalf("tb.SetPermissions() failed: %v", err)
	}

	// Checks A has no access to 'prefix_b' and vice versa.
	if err := tb.SetPermissions(clientACtx, nosql.Prefix("prefix_b"), aOnly); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.SetPermissions() should have failed: %v", err)
	}
	if err := tb.SetPermissions(clientACtx, nosql.Prefix("prefix_b_suffix"), aOnly); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.SetPermissions() should have failed: %v", err)
	}
	if err := tb.SetPermissions(clientBCtx, nosql.Prefix("prefix_a"), bOnly); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.SetPermissions() should have failed: %v", err)
	}
	if err := tb.SetPermissions(clientBCtx, nosql.Prefix("prefix_a_suffix"), bOnly); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.SetPermissions() should have failed: %v", err)
	}

	// Check GetPermissions.
	wantPerms := []nosql.PrefixPermissions{
		nosql.PrefixPermissions{Prefix: nosql.Prefix(""), Perms: aAndB},
	}
	if got, _ := tb.GetPermissions(clientACtx, ""); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := tb.GetPermissions(clientACtx, "abc"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	wantPerms = []nosql.PrefixPermissions{
		nosql.PrefixPermissions{Prefix: nosql.Prefix("prefix"), Perms: aAndB},
		nosql.PrefixPermissions{Prefix: nosql.Prefix(""), Perms: aAndB},
	}
	if got, _ := tb.GetPermissions(clientACtx, "prefix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := tb.GetPermissions(clientACtx, "prefix_c"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	wantPerms = []nosql.PrefixPermissions{
		nosql.PrefixPermissions{Prefix: nosql.Prefix("prefix_a"), Perms: aOnly},
		nosql.PrefixPermissions{Prefix: nosql.Prefix("prefix"), Perms: aAndB},
		nosql.PrefixPermissions{Prefix: nosql.Prefix(""), Perms: aAndB},
	}
	if got, _ := tb.GetPermissions(clientACtx, "prefix_a"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := tb.GetPermissions(clientACtx, "prefix_a_suffix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	wantPerms = []nosql.PrefixPermissions{
		nosql.PrefixPermissions{Prefix: nosql.Prefix("prefix_b"), Perms: bOnly},
		nosql.PrefixPermissions{Prefix: nosql.Prefix("prefix"), Perms: aAndB},
		nosql.PrefixPermissions{Prefix: nosql.Prefix(""), Perms: aAndB},
	}
	if got, _ := tb.GetPermissions(clientACtx, "prefix_b"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := tb.GetPermissions(clientACtx, "prefix_b_suffix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}

	// Delete some prefix permissions and check again.
	// Check that A can't delete permissions of B.
	if err := tb.DeletePermissions(clientACtx, nosql.Prefix("prefix_b")); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.DeletePermissions() should have failed: %v", err)
	}
	if err := tb.DeletePermissions(clientBCtx, nosql.Prefix("prefix_a")); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.DeletePermissions() should have failed: %v", err)
	}
	// Delete 'prefix' and 'prefix_a'
	if err := tb.DeletePermissions(clientACtx, nosql.Prefix("prefix")); err != nil {
		t.Fatalf("tb.DeletePermissions() failed: %v", err)
	}
	if err := tb.DeletePermissions(clientACtx, nosql.Prefix("prefix_a")); err != nil {
		t.Fatalf("tb.DeletePermissions() failed: %v", err)
	}
	// Check DeletePermissions is idempotent.
	if err := tb.DeletePermissions(clientACtx, nosql.Prefix("prefix")); err != nil {
		t.Fatalf("tb.DeletePermissions() failed: %v", err)
	}

	// Check GetPermissions again.
	wantPerms = []nosql.PrefixPermissions{
		nosql.PrefixPermissions{Prefix: nosql.Prefix(""), Perms: aAndB},
	}
	if got, _ := tb.GetPermissions(clientACtx, ""); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := tb.GetPermissions(clientACtx, "prefix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := tb.GetPermissions(clientACtx, "prefix_a"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := tb.GetPermissions(clientACtx, "prefix_a_suffix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	wantPerms = []nosql.PrefixPermissions{
		nosql.PrefixPermissions{Prefix: nosql.Prefix("prefix_b"), Perms: bOnly},
		nosql.PrefixPermissions{Prefix: nosql.Prefix(""), Perms: aAndB},
	}
	if got, _ := tb.GetPermissions(clientACtx, "prefix_b"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := tb.GetPermissions(clientACtx, "prefix_b_suffix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}

	// Remove B from table-level permissions and check B has no access.
	if err := tb.SetPermissions(clientACtx, nosql.Prefix(""), aOnly); err != nil {
		t.Fatalf("tb.SetPermissions() failed: %v", err)
	}
	if _, err := tb.GetPermissions(clientBCtx, ""); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.GetPermissions() should have failed: %v", err)
	}
	if _, err := tb.GetPermissions(clientBCtx, "prefix_b"); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.GetPermissions() should have failed: %v", err)
	}
	if err := tb.SetPermissions(clientBCtx, nosql.Prefix(""), bOnly); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.SetPermissions() should have failed: %v", err)
	}
	if err := tb.SetPermissions(clientBCtx, nosql.Prefix("prefix_b"), bOnly); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.SetPermissions() should have failed: %v", err)
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

// Tests that Table.Scan works as expected.
func TestTableScan(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tb := tu.CreateTable(t, ctx, d, "tb")

	tu.CheckScan(t, ctx, tb, nosql.Prefix(""), []string{}, []interface{}{})

	fooWant := Foo{I: 4, S: "f"}
	if err := tb.Put(ctx, "foo", &fooWant); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}
	barWant := Bar{F: 0.5, S: "b"}
	if err := tb.Put(ctx, "bar", &barWant); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}

	// Match all keys.
	tu.CheckScan(t, ctx, tb, nosql.Prefix(""), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})
	tu.CheckScan(t, ctx, tb, nosql.Range("", ""), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})
	tu.CheckScan(t, ctx, tb, nosql.Range("", "z"), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})
	tu.CheckScan(t, ctx, tb, nosql.Range("a", ""), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})
	tu.CheckScan(t, ctx, tb, nosql.Range("a", "z"), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})

	// Match "bar" only.
	tu.CheckScan(t, ctx, tb, nosql.Prefix("b"), []string{"bar"}, []interface{}{&barWant})
	tu.CheckScan(t, ctx, tb, nosql.Prefix("bar"), []string{"bar"}, []interface{}{&barWant})
	tu.CheckScan(t, ctx, tb, nosql.Range("bar", "baz"), []string{"bar"}, []interface{}{&barWant})
	tu.CheckScan(t, ctx, tb, nosql.Range("bar", "foo"), []string{"bar"}, []interface{}{&barWant})
	tu.CheckScan(t, ctx, tb, nosql.Range("", "foo"), []string{"bar"}, []interface{}{&barWant})

	// Match "foo" only.
	tu.CheckScan(t, ctx, tb, nosql.Prefix("f"), []string{"foo"}, []interface{}{&fooWant})
	tu.CheckScan(t, ctx, tb, nosql.Prefix("foo"), []string{"foo"}, []interface{}{&fooWant})
	tu.CheckScan(t, ctx, tb, nosql.Range("foo", "fox"), []string{"foo"}, []interface{}{&fooWant})
	tu.CheckScan(t, ctx, tb, nosql.Range("foo", ""), []string{"foo"}, []interface{}{&fooWant})

	// Match nothing.
	tu.CheckScan(t, ctx, tb, nosql.Range("a", "bar"), []string{}, []interface{}{})
	tu.CheckScan(t, ctx, tb, nosql.Range("bar", "bar"), []string{}, []interface{}{})
	tu.CheckScan(t, ctx, tb, nosql.Prefix("z"), []string{}, []interface{}{})
}

// Tests that Table.Delete works as expected.
func TestTableDeleteRowRange(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tb := tu.CreateTable(t, ctx, d, "tb")

	tu.CheckScan(t, ctx, tb, nosql.Prefix(""), []string{}, []interface{}{})

	// Put foo and bar.
	fooWant := Foo{I: 4, S: "f"}
	if err := tb.Put(ctx, "foo", &fooWant); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}
	barWant := Bar{F: 0.5, S: "b"}
	if err := tb.Put(ctx, "bar", &barWant); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}
	tu.CheckScan(t, ctx, tb, nosql.Prefix(""), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})

	// Delete foo.
	if err := tb.Delete(ctx, nosql.Prefix("f")); err != nil {
		t.Fatalf("tb.Delete() failed: %v", err)
	}
	tu.CheckScan(t, ctx, tb, nosql.Prefix(""), []string{"bar"}, []interface{}{&barWant})

	// Restore foo.
	if err := tb.Put(ctx, "foo", &fooWant); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}
	tu.CheckScan(t, ctx, tb, nosql.Prefix(""), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})

	// Delete everything.
	if err := tb.Delete(ctx, nosql.Prefix("")); err != nil {
		t.Fatalf("tb.Delete() failed: %v", err)
	}
	tu.CheckScan(t, ctx, tb, nosql.Prefix(""), []string{}, []interface{}{})
}

// Tests that Table.{Get,Put,Delete} work as expected.
func TestTableRowMethods(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tb := tu.CreateTable(t, ctx, d, "tb")

	got, want := Foo{}, Foo{I: 4, S: "foo"}
	if err := tb.Get(ctx, "f", &got); verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("tb.Get() should have failed: %v", err)
	}
	if err := tb.Put(ctx, "f", &want); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}
	if err := tb.Get(ctx, "f", &got); err != nil {
		t.Fatalf("tb.Get() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Values do not match: got %v, want %v", got, want)
	}
	// Overwrite value.
	want.I = 6
	if err := tb.Put(ctx, "f", &want); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}
	if err := tb.Get(ctx, "f", &got); err != nil {
		t.Fatalf("tb.Get() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Values do not match: got %v, want %v", got, want)
	}
	if err := tb.Delete(ctx, nosql.Prefix("f")); err != nil {
		t.Fatalf("tb.Delete() failed: %v", err)
	}
	if err := tb.Get(ctx, "f", &got); verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("r.Get() should have failed: %v", err)
	}
}

// Tests that Row.{Get,Put,Delete} work as expected.
func TestRowMethods(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tb := tu.CreateTable(t, ctx, d, "tb")

	r := tb.Row("f")
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

// Test permission checking in Row.{Get,Put,Delete} and
// Table.{Scan, DeleteRowRange}.
func TestRowPermissions(t *testing.T) {
	ctx, clientACtx, sName, rootp, cleanup := tu.SetupOrDieCustom("clientA", "server", nil)
	defer cleanup()
	clientBCtx := tu.NewCtx(ctx, rootp, "clientB")
	a := tu.CreateApp(t, clientACtx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, clientACtx, a, "d")
	tb := tu.CreateTable(t, clientACtx, d, "tb")

	// Permission objects.
	aAndB := tu.DefaultPerms("root/clientA", "root/clientB")
	aOnly := tu.DefaultPerms("root/clientA")
	bOnly := tu.DefaultPerms("root/clientB")

	// Set initial permissions.
	if err := tb.SetPermissions(clientACtx, nosql.Prefix(""), aAndB); err != nil {
		t.Fatalf("tb.SetPermissions() failed: %v", err)
	}
	if err := tb.SetPermissions(clientACtx, nosql.Prefix("a"), aOnly); err != nil {
		t.Fatalf("tb.SetPermissions() failed: %v", err)
	}
	if err := tb.SetPermissions(clientBCtx, nosql.Prefix("b"), bOnly); err != nil {
		t.Fatalf("tb.SetPermissions() failed: %v", err)
	}

	// Add some key-value pairs.
	ra := tb.Row("afoo")
	rb := tb.Row("bfoo")
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
	// Test Table.Delete and Scan.
	if err := tb.Delete(clientACtx, nosql.Prefix("")); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.Delete should have failed: %v", err)
	}
	s := tb.Scan(clientACtx, nosql.Prefix(""))
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
