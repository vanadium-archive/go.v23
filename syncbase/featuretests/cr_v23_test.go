// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"fmt"
	"time"

	"v.io/v23/context"
	"v.io/v23/naming"
	wire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/nosql"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/syncbase/server/util"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test/v23tests"
)

//go:generate jiri test generate

func V23TestCRDefault(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")

	server0Creds := forkCredentials(t, "s0")
	client0Ctx := forkContext(t, "c0")
	cleanup0 := tu.StartSyncbased(t, server0Creds, "sync0", "",
		`{"Read": {"In":["root:c0"]}, "Write": {"In":["root:c0"]}}`)
	defer cleanup0()

	server1Creds := forkCredentials(t, "s1")
	client1Ctx := forkContext(t, "c1")
	cleanup1 := tu.StartSyncbased(t, server1Creds, "sync1", "",
		`{"Read": {"In":["root:c1"]}, "Write": {"In":["root:c1"]}}`)
	defer cleanup1()

	sgName := naming.Join("sync0", util.SyncbaseSuffix, "SG1")

	// Setup database for App on sync0, create a syncgroup with sync0 and sync1
	// and populate some initial data.
	ok(t, setupAppA(client0Ctx, "sync0"))
	ok(t, createSyncgroup(client0Ctx, "sync0", sgName, "tb:foo", "", "root:s0;root:s1"))
	ok(t, populateData(client0Ctx, "sync0", "foo", 0, 1))

	// Setup database for App on sync1, join the syncgroup created above and
	// verify if the initial data was synced or not.
	ok(t, setupAppA(client1Ctx, "sync1"))
	ok(t, joinSyncgroup(client1Ctx, "sync1", sgName))
	ok(t, verifySyncgroupData(client1Ctx, "sync1", "foo", 0, 1))

	// Turn off syncing on both s0 and s1 by removing each other from syncgroup
	// ACLs.
	ok(t, toggleSync(client0Ctx, "sync0", sgName, "root:s0"))
	ok(t, toggleSync(client1Ctx, "sync1", sgName, "root:s1"))

	// Since sync is paused, the following updates are concurrent.
	ok(t, updateData(client0Ctx, "sync0", 0, 1, "concurrentUpdate"))
	ok(t, updateData(client1Ctx, "sync1", 0, 1, "concurrentUpdate"))

	// Re enable sync between the two syncbases and wait for a bit to let the
	// syncbases sync and call conflict resolution.
	ok(t, toggleSync(client0Ctx, "sync0", sgName, "root:s0;root:s1"))
	ok(t, toggleSync(client1Ctx, "sync1", sgName, "root:s0;root:s1"))

	// Verify that the resolved data looks correct.
	ok(t, waitForValue(client0Ctx, "sync0", "foo0", "concurrentUpdate"+"sync1", ""))
	ok(t, waitForValue(client1Ctx, "sync1", "foo0", "concurrentUpdate"+"sync1", ""))
}

