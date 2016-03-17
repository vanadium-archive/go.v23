// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/nosql"
	"v.io/v23/vom"
	"v.io/x/ref/services/syncbase/syncbaselib"
	"v.io/x/ref/test/v23test"
)

func TestV23SyncbasedPutGet(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	// Start syncbased.
	serverCreds := sh.ForkCredentials("server")
	// TODO(aghassemi): Resolve permission is currently needed for Watch.
	// See https://github.com/vanadium/issues/issues/1110
	sh.StartSyncbase(serverCreds, syncbaselib.Opts{Name: testSbName}, `{"Resolve": {"In":["root:server", "root:client"]}, "Read": {"In":["root:server", "root:client"]}, "Write": {"In":["root:server", "root:client"]}}`)

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
	marker, err := d.GetResumeMarker(ctx)
	if err != nil {
		t.Fatalf("unable to get the resume marker: %v", err)
	}

	// Do a Put followed by a Get.
	r := tb.Row("testkey")
	if err := r.Put(ctx, "testvalue"); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}
	var result string
	if err := r.Get(ctx, &result); err != nil {
		t.Fatalf("r.Get() failed: %v", err)
	}
	if got, want := result, "testvalue"; got != want {
		t.Fatalf("unexpected value: got %q, want %q", got, want)
	}

	// Do a watch from the resume marker before the put operation.
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	stream, err := d.Watch(ctxWithTimeout, "tb", "", marker)
	if err != nil {
		t.Fatalf("unable to start a watch %v", err)
	}
	if !stream.Advance() {
		t.Fatalf("watch stream unexpectedly reached the end: %v", stream.Err())
	}
	change := stream.Change()
	if got, want := change.Table, "tb"; got != want {
		t.Fatalf("unexpected watch table: got %q, want %q", got, want)
	}
	if got, want := change.Row, "testkey"; got != want {
		t.Fatalf("unexpected watch row: got %q, want %q", got, want)
	}
	if got, want := change.ChangeType, nosql.PutChange; got != want {
		t.Fatalf("unexpected watch change type: got %q, want %q", got, want)
	}
	if got, want := change.FromSync, false; got != want {
		t.Fatalf("unexpected FromSync value: got %t, want %t", got, want)
	}
	if err := vom.Decode(change.ValueBytes, &result); err != nil {
		t.Fatalf("couldn't decode watch value: %v", err)
	}
	if got, want := result, "testvalue"; got != want {
		t.Fatalf("unexpected watch value: got %q, want %q", got, want)
	}

}
