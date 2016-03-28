// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO(sadovsky): Once the storage engine layer is made public, implement a
// Syncbase-based storage engine so that we can run all the storage engine tests
// against Syncbase itself.

package syncbase_test

import (
	"fmt"
	"reflect"
	"testing"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/query/syncql"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/v23/verror"
	"v.io/v23/vom"
	_ "v.io/x/ref/runtime/factories/generic"
	tu "v.io/x/ref/services/syncbase/testutil"
)

// Tests various Name and FullName methods.
func TestName(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")

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
	if b.Name() != d.Name() {
		t.Errorf("Names should match: %q, %q", b.Name(), d.Name())
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
	d := tu.CreateDatabase(t, ctx, a, "d")
	tb := tu.CreateTable(t, ctx, d, "tb")

	tu.CheckScan(t, ctx, tb, syncbase.Prefix(""), []string{}, []interface{}{})

	var b1, b2 syncbase.BatchDatabase
	var b1tb, b2tb syncbase.Table
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
	tu.CheckScan(t, ctx, b1tb, syncbase.Prefix(""), []string{"fooKey"}, []interface{}{"fooValue"})

	// Check that foo is not yet visible outside of this transaction.
	tu.CheckScan(t, ctx, tb, syncbase.Prefix(""), []string{}, []interface{}{})

	// Start a scan in b1, advance the scan one row, put a new value that would
	// occur later in the scan (if it were visible) and then advance the scan to see
	// that it doesn't show (since we snapshot uncommiteed changes at the start).
	// Ditto for Exec.
	// start the scan and exec
	scanIt := b1tb.Scan(ctx, syncbase.Prefix(""))
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
		var str string
		if err := execIt.Result()[0].ToValue(&str); err != nil {
			t.Fatal(err)
		}
		if str == "zzzKey" {
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
	tu.CheckScan(t, ctx, tb, syncbase.Prefix(""), []string{"fooKey", "zzzKey"}, []interface{}{"fooValue", "zzzValue"})

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
	tu.CheckScan(t, ctx, tb, syncbase.Prefix(""), []string{"barKey", "bazKey", "fooKey", "zzzKey"}, []interface{}{"barValue", "bazValue", "fooValue", "zzzValue"})
}

// Tests that BatchDatabase.ListTables does not see the effect of concurrent
// table creation.
// Note, this test fails if Database.ListTables is implemented using glob,
// because b.ListTables() does not see "tb". The glob client library issues glob
// on each point along the path to check for Resolve access. Glob("a") returns
// "a/d" but not "a/d%%batchInfo", so the glob client library does not recurse
// further.
func TestBatchListTables(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
	tu.CreateTable(t, ctx, d, "tb")
	b, err := d.BeginBatch(ctx, wire.BatchOptions{})

	got, err := d.ListTables(ctx)
	want := []string{"tb"}
	if err != nil {
		t.Fatalf("self.ListTables() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Lists do not match: got %v, want %v", got, want)
	}

	// Table creation/destruction is not allowed within a batch.
	if err := b.Table("tb_batch").Create(ctx, nil); verror.ErrorID(err) != wire.ErrBoundToBatch.ID {
		t.Fatalf("b.tb_batch.Create() should have failed: %v", err)
	}
	if err := b.Table("tb").Destroy(ctx); verror.ErrorID(err) != wire.ErrBoundToBatch.ID {
		t.Fatalf("b.tb.Destroy() should have failed: %v", err)
	}

	tu.CreateTable(t, ctx, d, "tb_nonbatch")

	// Non-batch should see tb_nonbatch; batch should only see tb.
	got, err = d.ListTables(ctx)
	want = []string{"tb", "tb_nonbatch"}
	if err != nil {
		t.Fatalf("self.ListChildren() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Lists do not match: got %v, want %v", got, want)
	}

	got, err = b.ListTables(ctx)
	want = []string{"tb"}
	if err != nil {
		t.Fatalf("self.ListChildren() failed: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Lists do not match: got %v, want %v", got, want)
	}
}

// Tests that BatchDatabase.Exec doesn't see changes committed outside the
// batch.
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
	d := tu.CreateDatabase(t, ctx, a, "d")
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
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz)},
			{vom.RawBytesOf("foo"), vom.RawBytesOf(foo)},
		})

	// Add a row outside this batch
	newRow := Baz{Name: "Alice Wonderland", Active: false}
	if err := tb.Put(ctx, "newRow", newRow); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}

	// confirm fetching all rows doesn't get the new row
	tu.CheckExec(t, ctx, roBatch, "select k, v from tb",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz)},
			{vom.RawBytesOf("foo"), vom.RawBytesOf(foo)},
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
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz)},
			{vom.RawBytesOf("foo"), vom.RawBytesOf(foo)},
			{vom.RawBytesOf("newRow"), vom.RawBytesOf(newRow)},
		})

	// test error condition on batch
	tu.CheckExecError(t, ctx, roBatch, "select k, v from foo", syncql.ErrTableCantAccess.ID)
}

