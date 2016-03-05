// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
	"time"

	"v.io/v23/context"
	sbwire "v.io/v23/services/syncbase"
	nosqlwire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/nosql"
	"v.io/v23/verror"
	"v.io/v23/vom"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test/v23test"
)

const (
	acl = `{"Read": {"In":["root:u:client"]}, "Write": {"In":["root:u:client"]}, "Resolve": {"In":["root:u:client"]}}`
)

func restartabilityInit(sh *v23test.Shell) (rootDir string, clientCtx *context.T, serverCreds *v23test.Credentials) {
	sh.StartRootMountTable()

	rootDir = sh.MakeTempDir()
	clientCtx = sh.ForkContext("u:client")
	serverCreds = sh.ForkCredentials("r:server")
	return
}

// TODO(ivanpi): Duplicate of setupAppA.
func createAppDatabaseTable(t *testing.T, clientCtx *context.T) nosql.Database {
	a := syncbase.NewService(testSbName).App("a")
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

func TestV23RestartabilityHierarchy(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	rootDir, clientCtx, serverCreds := restartabilityInit(sh)
	cleanup := sh.StartSyncbase(serverCreds, testSbName, rootDir, acl)

	createHierarchy(t, clientCtx)
	checkHierarchy(t, clientCtx)
	cleanup(os.Interrupt)

	_ = sh.StartSyncbase(serverCreds, testSbName, rootDir, acl)
	checkHierarchy(t, clientCtx)
}

// Same as TestV23RestartabilityHierarchy except the first syncbase is killed
// with SIGKILL instead of SIGINT.
func TestV23RestartabilityCrash(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	rootDir, clientCtx, serverCreds := restartabilityInit(sh)
	cleanup := sh.StartSyncbase(serverCreds, testSbName, rootDir, acl)

	createHierarchy(t, clientCtx)
	checkHierarchy(t, clientCtx)
	cleanup(os.Kill)

	_ = sh.StartSyncbase(serverCreds, testSbName, rootDir, acl)
	checkHierarchy(t, clientCtx)
}

// Creates apps, dbs, tables, and rows.
func createHierarchy(t *testing.T, ctx *context.T) {
	s := syncbase.NewService(testSbName)
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
func checkHierarchy(t *testing.T, ctx *context.T) {
	s := syncbase.NewService(testSbName)
	var got, want []string
	var err error
	for len(got) != 2 {
		if got, err = s.ListApps(ctx); err != nil {
			t.Fatalf("s.ListApps() failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
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

func TestV23RestartabilityQuiescent(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	rootDir, clientCtx, serverCreds := restartabilityInit(sh)
	cleanup := sh.StartSyncbase(serverCreds, testSbName, rootDir, acl)
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

	cleanup(os.Kill)
	// Restart syncbase.
	_ = sh.StartSyncbase(serverCreds, testSbName, rootDir, acl)

	if err := r.Get(clientCtx, &result); err != nil {
		t.Fatalf("r.Get() failed: %v", err)
	}
	if got, want := result, "testkey"; got != want {
		t.Fatalf("unexpected value: got %q, want %q", got, want)
	}
}

// A read-only batch should fail if the server crashes in the middle.
func TestV23RestartabilityReadOnlyBatch(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	rootDir, clientCtx, serverCreds := restartabilityInit(sh)
	cleanup := sh.StartSyncbase(serverCreds, testSbName, rootDir, acl)
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

	cleanup(os.Kill)
	expectedFailCtx, _ := context.WithTimeout(clientCtx, time.Second)
	// We get a variety of errors depending on how much of the network state of
	// syncbased has been reclaimed when this rpc goes out.
	if err := r.Get(expectedFailCtx, &result); err == nil {
		t.Fatalf("expected r.Get() to fail.")
	}

	// Restart syncbase.
	_ = sh.StartSyncbase(serverCreds, testSbName, rootDir, acl)

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
func TestV23RestartabilityReadWriteBatch(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	rootDir, clientCtx, serverCreds := restartabilityInit(sh)
	cleanup := sh.StartSyncbase(serverCreds, testSbName, rootDir, acl)
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

	cleanup(os.Kill)
	expectedFailCtx, _ := context.WithTimeout(clientCtx, time.Second)
	// We get a variety of errors depending on how much of the network state of
	// syncbased has been reclaimed when this rpc goes out.
	if err := r.Get(expectedFailCtx, &result); err == nil {
		t.Fatalf("expected r.Get() to fail.")
	}

	// Restart syncbase.
	_ = sh.StartSyncbase(serverCreds, testSbName, rootDir, acl)

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

func decodeString(t *testing.T, val []byte) string {
	var ret string
	if err := vom.Decode(val, &ret); err != nil {
		t.Fatalf("unable to decode: %v", err)
	}
	return ret
}

func TestV23RestartabilityWatch(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	rootDir, clientCtx, serverCreds := restartabilityInit(sh)
	cleanup := sh.StartSyncbase(serverCreds, testSbName, rootDir, acl)
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
		cleanup(os.Interrupt)
		t.Fatalf("expected to be able to Advance: %v", stream.Err())
	}
	change := stream.Change()
	val := decodeString(t, change.ValueBytes)
	if change.Row != "r" || val != "testvalue1" {
		t.Fatalf("unexpected row: %s", change.Row)
	}
	marker = change.ResumeMarker

	// Kill syncbased.
	cleanup(os.Kill)

	// The stream should break when the server crashes.
	if stream.Advance() {
		t.Fatalf("unexpected Advance: %v", stream.Change())
	}

	// Restart syncbased.
	cleanup = sh.StartSyncbase(serverCreds, testSbName, rootDir, acl)

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

	cleanup(os.Kill)

	// The stream should break when the server crashes.
	if stream.Advance() {
		t.Fatalf("unexpected Advance: %v", stream.Change())
	}
}

func corruptFile(t *testing.T, rootDir, pathRegex string) {
	var fileToCorrupt string
	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if fileToCorrupt != "" {
			return nil
		}
		if match, _ := regexp.MatchString(pathRegex, path); match {
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
}

func TestV23RestartabilityServiceDBCorruption(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	rootDir, clientCtx, serverCreds := restartabilityInit(sh)
	cleanup := sh.StartSyncbase(serverCreds, testSbName, rootDir, acl)

	createHierarchy(t, clientCtx)
	checkHierarchy(t, clientCtx)
	cleanup(os.Kill)

	corruptFile(t, rootDir, filepath.Join(rootDir, `leveldb/.*\.log`))

	// TODO(ivanpi): Repeated below, refactor into method.
	// Expect syncbase to fail to start.
	syncbasedPath := v23test.BuildGoPkg(sh, "v.io/x/ref/services/syncbase/syncbased")
	syncbased := sh.Cmd(syncbasedPath,
		"--alsologtostderr=true",
		"--v23.tcp.address=127.0.0.1:0",
		"--v23.permissions.literal", acl,
		"--name="+testSbName,
		"--root-dir="+rootDir)
	syncbased = syncbased.WithCredentials(serverCreds)
	syncbased.ExitErrorIsOk = true
	stdout, stderr := syncbased.StdoutStderr()
	if syncbased.Err == nil {
		t.Fatal("Expected syncbased to fail to start.")
	}
	t.Logf("syncbased terminated\nstdout: %v\nstderr: %v\n", stdout, stderr)

	cleanup = sh.StartSyncbase(serverCreds, testSbName, rootDir, acl)

	createHierarchy(t, clientCtx)
	checkHierarchy(t, clientCtx)
	cleanup(os.Kill)
}

func TestV23RestartabilityAppDBCorruption(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	rootDir, clientCtx, serverCreds := restartabilityInit(sh)
	cleanup := sh.StartSyncbase(serverCreds, testSbName, rootDir, acl)

	createHierarchy(t, clientCtx)
	checkHierarchy(t, clientCtx)
	cleanup(os.Kill)

	corruptFile(t, rootDir, `apps/[^/]*/dbs/[^/]*/leveldb/.*\.log`)

	// Expect syncbase to fail to start.
	syncbasedPath := v23test.BuildGoPkg(sh, "v.io/x/ref/services/syncbase/syncbased")
	syncbased := sh.Cmd(syncbasedPath,
		"--alsologtostderr=true",
		"--v23.tcp.address=127.0.0.1:0",
		"--v23.permissions.literal", acl,
		"--name="+testSbName,
		"--root-dir="+rootDir)
	syncbased = syncbased.WithCredentials(serverCreds)
	syncbased.ExitErrorIsOk = true
	stdout, stderr := syncbased.StdoutStderr()
	if syncbased.Err == nil {
		t.Fatal("Expected syncbased to fail to start.")
	}
	t.Logf("syncbased terminated\nstdout: %v\nstderr: %v\n", stdout, stderr)

	cleanup = sh.StartSyncbase(serverCreds, testSbName, rootDir, acl)

	// Recreate a1/d1 since that is the one that got corrupted.
	d := syncbase.NewService(testSbName).App("a1").NoSQLDatabase("d1", nil)
	if err := d.Create(clientCtx, nil); err != nil {
		t.Fatalf("d.Create() failed: %v", err)
	}
	for _, tb := range []nosql.Table{d.Table("tb1"), d.Table("tb2")} {
		if err := tb.Create(clientCtx, nil); err != nil {
			t.Fatalf("d.CreateTable() failed: %v", err)
		}
		for _, k := range []string{"foo", "bar"} {
			if err := tb.Put(clientCtx, k, k); err != nil {
				t.Fatalf("tb.Put() failed: %v", err)
			}
		}
	}

	checkHierarchy(t, clientCtx)
	cleanup(os.Kill)
}

func TestV23RestartabilityStoreGarbageCollect(t *testing.T) {
	// TODO(ivanpi): Fully testing store garbage collection requires fault
	// injection or mocking out the store.
	// NOTE: Test assumes that leveldb destroy is implemented as 'rm -r'.
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	rootDir, clientCtx, serverCreds := restartabilityInit(sh)
	cleanup := sh.StartSyncbase(serverCreds, testSbName, rootDir, acl)

	createHierarchy(t, clientCtx)
	checkHierarchy(t, clientCtx)

	const writeMask = 0220
	// Find a leveldb directory for one of the database stores.
	var leveldbDir string
	var leveldbMode os.FileMode
	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if leveldbDir != "" {
			return nil
		}
		if match, _ := regexp.MatchString(`apps/[^/]*/dbs/[^/]*/leveldb`, path); match && info.IsDir() {
			leveldbDir = path
			leveldbMode = info.Mode()
			return errors.New("found match, stop walking")
		}
		return nil
	})
	if leveldbDir == "" {
		t.Fatalf("Could not find file")
	}

	// Create a subdirectory in the leveldb directory and populate it.
	anchorDir := filepath.Join(leveldbDir, "test-anchor")
	if err := os.Mkdir(anchorDir, leveldbMode); err != nil {
		t.Fatalf("Mkdir() failed: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(anchorDir, "a"), []byte("a"), 0644); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}
	// Remove write permission from the subdir so that store destroy fails.
	if err := os.Chmod(anchorDir, leveldbMode&^writeMask); err != nil {
		t.Fatalf("Chmod() failed: %v", err)
	}

	s := syncbase.NewService(testSbName)

	// Destroy all apps and their databases. Destroy() should not fail even
	// though the database store destruction should fail for leveldbDir picked
	// above.
	if apps, err := s.ListApps(clientCtx); err != nil {
		t.Fatalf("s.ListApps() failed: %v", err)
	} else {
		for _, aName := range apps {
			if err := s.App(aName).Destroy(clientCtx); err != nil {
				t.Fatalf("a.Destroy() failed: %v", err)
			}
		}
	}
	// TODO(ivanpi): Add Exists() checks.
	// TODO(ivanpi): Destroy databases individually instead of apps.

	// leveldbDir should still exist.
	// TODO(ivanpi): Check that other stores have been destroyed.
	if _, err := os.Stat(leveldbDir); err != nil {
		t.Errorf("os.Stat() for old store failed: %v", err)
	}

	// Recreate the hierarchy. This should not be affected by the old store.
	createHierarchy(t, clientCtx)
	checkHierarchy(t, clientCtx)

	cleanup(os.Kill)

	// leveldbDir should still exist.
	if _, err := os.Stat(leveldbDir); err != nil {
		t.Errorf("os.Stat() for old store failed: %v", err)
	}

	// Restarting syncbased should not affect the hierarchy. Garbage collection
	// should again fail to destroy leveldbDir.
	cleanup = sh.StartSyncbase(serverCreds, testSbName, rootDir, acl)
	checkHierarchy(t, clientCtx)
	cleanup(os.Kill)

	// leveldbDir should still exist.
	if _, err := os.Stat(leveldbDir); err != nil {
		t.Errorf("os.Stat() for old store failed: %v", err)
	}

	// Reinstate write permission for the anchor subdir in leveldbDir.
	if err := os.Chmod(anchorDir, leveldbMode|writeMask); err != nil {
		t.Fatalf("Chmod() failed: %v", err)
	}

	// Restart syncbased. Garbage collection should now succeed.
	_ = sh.StartSyncbase(serverCreds, testSbName, rootDir, acl)

	// leveldbDir should not exist anymore.
	if _, err := os.Stat(leveldbDir); !os.IsNotExist(err) {
		t.Errorf("os.Stat() for old store should have failed with ErrNotExist, got: %v", err)
	}

	// The hierarchy should not have been affected.
	checkHierarchy(t, clientCtx)
}
