// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase_test

import (
	"testing"

	"v.io/syncbase/v23/syncbase"
	tu "v.io/syncbase/v23/syncbase/testutil"
	_ "v.io/x/ref/runtime/factories/generic"
)

////////////////////////////////////////
// Test cases

func TestNameAndKey(t *testing.T) {
	a := syncbase.NewService("s").App("a")

	if a.Name() != "a" {
		t.Errorf("Wrong name: %q", a.Name())
	}
}

// Tests that Service.ListApps works as expected.
func TestListApps(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	s := syncbase.NewService(sName)
	tu.TestListChildren(t, ctx, s)
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

// Tests that App.Delete works as expected.
func TestAppDelete(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	tu.TestDelete(t, ctx, syncbase.NewService(sName))
}

// Tests that App.ListDatabases works as expected.
func TestListDatabases(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	tu.TestListChildren(t, ctx, a)
}

// Tests that App.{Set,Get}Permissions work as expected.
func TestAppPerms(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	tu.TestPerms(t, ctx, a)
}
