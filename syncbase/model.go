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
	"v.io/syncbase/v23/syncbase/util"
	"v.io/v23/context"
	"v.io/v23/security/access"
)

// NOTE(sadovsky): Various methods below may end up needing additional options.
// One can add options to a Go method in a backwards-compatible way by making
// the method variadic.

// TODO(sadovsky): Document the access control policy for every method where
// it's not obvious.

// Service represents a Vanadium Syncbase service.
// Use syncbase.NewService to get a Service.
type Service interface {
	// FullName returns the full name (object name) of this Service.
	FullName() string

	// App returns the App with the given name.
	// relativeName must not contain slashes.
	App(relativeName string) App

	// ListApps returns a list of all App names.
	ListApps(ctx *context.T) ([]string, error)

	// SetPermissions and GetPermissions are included from the AccessController
	// interface.
	util.AccessController
}

// App represents the data for a specific app instance (possibly a combination
// of user, device, and app).
type App interface {
	// Name returns the relative name of this App.
	Name() string

	// FullName returns the full name (object name) of this App.
	FullName() string

	// NoSQLDatabase returns the nosql.Database with the given name.
	// relativeName must not contain slashes.
	NoSQLDatabase(relativeName string) nosql.Database

	// ListDatabases returns a list of all Database names.
	// TODO(kash): Include the database type (NoSQL vs. SQL).
	ListDatabases(ctx *context.T) ([]string, error)

	// Create creates this App.
	// If perms is nil, we inherit (copy) the Service perms.
	Create(ctx *context.T, perms access.Permissions) error

	// Delete deletes this App.
	Delete(ctx *context.T) error

	// SetPermissions and GetPermissions are included from the AccessController
	// interface.
	util.AccessController
}