// TestCRAppResolved tests AppResolves resolution policy by creating conflicts
// for rows that will be resolved by the application. This test covers the
// following scenerios:
// 1) 5 independent rows under conflict resulting into 5 conflict resolution
//    calls to the app.
// 2) 5 rows written as a single batch on both syncbases resulting into a
//    single conflict for the batch.
func V23TestCRAppResolved(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")

	server0Creds := forkCredentials(t, "s0")
	client0Ctx := forkContext(t, "c0")
	cleanup0 := tu.StartSyncbased(t, server0Creds, "sync0", "",
		`{"Read": {"In":["root:c0"]}, "Write": {"In":["root:c0"]}}`)
	defer cleanup0()

	server1Creds := forkCredentials(t, "s1")
	client1Ctx := forkContext(t, "c1")
	cleanup1 := tu.StartSyncbased(t, server1Creds, "sync1", "",
		`{"Read": {"In":["root:c1"]}, "Write": {"In":["root:c1"]}}`)
	defer cleanup1()

	sgName := naming.Join("sync0", util.SyncbaseSuffix, "SG1")

	// Setup database for App on sync0, create a syncgroup with sync0 and sync1
	// and populate some initial data.
	ok(t, setupAppA(client0Ctx, "sync0"))
	ok(t, createSyncgroup(client0Ctx, "sync0", sgName, "tb:foo", "", "root:s0;root:s1"))
	ok(t, populateData(client0Ctx, "sync0", "foo", 0, 10))

	// Setup database for App on sync1, join the syncgroup created above and
	// verify if the initial data was synced or not.
	ok(t, setupAppA(client1Ctx, "sync1"))
	ok(t, joinSyncgroup(client1Ctx, "sync1", sgName))
	ok(t, verifySyncgroupData(client1Ctx, "sync1", "foo", 0, 10))

	// Turn off syncing on both s0 and s1 by removing each other from syncgroup
	// ACLs.
	ok(t, toggleSync(client0Ctx, "sync0", sgName, "root:s0"))
	ok(t, toggleSync(client1Ctx, "sync1", sgName, "root:s1"))

	// Since sync is paused, the following updates are concurrent.
	ok(t, updateData(client0Ctx, "sync0", 0, 5, "concurrentUpdate"))
	ok(t, updateData(client1Ctx, "sync1", 0, 5, "concurrentUpdate"))

	ok(t, updateDataInBatch(client0Ctx, "sync0", 5, 10, "concurrentBatchUpdate"))
	ok(t, updateDataInBatch(client1Ctx, "sync1", 5, 10, "concurrentBatchUpdate"))

	// Create and hold a conflict resolution connection on sync0 and sync1 to receive
	// future conflicts. The expected call count is 2 * the number of batches
	// because each batch is being concurrently resolved on sync0 and sync1
	// creating new values on each side. Later when the next round of sync
	// happens these new values cause another conflict. Since the conflict
	// resolver does not create new value for a duplicate conflict, no more
	// conflict pingpongs happen.
	// TODO(jlodhia): change the expected num conflicts from 12 to 6 once
	// sync's cr code handles duplicate resolutions internally.
	go func() {
		ok(t, runConflictResolver(client0Ctx, "sync0", "foo", "endKey", 12))
	}()
	go func() {
		ok(t, runConflictResolver(client1Ctx, "sync1", "foo", "endKey", 12))
	}()

	time.Sleep(1 * time.Millisecond) // let the above goroutines start up

	// Re enable sync between the two syncbases and wait for a bit to let the
	// syncbases sync and call conflict resolution.
	ok(t, toggleSync(client0Ctx, "sync0", sgName, "root:s0;root:s1"))
	ok(t, toggleSync(client1Ctx, "sync1", sgName, "root:s0;root:s1"))

	// Verify that the resolved data looks correct.
	keyUnderConflict := "foo8" // one of the keys under conflict
	ok(t, waitForValue(client0Ctx, "sync0", keyUnderConflict, "AppResolvedVal", "foo"))
	ok(t, verifyConflictResolvedData(client0Ctx, "sync0", "foo", 0, 5, "AppResolvedVal"))
	ok(t, verifyConflictResolvedData(client0Ctx, "sync0", "foo", 5, 10, "AppResolvedVal"))

	ok(t, waitForValue(client1Ctx, "sync1", keyUnderConflict, "AppResolvedVal", "foo"))
	ok(t, verifyConflictResolvedData(client1Ctx, "sync1", "foo", 0, 5, "AppResolvedVal"))
	ok(t, verifyConflictResolvedData(client1Ctx, "sync1", "foo", 5, 10, "AppResolvedVal"))

	// endTest signals conflict resolution thread to exit.
	// TODO(sadovsky): Use channels for signaling now that everything's in the
	// same process.
	ok(t, endTest(client0Ctx, "sync0", "foo", "endKey"))
	ok(t, endTest(client1Ctx, "sync1", "foo", "endKey"))

	// wait for conflict resolution thread to exit
	ok(t, waitForSignal(client0Ctx, "sync0", "foo", "endKeyAck"))
	ok(t, waitForSignal(client1Ctx, "sync1", "foo", "endKeyAck"))
}

//////////////////////////////////////////////
// Helpers specific to ConflictResolution

func runConflictResolver(ctx *context.T, syncbaseName, prefix, signalKey string, maxCallCount int) error {
	a := syncbase.NewService(syncbaseName).App("a")
	resolver := &CRImpl{syncbaseName: syncbaseName}
	d := a.NoSQLDatabase("d", makeSchema(prefix, resolver))
	defer d.Close()
	d.EnforceSchema(ctx)

	// Wait till end of test is signalled. The above statement starts a goroutine
	// with a cr connection to the server which needs to stay alive till the life
	// of the test in order to receive conflicts.
	if err := waitSignal(ctx, d, signalKey); err != nil {
		return err
	}

	// Check that OnConflict() was called at most 'maxCallCount' times.
	var onConflictErr error
	if resolver.onConflictCallCount > maxCallCount {
		onConflictErr = fmt.Errorf("Unexpected OnConflict call count. Max: %d, Actual: %d", maxCallCount, resolver.onConflictCallCount)
	}

	// Reply to the test with a signal to notify it that it may end.
	if err := sendSignal(ctx, d, signalKey+"Ack"); err != nil {
		return err
	}

	return onConflictErr
}

func verifyConflictResolvedData(ctx *context.T, syncbaseName, prefix string, start, end int, valuePrefix string) error {
	a := syncbase.NewService(syncbaseName).App("a")
	d := a.NoSQLDatabase("d", makeSchema(prefix, &CRImpl{syncbaseName: syncbaseName}))

	tb := d.Table(testTable)
	for i := start; i < end; i++ {
		var got string
		key := fmt.Sprintf("foo%d", i)
		r := tb.Row(key)
		if err := r.Get(ctx, &got); err != nil {
			return fmt.Errorf("r.Get() failed: %v", err)
		}
		if got != valuePrefix+key {
			return fmt.Errorf("unexpected value: got %v, want %v", got, valuePrefix)
		}
	}
	return nil
}

