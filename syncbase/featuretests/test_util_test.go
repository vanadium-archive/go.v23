// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security/access"
	wire_syncbase "v.io/v23/services/syncbase"
	wire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/nosql"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test/v23test"
)

const (
	testSbName = "syncbase" // Name that syncbase mounts itself at.
	testApp    = "a"
	testDb     = "d"
	testTable  = "tb"
)

////////////////////////////////////////////////////////////
// Helpers for setting up Syncbases, apps, and dbs

func setupHierarchy(ctx *context.T, syncbaseName string) error {
	a := syncbase.NewService(syncbaseName).App(testApp)
	if err := a.Create(ctx, nil); err != nil {
		return err
	}
	d := a.NoSQLDatabase(testDb, nil)
	if err := d.Create(ctx, nil); err != nil {
		return err
	}
	return d.Table(testTable).Create(ctx, nil)
}

type testSyncbase struct {
	sbName    string
	sbCreds   *v23test.Credentials
	clientId  string
	clientCtx *context.T
}

// Spawns "num" Syncbase instances and returns handles to them.
func setupSyncbases(t testing.TB, sh *v23test.Shell, num int, args ...string) []*testSyncbase {
	sbs := make([]*testSyncbase, num)
	for i, _ := range sbs {
		sbName, clientId := fmt.Sprintf("s%d", i), fmt.Sprintf("c%d", i)
		sbs[i] = &testSyncbase{
			sbName:    sbName,
			sbCreds:   sh.ForkCredentials(sbName),
			clientId:  clientId,
			clientCtx: sh.ForkContext(clientId),
		}
		// Give XRWA permissions to this Syncbase's client.
		acl := fmt.Sprintf(`{"Resolve":{"In":["root:%s"]},"Read":{"In":["root:%s"]},"Write":{"In":["root:%s"]},"Admin":{"In":["root:%s"]}}`, clientId, clientId, clientId, clientId)
		sh.StartSyncbase(sbs[i].sbCreds, sbs[i].sbName, "", acl, args...)
	}
	// Call setupHierarchy on each Syncbase.
	for _, sb := range sbs {
		ok(t, setupHierarchy(sb.clientCtx, sb.sbName))
	}
	return sbs
}

// Returns a ";"-separated list of Syncbase blessing names.
func sbBlessings(sbs []*testSyncbase) string {
	names := make([]string, len(sbs))
	for i, sb := range sbs {
		names[i] = "root:" + sb.sbName
	}
	return strings.Join(names, ";")
}

////////////////////////////////////////////////////////////
// Helpers for adding or updating data

func populateData(ctx *context.T, syncbaseName, keyPrefix string, start, end int) error {
	if end <= start {
		return fmt.Errorf("end (%d) <= start (%d)", end, start)
	}

	a := syncbase.NewService(syncbaseName).App(testApp)
	d := a.NoSQLDatabase(testDb, nil)
	tb := d.Table(testTable)

	for i := start; i < end; i++ {
		key := fmt.Sprintf("%s%d", keyPrefix, i)
		if err := tb.Put(ctx, key, "testkey"+key); err != nil {
			return fmt.Errorf("tb.Put() failed: %v", err)
		}
	}
	return nil
}

// Shared by updateData and updateBatchData.
// TODO(sadovsky): This is eerily similar to populateData. We should strive to
// avoid such redundancies by implementing common helpers and avoiding spurious
// differences.
func updateDataImpl(ctx *context.T, d nosql.DatabaseHandle, syncbaseName string, start, end int, valuePrefix string) error {
	if end <= start {
		return fmt.Errorf("end (%d) <= start (%d)", end, start)
	}
	if valuePrefix == "" {
		valuePrefix = "testkey"
	}

	tb := d.Table(testTable)
	for i := start; i < end; i++ {
		key := fmt.Sprintf("foo%d", i)
		if err := tb.Put(ctx, key, valuePrefix+syncbaseName+key); err != nil {
			return fmt.Errorf("tb.Put() failed: %v", err)
		}
	}
	return nil
}

