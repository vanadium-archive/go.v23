// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package nosql defines the client API for the NoSQL part of Syncbase.
package nosql

import (
	"time"

	"v.io/v23/context"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase/util"
	"v.io/v23/verror"
	"v.io/v23/vom"
)

// TODO(sadovsky): Document the access control policy for every method where
// it's not obvious.

// DatabaseHandle is the set of methods that work both with and without a batch.
// It allows clients to pass the handle to helper methods that are
// batch-agnostic.
type DatabaseHandle interface {
	// Name returns the relative name of this DatabaseHandle.
	Name() string

	// FullName returns the object name (escaped) of this DatabaseHandle.
	FullName() string

	// Table returns the Table with the given name.
	// relativeName must not contain slashes.
	Table(relativeName string) Table

	// ListTables returns a list of all Table names.
	ListTables(ctx *context.T) ([]string, error)

	// Exec executes a syncQL query.
	// A value must be provided for every positional parameter ('?' placeholder)
	// in the query.
	//
	// For select statements:
	// If no error is returned, Exec returns an array of headers (i.e., column
	// names) and a result stream which contains an array of values for each row
	// that matches the query.  The number of values returned in each row of the
	// result stream will match the size of the headers string array.
	//
	// For delete statements:
	// If no error is returned, Exec returns an array of headers with exactly
	// one column: "Count" and a result stream which contains an array of length
	// one, with a single element of type vdl.Int64.  The value represents
	// the number of k/v pairs deleted by the statement.
	//
	// Concurrency semantics: It is legal to perform writes concurrently with
	// Exec. The returned stream reads from a consistent snapshot taken at the
	// time of the RPC, and will not reflect subsequent writes to keys not yet
	// reached by the stream.
	Exec(ctx *context.T, query string, params ...interface{}) ([]string, ResultStream, error)

	// GetResumeMarker returns the ResumeMarker that points to the current end
	// of the event log.
	GetResumeMarker(ctx *context.T) (watch.ResumeMarker, error)

	// Close cleans up any state associated with this client handle including
	// shutting down any open conflict resolution stream.
	Close()
}

// Database represents a collection of Tables. Batches, queries, sync, watch,
// etc. all operate at the Database level.
type Database interface {
	DatabaseHandle

	// Create creates this Database.
	// If perms is nil, we inherit (copy) the App perms.
	Create(ctx *context.T, perms access.Permissions) error

	// Destroy destroys this Database, permanently removing all of its data.
	Destroy(ctx *context.T) error

	// Exists returns true only if this Database exists. Insufficient permissions
	// cause Exists to return false instead of an error.
	// TODO(ivanpi): Exists may fail with an error if higher levels of hierarchy
	// do not exist.
	Exists(ctx *context.T) (bool, error)

	// BeginBatch creates a new batch. Instead of calling this function directly,
	// clients are encouraged to use the RunInBatch() helper function, which
	// detects "concurrent batch" errors and handles retries internally.
	//
	// Default concurrency semantics:
	// - Reads (e.g. gets, scans) inside a batch operate over a consistent
	//   snapshot taken during BeginBatch(), and will see the effects of prior
	//   writes performed inside the batch.
	// - Commit() may fail with ErrConcurrentBatch, indicating that after
	//   BeginBatch() but before Commit(), some concurrent routine wrote to a key
	//   that matches a key or row-range read inside this batch.
	// - Other methods will never fail with error ErrConcurrentBatch, even if it
	//   is known that Commit() will fail with this error.
	//
	// Once a batch has been committed or aborted, subsequent method calls will
	// fail with no effect.
	//
	// Concurrency semantics can be configured using BatchOptions.
	// TODO(sadovsky): Maybe use varargs for options.
	BeginBatch(ctx *context.T, opts wire.BatchOptions) (BatchDatabase, error)

	// SetPermissions and GetPermissions are included from the AccessController
	// interface.
	util.AccessController

	// Watch allows a client to watch for updates to the database. For each watch
	// request, the client will receive a reliable stream of watch events without
	// re-ordering. See watch.GlobWatcher for a detailed explanation of the
	// behavior.
	//
	// If a nil ResumeMarker is provided, the WatchStream will begin with a Change
	// batch containing the initial state. Otherwise, the WatchStream will contain
	// only changes since the provided ResumeMarker.
	//
	// TODO(sadovsky): Watch should return just a WatchStream, similar to how Scan
	// returns just a ScanStream.
	Watch(ctx *context.T, table, prefix string, resumeMarker watch.ResumeMarker) (WatchStream, error)

	// Syncgroup returns a handle to the syncgroup with the given name.
	Syncgroup(sgName string) Syncgroup

	// GetSyncgroupNames returns the global names of all syncgroups attached to
	// this database.
	GetSyncgroupNames(ctx *context.T) ([]string, error)

	// CreateBlob returns a handle to the new Blob instantiated by Syncbase.
	CreateBlob(ctx *context.T) (Blob, error)

	// Blob returns a handle to the blob with the given BlobRef.
	Blob(br wire.BlobRef) Blob

	// EnforceSchema compares the current schema version of the database
	// with the schema version provided while creating this database handle and
	// updates the schema metadata if required.
	// This method also registers a conflict resolver with syncbase to receive
	// conflicts. Note: schema can be nil, in which case this method should not
	// be called and the caller is responsible for maintaining schema sanity.
	EnforceSchema(ctx *context.T) error

	// PauseSync pauses sync for this database. Incoming sync, as well as
	// outgoing sync of subsequent writes, will be disabled until ResumeSync
	// is called. PauseSync is idempotent.
	PauseSync(ctx *context.T) error

	// ResumeSync resumes sync for this database. ResumeSync is idempotent.
	ResumeSync(ctx *context.T) error
}

