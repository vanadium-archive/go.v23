// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase_test

import (
	"regexp"
	"sync"
	"testing"

	"v.io/v23/syncbase"
)

const (
	uuidLoopInvocations int = 100
)

func TestUUIDFormat(t *testing.T) {
	uuid := syncbase.UUID()
	regexp := regexp.MustCompile("(?i)^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$")
	if !regexp.MatchString(uuid) {
		t.Errorf("Incorrect UUID format: %v", uuid)
	}
}

func TestUUIDCollisions(t *testing.T) {
	var mutex sync.Mutex
	var waitGroup sync.WaitGroup
	uuidMap := make(map[string]bool)

	createUUID := func() {
		mutex.Lock()
		defer mutex.Unlock()
		uuidMap[syncbase.UUID()] = true
		waitGroup.Done()
	}

	for i := 0; i < uuidLoopInvocations; i++ {
		waitGroup.Add(1)
		go createUUID()
	}

	waitGroup.Wait()

	if len(uuidMap) != uuidLoopInvocations {
		t.Errorf("UUID collision for %d UUIDs", uuidLoopInvocations-len(uuidMap))
	}
}
