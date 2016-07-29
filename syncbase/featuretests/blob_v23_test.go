// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"crypto/md5"
	"crypto/rand"
	"fmt"
	"reflect"
	"testing"
	"time"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/x/ref/test/v23test"
)

func TestV23BlobWholeTransfer(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	sbs := setupSyncbases(t, sh, 2, false, false)

	sgId := wire.Id{Name: "SG1", Blessing: testCx.Blessing}

	ok(t, createCollection(sbs[0].clientCtx, sbs[0].sbName, testCx.Name))
	ok(t, populateData(sbs[0].clientCtx, sbs[0].sbName, testCx.Name, "foo", 0, 10))
	ok(t, createSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sgId, testCx.Name, "", nil, clBlessings(sbs), "", wire.SyncgroupMemberInfo{SyncPriority: 8}))
	ok(t, joinSyncgroup(sbs[1].clientCtx, sbs[1].sbName, sbs[0].sbName, sgId,
		wire.SyncgroupMemberInfo{SyncPriority: 10}))
	ok(t, verifySyncgroupData(sbs[1].clientCtx, sbs[1].sbName, testCx.Name, "foo", "", 0, 10))

	// FetchBlob first.
	ok(t, generateBlob(sbs[0].clientCtx, sbs[0].sbName, "foo", 0, []byte("foobarbaz")))
	ok(t, fetchBlob(sbs[1].clientCtx, sbs[1].sbName, "foo", 0, 9, false))
	ok(t, getBlob(sbs[1].clientCtx, sbs[1].sbName, "foo", 0, []byte("foobarbaz"), 0))

	// GetBlob directly.
	ok(t, generateBlob(sbs[1].clientCtx, sbs[1].sbName, "foo", 0, []byte("abcdefghijklmn")))
	// Sleep so that the update to key "foo0" makes it to the other side.
	time.Sleep(10 * time.Second)
	ok(t, getBlob(sbs[0].clientCtx, sbs[0].sbName, "foo", 0, []byte("fghijklmn"), 5))
	ok(t, fetchBlob(sbs[0].clientCtx, sbs[0].sbName, "foo", 0, 14, true))

	// Test with a big blob (1 MB).
	ok(t, generateBigBlob(sbs[0].clientCtx, sbs[0].sbName, "foo", 1))
	ok(t, getBigBlob(sbs[1].clientCtx, sbs[1].sbName, "foo", 1))
}

