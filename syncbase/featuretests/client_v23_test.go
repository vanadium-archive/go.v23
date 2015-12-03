// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"v.io/v23/syncbase"
	_ "v.io/x/ref/runtime/factories/generic"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test/v23tests"
)

//go:generate jiri test generate

func V23TestSyncbasedPutGet(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")

	// Start syncbased.
	serverCreds := forkCredentials(t, "server")
	cleanup := tu.StartSyncbased(t, serverCreds, testSbName, "", `{"Read": {"In":["root:server", "root:client"]}, "Write": {"In":["root:server", "root:client"]}}`)
	defer cleanup()

	// Create app, database and table.
	ctx := forkContext(t, "client")
	a := syncbase.NewService(testSbName).App("a")
	if err := a.Create(ctx, nil); err != nil {
		t.Fatalf("unable to create an app: %v", err)
	}
	d := a.NoSQLDatabase("d", nil)
	if err := d.Create(ctx, nil); err != nil {
		t.Fatalf("unable to create a database: %v", err)
	}
	tb := d.Table("tb")
	if err := tb.Create(ctx, nil); err != nil {
		t.Fatalf("unable to create a table: %v", err)
	}

	// Do a Put followed by a Get.
	r := tb.Row("r")
	if err := r.Put(ctx, "testkey"); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}
	var result string
	if err := r.Get(ctx, &result); err != nil {
		t.Fatalf("r.Get() failed: %v", err)
	}
	if got, want := result, "testkey"; got != want {
		t.Fatalf("unexpected value: got %q, want %q", got, want)
	}
}
