package vstore

import (
	"veyron2/context"
	"veyron2/naming"
	"veyron2/query"
	"veyron2/services/store"
	"veyron2/services/watch"
	"veyron2/services/watch/types"
	"veyron2/storage"
)

var (
	_ storage.Dir = (*dir)(nil)
	_ storage.Dir = (*errorDir)(nil)
)

// dir implements the storage.Dir interface.  It is just a thin wrapper for
// store.Dir except for the Glob and Query methods.
type dir struct {
	serv store.Dir
	// name is the full name of the Dir including the name of the Store.
	name string
}

func newDir(name string) storage.Dir {
	serv, err := store.BindDir(name)
	if err != nil {
		return newErrorDir(err)
	}
	return &dir{serv, name}
}

// NewTransaction implements the storage.Dir method.
func (d *dir) NewTransaction(ctx context.T, opts ...storage.TransactionOpt) (
	storage.Dir, storage.Transaction) {
	tid, err := d.serv.NewTransaction(ctx, nil)
	if err != nil {
		return newErrorDir(err), newErrorTransaction(err)
	}
	tname := naming.Join(d.name, tid)
	return newDir(tname), newTransaction(tname)
}

// BindDir implements the storage.Dir method.
func (d *dir) BindDir(relativeName string) storage.Dir {
	return newDir(naming.Join(d.name, relativeName))
}

// BindObject implements the storage.Dir method.
func (d *dir) BindObject(relativeName string) storage.Object {
	return newObject(naming.Join(d.name, relativeName))
}

// Make implements the storage.Dir method.
func (d *dir) Make(ctx context.T) error {
	return d.serv.Make(ctx)
}

// Remove implements the storage.Dir method.
func (d *dir) Remove(ctx context.T) error {
	return d.serv.Remove(ctx)
}

// Stat implements the storage.Dir method.
func (d *dir) Stat(ctx context.T) (storage.Stat, error) {
	return d.serv.Stat(ctx)
}

// Exists implements the storage.Dir method.
func (d *dir) Exists(ctx context.T) (bool, error) {
	return d.serv.Exists(ctx)
}

// Query implements the storage.Dir method.
func (d *dir) Query(ctx context.T, q query.Query) storage.QueryStream {
	stream, err := d.serv.Query(ctx, q)
	if err != nil {
		return &errorQueryStream{err}
	}
	return newQueryStream(stream)
}

// Glob implements the storage.Dir method.
func (d *dir) Glob(ctx context.T, pattern string) storage.GlobCall {
	stream, err := d.serv.Glob(ctx, pattern)
	if err != nil {
		return &errorGlobStream{err}
	}
	return newGlobStream(stream)
}

// WatchGlob implements the storage.Dir method.
func (d *dir) WatchGlob(ctx context.T, req types.GlobRequest) (watch.GlobWatcherWatchGlobCall, error) {
	return d.serv.WatchGlob(ctx, req)
}

// WatchQuery implements the storage.Dir method.
func (d *dir) WatchQuery(ctx context.T, req types.QueryRequest) (watch.QueryWatcherWatchQueryCall, error) {
	return d.serv.WatchQuery(ctx, req)
}

// errorDir responds with an error to all operations.
type errorDir struct {
	err error
}

func newErrorDir(err error) storage.Dir {
	return &errorDir{err}
}

func (d *errorDir) NewTransaction(ctx context.T, opts ...storage.TransactionOpt) (
	storage.Dir, storage.Transaction) {
	return d, newErrorTransaction(d.err)
}

func (d *errorDir) BindDir(relativeName string) storage.Dir {
	return d
}

func (d *errorDir) BindObject(relativeName string) storage.Object {
	return newErrorObject(d.err)
}

func (d *errorDir) Make(ctx context.T) error {
	return d.err
}

func (d *errorDir) Remove(ctx context.T) error {
	return d.err
}

func (d *errorDir) Stat(ctx context.T) (storage.Stat, error) {
	return nullStat, d.err
}

func (d *errorDir) Exists(ctx context.T) (bool, error) {
	return false, d.err
}

func (d *errorDir) Query(ctx context.T, q query.Query) storage.QueryStream {
	return &errorQueryStream{d.err}
}

func (d *errorDir) Glob(ctx context.T, pattern string) storage.GlobCall {
	return &errorGlobStream{d.err}
}

func (d *errorDir) WatchGlob(ctx context.T, req types.GlobRequest) (watch.GlobWatcherWatchGlobCall, error) {
	return nil, d.err
}

func (d *errorDir) WatchQuery(ctx context.T, req types.QueryRequest) (watch.QueryWatcherWatchQueryCall, error) {
	return nil, d.err
}
