// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testutil defines helpers for tests.
package testutil

import (
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

	clientCtx, err = v23.WithPrincipal(ctx, cp)
	if err != nil {
		vlog.Fatal("v23.WithPrincipal() failed: ", err)
	}
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

////////////////////////////////////////
// Internal helpers

func getPermsOrDie(t *testing.T, ctx *context.T, ac util.AccessController) access.Permissions {
	perms, _, err := ac.GetPermissions(ctx)
	if err != nil {
		Fatalf(t, "GetPermissions failed: %s", err)
	}
	return perms
}

func defaultPerms() access.Permissions {
	perms := access.Permissions{}
	for _, tag := range access.AllTypicalTags() {
		perms.Add(security.BlessingPattern("server/client"), string(tag))
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
	service, err := server.NewService(nil, nil, perms)
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
	}
}
