// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	"v.io/v23/context"
	wire "v.io/v23/services/syncbase/nosql"
)

var (
	_ Syncgroup = (*syncgroup)(nil)
)

type syncgroup struct {
	c    wire.DatabaseClientMethods
	name string // globally unique syncgroup name
}

func newSyncgroup(dbName, sgName string) Syncgroup {
	return &syncgroup{
		c:    wire.DatabaseClient(dbName),
		name: sgName,
	}
}

// Create implements Syncgroup.Create.
func (sg *syncgroup) Create(ctx *context.T, spec wire.SyncgroupSpec, myInfo wire.SyncgroupMemberInfo) error {
	return sg.c.CreateSyncgroup(ctx, sg.name, spec, myInfo)
}

// Join implements Syncgroup.Join.
func (sg *syncgroup) Join(ctx *context.T, myInfo wire.SyncgroupMemberInfo) (wire.SyncgroupSpec, error) {
	return sg.c.JoinSyncgroup(ctx, sg.name, myInfo)
}

// Leave implements Syncgroup.Leave.
func (sg *syncgroup) Leave(ctx *context.T) error {
	return sg.c.LeaveSyncgroup(ctx, sg.name)
}

// Destroy implements Syncgroup.Destroy.
func (sg *syncgroup) Destroy(ctx *context.T) error {
	return sg.c.DestroySyncgroup(ctx, sg.name)
}

// Eject implements Syncgroup.Eject.
func (sg *syncgroup) Eject(ctx *context.T, member string) error {
	return sg.c.EjectFromSyncgroup(ctx, sg.name, member)
}

// GetSpec implements Syncgroup.GetSpec.
func (sg *syncgroup) GetSpec(ctx *context.T) (wire.SyncgroupSpec, string, error) {
	return sg.c.GetSyncgroupSpec(ctx, sg.name)
}

// SetSpec implements Syncgroup.SetSpec.
func (sg *syncgroup) SetSpec(ctx *context.T, spec wire.SyncgroupSpec, version string) error {
	return sg.c.SetSyncgroupSpec(ctx, sg.name, spec, version)
}

// GetMembers implements Syncgroup.GetMembers.
func (sg *syncgroup) GetMembers(ctx *context.T) (map[string]wire.SyncgroupMemberInfo, error) {
	return sg.c.GetSyncgroupMembers(ctx, sg.name)
}
