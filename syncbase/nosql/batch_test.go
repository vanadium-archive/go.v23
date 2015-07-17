// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO(sadovsky): Once the storage engine layer is made public, implement a
// Syncbase-based storage engine so that we can run all the storage engine tests
// against Syncbase itself.

package nosql_test

import (
	"testing"

	wire "v.io/syncbase/v23/services/syncbase/nosql"
	"v.io/syncbase/v23/syncbase"
	"v.io/syncbase/v23/syncbase/nosql"
	"v.io/syncbase/v23/syncbase/nosql/syncql"
	tu "v.io/syncbase/v23/syncbase/testutil"
	"v.io/v23/naming"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/generic"
)

// Tests various Name and FullName methods.
func TestName(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")

	b, err := d.BeginBatch(ctx, wire.BatchOptions{})
	if err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}

	if d.Name() != "d" {
		t.Errorf("Wrong name: %q", d.Name())
	}
	if d.FullName() != naming.Join(sName, "a", "d") {
		t.Errorf("Wrong full name: %q", d.FullName())
	}
	if b.Name() == d.Name() {
		t.Errorf("Names should not match: %q", b.Name())
	}
	if b.FullName() == d.FullName() {
		t.Errorf("Full names should not match: %q", b.FullName())
	}
}

// Tests basic functionality of BatchDatabase.
func TestBatchBasics(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tb := tu.CreateTable(t, ctx, d, "tb")

	tu.CheckScan(t, ctx, tb, nosql.Prefix(""), []string{}, []interface{}{})

	var b1, b2 nosql.BatchDatabase
	var b1tb, b2tb nosql.Table
	var err error

	// Test that the effects of a transaction are not visible until commit.
	b1, err = d.BeginBatch(ctx, wire.BatchOptions{})
	if err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}
	b1tb = b1.Table("tb")

	if err := b1tb.Put(ctx, "fooKey", "fooValue"); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	// Check that foo is visible inside of this transaction.
	tu.CheckScan(t, ctx, b1tb, nosql.Prefix(""), []string{"fooKey"}, []interface{}{"fooValue"})

	// Check that foo is not yet visible outside of this transaction.
	tu.CheckScan(t, ctx, tb, nosql.Prefix(""), []string{}, []interface{}{})

	// Start a scan in b1, advance the scan one row, put a new value that would
	// occur later in the scan (if it were visible) and then advance the scan to see
	// that it doesn't show (since we snapshot uncommiteed changes at the start).
	// Ditto for Exec.
	// start the scan and exec
	scanIt := b1tb.Scan(ctx, nosql.Prefix(""))
	if !scanIt.Advance() {
		t.Fatal("scanIt.Advance() returned false")
	}
	_, execIt, err := b1.Exec(ctx, "select k from tb")
	if err != nil {
		t.Fatalf("b1.Exec() failed: %v", err)
	}
	if !execIt.Advance() {
		t.Fatal("execIt.Advance() returned false")
	}
	// put "zzzKey"
	if err := b1tb.Put(ctx, "zzzKey", "zzzValue"); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	// make sure Scan's Advance doesn't return a "zzzKey"
	for scanIt.Advance() {
		if string(scanIt.Key()) == "zzzKey" {
			t.Fatal("scanIt.Advance() found zzzKey")
		}
	}
	if scanIt.Err() != nil {
		t.Fatalf("scanIt.Advance() failed: %v", err)
	}
	// make sure Exec's Advance doesn't return a "zzzKey"
	for execIt.Advance() {
		if string(execIt.Result()[0].String()) == "zzzKey" {
			t.Fatal("execIt.Advance() found zzzKey")
		}
	}
	if execIt.Err() != nil {
		t.Fatalf("execIt.Advance() failed: %v", err)
	}

	if err := b1.Commit(ctx); err != nil {
		t.Fatalf("b1.Commit() failed: %v", err)
	}

	// Check that foo is now visible.
	tu.CheckScan(t, ctx, tb, nosql.Prefix(""), []string{"fooKey", "zzzKey"}, []interface{}{"fooValue", "zzzValue"})

	// Test that concurrent transactions are isolated.
	if b1, err = d.BeginBatch(ctx, wire.BatchOptions{}); err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}
	if b2, err = d.BeginBatch(ctx, wire.BatchOptions{}); err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}
	b1tb, b2tb = b1.Table("tb"), b2.Table("tb")

	if err := b1tb.Put(ctx, "barKey", "barValue"); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	if err := b1tb.Put(ctx, "bazKey", "bazValue"); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	var got string
	if err := b2tb.Get(ctx, "barKey", &got); verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("Get() should have failed: %v", err)
	}
	if err := b2tb.Put(ctx, "rabKey", "rabValue"); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	if err := b1.Commit(ctx); err != nil {
		t.Fatalf("b1.Commit() failed: %v", err)
	}
	if err := b2.Commit(ctx); err == nil {
		t.Fatalf("b2.Commit() should have failed: %v", err)
	}

	// Check that foo, bar, baz and zzz (but not rab) are now visible.
	tu.CheckScan(t, ctx, tb, nosql.Prefix(""), []string{"barKey", "bazKey", "fooKey", "zzzKey"}, []interface{}{"barValue", "bazValue", "fooValue", "zzzValue"})
}

