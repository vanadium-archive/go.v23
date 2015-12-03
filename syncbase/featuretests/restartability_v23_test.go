// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"syscall"
	"time"

	"v.io/v23/context"
	sbwire "v.io/v23/services/syncbase"
	nosqlwire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/nosql"
	"v.io/v23/verror"
	"v.io/v23/vom"
	_ "v.io/x/ref/runtime/factories/generic"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test/modules"
	"v.io/x/ref/test/v23tests"
)

//go:generate jiri test generate

const (
	acl = `{"Read": {"In":["root:u:client"]}, "Write": {"In":["root:u:client"]}, "Resolve": {"In":["root:u:client"]}}`
)

func restartabilityInit(t *v23tests.T) (rootDir string, clientCtx *context.T, serverCreds *modules.CustomCredentials) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")

	var err error
	rootDir, err = ioutil.TempDir("", "syncbase_leveldb")
	if err != nil {
		tu.V23Fatalf(t, "can't create temp dir: %v", err)
	}

	clientCtx = forkContext(t, "u:client")
	serverCreds = forkCredentials(t, "r:server")
	return
}

func createAppDatabaseTable(t *v23tests.T, clientCtx *context.T) nosql.Database {
	a := syncbase.NewService(syncbaseName).App("a")
	if err := a.Create(clientCtx, nil); err != nil {
		t.Fatalf("unable to create an app: %v", err)
	}
	d := a.NoSQLDatabase("d", nil)
	if err := d.Create(clientCtx, nil); err != nil {
		t.Fatalf("unable to create a database: %v", err)
	}
	if err := d.Table("tb").Create(clientCtx, nil); err != nil {
		t.Fatalf("unable to create a table: %v", err)
	}
	return d
}

func V23TestRestartabilityHierarchy(t *v23tests.T) {
	rootDir, clientCtx, serverCreds := restartabilityInit(t)
	cleanup := tu.StartSyncbased(t, serverCreds, syncbaseName, rootDir, acl)

	createHierarchy(t, clientCtx)
	checkHierarchy(t, clientCtx)
	cleanup()

	cleanup = tu.StartSyncbased(t, serverCreds, syncbaseName, rootDir, acl)
	defer cleanup()
	checkHierarchy(t, clientCtx)
}

// Same as V23TestRestartabilityHierarchy except the first syncbase is killed
// with SIGKILL instead of SIGINT.
func V23TestRestartabilityCrash(t *v23tests.T) {
	rootDir, clientCtx, serverCreds := restartabilityInit(t)
	cleanup := tu.StartKillableSyncbased(t, serverCreds, syncbaseName, rootDir, acl)

	createHierarchy(t, clientCtx)
	checkHierarchy(t, clientCtx)
	cleanup(syscall.SIGKILL)

	cleanup2 := tu.StartSyncbased(t, serverCreds, syncbaseName, rootDir, acl)
	defer cleanup2()
	checkHierarchy(t, clientCtx)
}

// Creates apps, dbs, tables, and rows.
func createHierarchy(t *v23tests.T, ctx *context.T) {
	s := syncbase.NewService(syncbaseName)
	for _, a := range []syncbase.App{s.App("a1"), s.App("a2")} {
		if err := a.Create(ctx, nil); err != nil {
			t.Fatalf("a.Create() failed: %v", err)
		}
		for _, d := range []nosql.Database{a.NoSQLDatabase("d1", nil), a.NoSQLDatabase("d2", nil)} {
			if err := d.Create(ctx, nil); err != nil {
				t.Fatalf("d.Create() failed: %v", err)
			}
			for _, tb := range []nosql.Table{d.Table("tb1"), d.Table("tb2")} {
				if err := d.Table(tb.Name()).Create(ctx, nil); err != nil {
					t.Fatalf("d.CreateTable() failed: %v", err)
				}
				for _, k := range []string{"foo", "bar"} {
					if err := tb.Put(ctx, k, k); err != nil {
						t.Fatalf("tb.Put() failed: %v", err)
					}
				}
			}
		}
	}
}