// BatchDatabase is a handle to a set of reads and writes to the database that
// should be considered an atomic unit. See BeginBatch() for concurrency
// semantics.
// TODO(sadovsky): If/when needed, add a CommitWillFail() method so that clients
// can avoid doing extra work inside a doomed batch.
type BatchDatabase interface {
	DatabaseHandle

	// Commit persists the pending changes to the database.
	// If the batch is readonly, Commit() will fail with ErrReadOnlyBatch; Abort()
	// should be used instead.
	Commit(ctx *context.T) error

	// Abort notifies the server that any pending changes can be discarded.
	// It is not strictly required, but it may allow the server to release locks
	// or other resources sooner than if it was not called.
	Abort(ctx *context.T) error
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

	// FullName returns the object name (escaped) of this Table.
	FullName() string

	// Exists returns true only if this Table exists. Insufficient permissions
	// cause Exists to return false instead of an error.
	// TODO(ivanpi): Exists may fail with an error if higher levels of hierarchy
	// do not exist.
	Exists(ctx *context.T) (bool, error)

	// Create creates this Table.
	// If perms is nil, we inherit (copy) the Database perms.
	// Create must not be called from within a batch.
	Create(ctx *context.T, perms access.Permissions) error

	// Destroy destroys this Table, permanently removing all of its data.
	// Destroy must not be called from within a batch.
	Destroy(ctx *context.T) error

	// GetPermissions returns the current Permissions for the Table.
	GetPermissions(ctx *context.T) (access.Permissions, error)

	// SetPermissions replaces the current Permissions for the Table.
	SetPermissions(ctx *context.T, perms access.Permissions) error

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

	// Delete deletes the row for the given primary key.
	Delete(ctx *context.T, key string) error

	// DeleteRange deletes all rows in the given half-open range [start, limit).
	// If limit is "", all rows with keys >= start are included.
	// TODO(sadovsky): Maybe add option to delete prefix perms fully covered by
	// the row range.
	// See helpers nosql.Prefix(), nosql.Range(), nosql.SingleRow().
	DeleteRange(ctx *context.T, r RowRange) error

	// Scan returns all rows in the given half-open range [start, limit). If limit
	// is "", all rows with keys >= start are included.
	// Concurrency semantics: It is legal to perform writes concurrently with
	// Scan. The returned stream reads from a consistent snapshot taken at the
	// time of the RPC (or at the time of BeginBatch, if in a batch), and will not
	// reflect subsequent writes to keys not yet reached by the stream.
	// See helpers nosql.Prefix(), nosql.Range(), nosql.SingleRow().
	Scan(ctx *context.T, r RowRange) ScanStream

	// GetPrefixPermissions returns an array of (prefix, perms) pairs. The array is
	// sorted from longest prefix to shortest, so element zero is the one that
	// applies to the row with the given key. The last element is always the
	// prefix "" which represents the table's permissions -- the array will always
	// have at least one element.
	GetPrefixPermissions(ctx *context.T, key string) ([]PrefixPermissions, error)

	// SetPrefixPermissions sets the permissions for all current and future rows with
	// the given prefix. If the prefix overlaps with an existing prefix, the
	// longest prefix that matches a row applies. For example:
	//     SetPrefixPermissions(ctx, Prefix("a/b"), perms1)
	//     SetPrefixPermissions(ctx, Prefix("a/b/c"), perms2)
	// The permissions for row "a/b/1" are perms1, and the permissions for row
	// "a/b/c/1" are perms2.
	SetPrefixPermissions(ctx *context.T, prefix PrefixRange, perms access.Permissions) error

	// DeletePrefixPermissions deletes the permissions for the specified prefix. Any
	// rows covered by this prefix will use the next longest prefix's permissions
	// (see the array returned by GetPrefixPermissions).
	DeletePrefixPermissions(ctx *context.T, prefix PrefixRange) error
}