// Tests that BatchDatabase.Exec doesn't see changes committed outside the batch.
// 1. Create a read only batch.
// 2. query all rows in the table
// 3. commit a new row outside of the batch
// 4. confirm new row not seen when querying all rows in the table
// 5. abort the batch and create a new readonly batch
// 6. confirm new row NOW seen when querying all rows in the table
func TestBatchExecIsolation(t *testing.T) {
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

	// Begin a readonly batch.
	roBatch, err := d.BeginBatch(ctx, wire.BatchOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}

	// fetch all rows
	tu.CheckExec(t, ctx, roBatch, "select k, v from tb",
		[]string{"k", "v"},
		[][]*vdl.Value{
			[]*vdl.Value{vdl.ValueOf("bar"), vdl.ValueOf(bar)},
			[]*vdl.Value{vdl.ValueOf("baz"), vdl.ValueOf(baz)},
			[]*vdl.Value{vdl.ValueOf("foo"), vdl.ValueOf(foo)},
		})

	// Add a row outside this batch
	newRow := Baz{Name: "Alice Wonderland", Active: false}
	if err := tb.Put(ctx, "newRow", newRow); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}

	// confirm fetching all rows doesn't get the new row
	tu.CheckExec(t, ctx, roBatch, "select k, v from tb",
		[]string{"k", "v"},
		[][]*vdl.Value{
			[]*vdl.Value{vdl.ValueOf("bar"), vdl.ValueOf(bar)},
			[]*vdl.Value{vdl.ValueOf("baz"), vdl.ValueOf(baz)},
			[]*vdl.Value{vdl.ValueOf("foo"), vdl.ValueOf(foo)},
		})

	// start a new batch
	roBatch.Abort(ctx)
	roBatch, err = d.BeginBatch(ctx, wire.BatchOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}
	defer roBatch.Abort(ctx)

	// confirm fetching all rows NOW gets the new row
	tu.CheckExec(t, ctx, roBatch, "select k, v from tb",
		[]string{"k", "v"},
		[][]*vdl.Value{
			[]*vdl.Value{vdl.ValueOf("bar"), vdl.ValueOf(bar)},
			[]*vdl.Value{vdl.ValueOf("baz"), vdl.ValueOf(baz)},
			[]*vdl.Value{vdl.ValueOf("foo"), vdl.ValueOf(foo)},
			[]*vdl.Value{vdl.ValueOf("newRow"), vdl.ValueOf(newRow)},
		})

	// test error condition on batch
	tu.CheckExecError(t, ctx, roBatch, "select k, v from foo", syncql.ErrTableCantAccess.ID)
}

