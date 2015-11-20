// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"fmt"

	"v.io/v23"
	"v.io/v23/syncbase"
	_ "v.io/x/ref/runtime/factories/generic"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test/modules"
	"v.io/x/ref/test/v23tests"
)

//go:generate jiri test generate

// TODO(sadovsky): All tests in this file should be updated so that the client
// carries blessing "root:client", so that access is not granted anywhere just
// because the server blessing name is a prefix of the client blessing name or
// vice versa.

func V23TestSyncbasedPutGet(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	clientCreds, _ := t.Shell().NewChildCredentials("server:client")
	serverCreds, _ := t.Shell().NewChildCredentials("server")
	cleanup := tu.StartSyncbased(t, serverCreds, syncbaseName, "",
		`{"Read": {"In":["root:server:client"]}, "Write": {"In":["root:server:client"]}}`)
	defer cleanup()

	tu.RunClient(t, clientCreds, runTestSyncbasedPutGet)
}

var runTestSyncbasedPutGet = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	// Create app, database and table.
	a := syncbase.NewService(syncbaseName).App("a")
	if err := a.Create(ctx, nil); err != nil {
		return fmt.Errorf("unable to create an app: %v", err)
	}
	d := a.NoSQLDatabase("d", nil)
	if err := d.Create(ctx, nil); err != nil {
		return fmt.Errorf("unable to create a database: %v", err)
	}
	if err := d.Table("tb").Create(ctx, nil); err != nil {
		return fmt.Errorf("unable to create a table: %v", err)
	}
	tb := d.Table("tb")

	// Do Put followed by Get on a row.
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
}, "runTestSyncbasedPutGet")