func updateData(ctx *context.T, syncbaseName string, start, end int, valuePrefix string) error {
	a := syncbase.NewService(syncbaseName).App(testApp)
	d := a.NoSQLDatabase(testDb, nil)
	return updateDataImpl(ctx, d, syncbaseName, start, end, valuePrefix)
}

// Signal key tells the module to send an end signal, using the signalKey
// provided as the key, once the module finishes its execution. This is useful
// if the module is run as a goroutine and the parent needs to wait for it to
// end.
func updateDataInBatch(ctx *context.T, syncbaseName string, start, end int, valuePrefix, signalKey string) error {
	a := syncbase.NewService(syncbaseName).App(testApp)
	d := a.NoSQLDatabase(testDb, nil)
	batch, err := d.BeginBatch(ctx, wire.BatchOptions{})
	if err != nil {
		return fmt.Errorf("BeginBatch failed: %v", err)
	}
	if err = updateDataImpl(ctx, batch, syncbaseName, start, end, valuePrefix); err != nil {
		return err
	}
	if err = batch.Commit(ctx); err != nil {
		return fmt.Errorf("Commit failed: %v", err)
	}
	if signalKey != "" {
		return sendSignal(ctx, d, signalKey)
	}
	return nil
}

// TODO(ivanpi): Remove sendSignal now that all functions using it are in the
// same process.
func sendSignal(ctx *context.T, d nosql.Database, signalKey string) error {
	tb := d.Table(testTable)
	r := tb.Row(signalKey)

	if err := r.Put(ctx, true); err != nil {
		return fmt.Errorf("r.Put() failed: %v", err)
	}
	return nil
}

////////////////////////////////////////////////////////////
// Helpers for verifying data

const skipScan = true

func verifySyncgroupData(ctx *context.T, syncbaseName, keyPrefix string, start, count int) error {
	a := syncbase.NewService(syncbaseName).App(testApp)
	d := a.NoSQLDatabase(testDb, nil)
	tb := d.Table(testTable)

	// Wait a bit (up to 10 seconds) for the last key to appear.
	lastKey := fmt.Sprintf("%s%d", keyPrefix, start+count-1)
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		var value string
		if err := tb.Get(ctx, lastKey, &value); err == nil {
			break
		}
	}

	// Verify that all keys and values made it over correctly.
	for i := start; i < start+count; i++ {
		key := fmt.Sprintf("%s%d", keyPrefix, i)
		var got string
		if err := tb.Get(ctx, key, &got); err != nil {
			return fmt.Errorf("tb.Get() failed: %v", err)
		}
		want := "testkey" + key
		if got != want {
			return fmt.Errorf("unexpected value: got %q, want %q", got, want)
		}
	}

	// TODO(sadovsky): Drop this? (Does it buy us much?)
	if !skipScan {
		// Re-verify using a scan operation.
		stream := tb.Scan(ctx, nosql.Prefix(keyPrefix))
		for i := 0; stream.Advance(); i++ {
			got, want := stream.Key(), fmt.Sprintf("%s%d", keyPrefix, i)
			if got != want {
				return fmt.Errorf("unexpected key in scan: got %q, want %q", got, want)
			}
			want = "testkey" + want
			if err := stream.Value(&got); err != nil {
				return fmt.Errorf("failed to fetch value in scan: %v", err)
			}
			if got != want {
				return fmt.Errorf("unexpected value in scan: got %q, want %q", got, want)
			}
		}
		if err := stream.Err(); err != nil {
			return fmt.Errorf("scan stream error: %v", err)
		}
	}
	return nil
}

////////////////////////////////////////////////////////////
// Helpers for managing syncgroups