// Checks for the apps, dbs, tables, and rows created by runCreateHierarchy.
func checkHierarchy(t *v23tests.T, ctx *context.T) {
	s := syncbase.NewService(syncbaseName)
	var got, want []string
	var err error
	if got, err = s.ListApps(ctx); err != nil {
		t.Fatalf("s.ListApps() failed: %v", err)
	}
	want = []string{"a1", "a2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Apps do not match: got %v, want %v", got, want)
	}
	for _, aName := range want {
		a := s.App(aName)
		if got, err = a.ListDatabases(ctx); err != nil {
			t.Fatalf("a.ListDatabases() failed: %v", err)
		}
		want = []string{"d1", "d2"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Databases do not match: got %v, want %v", got, want)
		}
		for _, dName := range want {
			d := a.NoSQLDatabase(dName, nil)
			if got, err = d.ListTables(ctx); err != nil {
				t.Fatalf("d.ListTables() failed: %v", err)
			}
			want = []string{"tb1", "tb2"}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Tables do not match: got %v, want %v", got, want)
			}
			for _, tbName := range want {
				tb := d.Table(tbName)
				if err := tu.ScanMatches(ctx, tb, nosql.Prefix(""), []string{"bar", "foo"}, []interface{}{"bar", "foo"}); err != nil {
					t.Fatalf("Scan does not match: %v", err)
				}
			}
		}
	}
}

func V23TestRestartabilityQuiescent(t *v23tests.T) {
	rootDir, clientCtx, serverCreds := restartabilityInit(t)
	cleanup := tu.StartKillableSyncbased(t, serverCreds, syncbaseName, rootDir, acl)
	d := createAppDatabaseTable(t, clientCtx)

	tb := d.Table("tb")

	// Do Put followed by Get on a row.
	r := tb.Row("r")
	if err := r.Put(clientCtx, "testkey"); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}
	var result string
	if err := r.Get(clientCtx, &result); err != nil {
		t.Fatalf("r.Get() failed: %v", err)
	}
	if got, want := result, "testkey"; got != want {
		t.Fatalf("unexpected value: got %q, want %q", got, want)
	}

	cleanup(syscall.SIGKILL)
	// Restart syncbase.
	cleanup2 := tu.StartSyncbased(t, serverCreds, syncbaseName, rootDir, acl)
	defer cleanup2()

	if err := r.Get(clientCtx, &result); err != nil {
		t.Fatalf("r.Get() failed: %v", err)
	}
	if got, want := result, "testkey"; got != want {
		t.Fatalf("unexpected value: got %q, want %q", got, want)
	}
}

