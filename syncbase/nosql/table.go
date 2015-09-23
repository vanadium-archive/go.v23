// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/syncbase/util"
)

func newTable(parentFullName, relativeName string, schemaVersion int32) Table {
	fullName := naming.Join(parentFullName, util.NameSep, relativeName)
	return &table{
		c:               wire.TableClient(fullName),
		fullName:        fullName,
		name:            relativeName,
		dbSchemaVersion: schemaVersion,
	}
}

type table struct {
	c               wire.TableClientMethods
	fullName        string
	name            string
	dbSchemaVersion int32
}

var _ Table = (*table)(nil)

// TODO(sadovsky): Validate names before sending RPCs.

// Name implements Table.Name.
func (t *table) Name() string {
	return t.name
}

// FullName implements Table.FullName.
func (t *table) FullName() string {
	return t.fullName
}

// Exists implements Table.Exists.
func (t *table) Exists(ctx *context.T) (bool, error) {
	return t.c.Exists(ctx, t.dbSchemaVersion)
}

// Create implements Table.Create.
func (t *table) Create(ctx *context.T, perms access.Permissions) error {
	return t.c.Create(ctx, t.dbSchemaVersion, perms)
}

// Destroy implements Table.Destroy.
func (t *table) Destroy(ctx *context.T) error {
	return t.c.Destroy(ctx, t.dbSchemaVersion)
}

// GetPermissions implements Table.GetPermissions.
func (t *table) GetPermissions(ctx *context.T) (access.Permissions, error) {
	return t.c.GetPermissions(ctx, t.dbSchemaVersion)
}

// SetPermissions implements Table.SetPermissions.
func (t *table) SetPermissions(ctx *context.T, perms access.Permissions) error {
	return t.c.SetPermissions(ctx, t.dbSchemaVersion, perms)
}

// Row implements Table.Row.
func (t *table) Row(key string) Row {
	return newRow(t.fullName, key, t.dbSchemaVersion)
}

// Get implements Table.Get.
func (t *table) Get(ctx *context.T, key string, value interface{}) error {
	return t.Row(key).Get(ctx, value)
}

// Put implements Table.Put.
func (t *table) Put(ctx *context.T, key string, value interface{}) error {
	return t.Row(key).Put(ctx, value)
}

// Delete implements Table.Delete.
func (t *table) Delete(ctx *context.T, key string) error {
	return t.Row(key).Delete(ctx)
}

// DeleteRange implements Table.DeleteRange.
func (t *table) DeleteRange(ctx *context.T, r RowRange) error {
	return t.c.DeleteRange(ctx, t.dbSchemaVersion, []byte(r.Start()), []byte(r.Limit()))
}

// Scan implements Table.Scan.
func (t *table) Scan(ctx *context.T, r RowRange) ScanStream {
	ctx, cancel := context.WithCancel(ctx)
	call, err := t.c.Scan(ctx, t.dbSchemaVersion, []byte(r.Start()), []byte(r.Limit()))
	if err != nil {
		return &InvalidScanStream{Error: err}
	}
	return newScanStream(cancel, call)
}

// GetPrefixPermissions implements Table.GetPrefixPermissions.
func (t *table) GetPrefixPermissions(ctx *context.T, key string) ([]PrefixPermissions, error) {
	wirePermsList, err := t.c.GetPrefixPermissions(ctx, t.dbSchemaVersion, key)
	permsList := []PrefixPermissions{}
	for _, v := range wirePermsList {
		permsList = append(permsList, PrefixPermissions{Prefix: Prefix(v.Prefix), Perms: v.Perms})
	}
	return permsList, err
}

// SetPrefixPermissions implements Table.SetPrefixPermissions.
func (t *table) SetPrefixPermissions(ctx *context.T, prefix PrefixRange, perms access.Permissions) error {
	return t.c.SetPrefixPermissions(ctx, t.dbSchemaVersion, prefix.Prefix(), perms)
}

// DeletePrefixPermissions implements Table.DeletePrefixPermissions.
func (t *table) DeletePrefixPermissions(ctx *context.T, prefix PrefixRange) error {
	return t.c.DeletePrefixPermissions(ctx, t.dbSchemaVersion, prefix.Prefix())
}
