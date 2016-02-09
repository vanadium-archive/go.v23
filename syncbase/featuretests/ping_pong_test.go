// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"fmt"
	"testing"
	"time"

	"v.io/v23/naming"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/nosql"
	constants "v.io/x/ref/services/syncbase/server/util"
	"v.io/x/ref/test/v23test"
)

// BenchmarkPingPongPair measures the round trip sync latency between a pair of
// Syncbase instances that Ping Pong data to each other over a syncgroup.
//
// This benchmark performs the following operations:
// - Create two syncbase instances and have them join the same syncgroup.
// - Each watches the other syncbase's "section" of the table.
// - A preliminary write by each syncbase ensures that clocks are synced.
// - During Ping Pong, each syncbase instance finds its local dev time and
//   writes it to its section of the table. The other syncbase watches for
//   this value; once received, it does the same.
//
// After the benchmark completes, the "ns/op" value refers to the average time
// per Ping Pong roundtrip completed by the two syncbases.
//
// Note: This benchmark can write simpler values (like int32) or use time.Now()
// instead of using a DevModeGetTime RPC. This can affect the benchmark stats.
func BenchmarkPingPongPair(b *testing.B) {
	sh := v23test.NewShell(b, v23test.Opts{})
	defer sh.Cleanup()
	sh.StartRootMountTable()

	// Setup 2 Syncbases with the dev mode clock on.
	sbs := setupSyncbases(b, sh, 2, "--dev")

	// Syncbase s0 is the creator.
	sgName := naming.Join(sbs[0].sbName, constants.SyncbaseSuffix, "SG1")

	// TODO(alexfandrianto): Was unable to use the empty prefix ("tb:").
	// Observation: w0's watch isn't working with the empty prefix.
	// Possible Explanation: The empty prefix ACL receives an initial value from
	// the Table ACL. If this value is synced over from the opposing peer,
	// conflict resolution can mean that s0 loses the ability to watch.
	syncString := fmt.Sprintf("%s:p", testTable)
	ok(b, createSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sgName, syncString, "", sbBlessings(sbs), nil))

	// Syncbase s1 will attempt to join the syncgroup.
	ok(b, joinSyncgroup(sbs[1].clientCtx, sbs[1].sbName, sgName))

	// Obtain the handles to the databases.
	db0, _ := getDbAndTable(sbs[0].sbName)
	db1, _ := getDbAndTable(sbs[1].sbName)

	// Set up the watch streams (watching the other syncbase's prefix).
	prefix0, prefix1 := "prefix0", "prefix1"
	w0, err := db0.Watch(sbs[0].clientCtx, testTable, prefix1, watch.ResumeMarker("now"))
	ok(b, err)
	w1, err := db1.Watch(sbs[1].clientCtx, testTable, prefix0, watch.ResumeMarker("now"))
	ok(b, err)

	// The join has succeeded, so it's time to ensure clocks are synchronized.
	// The strategy is: s0 sends to s1, and then s1 responds.
	sendTimeSync(b, sbs[0], prefix0, w1)
	sendTimeSync(b, sbs[1], prefix1, w0)

	// The clocks are synchronized, so it is now time to really start our watch
	// streams. This will allow us to ping and pong.
	c0, c1 := make(chan time.Time), make(chan time.Time)
	go watchTimes(b, w0, c0)
	go watchTimes(b, w1, c1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		suffix := fmt.Sprintf("%d", i)
		ok(b, writeTime(sbs[0], prefix0, suffix))
		<-c1
		ok(b, writeTime(sbs[1], prefix1, suffix))
		<-c0
	}

	// TODO(alexfandrianto): Should cancel these watch streams. Unfortunately, we
	// cannot cancel while a watch stream's Advance() blocks.
	//w0.Cancel()
	//w1.Cancel()
}

// sendTimeSync sends data from 1 syncbase to another.
// Be sure that the receiving watch stream sees the data and is still ok.
func sendTimeSync(b *testing.B, ts *testSyncbase, senderPrefix string, w nosql.WatchStream) {
	ok(b, writeTime(ts, senderPrefix, "synctime"))
	if w.Advance() {
		w.Change() // grab the change, but ignore the value.
	}
	watchStreamOk(b, w)
}

// watchTimes sends the value of each put through to the channel.
func watchTimes(b *testing.B, w nosql.WatchStream, c chan time.Time) {
	for w.Advance() {
		var t time.Time
		change := w.Change()
		if change.ChangeType == nosql.DeleteChange {
			b.Error("Received a delete change")
		}
		err := change.Value(&t)
		if err != nil {
			b.Error(err)
		}
		c <- t
		watchStreamOk(b, w)
	}
}

// getDbAndTable obtains the database and table handles for a syncbase name.
func getDbAndTable(syncbaseName string) (d nosql.Database, tb nosql.Table) {
	a := syncbase.NewService(syncbaseName).App(testApp)
	d = a.NoSQLDatabase(testDb, nil)
	tb = d.Table(testTable)
	return
}

// writeTime writes a timestamp into syncbase at the combination of keyPrefix
// and keySuffix.
func writeTime(ts *testSyncbase, keyPrefix, keySuffix string) error {
	ctx := ts.clientCtx
	syncbaseName := ts.sbName
	_, tb := getDbAndTable(syncbaseName)

	key := fmt.Sprintf("%s/%s", keyPrefix, keySuffix)
	// sc is a helper to obtain the syncbase wire service.
	time, err := sc(syncbaseName).DevModeGetTime(ctx)
	if err != nil {
		return fmt.Errorf("sb.DevModeGetTime() failed: %v", err)
	}

	if err2 := tb.Put(ctx, key, time); err2 != nil {
		return fmt.Errorf("tb.Put() failed: %v", err2)
	}
	return nil
}

// watchStreamOk emits an error if the watch stream has an error.
func watchStreamOk(b *testing.B, w nosql.WatchStream) {
	if w.Err() != nil {
		b.Errorf("stream error: %v", w.Err())
	}
}
