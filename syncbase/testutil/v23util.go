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

// StartSyncbased starts a syncbased process, intended to be accessed from an
// integration test (run using --v23.tests). The returned cleanup function
// should be called once the syncbased process is no longer needed.
func StartSyncbased(t *v23tests.T, creds *modules.CustomCredentials, name, rootDir, permsLiteral string) (cleanup func()) {
	syncbased := t.BuildV23Pkg("v.io/syncbase/x/ref/services/syncbase/syncbased")
	// Create root dir for the store.
	rmRootDir := false
	if rootDir == "" {
		var err error
		rootDir, err = ioutil.TempDir("", "syncbase_leveldb")
		if err != nil {
			V23Fatalf(t, "can't create temp dir: %v", err)
		}
		rmRootDir = true
	}
	// Start syncbased.
	invocation := syncbased.WithStartOpts(syncbased.StartOpts().WithCustomCredentials(creds)).Start(
		"--v23.tcp.address=127.0.0.1:0",
		"--v23.permissions.literal", permsLiteral,
		"--name="+name,
		"--root-dir="+rootDir)
	return func() {
		// TODO(sadovsky): Something's broken here. If the syncbased invocation
		// fails (e.g. if NewService returns an error), currently it's possible for
		// the test to fail without the crash error getting logged. This makes
		// debugging a challenge.
		go invocation.Kill(syscall.SIGINT)
		stdout, stderr := bytes.NewBuffer(nil), bytes.NewBuffer(nil)
		if err := invocation.Shutdown(stdout, stderr); err != nil {
			log.Printf("syncbased terminated with an error: %v\nstdout: %v\nstderr: %v\n", err, stdout, stderr)
		}
		if rmRootDir {
			if err := os.RemoveAll(rootDir); err != nil {
				V23Fatalf(t, "can't remove dir %v: %v", rootDir, err)
			}
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
