// +build ignore

// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	wire "v.io/syncbase/v23/services/syncbase"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security/access"
)

// TODO(sadovsky): Maybe put this elsewhere.
func BindService(name string) Service {
	return &service{wire.ServiceClient(name), name}
}

type service struct {
	c    wire.ServiceClientMethods
	name string
}

var _ Service = (*service)(nil)

// BindUniverse implements Service.BindUniverse.
func (s *service) BindUniverse(relativeName string) Universe {
	name := naming.Join(s.name, relativeName)
	return &universe{wire.UniverseClient(name), name, relativeName}
}

// ListUniverses implements Service.ListUniverses.
func (s *service) ListUniverses(ctx *context.T) ([]string, error) {
	// TODO(sadovsky): Implement on top of Glob.
	return nil, nil
}

// SetPermissions implements Service.SetPermissions.
func (s *service) SetPermissions(ctx *context.T, acl access.Permissions, etag string) error {
	return s.c.SetPermissions(ctx, acl, etag)
}

// GetPermissions implements Service.GetPermissions.
func (s *service) GetPermissions(ctx *context.T) (acl access.Permissions, etag string, err error) {
	return s.c.GetPermissions(ctx)
}