// Row represents a single row in a Table.
type Row interface {
	// Key returns the primary key for this Row.
	Key() string

	// FullName returns the object name (escaped) of this Row.
	FullName() string

	// Exists returns true only if this Row exists. Insufficient permissions
	// cause Exists to return false instead of an error.
	// TODO(ivanpi): Exists may fail with an error if higher levels of hierarchy
	// do not exist.
	Exists(ctx *context.T) (bool, error)

	// Get returns the value for this Row.
	Get(ctx *context.T, value interface{}) error

	// Put writes the given value for this Row.
	Put(ctx *context.T, value interface{}) error

	// Delete deletes this Row.
	Delete(ctx *context.T) error
}

// Stream is an interface for iterating through a collection of elements.
type Stream interface {
	// Advance stages an element so the client can retrieve it. Advance returns
	// true iff there is an element to retrieve. The client must call Advance
	// before retrieving the element. The client must call Cancel if it does not
	// iterate through all elements (i.e. until Advance returns false).
	// Advance may block if an element is not immediately available.
	Advance() bool

	// Err returns a non-nil error iff the stream encountered any errors. Err does
	// not block.
	Err() error

	// Cancel notifies the stream provider that it can stop producing elements.
	// The client must call Cancel if it does not iterate through all elements
	// (i.e. until Advance returns false). Cancel is idempotent and can be called
	// concurrently with a goroutine that is iterating via Advance.
	// Cancel causes Advance to subsequently return false. Cancel does not block.
	Cancel()
}

// ScanStream is an interface for iterating through a collection of key-value
// pairs.
type ScanStream interface {
	Stream

	// Key returns the key of the element that was staged by Advance.
	// Key may panic if Advance returned false or was not called at all.
	// Key does not block.
	Key() string

	// Value returns the value of the element that was staged by Advance, or an
	// error if the value could not be decoded.
	// Value may panic if Advance returned false or was not called at all.
	// Value does not block.
	Value(value interface{}) error
}

// ResultStream is an interface for iterating through rows resulting from an
// Exec.
type ResultStream interface {
	Stream

	// Result returns the result that was staged by Advance.
	// Result may panic if Advance returned false or was not called at all.
	// Result does not block.
	Result() []*vom.RawBytes
}

// WatchStream is an interface for receiving database updates.
type WatchStream interface {
	Stream

	// Change returns the element that was staged by Advance.
	// Change may panic if Advance returned false or was not called at all.
	// Change does not block.
	Change() WatchChange
}

// ChangeType describes the type of the row change: Put or Delete.
// TODO(rogulenko): Add types to represent changes to ACLs and syncgroups in
// this database. Also, consider adding the Shell type.
type ChangeType uint32

const (
	PutChange ChangeType = iota
	DeleteChange
)

