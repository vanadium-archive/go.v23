// Package syncbase defines the client API for a structured store that supports
// peer-to-peer synchronization.
//
// TODO(sadovsky): Write a detailed package description.
package syncbase

import (
	wire "v.io/syncbase/v23/services/syncbase"
	"v.io/v23/context"
	"v.io/v23/security/access"
	"v.io/v23/vdl"
)

// NOTE(sadovsky): Various methods below may end up needing additional options.
// One can add options to a Go method in a backwards-compatible way by making
// the method variadic.
//
// TODO(sadovsky): Consider changing *vdl.Value to interface{} everywhere.

// AccessController provides access control for various syncbase objects.
type AccessController interface {
	// SetPermissions replaces the current Permissions for an object.
	// For detailed documentation, see Object.SetPermissions.
	SetPermissions(ctx *context.T, acl access.Permissions, etag string) error

	// GetPermissions returns the current Permissions for an object.
	// For detailed documentation, see Object.GetPermissions.
	GetPermissions(ctx *context.T) (acl access.Permissions, etag string, err error)
}

// TODO(sadovsky): Is the terminology still "bind", or has it changed?
// TODO(sadovsky): Maybe move Create/Delete methods from children to parents?

// Service represents a Vanadium syncbase service.
// Use syncbase.BindService to get a Service.
type Service interface {
	// BindUniverse returns a Universe.
	// relativeName must not contain slashes.
	BindUniverse(relativeName string) Universe

	// ListUniverses returns a list of all Universe names.
	ListUniverses(ctx *context.T) ([]string, error)

	// SetPermissions and GetPermissions are included from the AccessController interface.
	AccessController
}

// Universe represents a collection of Databases.
// We expect there to be one Universe per app, likely created by the node
// manager as part of the app installation procedure.
type Universe interface {
	// Name returns the relative name of this Universe.
	Name() string

	// BindDatabase returns a Database.
	// relativeName must not contain slashes.
	BindDatabase(relativeName string) Database

	// ListDatabases returns a list of all Database names.
	ListDatabases(ctx *context.T) ([]string, error)

	// Create creates this Universe.
	// If acl is nil, the Permissions is inherited (copied) from the Service.
	// Create requires the caller to have Write permission at the Service.
	Create(ctx *context.T, acl access.Permissions) error

	// Delete deletes this Universe.
	Delete(ctx *context.T) error

	// SetPermissions and GetPermissions are included from the AccessController interface.
	AccessController
}

// Database represents a collection of Tables. Batch operations, queries, sync,
// watch, etc. all currently operate at the Database level. A Database's etag
// covers both its Permissions and its schema.
//
// TODO(sadovsky): Add Watch method.
// TODO(sadovsky): Support batch operations.
// TODO(sadovsky): Iterate on the schema management API as we figure out how to
// deal with schema versioning and sync.
type Database interface {
	// Name returns the relative name of this Database.
	Name() string

	// BindTable returns a Table.
	// relativeName must not contain slashes.
	BindTable(relativeName string) Table

	// Create creates this Database.
	// If acl is nil, the Permissions is inherited (copied) from the Universe.
	// Create requires the caller to have Write permission at the Universe.
	Create(ctx *context.T, acl access.Permissions) error

	// Delete deletes this Database.
	Delete(ctx *context.T) error

	// UpdateSchema updates the schema for this Database, creating and deleting
	// Tables under the hood as needed.
	UpdateSchema(ctx *context.T, schema wire.Schema, etag string) error

	// GetSchema returns the schema for this Database.
	GetSchema(ctx *context.T) (schema wire.Schema, etag string, err error)

	// SetPermissions and GetPermissions are included from the AccessController interface.
	AccessController
}

// Table represents a collection of Items (rows).
// All Permissions checks are performed against the Database Permissions.
//
// TODO(sadovsky): Add Scan method.
// TODO(sadovsky): Maybe generate a data access object (DAO) from the schema,
// have the DAO define a Key type, and have BindItem/Get/Delete take a Key.
// TODO(sadovsky): Currently we provide Get/Put/Delete methods on both Table and
// Item, because we're not sure which will feel more natural. Eventually, we'll
// need to pick one.
type Table interface {
	// Name returns the relative name of this Table.
	Name() string

	// BindItem returns an Item for the given primary key.
	BindItem(key *vdl.Value) Item

	// Get returns the value for the given primary key.
	Get(ctx *context.T, key *vdl.Value) (*vdl.Value, error)

	// Put writes the given value to this Table. The value's primary key field
	// must be set.
	Put(ctx *context.T, value *vdl.Value) error

	// Delete deletes the entry for the given primary key.
	Delete(ctx *context.T, key *vdl.Value) error
}

// Item represents a single row in a Table. The type of data stored in an Item
// is dictated by the Database schema.
// All Permissions checks are performed against the Database Permissions.
type Item interface {
	// Key returns the primary key for this Item.
	Key() *vdl.Value

	// Get returns the value for this Item.
	Get(ctx *context.T) (*vdl.Value, error)

	// Put writes the given value for this Item. The value's primary key field
	// must match Item.Key().
	Put(ctx *context.T, value *vdl.Value) error

	// Delete deletes this Item.
	Delete(ctx *context.T) error
}
