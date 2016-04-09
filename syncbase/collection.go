// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase/util"
)

func newCollection(parentFullName, relativeName string, bh wire.BatchHandle) Collection {
	// Encode relativeName so that any forward slashes get dropped, thus ensuring
	// that the server will interpret fullName as referring to a collection
	// object. Note that the server will still reject this name if
	// util.ValidCollectionName returns false.
	fullName := naming.Join(parentFullName, util.Encode(relativeName))
	return &collection{
		c:        wire.CollectionClient(fullName),
		fullName: fullName,
		name:     relativeName,
		bh:       bh,
	}
}

type collection struct {
	c        wire.CollectionClientMethods
	fullName string
	name     string
	bh       wire.BatchHandle
}

var _ Collection = (*collection)(nil)

// Name implements Collection.Name.
func (c *collection) Name() string {
	return c.name
}

// FullName implements Collection.FullName.
func (c *collection) FullName() string {
	return c.fullName
}

// Exists implements Collection.Exists.
func (c *collection) Exists(ctx *context.T) (bool, error) {
	return c.c.Exists(ctx, c.bh)
}

// Create implements Collection.Create.
func (c *collection) Create(ctx *context.T, perms access.Permissions) error {
	return c.c.Create(ctx, c.bh, perms)
}

// Destroy implements Collection.Destroy.
func (c *collection) Destroy(ctx *context.T) error {
	return c.c.Destroy(ctx, c.bh)
}

// GetPermissions implements Collection.GetPermissions.
func (c *collection) GetPermissions(ctx *context.T) (access.Permissions, error) {
	return c.c.GetPermissions(ctx, c.bh)
}

// SetPermissions implements Collection.SetPermissions.
func (c *collection) SetPermissions(ctx *context.T, perms access.Permissions) error {
	return c.c.SetPermissions(ctx, c.bh, perms)
}

// Row implements Collection.Row.
func (c *collection) Row(key string) Row {
	return newRow(c.fullName, key, c.bh)
}

// Get implements Collection.Get.
func (c *collection) Get(ctx *context.T, key string, value interface{}) error {
	return c.Row(key).Get(ctx, value)
}

// Put implements Collection.Put.
func (c *collection) Put(ctx *context.T, key string, value interface{}) error {
	return c.Row(key).Put(ctx, value)
}

// Delete implements Collection.Delete.
func (c *collection) Delete(ctx *context.T, key string) error {
	return c.Row(key).Delete(ctx)
}

// DeleteRange implements Collection.DeleteRange.
func (c *collection) DeleteRange(ctx *context.T, r RowRange) error {
	return c.c.DeleteRange(ctx, c.bh, []byte(r.Start()), []byte(r.Limit()))
}

// Scan implements Collection.Scan.
func (c *collection) Scan(ctx *context.T, r RowRange) ScanStream {
	ctx, cancel := context.WithCancel(ctx)
	call, err := c.c.Scan(ctx, c.bh, []byte(r.Start()), []byte(r.Limit()))
	if err != nil {
		return &InvalidScanStream{Error: err}
	}
	return newScanStream(cancel, call)
}