func waitForValue(ctx *context.T, syncbaseName, key, valuePrefix, schemaPrefix string) error {
	var schema *nosql.Schema
	if schemaPrefix != "" {
		schema = makeSchema(schemaPrefix, &CRImpl{syncbaseName: syncbaseName})
	}

	a := syncbase.NewService(syncbaseName).App("a")
	d := a.NoSQLDatabase("d", schema)

	tb := d.Table(testTable)
	r := tb.Row(key)
	want := valuePrefix + key

	// Wait upto 5 seconds for the correct key and value to appear.
	sleepTimeMs, maxAttempts := 50, 100
	var value string
	for i := 0; i < maxAttempts; i++ {
		if err := r.Get(ctx, &value); (err == nil) && (value == want) {
			return nil
		} else if err != nil {
			return fmt.Errorf("Syncbase Error while fetching key %v: %v", key, err)
		}
		time.Sleep(time.Duration(sleepTimeMs) * time.Millisecond)
	}
	return fmt.Errorf("Timed out waiting for value %v but found %v after %d milliseconds.", want, value, maxAttempts*sleepTimeMs)
}

func endTest(ctx *context.T, syncbaseName, prefix, signalKey string) error {
	a := syncbase.NewService(syncbaseName).App("a")
	d := a.NoSQLDatabase("d", makeSchema(prefix, &CRImpl{syncbaseName: syncbaseName}))

	// signal end of test so that conflict resolution can clean up its stream.
	return sendSignal(ctx, d, signalKey)
}

func waitForSignal(ctx *context.T, syncbaseName, prefix, signalKey string) error {
	a := syncbase.NewService(syncbaseName).App("a")
	d := a.NoSQLDatabase("d", makeSchema(prefix, &CRImpl{syncbaseName: syncbaseName}))

	// wait for signal.
	return waitSignal(ctx, d, signalKey)
}

func waitSignal(ctx *context.T, d nosql.Database, signalKey string) error {
	tb := d.Table(testTable)
	r := tb.Row(signalKey)

	var end bool
	sleepTimeMs, maxAttempts := 50, 100 // Max wait time of 5 seconds.
	for cnt := 0; cnt < maxAttempts; cnt++ {
		time.Sleep(time.Duration(sleepTimeMs) * time.Millisecond)
		if err := r.Get(ctx, &end); err != nil {
			if verror.ErrorID(err) != verror.ErrNoExist.ID {
				return fmt.Errorf("r.Get() for endkey failed: %v", err)
			}
		}
		if end {
			return nil
		}
	}
	return fmt.Errorf("Timed out waiting for signal %v after %d milliseconds.", signalKey, maxAttempts*sleepTimeMs)
}

func sendSignal(ctx *context.T, d nosql.Database, signalKey string) error {
	tb := d.Table(testTable)
	r := tb.Row(signalKey)

	if err := r.Put(ctx, true); err != nil {
		return fmt.Errorf("r.Put() failed: %v", err)
	}
	return nil
}

////////////////////////////////////////////////////////
// Conflict Resolution related code.

func makeSchema(keyPrefix string, resolver *CRImpl) *nosql.Schema {
	metadata := wire.SchemaMetadata{
		Version: 1,
		Policy: wire.CrPolicy{
			Rules: []wire.CrRule{
				wire.CrRule{
					TableName: testTable,
					KeyPrefix: keyPrefix,
					Resolver:  wire.ResolverTypeAppResolves,
				},
			},
		},
	}
	return &nosql.Schema{
		Metadata: metadata,
		Upgrader: nil,
		Resolver: resolver,
	}
}

// Client conflict resolution impl.
type CRImpl struct {
	syncbaseName        string
	onConflictCallCount int
}

func (ri *CRImpl) OnConflict(ctx *context.T, conflict *nosql.Conflict) nosql.Resolution {
	resolvedPrefix := "AppResolvedVal"
	ri.onConflictCallCount++
	res := nosql.Resolution{ResultSet: map[string]nosql.ResolvedRow{}}
	for rowKey, row := range conflict.WriteSet.ByKey {
		resolvedRow := nosql.ResolvedRow{}
		resolvedRow.Key = row.Key

		var localVal, remoteVal string
		row.LocalValue.Get(&localVal)
		row.RemoteValue.Get(&remoteVal)

		if localVal == remoteVal {
			if row.RemoteValue.WriteTs.After(row.LocalValue.WriteTs) {
				resolvedRow.Result = row.RemoteValue
			} else {
				resolvedRow.Result = row.LocalValue
			}
		} else {
			resolvedRow.Result, _ = nosql.NewValue(ctx, resolvedPrefix+keyPart(rowKey))
		}
		res.ResultSet[row.Key] = resolvedRow
	}
	return res
}

func keyPart(rowKey string) string {
	return util.SplitKeyParts(rowKey)[1]
}
