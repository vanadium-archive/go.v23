// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	"v.io/v23/context"
	"v.io/v23/security/access"
)

// DatabaseHandle is the set of methods that work both with and without a batch.
// It allows clients to pass the handle to helper methods that are
// batch-agnostic.
type DatabaseHandle interface {
	// BindTable returns a Table.
	// relativeName must not contain slashes.
	BindTable(relativeName string) Table
}

// BatchOptions configures a batch.
// TODO(sadovsky): Add more options, e.g. to configure isolation, whether to
// track the read set, etc.
type BatchOptions struct {
	// Arbitrary string, typically used to describe the intent behind a batch.
	// Hints are surfaced to clients during conflict resolution.
	Hint string

	// FailEagerly specifies whether individual operations inside a batch should
	// fail (with error ErrConcurrentBatch) if some other batch has committed rows
	// that conflict with rows touched by this batch. If false, only Commit() can
	// fail with ErrConcurrentBatch.
	// Typically, this option should be set to false for read-only batches and to
	// true for read-write batches.
	FailEagerly bool
}

// Database represents a collection of Tables. Batches, queries, sync, watch,
// etc. all operate at the Database level.
//
// TODO(sadovsky): Add Watch method.
type Database interface {
	DatabaseHandle

	// Name returns the relative name of this Database.
	Name() string

	// Create creates the specified Table.
	// If acl is nil, Permissions is inherited (copied) from the Database.
	// Create requires the caller to have Write permission at the Database.
	// relativeName must not contain slashes.
	CreateTable(ctx *context.T, relativeName string, acl access.Permissions) error

	// DeleteTable deletes the specified Table.
	DeleteTable(ctx *context.T, relativeName string) error

	// Create creates this Database.
	// If acl is nil, Permissions is inherited (copied) from the App.
	// Create requires the caller to have Write permission at the App.
	Create(ctx *context.T, acl access.Permissions) error

	// Delete deletes this Database.
	Delete(ctx *context.T) error

	// BeginBatch creates a new batch. Instead of calling this function directly,
	// clients are recommended to use the RunInBatch() helper function, which
	// detects "concurrent batch" errors and handles retries internally.
	// TODO(kash): Document concurrency semantics.
	// TODO(sadovsky): Maybe use varargs for options.
	// TODO(sadovsky): Must this method return an error? (Can we defer the RPC
	// until the first read or write?)
	BeginBatch(ctx *context.T, opts BatchOptions) (BatchDatabase, error)

	// We can't reuse the AccessController interface from v23/syncbase/model.go
	// because that would introduce a circular dependency.

	// SetPermissions replaces the current ACL for an object.
	// For detailed documentation, see Object.SetPermissions.
	SetPermissions(ctx *context.T, acl access.Permissions, version string) error

	// GetPermissions returns the current ACL for an object.
	// For detailed documentation, see Object.GetPermissions.
	GetPermissions(ctx *context.T) (acl access.Permissions, version string, err error)
}

// BatchDatabase is a handle to a set of reads and writes to the database that
// should be considered an atomic unit.
// TODO(kash): Document concurrency semantics.
type BatchDatabase interface {
	DatabaseHandle

	// Abort notifies the server that any pending changes can be discarded.
	// It is not strictly required, but it may allow the server to release locks
	// or other resources sooner than if it was not called.
	Abort(ctx *context.T)

	// Commit persists the pending changes to the database.
	Commit(ctx *context.T) error
}

// PrefixPermissions represents a pair of (prefix, permissions).
type PrefixPermissions struct {
	prefix PrefixRange
	acl    access.Permissions
}

