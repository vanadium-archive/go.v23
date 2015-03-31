// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase_test

// Note: core/veyron/services/security/groups/server/server_test.go has some
// helpful code snippets to model after.

import (
	"reflect"
	"runtime/debug"
	"testing"

	wire "v.io/syncbase/v23/services/syncbase"
	"v.io/syncbase/v23/syncbase"
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/vdl"
	"v.io/x/lib/vlog"

	"v.io/syncbase/veyron/services/syncbase/server"
	"v.io/syncbase/veyron/services/syncbase/store/memstore"
	_ "v.io/x/ref/profiles"
	tsecurity "v.io/x/ref/test/security"
)

func defaultPermissions() access.Permissions {
	acl := access.Permissions{}
	for _, tag := range access.AllTypicalTags() {
		acl.Add(security.BlessingPattern("server/client"), string(tag))
	}
	return acl
}

func newServer(ctx *context.T, acl access.Permissions) (string, func()) {
	s, err := v23.NewServer(ctx)
	if err != nil {
		vlog.Fatal("v23.NewServer() failed: ", err)
	}
	eps, err := s.Listen(v23.GetListenSpec(ctx))
	if err != nil {
		vlog.Fatal("s.Listen() failed: ", err)
	}

	service := server.NewService(memstore.New())
	if acl == nil {
		acl = defaultPermissions()
	}
	if err := service.Create(acl); err != nil {
		vlog.Fatal("service.Create() failed: ", err)
	}
	d := server.NewDispatcher(service)

	if err := s.ServeDispatcher("", d); err != nil {
		vlog.Fatal("s.ServeDispatcher() failed: ", err)
	}

	name := naming.JoinAddressName(eps[0].String(), "")
	return name, func() {
		s.Stop()
	}
}

func setupOrDie(acl access.Permissions) (clientCtx *context.T, serverName string, cleanup func()) {
	ctx, shutdown := v23.Init()
	cp, sp := tsecurity.NewPrincipal("client"), tsecurity.NewPrincipal("server")

	// Have the server principal bless the client principal as "client".
	blessings, err := sp.Bless(cp.PublicKey(), sp.BlessingStore().Default(), "client", security.UnconstrainedUse())
	if err != nil {
		vlog.Fatal("sp.Bless() failed: ", err)
	}
	// Have the client present its "client" blessing when talking to the server.
	if _, err := cp.BlessingStore().Set(blessings, "server"); err != nil {
		vlog.Fatal("cp.BlessingStore().Set() failed: ", err)
	}
	// Have the client treat the server's public key as an authority on all
	// blessings that match the pattern "server".
	if err := cp.AddToRoots(blessings); err != nil {
		vlog.Fatal("cp.AddToRoots() failed: ", err)
	}

	clientCtx, err = v23.SetPrincipal(ctx, cp)
	if err != nil {
		vlog.Fatal("v23.SetPrincipal() failed: ", err)
	}
	serverCtx, err := v23.SetPrincipal(ctx, sp)
	if err != nil {
		vlog.Fatal("v23.SetPrincipal() failed: ", err)
	}

	serverName, stopServer := newServer(serverCtx, acl)
	cleanup = func() {
		stopServer()
		shutdown()
	}
	return
}

func getPermissionsOrDie(ac syncbase.AccessController, ctx *context.T, t *testing.T) access.Permissions {
	acl, _, err := ac.GetPermissions(ctx)
	if err != nil {
		debug.PrintStack()
		t.Fatalf("GetPermissions failed: %s", err)
	}
	return acl
}

// NOTE(sadovsky): Ordinarily we'd use names s,u,d,t,i for the various layers,
// but since t conflicts with "t *testing.T", we instead use sv,uv,db,tb,it.

// Inputs uvName and dbName are universe and database names respectively.
func createDatabase(ctx *context.T, s syncbase.Service, uvName, dbName string, acl access.Permissions, schema wire.Schema) error {
	uv := s.BindUniverse(uvName)
	if err := uv.Create(ctx, acl); err != nil {
		return err
	}
	db := uv.BindDatabase(dbName)
	if err := db.Create(ctx, acl); err != nil {
		return err
	}
	return db.UpdateSchema(ctx, schema, "") // empty etag, i.e. blind write
}

////////////////////////////////////////
// Test cases

// TODO(sadovsky): Finish writing tests.