// TestV23ServerBlobFetch tests the mechanism for fetch blobs automatically on
// servers.  It sets up two syncbases:
// - 0 is a normal syncgroup member, and creates a syncgroup.
// - 1 is a server, and joins the syncgroup.
// Both create blobs, then the test waits until the autmoatic fetch mechanism
// has had a chance to run.  Then, it checks that each syncbase has the blob it
// created itself, that the server has the blob created by the non-server, and
// the non-server does not (initially) have the blob created by the server
// (this latter check fetches the blob to the non-creator as a side effect).  Then
// the values of the blobs are checked at both syncbases.
func TestV23ServerBlobFetch(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	sbs := setupSyncbases(t, sh, 2, true, false)

	sgId := wire.Id{Name: "SG1", Blessing: testCx.Blessing}

	ok(t, createCollection(sbs[0].clientCtx, sbs[0].sbName, testCx.Name))
	ok(t, populateData(sbs[0].clientCtx, sbs[0].sbName, testCx.Name, "foo", 0, 10))
	// sbs[0] is not a server.
	ok(t, createSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sgId, testCx.Name, "", nil, clBlessings(sbs), "",
		wire.SyncgroupMemberInfo{SyncPriority: 8}))
	// sbs[1] is a server, so will fetch blobs automatically; it joins the syncgroup.
	ok(t, joinSyncgroup(sbs[1].clientCtx, sbs[1].sbName, sbs[0].sbName, sgId,
		wire.SyncgroupMemberInfo{SyncPriority: 10, BlobDevType: byte(wire.BlobDevTypeServer)}))
	ok(t, verifySyncgroupData(sbs[1].clientCtx, sbs[1].sbName, testCx.Name, "foo", "", 0, 10))

	// Generate a blob on each member.
	ok(t, generateBlob(sbs[0].clientCtx, sbs[0].sbName, "foo", 0, []byte("foobarbaz")))
	ok(t, generateBlob(sbs[1].clientCtx, sbs[1].sbName, "foo", 1, []byte("wombatnumbat")))

	// Before the server fetch, each node should have just its own blob.
	checkShares(t, "before server fetch", sbs, []int{
		2, 0,
		0, 2,
	})

	// Sleep until the fetch mechanism has a chance to run  (its initial timeout is 10s,
	// to make this test possible).
	time.Sleep(15 * time.Second)

	// Now the server should have all blob shares.
	checkShares(t, "after server fetch", sbs, []int{
		0, 0,
		2, 2,
	})

	// The syncbases should have the blobs they themselves created.
	ok(t, fetchBlob(sbs[0].clientCtx, sbs[0].sbName, "foo", 0, 9, true /*already present*/))
	ok(t, fetchBlob(sbs[1].clientCtx, sbs[1].sbName, "foo", 1, 12, true /*already present*/))

	// sbs[1] should already have the blob created by sbs[0], since sbs[1] is a server.
	ok(t, fetchBlob(sbs[1].clientCtx, sbs[1].sbName, "foo", 0, 9, true /*already present*/))

	// sbs[0] should not already have the blob created by sbs[1], since sbs[0] is not a server.
	ok(t, fetchBlob(sbs[0].clientCtx, sbs[0].sbName, "foo", 1, 12, false /*not already present*/))

	// The share distribution should not have changed.
	checkShares(t, "at end", sbs, []int{
		0, 0,
		2, 2,
	})

	// Wrap up by checking the blob values.
	ok(t, getBlob(sbs[0].clientCtx, sbs[0].sbName, "foo", 1, []byte("wombatnumbat"), 0))
	ok(t, getBlob(sbs[1].clientCtx, sbs[1].sbName, "foo", 1, []byte("wombatnumbat"), 0))
	ok(t, getBlob(sbs[0].clientCtx, sbs[0].sbName, "foo", 0, []byte("foobarbaz"), 0))
	ok(t, getBlob(sbs[1].clientCtx, sbs[1].sbName, "foo", 0, []byte("foobarbaz"), 0))
}

// TestV23LeafBlobFetch tests the mechanism for leaf syncbases to give blobs to
// non-leaf syncbases with which they sync.  It sets up two syncbases:
// - 0 is a non-leaf syncgroup member; it creates a syncgroup.
// - 1 is a leaf syncgroup member; it joins the syncgroup.
// Both create blobs.   The test checks that the non-leaf syncbase automatically
// fetches the blob created on the leaf, but not vice versa.
func TestV23LeafBlobFetch(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	sbs := setupSyncbases(t, sh, 2, true, false)

	sgId := wire.Id{Name: "SG1", Blessing: testCx.Blessing}

	ok(t, createCollection(sbs[0].clientCtx, sbs[0].sbName, testCx.Name))
	ok(t, populateData(sbs[0].clientCtx, sbs[0].sbName, testCx.Name, "foo", 0, 10))
	// sbs[0] is a non-leaf member that creates the syncgroup, and creates a blob.
	ok(t, createSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sgId, testCx.Name, "", nil, clBlessings(sbs), "",
		wire.SyncgroupMemberInfo{SyncPriority: 8}))
	// sbs[1] is a leaf member; it joins the syncgroup, and creates a blob.
	// It will tell the non-leaf member to fetch its blob.
	ok(t, joinSyncgroup(sbs[1].clientCtx, sbs[1].sbName, sbs[0].sbName, sgId,
		wire.SyncgroupMemberInfo{SyncPriority: 8, BlobDevType: byte(wire.BlobDevTypeLeaf)}))
	ok(t, verifySyncgroupData(sbs[1].clientCtx, sbs[1].sbName, testCx.Name, "foo", "", 0, 10))

	// Generate a blob on each member.
	ok(t, generateBlob(sbs[0].clientCtx, sbs[0].sbName, "foo", 0, []byte("foobarbaz")))
	ok(t, generateBlob(sbs[1].clientCtx, sbs[1].sbName, "foo", 1, []byte("wombatnumbat")))

	time.Sleep(5 * time.Second)

	// One ownership share of the leaf's blob should have been transferred to the non-leaf.
	checkShares(t, "leaf initial", sbs, []int{
		2, 1,
		0, 1,
	})

	// The syncbases should have the blobs they themselves created.
	ok(t, fetchBlob(sbs[0].clientCtx, sbs[0].sbName, "foo", 0, 9, true /*already present*/))
	ok(t, fetchBlob(sbs[1].clientCtx, sbs[1].sbName, "foo", 1, 12, true /*already present*/))

	// sbs[0] should have the leaf's blob too.
	ok(t, fetchBlob(sbs[0].clientCtx, sbs[0].sbName, "foo", 1, 12, true /*already present*/))

	// sbs[1] should not have the blob created by sbs[0].
	ok(t, fetchBlob(sbs[1].clientCtx, sbs[1].sbName, "foo", 0, 9, false /*not already present*/))

	// Wrap up by checking the blob values.
	ok(t, getBlob(sbs[0].clientCtx, sbs[0].sbName, "foo", 0, []byte("foobarbaz"), 0))
	ok(t, getBlob(sbs[1].clientCtx, sbs[1].sbName, "foo", 0, []byte("foobarbaz"), 0))
	ok(t, getBlob(sbs[0].clientCtx, sbs[0].sbName, "foo", 1, []byte("wombatnumbat"), 0))
	ok(t, getBlob(sbs[1].clientCtx, sbs[1].sbName, "foo", 1, []byte("wombatnumbat"), 0))
}

