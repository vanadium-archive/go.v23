// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

package featuretests_test

import (
	"os"
	"testing"

	"v.io/x/ref/test/modules"
	"v.io/x/ref/test/v23tests"
)

func TestMain(m *testing.M) {
	modules.DispatchAndExitIfChild()
	cleanup := v23tests.UseSharedBinDir()
	r := m.Run()
	cleanup()
	os.Exit(r)
}

func TestV23DefaultCR(t *testing.T) {
	v23tests.RunTest(t, V23TestDefaultCR)
}

func TestV23SyncbasedSyncWithAppResolvedConflicts(t *testing.T) {
	v23tests.RunTest(t, V23TestSyncbasedSyncWithAppResolvedConflicts)
}

func TestV23SyncbasedJoinSyncgroup(t *testing.T) {
	v23tests.RunTest(t, V23TestSyncbasedJoinSyncgroup)
}