// WatchChange is the new value for a watched entity.
type WatchChange struct {
	// Table is the name of the table that contains the changed row.
	Table string

	// Row is the key of the changed row.
	Row string

	// ChangeType describes the type of the change. If the ChangeType equals to
	// PutChange, then the row exists in the table and the Value contains the new
	// value for this row. If the state equals to DeleteChange, then the row was
	// removed from the table.
	ChangeType ChangeType

	// ValueBytes is the new VOM-encoded value for the row if the ChangeType is
	// Put or nil otherwise.
	ValueBytes []byte

	// ResumeMarker provides a compact representation of all the messages
	// that have been received by the caller for the given Watch call.
	// This marker can be provided in the Request message to allow the caller
	// to resume the stream watching at a specific point without fetching the
	// initial state.
	ResumeMarker watch.ResumeMarker

	// FromSync indicates whether the change came from sync. If FromSync is
	// false, then the change originated from the local device.
	FromSync bool

	// If true, this WatchChange is followed by more WatchChanges that are in the
	// same batch as this WatchChange.
	Continued bool
}

// Value decodes the new value of the watched element. Panics if the change type
// is DeleteChange.
func (c *WatchChange) Value(value interface{}) error {
	if c.ChangeType == DeleteChange {
		panic("invalid change type")
	}
	return vom.Decode(c.ValueBytes, value)
}

// Syncgroup is the interface for a syncgroup in the store.
type Syncgroup interface {
	// Create creates a new syncgroup with the given spec.
	//
	// Requires: Client must have at least Read access on the Database; prefix ACL
	// must exist at each syncgroup prefix; Client must have at least Read access
	// on each of these prefix ACLs.
	Create(ctx *context.T, spec wire.SyncgroupSpec, myInfo wire.SyncgroupMemberInfo) error

	// Join joins a syncgroup.
	//
	// Requires: Client must have at least Read access on the Database and on the
	// syncgroup ACL.
	Join(ctx *context.T, myInfo wire.SyncgroupMemberInfo) (wire.SyncgroupSpec, error)

	// Leave leaves the syncgroup. Previously synced data will continue
	// to be available.
	//
	// Requires: Client must have at least Read access on the Database.
	Leave(ctx *context.T) error

	// Destroy destroys the syncgroup. Previously synced data will
	// continue to be available to all members.
	//
	// Requires: Client must have at least Read access on the Database, and must
	// have Admin access on the syncgroup ACL.
	Destroy(ctx *context.T) error

	// Eject ejects a member from the syncgroup. The ejected member
	// will not be able to sync further, but will retain any data it has already
	// synced.
	//
	// Requires: Client must have at least Read access on the Database, and must
	// have Admin access on the syncgroup ACL.
	Eject(ctx *context.T, member string) error

	// GetSpec gets the syncgroup spec. version allows for atomic
	// read-modify-write of the spec - see comment for SetSpec.
	//
	// Requires: Client must have at least Read access on the Database and on the
	// syncgroup ACL.
	GetSpec(ctx *context.T) (spec wire.SyncgroupSpec, version string, err error)

	// SetSpec sets the syncgroup spec. version may be either empty or
	// the value from a previous Get. If not empty, Set will only succeed if the
	// current version matches the specified one.
	//
	// Requires: Client must have at least Read access on the Database, and must
	// have Admin access on the syncgroup ACL.
	SetSpec(ctx *context.T, spec wire.SyncgroupSpec, version string) error

	// GetMembers gets the info objects for members of the syncgroup.
	//
	// Requires: Client must have at least Read access on the Database and on the
	// syncgroup ACL.
	GetMembers(ctx *context.T) (map[string]wire.SyncgroupMemberInfo, error)
}

// Blob is the interface for a Blob in the store.
type Blob interface {
	// Ref returns Syncbase's BlobRef for this blob.
	Ref() wire.BlobRef

	// Put appends the byte stream to the blob.
	Put(ctx *context.T) (BlobWriter, error)

	// Commit marks the blob as immutable.
	Commit(ctx *context.T) error

	// Size returns the count of bytes written as part of the blob
	// (committed or uncommitted).
	Size(ctx *context.T) (int64, error)

	// Delete locally deletes the blob (committed or uncommitted).
	Delete(ctx *context.T) error

	// Get returns the byte stream from a committed blob starting at offset.
	Get(ctx *context.T, offset int64) (BlobReader, error)

	// Fetch initiates fetching a blob if not locally found. priority
	// controls the network priority of the blob. Higher priority blobs are
	// fetched before the lower priority ones. However an ongoing blob
	// transfer is not interrupted. Status updates are streamed back to the
	// client as fetch is in progress.
	Fetch(ctx *context.T, priority uint64) (BlobStatus, error)

	// Pin locally pins the blob so that it is not evicted.
	Pin(ctx *context.T) error

	// Unpin locally unpins the blob so that it can be evicted if needed.
	Unpin(ctx *context.T) error

	// Keep locally caches the blob with the specified rank. Lower
	// ranked blobs are more eagerly evicted.
	Keep(ctx *context.T, rank uint64) error
}