// Tests that BatchDatabase.Exec DOES see changes made inside the transaction but before
// Exec is called.
func TestBatchExec(t *testing.T) {
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

	// Begin a readwrite batch.
	rwBatch, err := d.BeginBatch(ctx, wire.BatchOptions{ReadOnly: false})
	if err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}

	// fetch all rows
	tu.CheckExec(t, ctx, rwBatch, "select k, v from tb",
		[]string{"k", "v"},
		[][]*vdl.Value{
			[]*vdl.Value{vdl.ValueOf("bar"), vdl.ValueOf(bar)},
			[]*vdl.Value{vdl.ValueOf("baz"), vdl.ValueOf(baz)},
			[]*vdl.Value{vdl.ValueOf("foo"), vdl.ValueOf(foo)},
		})

	rwBatchTb := rwBatch.Table("tb")

	// Add a row in this batch
	newRow := Baz{Name: "Snow White", Active: true}
	if err := rwBatchTb.Put(ctx, "newRow", newRow); err != nil {
		t.Fatalf("rwBatchTb.Put() failed: %v", err)
	}

	// confirm fetching all rows DOES get the new row
	tu.CheckExec(t, ctx, rwBatch, "select k, v from tb",
		[]string{"k", "v"},
		[][]*vdl.Value{
			[]*vdl.Value{vdl.ValueOf("bar"), vdl.ValueOf(bar)},
			[]*vdl.Value{vdl.ValueOf("baz"), vdl.ValueOf(baz)},
			[]*vdl.Value{vdl.ValueOf("foo"), vdl.ValueOf(foo)},
			[]*vdl.Value{vdl.ValueOf("newRow"), vdl.ValueOf(newRow)},
		})

	// Delete the first row (bar) and the last row (newRow).
	// Change the baz row.  Confirm these rows are no longer fetched and that
	// the change to baz is seen.
	if err := rwBatchTb.Delete(ctx, nosql.SingleRow("bar")); err != nil {
		t.Fatalf("rwBatchTb.Delete(bar) failed: %v", err)
	}
	if err := rwBatchTb.Delete(ctx, nosql.SingleRow("newRow")); err != nil {
		t.Fatalf("rwBatchTb.Delete(newRow) failed: %v", err)
	}
	baz2 := Baz{Name: "Batman", Active: false}
	if err := rwBatchTb.Put(ctx, "baz", baz2); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}
	tu.CheckExec(t, ctx, rwBatch, "select k, v from tb",
		[]string{"k", "v"},
		[][]*vdl.Value{
			[]*vdl.Value{vdl.ValueOf("baz"), vdl.ValueOf(baz2)},
			[]*vdl.Value{vdl.ValueOf("foo"), vdl.ValueOf(foo)},
		})

	// Add the 2 rows (we just deleted) back again.
	// Delete the other two rows (baz, foo).
	// Confirm we just see the three rows we added back.
	// Add a row in this batch
	bar2 := Baz{Name: "Tom Thumb", Active: true}
	if err := rwBatchTb.Put(ctx, "bar", bar2); err != nil {
		t.Fatalf("rwBatchTb.Put() failed: %v", err)
	}
	newRow2 := Baz{Name: "Snow White", Active: false}
	if err := rwBatchTb.Put(ctx, "newRow", newRow2); err != nil {
		t.Fatalf("rwBatchTb.Put() failed: %v", err)
	}
	if err := rwBatchTb.Delete(ctx, nosql.SingleRow("baz")); err != nil {
		t.Fatalf("rwBatchTb.Delete(baz) failed: %v", err)
	}
	if err := rwBatchTb.Delete(ctx, nosql.SingleRow("foo")); err != nil {
		t.Fatalf("rwBatchTb.Delete(foo) failed: %v", err)
	}
	tu.CheckExec(t, ctx, rwBatch, "select k, v from tb",
		[]string{"k", "v"},
		[][]*vdl.Value{
			[]*vdl.Value{vdl.ValueOf("bar"), vdl.ValueOf(bar2)},
			[]*vdl.Value{vdl.ValueOf("newRow"), vdl.ValueOf(newRow2)},
		})

	// commit rw batch
	rwBatch.Commit(ctx)

	// start a new (ro) batch
	roBatch, err := d.BeginBatch(ctx, wire.BatchOptions{ReadOnly: false})
	if err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}
	defer roBatch.Abort(ctx)

	// confirm fetching all rows gets the rows committed above
	// as it was never committed
	tu.CheckExec(t, ctx, roBatch, "select k, v from tb",
		[]string{"k", "v"},
		[][]*vdl.Value{
			[]*vdl.Value{vdl.ValueOf("bar"), vdl.ValueOf(bar2)},
			[]*vdl.Value{vdl.ValueOf("newRow"), vdl.ValueOf(newRow2)},
		})
}

