// Package storage specifies a hierarchical structured data store with support
// for syncing and replication.  This is similar to a filesystem with files and
// directories, but the "files" are not blobs, they are structured data with a
// schema specified in the IDL.
//
// See http://goto/veyron:local-store for a more complete description.
// Currently, that's a discussion doc, rather than a design doc.  Here is a
// shorter summary.
//
// - Each Veyron process/device has local storage.  All storage operations are
//   performed on the local storage.  Data can be replicated across stores.
//   Shared data can be synchronized when two or more stores sharing data are in
//   communication.
//
// - In general, there is no centralized authoritative store.  All store
//   instances are peers.  However, one or more of the peers can be hosted "in
//   the cloud" to improve availability, reliability, etc.
//
// - Each store entry has at least one pathname that refers to it, a globally
//   unique ID for the entry, and a value.
//
// - Values are structured, specified in the IDL.  Here is an example:
//
//       // photo.idl
//       type Album struct {
//           Title string
//           Images map[string]Name
//       }
//       type Image struct {
//           Comment string
//           Content string // Object name
//       }
//       type Dir struct {
//           Entries map[string]Name
//       }
//
//   The Name type is used for references.  Internally, the values are
//   represented with IDs.
//
// - Use store.Bind(path) to get an Object in the store.  An Object is just a
//   reference, it does not mean that there is a value associated with the
//   Object.  Use Put(v) to store a value.
//
// - Use store.NewTransaction(path) to create a transaction.
//   Transactions acquire a snapshot of the state the first time they are used,
//   and may not span multiple store instances.  All Objects that participate
//   in the transaction must have names that are relative to path.
//
// - Use Object.Put(value) to add a value to the store at the specified path.
//   For example, here is how we can create a root directory containing a photo
//   album with two images.  Error handling is omitted for brevity.
//
//       tx := st.NewTransaction(storeName)
//       d, err := tx.Bind("").Put(&Dir{})
//       y, err := tx.Bind("/Entries/Yosemite").Put(
//         &Album{Title: "Yosemite 2013", Images: make(map[string]Name)})
//       img, err := tx.Bind("/Entries/Yosemite/Images/A").Put(
//         &Image{Comment: "Waterfall", Content: "global/media/xxx.jpg"})
//       img, err = tx.Bind("/Entries/Yosemite/Images/B").Put(
//         &Image{Comment: "Jason", Content: "global/media/yyy.jpg"})
//       err := tx.Commit()
//
// - The fields of an entry are accessible using the path separator "/" in
//   pathnames.  Given this example, the pathnames resolve as follows:
//
//       /Entries/Yosemite = Album{Title: "Yosemite 2013", Images: map[string]Name{"A": 37, "B": 21}}
//       /Entries/Yosemite/Title = "Yosemite 2013"
//       /Entries/Yosemite/Images/A = Image{Comment: "Waterfall", Content: "global/media/xxx.jpg"}
//       /Entries/Yosemite/Images/B = Image{Comment: "Jason", Content: "global/media/yyy.jpg"}
//       /Entries/Yosemite/Images/A/Comment = "Waterfall"
//       /Entries/Yosemite/Images/A/Content = "global/media/xxx.jpg"
//       /Entries/Yosemite/Images/B/Comment = "Jason"
//       /Entries/Yosemite/Images/B/Content = "global/media/yyy.jpg"
//
// - Hard links are supported.  To add a hard link, use a Bind() and Put() to add a
//   value with each of its paths.
//
//       // Set an existing image as the Wallpaper.
//       tx := st.NewTransaction(storeName)
//       img1, err := tx.Bind("/Entries/Yosemite/Images/A").Get()
//       stat, err := tx.Bind("/Entries/Wallpaper").Put(img1.Stat.ID)
//       err := tx.Commit()
//
//   The references form an arbitrary directed graph, posssibly cyclic.  Entries
//   are removed immediately when there are no more references to them.
//
// - Operations are transactional.  A transaction contains a consistent snapshot
//   of the current state, and a set of mutations to be applied to the store
//   atomically.  Transactions affect only the local store.
//
// - Complex queries are supported.  See http://go/vql for details.
//
// - Replication is a work in progress.
package storage

import (
	"veyron2/context"
	"veyron2/query"
	"veyron2/services/watch"
	"veyron2/services/watch/types"
)

// Store is the client interface to the storage system.
type Store interface {
	// Bind returns the Object associated with a name for use outside of any
	// transaction.
	Bind(name string) Object

	// NewTransaction creates a transaction that is rooted at a given name
	// in the store.  All objects to be used within the transaction must
	// be addressed with a name relative to the given name.
	NewTransaction(ctx context.T, name string, opts ...TransactionOpt) Transaction
}

// TransactionOpt represents the options for creating transactions.
// This is just a placeholder as there are currently no such options.
type TransactionOpt interface {
	TransactionOpt()
}

// Transaction is an atomic state mutation.
//
// Each Transaction contains a snapshot of the committed state of the store and
// a set of mutations (created by Put and Remove operations).  The snapshot is
// taken when the Transaction is created, so it contains the state of the store
// as it was at some (hopefully recent) point in time.  This is used to support
// read-modify-write operations where reads refer to a consistent snapshot of
// the state.
type Transaction interface {
	// Bind returns an object whose name is relative to the transaction's name.
	Bind(relativeName string) Object

	// Commit commits the changes (the Put and Remove operations) in the
	// transaction to the store.  The operation is atomic, so all Put/Remove
	// operations are performed, or none.  Returns an error if the transaction
	// aborted.
	//
	// The Transaction should be discarded once Commit is called.  It can no
	// longer be used.
	Commit(ctx context.T) error

	// Abort discards a transaction.  This is an optimization; transactions
	// eventually time out and get discarded.  However, live transactions
	// consume resources, so it is good practice to clean up.
	Abort(ctx context.T) error
}