// BlobWriter is an interface for putting a blob.
type BlobWriter interface {
	// Send places the bytes given by the client onto the output
	// stream. Returns errors encountered while sending. Blocks if there is
	// no buffer space.
	Send([]byte) error

	// Close indicates that no more bytes will be sent.
	Close() error
}

// BlobReader is an interface for getting a blob.
type BlobReader interface {
	// Advance() stages bytes so that they may be retrieved via
	// Value(). Returns true iff there are bytes to retrieve. Advance() must
	// be called before Value() is called. The caller is expected to read
	// until Advance() returns false, or to call Cancel().
	Advance() bool

	// Value() returns the bytes that were staged by Advance(). May panic if
	// Advance() returned false or was not called. Never blocks.
	Value() []byte

	// Err() returns any error encountered by Advance. Never blocks.
	Err() error

	// Cancel notifies the stream provider that it can stop producing
	// elements.  The client must call Cancel if it does not iterate through
	// all elements (i.e. until Advance returns false). Cancel is idempotent
	// and can be called concurrently with a goroutine that is iterating via
	// Advance.  Cancel causes Advance to subsequently return false. Cancel
	// does not block.
	Cancel()
}

// BlobStatus is an interface for getting the status of a blob transfer.
type BlobStatus interface {
	// Advance() stages an item so that it may be retrieved via
	// Value(). Returns true iff there are items to retrieve. Advance() must
	// be called before Value() is called. The caller is expected to read
	// until Advance() returns false, or to call Cancel().
	Advance() bool

	// Value() returns the item that was staged by Advance(). May panic if
	// Advance() returned false or was not called. Never blocks.
	Value() wire.BlobFetchStatus

	// Err() returns any error encountered by Advance. Never blocks.
	Err() error

	// Cancel notifies the stream provider that it can stop producing
	// elements.  The client must call Cancel if it does not iterate through
	// all elements (i.e. until Advance returns false). Cancel is idempotent
	// and can be called concurrently with a goroutine that is iterating via
	// Advance.  Cancel causes Advance to subsequently return false. Cancel
	// does not block.
	Cancel()
}

// Each database has a Schema associated with it which defines the current
// version of the database. When a new version of app wishes to change
// its data in a way that it is not compatible with the old app's data,
// the app must change the schema version and perform relevant upgrade logic.
// The conflict resolution rules are also associated with the
// schema version. Hence if the conflict resolution rules change then the schema
// version also must be bumped.
//
// Schema provides metadata and a ConflictResolver for a given database.
// ConflictResolver is purely local and not persisted.
type Schema struct {
	Metadata wire.SchemaMetadata
	Resolver ConflictResolver
}

// ConflictResolver interface allows the App to define resolution of conflicts
// that it requested to handle.
type ConflictResolver interface {
	OnConflict(ctx *context.T, conflict *Conflict) Resolution
}

// Get takes a reference to an instance of a type that is expected to be
// represented by Value.
func (v *Value) Get(value interface{}) error {
	return vom.Decode(v.Val, value)
}

// NewValue creates a new Value to be added to Resolution.
func NewValue(ctx *context.T, data interface{}) (*Value, error) {
	if data == nil {
		return nil, verror.New(verror.ErrBadArg, ctx, "data cannot be nil")
	}
	bytes, err := vom.Encode(data)
	if err != nil {
		return nil, err
	}
	return &Value{
		Val:       bytes,
		WriteTs:   time.Now(), // ignored by syncbase
		Selection: wire.ValueSelectionOther,
	}, nil
}
