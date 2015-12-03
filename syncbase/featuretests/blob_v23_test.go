// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"crypto/md5"
	"crypto/rand"
	"fmt"
	"reflect"
	"time"

	"v.io/v23/context"
	"v.io/v23/naming"
	wire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/syncbase"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/syncbase/server/util"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test/v23tests"
)

//go:generate jiri test generate

func V23TestBlobWholeTransfer(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")

	server0Creds := forkCredentials(t, "s0")
	client0Ctx := forkContext(t, "c0")
	cleanup0 := tu.StartSyncbased(t, server0Creds, "sync0", "",
		`{"Read": {"In":["root:c0"]}, "Write": {"In":["root:c0"]}}`)
	defer cleanup0()

	server1Creds := forkCredentials(t, "s1")
	client1Ctx := forkContext(t, "c1")
	cleanup1 := tu.StartSyncbased(t, server1Creds, "sync1", "",
		`{"Read": {"In":["root:c1"]}, "Write": {"In":["root:c1"]}}`)
	defer cleanup1()

	sgName := naming.Join("sync0", util.SyncbaseSuffix, "SG1")

	ok(t, setupAppA(client0Ctx, "sync0"))
	ok(t, populateData(client0Ctx, "sync0", "foo", 0, 10))
	ok(t, createSyncgroup(client0Ctx, "sync0", sgName, "tb:foo", "", "root:s0;root:s1"))

	ok(t, setupAppA(client1Ctx, "sync1"))
	ok(t, joinSyncgroup(client1Ctx, "sync1", sgName))
	ok(t, verifySyncgroupData(client1Ctx, "sync1", "foo", 0, 10))

	// FetchBlob first.
	ok(t, generateBlob(client0Ctx, "sync0", "foo", 0, []byte("foobarbaz")))
	ok(t, fetchBlob(client1Ctx, "sync1", "foo", 0, 9, false))
	ok(t, getBlob(client1Ctx, "sync1", "foo", 0, []byte("foobarbaz"), 0))

	// GetBlob directly.
	ok(t, generateBlob(client1Ctx, "sync1", "foo", 0, []byte("abcdefghijklmn")))
	// Sleep so that the update to key "foo0" makes it to the other side.
	time.Sleep(10 * time.Second)
	ok(t, getBlob(client0Ctx, "sync0", "foo", 0, []byte("fghijklmn"), 5))
	ok(t, fetchBlob(client0Ctx, "sync0", "foo", 0, 14, true))

	// Test with a big blob (1 MB).
	ok(t, generateBigBlob(client0Ctx, "sync0", "foo", 1))
	ok(t, getBigBlob(client1Ctx, "sync1", "foo", 1))
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
	a := syncbase.NewService(syncbaseName).App(testApp)
	d := a.NoSQLDatabase(testDb, nil)
	tb := d.Table(testTable)

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
	r := tb.Row(key)
	s := testStruct{Val: "testkey" + key, Blob: b.Ref()}
	if err := r.Put(ctx, s); err != nil {
		return fmt.Errorf("r.Put() failed: %v", err)
	}

	return nil
}

func fetchBlob(ctx *context.T, syncbaseName, keyPrefix string, pos int, wantSize int64, skipIncStatus bool) error {
	a := syncbase.NewService(syncbaseName).App(testApp)
	d := a.NoSQLDatabase(testDb, nil)
	tb := d.Table(testTable)

	key := fmt.Sprintf("%s%d", keyPrefix, pos)
	r := tb.Row(key)
	var s testStruct

	// Try for 10 seconds to get the new value.
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
		return fmt.Errorf("r.Get() failed: %v", err)
	}

	b := d.Blob(s.Blob)
	bs, err := b.Fetch(ctx, 100)
	if err != nil {
		return fmt.Errorf("Fetch RPC failed, err %v", err)
	}

	status := []wire.BlobFetchStatus{
		wire.BlobFetchStatus{State: wire.BlobFetchStatePending},
		wire.BlobFetchStatus{State: wire.BlobFetchStateLocating},
		wire.BlobFetchStatus{State: wire.BlobFetchStateFetching},
		wire.BlobFetchStatus{State: wire.BlobFetchStateDone}}

	var gotStatus wire.BlobFetchStatus
	i := 0
	for bs.Advance() {
		gotStatus = bs.Value()

		if !skipIncStatus {
			if i <= 1 {
				if !reflect.DeepEqual(gotStatus, status[i]) {
					return fmt.Errorf("Fetch blob failed, got status %v want status %v", gotStatus, status[i])
				}
				i++
			} else if !(gotStatus.State == status[2].State || reflect.DeepEqual(gotStatus, status[3])) {
				return fmt.Errorf("Fetch blob failed, got status %v", gotStatus)
			}
		}
	}

	if !reflect.DeepEqual(gotStatus, status[3]) {
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
	a := syncbase.NewService(syncbaseName).App(testApp)
	d := a.NoSQLDatabase(testDb, nil)
	tb := d.Table(testTable)

	key := fmt.Sprintf("%s%d", keyPrefix, pos)
	r := tb.Row(key)
	var s testStruct

	// Try for 10 seconds to get the new value.
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
		return fmt.Errorf("r.Get() failed: %v", err)
	}

	b := d.Blob(s.Blob)
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
	a := syncbase.NewService(syncbaseName).App(testApp)
	d := a.NoSQLDatabase(testDb, nil)
	tb := d.Table(testTable)

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
	r := tb.Row(key)

	// Blob hash is transferred via structured store.
	s := testStruct{Val: hashToString(hasher.Sum(nil)), Blob: b.Ref()}

	if err := r.Put(ctx, s); err != nil {
		return fmt.Errorf("r.Put() failed: %v", err)
	}

	return nil
}

func getBigBlob(ctx *context.T, syncbaseName, keyPrefix string, pos int) error {
	a := syncbase.NewService(syncbaseName).App(testApp)
	d := a.NoSQLDatabase(testDb, nil)
	tb := d.Table(testTable)

	key := fmt.Sprintf("%s%d", keyPrefix, pos)
	r := tb.Row(key)
	var s testStruct

	// Try for 10 seconds to get the new value.
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
		return fmt.Errorf("r.Get() failed: %v", err)
	}

	b := d.Blob(s.Blob)
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