// GlobWatcher allows a client to receive updates for changes to objects
// that match a pattern.  See the package comments for details.
// TODO(tilaks): If we build a suite of client-side libraries, move GlobWatcher
// into a shared location, such as veyron2/watch.
type GlobWatcher interface {
	// WatchGlob returns a stream of changes.
	WatchGlob(ctx context.T, req types.GlobRequest) (watch.GlobWatcherWatchGlobCall, error)
}

// QueryWatcher allows a client to receive updates for changes to objects
// that match a query.  See the package comments for details.
// TODO(tilaks): If we build a suite of client-side libraries, move QueryWatcher
// into a shared location, such as veyron2/watch.
type QueryWatcher interface {
	// WatchQuery returns a stream of changes.
	WatchQuery(ctx context.T, req types.QueryRequest) (watch.QueryWatcherWatchQueryCall, error)
}

// Object is the interface for a value in the store.
type Object interface {
	GlobWatcher
	QueryWatcher

	// Bind returns an object whose name is relative to this object's name.
	Bind(relativeName string) Object

	// Exists returns true iff the Entry has a value.
	Exists(ctx context.T) (bool, error)

	// Get returns the value for the Object.  The value returned is from the
	// most recent mutation of the entry in the Transaction, or from the
	// Transaction's snapshot if there is no mutation.
	Get(ctx context.T) (Entry, error)

	// Put adds or modifies the Object.  If there is no current value, the
	// object is created with default attributes.  It is legal to update a
	// subfield of a value.  Returns the updated *Stat of the store value.  If
	// putting a subfield, the *Stat is for the enclosing store value.
	Put(ctx context.T, v interface{}) (Stat, error)

	// Remove removes the Object.
	Remove(ctx context.T) error

	// Stat returns entry info.
	Stat(ctx context.T) (Stat, error)

	// Query returns entries matching the given query.
	Query(ctx context.T, q query.Query) QueryStream

	// Glob returns names matching the given pattern.
	Glob(ctx context.T, pattern string) GlobCall
}

// QueryStream provides a stream of query results.  Typical usage:
//   for stream.Advance() {
//     result := stream.Value()
//     // Process result.
//     if enough_results {
//       stream.Cancel()
//       break
//     }
//   }
//   if stream.Err() != nil {
//     return stream.Err()
//   }
type QueryStream interface {
	// Advance stages an element so the client can retrieve it with Value.
	// Advance returns true iff there is an element to retrieve.  The client must
	// call Advance before calling Value.  The client must call Cancel if it does
	// not iterate through all elements (i.e. until Advance returns false).
	// Advance may block if an element is not immediately available.
	Advance() bool

	// Value returns the element that was staged by Advance.  Value may panic if
	// Advance returned false or was not called at all.  Value does not block.
	Value() QueryResult

	// Err returns a non-nil error iff the stream encountered any errors.  Err
	// does not block.
	Err() error

	// Cancel notifies the stream provider that it can stop producing elements.
	// The client must call Cancel if it does not iterate through all elements
	// (i.e. until Advance returns false).  Cancel is idempotent and can be called
	// concurrently with a goroutine that is iterating via Advance/Value.  Cancel
	// causes Advance to subsequently return false.  Cancel does not block.
	Cancel()
}

// QueryResult is a single result of a query.  Because a query might
// produce a dynamically typed value via the selection operator, a
// QueryResult contains either a Value or a map of Fields.
type QueryResult interface {
	// Name returns the name of the object relative to the query root.
	Name() string

	// Value will return a non-nil value if this query result is of a known
	// type.
	Value() interface{}

	// Fields will return a non-nil map if this query result is of a dynamic
	// type specified by the selection operator. The keys will be the names used
	// in the selection.  If a field represents a subquery, the value will be
	// a QueryStream.
	Fields() map[string]interface{}
}

// TODO(kash): We don't need both GlobStream and GlobCall.  We can combine the
// two Err methods and move Cancel to GlobStream.

// GlobStream is the interface for streaming responses from the Glob method.
type GlobStream interface {
	// Advance stages an element so the client can retrieve it with Value.
	// Advance returns true iff there is an element to retrieve.  The client must
	// call Advance before calling Value.  The client must call Cancel if it does
	// not iterate through all elements (i.e. until Advance returns false).
	// Advance may block if an element is not immediately available.
	Advance() bool

	// Value returns the element that was staged by Advance.  Value may panic if
	// Advance returned false or was not called at all.  Value does not block.
	Value() string

	// Err returns a non-nil error iff the stream encountered
	// any errors.  Err does not block.
	Err() error
}

// GlobRPC is the interface for the RPC handle for the streaming responses for Glob
type GlobCall interface {
	RecvStream() GlobStream

	// Err returns a non-nil error iff the stream encountered any errors.  Err
	// does not block.
	Err() error

	// Cancel notifies the stream provider that it can stop producing elements.
	// The client must call Cancel if it does not iterate through all elements
	// (i.e. until Advance returns false).  Cancel is idempotent and can be called
	// concurrently with a goroutine that is iterating via Advance/Value.  Cancel
	// causes Advance to subsequently return false.  Cancel does not block.
	Cancel()
}
