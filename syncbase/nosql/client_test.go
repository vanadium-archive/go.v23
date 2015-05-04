// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql_test

import (
	"testing"

	"v.io/syncbase/v23/syncbase"
	tu "v.io/syncbase/v23/syncbase/testutil"
	_ "v.io/x/ref/profiles"
)

////////////////////////////////////////
// Test cases

// TODO(sadovsky): Finish writing tests.

func TestNameAndKey(t *testing.T) {
	a := syncbase.NewService("s").App("a")
	d := a.NoSQLDatabase("d")
	tb := d.Table("tb")
	r := tb.Row("r")

	if d.Name() != "d" {
		t.Errorf("Wrong name: %s", d.Name())
	}
	if tb.Name() != "tb" {
		t.Errorf("Wrong name: %s", tb.Name())
	}
	if r.Key() != "r" {
		t.Errorf("Wrong key: %s", r.Key())
	}
}

// Tests that Database.Create works as expected.
func TestDatabaseCreate(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := syncbase.NewService(sName).App("a")
	if err := a.Create(ctx, nil); err != nil {
		t.Fatalf("a.Create() failed: %s", err)
	}
	tu.TestCreate(t, ctx, a)
}

// Tests that Database.Delete works as expected.
func TestDatabaseDelete(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := syncbase.NewService(sName).App("a")
	if err := a.Create(ctx, nil); err != nil {
		t.Fatalf("a.Create() failed: %s", err)
	}
	tu.TestDelete(t, ctx, a)
}

// Tests that Database.{Set,Get}Permissions work as expected.
func TestDatabasePerms(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := syncbase.NewService(sName).App("a")
	if err := a.Create(ctx, nil); err != nil {
		t.Fatalf("a.Create() failed: %s", err)
	}
	d := a.NoSQLDatabase("d")
	if err := d.Create(ctx, nil); err != nil {
		t.Fatalf("d.Create() failed: %s", err)
	}
	tu.TestPerms(t, ctx, d)
}

// Tests that Database.CreateTable works as expected.
func TestTableCreate(t *testing.T) {
	// TODO(sadovsky): Implement.
}

// Tests that Database.DeleteTable works as expected.
func TestTableDelete(t *testing.T) {
	// TODO(sadovsky): Implement.
}

// Tests that Table.{Set,Get,Delete}Permissions methods work as expected.
func TestTablePerms(t *testing.T) {
	// TODO(sadovsky): Implement.
}

// Tests that Table.{Get,Put,Delete} work as expected.
func TestTableRowMethods(t *testing.T) {
	// TODO(sadovsky): Implement.
}

// Tests that Row.{Get,Put,Delete} work as expected.
func TestRowMethods(t *testing.T) {
	// TODO(sadovsky): Implement.
}
