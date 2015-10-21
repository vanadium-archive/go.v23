// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

package nosql_test

import (
	"os"
	"testing"

	"v.io/x/ref/test"
	"v.io/x/ref/test/modules"
	"v.io/x/ref/test/v23tests"
)

func TestMain(m *testing.M) {
	test.Init()
	modules.DispatchAndExitIfChild()
	cleanup := v23tests.UseSharedBinDir()
	r := m.Run()
	cleanup()
	os.Exit(r)
}

func TestV23SyncbasedWholeBlobTransfer(t *testing.T) {
	v23tests.RunTest(t, V23TestSyncbasedWholeBlobTransfer)
}

func TestV23SyncbasedSyncWithAppResolvedConflicts(t *testing.T) {
	v23tests.RunTest(t, V23TestSyncbasedSyncWithAppResolvedConflicts)
}

func TestV23SyncbasedJoinSyncgroup(t *testing.T) {
	v23tests.RunTest(t, V23TestSyncbasedJoinSyncgroup)
}

func TestV23SyncbasedGetDeltas(t *testing.T) {
	v23tests.RunTest(t, V23TestSyncbasedGetDeltas)
}

func TestV23SyncbasedGetDeltasWithDel(t *testing.T) {
	v23tests.RunTest(t, V23TestSyncbasedGetDeltasWithDel)
}

func TestV23SyncbasedCompEval(t *testing.T) {
	v23tests.RunTest(t, V23TestSyncbasedCompEval)
}

func TestV23SyncbasedExchangeDeltasWithAcls(t *testing.T) {
	v23tests.RunTest(t, V23TestSyncbasedExchangeDeltasWithAcls)
}

func TestV23SyncbasedExchangeDeltasWithConflicts(t *testing.T) {
	v23tests.RunTest(t, V23TestSyncbasedExchangeDeltasWithConflicts)
}

func TestV23NestedSyncgroups(t *testing.T) {
	v23tests.RunTest(t, V23TestNestedSyncgroups)
}

func TestV23NestedAndPeerSyncgroups(t *testing.T) {
	v23tests.RunTest(t, V23TestNestedAndPeerSyncgroups)
}

func TestV23SyncbasedGetDeltasPrePopulate(t *testing.T) {
	v23tests.RunTest(t, V23TestSyncbasedGetDeltasPrePopulate)
}

func TestV23SyncbasedGetDeltasMultiApp(t *testing.T) {
	v23tests.RunTest(t, V23TestSyncbasedGetDeltasMultiApp)
}

func TestV23SyncgroupSync(t *testing.T) {
	v23tests.RunTest(t, V23TestSyncgroupSync)
}
