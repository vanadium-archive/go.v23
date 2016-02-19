// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"v.io/v23/naming"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/nosql"
	constants "v.io/x/ref/services/syncbase/server/util"
	"v.io/x/ref/test/v23test"
)

const pingPongPairIterations = 500

// BenchmarkPingPongPair measures the round trip sync latency between a pair of
// Syncbase instances that Ping Pong data to each other over a syncgroup.
//
// This benchmark performs the following operations:
// - Create two syncbase instances and have them join the same syncgroup.
// - Each watches the other syncbase's "section" of the table.
// - A preliminary write by each syncbase ensures that clocks are synced.
// - During Ping Pong, each syncbase instance writes data to its own section of
//   the table. The other syncbase watches that section for writes. Once a write
//   is received, it does the same.
//
// After the benchmark completes, the "ns/op" value refers to the average time
// for |pingPongPairIterations| of Ping Pong roundtrips completed.
func BenchmarkPingPongPair(b *testing.B) {
	sh := v23test.NewShell(b, v23test.Opts{})
	defer sh.Cleanup()
	sh.StartRootMountTable()

	b.ResetTimer()
	b.StopTimer()

	for iter := 0; iter < b.N; iter++ {
		// Setup 2 Syncbases.
		sbs := setupSyncbases(b, sh, 2)

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

		// The join has succeeded, so make sure sync is initialized.
		// The strategy is: s0 sends to s1, and then s1 responds.
		sendInt32Sync(b, sbs[0], prefix0, w1)
		sendInt32Sync(b, sbs[1], prefix1, w0)

		// Sync is now active, so it is time to really start our watch streams.
		c0, c1 := make(chan int32), make(chan int32)
		go watchInt32s(b, w0, c0)
		go watchInt32s(b, w1, c1)

		// Perform and time |pingPongIterations| of ping pong between syncbases.
		b.StartTimer()
		lastTime := time.Now()
		var t0to1, t1to0 int64
		for i := 0; i < pingPongPairIterations; i++ {
			var delta int64
			value := int32(i)
			ok(b, writeInt32(sbs[0], prefix0, value))
			<-c1
			delta, lastTime = getDelta(lastTime)
			t0to1 += delta
			ok(b, writeInt32(sbs[1], prefix1, value))
			<-c0
			delta, lastTime = getDelta(lastTime)
			t1to0 += delta
		}
		b.StopTimer()

		// Clean up syncbases.
		for i, _ := range sbs {
			sbs[i].cleanup(os.Interrupt)
		}

		// Print out intermediate information.
		fmt.Printf("Iteration %d of %d\n", iter, b.N)
		fmt.Printf("Avg Time from 0 to 1: %d ns\n", t0to1/int64(pingPongPairIterations))
		fmt.Printf("Avg Time from 1 to 0: %d ns\n", t1to0/int64(pingPongPairIterations))
		fmt.Printf("Avg Time per iteration: %d ns\n", (t1to0+t0to1)/int64(pingPongPairIterations))
	}
}

// getDelta computes the delta between the last time and now.
// It returns both that delta and the new time.
func getDelta(lastTime time.Time) (delta int64, nextTime time.Time) {
	nextTime = time.Now()
	delta = nextTime.Sub(lastTime).Nanoseconds()
	return
}

// sendInt32Sync sends data from 1 syncbase to another.
// Be sure that the receiving watch stream sees the data and is still ok.
func sendInt32Sync(b *testing.B, ts *testSyncbase, senderPrefix string, w nosql.WatchStream) {
	ok(b, writeInt32(ts, senderPrefix, -1))
	if w.Advance() {
		w.Change() // grab the change, but ignore the value.
	}
	watchStreamOk(b, w)
}

// watchInt32s sends the value of each put through to the channel.
func watchInt32s(b *testing.B, w nosql.WatchStream, c chan int32) {
	var count int
	for count < pingPongPairIterations && w.Advance() {
		var t int32
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
		count++
	}

	w.Cancel() // The stream can be canceled since we've seen enough iterations.
}

// getDbAndTable obtains the database and table handles for a syncbase name.
func getDbAndTable(syncbaseName string) (d nosql.Database, tb nosql.Table) {
	a := syncbase.NewService(syncbaseName).App(testApp)
	d = a.NoSQLDatabase(testDb, nil)
	tb = d.Table(testTable)
	return
}

// writeInt32 writes keyValue into syncbase at keyPrefix/keyValue.
func writeInt32(ts *testSyncbase, keyPrefix string, keyValue int32) error {
	ctx := ts.clientCtx
	syncbaseName := ts.sbName
	_, tb := getDbAndTable(syncbaseName)

	key := fmt.Sprintf("%s/%d", keyPrefix, keyValue)

	if err := tb.Put(ctx, key, keyValue); err != nil {
		return fmt.Errorf("tb.Put() failed: %v", err)
	}
	return nil
}

// watchStreamOk emits an error if the watch stream has an error.
func watchStreamOk(b *testing.B, w nosql.WatchStream) {
	if w.Err() != nil {
		b.Errorf("stream error: %v", w.Err())
	}
}
