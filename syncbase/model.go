// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package syncbase defines the client API for a structured store that supports
// peer-to-peer synchronization.
//
// TODO(sadovsky): Write a detailed package description.
package syncbase

import (
	"v.io/syncbase/v23/syncbase/nosql"
	"v.io/v23/context"
	"v.io/v23/security/access"
)

// NOTE(sadovsky): Various methods below may end up needing additional options.
// One can add options to a Go method in a backwards-compatible way by making
// the method variadic.

// AccessController provides access control for various syncbase objects.
type AccessController interface {
	// SetPermissions replaces the current Permissions for an object.
	// For detailed documentation, see Object.SetPermissions.
	SetPermissions(ctx *context.T, acl access.Permissions, version string) error

	// GetPermissions returns the current Permissions for an object.
	// For detailed documentation, see Object.GetPermissions.
	GetPermissions(ctx *context.T) (acl access.Permissions, version string, err error)
}

// TODO(sadovsky): Is the terminology still "bind", or has it changed?

// Service represents a Vanadium syncbase service.
// Use syncbase.BindService to get a Service.
type Service interface {
	// BindApp returns an App.
	// relativeName must not contain slashes.
	BindApp(relativeName string) App

	// ListApps returns a list of all App names.
	ListApps(ctx *context.T) ([]string, error)

	// SetPermissions and GetPermissions are included from the AccessController
	// interface.
	AccessController
}

// App represents the data for a specific app instance (possibly a combination
// of user, device, and app).
// TODO(sadovsky): Figure out precisely what the App name would be under a
// normal Vanadium installation.
type App interface {
	// Name returns the relative name of this App.
	Name() string

	// BindNoSQLDatabase returns a nosql.Database.
	// relativeName must not contain slashes.
	BindNoSQLDatabase(relativeName string) nosql.Database

	// ListDatabases returns a list of all Database names.
	// TODO(kash): Include the database type (nosql vs. sql).
	ListDatabases(ctx *context.T) ([]string, error)

	// Create creates this App.
	// If acl is nil, Permissions is inherited (copied) from the Service.
	// Create requires the caller to have Write permission at the Service.
	Create(ctx *context.T, acl access.Permissions) error

	// Delete deletes this App.
	Delete(ctx *context.T) error

	// SetPermissions and GetPermissions are included from the AccessController
	// interface.
	AccessController
}
