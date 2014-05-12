// Package vstore implements a client interface to a Veyron store.
// The API is defined in veyron2/storage.
package vstore

import (
	"veyron2"
	"veyron2/ipc"
	"veyron2/query"
	"veyron2/services/store"
	"veyron2/services/watch"
	"veyron2/storage"
	"veyron2/storage/vstore/primitives"
)

type VStore struct {
	mount string
	serv  store.Store
}

var _ storage.Store = (*VStore)(nil)

// New returns a storage.Store for a Veyron store mounted at a Veyron name.
func New(mount string, opts ...ipc.BindOpt) (storage.Store, error) {
	serv, err := store.BindStore(mount+"/.store", opts...)
	if err != nil {
		return nil, err
	}

	st := &VStore{}
	st.Init(mount, serv)
	return st, nil
}

// Init initializes a storage.Store for the specified store mounted at the
// specified Veyron name.
func (st *VStore) Init(mount string, serv store.Store) {
	st.mount = mount
	st.serv = serv
}

// Bind returns a storage.Object for a value at a Veyron name.  The Bind always
// succeeds.  If the Veyron name is not a value in a Veyron storage. all
// subsequent operations on the object will fail.
func (st *VStore) Bind(name string) storage.Object {
	return primitives.BindObject(st.serv, st.mount, name)
}

// Glob returns a sequence of names that match the given pattern.
func (st *VStore) Glob(t storage.Transaction, pattern string) (storage.GlobStream, error) {
	id, err := primitives.UpdateTransaction(t, st.serv)
	if err != nil {
		return nil, err
	}
	return st.serv.Glob(id, pattern)
}

func (st *VStore) Watch(req watch.Request) (watch.WatcherWatchStream, error) {
	return st.serv.Watch(req, veyron2.CallTimeout(ipc.NoTimeout))
}

// Search returns an Iterator to a sequence of elements that satisfy the
// query.
func (st *VStore) Search(t storage.Transaction, q query.Query) storage.Iterator {
	panic("not implemented")
}

// SetConflictResolver specifies a function to perform conflict resolution.
// The <ty> represents the IDL name for the type.
func (st *VStore) SetConflictResolver(ty string, r storage.ConflictResolver) {
	panic("not implemented")
}

// Close closes the Store.
func (st *VStore) Close() error {
	return nil
}