// Table represents a collection of Rows.
//
// TODO(sadovsky): Currently we provide Get/Put/Delete methods on both Table and
// Row, because we're not sure which will feel more natural. Eventually, we'll
// need to pick one.
type Table interface {
	// Name returns the relative name of this Table.
	Name() string

	// BindRow returns a Row for the given primary key.
	BindRow(key string) Row

	// Get stores the value for the given primary key in value. If value's type
	// does not match the stored type, Get will return an error.
	// Expected usage:
	//   var val mytype
	//   if err := table.Get(ctx, key, &val); err != nil {
	//     return err
	//   }
	Get(ctx *context.T, key string, value interface{}) error

	// Put writes the given value to this Table. The value's primary key field
	// must be set.
	// TODO(kash): Can VOM handle everything that satisfies interface{}?
	// Need to talk to Todd.
	Put(ctx *context.T, key string, value interface{}) error

	// Scan returns all rows in the given range. See helpers nosql.Prefix(),
	// nosql.Range(), nosql.SingleRow().
	Scan(ctx *context.T, r RowRange) (Stream, error)

	// Delete deletes all rows in the given range. See helpers nosql.Prefix(),
	// nosql.Range(), nosql.SingleRow(). If the last row that is covered by a
	// prefix from SetPermissions is deleted, that (prefix, Permissions) pair is
	// removed.
	// TODO(sadovsky): Automatic GC does not interact well with sync, especially
	// in the presence of fine-grained ACLs. Need to think this through.
	Delete(ctx *context.T, r RowRange) error

	// SetPermissions sets the access control for all current and future rows with
	// the given prefix. If the prefix overlaps with an existing prefix, the
	// longest prefix that matches a row applies. For example:
	//    SetPermissions(ctx, Prefix("a/b"), acl1)
	//    SetPermissions(ctx, Prefix("a/b/c"), acl2)
	// The permissions for row "a/b/1" are acl1, and the permissions for "a/b/c/1"
	// are acl2.
	//
	// SetPermissions will fail if called on a prefix that does not match any
	// rows.
	SetPermissions(ctx *context.T, prefix PrefixRange, acl access.Permissions) error

	// GetPermissions returns an array of (prefix, permissions) pairs. The array
	// is sorted from longest prefix to shortest, so element zero is the one that
	// applies to the row at 'key'. The last element is always the prefix "" which
	// represents the table's permissions -- the array will always have at least
	// one element.
	GetPermissions(ctx *context.T, key string) ([]PrefixPermissions, error)

	// DeletePermissions deletes the prefix-specific access control. The rows
	// covered by this prefix will use the next longest prefix's permissions (see
	// the array returned by GetPermissions).
	DeletePermissions(ctx *context.T, prefix PrefixRange) error
}

// Row represents a single row in a Table.
type Row interface {
	// Key returns the primary key for this Row.
	Key() string

	// Get returns the value for this Row.
	Get(ctx *context.T, value interface{}) error

	// Put writes the given value for this Row.
	Put(ctx *context.T, value interface{}) error

	// Delete deletes this Row.
	Delete(ctx *context.T) error
}

// KeyValue is a wrapper for the key and value from a single row.
type KeyValue struct {
	Key   string
	Value interface{}
}

type Stream interface {
	// Advance stages an element so the client can retrieve it with Value. Advance
	// returns true iff there is an element to retrieve. The client must call
	// Advance before calling Value. The client must call Cancel if it does not
	// iterate through all elements (i.e. until Advance returns false). Advance
	// may block if an element is not immediately available.
	Advance() bool
	// Value returns the element that was staged by Advance. Value may panic if
	// Advance returned false or was not called at all. Value does not block.
	Value() KeyValue
	// Err returns a non-nil error iff the stream encountered any errors. Err does
	// not block.
	Err() error
	// Cancel notifies the stream provider that it can stop producing elements.
	// The client must call Cancel if it does not iterate through all elements
	// (i.e. until Advance returns false). Cancel is idempotent and can be called
	// concurrently with a goroutine that is iterating via Advance/Value. Cancel
	// causes Advance to subsequently return false. Cancel does not block.
	Cancel()
}
