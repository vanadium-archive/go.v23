// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	wire "v.io/syncbase/v23/services/syncbase"
	"v.io/syncbase/v23/syncbase/util"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security/access"
)

func NewService(name string) Service {
	return &service{wire.ServiceClient(name), name}
}

type service struct {
	c    wire.ServiceClientMethods
	name string
}

var _ Service = (*service)(nil)

// TODO(sadovsky): Validate names before sending RPCs.

// FullName implements Service.FullName.
func (s *service) FullName() string {
	return s.name
}

// App implements Service.App.
func (s *service) App(relativeName string) App {
	name := naming.Join(s.name, relativeName)
	return &app{wire.AppClient(name), name, relativeName}
}

// ListApps implements Service.ListApps.
func (s *service) ListApps(ctx *context.T) ([]string, error) {
	return util.List(ctx, s.name)
}

// SetPermissions implements Service.SetPermissions.
func (s *service) SetPermissions(ctx *context.T, perms access.Permissions, version string) error {
	return s.c.SetPermissions(ctx, perms, version)
}

// GetPermissions implements Service.GetPermissions.
func (s *service) GetPermissions(ctx *context.T) (perms access.Permissions, version string, err error) {
	return s.c.GetPermissions(ctx)
}
