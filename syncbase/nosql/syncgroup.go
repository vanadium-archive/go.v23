// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	wire "v.io/syncbase/v23/services/syncbase/nosql"
	"v.io/v23/context"
)

var (
	_ SyncGroup = (*syncgroup)(nil)
)

type syncgroup struct {
	c    wire.DatabaseClientMethods
	name string // Globally unique SyncGroup name
}

func newSyncGroup(dbName, sgName string) SyncGroup {
	return &syncgroup{
		c:    wire.DatabaseClient(dbName),
		name: sgName,
	}
}

// Create implements SyncGroup.Create.
func (sg *syncgroup) Create(ctx *context.T, spec wire.SyncGroupSpec, myInfo wire.SyncGroupMemberInfo) error {
	return sg.c.CreateSyncGroup(ctx, sg.name, spec, myInfo)
}

// Join implements SyncGroup.Join.
func (sg *syncgroup) Join(ctx *context.T, myInfo wire.SyncGroupMemberInfo) (wire.SyncGroupSpec, error) {
	return sg.c.JoinSyncGroup(ctx, sg.name, myInfo)
}

// Leave implements SyncGroup.Leave.
func (sg *syncgroup) Leave(ctx *context.T) error {
	return sg.c.LeaveSyncGroup(ctx, sg.name)
}

// Destroy implements SyncGroup.Destroy.
func (sg *syncgroup) Destroy(ctx *context.T) error {
	return sg.c.DestroySyncGroup(ctx, sg.name)
}

// Eject implements SyncGroup.Eject.
func (sg *syncgroup) Eject(ctx *context.T, member string) error {
	return sg.c.EjectFromSyncGroup(ctx, sg.name, member)
}

// GetSpec implements SyncGroup.GetSpec.
func (sg *syncgroup) GetSpec(ctx *context.T) (wire.SyncGroupSpec, string, error) {
	return sg.c.GetSyncGroupSpec(ctx, sg.name)
}

// SetSpec implements SyncGroup.SetSpec.
func (sg *syncgroup) SetSpec(ctx *context.T, spec wire.SyncGroupSpec, version string) error {
	return sg.c.SetSyncGroupSpec(ctx, sg.name, spec, version)
}

// GetMembers implements SyncGroup.GetMembers.
func (sg *syncgroup) GetMembers(ctx *context.T) (map[string]wire.SyncGroupMemberInfo, error) {
	return sg.c.GetSyncGroupMembers(ctx, sg.name)
}
