// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	wire "v.io/syncbase/v23/services/syncbase/nosql"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security/access"
)

func newTable(parentFullName, relativeName string) Table {
	fullName := naming.Join(parentFullName, relativeName)
	return &table{
		c:        wire.TableClient(fullName),
		fullName: fullName,
		name:     relativeName,
	}
}

type table struct {
	c        wire.TableClientMethods
	fullName string
	name     string
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
	return t.c.Exists(ctx)
}

// Row implements Table.Row.
func (t *table) Row(key string) Row {
	return newRow(t.fullName, key)
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
func (t *table) Delete(ctx *context.T, r RowRange) error {
	return t.c.DeleteRowRange(ctx, []byte(r.Start()), []byte(r.Limit()))
}

// Scan implements Table.Scan.
func (t *table) Scan(ctx *context.T, r RowRange) Stream {
	ctx, cancel := context.WithCancel(ctx)
	call, err := t.c.Scan(ctx, []byte(r.Start()), []byte(r.Limit()))
	if err != nil {
		return &InvalidStream{Error: err}
	}
	return newStream(cancel, call)
}

// GetPermissions implements Table.GetPermissions.
func (t *table) GetPermissions(ctx *context.T, key string) ([]PrefixPermissions, error) {
	wirePermsList, err := t.c.GetPermissions(ctx, key)
	permsList := []PrefixPermissions{}
	for _, v := range wirePermsList {
		permsList = append(permsList, PrefixPermissions{Prefix: Prefix(v.Prefix), Perms: v.Perms})
	}
	return permsList, err
}

// SetPermissions implements Table.SetPermissions.
func (t *table) SetPermissions(ctx *context.T, prefix PrefixRange, perms access.Permissions) error {
	return t.c.SetPermissions(ctx, prefix.Prefix(), perms)
}

// DeletePermissions implements Table.DeletePermissions.
func (t *table) DeletePermissions(ctx *context.T, prefix PrefixRange) error {
	return t.c.DeletePermissions(ctx, prefix.Prefix())
}
