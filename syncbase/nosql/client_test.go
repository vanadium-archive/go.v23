// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql_test

import (
	"reflect"
	"testing"

	"v.io/syncbase/v23/syncbase"
	"v.io/syncbase/v23/syncbase/nosql"
	tu "v.io/syncbase/v23/syncbase/testutil"
	"v.io/v23/context"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/generic"
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
		t.Errorf("Wrong name: %q", d.Name())
	}
	if tb.Name() != "tb" {
		t.Errorf("Wrong name: %q", tb.Name())
	}
	if r.Key() != "r" {
		t.Errorf("Wrong key: %q", r.Key())
	}
}

// Tests that Database.Create works as expected.
func TestDatabaseCreate(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	tu.TestCreate(t, ctx, a)
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
	// TODO(sadovsky): Implement.
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

func checkScan(t *testing.T, ctx *context.T, tb nosql.Table, r nosql.RowRange, wantKeys []string, wantValues []interface{}) {
	if len(wantKeys) != len(wantValues) {
		panic("bad input args")
	}
	it, err := tb.Scan(ctx, r)
	if err == nil {
		i := 0
		for it.Advance() {
			gotKey, wantKey := it.Key(), wantKeys[i]
			wantValue := wantValues[i]
			gotValue := reflect.Zero(reflect.TypeOf(wantValue)).Interface()
			if gotKey != wantKey {
				tu.Fatalf(t, "Keys do not match: got %q, want %q", gotKey, wantKey)
			}
			if err := it.Value(&gotValue); err != nil {
				tu.Fatalf(t, "it.Value() failed: %v", err)
			}
			if !reflect.DeepEqual(gotValue, wantValue) {
				tu.Fatalf(t, "Values do not match: got %v, want %v", gotValue, wantValue)
			}
			i++
		}
		err = it.Err()
		if i < len(wantKeys) {
			tu.Fatalf(t, "Unmatched keys: %v", wantKeys[i:])
		}
	}
	if err != nil {
		tu.Fatalf(t, "tb.Scan() failed: %v", err)
	}
}

// Tests that Table.Scan works as expected.
func TestTableScan(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	d := tu.CreateNoSQLDatabase(t, ctx, a, "d")
	tb := tu.CreateTable(t, ctx, d, "tb")

	checkScan(t, ctx, tb, nosql.Prefix(""), []string{}, []interface{}{})

	fooWant := Foo{I: 4, S: "f"}
	if err := tb.Put(ctx, "foo", &fooWant); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}
	barWant := Bar{F: 0.5, S: "b"}
	if err := tb.Put(ctx, "bar", &barWant); err != nil {
		t.Fatalf("tb.Put() failed: %v", err)
	}

	// Match all keys.
	checkScan(t, ctx, tb, nosql.Prefix(""), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})
	checkScan(t, ctx, tb, nosql.Range("", ""), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})
	checkScan(t, ctx, tb, nosql.Range("", "z"), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})
	checkScan(t, ctx, tb, nosql.Range("a", ""), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})
	checkScan(t, ctx, tb, nosql.Range("a", "z"), []string{"bar", "foo"}, []interface{}{&barWant, &fooWant})

	// Match "bar" only.
	checkScan(t, ctx, tb, nosql.Prefix("b"), []string{"bar"}, []interface{}{&barWant})
	checkScan(t, ctx, tb, nosql.Prefix("bar"), []string{"bar"}, []interface{}{&barWant})
	checkScan(t, ctx, tb, nosql.Range("bar", "baz"), []string{"bar"}, []interface{}{&barWant})
	checkScan(t, ctx, tb, nosql.Range("bar", "foo"), []string{"bar"}, []interface{}{&barWant})
	checkScan(t, ctx, tb, nosql.Range("", "foo"), []string{"bar"}, []interface{}{&barWant})

	// Match "foo" only.
	checkScan(t, ctx, tb, nosql.Prefix("f"), []string{"foo"}, []interface{}{&fooWant})
	checkScan(t, ctx, tb, nosql.Prefix("foo"), []string{"foo"}, []interface{}{&fooWant})
	checkScan(t, ctx, tb, nosql.Range("foo", "fox"), []string{"foo"}, []interface{}{&fooWant})
	checkScan(t, ctx, tb, nosql.Range("foo", ""), []string{"foo"}, []interface{}{&fooWant})

	// Match nothing.
	checkScan(t, ctx, tb, nosql.Range("a", "bar"), []string{}, []interface{}{})
	checkScan(t, ctx, tb, nosql.Range("bar", "bar"), []string{}, []interface{}{})
	checkScan(t, ctx, tb, nosql.Prefix("z"), []string{}, []interface{}{})
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
	// TODO(sadovsky): Enable once table.DeleteRowRange is implemented.
	/*
		if err := tb.Delete(ctx, nosql.Prefix("f")); err != nil {
			t.Fatalf("tb.Delete() failed: %v", err)
		}
		if err := tb.Get(ctx, "f", &got); verror.ErrorID(err) != verror.ErrNoExist.ID {
			t.Fatalf("r.Get() should have failed: %v", err)
		}
	*/
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
