// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"runtime/debug"
	"syscall"

	"v.io/x/ref/test/modules"
	"v.io/x/ref/test/v23tests"
)

// StartSyncbased starts the syncbased binary. The syncbased invocation is
// intended to be used from an integration test (v23.test). The returned
// cleanup function should be called once the invokation is no longer used.
func StartSyncbased(t *v23tests.T, creds *modules.CustomCredentials, name, permsLiteral string) (cleanup func()) {
	syncbased := t.BuildV23Pkg("v.io/syncbase/x/ref/services/syncbase/syncbased")
	// Create dir for the store.
	path, err := ioutil.TempDir("", "syncbase_leveldb")
	if err != nil {
		V23Fatalf(t, "can't create temp dir: %v", err)
	}
	// Start syncbased
	invocation := syncbased.WithStartOpts(syncbased.StartOpts().WithCustomCredentials(creds)).Start(
		"--v23.tcp.address=127.0.0.1:0",
		"--v23.permissions.literal", permsLiteral,
		"--name="+name,
		"--root-dir="+path)
	return func() {
		go invocation.Kill(syscall.SIGINT)
		stdout, stderr := bytes.NewBuffer(nil), bytes.NewBuffer(nil)
		if err := invocation.Shutdown(stdout, stderr); err != nil {
			log.Printf("syncbase terminated with an error: %v\nstdout: %v\nstderr: %v\n", err, stdout, stderr)
		}
		if err := os.RemoveAll(path); err != nil {
			V23Fatalf(t, "can't destroy db at %v: %v", path, err)
		}
	}
}

// RunClient runs modules.Program and waits until it terminates.
func RunClient(t *v23tests.T, creds *modules.CustomCredentials, program modules.Program) {
	client, err := t.Shell().StartWithOpts(
		t.Shell().DefaultStartOpts().WithCustomCredentials(creds),
		nil,
		program)
	if err != nil {
		V23Fatalf(t, "unable to start the client: %v", err)
	}
	stdout, stderr := bytes.NewBuffer(nil), bytes.NewBuffer(nil)
	if err := client.Shutdown(stdout, stderr); err != nil {
		V23Fatalf(t, "client failed: %v\nstdout: %v\nstderr: %v\n", err, stdout, stderr)
	}
}

func V23Fatalf(t *v23tests.T, format string, args ...interface{}) {
	debug.PrintStack()
	t.Fatalf(format, args...)
}
