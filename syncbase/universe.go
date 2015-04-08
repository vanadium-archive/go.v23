// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package syncbase

import (
	wire "v.io/syncbase/v23/services/syncbase"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security/access"
)

type universe struct {
	c            wire.UniverseClientMethods
	name         string
	relativeName string
}

var _ Universe = (*universe)(nil)

// Name implements Universe.Name.
func (u *universe) Name() string {
	return u.relativeName
}

// BindDatabase implements Universe.BindDatabase.
func (u *universe) BindDatabase(relativeName string) Database {
	name := naming.Join(u.name, relativeName)
	return &database{wire.DatabaseClient(name), name, relativeName}
}

// ListDatabases implements Universe.ListDatabases.
func (u *universe) ListDatabases(ctx *context.T) ([]string, error) {
	// TODO(sadovsky): Implement on top of Glob.
	return nil, nil
}

// Create implements Universe.Create.
func (u *universe) Create(ctx *context.T, acl access.Permissions) error {
	return u.c.Create(ctx, acl)
}

// Delete implements Universe.Delete.
func (u *universe) Delete(ctx *context.T) error {
	return u.c.Delete(ctx)
}

// SetPermissions implements Universe.SetPermissions.
func (u *universe) SetPermissions(ctx *context.T, acl access.Permissions, etag string) error {
	return u.c.SetPermissions(ctx, acl, etag)
}

// GetPermissions implements Universe.GetPermissions.
func (u *universe) GetPermissions(ctx *context.T) (acl access.Permissions, etag string, err error) {
	return u.c.GetPermissions(ctx)
}
