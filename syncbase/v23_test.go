// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

package syncbase_test

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

func TestV23SyncbasedPutGet(t *testing.T) {
	v23tests.RunTest(t, V23TestSyncbasedPutGet)
}

func TestV23ServiceRestart(t *testing.T) {
	v23tests.RunTest(t, V23TestServiceRestart)
}

func TestV23ServiceCrashRestart(t *testing.T) {
	v23tests.RunTest(t, V23TestServiceCrashRestart)
}

func TestV23DeviceManager(t *testing.T) {
	v23tests.RunTest(t, V23TestDeviceManager)
}
