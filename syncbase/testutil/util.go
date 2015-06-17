// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testutil defines helpers for tests.
package testutil

import (
	"io/ioutil"
	"os"
	"reflect"
	"runtime/debug"
	"testing"

	"v.io/syncbase/v23/syncbase"
	"v.io/syncbase/v23/syncbase/nosql"
	"v.io/syncbase/v23/syncbase/util"
	"v.io/syncbase/x/ref/services/syncbase/server"
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
	tsecurity "v.io/x/ref/test/testutil"
)

func Fatal(t *testing.T, args ...interface{}) {
	debug.PrintStack()
	t.Fatal(args...)
}

func Fatalf(t *testing.T, format string, args ...interface{}) {
	debug.PrintStack()
	t.Fatalf(format, args...)
}

func CreateApp(t *testing.T, ctx *context.T, s syncbase.Service, name string) syncbase.App {
	a := s.App(name)
	if err := a.Create(ctx, nil); err != nil {
		Fatalf(t, "a.Create() failed: %v", err)
	}
	return a
}

func CreateNoSQLDatabase(t *testing.T, ctx *context.T, a syncbase.App, name string) nosql.Database {
	d := a.NoSQLDatabase(name)
	if err := d.Create(ctx, nil); err != nil {
		Fatalf(t, "d.Create() failed: %v", err)
	}
	return d
}

func CreateTable(t *testing.T, ctx *context.T, d nosql.Database, name string) nosql.Table {
	if err := d.CreateTable(ctx, name, nil); err != nil {
		Fatalf(t, "d.CreateTable() failed: %v", err)
	}
	return d.Table(name)
}

func SetupOrDie(perms access.Permissions) (clientCtx *context.T, serverName string, cleanup func()) {
	ctx, sName, cleanup, _, _ := SetupOrDieCustom("client", "server", perms)
	return ctx, sName, cleanup
}

func SetupOrDieCustom(client, server string, perms access.Permissions) (clientCtx *context.T, serverName string, cleanup func(), sp security.Principal, ctx *context.T) {
	ctx, shutdown := v23.Init()
	sp = tsecurity.NewPrincipal(server)

	clientCtx = NewClient(client, server, ctx, sp)

	serverCtx, err := v23.WithPrincipal(ctx, sp)
	if err != nil {
		vlog.Fatal("v23.WithPrincipal() failed: ", err)
	}

	serverName, stopServer := newServer(serverCtx, perms)
	cleanup = func() {
		stopServer()
		shutdown()
	}
	return
}

func defaultPerms() access.Permissions {
	perms := access.Permissions{}
	for _, tag := range access.AllTypicalTags() {
		perms.Add(security.BlessingPattern("server/client"), string(tag))
	}
	return perms
}

func CheckScan(t *testing.T, ctx *context.T, tb nosql.Table, r nosql.RowRange, wantKeys []string, wantValues []interface{}) {
	if len(wantKeys) != len(wantValues) {
		panic("bad input args")
	}
	it := tb.Scan(ctx, r)
	gotKeys := []string{}
	for it.Advance() {
		gotKey := it.Key()
		gotKeys = append(gotKeys, gotKey)
		i := len(gotKeys) - 1
		if i >= len(wantKeys) {
			continue
		}
		// Check key.
		wantKey := wantKeys[i]
		if gotKey != wantKey {
			Fatalf(t, "Keys do not match: got %q, want %q", gotKey, wantKey)
		}
		// Check value.
		wantValue := wantValues[i]
		gotValue := reflect.Zero(reflect.TypeOf(wantValue)).Interface()
		if err := it.Value(&gotValue); err != nil {
			Fatalf(t, "it.Value() failed: %v", err)
		}
		if !reflect.DeepEqual(gotValue, wantValue) {
			Fatalf(t, "Values do not match: got %v, want %v", gotValue, wantValue)
		}
	}
	if err := it.Err(); err != nil {
		Fatalf(t, "tb.Scan() failed: %v", err)
	}
	if len(gotKeys) != len(wantKeys) {
		Fatalf(t, "Unmatched keys: got %v, want %v", gotKeys, wantKeys)
	}
}

