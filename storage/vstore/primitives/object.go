package primitives

import (
	"errors"

	"veyron2"
	"veyron2/context"
	"veyron2/ipc"
	"veyron2/naming"
	"veyron2/query"
	"veyron2/services/store"
	"veyron2/services/watch"
	"veyron2/storage"
)

type object struct {
	serv store.Object
}

type errorObject struct {
	err error
}

var (
	ErrBadAttr = errors.New("bad attribute")

	_ storage.Object      = (*object)(nil)
	_ storage.Object      = (*errorObject)(nil)
	_ storage.QueryStream = (*errorQueryStream)(nil)
	_ storage.GlobStream  = (*errorGlobStream)(nil)

	nullEntry       storage.Entry
	nullStat        storage.Stat
	nullChangeBatch watch.ChangeBatch
)

// BindObject returns a storage.Object for a value at an Object name. This
// method always succeeds. If the Object name is not a value in a Veyron store,
// all subsequent operations on the Object will fail.
// TODO(sadovsky): Maybe just take a single name argument, so that the client
// need not know the store mount name?
func BindObject(mount, name string) storage.Object {
	serv, err := store.BindObject(naming.Join(mount, name))
	if err != nil {
		return &errorObject{err: err}
	}
	return &object{serv: serv}
}

// Exists returns true iff the Entry has a value.
func (o *object) Exists(ctx context.T) (bool, error) {
	return o.serv.Exists(ctx)
}

// Get returns the value for the Object.  The value returned is from the
// most recent mutation of the entry in the storage.Transaction, or from the
// storage.Transaction's snapshot if there is no mutation.
func (o *object) Get(ctx context.T) (storage.Entry, error) {
	return o.serv.Get(ctx)
}

// Put adds or modifies the Object.
func (o *object) Put(ctx context.T, v interface{}) (storage.Stat, error) {
	return o.serv.Put(ctx, v)
}

// Remove removes the Object.
func (o *object) Remove(ctx context.T) error {
	return o.serv.Remove(ctx)
}

// Stat returns entry info.
func (o *object) Stat(ctx context.T) (storage.Stat, error) {
	return o.serv.Stat(ctx)
}

// Query returns entries matching the given query.
func (o *object) Query(ctx context.T, q query.Query) storage.QueryStream {
	stream, err := o.serv.Query(ctx, q)
	if err != nil {
		return &errorQueryStream{err}
	}
	return newQueryStream(stream)
}

// Glob returns names matching the given pattern.
func (o *object) Glob(ctx context.T, pattern string) storage.GlobCall {
	stream, err := o.serv.Glob(ctx, pattern)
	if err != nil {
		return &errorGlobStream{err}
	}
	return newGlobStream(stream)
}

// WatchGlob returns a stream of changes that match a pattern.
func (o *object) WatchGlob(ctx context.T, req watch.GlobRequest) (watch.GlobWatcherWatchGlobCall, error) {
	return o.serv.WatchGlob(ctx, req, veyron2.CallTimeout(ipc.NoTimeout))
}

// WatchQuery returns a stream of changes that satisy a query.
func (o *object) WatchQuery(ctx context.T, req watch.QueryRequest) (watch.QueryWatcherWatchQueryCall, error) {
	return o.serv.WatchQuery(ctx, req, veyron2.CallTimeout(ipc.NoTimeout))
}

// errorObject responds with an error to all operations.
func (o *errorObject) Exists(ctx context.T) (bool, error) {
	return false, o.err
}

func (o *errorObject) Get(ctx context.T) (storage.Entry, error) {
	return nullEntry, o.err
}

func (o *errorObject) Put(ctx context.T, v interface{}) (storage.Stat, error) {
	return nullStat, o.err
}

func (o *errorObject) Remove(ctx context.T) error {
	return o.err
}

func (o *errorObject) Stat(ctx context.T) (storage.Stat, error) {
	return nullStat, o.err
}

func (o *errorObject) GetAllPaths(ctx context.T) ([]string, error) {
	return nil, o.err
}

func (o *errorObject) Query(ctx context.T, q query.Query) storage.QueryStream {
	return &errorQueryStream{o.err}
}

func (o *errorObject) Glob(ctx context.T, pattern string) storage.GlobCall {
	return &errorGlobStream{o.err}
}

func (o *errorObject) Commit(ctx context.T) error {
	return o.err
}

func (o *errorObject) Abort(ctx context.T) error {
	return o.err
}

func (o *errorObject) WatchGlob(ctx context.T, req watch.GlobRequest) (watch.GlobWatcherWatchGlobCall, error) {
	return nil, o.err
}

func (o *errorObject) WatchQuery(ctx context.T, req watch.QueryRequest) (watch.QueryWatcherWatchQueryCall, error) {
	return nil, o.err
}

// errorQueryStream implements storage.QueryStream.
type errorQueryStream struct {
	err error
}

func (e *errorQueryStream) Advance() bool {
	return false
}

func (e *errorQueryStream) Value() storage.QueryResult {
	panic("No value.")
}

func (e *errorQueryStream) Err() error {
	return e.err
}

func (e *errorQueryStream) Cancel() {}

// errorGlobStream implements storage.GlobCall.
type errorGlobStream struct {
	err error
}

func (e *errorGlobStream) RecvStream() storage.GlobStream {
	return e
}

func (e *errorGlobStream) Advance() bool {
	return false
}

func (e *errorGlobStream) Value() string {
	panic("No value.")
}

func (e *errorGlobStream) Err() error {
	return e.err
}

func (e *errorGlobStream) Cancel() {}