////////////////////////////////////
// Helpers.

// TODO(sadovsky): Noticed while refactoring: there is a lot of duplicated code
// below, e.g. for generating blobs, getting testStruct values, etc. We should
// aim to avoid such duplication as it makes it more difficult to change APIs,
// fix bugs, etc.

type testStruct struct {
	Val  string
	Blob wire.BlobRef
}

func generateBlob(ctx *context.T, syncbaseName, keyPrefix string, pos int, data []byte) error {
	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	c := d.CollectionForId(testCx)

	b, err := d.CreateBlob(ctx)
	if err != nil {
		return fmt.Errorf("CreateBlob failed, err %v", err)
	}
	bw, err := b.Put(ctx)
	if err != nil {
		return fmt.Errorf("PutBlob RPC failed, err %v", err)
	}

	if err := bw.Send(data); err != nil {
		return fmt.Errorf("Sending blob data failed, err %v", err)
	}
	if err := bw.Close(); err != nil {
		return fmt.Errorf("Closing blob writer failed, err %v", err)
	}

	// Commit the blob.
	if err := b.Commit(ctx); err != nil {
		return fmt.Errorf("Committing a blob failed, err %v", err)
	}

	// Put the BlobRef in a key.
	key := fmt.Sprintf("%s%d", keyPrefix, pos)
	r := c.Row(key)
	s := testStruct{Val: "testkey" + key, Blob: b.Ref()}
	if err := r.Put(ctx, s); err != nil {
		return fmt.Errorf("r.Put() failed: %v", err)
	}

	return nil
}

// getTestStructAndBlobFromRow waits until the row with the key keyPrefix + pos
// is accessible on the specified syncbase, then reads and returns the value
// from that row, which is expected to be a testStruct.  A blob handle from the
// blobref is also returned.
// This is a common subroutine used in several of the routines below.
func getTestStructAndBlobFromRow(ctx *context.T, syncbaseName, keyPrefix string, pos int) (testStruct, syncbase.Blob, error) {
	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	c := d.CollectionForId(testCx)

	key := fmt.Sprintf("%s%d", keyPrefix, pos)
	r := c.Row(key)

	// Try for 10 seconds to get the new value.
	var s testStruct
	var err error
	for i := 0; i < 10; i++ {
		// Note: the error is a decode error since the old value is a
		// string, and the new value is testStruct.
		if err = r.Get(ctx, &s); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		return testStruct{}, nil, fmt.Errorf("r.Get() failed: %v", err)
	}
	return s, d.Blob(s.Blob), nil
}