func CheckExec(t *testing.T, ctx *context.T, db nosql.DatabaseHandle, q string, wantHeaders []string, wantResults [][]*vdl.Value) {
	gotHeaders, it, err := db.Exec(ctx, q)
	if err != nil {
		t.Errorf("query %q: got %v, want nil", q, err)
	}
	if !reflect.DeepEqual(gotHeaders, wantHeaders) {
		t.Errorf("query %q: got %v, want %v", q, gotHeaders, wantHeaders)
	}
	gotResults := [][]*vdl.Value{}
	for it.Advance() {
		gotResult := it.Result()
		gotResults = append(gotResults, gotResult)
	}
	if it.Err() != nil {
		t.Errorf("query %q: got %v, want nil", q, it.Err())
	}
	if !reflect.DeepEqual(gotResults, wantResults) {
		t.Errorf("query %q: got %v, want %v", q, gotResults, wantResults)
	}
}

func CheckExecError(t *testing.T, ctx *context.T, db nosql.DatabaseHandle, q string, wantErrorID verror.ID) {
	_, rs, err := db.Exec(ctx, q)
	if err == nil {
		if rs.Advance() {
			t.Errorf("query %q: got true, want false", q)
		}
		err = rs.Err()
	}
	if verror.ErrorID(err) != wantErrorID {
		t.Errorf("%q", verror.DebugString(err))
		t.Errorf("query %q: got %v, want: %v", q, verror.ErrorID(err), wantErrorID)
	}
}

////////////////////////////////////////
// Internal helpers

func getPermsOrDie(t *testing.T, ctx *context.T, ac util.AccessController) access.Permissions {
	perms, _, err := ac.GetPermissions(ctx)
	if err != nil {
		Fatalf(t, "GetPermissions failed: %v", err)
	}
	return perms
}

func newServer(ctx *context.T, perms access.Permissions) (string, func()) {
	s, err := v23.NewServer(ctx)
	if err != nil {
		vlog.Fatal("v23.NewServer() failed: ", err)
	}
	eps, err := s.Listen(rpc.ListenSpec{Addrs: rpc.ListenAddrs{{"tcp", "127.0.0.1:0"}}})
	if err != nil {
		vlog.Fatal("s.Listen() failed: ", err)
	}

	if perms == nil {
		perms = defaultPerms()
	}
	rootDir, err := ioutil.TempDir("", "syncbase")
	if err != nil {
		vlog.Fatal("ioutil.TempDir() failed: ", err)
	}
	service, err := server.NewService(nil, nil, server.ServiceOptions{
		Perms:   perms,
		RootDir: rootDir,
		Engine:  "leveldb",
	})
	if err != nil {
		vlog.Fatal("server.NewService() failed: ", err)
	}
	d := server.NewDispatcher(service)

	if err := s.ServeDispatcher("", d); err != nil {
		vlog.Fatal("s.ServeDispatcher() failed: ", err)
	}

	name := naming.JoinAddressName(eps[0].String(), "")
	return name, func() {
		s.Stop()
		os.RemoveAll(rootDir)
	}
}

func NewClient(client, server string, ctx *context.T, sp security.Principal) (clientCtx *context.T) {
	cp := tsecurity.NewPrincipal(client)

	// Have the server principal bless the client principal as "client".
	blessings, err := sp.Bless(cp.PublicKey(), sp.BlessingStore().Default(), client, security.UnconstrainedUse())
	if err != nil {
		vlog.Fatal("sp.Bless() failed: ", err)
	}

	// Have the client present its "client" blessing when talking to the server.
	if _, err := cp.BlessingStore().Set(blessings, security.BlessingPattern(server)); err != nil {
		vlog.Fatal("cp.BlessingStore().Set() failed: ", err)
	}

	// Have the client treat the server's public key as an authority on all
	// blessings that match the pattern "server".
	if err := cp.AddToRoots(blessings); err != nil {
		vlog.Fatal("cp.AddToRoots() failed: ", err)
	}

	clientCtx, err = v23.WithPrincipal(ctx, cp)
	if err != nil {
		vlog.Fatal("v23.WithPrincipal() failed: ", err)
	}

	return clientCtx
}
