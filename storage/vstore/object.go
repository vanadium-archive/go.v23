package vstore

import (
	"errors"

	"veyron2/context"
	"veyron2/naming"
	"veyron2/query"
	"veyron2/services/store"
	"veyron2/services/watch"
	"veyron2/services/watch/types"
	"veyron2/storage"
)

var (
	ErrBadAttr = errors.New("bad attribute")

	_ storage.Object      = (*object)(nil)
	_ storage.Object      = (*errorObject)(nil)
	_ storage.QueryStream = (*errorQueryStream)(nil)
	_ storage.GlobStream  = (*errorGlobStream)(nil)

	nullEntry storage.Entry
	nullStat  storage.Stat
)

// object implements the store.Object interface.  It is just a thin
// wrapper for store.Object except for the Glob and Query methods.
type object struct {
	serv store.Object
	name string
}

func newObject(name string) storage.Object {
	serv, err := store.BindObject(name)
	if err != nil {
		return newErrorObject(err)
	}
	return &object{serv, name}
}

// Bind implements the storage.Object method.
func (o *object) Bind(relativeName string) storage.Object {
	return newObject(naming.Join(o.name, relativeName))
}

// Exists implements the storage.Object method.
func (o *object) Exists(ctx context.T) (bool, error) {
	return o.serv.Exists(ctx)
}

// Get implements the storage.Object method.
func (o *object) Get(ctx context.T) (storage.Entry, error) {
	return o.serv.Get(ctx)
}

// Put implements the storage.Object method.
func (o *object) Put(ctx context.T, v interface{}) (storage.Stat, error) {
	return o.serv.Put(ctx, v)
}

// Remove implements the storage.Object method.
func (o *object) Remove(ctx context.T) error {
	return o.serv.Remove(ctx)
}

// Stat implements the storage.Object method.
func (o *object) Stat(ctx context.T) (storage.Stat, error) {
	return o.serv.Stat(ctx)
}

// Query implements the storage.Object method.
func (o *object) Query(ctx context.T, q query.Query) storage.QueryStream {
	stream, err := o.serv.Query(ctx, q)
	if err != nil {
		return &errorQueryStream{err}
	}
	return newQueryStream(stream)
}

// Glob implements the storage.Object method.
func (o *object) Glob(ctx context.T, pattern string) storage.GlobCall {
	stream, err := o.serv.Glob(ctx, pattern)
	if err != nil {
		return &errorGlobStream{err}
	}
	return newGlobStream(stream)
}

// WatchGlob implements the storage.Object method.
func (o *object) WatchGlob(ctx context.T, req types.GlobRequest) (watch.GlobWatcherWatchGlobCall, error) {
	return o.serv.WatchGlob(ctx, req)
}

// WatchQuery implements the storage.Object method.
func (o *object) WatchQuery(ctx context.T, req types.QueryRequest) (watch.QueryWatcherWatchQueryCall, error) {
	return o.serv.WatchQuery(ctx, req)
}

// errorObject responds with an error to all operations.
type errorObject struct {
	err error
}

func newErrorObject(err error) storage.Object {
	return &errorObject{err}
}

func (o *errorObject) Bind(relativeName string) storage.Object {
	return o
}

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

func (o *errorObject) Query(ctx context.T, q query.Query) storage.QueryStream {
	return &errorQueryStream{o.err}
}

func (o *errorObject) Glob(ctx context.T, pattern string) storage.GlobCall {
	return &errorGlobStream{o.err}
}

func (o *errorObject) WatchGlob(ctx context.T, req types.GlobRequest) (watch.GlobWatcherWatchGlobCall, error) {
	return nil, o.err
}

func (o *errorObject) WatchQuery(ctx context.T, req types.QueryRequest) (watch.QueryWatcherWatchQueryCall, error) {
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

// errorGlobStream implements storage.GlobCall and storage.GlobStream.
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