// fetchBlob ensures that the blob named by key (keyPrefix, pos) can be fetched
// by syncbaseName, and has size wantSize.  If alreadyPresent is set, it checks
// that the blob is already present, and other checks that the blob is fetched
// from some remote syncbase.
func fetchBlob(ctx *context.T, syncbaseName, keyPrefix string, pos int, wantSize int64, alreadyPresent bool) error {
	var b syncbase.Blob
	var err error
	_, b, err = getTestStructAndBlobFromRow(ctx, syncbaseName, keyPrefix, pos)
	if err != nil {
		return err
	}

	bs, err := b.Fetch(ctx, 100)
	if err != nil {
		return fmt.Errorf("Fetch RPC failed, err %v", err)
	}

	var status []wire.BlobFetchStatus
	if alreadyPresent {
		status = []wire.BlobFetchStatus{wire.BlobFetchStatus{State: wire.BlobFetchStateDone}}
	} else {
		status = []wire.BlobFetchStatus{
			wire.BlobFetchStatus{State: wire.BlobFetchStatePending},
			wire.BlobFetchStatus{State: wire.BlobFetchStateLocating},
			wire.BlobFetchStatus{State: wire.BlobFetchStateFetching},
			wire.BlobFetchStatus{State: wire.BlobFetchStateDone}}
	}

	var gotStatus wire.BlobFetchStatus
	i := 0
	for bs.Advance() {
		gotStatus = bs.Value()
		if i <= 1 {
			if !reflect.DeepEqual(gotStatus, status[i]) {
				return fmt.Errorf("Fetch blob failed, got status %v want status %v", gotStatus, status[i])
			}
			i++
		} else if !(gotStatus.State == status[2].State || reflect.DeepEqual(gotStatus, status[3])) {
			return fmt.Errorf("Fetch blob failed, got status %v", gotStatus)
		}
	}

	if !reflect.DeepEqual(gotStatus, status[len(status)-1]) {
		return fmt.Errorf("Fetch blob failed, got status %v want status %v", gotStatus, status[3])
	}

	if bs.Err() != nil {
		return fmt.Errorf("Fetch blob failed, err %v", err)
	}

	gotSize, err := b.Size(ctx)
	if err != nil || gotSize != wantSize {
		return fmt.Errorf("Blob size incorrect, got %v want %v, err %v", gotSize, wantSize, err)
	}

	return nil
}

func getBlob(ctx *context.T, syncbaseName, keyPrefix string, pos int, wantVal []byte, offset int64) error {
	var b syncbase.Blob
	var err error
	_, b, err = getTestStructAndBlobFromRow(ctx, syncbaseName, keyPrefix, pos)
	if err != nil {
		return err
	}

	br, err := b.Get(ctx, offset)
	if err != nil {
		return fmt.Errorf("GetBlob RPC failed, err %v", err)
	}
	var gotVal []byte
	for br.Advance() {
		gotVal = append(gotVal, br.Value()...)
	}
	if br.Err() != nil {
		return fmt.Errorf("Getting a blob failed, err %v", br.Err())
	}
	if !reflect.DeepEqual(gotVal, wantVal) {
		return fmt.Errorf("Getting a blob failed, got %v want %v", gotVal, wantVal)
	}

	return nil
}

