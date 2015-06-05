// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package nosql defines the client API for the NoSQL part of Syncbase.
package nosql

import (
	wire "v.io/syncbase/v23/services/syncbase/nosql"
	"v.io/syncbase/v23/syncbase/util"
	"v.io/v23/context"
	"v.io/v23/security/access"
)

// TODO(sadovsky): Document the access control policy for every method where
// it's not obvious.

// DatabaseHandle is the set of methods that work both with and without a batch.
// It allows clients to pass the handle to helper methods that are
// batch-agnostic.
type DatabaseHandle interface {
	// Name returns the relative name of this DatabaseHandle.
	Name() string

	// FullName returns the full name (object name) of this DatabaseHandle.
	FullName() string

	// Table returns the Table with the given name.
	// relativeName must not contain slashes.
	Table(relativeName string) Table

	// ListTables returns a list of all Table names.
	ListTables(ctx *context.T) ([]string, error)
}

// Database represents a collection of Tables. Batches, queries, sync, watch,
// etc. all operate at the Database level.
//
// TODO(sadovsky): Add Watch method.
type Database interface {
	DatabaseHandle

	// Create creates this Database.
	// If perms is nil, we inherit (copy) the App perms.
	Create(ctx *context.T, perms access.Permissions) error

	// Delete deletes this Database.
	Delete(ctx *context.T) error

	// Create creates the specified Table.
	// If perms is nil, we inherit (copy) the Database perms.
	// relativeName must not contain slashes.
	CreateTable(ctx *context.T, relativeName string, perms access.Permissions) error

	// DeleteTable deletes the specified Table.
	DeleteTable(ctx *context.T, relativeName string) error

	// BeginBatch creates a new batch. Instead of calling this function directly,
	// clients are recommended to use the RunInBatch() helper function, which
	// detects "concurrent batch" errors and handles retries internally.
	//
	// Default concurrency semantics:
	// - Reads inside a batch see a consistent snapshot, taken during
	//   BeginBatch(), and will not see the effect of writes inside the batch.
	// - Commit() may fail with ErrConcurrentBatch, indicating that after
	//   BeginBatch() but before Commit(), some concurrent routine wrote to a key
	//   that matches a key or row-range read inside this batch. (Writes inside a
	//   batch cannot cause that batch's Commit() to fail.)
	// - Other methods (e.g. Get) will never fail with error ErrConcurrentBatch,
	//   even if it is known that Commit() will fail with this error.
	//
	// Concurrency semantics can be configured using BatchOptions.
	// TODO(sadovsky): Maybe use varargs for options.
	BeginBatch(ctx *context.T, opts wire.BatchOptions) (BatchDatabase, error)

	// SetPermissions and GetPermissions are included from the AccessController
	// interface.
	util.AccessController

	// SyncGroup returns a handle to the SyncGroup with the given name.
	SyncGroup(sgName string) SyncGroup

	// GetSyncGroupNames returns the global names of all SyncGroups attached to
	// this database.
	GetSyncGroupNames(ctx *context.T) ([]string, error)
}

// BatchDatabase is a handle to a set of reads and writes to the database that
// should be considered an atomic unit. See BeginBatch() for concurrency
// semantics.
// TODO(sadovsky): If/when needed, add a CommitWillFail() method so that clients
// can avoid doing extra work inside a doomed batch.
type BatchDatabase interface {
	DatabaseHandle

	// Commit persists the pending changes to the database.
	Commit(ctx *context.T) error

	// Abort notifies the server that any pending changes can be discarded.
	// It is not strictly required, but it may allow the server to release locks
	// or other resources sooner than if it was not called.
	Abort(ctx *context.T)
}

// PrefixPermissions represents a pair of (prefix, perms).
type PrefixPermissions struct {
	Prefix PrefixRange
	Perms  access.Permissions
}

// Table represents a collection of Rows.
//
// TODO(sadovsky): Currently we provide Get/Put/Delete methods on both Table and
// Row, because we're not sure which will feel more natural. Eventually, we'll
// need to pick one.
type Table interface {
	// Name returns the relative name of this Table.
	Name() string

	// FullName returns the full name (object name) of this Table.
	FullName() string

	// Row returns the Row with the given primary key.
	Row(key string) Row

	// Get stores the value for the given primary key in value. If value's type
	// does not match the stored type, Get will return an error. Expected usage:
	//     var val mytype
	//     if err := table.Get(ctx, key, &val); err != nil {
	//       return err
	//     }
	Get(ctx *context.T, key string, value interface{}) error

	// Put writes the given value to this Table. The value's primary key field
	// must be set.
	// TODO(kash): Can VOM handle everything that satisfies interface{}?
	// Need to talk to Todd.
	// TODO(sadovsky): Maybe distinguish insert from update (and also offer
	// upsert) so that last-one-wins can have deletes trump updates.
	Put(ctx *context.T, key string, value interface{}) error

	// Delete deletes all rows in the given half-open range [start, limit). If
	// limit is "", all rows with keys >= start are included. If the last row that
	// is covered by a prefix from SetPermissions is deleted, that (prefix, perms)
	// pair is removed.
	// See helpers nosql.Prefix(), nosql.Range(), nosql.SingleRow().
	// TODO(sadovsky): Automatic GC interacts poorly with sync. Revisit this API.
	Delete(ctx *context.T, r RowRange) error

	// Scan returns all rows in the given half-open range [start, limit). If limit
	// is "", all rows with keys >= start are included. The returned stream reads
	// from a consistent snapshot taken at the time of the Scan RPC.
	// See helpers nosql.Prefix(), nosql.Range(), nosql.SingleRow().
	Scan(ctx *context.T, r RowRange) Stream

	// SetPermissions sets the permissions for all current and future rows with
	// the given prefix. If the prefix overlaps with an existing prefix, the
	// longest prefix that matches a row applies. For example:
	//     SetPermissions(ctx, Prefix("a/b"), perms1)
	//     SetPermissions(ctx, Prefix("a/b/c"), perms2)
	// The permissions for row "a/b/1" are perms1, and the permissions for row
	// "a/b/c/1" are perms2.
	//
	// SetPermissions will fail if called with a prefix that does not match any
	// rows.
	SetPermissions(ctx *context.T, prefix PrefixRange, perms access.Permissions) error

	// GetPermissions returns an array of (prefix, perms) pairs. The array is
	// sorted from longest prefix to shortest, so element zero is the one that
	// applies to the row with the given key. The last element is always the
	// prefix "" which represents the table's permissions -- the array will always
	// have at least one element.
	GetPermissions(ctx *context.T, key string) ([]PrefixPermissions, error)

	// DeletePermissions deletes the permissions for the specified prefix. Any
	// rows covered by this prefix will use the next longest prefix's permissions
	// (see the array returned by GetPermissions).
	DeletePermissions(ctx *context.T, prefix PrefixRange) error
}

