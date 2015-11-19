// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"fmt"
	"strconv"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	wire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/nosql"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/syncbase/server/util"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test/modules"
	"v.io/x/ref/test/v23tests"
)

func V23TestDefaultCR(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	server0Creds, _ := t.Shell().NewChildCredentials("s0")
	client0Creds, _ := t.Shell().NewChildCredentials("c0")
	cleanSync0 := tu.StartSyncbased(t, server0Creds, "sync0", "",
		`{"Read": {"In":["root/c0"]}, "Write": {"In":["root/c0"]}}`)
	defer cleanSync0()

	server1Creds, _ := t.Shell().NewChildCredentials("s1")
	client1Creds, _ := t.Shell().NewChildCredentials("c1")
	cleanSync1 := tu.StartSyncbased(t, server1Creds, "sync1", "",
		`{"Read": {"In":["root/c1"]}, "Write": {"In":["root/c1"]}}`)
	defer cleanSync1()

	sgName := naming.Join("sync0", util.SyncbaseSuffix, "SG1")

	// Setup database for App on sync0, create a syncgroup with sync0 and sync1
	// and populate some initial data.
	tu.RunClient(t, client0Creds, runSetupAppA, "sync0")
	tu.RunClient(t, client0Creds, runCreateSyncgroup, "sync0", sgName, "tb:foo", "", "root/s0", "root/s1")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foo", "0", "1")

	// Setup database for App on sync1, join the syncgroup created above and
	// verify if the initial data was synced or not.
	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncgroup, "sync1", sgName)
	tu.RunClient(t, client1Creds, runVerifySyncgroupData, "sync1", "foo", "0", "1", "true")

	// Turn off syncing on both s0 and s1 by removing each other from syncgroup ACLs.
	tu.RunClient(t, client0Creds, runToggleSync, "sync0", sgName, "root/s0")
	tu.RunClient(t, client1Creds, runToggleSync, "sync1", sgName, "root/s1")

	// Since sync is paused, the following updates are concurrent.
	tu.RunClient(t, client0Creds, runUpdateData, "sync0", "0", "1", "concurrentUpdate")
	tu.RunClient(t, client1Creds, runUpdateData, "sync1", "0", "1", "concurrentUpdate")

	// Re enable sync between the two syncbases and wait for a bit to let the
	// syncbases sync and call conflict resolution.
	tu.RunClient(t, client0Creds, runToggleSync, "sync0", sgName, "root/s0", "root/s1")
	tu.RunClient(t, client1Creds, runToggleSync, "sync1", sgName, "root/s0", "root/s1")

	// Verify that the resolved data looks correct.
	tu.RunClient(t, client0Creds, runWaitForValue, "sync0", "foo0", "concurrentUpdate"+"sync1")
	tu.RunClient(t, client1Creds, runWaitForValue, "sync1", "foo0", "concurrentUpdate"+"sync1")
}