// A read-only batch should fail if the server crashes in the middle.
func V23TestRestartabilityReadOnlyBatch(t *v23tests.T) {
	rootDir, clientCtx, serverCreds := restartabilityInit(t)
	cleanup := tu.StartKillableSyncbased(t, serverCreds, syncbaseName, rootDir, acl)
	d := createAppDatabaseTable(t, clientCtx)

	// Add one row.
	if err := d.Table("tb").Row("r").Put(clientCtx, "testkey"); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}

	batch, err := d.BeginBatch(clientCtx, nosqlwire.BatchOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("unable to start batch: %v", err)
	}
	tb := batch.Table("tb")
	r := tb.Row("r")

	var result string
	if err := r.Get(clientCtx, &result); err != nil {
		t.Fatalf("r.Get() failed: %v", err)
	}
	if got, want := result, "testkey"; got != want {
		t.Fatalf("unexpected value: got %q, want %q", got, want)
	}

	cleanup(syscall.SIGKILL)
	expectedFailCtx, _ := context.WithTimeout(clientCtx, time.Second)
	// We get a variety of errors depending on how much of the network state of
	// syncbased has been reclaimed when this rpc goes out.
	if err := r.Get(expectedFailCtx, &result); err == nil {
		t.Fatalf("expected r.Get() to fail.")
	}

	// Restart syncbase.
	cleanup2 := tu.StartSyncbased(t, serverCreds, syncbaseName, rootDir, acl)
	defer cleanup2()

	if err := r.Get(clientCtx, &result); verror.ErrorID(err) != sbwire.ErrUnknownBatch.ID {
		t.Fatalf("expected r.Get() to fail because of ErrUnknownBatch.  got: %v", err)
	}
	if err := batch.Commit(clientCtx); verror.ErrorID(err) != sbwire.ErrUnknownBatch.ID {
		t.Fatalf("expected Commit() to fail because of ErrUnknownBatch.  got: %v", err)
	}

	// Try to get the row outside of a batch.  It should exist.
	if err := d.Table("tb").Row("r").Get(clientCtx, &result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// A read/write batch should fail if the server crashes in the middle.
func V23TestRestartabilityReadWriteBatch(t *v23tests.T) {
	rootDir, clientCtx, serverCreds := restartabilityInit(t)
	cleanup := tu.StartKillableSyncbased(t, serverCreds, syncbaseName, rootDir, acl)
	d := createAppDatabaseTable(t, clientCtx)

	batch, err := d.BeginBatch(clientCtx, nosqlwire.BatchOptions{})
	if err != nil {
		t.Fatalf("unable to start batch: %v", err)
	}
	tb := batch.Table("tb")

	// Do Put followed by Get on a row.
	r := tb.Row("r")
	if err := r.Put(clientCtx, "testkey"); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}
	var result string
	if err := r.Get(clientCtx, &result); err != nil {
		t.Fatalf("r.Get() failed: %v", err)
	}
	if got, want := result, "testkey"; got != want {
		t.Fatalf("unexpected value: got %q, want %q", got, want)
	}

	cleanup(syscall.SIGKILL)
	expectedFailCtx, _ := context.WithTimeout(clientCtx, time.Second)
	// We get a variety of errors depending on how much of the network state of
	// syncbased has been reclaimed when this rpc goes out.
	if err := r.Get(expectedFailCtx, &result); err == nil {
		t.Fatalf("expected r.Get() to fail.")
	}

	// Restart syncbase.
	cleanup2 := tu.StartSyncbased(t, serverCreds, syncbaseName, rootDir, acl)
	defer cleanup2()

	if err := r.Get(clientCtx, &result); verror.ErrorID(err) != sbwire.ErrUnknownBatch.ID {
		t.Fatalf("expected r.Get() to fail because of ErrUnknownBatch.  got: %v", err)
	}
	if err := batch.Commit(clientCtx); verror.ErrorID(err) != sbwire.ErrUnknownBatch.ID {
		t.Fatalf("expected Commit() to fail because of ErrUnknownBatch.  got: %v", err)
	}

	// Try to get the row outside of a batch.  It should not exist.
	if err := d.Table("tb").Row("r").Get(clientCtx, &result); verror.ErrorID(err) != verror.ErrNoExist.ID {
		t.Fatalf("expected r.Get() to fail because of ErrNoExist.  got: %v", err)
	}
}

func decodeString(t *v23tests.T, val []byte) string {
	var ret string
	if err := vom.Decode(val, &ret); err != nil {
		t.Fatalf("unable to decode: %v", err)
	}
	return ret
}