// Tests enforcement of BatchOptions.ReadOnly.
func TestReadOnlyBatch(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tb := tu.CreateTable(t, ctx, d, "tb")

	if err := tb.Put(ctx, "fooKey", "fooValue"); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}

	b1, err := d.BeginBatch(ctx, wire.BatchOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}
	b1tb := b1.Table("tb")

	if err := b1tb.Put(ctx, "barKey", "barValue"); verror.ErrorID(err) != wire.ErrReadOnlyBatch.ID {
		t.Fatalf("Put() should have failed: %v", err)
	}
	if err := b1tb.Delete(ctx, nosql.Prefix("fooKey")); verror.ErrorID(err) != wire.ErrReadOnlyBatch.ID {
		t.Fatalf("Table.Delete() should have failed: %v", err)
	}
	if err := b1tb.Row("fooKey").Delete(ctx); verror.ErrorID(err) != wire.ErrReadOnlyBatch.ID {
		t.Fatalf("Row.Delete() should have failed: %v", err)
	}
}

// Tests that all ops fail after attempted commit or abort.
func TestOpAfterFinalize(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tb := tu.CreateTable(t, ctx, d, "tb")

	// TODO(sadovsky): Add some sort of "op after finalize" error type and check
	// for it specifically below.
	checkOpsFail := func(b nosql.BatchDatabase) {
		btb := b.Table("tb")
		var got string
		if err := btb.Get(ctx, "fooKey", &got); err == nil {
			tu.Fatal(t, "Get() should have failed")
		}
		it := btb.Scan(ctx, nosql.Prefix(""))
		it.Advance()
		if it.Err() == nil {
			tu.Fatal(t, "Scan() should have failed")
		}
		if err := btb.Put(ctx, "barKey", "barValue"); err == nil {
			tu.Fatal(t, "Put() should have failed")
		}
		if err := btb.Delete(ctx, nosql.Prefix("fooKey")); err == nil {
			tu.Fatal(t, "Table.Delete() should have failed: %v", err)
		}
		if err := btb.Row("fooKey").Delete(ctx); err == nil {
			tu.Fatal(t, "Row.Delete() should have failed: %v", err)
		}
		if err := b.Commit(ctx); err == nil {
			tu.Fatal(t, "Commit() should have failed")
		}
	}

	// Commit a transaction, check that subsequent ops fail.
	b1, err := d.BeginBatch(ctx, wire.BatchOptions{})
	if err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}
	b1tb := b1.Table("tb")

	if err := b1tb.Put(ctx, "fooKey", "fooValue"); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}
	if err := b1.Commit(ctx); err != nil {
		t.Fatalf("b1.Commit() failed: %v", err)
	}
	checkOpsFail(b1)

	// Create a transaction with a conflict, check that the commit fails, then
	// check that subsequent ops fail.
	if b1, err = d.BeginBatch(ctx, wire.BatchOptions{}); err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}
	b1tb = b1.Table("tb")

	// Conflicts with future b1tb.Get().
	if err := tb.Put(ctx, "fooKey", "v2"); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}

	var got string
	if err := b1tb.Get(ctx, "fooKey", &got); err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	want := "fooValue"
	if got != want {
		t.Fatalf("Get() returned unexpected value: got %q, want %q", got, want)
	}
	if err := b1.Commit(ctx); err == nil {
		t.Fatalf("b1.Commit() should have failed: %v", err)
	}
	checkOpsFail(b1)

	// Create a transaction and immediately abort it, then check that subsequent
	// ops fail.
	if b1, err = d.BeginBatch(ctx, wire.BatchOptions{}); err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}
	b1tb = b1.Table("tb")
	b1.Abort(ctx)
	checkOpsFail(b1)
}

