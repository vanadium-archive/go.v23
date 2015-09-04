// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql_test

import (
	"crypto/md5"
	"crypto/rand"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"v.io/v23"
	"v.io/v23/naming"
	wire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/syncbase"
	_ "v.io/x/ref/runtime/factories/generic"
	constants "v.io/x/ref/services/syncbase/server/util"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test/modules"
	"v.io/x/ref/test/v23tests"
)

//go:generate v23 test generate

func V23TestSyncbasedWholeBlobTransfer(t *v23tests.T) {
	v23tests.RunRootMT(t, "--v23.tcp.address=127.0.0.1:0")
	server0Creds, _ := t.Shell().NewChildCredentials("s0")
	client0Creds, _ := t.Shell().NewChildCredentials("c0")
	cleanSync0 := tu.StartSyncbased(t, server0Creds, "sync0", "",
		`{"Read": {"In":["root/c0"]}, "Write": {"In":["root/c0"]}}`)
	defer cleanSync0()

	server1Creds, _ := t.Shell().NewChildCredentials("s1")
	client1Creds, _ := t.Shell().NewChildCredentials("c1")
	cleanSync1 := tu.StartSyncbased(t, server1Creds, "sync1", "",
		`{"Read": {"In":["root/c1"]}, "Write": {"In":["root/c1"]}}`)
	defer cleanSync1()

	sgName := naming.Join("sync0", constants.SyncbaseSuffix, "SG1")

	tu.RunClient(t, client0Creds, runSetupAppA, "sync0")
	tu.RunClient(t, client0Creds, runPopulateData, "sync0", "foo", "0")
	tu.RunClient(t, client0Creds, runCreateSyncGroup, "sync0", sgName, "tb:foo", "root/s0", "root/s1")

	tu.RunClient(t, client1Creds, runSetupAppA, "sync1")
	tu.RunClient(t, client1Creds, runJoinSyncGroup, "sync1", sgName)
	tu.RunClient(t, client1Creds, runVerifySyncGroupData, "sync1", "foo", "0", "10")

	// FetchBlob first.
	tu.RunClient(t, client0Creds, runGenerateBlob, "sync0", "foo", "0", "foobarbaz")
	tu.RunClient(t, client1Creds, runFetchBlob, "sync1", "foo", "0", "9", "false")
	tu.RunClient(t, client1Creds, runGetBlob, "sync1", "foo", "0", "foobarbaz", "0")

	// GetBlob directly.
	tu.RunClient(t, client1Creds, runGenerateBlob, "sync1", "foo", "0", "abcdefghijklmn")
	// Sleep so that the update to key "foo0" makes it to the other side.
	time.Sleep(10 * time.Second)
	tu.RunClient(t, client0Creds, runGetBlob, "sync0", "foo", "0", "fghijklmn", "5")
	tu.RunClient(t, client0Creds, runFetchBlob, "sync0", "foo", "0", "14", "true")

	// Test with a big blob (1 MB).
	tu.RunClient(t, client0Creds, runGenerateBigBlob, "sync0", "foo", "1")
	tu.RunClient(t, client1Creds, runGetBigBlob, "sync1", "foo", "1")
}

////////////////////////////////////
// Helpers.

type testStruct struct {
	Val  string
	Blob wire.BlobRef
}

// Arguments: 0: syncbase name, 1: key prefix, 2: key position (1+2 is the
// actual key), 3: blob data.
var runGenerateBlob = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	b, err := d.CreateBlob(ctx)
	if err != nil {
		return fmt.Errorf("CreateBlob failed, err %v\n", err)
	}
	bw, err := b.Put(ctx)
	if err != nil {
		return fmt.Errorf("PutBlob RPC failed, err %v\n", err)
	}

	data := args[3]
	if err := bw.Send([]byte(data)); err != nil {
		return fmt.Errorf("Sending blob data failed, err %v\n", err)
	}
	if err := bw.Close(); err != nil {
		return fmt.Errorf("Closing blob writer failed, err %v\n", err)
	}

	// Commit the blob.
	if err := b.Commit(ctx); err != nil {
		return fmt.Errorf("Committing a blob failed, err %v\n", err)
	}

	// Put the BlobRef in a key.
	tb := d.Table("tb")
	pos, _ := strconv.ParseUint(args[2], 10, 64)

	key := fmt.Sprintf("%s%d", args[1], pos)
	r := tb.Row(key)
	s := testStruct{Val: "testkey" + key, Blob: b.Ref()}
	if err := r.Put(ctx, s); err != nil {
		return fmt.Errorf("r.Put() failed: %v\n", err)
	}

	return nil
}, "runGenerateBlob")