// Test exec of delete statement in readonly batch (it should fail).
func TestBatchReadonlyExecDelete(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
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

	// Attempt to delete "foo" k/v pair with a syncQL delete.
	tu.CheckExecError(t, ctx, roBatch, "delete from tb where k = \"foo\"", syncql.ErrTableCantAccess.ID)

	// start a new batch
	roBatch.Abort(ctx)
}

// Tests that BatchDatabase.Exec DOES see changes made inside the transaction
// but before Exec is called.
func TestBatchExec(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
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
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz)},
			{vom.RawBytesOf("foo"), vom.RawBytesOf(foo)},
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
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar)},
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz)},
			{vom.RawBytesOf("foo"), vom.RawBytesOf(foo)},
			{vom.RawBytesOf("newRow"), vom.RawBytesOf(newRow)},
		})

	// Delete the first row (bar) and the last row (newRow).
	// Change the baz row.  Confirm these rows are no longer fetched and that
	// the change to baz is seen.
	tu.CheckExec(t, ctx, rwBatch, "delete from tb where k = \"bar\" or k = \"newRow\"",
		[]string{"Count"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf(2)},
		})
	baz2 := Baz{Name: "Batman", Active: false}
	if err := rwBatchTb.Put(ctx, "baz", baz2); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}
	tu.CheckExec(t, ctx, rwBatch, "select k, v from tb",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("baz"), vom.RawBytesOf(baz2)},
			{vom.RawBytesOf("foo"), vom.RawBytesOf(foo)},
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
	tu.CheckExec(t, ctx, rwBatch, "delete from tb where k = \"baz\" or k = \"foo\"",
		[]string{"Count"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf(2)},
		})
	tu.CheckExec(t, ctx, rwBatch, "select k, v from tb",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar2)},
			{vom.RawBytesOf("newRow"), vom.RawBytesOf(newRow2)},
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
	tu.CheckExec(t, ctx, roBatch, "select k, v from tb",
		[]string{"k", "v"},
		[][]*vom.RawBytes{
			{vom.RawBytesOf("bar"), vom.RawBytesOf(bar2)},
			{vom.RawBytesOf("newRow"), vom.RawBytesOf(newRow2)},
		})
}

