// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql_test

import (
	"reflect"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/query/syncql"
	wire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/nosql"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	"v.io/v23/vom"
	_ "v.io/x/ref/runtime/factories/generic"
	tu "v.io/x/ref/services/syncbase/testutil"
)

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

// Tests name-checking on database creation.
func TestDatabaseCreateNameValidation(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	tu.TestCreateNameValidation(t, ctx, a, tu.OkDbTableNames, tu.NotOkDbTableNames)
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
// Also, the query package is tested in its entirety in
// v23/syncbase/nosql/query/...
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

	tu.CheckExec(t, ctx, d, "select k, v.Name from tb where Type(v) like \"%.Baz\"",
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

	tu.CheckExec(t, ctx, d, "select k, v from tb where Type(v) like \"%.Bar\"",
		[]string{"k", "v"},
		[][]*vdl.Value{
			[]*vdl.Value{vdl.ValueOf("bar"), vdl.ValueOf(bar)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from tb where v.F = 0.5",
		[]string{"k", "v"},
		[][]*vdl.Value{
			[]*vdl.Value{vdl.ValueOf("bar"), vdl.ValueOf(bar)},
		})

	tu.CheckExec(t, ctx, d, "select k, v from tb where Type(v) like \"%.Baz\"",
		[]string{"k", "v"},
		[][]*vdl.Value{
			[]*vdl.Value{vdl.ValueOf("baz"), vdl.ValueOf(baz)},
		})

	tu.CheckExecError(t, ctx, d, "select k, v from foo", syncql.ErrTableCantAccess.ID)
}

// Tests that Database.ListTables works as expected.
func TestListTables(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "app/a#%b")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tu.TestListChildren(t, ctx, d, tu.OkDbTableNames)
}

// Tests that Database.{Set,Get}Permissions work as expected.
func TestDatabasePerms(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tu.TestPerms(t, ctx, d)
}

// Tests that Table.Create works as expected.
func TestTableCreate(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tu.TestCreate(t, ctx, d)
}

// Tests name-checking on table creation.
func TestTableCreateNameValidation(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tu.TestCreateNameValidation(t, ctx, d, tu.OkDbTableNames, tu.NotOkDbTableNames)
}

// Tests that Table.Destroy works as expected.
func TestTableDestroy(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tu.TestDestroy(t, ctx, d)
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
	if err := tb.SetPermissions(clientACtx, aAndB); err != nil {
		t.Fatalf("tb.SetPermissions() failed: %v", err)
	}
	if err := tb.SetPrefixPermissions(clientACtx, nosql.Prefix(""), aAndB); err != nil {
		t.Fatalf("tb.SetPrefixPermissions() failed: %v", err)
	}
	if err := tb.SetPrefixPermissions(clientACtx, nosql.Prefix("prefix"), aAndB); err != nil {
		t.Fatalf("tb.SetPrefixPermissions() failed: %v", err)
	}
	if err := tb.SetPrefixPermissions(clientBCtx, nosql.Prefix("prefix"), aAndB); err != nil {
		t.Fatalf("tb.SetPrefixPermissions() failed: %v", err)
	}
	if err := tb.SetPrefixPermissions(clientACtx, nosql.Prefix("prefix_a"), aOnly); err != nil {
		t.Fatalf("tb.SetPrefixPermissions() failed: %v", err)
	}
	if err := tb.SetPrefixPermissions(clientBCtx, nosql.Prefix("prefix_b"), bOnly); err != nil {
		t.Fatalf("tb.SetPrefixPermissions() failed: %v", err)
	}

	// Checks A has no access to 'prefix_b' and vice versa.
	if err := tb.SetPrefixPermissions(clientACtx, nosql.Prefix("prefix_b"), aOnly); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.SetPrefixPermissions() should have failed: %v", err)
	}
	if err := tb.SetPrefixPermissions(clientACtx, nosql.Prefix("prefix_b_suffix"), aOnly); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.SetPrefixPermissions() should have failed: %v", err)
	}
	if err := tb.SetPrefixPermissions(clientBCtx, nosql.Prefix("prefix_a"), bOnly); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.SetPrefixPermissions() should have failed: %v", err)
	}
	if err := tb.SetPrefixPermissions(clientBCtx, nosql.Prefix("prefix_a_suffix"), bOnly); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.SetPrefixPermissions() should have failed: %v", err)
	}

	// Check GetPrefixPermissions.
	wantPerms := []nosql.PrefixPermissions{
		nosql.PrefixPermissions{Prefix: nosql.Prefix(""), Perms: aAndB},
	}
	if got, _ := tb.GetPrefixPermissions(clientACtx, ""); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := tb.GetPrefixPermissions(clientACtx, "abc"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	wantPerms = []nosql.PrefixPermissions{
		nosql.PrefixPermissions{Prefix: nosql.Prefix("prefix"), Perms: aAndB},
		nosql.PrefixPermissions{Prefix: nosql.Prefix(""), Perms: aAndB},
	}
	if got, _ := tb.GetPrefixPermissions(clientACtx, "prefix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := tb.GetPrefixPermissions(clientACtx, "prefix_c"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	wantPerms = []nosql.PrefixPermissions{
		nosql.PrefixPermissions{Prefix: nosql.Prefix("prefix_a"), Perms: aOnly},
		nosql.PrefixPermissions{Prefix: nosql.Prefix("prefix"), Perms: aAndB},
		nosql.PrefixPermissions{Prefix: nosql.Prefix(""), Perms: aAndB},
	}
	if got, _ := tb.GetPrefixPermissions(clientACtx, "prefix_a"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := tb.GetPrefixPermissions(clientACtx, "prefix_a_suffix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	wantPerms = []nosql.PrefixPermissions{
		nosql.PrefixPermissions{Prefix: nosql.Prefix("prefix_b"), Perms: bOnly},
		nosql.PrefixPermissions{Prefix: nosql.Prefix("prefix"), Perms: aAndB},
		nosql.PrefixPermissions{Prefix: nosql.Prefix(""), Perms: aAndB},
	}
	if got, _ := tb.GetPrefixPermissions(clientACtx, "prefix_b"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := tb.GetPrefixPermissions(clientACtx, "prefix_b_suffix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}

	// Delete some prefix permissions and check again.
	// Check that A can't delete permissions of B.
	if err := tb.DeletePrefixPermissions(clientACtx, nosql.Prefix("prefix_b")); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.DeletePrefixPermissions() should have failed: %v", err)
	}
	if err := tb.DeletePrefixPermissions(clientBCtx, nosql.Prefix("prefix_a")); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.DeletePrefixPermissions() should have failed: %v", err)
	}
	// Delete 'prefix' and 'prefix_a'
	if err := tb.DeletePrefixPermissions(clientACtx, nosql.Prefix("prefix")); err != nil {
		t.Fatalf("tb.DeletePrefixPermissions() failed: %v", err)
	}
	if err := tb.DeletePrefixPermissions(clientACtx, nosql.Prefix("prefix_a")); err != nil {
		t.Fatalf("tb.DeletePrefixPermissions() failed: %v", err)
	}
	// Check DeletePrefixPermissions is idempotent.
	if err := tb.DeletePrefixPermissions(clientACtx, nosql.Prefix("prefix")); err != nil {
		t.Fatalf("tb.DeletePrefixPermissions() failed: %v", err)
	}

	// Check GetPrefixPermissions again.
	wantPerms = []nosql.PrefixPermissions{
		nosql.PrefixPermissions{Prefix: nosql.Prefix(""), Perms: aAndB},
	}
	if got, _ := tb.GetPrefixPermissions(clientACtx, ""); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := tb.GetPrefixPermissions(clientACtx, "prefix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := tb.GetPrefixPermissions(clientACtx, "prefix_a"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := tb.GetPrefixPermissions(clientACtx, "prefix_a_suffix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	wantPerms = []nosql.PrefixPermissions{
		nosql.PrefixPermissions{Prefix: nosql.Prefix("prefix_b"), Perms: bOnly},
		nosql.PrefixPermissions{Prefix: nosql.Prefix(""), Perms: aAndB},
	}
	if got, _ := tb.GetPrefixPermissions(clientACtx, "prefix_b"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := tb.GetPrefixPermissions(clientACtx, "prefix_b_suffix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}

	// Remove B from table-level permissions and check B has no access.
	if err := tb.SetPermissions(clientACtx, aOnly); err != nil {
		t.Fatalf("tb.SetPermissions() failed: %v", err)
	}
	if err := tb.SetPrefixPermissions(clientACtx, nosql.Prefix(""), aOnly); err != nil {
		t.Fatalf("tb.SetPrefixPermissions() failed: %v", err)
	}
	if _, err := tb.GetPrefixPermissions(clientBCtx, ""); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.GetPrefixPermissions() should have failed: %v", err)
	}
	if _, err := tb.GetPrefixPermissions(clientBCtx, "prefix_b"); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.GetPrefixPermissions() should have failed: %v", err)
	}
	if err := tb.SetPermissions(clientBCtx, bOnly); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.SetPermissions() should have failed: %v", err)
	}
	if err := tb.SetPrefixPermissions(clientBCtx, nosql.Prefix("prefix_b"), bOnly); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.SetPrefixPermissions() should have failed: %v", err)
	}
}