// Tests that batch methods called on non-batch return ErrNotBoundToBatch and
// that non-batch methods called on batch return ErrBoundToBatch.
func TestDisallowedMethods(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	b, err := d.BeginBatch(ctx, wire.BatchOptions{})
	if err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}

	// Batch methods on non-batch.
	dc := wire.DatabaseClient(d.FullName())
	if err := dc.Commit(ctx); verror.ErrorID(err) != wire.ErrNotBoundToBatch.ID {
		t.Fatalf("dc.Commit() should have failed: %v", err)
	}
	if err := dc.Abort(ctx); verror.ErrorID(err) != wire.ErrNotBoundToBatch.ID {
		t.Fatalf("dc.Abort() should have failed: %v", err)
	}

	// Non-batch methods on batch.
	bc := wire.DatabaseClient(b.FullName())
	// For bc.Create(), we check that err is not nil instead of checking for
	// ErrBoundToBatch specifically since in practice bc.Create() will return
	// either ErrExist or "invalid name" depending on whether the database and
	// batch exist.
	if err := bc.Create(ctx, nil, nil); err == nil {
		t.Fatalf("bc.Create() should have failed: %v", err)
	}
	if err := bc.Delete(ctx); verror.ErrorID(err) != wire.ErrBoundToBatch.ID {
		t.Fatalf("bc.Delete() should have failed: %v", err)
	}
	if _, err := bc.BeginBatch(ctx, wire.BatchOptions{}); verror.ErrorID(err) != wire.ErrBoundToBatch.ID {
		t.Fatalf("bc.BeginBatch() should have failed: %v", err)
	}
	if _, _, err := bc.GetPermissions(ctx); verror.ErrorID(err) != wire.ErrBoundToBatch.ID {
		t.Fatalf("bc.GetPermissions() should have failed: %v", err)
	}
	if err := bc.SetPermissions(ctx, nil, ""); verror.ErrorID(err) != wire.ErrBoundToBatch.ID {
		t.Fatalf("bc.SetPermissions() should have failed: %v", err)
	}
	// TODO(sadovsky): Test all other SyncGroupManager methods.
	if _, err := bc.GetSyncGroupNames(ctx); verror.ErrorID(err) != wire.ErrBoundToBatch.ID {
		t.Fatalf("bc.GetSyncGroupNames() should have failed: %v", err)
	}

	// Test that Table.{Create,Delete} fail with ErrBoundToBatch.
	tc := wire.TableClient(naming.Join(b.FullName(), "tb"))
	if err := tc.Create(ctx, nil); verror.ErrorID(err) != wire.ErrBoundToBatch.ID {
		t.Fatalf("tc.Create() should have failed: %v", err)
	}
	if err := tc.Delete(ctx); verror.ErrorID(err) != wire.ErrBoundToBatch.ID {
		t.Fatalf("tc.Delete() should have failed: %v", err)
	}
}

// Tests that d.BeginBatch() fails gracefully if the database does not exist.
func TestBeginBatchWithNonexistentDatabase(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := a.NoSQLDatabase("d", nil)
	if _, err := d.BeginBatch(ctx, wire.BatchOptions{}); verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("d.BeginBatch() should have failed: %v", err)
	}
}
