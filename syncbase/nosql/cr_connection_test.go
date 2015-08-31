// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	"fmt"
	"testing"
	"time"

	wire "v.io/syncbase/v23/services/syncbase/nosql"
	"v.io/syncbase/v23/syncbase/nosql/crtestutil"
	"v.io/v23/context"
)

type resolverImpl1 struct {
}

func (ri *resolverImpl1) OnConflict(ctx *context.T, conflict *Conflict) (r Resolution) {
	return
}

// TestCrConnectionClose tests if db.Close() correctly shuts down the goroutine
// that runs conflict resolution code in an infinite loop.
// Setup:
//   - A mutex lock is used to cause Advance() on the receive stream to block.
//     This lock is held by the test before starting off the CR thread.
//   - ctx.Cancel() is simulated by releasing the mutex lock. This is done right
//     after calling db.Close()
//   - The mutex is held again by the test to cause the CR thread to block on
//     Advance() in case the CR thead did not shutdown. This will lead to
//     failure of the test.
// Expected result:
//   - The CR thread should not get/remain blocked on Advance() after db.Close()
func TestCrConnectionClose(t *testing.T) {
	db := NewDatabase("parentName", "db1", getSchema(&resolverImpl1{}))
	advance := func(st *crtestutil.State) bool {
		fmt.Println("Advance() for ConflictStreamImpl called")
		st.IsBlocked = true
		st.Mu.Lock()
		defer st.Mu.Unlock()
		fmt.Println("Advance() for ConflictStreamImpl returning")
		st.IsBlocked = false
		return false
	}

	st := &crtestutil.State{}
	crStream := &crtestutil.CrStreamImpl{
		C: &crtestutil.ConflictStreamImpl{St: st, AdvanceFn: advance},
		R: &crtestutil.ResolutionStreamImpl{St: st},
	}
	db.c = crtestutil.MockDbClient(db.c, crStream)
	db.crState.reconnectWaitTime = 10 * time.Millisecond

	ctx, _ := context.RootContext()
	st.Mu.Lock()                                // causes Advance() to block
	db.EnforceSchema(ctx)                       // kicks off the CR thread
	for i := 0; i < 100 && !st.IsBlocked; i++ { // wait till Advance() call is blocked
		time.Sleep(time.Millisecond)
	}
	db.Close()
	st.Mu.Unlock() // simulate ctx cancel()
	for i := 0; i < 100 && st.IsBlocked; i++ {
		time.Sleep(time.Millisecond)
	}

	st.Mu.Lock() // causes Advance() to block if called again
	// let conflict resolution routine complete its cleanup.
	// If the code does not work as intended then it will end up reconnecting
	// the stream and blocking on the mutex
	time.Sleep(time.Millisecond)
	if st.IsBlocked {
		t.Error("Error: The conflict resolution routine did not die")
	}
}

// TestCrConnectionReestablish tests if the CR thread reestablishes a broken
// connection stream with syncbase.
// Setup:
//   - Advance() is allowed to return immediately 3 times with return value
//     false signifying 3 consecutive breaks in the connection.
//   - A mutex lock is used to cause Advance() on the recieve stream to block
//     on the 4th attempt. This lock is held by the test before starting off the
//     CR thread. This simulates a stable connection between CR and syncbase.
// Expected result:
//   - Advance() should get blocked after some time.
//   - st.AdvanceCount should be equal to 3
func TestCrConnectionReestablish(t *testing.T) {
	db := NewDatabase("parentName", "db1", getSchema(&resolverImpl1{}))
	advance := func(st *crtestutil.State) bool {
		fmt.Println("Advance() for ConflictStreamImpl called")
		st.AdvanceCount++
		if st.AdvanceCount > 2 {
			// Connection stays stable after 3 attempts
			st.IsBlocked = true
			st.Mu.Lock()
			defer st.Mu.Unlock()
		}
		fmt.Println("Advance() for ConflictStreamImpl returning")
		st.IsBlocked = false
		return false
	}

	st := &crtestutil.State{}
	crStream := &crtestutil.CrStreamImpl{
		C: &crtestutil.ConflictStreamImpl{St: st, AdvanceFn: advance},
		R: &crtestutil.ResolutionStreamImpl{St: st},
	}
	db.c = crtestutil.MockDbClient(db.c, crStream)
	db.crState.reconnectWaitTime = 10 * time.Millisecond

	ctx, _ := context.RootContext()
	st.Mu.Lock() // causes Advance() to block
	db.EnforceSchema(ctx)
	for i := 0; i < 100 && !st.IsBlocked; i++ {
		time.Sleep(time.Millisecond) // wait till Advance() call is blocked
	}

	if !st.IsBlocked {
		t.Error("Error: Advance() did not block")
	}
	if st.AdvanceCount != 3 {
		t.Errorf("Error: Advance() expected to be called 3 times, instead called %d times", st.AdvanceCount)
	}

	// Shutdown the cr routine
	db.Close()
	st.Mu.Unlock()
}

func getSchema(cr ConflictResolver) *Schema {
	return &Schema{
		Metadata: wire.SchemaMetadata{},
		Upgrader: nil,
		Resolver: cr,
	}
}
