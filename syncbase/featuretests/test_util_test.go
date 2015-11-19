// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"v.io/v23"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/nosql"
	"v.io/x/ref"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test/modules"
)

const (
	testTable = "tb"
)

///////////////////////////////////////////////////////
// Helpers for setting up App and db.

var runSetupAppA = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	a.Create(ctx, nil)
	d := a.NoSQLDatabase("d", nil)
	d.Create(ctx, nil)
	d.Table(testTable).Create(ctx, nil)

	return nil
}, "runSetupAppA")

/////////////////////////////////////////////////////////
// Helpers for adding or updating data

// Arguments: 0: Syncbase name, 1: key prefix, 2: start index
// Optional args: 3: end index.
// Values are from [start,end) or [start, start+10) depending on whether end
// param was provided.
var runPopulateData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Do Puts.
	tb := d.Table(testTable)
	start, _ := strconv.ParseUint(args[2], 10, 64)
	end := start + 10
	if len(args) > 3 {
		end, _ = strconv.ParseUint(args[3], 10, 64)
		if end <= start {
			return fmt.Errorf("Test error: end <= start. start: %d, end: %d", start, end)
		}
	}

	for i := start; i < end; i++ {
		key := fmt.Sprintf("%s%d", args[1], i)
		r := tb.Row(key)
		if err := r.Put(ctx, "testkey"+key); err != nil {
			return fmt.Errorf("r.Put() failed: %v\n", err)
		}
	}
	return nil
}, "runPopulateData")

// Arguments: 0: Syncbase name, 1: start index.
// Optional args: 2: end index, 3: value prefix.
// Values are from [start,end) or [start, start+5) depending on whether end
// param was provided.
var runUpdateData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	serviceName, startStr := args[0], args[1]
	start, _ := strconv.ParseUint(startStr, 10, 64)
	end, prefix := start+5, "testkey"
	if len(args) > 2 {
		end, _ = strconv.ParseUint(args[2], 10, 64)
		if end <= start {
			return fmt.Errorf("Test error: end <= start. start: %d, end: %d", start, end)
		}
	}
	if len(args) > 3 {
		prefix = args[3]
	}

	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Do Puts.
	tb := d.Table(testTable)

	for i := start; i < end; i++ {
		key := fmt.Sprintf("foo%d", i)
		r := tb.Row(key)
		if err := r.Put(ctx, prefix+serviceName+key); err != nil {
			return fmt.Errorf("r.Put() failed: %v\n", err)
		}
	}

	return nil
}, "runUpdateData")

// Updates data within a batch.
// Arguments: 0: Syncbase name, 1: start index.
// Optional args: 2: end index, 3: value prefix.
// Values are from [start,end) or [start, start+5) depending on whether end
// param was provided.
var runUpdateBatchData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	serviceName, startStr := args[0], args[1]
	start, _ := strconv.ParseUint(startStr, 10, 64)
	end, prefix := start+5, "testkey"
	if len(args) > 2 {
		end, _ = strconv.ParseUint(args[2], 10, 64)
		if end <= start {
			return fmt.Errorf("Test error: end <= start. start: %d, end: %d", start, end)
		}
	}
	if len(args) > 3 {
		prefix = args[3]
	}

	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Do Puts.
	batch, err := d.BeginBatch(ctx, wire.BatchOptions{})
	if err != nil {
		return fmt.Errorf("BeginBatch failed: %v\n", err)
	}
	tb := batch.Table(testTable)
	for i := start; i < end; i++ {
		key := fmt.Sprintf("foo%d", i)
		r := tb.Row(key)
		if err := r.Put(ctx, prefix+serviceName+key); err != nil {
			batch.Abort(ctx)
			return fmt.Errorf("r.Put() failed: %v\n", err)
		}
	}
	return batch.Commit(ctx)
}, "runUpdateBatchData")

//////////////////////////////////////////////
// Helpers for verifying data

