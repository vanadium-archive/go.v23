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
	tu "v.io/syncbase/v23/syncbase/testutil"
	"v.io/v23/naming"
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

	// Check that foo is not yet visible.
	tu.CheckScan(t, ctx, tb, nosql.Prefix(""), []string{}, []interface{}{})

	if err := b1.Commit(ctx); err != nil {
		t.Fatalf("b1.Commit() failed: %v", err)
	}

	// Check that foo is now visible.
	tu.CheckScan(t, ctx, tb, nosql.Prefix(""), []string{"fooKey"}, []interface{}{"fooValue"})

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

	// Check that foo, bar, and baz (but not rab) are now visible.
	tu.CheckScan(t, ctx, tb, nosql.Prefix(""), []string{"barKey", "bazKey", "fooKey"}, []interface{}{"barValue", "bazValue", "fooValue"})
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
	if err := bc.Create(ctx, nil); verror.ErrorID(err) != wire.ErrBoundToBatch.ID {
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
}