func TestNameAndKey(t *testing.T) {
	uv := syncbase.BindService("sv").BindUniverse("uv")
	db := uv.BindDatabase("db")
	tb := db.BindTable("tb")
	it := tb.BindItem(vdl.StringValue("it"))

	if uv.Name() != "uv" {
		t.Errorf("Wrong name: %s", uv.Name())
	}
	if db.Name() != "db" {
		t.Errorf("Wrong name: %s", db.Name())
	}
	if tb.Name() != "tb" {
		t.Errorf("Wrong name: %s", tb.Name())
	}
	if !vdl.EqualValue(it.Key(), vdl.StringValue("it")) {
		t.Errorf("Wrong key: %s", it.Key())
	}
}

// Tests that {Universe,Database}.Create work as expected, including default
// Permissions.
func TestCreate(t *testing.T) {
	ctx, svName, cleanup := setupOrDie(nil)
	defer cleanup()

	sv := syncbase.BindService(svName)
	uv := sv.BindUniverse("uv")
	db := uv.BindDatabase("db")

	// Database Create should fail, since Universe does not exist.
	if err := db.Create(ctx, nil); err == nil {
		t.Fatalf("db.Create() should have failed")
	}

	// Create the Universe.
	// TODO(sadovsky): Test auth check (against Service Permissions).
	if err := uv.Create(ctx, nil); err != nil {
		t.Fatalf("uv.Create() failed: %s", err)
	}
	if wantPermissions, gotPermissions := defaultPermissions(), getPermissionsOrDie(uv, ctx, t); !reflect.DeepEqual(wantPermissions, gotPermissions) {
		t.Errorf("Permissionss do not match: want %v, got %v", wantPermissions, gotPermissions)
	}

	// Database GetSchema should fail, since Database does not exist.
	if _, _, err := db.GetSchema(ctx); err == nil {
		t.Fatalf("db.GetSchema() should have failed")
	}

	// Create the Database.
	// TODO(sadovsky): Test auth check (against Universe Permissions).
	if err := db.Create(ctx, nil); err != nil {
		t.Fatalf("db.Create() failed: %s", err)
	}
	if wantPermissions, gotPermissions := defaultPermissions(), getPermissionsOrDie(uv, ctx, t); !reflect.DeepEqual(wantPermissions, gotPermissions) {
		t.Errorf("Permissionss do not match: want %v, got %v", wantPermissions, gotPermissions)
	}

	// Database GetSchema should succeed this time, and should return the default
	// (empty) schema.
	if schema, _, err := db.GetSchema(ctx); err != nil {
		t.Fatalf("db.GetSchema() failed: %s", err)
	} else if len(schema) > 0 {
		t.Fatalf("Schema is not empty: %v", schema)
	}

	// Test Universe.Create with non-default Permissions.
	uv2 := sv.BindUniverse("uv2")
	acl := access.Permissions{}
	acl.Add(security.BlessingPattern("server/client"), string(access.Admin))
	if err := uv2.Create(ctx, acl); err != nil {
		t.Fatalf("uv2.Create() failed: %s", err)
	}
	if wantPermissions, gotPermissions := acl, getPermissionsOrDie(uv2, ctx, t); !reflect.DeepEqual(wantPermissions, gotPermissions) {
		t.Errorf("Permissionss do not match: want %v, got %v", wantPermissions, gotPermissions)
	}

	// Test Database.Create with non-default Permissions.
	db2 := uv.BindDatabase("db2")
	acl = access.Permissions{}
	acl.Add(security.BlessingPattern("server/client"), string(access.Admin))
	if err := db2.Create(ctx, acl); err != nil {
		t.Fatalf("db2.Create() failed: %s", err)
	}
	if wantPermissions, gotPermissions := acl, getPermissionsOrDie(db2, ctx, t); !reflect.DeepEqual(wantPermissions, gotPermissions) {
		t.Errorf("Permissionss do not match: want %v, got %v", wantPermissions, gotPermissions)
	}
}

// Tests that Universe.Delete and Database.Delete work as expected.
func TestDelete(t *testing.T) {
}

// Tests that the various {Set,Get}Permissions methods work as expected.
func TestPermissionsMethods(t *testing.T) {
}

// Tests that the various {Update,Get}Schema methods work as expected.
func TestSchemaMethods(t *testing.T) {
}

// Tests that Table.{Get,Put,Delete} work as expected.
func TestTableItemMethods(t *testing.T) {
}

// Tests that Item.{Get,Put,Delete} work as expected.
func TestItemMethods(t *testing.T) {
}
