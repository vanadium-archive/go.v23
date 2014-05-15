// Package storage specifies a hierarchical structured data store with support for
// syncing and replication.  This is similar to a filesystem with files and
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
//   unique ID for the entry, and a value.  Entries can be accessed by ID by
//   using the pathname "/uid/<ID>".
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
//           Content string // Veyron name
//       }
//       type Dir struct {
//           Entries map[string]Name
//       }
//
//   The Name type is used for references.  Internally, the values are
//   represented with IDs.
//
// - Use Bind(path) to get an Entry in the store.  An Entry is just a reference,
//   it does not mean that there is a value associated with the Entry.  Use Put(v)
//   to store a value.
//
// - Use NewTransaction() to get a fresh transaction.  This is a package-level
//   function provided by the store implementation.  Transactions acquire a
//   snapshot of the state the first time they are used.  Transactions may or
//   may not span multiple store instances.  Usually, they will not, but see the
//   implementation's documentation.
//
// - Use Put(path, value) to add a value to the store at the specified path.
//   For example, here is how we can create a root directory containing a photo
//   album with two images.
//
//       t := NewTransaction()
//       d, err := st.Bind("/")
//       d.Put(t, &Dir{ID: 10})
//       y, err := st.Bind("/Entries/Yosemite")
//       y.Put(t, &Album{ID: 11, Title: "Yosemite 2013", Images: make(map[string]Name)})
//       img, err := st.Bind("/Entries/Yosemite/Images/A")
//       img.Put(t, &Image{ID: 37, Comment: "Waterfall", Content: "global/media/xxx.jpg"})
//       img, err : st.Bind("/Entries/Yosemite/Images/B")
//       img.Put(t, &Image{ID: 21, Comment: "Jason", Content: "global/media/yyy.jpg"})
//       t.Commit()
//
// - The fields of an entry are accessible using the path separator "/" in
//   pathnames.  For example, the following entries represent the photo album we
//   just created.
//
//       / = &Dir{ID: 10, Entries: map[string]Name{"Yosemite": 11}
//       Album{ID: 11, Title: "Yosemite 2013", Images: map[string]Name{"A": 37, "B": 21}}
//       Image{ID: 37, Comment: "Waterfall", Content: "global/media/xxx.jpg"}
//       Image{ID: 21, Comment: "Jason", Content: "global/media/yyy.jpg"}
//
//   Given this example, the pathnames resolve as follows:
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
// - Hard links are supported.  To add a hard link, use a Bind()/Put() to add a value with
//   each of its paths.
//
//      // Add a link to the image by date.
//      t := NewTransaction()
//      img1, err := st.Bind("/Entries/Yosemite/Images/A")
//      v, err := img1.Get(t)
//      img2, err := st.Bind("/Entries/Wallpaper")
//      img2.Put(t, v)
//      t.Commit()
//
//   The references form an arbitrary directed graph, posssibly cyclic.  Entries
//   are removed immediately when there are no more references to it.  The
//   "/uid/<ID>" names do not count as references.
//
// - Operations are transactional.  A transaction contains a consistent snapshot
//   of the current state, and a set of mutations to be applied to the store
//   atomically.  Transactions affect only the local store.
//
// - Search is supported, as well as standing queries.  The search language is
//   TBD.
//
//   Every entry value includes two pseudo-fields that can be used in searches.
//
//      $UID is the ID for the entry.
//      $NAMES is the set of paths that refer to the entry.
//
// - For sharing, data is replicated via replication groups.  Directories
//   are replicated by adding a ReplicationGroupAttr attribute.
//
//       type ReplicationGroupAttr struct {
//           Attr()
//           Name string // A Veyron name for the replication group.
//       }
//
//   The Name is a Veyron name like "/replication_groups/jyh/mydevices".  The
//   nameserver can be used to determine the reachable group members.
//
// - When stores that share a replication group can communicate, they can
//   perform a sync, with the goal that all directories in the replication group
//   have the same contents.
//
//     + any entry in one replication group is mirrored to all replicas.
//     + deletions in one replication group are propagated to all replicas.
//     + entries with the same ID represent the same value.
//
// - If an entry was modified concurrently in two replicas, the conflict
//   is detected, and the entry is passed to a conflict resolver.
//
//   The resolver is presented with three versions of the entry, including the
//   current versions, and a shared ancestor if there is one.
//
//   By "version," we mean a value of the entry at some point in time.  Each
//   time an entry is mutated, a new version is created.  Versions are
//   identified by the time they were created and a hash.  TODO(jyh): define the
//   versioning more precisely.
package storage

import (
	"time"

	"veyron2/query"
	"veyron2/services/watch"
)