// blessingPatterns is a ";"-separated list of blessing patterns.
func createSyncgroup(ctx *context.T, syncbaseName, sgName, sgPrefixes, mtName, blessingPatterns string, perms access.Permissions) error {
	if mtName == "" {
		roots := v23.GetNamespace(ctx).Roots()
		if len(roots) == 0 {
			return errors.New("no namespace roots")
		}
		mtName = roots[0]
	}

	a := syncbase.NewService(syncbaseName).App(testApp)
	d := a.NoSQLDatabase(testDb, nil)

	if perms == nil {
		perms = tu.DefaultPerms(strings.Split(blessingPatterns, ";")...)
	}

	spec := wire.SyncgroupSpec{
		Description: "test syncgroup sg",
		Perms:       perms,
		Prefixes:    parseSgPrefixes(sgPrefixes),
		MountTables: []string{mtName},
	}

	sg := d.Syncgroup(sgName)
	info := wire.SyncgroupMemberInfo{SyncPriority: 8}
	if err := sg.Create(ctx, spec, info); err != nil {
		return fmt.Errorf("{%q, %q} sg.Create() failed: %v", syncbaseName, sgName, err)
	}
	return nil
}

func joinSyncgroup(ctx *context.T, syncbaseName, sgName string) error {
	a := syncbase.NewService(syncbaseName).App(testApp)
	d := a.NoSQLDatabase(testDb, nil)

	sg := d.Syncgroup(sgName)
	info := wire.SyncgroupMemberInfo{SyncPriority: 10}
	if _, err := sg.Join(ctx, info); err != nil {
		return fmt.Errorf("{%q, %q} sg.Join() failed: %v", syncbaseName, sgName, err)
	}
	return nil
}

func verifySyncgroupMembers(ctx *context.T, syncbaseName, sgName string, wantMembers int) error {
	a := syncbase.NewService(syncbaseName).App(testApp)
	d := a.NoSQLDatabase(testDb, nil)
	sg := d.Syncgroup(sgName)

	var gotMembers int
	for i := 0; i < 8; i++ {
		time.Sleep(500 * time.Millisecond)
		members, err := sg.GetMembers(ctx)
		if err != nil {
			return fmt.Errorf("{%q, %q} sg.GetMembers() failed: %v", syncbaseName, sgName, err)
		}
		gotMembers = len(members)
		if gotMembers == wantMembers {
			break
		}
	}
	if gotMembers != wantMembers {
		return fmt.Errorf("{%q, %q} verifySyncgroupMembers failed: got %d members, want %d", syncbaseName, sgName, gotMembers, wantMembers)
	}
	return nil
}

func pauseSync(ctx *context.T, syncbaseName string) error {
	return syncbase.NewService(syncbaseName).App(testApp).NoSQLDatabase(testDb, nil).PauseSync(ctx)
}

func resumeSync(ctx *context.T, syncbaseName string) error {
	return syncbase.NewService(syncbaseName).App(testApp).NoSQLDatabase(testDb, nil).ResumeSync(ctx)
}

////////////////////////////////////////////////////////////
// Syncbase-specific testing helpers

// parseSgPrefixes converts, for example, "a:b,c:" to
// [{TableName: testApp, Row: "b"}, {TableName: "c", Row: ""}].
func parseSgPrefixes(csv string) []wire.TableRow {
	strs := strings.Split(csv, ",")
	res := make([]wire.TableRow, len(strs))
	for i, v := range strs {
		parts := strings.SplitN(v, ":", 2)
		if len(parts) != 2 {
			panic(fmt.Sprintf("invalid prefix string: %q", v))
		}
		res[i] = wire.TableRow{TableName: parts[0], Row: parts[1]}
	}
	return res
}

////////////////////////////////////////////////////////////
// Helpers to interact with the Syncbase service directly.

// Obtain the Syncbase client stub.
func sc(name string) wire_syncbase.ServiceClientStub {
	return wire_syncbase.ServiceClient(name)
}

////////////////////////////////////////////////////////////
// Generic testing helpers

func ok(t testing.TB, err error) {
	if err != nil {
		tu.Fatal(t, err)
	}
}

func nok(t testing.TB, err error) {
	if err == nil {
		tu.Fatal(t, "nil err")
	}
}

func eq(t testing.TB, got, want interface{}) {
	if !reflect.DeepEqual(got, want) {
		tu.Fatalf(t, "got %v, want %v", got, want)
	}
}

func neq(t testing.TB, got, notWant interface{}) {
	if reflect.DeepEqual(got, notWant) {
		tu.Fatalf(t, "got %v", got)
	}
}