// V23TestSyncbasedSyncWithAppResolvedConflicts tests AppResolves resolution
// policy by creating conflicts for rows that will be resolved by the
// application. This test covers the following scenerios:
// 1) 5 independent rows under conflict resulting into 5 conflict resolution
//    calls to the app.
// 2) 5 rows written as a single batch on both syncbases resulting into a
//    single conflict for the batch.
func V23TestSyncbasedSyncWithAppResolvedConflicts(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	server0Creds, _ := t.Shell().NewChildCredentials("s0")
	client0Creds, _ := t.Shell().NewChildCredentials("c0")
	cleanSync0 := tu.StartSyncbased(t, server0Creds, "sync0", "",
		`{"Read": {"In":["root/c0"]}, "Write": {"In":["root/c0"]}}`)
	defer cleanSync0()

	server1Creds, _ := t.Shell().NewChildCredentials("s1")
	client1Creds, _ := t.Shell().NewChildCredentials("c1")
	cleanSync1 := tu.StartSyncbased(t, server1Creds, "sync1", "",
		`{"Read": {"In":["root/c1"]}, "Write": {"In":["root/c1"]}}`)
	defer cleanSync1()

	sgName := naming.Join("sync0", util.SyncbaseSuffix, "SG1")

	// Setup database for App on sync0, create a syncgroup with sync0 and sync1
	// and populate some initial data.
	tu.RunClient(t, client0Creds, runSetupAppA, "sync0")
	tu.RunClient(t, client0Creds, runCreateSyncgroup, "sync0", sgName, "tb:foo", "", "root/s0", "root/s1")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foo", "0")

	// Setup database for App on sync1, join the syncgroup created above and
	// verify if the initial data was synced or not.
	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncgroup, "sync1", sgName)
	tu.RunClient(t, client1Creds, runVerifySyncgroupData, "sync1", "foo", "0", "10", "false")

	// Turn off syncing on both s0 and s1 by removing each other from syncgroup ACLs.
	tu.RunClient(t, client0Creds, runToggleSync, "sync0", sgName, "root/s0")
	tu.RunClient(t, client1Creds, runToggleSync, "sync1", sgName, "root/s1")

	// Since sync is paused, the following updates are concurrent.
	tu.RunClient(t, client0Creds, runUpdateData, "sync0", "0", "5", "concurrentUpdate")
	tu.RunClient(t, client1Creds, runUpdateData, "sync1", "0", "5", "concurrentUpdate")

	tu.RunClient(t, client0Creds, runUpdateBatchData, "sync0", "5", "10", "concurrentBatchUpdate")
	tu.RunClient(t, client1Creds, runUpdateBatchData, "sync1", "5", "10", "concurrentBatchUpdate")

	// Create and hold a conflict resolution connection on sync0 and sync 1 to receive
	// future conflicts. The expected call count is 2 * the number of batches
	// because each batch is being concurrently resolved on sync0 and sync1
	// creating new values on each side. Later when the next round of sync
	// happens these new values cause another conflict. Since the conflict
	// resolver does not create new value for a duplicate conflict, no more
	// conflict pingpongs happen.
	// TODO(jlodhia): change the expected num conflicts from 12 to 6 once
	// sync's cr code handles duplicate resolutions internally.
	go tu.RunClient(t, client0Creds, runConflictResolver, "sync0", "foo", "endKey", "12")
	go tu.RunClient(t, client1Creds, runConflictResolver, "sync1", "foo", "endKey", "12")

	time.Sleep(1 * time.Millisecond) // let the above go routine get scheduled.

	// Re enable sync between the two syncbases and wait for a bit to let the
	// syncbases sync and call conflict resolution.
	tu.RunClient(t, client0Creds, runToggleSync, "sync0", sgName, "root/s0", "root/s1")
	tu.RunClient(t, client1Creds, runToggleSync, "sync1", sgName, "root/s0", "root/s1")

	// Verify that the resolved data looks correct.
	keyUnderConflict := "foo8" // one of the keys under conflict
	tu.RunClient(t, client0Creds, runWaitForValue, "sync0", keyUnderConflict, "AppResolvedVal", "foo")
	tu.RunClient(t, client0Creds, runVerifyConflictResolvedData, "sync0", "foo", "0", "5", "AppResolvedVal")
	tu.RunClient(t, client0Creds, runVerifyConflictResolvedData, "sync0", "foo", "5", "10", "AppResolvedVal")

	tu.RunClient(t, client1Creds, runWaitForValue, "sync1", keyUnderConflict, "AppResolvedVal", "foo")
	tu.RunClient(t, client1Creds, runVerifyConflictResolvedData, "sync1", "foo", "0", "5", "AppResolvedVal")
	tu.RunClient(t, client1Creds, runVerifyConflictResolvedData, "sync1", "foo", "5", "10", "AppResolvedVal")

	// runEndTest signals conflict resolution thread to exit.
	tu.RunClient(t, client0Creds, runEndTest, "sync0", "foo", "endKey")
	tu.RunClient(t, client1Creds, runEndTest, "sync1", "foo", "endKey")

	// wait for conflict resolution thread to exit
	tu.RunClient(t, client0Creds, runWaitSignal, "sync0", "foo", "endKeyAck")
	tu.RunClient(t, client1Creds, runWaitSignal, "sync1", "foo", "endKeyAck")
}

//////////////////////////////////////////////
// Helpers specific to ConflictResolution

