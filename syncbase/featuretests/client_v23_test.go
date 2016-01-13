// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"testing"

	"v.io/v23/syncbase"
	"v.io/x/ref/test/v23test"
)

func TestV23SyncbasedPutGet(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, v23test.Opts{})
	defer sh.Cleanup()
	sh.StartRootMountTable()

	// Start syncbased.
	serverCreds := sh.ForkCredentials("server")
	sh.StartSyncbase(serverCreds, testSbName, "", `{"Read": {"In":["root:server", "root:client"]}, "Write": {"In":["root:server", "root:client"]}}`)

	// Create app, database and table.
	// TODO(ivanpi): Use setupAppA.
	ctx := sh.ForkContext("client")
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