// Row represents a single row in a Table.
type Row interface {
	// Key returns the primary key for this Row.
	Key() string

	// FullName returns the full name (object name) of this Row.
	FullName() string

	// Get returns the value for this Row.
	Get(ctx *context.T, value interface{}) error

	// Put writes the given value for this Row.
	Put(ctx *context.T, value interface{}) error

	// Delete deletes this Row.
	Delete(ctx *context.T) error
}

// Stream is an interface for iterating through a collection of key-value pairs.
type Stream interface {
	// Advance stages an element so the client can retrieve it with Key or Value.
	// Advance returns true iff there is an element to retrieve. The client must
	// call Advance before calling Key or Value. The client must call Cancel if it
	// does not iterate through all elements (i.e. until Advance returns false).
	// Advance may block if an element is not immediately available.
	Advance() bool

	// Key returns the key of the element that was staged by Advance.
	// Key may panic if Advance returned false or was not called at all.
	// Key does not block.
	Key() string

	// Value returns the value of the element that was staged by Advance, or an
	// error if the value could not be decoded.
	// Value may panic if Advance returned false or was not called at all.
	// Value does not block.
	Value(value interface{}) error

	// Err returns a non-nil error iff the stream encountered any errors. Err does
	// not block.
	Err() error

	// Cancel notifies the stream provider that it can stop producing elements.
	// The client must call Cancel if it does not iterate through all elements
	// (i.e. until Advance returns false). Cancel is idempotent and can be called
	// concurrently with a goroutine that is iterating via Advance/Key/Value.
	// Cancel causes Advance to subsequently return false. Cancel does not block.
	Cancel()
}

// SyncGroup is the interface for a SyncGroup in the store.
type SyncGroup interface {
	// Create creates a new SyncGroup with the given spec.
	//
	// Requires: Client must have at least Read access on the Database; prefix ACL
	// must exist at each SyncGroup prefix; Client must have at least Read access
	// on each of these prefix ACLs.
	Create(ctx *context.T, spec wire.SyncGroupSpec, myInfo wire.SyncGroupMemberInfo) error

	// Join joins a SyncGroup.
	//
	// Requires: Client must have at least Read access on the Database and on the
	// SyncGroup ACL.
	Join(ctx *context.T, myInfo wire.SyncGroupMemberInfo) (wire.SyncGroupSpec, error)

	// Leave leaves the SyncGroup. Previously synced data will continue
	// to be available.
	//
	// Requires: Client must have at least Read access on the Database.
	Leave(ctx *context.T) error

	// Destroy destroys the SyncGroup. Previously synced data will
	// continue to be available to all members.
	//
	// Requires: Client must have at least Read access on the Database, and must
	// have Admin access on the SyncGroup ACL.
	Destroy(ctx *context.T) error

	// Eject ejects a member from the SyncGroup. The ejected member
	// will not be able to sync further, but will retain any data it has already
	// synced.
	//
	// Requires: Client must have at least Read access on the Database, and must
	// have Admin access on the SyncGroup ACL.
	Eject(ctx *context.T, member string) error

	// GetSpec gets the SyncGroup spec. version allows for atomic
	// read-modify-write of the spec - see comment for SetSpec.
	//
	// Requires: Client must have at least Read access on the Database and on the
	// SyncGroup ACL.
	GetSpec(ctx *context.T) (spec wire.SyncGroupSpec, version string, err error)

	// SetSpec sets the SyncGroup spec. version may be either empty or
	// the value from a previous Get. If not empty, Set will only succeed if the
	// current version matches the specified one.
	//
	// Requires: Client must have at least Read access on the Database, and must
	// have Admin access on the SyncGroup ACL.
	SetSpec(ctx *context.T, spec wire.SyncGroupSpec, version string) error

	// GetMembers gets the info objects for members of the SyncGroup.
	//
	// Requires: Client must have at least Read access on the Database and on the
	// SyncGroup ACL.
	GetMembers(ctx *context.T) (map[string]wire.SyncGroupMemberInfo, error)
}
