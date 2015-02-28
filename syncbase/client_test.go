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
	"v.io/v23/services/security/access"
	"v.io/v23/vdl"
	"v.io/x/lib/vlog"

	"v.io/syncbase/veyron/services/syncbase/server"
	"v.io/syncbase/veyron/services/syncbase/store/memstore"
	tsecurity "v.io/x/ref/lib/testutil/security"
	_ "v.io/x/ref/profiles"
)

func defaultACL() access.TaggedACLMap {
	acl := access.TaggedACLMap{}
	for _, tag := range access.AllTypicalTags() {
		acl.Add(security.BlessingPattern("server/client"), string(tag))
	}
	return acl
}

func newServer(ctx *context.T, acl access.TaggedACLMap) (string, func()) {
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
		acl = defaultACL()
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

func setupOrDie(acl access.TaggedACLMap) (clientCtx *context.T, serverName string, cleanup func()) {
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

func getACLOrDie(ac syncbase.AccessController, ctx *context.T, t *testing.T) access.TaggedACLMap {
	acl, _, err := ac.GetACL(ctx)
	if err != nil {
		debug.PrintStack()
		t.Fatalf("GetACL failed: %s", err)
	}
	return acl
}

// NOTE(sadovsky): Ordinarily we'd use names s,u,d,t,i for the various layers,
// but since t conflicts with "t *testing.T", we instead use sv,uv,db,tb,it.

// Inputs uvName and dbName are universe and database names respectively.
func createDatabase(ctx *context.T, s syncbase.Service, uvName, dbName string, acl access.TaggedACLMap, schema wire.Schema) error {
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
// ACLs.
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
	// TODO(sadovsky): Test auth check (against Service ACL).
	if err := uv.Create(ctx, nil); err != nil {
		t.Fatalf("uv.Create() failed: %s", err)
	}
	if wantACL, gotACL := defaultACL(), getACLOrDie(uv, ctx, t); !reflect.DeepEqual(wantACL, gotACL) {
		t.Errorf("ACLs do not match: want %v, got %v", wantACL, gotACL)
	}

	// Database GetSchema should fail, since Database does not exist.
	if _, _, err := db.GetSchema(ctx); err == nil {
		t.Fatalf("db.GetSchema() should have failed")
	}

	// Create the Database.
	// TODO(sadovsky): Test auth check (against Universe ACL).
	if err := db.Create(ctx, nil); err != nil {
		t.Fatalf("db.Create() failed: %s", err)
	}
	if wantACL, gotACL := defaultACL(), getACLOrDie(uv, ctx, t); !reflect.DeepEqual(wantACL, gotACL) {
		t.Errorf("ACLs do not match: want %v, got %v", wantACL, gotACL)
	}

	// Database GetSchema should succeed this time, and should return the default
	// (empty) schema.
	if schema, _, err := db.GetSchema(ctx); err != nil {
		t.Fatalf("db.GetSchema() failed: %s", err)
	} else if len(schema) > 0 {
		t.Fatalf("Schema is not empty: %v", schema)
	}

	// Test Universe.Create with non-default ACL.
	uv2 := sv.BindUniverse("uv2")
	acl := access.TaggedACLMap{}
	acl.Add(security.BlessingPattern("server/client"), string(access.Admin))
	if err := uv2.Create(ctx, acl); err != nil {
		t.Fatalf("uv2.Create() failed: %s", err)
	}
	if wantACL, gotACL := acl, getACLOrDie(uv2, ctx, t); !reflect.DeepEqual(wantACL, gotACL) {
		t.Errorf("ACLs do not match: want %v, got %v", wantACL, gotACL)
	}

	// Test Database.Create with non-default ACL.
	db2 := uv.BindDatabase("db2")
	acl = access.TaggedACLMap{}
	acl.Add(security.BlessingPattern("server/client"), string(access.Admin))
	if err := db2.Create(ctx, acl); err != nil {
		t.Fatalf("db2.Create() failed: %s", err)
	}
	if wantACL, gotACL := acl, getACLOrDie(db2, ctx, t); !reflect.DeepEqual(wantACL, gotACL) {
		t.Errorf("ACLs do not match: want %v, got %v", wantACL, gotACL)
	}
}

// Tests that Universe.Delete and Database.Delete work as expected.
func TestDelete(t *testing.T) {
}

// Tests that the various {Set,Get}ACL methods work as expected.
func TestACLMethods(t *testing.T) {
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
