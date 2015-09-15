// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"v.io/v23/context"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
)

func NewService(fullName string) Service {
	return &service{wire.ServiceClient(fullName), fullName}
}

type service struct {
	c        wire.ServiceClientMethods
	fullName string
}

var _ Service = (*service)(nil)

// TODO(sadovsky): Validate names before sending RPCs.

// FullName implements Service.FullName.
func (s *service) FullName() string {
	return s.fullName
}

// App implements Service.App.
func (s *service) App(relativeName string) App {
	return newApp(s.fullName, relativeName)
}

// ListApps implements Service.ListApps.
func (s *service) ListApps(ctx *context.T) ([]string, error) {
	return s.c.ListApps(ctx)
}

// SetPermissions implements Service.SetPermissions.
func (s *service) SetPermissions(ctx *context.T, perms access.Permissions, version string) error {
	return s.c.SetPermissions(ctx, perms, version)
}

// GetPermissions implements Service.GetPermissions.
func (s *service) GetPermissions(ctx *context.T) (perms access.Permissions, version string, err error) {
	return s.c.GetPermissions(ctx)
}
