// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testutil defines helpers for tests.
package testutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"runtime/debug"
	"testing"

	wire "v.io/syncbase/v23/services/syncbase/nosql"
	"v.io/syncbase/v23/syncbase"
	"v.io/syncbase/v23/syncbase/nosql"
	"v.io/syncbase/v23/syncbase/util"
	"v.io/syncbase/x/ref/services/syncbase/server"
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
	"v.io/x/ref/lib/flags"
	"v.io/x/ref/lib/xrpc"
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
	d := a.NoSQLDatabase(name, nil)
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

// TODO(sadovsky): Drop the 'perms' argument. The only client that passes
// non-nil, syncgroup_test.go, should use SetupOrDieCustom instead.
func SetupOrDie(perms access.Permissions) (clientCtx *context.T, serverName string, cleanup func()) {
	_, clientCtx, serverName, _, cleanup = SetupOrDieCustom("client", "server", perms)
	return
}

func SetupOrDieCustom(clientSuffix, serverSuffix string, perms access.Permissions) (ctx, clientCtx *context.T, serverName string, rootp security.Principal, cleanup func()) {
	// TODO(mattr): Instead of SetDefaultHostPort the arguably more correct thing
	// would be to call v.io/x/ref/test.Init() from the test packages that import
	// the profile.  Note you should only call that from the package that imports
	// the profile, not from libraries like this.  Also, it would be better if
	// v23.Init was test.V23Init().
	flags.SetDefaultHostPort("127.0.0.1:0")
	ctx, shutdown := v23.Init()

	rootp = tsecurity.NewPrincipal("root")
	clientCtx, serverCtx := NewCtx(ctx, rootp, clientSuffix), NewCtx(ctx, rootp, serverSuffix)

	if perms == nil {
		perms = DefaultPerms(fmt.Sprintf("%s/%s", "root", clientSuffix))
	}
	serverName, stopServer := newServer(serverCtx, perms)
	cleanup = func() {
		stopServer()
		shutdown()
	}
	return
}

func DefaultPerms(patterns ...string) access.Permissions {
	perms := access.Permissions{}
	for _, tag := range access.AllTypicalTags() {
		for _, pattern := range patterns {
			perms.Add(security.BlessingPattern(pattern), string(tag))
		}
	}
	return perms
}

func ScanMatches(ctx *context.T, tb nosql.Table, r nosql.RowRange, wantKeys []string, wantValues []interface{}) error {
	if len(wantKeys) != len(wantValues) {
		return fmt.Errorf("bad input args")
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
			return fmt.Errorf("Keys do not match: got %q, want %q", gotKey, wantKey)
		}
		// Check value.
		wantValue := wantValues[i]
		gotValue := reflect.Zero(reflect.TypeOf(wantValue)).Interface()
		if err := it.Value(&gotValue); err != nil {
			return fmt.Errorf("it.Value() failed: %v", err)
		}
		if !reflect.DeepEqual(gotValue, wantValue) {
			return fmt.Errorf("Values do not match: got %v, want %v", gotValue, wantValue)
		}
	}
	if err := it.Err(); err != nil {
		return fmt.Errorf("tb.Scan() failed: %v", err)
	}
	if len(gotKeys) != len(wantKeys) {
		return fmt.Errorf("Unmatched keys: got %v, want %v", gotKeys, wantKeys)
	}
	return nil
}

func CheckScan(t *testing.T, ctx *context.T, tb nosql.Table, r nosql.RowRange, wantKeys []string, wantValues []interface{}) {
	if err := ScanMatches(ctx, tb, r, wantKeys, wantValues); err != nil {
		Fatalf(t, err.Error())
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

type MockSchemaUpgrader struct {
	CallCount int
}

func (msu *MockSchemaUpgrader) Run(db nosql.Database, oldVersion, newVersion int32) error {
	msu.CallCount++
	return nil
}

var _ nosql.SchemaUpgrader = (*MockSchemaUpgrader)(nil)

func DefaultSchema(version int32) *nosql.Schema {
	return &nosql.Schema{
		Metadata: wire.SchemaMetadata{
			Version: version,
		},
		Upgrader: nosql.SchemaUpgrader(&MockSchemaUpgrader{}),
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

func newServer(serverCtx *context.T, perms access.Permissions) (string, func()) {
	if perms == nil {
		vlog.Fatal("perms must be specified")
	}
	rootDir, err := ioutil.TempDir("", "syncbase")
	if err != nil {
		vlog.Fatal("ioutil.TempDir() failed: ", err)
	}
	service, err := server.NewService(serverCtx, nil, server.ServiceOptions{
		Perms:   perms,
		RootDir: rootDir,
		Engine:  "leveldb",
	})
	if err != nil {
		vlog.Fatal("server.NewService() failed: ", err)
	}
	s, err := xrpc.NewDispatchingServer(serverCtx, "", server.NewDispatcher(service))
	if err != nil {
		vlog.Fatal("xrpc.NewDispatchingServer() failed: ", err)
	}
	name := s.Status().Endpoints[0].Name()
	return name, func() {
		s.Stop()
		os.RemoveAll(rootDir)
	}
}

// Creates a new context object with blessing "root/<suffix>", configured to
// present this blessing when acting as a server as well as when acting as a
// client and talking to a server that presents a blessing rooted at "root".
func NewCtx(ctx *context.T, rootp security.Principal, suffix string) *context.T {
	// Principal for the new context.
	p := tsecurity.NewPrincipal(suffix)

	// Bless the new principal as "root/<suffix>".
	blessings, err := rootp.Bless(p.PublicKey(), rootp.BlessingStore().Default(), suffix, security.UnconstrainedUse())
	if err != nil {
		vlog.Fatal("rootp.Bless() failed: ", err)
	}

	// Make it so users of the new context present their "root/<suffix>" blessing
	// when talking to servers with blessings rooted at "root".
	if _, err := p.BlessingStore().Set(blessings, security.BlessingPattern("root")); err != nil {
		vlog.Fatal("p.BlessingStore().Set() failed: ", err)
	}

	// Make it so that when users of the new context act as a server, they present
	// their "root/<suffix>" blessing.
	if err := p.BlessingStore().SetDefault(blessings); err != nil {
		vlog.Fatal("p.BlessingStore().SetDefault() failed: ", err)
	}

	// Have users of the prepared context treat root's public key as an authority
	// on all blessings rooted at "root".
	if err := p.AddToRoots(blessings); err != nil {
		vlog.Fatal("p.AddToRoots() failed: ", err)
	}

	resCtx, err := v23.WithPrincipal(ctx, p)
	if err != nil {
		vlog.Fatal("v23.WithPrincipal() failed: ", err)
	}

	return resCtx
}
