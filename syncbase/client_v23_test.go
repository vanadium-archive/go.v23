// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"time"

	"v.io/v23"
	"v.io/v23/security/access"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/nosql"
	_ "v.io/x/ref/runtime/factories/generic"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test/modules"
	"v.io/x/ref/test/v23tests"
)

//go:generate jiri test generate

const (
	syncbaseName = "syncbase" // Name that syncbase mounts itself at.
)

// TODO(sadovsky): All tests in this file should be updated so that the client
// carries blessing "root/client", so that access is not granted anywhere just
// because the server blessing name is a prefix of the client blessing name or
// vice versa.

func V23TestSyncbasedPutGet(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	clientCreds, _ := t.Shell().NewChildCredentials("server/client")
	serverCreds, _ := t.Shell().NewChildCredentials("server")
	cleanup := tu.StartSyncbased(t, serverCreds, syncbaseName, "",
		`{"Read": {"In":["root/server/client"]}, "Write": {"In":["root/server/client"]}}`)
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

func V23TestServiceRestart(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	clientCreds, _ := t.Shell().NewChildCredentials("server/client")
	serverCreds, _ := t.Shell().NewChildCredentials("server")

	rootDir, err := ioutil.TempDir("", "syncbase_leveldb")
	if err != nil {
		tu.V23Fatalf(t, "can't create temp dir: %v", err)
	}

	perms := tu.DefaultPerms("root/server/client")
	buf := new(bytes.Buffer)
	access.WritePermissions(buf, perms)
	permsLiteral := buf.String()

	cleanup := tu.StartSyncbased(t, serverCreds, syncbaseName, rootDir, permsLiteral)
	tu.RunClient(t, clientCreds, runCreateHierarchy)
	tu.RunClient(t, clientCreds, runCheckHierarchy)
	cleanup()

	cleanup = tu.StartSyncbased(t, serverCreds, syncbaseName, rootDir, permsLiteral)
	// TODO(sadovsky): This time.Sleep() is needed so that we wait for the
	// syncbased server to initialize before sending it RPCs from
	// runCheckHierarchy. Without this sleep, we get errors like "Apps do not
	// match: got [], want [a1 a2]". It'd be nice if tu.StartSyncbased would wait
	// until the server reports that it's ready, and/or if Glob wouldn't return an
	// empty result set before the server is ready. (Perhaps the latter happens
	// because the mount table doesn't care that the glob receiver itself doesn't
	// exist?)
	time.Sleep(2 * time.Second)
	tu.RunClient(t, clientCreds, runCheckHierarchy)
	cleanup()

	if err := os.RemoveAll(rootDir); err != nil {
		tu.V23Fatalf(t, "can't remove dir %v: %v", rootDir, err)
	}
}

// Creates apps, dbs, tables, and rows.
var runCreateHierarchy = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()
	s := syncbase.NewService(syncbaseName)
	for _, a := range []syncbase.App{s.App("a1"), s.App("a2")} {
		if err := a.Create(ctx, nil); err != nil {
			return fmt.Errorf("a.Create() failed: %v", err)
		}
		for _, d := range []nosql.Database{a.NoSQLDatabase("d1", nil), a.NoSQLDatabase("d2", nil)} {
			if err := d.Create(ctx, nil); err != nil {
				return fmt.Errorf("d.Create() failed: %v", err)
			}
			for _, tb := range []nosql.Table{d.Table("tb1"), d.Table("tb2")} {
				if err := d.Table(tb.Name()).Create(ctx, nil); err != nil {
					return fmt.Errorf("d.CreateTable() failed: %v", err)
				}
				for _, k := range []string{"foo", "bar"} {
					if err := tb.Put(ctx, k, k); err != nil {
						return fmt.Errorf("tb.Put() failed: %v", err)
					}
				}
			}
		}
	}
	return nil
}, "runCreateHierarchy")

// Checks for the apps, dbs, tables, and rows created by runCreateHierarchy.
var runCheckHierarchy = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()
	s := syncbase.NewService(syncbaseName)
	var got, want []string
	var err error
	if got, err = s.ListApps(ctx); err != nil {
		return fmt.Errorf("s.ListApps() failed: %v", err)
	}
	want = []string{"a1", "a2"}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("Apps do not match: got %v, want %v", got, want)
	}
	for _, aName := range want {
		a := s.App(aName)
		if got, err = a.ListDatabases(ctx); err != nil {
			return fmt.Errorf("a.ListDatabases() failed: %v", err)
		}
		want = []string{"d1", "d2"}
		if !reflect.DeepEqual(got, want) {
			return fmt.Errorf("Databases do not match: got %v, want %v", got, want)
		}
		for _, dName := range want {
			d := a.NoSQLDatabase(dName, nil)
			if got, err = d.ListTables(ctx); err != nil {
				return fmt.Errorf("d.ListTables() failed: %v", err)
			}
			want = []string{"tb1", "tb2"}
			if !reflect.DeepEqual(got, want) {
				return fmt.Errorf("Tables do not match: got %v, want %v", got, want)
			}
			for _, tbName := range want {
				tb := d.Table(tbName)
				if err := tu.ScanMatches(ctx, tb, nosql.Prefix(""), []string{"bar", "foo"}, []interface{}{"bar", "foo"}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}, "runCheckHierarchy")