// Arguments: 0: syncbase name, 1: key prefix, 2: key position (1+2 is the
// actual key), 3: blob size, 4: skip incremental status checking.
var runFetchBlob = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	tb := d.Table("tb")
	pos, _ := strconv.ParseUint(args[2], 10, 64)

	key := fmt.Sprintf("%s%d", args[1], pos)
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
		return fmt.Errorf("r.Get() failed: %v\n", err)
	}

	b := d.Blob(s.Blob)
	bs, err := b.Fetch(ctx, 100)
	if err != nil {
		return fmt.Errorf("Fetch RPC failed, err %v\n", err)
	}

	status := []wire.BlobFetchStatus{
		wire.BlobFetchStatus{State: wire.BlobFetchStatePending},
		wire.BlobFetchStatus{State: wire.BlobFetchStateLocating},
		wire.BlobFetchStatus{State: wire.BlobFetchStateFetching},
		wire.BlobFetchStatus{State: wire.BlobFetchStateDone}}

	skipIncStatus, _ := strconv.ParseBool(args[4])

	var gotStatus wire.BlobFetchStatus
	i := 0
	for bs.Advance() {
		gotStatus = bs.Value()

		if !skipIncStatus {
			if i <= 1 {
				if !reflect.DeepEqual(gotStatus, status[i]) {
					return fmt.Errorf("Fetch blob failed, got status %v want status %v\n", gotStatus, status[i])
				}
				i++
			} else if !(gotStatus.State == status[2].State || reflect.DeepEqual(gotStatus, status[3])) {
				return fmt.Errorf("Fetch blob failed, got status %v\n", gotStatus)
			}
		}
	}

	if !reflect.DeepEqual(gotStatus, status[3]) {
		return fmt.Errorf("Fetch blob failed, got status %v want status %v\n", gotStatus, status[3])
	}

	if bs.Err() != nil {
		return fmt.Errorf("Fetch blob failed, err %v\n", err)
	}

	wantSize, _ := strconv.ParseInt(args[3], 10, 64)
	gotSize, err := b.Size(ctx)
	if err != nil || gotSize != wantSize {
		return fmt.Errorf("Blob size incorrect, got %v want %v, err %v\n", gotSize, wantSize, err)
	}

	return nil
}, "runFetchBlob")

// Arguments: 0: syncbase name, 1: key prefix, 2: key position (1+2 is the
// actual key), 3: expected blob data, 4: offset for get.
var runGetBlob = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	tb := d.Table("tb")
	pos, _ := strconv.ParseUint(args[2], 10, 64)

	key := fmt.Sprintf("%s%d", args[1], pos)
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
		return fmt.Errorf("r.Get() failed: %v\n", err)
	}

	b := d.Blob(s.Blob)
	offset, _ := strconv.ParseInt(args[4], 10, 64)
	br, err := b.Get(ctx, offset)
	if err != nil {
		return fmt.Errorf("GetBlob RPC failed, err %v\n", err)
	}
	var gotVal []byte
	for br.Advance() {
		gotVal = append(gotVal, br.Value()...)
	}
	if br.Err() != nil {
		return fmt.Errorf("Getting a blob failed, err %v\n", br.Err())
	}
	if !reflect.DeepEqual(gotVal, []byte(args[3])) {
		return fmt.Errorf("Getting a blob failed, got %v want %v\n", gotVal, []byte(args[3]))
	}

	return nil
}, "runGetBlob")

// Arguments: 0: syncbase name, 1: key prefix, 2: key position (1+2 is the
// actual key).
var runGenerateBigBlob = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	b, err := d.CreateBlob(ctx)
	if err != nil {
		return fmt.Errorf("CreateBlob failed, err %v\n", err)
	}
	bw, err := b.Put(ctx)
	if err != nil {
		return fmt.Errorf("PutBlob RPC failed, err %v\n", err)
	}

	hasher := md5.New()

	chunkSize := 8192
	content := make([]byte, chunkSize)
	// Send 1 MB blob.
	for i := 0; i < 128; i++ {
		if n, err := rand.Read(content); err != nil || n != chunkSize {
			return fmt.Errorf("Creating blob data failed, n %v err %v\n", n, err)
		}
		if err := bw.Send(content); err != nil {
			return fmt.Errorf("Sending blob data failed, err %v\n", err)
		}
		hasher.Write(content)
	}
	if err := bw.Close(); err != nil {
		return fmt.Errorf("Closing blob writer failed, err %v\n", err)
	}

	// Commit the blob.
	if err := b.Commit(ctx); err != nil {
		return fmt.Errorf("Committing a blob failed, err %v\n", err)
	}

	// Put the BlobRef in a key.
	tb := d.Table("tb")
	pos, _ := strconv.ParseUint(args[2], 10, 64)

	key := fmt.Sprintf("%s%d", args[1], pos)
	r := tb.Row(key)

	// Blob hash is transferred via structured store.
	s := testStruct{Val: hashToString(hasher.Sum(nil)), Blob: b.Ref()}

	if err := r.Put(ctx, s); err != nil {
		return fmt.Errorf("r.Put() failed: %v\n", err)
	}

	return nil
}, "runGenerateBigBlob")

// Arguments: 0: syncbase name, 1: key prefix, 2: key position (1+2 is the
// actual key).
var runGetBigBlob = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	tb := d.Table("tb")
	pos, _ := strconv.ParseUint(args[2], 10, 64)

	key := fmt.Sprintf("%s%d", args[1], pos)
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
		return fmt.Errorf("r.Get() failed: %v\n", err)
	}

	b := d.Blob(s.Blob)
	br, err := b.Get(ctx, 0)
	if err != nil {
		return fmt.Errorf("GetBlob RPC failed, err %v\n", err)
	}
	hasher := md5.New()
	for br.Advance() {
		content := br.Value()
		hasher.Write(content)
	}
	if br.Err() != nil {
		return fmt.Errorf("Getting a blob failed, err %v\n", br.Err())
	}

	gotHash := hashToString(hasher.Sum(nil))
	if !reflect.DeepEqual(gotHash, s.Val) {
		return fmt.Errorf("Getting a blob failed, got %v want %v\n", gotHash, s.Val)
	}

	return nil
}, "runGetBigBlob")

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