// Tests that Table.{Set,Get}Permissions methods work as expected.
func TestTablePermsDifferentOrder(t *testing.T) {
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
	if err := tb.SetPermissions(clientACtx, aAndB); err != nil {
		t.Fatalf("tb.SetPermissions() failed: %v", err)
	}
	if err := tb.SetPrefixPermissions(clientACtx, nosql.Prefix(""), aAndB); err != nil {
		t.Fatalf("tb.SetPrefixPermissions() failed: %v", err)
	}
	if err := tb.SetPrefixPermissions(clientACtx, nosql.Prefix("prefix"), aAndB); err != nil {
		t.Fatalf("tb.SetPrefixPermissions() failed: %v", err)
	}
	if err := tb.SetPrefixPermissions(clientBCtx, nosql.Prefix("prefix_b"), bOnly); err != nil {
		t.Fatalf("tb.SetPrefixPermissions() failed: %v", err)
	}
	if err := tb.SetPrefixPermissions(clientACtx, nosql.Prefix("prefix_a"), aOnly); err != nil {
		t.Fatalf("tb.SetPrefixPermissions() failed: %v", err)
	}

	wantPerms := []nosql.PrefixPermissions{
		nosql.PrefixPermissions{Prefix: nosql.Prefix("prefix_a"), Perms: aOnly},
		nosql.PrefixPermissions{Prefix: nosql.Prefix("prefix"), Perms: aAndB},
		nosql.PrefixPermissions{Prefix: nosql.Prefix(""), Perms: aAndB},
	}
	if got, _ := tb.GetPrefixPermissions(clientACtx, "prefix_a"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := tb.GetPrefixPermissions(clientACtx, "prefix_a_suffix"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	wantPerms = []nosql.PrefixPermissions{
		nosql.PrefixPermissions{Prefix: nosql.Prefix("prefix_b"), Perms: bOnly},
		nosql.PrefixPermissions{Prefix: nosql.Prefix("prefix"), Perms: aAndB},
		nosql.PrefixPermissions{Prefix: nosql.Prefix(""), Perms: aAndB},
	}
	if got, _ := tb.GetPrefixPermissions(clientACtx, "prefix_b"); !reflect.DeepEqual(got, wantPerms) {
		t.Fatalf("Unexpected permissions: got %v, want %v", got, wantPerms)
	}
	if got, _ := tb.GetPrefixPermissions(clientACtx, "prefix_b_suffix"); !reflect.DeepEqual(got, wantPerms) {
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

func checkGetPermissions(t *testing.T, ctx *context.T, tb nosql.Table, prefix string, max, min int) {
	perms := tu.DefaultPerms("root/client")
	for len(prefix) < max {
		prefix += "a"
	}
	var wantPerms []nosql.PrefixPermissions
	for k := max; k >= min; k-- {
		wantPerms = append(wantPerms, nosql.PrefixPermissions{Prefix: nosql.Prefix(prefix[:k]), Perms: perms})
	}
	wantPerms = append(wantPerms, nosql.PrefixPermissions{Prefix: nosql.Prefix(""), Perms: perms})
	if gotPerms, _ := tb.GetPrefixPermissions(ctx, prefix); !reflect.DeepEqual(gotPerms, wantPerms) {
		tu.Fatalf(t, "Unexpected permissions for %q: got %v, want %v", prefix, gotPerms, wantPerms)
	}
}

// Tests that Table.{Set,Get,Delete}Permissions methods work as expected
// for nested prefixes.
func TestTablePermsNested(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tb := tu.CreateTable(t, ctx, d, "tb")
	// The permission object.
	perms := tu.DefaultPerms("root/client")
	depth := 4
	// Set permissions in order b, a, bb, ba, ab, aa, ...
	for i := 1; i <= depth; i++ {
		for j := 0; j < 1<<uint32(i); j++ {
			prefix := bitmaskToPrefix(j, i)
			if err := tb.SetPrefixPermissions(ctx, nosql.Prefix(prefix), perms); err != nil {
				t.Fatalf("tb.SetPrefixPermissions() failed for %q: %v", prefix, err)
			}
			checkGetPermissions(t, ctx, tb, prefix, i, 1)
		}
	}
	// Delete permissions in the reverse order.
	for i := depth; i >= 1; i-- {
		for j := 0; j < 1<<uint32(i); j++ {
			prefix := bitmaskToPrefix(j, i)
			if err := tb.DeletePrefixPermissions(ctx, nosql.Prefix(prefix)); err != nil {
				t.Fatalf("tb.DeletePrefixPermissions() failed for %q: %v", prefix, err)
			}
			checkGetPermissions(t, ctx, tb, prefix, i-1, 1)
		}
	}
	// Do again the first two steps in the reverse order.
	for i := depth; i >= 1; i-- {
		for j := 0; j < 1<<uint32(i); j++ {
			prefix := bitmaskToPrefix(j, i)
			if err := tb.SetPrefixPermissions(ctx, nosql.Prefix(prefix), perms); err != nil {
				t.Fatalf("tb.SetPrefixPermissions() failed for %q: %v", prefix, err)
			}
			checkGetPermissions(t, ctx, tb, prefix, depth, i)
		}
	}
	for i := 1; i <= depth; i++ {
		for j := 0; j < 1<<uint32(i); j++ {
			prefix := bitmaskToPrefix(j, i)
			if err := tb.DeletePrefixPermissions(ctx, nosql.Prefix(prefix)); err != nil {
				t.Fatalf("tb.DeletePrefixPermissions() failed for %q: %v", prefix, err)
			}
			checkGetPermissions(t, ctx, tb, prefix, depth, i+1)
		}
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

// Tests that Table.DeleteRange works as expected.
func TestTableDeleteRange(t *testing.T) {
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
	if err := tb.DeleteRange(ctx, nosql.Prefix("f")); err != nil {
		t.Fatalf("tb.DeleteRange() failed: %v", err)
	}
	tu.CheckScan(t, ctx, tb, nosql.Prefix(""), []string{"bar"}, []interface{}{&barWant})

	// Restore foo.
	if err := tb.Put(ctx, "foo", &fooWant); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}
	tu.CheckScan(t, ctx, tb, nosql.Prefix(""), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})

	// Delete everything.
	if err := tb.DeleteRange(ctx, nosql.Prefix("")); err != nil {
		t.Fatalf("tb.DeleteRange() failed: %v", err)
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
	if err := tb.Delete(ctx, "f"); err != nil {
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

// Tests name-checking on row creation.
func TestRowNameValidation(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tb := tu.CreateTable(t, ctx, d, "tb")
	tu.TestCreateNameValidation(t, ctx, tb, tu.OkAppRowNames, tu.NotOkAppRowNames)
}

// Test permission checking in Row.{Get,Put,Delete} and
// Table.{Scan, DeleteRange}.
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
	if err := tb.SetPermissions(clientACtx, aAndB); err != nil {
		t.Fatalf("tb.SetPermissions() failed: %v", err)
	}
	if err := tb.SetPrefixPermissions(clientACtx, nosql.Prefix(""), aAndB); err != nil {
		t.Fatalf("tb.SetPrefixPermissions() failed: %v", err)
	}
	if err := tb.SetPrefixPermissions(clientACtx, nosql.Prefix("a"), aOnly); err != nil {
		t.Fatalf("tb.SetPrefixPermissions() failed: %v", err)
	}
	if err := tb.SetPrefixPermissions(clientBCtx, nosql.Prefix("b"), bOnly); err != nil {
		t.Fatalf("tb.SetPrefixPermissions() failed: %v", err)
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
	// Test Table.DeleteRange and Scan.
	if err := tb.DeleteRange(clientACtx, nosql.Prefix("")); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Fatalf("tb.DeleteRange should have failed: %v", err)
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

// TestWatchBasic test the basic client watch functionality: no perms,
// no batches.
func TestWatchBasic(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tb := tu.CreateTable(t, ctx, d, "tb")
	var resumeMarkers []watch.ResumeMarker

	// Generate the data and resume markers.
	// Initial state.
	resumeMarker, err := d.GetResumeMarker(ctx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	resumeMarkers = append(resumeMarkers, resumeMarker)
	// Put "abc".
	r := tb.Row("abc")
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
	r = tb.Row("a")
	if err := r.Put(ctx, "value"); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}
	if resumeMarker, err = d.GetResumeMarker(ctx); err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	resumeMarkers = append(resumeMarkers, resumeMarker)

	vomValue, _ := vom.Encode("value")
	allChanges := []nosql.WatchChange{
		nosql.WatchChange{
			Table:        "tb",
			Row:          "abc",
			ChangeType:   nosql.PutChange,
			ValueBytes:   vomValue,
			ResumeMarker: resumeMarkers[1],
		},
		nosql.WatchChange{
			Table:        "tb",
			Row:          "abc",
			ChangeType:   nosql.DeleteChange,
			ResumeMarker: resumeMarkers[2],
		},
		nosql.WatchChange{
			Table:        "tb",
			Row:          "a",
			ChangeType:   nosql.PutChange,
			ValueBytes:   vomValue,
			ResumeMarker: resumeMarkers[3],
		},
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	wstream, _ := d.Watch(ctxWithTimeout, "tb", "a", resumeMarkers[0])
	tu.CheckWatch(t, wstream, allChanges)
	wstream, _ = d.Watch(ctxWithTimeout, "tb", "a", resumeMarkers[1])
	tu.CheckWatch(t, wstream, allChanges[1:])
	wstream, _ = d.Watch(ctxWithTimeout, "tb", "a", resumeMarkers[2])
	tu.CheckWatch(t, wstream, allChanges[2:])

	wstream, _ = d.Watch(ctxWithTimeout, "tb", "abc", resumeMarkers[0])
	tu.CheckWatch(t, wstream, allChanges[:2])
	wstream, _ = d.Watch(ctxWithTimeout, "tb", "abc", resumeMarkers[1])
	tu.CheckWatch(t, wstream, allChanges[1:2])
}

// TestWatchWithBatchAndPerms test that the client watch correctly handles
// batches and prefix perms.
func TestWatchWithBatchAndPerms(t *testing.T) {
	ctx, clientACtx, sName, rootp, cleanup := tu.SetupOrDieCustom("clientA", "server", nil)
	defer cleanup()
	clientBCtx := tu.NewCtx(ctx, rootp, "clientB")
	a := tu.CreateApp(t, clientACtx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, clientACtx, a, "d")
	tb := tu.CreateTable(t, clientACtx, d, "tb")

	// Set initial permissions.
	aAndB := tu.DefaultPerms("root/clientA", "root/clientB")
	aOnly := tu.DefaultPerms("root/clientA")
	if err := tb.SetPermissions(clientACtx, aAndB); err != nil {
		t.Fatalf("tb.SetPermissions() failed: %v", err)
	}
	if err := tb.SetPrefixPermissions(clientACtx, nosql.Prefix(""), aAndB); err != nil {
		t.Fatalf("tb.SetPrefixPermissions() failed: %v", err)
	}
	if err := tb.SetPrefixPermissions(clientACtx, nosql.Prefix("a"), aOnly); err != nil {
		t.Fatalf("tb.SetPrefixPermissions() failed: %v", err)
	}
	// Get the initial resume marker.
	resumeMarker, err := d.GetResumeMarker(clientACtx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	initMarker := resumeMarker
	// Do two puts in a batch.
	if err := nosql.RunInBatch(clientACtx, d, wire.BatchOptions{}, func(b nosql.BatchDatabase) error {
		tb := b.Table("tb")
		if err := tb.Put(clientACtx, "a", "value"); err != nil {
			return err
		}
		return tb.Put(clientACtx, "b", "value")
	}); err != nil {
		t.Fatalf("RunInBatch failed: %v", err)
	}

	if resumeMarker, err = d.GetResumeMarker(clientACtx); err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	vomValue, _ := vom.Encode("value")
	allChanges := []nosql.WatchChange{
		nosql.WatchChange{
			Table:        "tb",
			Row:          "a",
			ChangeType:   nosql.PutChange,
			ValueBytes:   vomValue,
			ResumeMarker: nil,
			Continued:    true,
		},
		nosql.WatchChange{
			Table:        "tb",
			Row:          "b",
			ChangeType:   nosql.PutChange,
			ValueBytes:   vomValue,
			ResumeMarker: resumeMarker,
		},
	}

	ctxAWithTimeout, cancelA := context.WithTimeout(clientACtx, 10*time.Second)
	defer cancelA()
	ctxBWithTimeout, cancelB := context.WithTimeout(clientBCtx, 10*time.Second)
	defer cancelB()
	// ClientA should see both changes as one batch.
	wstream, _ := d.Watch(ctxAWithTimeout, "tb", "", initMarker)
	tu.CheckWatch(t, wstream, allChanges)
	// ClientB should see only one change.
	wstream, _ = d.Watch(ctxBWithTimeout, "tb", "", initMarker)
	tu.CheckWatch(t, wstream, allChanges[1:])
}

// TestBlockingWatch tests that the server side of the client watch correctly
// blocks until new updates to the database arrive.
func TestBlockingWatch(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tb := tu.CreateTable(t, ctx, d, "tb")

	resumeMarker, err := d.GetResumeMarker(ctx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	wstream, _ := d.Watch(ctxWithTimeout, "tb", "a", resumeMarker)
	vomValue, _ := vom.Encode("value")
	for i := 0; i < 10; i++ {
		// Put "abc".
		r := tb.Row("abc")
		if err := r.Put(ctx, "value"); err != nil {
			t.Fatalf("r.Put() failed: %v", err)
		}
		if resumeMarker, err = d.GetResumeMarker(ctx); err != nil {
			t.Fatalf("d.GetResumeMarker() failed: %v", err)
		}
		if !wstream.Advance() {
			t.Fatalf("wstream.Advance() reached the end: %v", wstream.Err())
		}
		want := nosql.WatchChange{
			Table:        "tb",
			Row:          "abc",
			ChangeType:   nosql.PutChange,
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
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")

	resumeMarker, err := d.GetResumeMarker(ctx)
	if err != nil {
		t.Fatalf("d.GetResumeMarker() failed: %v", err)
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	wstream, _ := d.Watch(ctxWithTimeout, "tb", "a", resumeMarker)
	if wstream.Advance() {
		t.Fatalf("wstream advanced")
	}
	// TODO(rogulenko): enable this again when the RPC system always returns
	// a correct verror ID. See: https://github.com/vanadium/issues/issues/775.
	// if got, want := verror.ErrorID(wstream.Err()), verror.ErrTimeout.ID; got != want {
	// 	t.Fatalf("unexpected wstream error ID: got %v, want %v", got, want)
	// }
}
