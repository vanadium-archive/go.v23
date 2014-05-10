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
)

type vstore struct {
	mount string
	serv  store.Store
}

var _ storage.Store = (*vstore)(nil)

// New returns a storage.Store for a Veyron store mounted at a Veyron name.
func New(mount string, opts ...ipc.BindOpt) (storage.Store, error) {
	serv, err := store.BindStore(mount+"/.store", opts...)
	if err != nil {
		return nil, err
	}

	st := &vstore{mount: mount, serv: serv}
	return st, nil
}

// Glob returns a sequence of names that match the given pattern.
func (st *vstore) Glob(t storage.Transaction, pattern string) (storage.GlobStream, error) {
	id, ok := st.updateTransaction(t)
	if !ok {
		return nil, errBadTransaction
	}
	return st.serv.Glob(id, pattern)
}

func (st *vstore) Watch(req watch.Request) (watch.WatcherWatchStream, error) {
	return st.serv.Watch(req, veyron2.CallTimeout(ipc.NoTimeout))
}

func (st *vstore) updateTransaction(t storage.Transaction) (store.TransactionID, bool) {
	if t == nil {
		return nullTransactionID, true
	}
	tr, ok := t.(*transaction)
	if !ok || !tr.setStore(st) {
		return nullTransactionID, false
	}
	return tr.id, true
}

// Search returns an Iterator to a sequence of elements that satisfy the
// query.
func (st *vstore) Search(t storage.Transaction, q query.Query) storage.Iterator {
	panic("not implemented")
}

// SetConflictResolver specifies a function to perform conflict resolution.
// The <ty> represents the IDL name for the type.
func (st *vstore) SetConflictResolver(ty string, r storage.ConflictResolver) {
	panic("not implemented")
}

// Close closes the Store.
func (st *vstore) Close() error {
	return nil
}