// Arguments: 0: Syncbase name, 1: conflict prefix, 2: signalKey, 3: max onConflict call count.
var runConflictResolver = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	serviceName, prefix, signalKey, maxCallCountStr := args[0], args[1], args[2], args[3]
	maxCallCount, _ := strconv.ParseUint(maxCallCountStr, 10, 64)

	a := syncbase.NewService(serviceName).App("a")
	resolver := &CRImpl{serviceName: serviceName}
	d := a.NoSQLDatabase("d", makeSchema(prefix, resolver))
	defer d.Close()
	d.EnforceSchema(ctx)

	// Wait till end of test is signalled. The above statement starts a go
	// routine with a cr connection to the server which needs to stay alive
	// till the life of the test in order to receive conflicts.
	if err := waitSignal(ctx, d, signalKey); err != nil {
		return err
	}
	if err := sendSignal(ctx, d, signalKey+"Ack"); err != nil {
		return err
	}

	// Check that the onConflict() was called exactly as many times as was
	// expected.
	if resolver.onConflictCallCount > maxCallCount {
		return fmt.Errorf("Unexpected OnConflict call count. Max: %d, Actual: %d\n", maxCallCount, resolver.onConflictCallCount)
	}
	return nil
}, "runConflictResolver")

// Arguments: 0: Syncbase name, 1: schema prefix, 2: start index, 3: end index (not included), 4: valuePrefix.
var runVerifyConflictResolvedData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	serviceName, prefix, startStr, endStr, valuePrefix := args[0], args[1], args[2], args[3], args[4]
	start, _ := strconv.ParseUint(startStr, 10, 64)
	end, _ := strconv.ParseUint(endStr, 10, 64)

	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", makeSchema(prefix, &CRImpl{serviceName: serviceName}))

	tb := d.Table(testTable)
	for i := start; i < end; i++ {
		var got string
		key := fmt.Sprintf("foo%d", i)
		r := tb.Row(key)
		if err := r.Get(ctx, &got); err != nil {
			return fmt.Errorf("r.Get() failed: %v\n", err)
		}
		if got != valuePrefix+key {
			return fmt.Errorf("unexpected value: got %v, want %v\n", got, valuePrefix)
		}
	}
	return nil
}, "runVerifyConflictResolvedData")

// Arguments: 0: Syncbase name, 1: key, 2: valuePrefix.
// Optional Args: 3: schema prefix.
var runWaitForValue = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	var schema *nosql.Schema = nil
	serviceName, key, valuePrefix := args[0], args[1], args[2]
	if len(args) > 3 {
		schema = makeSchema(args[3], &CRImpl{serviceName: serviceName})
	}

	a := syncbase.NewService(serviceName).App("a")
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
}, "runWaitForValue")

// Arguments: 0: Syncbase name, 1: conflict prefix, 2: signalKey.
var runEndTest = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	serviceName, prefix, signalKey := args[0], args[1], args[2]

	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", makeSchema(prefix, &CRImpl{serviceName: serviceName}))

	// signal end of test so that conflict resolution can clean up its stream.
	return sendSignal(ctx, d, signalKey)
}, "runEndTest")

// Arguments: 0: Syncbase name, 1: conflict prefix, 2: signalKey.
var runWaitSignal = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	serviceName, prefix, signalKey := args[0], args[1], args[2]

	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", makeSchema(prefix, &CRImpl{serviceName: serviceName}))

	// wait for signal.
	return waitSignal(ctx, d, signalKey)
}, "runEndTest")

func waitSignal(ctx *context.T, d nosql.Database, signalKey string) error {
	tb := d.Table(testTable)
	r := tb.Row(signalKey)

	var end bool
	sleepTimeMs, maxAttempts := 50, 100 // Max wait time of 5 seconds.
	for cnt := 0; cnt < maxAttempts; cnt++ {
		time.Sleep(time.Duration(sleepTimeMs) * time.Millisecond)
		if err := r.Get(ctx, &end); err != nil {
			if verror.ErrorID(err) != verror.ErrNoExist.ID {
				return fmt.Errorf("r.Get() for endkey failed: %v\n", err)
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
		return fmt.Errorf("r.Put() failed: %v\n", err)
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

// Client conflict reolution impl.
type CRImpl struct {
	serviceName         string
	onConflictCallCount uint64
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