// Tests enforcement of BatchOptions.ReadOnly.
func TestReadOnlyBatch(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
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
	if err := b1tb.DeleteRange(ctx, syncbase.Prefix("fooKey")); verror.ErrorID(err) != wire.ErrReadOnlyBatch.ID {
		t.Fatalf("Table.DeleteRange() should have failed: %v", err)
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
	d := tu.CreateDatabase(t, ctx, a, "d")
	tb := tu.CreateTable(t, ctx, d, "tb")

	// TODO(sadovsky): Add some sort of "op after finalize" error type and check
	// for it specifically below.
	checkOpsFail := func(b syncbase.BatchDatabase) {
		btb := b.Table("tb")
		var got string
		if err := btb.Get(ctx, "fooKey", &got); err == nil {
			tu.Fatal(t, "Get() should have failed")
		}
		it := btb.Scan(ctx, syncbase.Prefix(""))
		it.Advance()
		if it.Err() == nil {
			tu.Fatal(t, "Scan() should have failed")
		}
		if err := btb.Put(ctx, "barKey", "barValue"); err == nil {
			tu.Fatal(t, "Put() should have failed")
		}
		if err := btb.DeleteRange(ctx, syncbase.Prefix("fooKey")); err == nil {
			tu.Fatal(t, "Table.DeleteRange() should have failed: %v", err)
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
	d := tu.CreateDatabase(t, ctx, a, "d")
	b, err := d.BeginBatch(ctx, wire.BatchOptions{})
	if err != nil {
		t.Fatalf("d.BeginBatch() failed: %v", err)
	}

	// Batch methods on non-batch.
	dc := wire.DatabaseClient(d.FullName())
	if err := dc.Commit(ctx, -1); verror.ErrorID(err) != wire.ErrNotBoundToBatch.ID {
		t.Fatalf("dc.Commit() should have failed: %v", err)
	}
	if err := dc.Abort(ctx, -1); verror.ErrorID(err) != wire.ErrNotBoundToBatch.ID {
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
	if err := bc.Destroy(ctx, -1); verror.ErrorID(err) != wire.ErrBoundToBatch.ID {
		t.Fatalf("bc.Destroy() should have failed: %v", err)
	}
	if _, err := bc.BeginBatch(ctx, -1, wire.BatchOptions{}); verror.ErrorID(err) != wire.ErrBoundToBatch.ID {
		t.Fatalf("bc.BeginBatch() should have failed: %v", err)
	}
	if _, _, err := bc.GetPermissions(ctx); verror.ErrorID(err) != wire.ErrBoundToBatch.ID {
		t.Fatalf("bc.GetPermissions() should have failed: %v", err)
	}
	if err := bc.SetPermissions(ctx, nil, ""); verror.ErrorID(err) != wire.ErrBoundToBatch.ID {
		t.Fatalf("bc.SetPermissions() should have failed: %v", err)
	}
	// TODO(sadovsky): Test all other SyncgroupManager methods.
	if _, err := bc.GetSyncgroupNames(ctx); verror.ErrorID(err) != wire.ErrBoundToBatch.ID {
		t.Fatalf("bc.GetSyncgroupNames() should have failed: %v", err)
	}

	// Test that Table.{Create,Destroy} fail with ErrBoundToBatch.
	tc := wire.TableClient(naming.Join(b.FullName(), "tb"))
	if err := tc.Create(ctx, -1, nil); verror.ErrorID(err) != wire.ErrBoundToBatch.ID {
		t.Fatalf("tc.Create() should have failed: %v", err)
	}
	if err := tc.Destroy(ctx, -1); verror.ErrorID(err) != wire.ErrBoundToBatch.ID {
		t.Fatalf("tc.Destroy() should have failed: %v", err)
	}
}

// Tests that d.BeginBatch() fails gracefully if the database does not exist.
func TestBeginBatchWithNonexistentDatabase(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := a.Database("d", nil)
	if _, err := d.BeginBatch(ctx, wire.BatchOptions{}); verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("d.BeginBatch() should have failed: %v", err)
	}
}

// tryWithConcurrentWrites is a RunInBatch() test helper that causes the first
// failTimes attempts to fail with a concurrent write before succeeding.
func tryWithConcurrentWrites(t *testing.T, ctx *context.T, d syncbase.Database, failTimes int, returnErr error) error {
	var value string
	retries := 0
	return syncbase.RunInBatch(ctx, d, wire.BatchOptions{}, func(b syncbase.BatchDatabase) error {
		retries++
		// Read foo.
		if err := b.Table("tb").Get(ctx, fmt.Sprintf("foo-%d", retries), &value); verror.ErrorID(err) != verror.ErrNoExist.ID {
			t.Errorf("b.Get() should have failed with ErrNoExist, got: %v", err)
		}
		// If we need to fail, write to foo in a separate concurrent batch. This
		// is always written on every attempt.
		if retries < failTimes {
			if err := d.Table("tb").Put(ctx, fmt.Sprintf("foo-%d", retries), "foo"); err != nil {
				t.Errorf("d.Put() failed: %v", err)
			}
		}
		// Write to bar. This is only committed on a successful attempt.
		if err := b.Table("tb").Put(ctx, fmt.Sprintf("bar-%d", retries), "bar"); err != nil {
			t.Errorf("b.Put() failed: %v", err)
		}
		// Return user defined error.
		return returnErr
	})
}

// Tests that RunInBatch() properly retries on Commit failure.
func TestRunInBatchRetry(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
	tb := tu.CreateTable(t, ctx, d, "tb")

	// Succeed (no conflict) on second try.
	if err := tryWithConcurrentWrites(t, ctx, d, 2, nil); err != nil {
		t.Errorf("RunInBatch() failed: %v", err)
	}
	// First try failed, second succeeded.
	tu.CheckScan(t, ctx, tb, syncbase.Prefix(""),
		[]string{"bar-2", "foo-1"},
		[]interface{}{"bar", "foo"})
}

// Tests that RunInBatch() gives up after too many Commit failures.
func TestRunInBatchMaxRetries(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
	tb := tu.CreateTable(t, ctx, d, "tb")

	// Succeed (no conflict) on 10th try. RunInBatch will retry 3 times and give
	// up with ErrConcurrentBatch.
	if err := tryWithConcurrentWrites(t, ctx, d, 10, nil); verror.ErrorID(err) != wire.ErrConcurrentBatch.ID {
		t.Errorf("RunInBatch() should have failed with ErrConcurrentBatch, got: %v", err)
	}
	// Three failed tries.
	tu.CheckScan(t, ctx, tb, syncbase.Prefix(""),
		[]string{"foo-1", "foo-2", "foo-3"},
		[]interface{}{"foo", "foo", "foo"})
}

// Tests that RunInBatch() passes through errors without retrying.
func TestRunInBatchError(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
	tb := tu.CreateTable(t, ctx, d, "tb")

	// Return error from fn. Errors other than ErrConcurrentTransaction are not
	// retried.
	dummyError := fmt.Errorf("dummyError")
	if err := tryWithConcurrentWrites(t, ctx, d, 10, dummyError); err != dummyError {
		t.Errorf("RunInBatch() should have failed with %v, got: %v", dummyError, err)
	}
	// Single failed try.
	tu.CheckScan(t, ctx, tb, syncbase.Prefix(""),
		[]string{"foo-1"},
		[]interface{}{"foo"})
}

// Tests that RunInBatch() works with readonly batches without trying to Commit.
func TestRunInBatchReadOnly(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateDatabase(t, ctx, a, "d")
	tb := tu.CreateTable(t, ctx, d, "tb")

	// Test readonly batch.
	if err := tb.Put(ctx, "foo", 1); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}
	if err := syncbase.RunInBatch(ctx, d, wire.BatchOptions{ReadOnly: true}, func(b syncbase.BatchDatabase) error {
		var value int32
		// Read foo.
		if err := b.Table("tb").Get(ctx, "foo", &value); err != nil {
			t.Fatalf("b.Get() failed: %v", err)
		}
		newValue := value + 1
		// Write to foo in a separate concurrent batch. This is always written on
		// every iteration. It should not cause a retry since readonly batches are
		// not committed.
		if err := d.Table("tb").Put(ctx, "foo", newValue); err != nil {
			t.Errorf("d.Put() failed: %v", err)
		}
		// Read foo again. Batch should not see the incremented value.
		var rereadValue int32
		if err := b.Table("tb").Get(ctx, "foo", &rereadValue); err != nil {
			t.Fatalf("b.Get() failed: %v", err)
		}
		if value != rereadValue {
			t.Fatal("batch should not see value change outside batch")
		}
		// Try writing to bar. This should fail since the batch is readonly.
		if err := b.Table("tb").Put(ctx, "bar", value); verror.ErrorID(err) != wire.ErrReadOnlyBatch.ID {
			t.Errorf("b.Put() should have failed with ErrReadOnlyBatch, got: %v", err)
		}
		return nil
	}); err != nil {
		t.Errorf("RunInBatch() failed: %v", err)
	}
	// Single uncommitted iteration.
	tu.CheckScan(t, ctx, tb, syncbase.Prefix(""),
		[]string{"foo"},
		[]interface{}{int32(2)})
}
