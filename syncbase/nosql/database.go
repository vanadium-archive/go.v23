// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	wire "v.io/syncbase/v23/services/syncbase/nosql"
	"v.io/syncbase/v23/syncbase/util"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security/access"
	"v.io/v23/verror"
)

func NewDatabase(name, relativeName string) Database {
	return &database{wire.DatabaseClient(name), name, relativeName}
}

type database struct {
	c            wire.DatabaseClientMethods
	name         string
	relativeName string
}

var _ Database = (*database)(nil)

// TODO(sadovsky): Validate names before sending RPCs.

// Name implements Database.Name.
func (d *database) Name() string {
	return d.relativeName
}

// FullName implements Database.FullName.
func (d *database) FullName() string {
	return d.name
}

// Table implements Database.Table.
func (d *database) Table(relativeName string) Table {
	name := naming.Join(d.name, relativeName)
	return &table{wire.TableClient(name), name, relativeName}
}

// ListTables implements Database.ListTables.
func (d *database) ListTables(ctx *context.T) ([]string, error) {
	return util.List(ctx, d.name)
}

// Create implements Database.Create.
func (d *database) Create(ctx *context.T, perms access.Permissions) error {
	return d.c.Create(ctx, perms)
}

// Delete implements Database.Delete.
func (d *database) Delete(ctx *context.T) error {
	return d.c.Delete(ctx)
}

// CreateTable implements Database.CreateTable.
func (d *database) CreateTable(ctx *context.T, relativeName string, perms access.Permissions) error {
	return wire.TableClient(naming.Join(d.name, relativeName)).Create(ctx, perms)
}

// DeleteTable implements Database.DeleteTable.
func (d *database) DeleteTable(ctx *context.T, relativeName string) error {
	return wire.TableClient(naming.Join(d.name, relativeName)).Delete(ctx)
}

// BeginBatch implements Database.BeginBatch.
func (d *database) BeginBatch(ctx *context.T, opts BatchOptions) (BatchDatabase, error) {
	return nil, verror.NewErrNotImplemented(ctx)
}

// SetPermissions implements Database.SetPermissions.
func (d *database) SetPermissions(ctx *context.T, perms access.Permissions, version string) error {
	return d.c.SetPermissions(ctx, perms, version)
}

// GetPermissions implements Database.GetPermissions.
func (d *database) GetPermissions(ctx *context.T) (perms access.Permissions, version string, err error) {
	return d.c.GetPermissions(ctx)
}