func V23TestRestartabilityWatch(t *v23tests.T) {
	rootDir, clientCtx, serverCreds := restartabilityInit(t)
	cleanup := tu.StartKillableSyncbased(t, serverCreds, syncbaseName, rootDir, acl)
	d := createAppDatabaseTable(t, clientCtx)

	// Put one row as well as get the initial ResumeMarker.
	batch, err := d.BeginBatch(clientCtx, nosqlwire.BatchOptions{})
	if err != nil {
		t.Fatalf("unable to start batch: %v", err)
	}
	marker, err := batch.GetResumeMarker(clientCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := batch.Table("tb").Row("r")
	if err := r.Put(clientCtx, "testvalue1"); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}
	if err := batch.Commit(clientCtx); err != nil {
		t.Fatalf("could not commit: %v", err)
	}

	// Watch for the row change.
	timeout, _ := context.WithTimeout(clientCtx, time.Second)
	stream, err := d.Watch(timeout, "tb", "r", marker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !stream.Advance() {
		cleanup(syscall.SIGINT)
		t.Fatalf("expected to be able to Advance: %v", stream.Err())
	}
	change := stream.Change()
	val := decodeString(t, change.ValueBytes)
	if change.Row != "r" || val != "testvalue1" {
		t.Fatalf("unexpected row: %s", change.Row)
	}
	marker = change.ResumeMarker

	// Kill syncbased.
	cleanup(syscall.SIGKILL)

	// The stream should break when the server crashes.
	if stream.Advance() {
		t.Fatalf("unexpected Advance: %v", stream.Change())
	}

	// Restart syncbased.
	cleanup = tu.StartKillableSyncbased(t, serverCreds, syncbaseName, rootDir, acl)

	// Put another row.
	r = d.Table("tb").Row("r")
	if err := r.Put(clientCtx, "testvalue2"); err != nil {
		t.Fatalf("r.Put() failed: %v", err)
	}

	// Resume the watch from after the first Put.  We should see only the second
	// Put.
	stream, err = d.Watch(clientCtx, "tb", "", marker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !stream.Advance() {
		t.Fatalf("expected to be able to Advance: %v", stream.Err())
	}
	change = stream.Change()
	val = decodeString(t, change.ValueBytes)
	if change.Row != "r" || val != "testvalue2" {
		t.Fatalf("unexpected row: %s, %s", change.Row, val)
	}

	cleanup(syscall.SIGKILL)

	// The stream should break when the server crashes.
	if stream.Advance() {
		t.Fatalf("unexpected Advance: %v", stream.Change())
	}
}
func V23TestRestartabilityCorruption(t *v23tests.T) {
	rootDir, clientCtx, serverCreds := restartabilityInit(t)
	cleanup := tu.StartKillableSyncbased(t, serverCreds, syncbaseName, rootDir, acl)

	createHierarchy(t, clientCtx)
	checkHierarchy(t, clientCtx)
	cleanup(syscall.SIGKILL)

	var fileToCorrupt string
	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if fileToCorrupt != "" {
			return nil
		}
		if match, _ := regexp.MatchString(`apps/[^/]*/dbs/[^/]*/leveldb/.*\.log`, path); match {
			fileToCorrupt = path
			return errors.New("found match, stop walking")
		}
		return nil
	})
	if fileToCorrupt == "" {
		t.Fatalf("Could not find file")
	}
	fileBytes, err := ioutil.ReadFile(fileToCorrupt)
	if err != nil {
		t.Fatalf("Could not read log file: %v", err)
	}
	// Overwrite last 100 bytes.
	offset := len(fileBytes) - 100 - 1
	if offset < 0 {
		t.Fatalf("Expected bigger log file.  Found: %d", len(fileBytes))
	}
	for i := 0; i < 100; i++ {
		fileBytes[i+offset] = 0x80
	}
	if err := ioutil.WriteFile(fileToCorrupt, fileBytes, 0); err != nil {
		t.Fatalf("Could not corrupt file: %v", err)
	}

	// Expect syncbase to fail to start.
	syncbased := t.BuildV23Pkg("v.io/x/ref/services/syncbase/syncbased")
	startOpts := syncbased.StartOpts().WithCustomCredentials(serverCreds)
	invocation := syncbased.WithStartOpts(startOpts).Start(
		"--alsologtostderr=true",
		"--v23.tcp.address=127.0.0.1:0",
		"--v23.permissions.literal", acl,
		"--name="+syncbaseName,
		"--root-dir="+rootDir)
	stdout, stderr := bytes.NewBuffer(nil), bytes.NewBuffer(nil)
	if err := invocation.Wait(stdout, stderr); err == nil {
		t.Fatalf("Expected syncbased to fail to start.")
	}
	log.Printf("syncbased terminated\nstdout: %v\nstderr: %v\n", stdout, stderr)
	// TODO(kash): What should really happen is syncbased returns a specific
	// error for the corruption and moves the corrupted leveldb aside.  That
	// would cause the app to create a new one on restart.
}
