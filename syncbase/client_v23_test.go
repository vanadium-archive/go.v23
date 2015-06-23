// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase_test

import (
	"fmt"

	"v.io/syncbase/v23/syncbase"
	tu "v.io/syncbase/v23/syncbase/testutil"
	"v.io/v23"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test/modules"
	"v.io/x/ref/test/v23tests"
)

//go:generate v23 test generate

const (
	syncbaseName = "sync" // Name which syncbase mounts itself at
)

func V23TestSyncbasedPutGet(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	clientCreds, _ := t.Shell().NewChildCredentials("server/client")
	serverCreds, _ := t.Shell().NewChildCredentials("server")
	cleanup := tu.StartSyncbased(t, serverCreds, syncbaseName,
		`{"Read": {"In":["root/server/client"]}, "Write": {"In":["root/server/client"]}}`)
	defer cleanup()

	tu.RunClient(t, clientCreds, runClient)
}

var runClient = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	// Create app, database and table.
	a := syncbase.NewService(syncbaseName).App("a")
	if err := a.Create(ctx, nil); err != nil {
		return fmt.Errorf("unable to create an app: %v", err)
	}
	d := a.NoSQLDatabase("d")
	if err := d.Create(ctx, nil); err != nil {
		return fmt.Errorf("unable to create a database: %v", err)
	}
	if err := d.CreateTable(ctx, "tb", nil); err != nil {
		return fmt.Errorf("unable to create a table: %v", err)
	}
	tb := d.Table("tb")
	// Do Put, Get on a row.
	r := tb.Row("r")
	if err := r.Put(ctx, "testkey"); err != nil {
		return fmt.Errorf("r.Put() failed: %v", err)
	}
	var result string
	if err := r.Get(ctx, &result); err != nil {
		return fmt.Errorf("r.Get() failed: %v", err)
	}
	if got, want := result, "testkey"; got != want {
		return fmt.Errorf("unexpected value: got %q, want %q", got, want)
	}
	return nil
}, "runClient")
