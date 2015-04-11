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

type table struct {
	c            wire.TableClientMethods
	name         string
	relativeName string
}

var _ Table = (*table)(nil)

// TODO(sadovsky): Validate names before sending RPCs.

// Name implements Table.Name.
func (t *table) Name() string {
	return t.relativeName
}

// Row implements Table.Row.
func (t *table) Row(key string) Row {
	// TODO(sadovsky): Escape delimiters in key?
	name := naming.Join(t.name, key)
	return &row{wire.RowClient(name), name, key}
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
	return t.c.Delete(ctx, r.Start(), r.Limit())
}

// Scan implements Table.Scan.
func (t *table) Scan(ctx *context.T, r RowRange) (Stream, error) {
	// TODO(sadovsky): Implement.
	return nil, nil
}

// SetPermissions implements Table.SetPermissions.
func (t *table) SetPermissions(ctx *context.T, prefix PrefixRange, perms access.Permissions) error {
	// TODO(sadovsky): Implement.
	return nil
}

// GetPermissions implements Table.GetPermissions.
func (t *table) GetPermissions(ctx *context.T, key string) ([]PrefixPermissions, error) {
	// TODO(sadovsky): Implement.
	return nil, nil
}

// DeletePermissions implements Table.DeletePermissions.
func (t *table) DeletePermissions(ctx *context.T, prefix PrefixRange) error {
	// TODO(sadovsky): Implement.
	return nil
}