// Arguments: 0: syncbase name, 1: key prefix, 2: start index, 3: number of keys, 4: skip scan.
var runVerifySyncgroupData = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	// Wait for a bit (up to 4 sec) until the last key appears.
	tb := d.Table(testTable)

	start, _ := strconv.ParseUint(args[2], 10, 64)
	count, _ := strconv.ParseUint(args[3], 10, 64)
	skipScan, _ := strconv.ParseBool(args[4])
	lastKey := fmt.Sprintf("%s%d", args[1], start+count-1)

	r := tb.Row(lastKey)
	for i := 0; i < 8; i++ {
		time.Sleep(500 * time.Millisecond)
		var value string
		if err := r.Get(ctx, &value); err == nil {
			break
		}
	}

	// Verify that all keys and values made it correctly.
	for i := start; i < start+count; i++ {
		key := fmt.Sprintf("%s%d", args[1], i)
		r := tb.Row(key)
		var got string
		if err := r.Get(ctx, &got); err != nil {
			return fmt.Errorf("r.Get() failed: %v\n", err)
		}
		want := "testkey" + key
		if got != want {
			return fmt.Errorf("unexpected value: got %q, want %q\n", got, want)
		}
	}

	if !skipScan {
		// Re-verify using a scan operation.
		stream := tb.Scan(ctx, nosql.Prefix(args[1]))
		for i := 0; stream.Advance(); i++ {
			want := fmt.Sprintf("%s%d", args[1], i)
			got := stream.Key()
			if got != want {
				return fmt.Errorf("unexpected key in scan: got %q, want %q\n", got, want)
			}
			want = "testkey" + want
			if err := stream.Value(&got); err != nil {
				return fmt.Errorf("cannot fetch value in scan: %v\n", err)
			}
			if got != want {
				return fmt.Errorf("unexpected value in scan: got %q, want %q\n", got, want)
			}
		}

		if err := stream.Err(); err != nil {
			return fmt.Errorf("scan stream error: %v\n", err)
		}
	}
	return nil
}, "runVerifySyncgroupData")

//////////////////////////////////////////////
// Helpers for managing syncgroups

// Arguments: 0: Syncbase name, 1: syncgroup name, 2: prefixes, 3: mount table,
// 4 onwards: syncgroup permission blessings.
var runCreateSyncgroup = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	mtName := args[3]
	if mtName == "" {
		mtName = env.Vars[ref.EnvNamespacePrefix]
	}

	spec := wire.SyncgroupSpec{
		Description: "test syncgroup sg",
		Perms:       perms(args[4:]...),
		Prefixes:    toSgPrefixes(args[2]),
		MountTables: []string{mtName},
	}

	sg := d.Syncgroup(args[1])
	info := wire.SyncgroupMemberInfo{SyncPriority: 8}
	if err := sg.Create(ctx, spec, info); err != nil {
		return fmt.Errorf("Create SG %q failed: %v\n", args[1], err)
	}
	return nil
}, "runCreateSyncgroup")

func perms(bps ...string) access.Permissions {
	perms := access.Permissions{}
	for _, bp := range bps {
		for _, tag := range access.AllTypicalTags() {
			perms.Add(security.BlessingPattern(bp), string(tag))
		}
	}
	return perms
}

// toSgPrefixes converts, for example, "a:b,c:" to
// [{TableName: "a", Row: "b"}, {TableName: "c", Row: ""}].
func toSgPrefixes(csv string) []wire.TableRow {
	strs := strings.Split(csv, ",")
	res := make([]wire.TableRow, len(strs))
	for i, v := range strs {
		parts := strings.SplitN(v, ":", 2)
		if len(parts) != 2 {
			panic(fmt.Sprintf("invalid prefix string: %q", v))
		}
		res[i] = wire.TableRow{TableName: parts[0], Row: parts[1]}
	}
	return res
}

var runJoinSyncgroup = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	a := syncbase.NewService(args[0]).App("a")
	d := a.NoSQLDatabase("d", nil)

	sg := d.Syncgroup(args[1])
	info := wire.SyncgroupMemberInfo{SyncPriority: 10}
	if _, err := sg.Join(ctx, info); err != nil {
		return fmt.Errorf("Join SG %q failed: %v\n", args[1], err)
	}
	return nil
}, "runJoinSyncgroup")

// Arguments: 0: Syncbase name, 1: Syncgroup name, 2 onwards: Syncgroup permission blessings.
var runToggleSync = modules.Register(func(env *modules.Env, args ...string) error {
	ctx, shutdown := v23.Init()
	defer shutdown()

	serviceName, sgName, blessings := args[0], args[1], args[2:]

	a := syncbase.NewService(serviceName).App("a")
	d := a.NoSQLDatabase("d", nil)

	sg := d.Syncgroup(sgName)
	spec, ver, err := sg.GetSpec(ctx)
	if err != nil {
		return err
	}
	spec.Perms = perms(blessings...)

	return sg.SetSpec(ctx, spec, ver)
}, "runToggleSync")