func generateBigBlob(ctx *context.T, syncbaseName, keyPrefix string, pos int) error {
	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	c := d.CollectionForId(testCx)

	b, err := d.CreateBlob(ctx)
	if err != nil {
		return fmt.Errorf("CreateBlob failed, err %v", err)
	}
	bw, err := b.Put(ctx)
	if err != nil {
		return fmt.Errorf("PutBlob RPC failed, err %v", err)
	}

	hasher := md5.New()

	chunkSize := 8192
	content := make([]byte, chunkSize)
	// Send 1 MB blob.
	for i := 0; i < 128; i++ {
		if n, err := rand.Read(content); err != nil || n != chunkSize {
			return fmt.Errorf("Creating blob data failed, n %v err %v", n, err)
		}
		if err := bw.Send(content); err != nil {
			return fmt.Errorf("Sending blob data failed, err %v", err)
		}
		hasher.Write(content)
	}
	if err := bw.Close(); err != nil {
		return fmt.Errorf("Closing blob writer failed, err %v", err)
	}

	// Commit the blob.
	if err := b.Commit(ctx); err != nil {
		return fmt.Errorf("Committing a blob failed, err %v", err)
	}

	// Put the BlobRef in a key.
	key := fmt.Sprintf("%s%d", keyPrefix, pos)
	r := c.Row(key)

	// Blob hash is transferred via structured store.
	s := testStruct{Val: hashToString(hasher.Sum(nil)), Blob: b.Ref()}

	if err := r.Put(ctx, s); err != nil {
		return fmt.Errorf("r.Put() failed: %v", err)
	}

	return nil
}

func getBigBlob(ctx *context.T, syncbaseName, keyPrefix string, pos int) error {
	var s testStruct
	var b syncbase.Blob
	var err error
	s, b, err = getTestStructAndBlobFromRow(ctx, syncbaseName, keyPrefix, pos)
	if err != nil {
		return err
	}

	br, err := b.Get(ctx, 0)
	if err != nil {
		return fmt.Errorf("GetBlob RPC failed, err %v", err)
	}
	hasher := md5.New()
	for br.Advance() {
		content := br.Value()
		hasher.Write(content)
	}
	if br.Err() != nil {
		return fmt.Errorf("Getting a blob failed, err %v", br.Err())
	}

	gotHash := hashToString(hasher.Sum(nil))
	if !reflect.DeepEqual(gotHash, s.Val) {
		return fmt.Errorf("Getting a blob failed, got %v want %v", gotHash, s.Val)
	}

	return nil
}

// getBlobOwnershipShares returns the ownership shares (summed across all syncgroups) for blob br.
func getBlobOwnershipShares(ctx *context.T, syncbaseName string, keyPrefix string, pos int) (shares int) {
	var s testStruct
	var err error
	s, _, err = getTestStructAndBlobFromRow(ctx, syncbaseName, keyPrefix, pos)
	if err == nil {
		var shareMap map[string]int32
		shareMap, err = sc(syncbaseName).DevModeGetBlobShares(ctx, s.Blob)
		if err == nil {
			// Sum the shares across all syncgroups.
			for _, shareCount := range shareMap {
				shares += int(shareCount)
			}
		}
	}
	return shares
}

// checkShares checks that the shares of the blobs mentioned in rows fooN (for N in 0..len(sbs)-1)
// are at the expected values for the various syncbases.   The elements of expectedShares[]
// are in the order sbs[0] for foo0, foo1, ...,  followed by sbs[1] for foo0, foo1, ... etc.
// An expected value of -1 means "don't care".
func checkShares(t *testing.T, msg string, sbs []*testSyncbase, expectedShares []int) {
	for sbIndex := 0; sbIndex != len(sbs); sbIndex++ {
		for blobIndex := 0; blobIndex != len(sbs); blobIndex++ {
			var expected int = expectedShares[blobIndex+len(sbs)*sbIndex]
			if expected != -1 {
				var shares int = getBlobOwnershipShares(sbs[sbIndex].clientCtx, sbs[sbIndex].sbName, "foo", blobIndex)
				if expected != shares {
					t.Errorf("%s: sb %d blob %d got %d shares, expected %d\n",
						msg, sbIndex, blobIndex, shares, expected)
				}
			}
		}
	}
}

// Copied from localblobstore/fs_cablobstore/fs_cablobstore.go.
//
// hashToString() returns a string representation of the hash.
// Requires len(hash)==16.  An md5 hash is suitable.
func hashToString(hash []byte) string {
	return fmt.Sprintf("%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x%02x",
		hash[0], hash[1], hash[2], hash[3],
		hash[4], hash[5], hash[6], hash[7],
		hash[8], hash[9], hash[10], hash[11],
		hash[12], hash[13], hash[14], hash[15])
}