// Watcher allows a client to receive updates for changes to objects
// that match a query.  See the package comments for details.
// TODO(tilaks): if we build a suite of client-side libraries, move Watcher
// into a shared location, such as veyron2/watch.
type Watcher interface {
	// Watch returns a stream of changes.
	Watch(req watch.Request) (watch.WatcherWatchStream, error)
}

// Object is the interface for a value in the store.
type Object interface {
	// Exists returns true iff the Entry has a value.
	Exists(t Transaction) (bool, error)

	// Get returns the value for the Object.  The value returned is from the
	// most recent mutation of the entry in the Transaction, or from the
	// Transaction's snapshot if there is no mutation.
	Get(t Transaction) (Entry, error)

	// Put adds or modifies the Object.  If there is no current value, the
	// object is created with default attributes.  It is legal to update a
	// subfield of a value.  Returns the updated *Stat of the store value.  If
	// putting a subfield, the *Stat is for the enclosing store value.
	Put(t Transaction, v interface{}) (Stat, error)

	// Remove removes the Object.
	Remove(t Transaction) error

	// SetAttr changes the attributes of the entry, such as permissions and
	// replication groups.  Attributes are associated with the value, not the
	// path.
	SetAttr(t Transaction, attrs ...Attr) error

	// Stat returns entry info.
	Stat(t Transaction) (Stat, error)

	// Query returns an Iterator to a sequence of elements that satisfy the
	// query.
	Query(t Transaction, q query.Query) Iterator

	// GlobT returns a set of names that match the glob pattern.
	// This is the same as Glob, but takes a transaction.
	GlobT(t Transaction, pattern string) (GlobStream, error)
}

// Transaction is an atomic state mutation.
//
// Each Transaction contains a snapshot of the committed state of the store and
// a set of mutations (created by Set and Delete operations).  The snapshot is
// taken when the Transaction is created, so it contains the state of the store
// as it was at some (hopefully recent) point in time.  This is used to support
// read-modify-write operations where reads refer to a consistent snapshot of
// the state.
type Transaction interface {
	// Commit commits the changes (the Set and Delete operations) in the
	// transaction to the store.  The operation is atomic, so all Set/Delete
	// operations are performed, or none.  Returns an error if the transaction
	// aborted.
	//
	// The Transaction should be discarded once Commit is called.  It can no
	// longer be used.
	Commit() error

	// Abort discards a transaction.  This is an optimization; transactions
	// eventually time out and get discarded.  However, live transactions
	// consume resources, so it is good practice to clean up.
	Abort() error
}

// TransactionOpt represents the options for creating transactions.
type TransactionOpt interface {
	TransactionOpt()
}

// Store is the client interface to the storage system.
type Store interface {
	Watcher

	// Bind returns the Object associated with a path.
	Bind(path string) Object

	// SetConflictResolver specifies a function to perform conflict resolution.
	// The <ty> represents the IDL name for the type.
	SetConflictResolver(ty string, r ConflictResolver)

	// Close closes the Store.
	Close() error
}

// Entry contains the metadata and data for an entry in the store.
type Entry struct {
	// Stat is the entry's metadata.
	Stat Stat

	// Value is the value of the entry.
	Value interface{}
}

// Iterator is an iterator that refers to a sequence of entries.
type Iterator interface {
	// IsValid returns true iff the Iterator refers to a valid entry,
	// so Name and Value return valid results.
	IsValid() bool

	// Value returns the value of the current entry.
	Value() interface{}

	// Next moves to the next entry.
	Next()
}

// GlobStream is the interface for streaming responses from the Glob method.
type GlobStream interface {
	// Recv returns the next item in the input stream, blocking until
	// an item is available.  Returns io.EOF to indicate graceful end of input.
	Recv() (item string, err error)

	// Finish closes the stream and returns the positional return values for
	// call.
	Finish() (err error)

	// Cancel cancels the RPC, notifying the server to stop processing.
	Cancel()
}

// Stat provides information about an entry in the store.
//
// TODO(jyh): Specify versioning more precisely.
type Stat struct {
	// ID is the unique identifier of the entry.
	ID ID

	// MTime is the last modification time.
	MTime time.Time

	// Attrs are the attributes associated with the entry.
	Attrs []Attr
}

// Attr represents attributes associated with an entry.
//
// TODO: define attributes for,
//    1. security properties
//    2. versions
type Attr interface {
	Attr()
}

// ReplicationGroupAttr specifies a replication group.  The Name is thre Veyron
// name of the replication group.
type ReplicationGroupAttr struct {
	Attr

	Name string
}

// ConflictResolver is a function that performs a conflict resolution.  <remote>
// is a remote value, <local> is the local value, and <root> is a common
// ancestor (or nil if the common ancestory can't be determined).
//
// A ConflictResolver function is expected to handle values of a single type
// (see the SetConflictResolver method above).
type ConflictResolver func(local, remote, root Entry) interface{}
