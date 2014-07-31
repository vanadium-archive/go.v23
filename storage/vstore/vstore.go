// Package vstore implements a client interface to a Veyron store.
// The API is defined in veyron2/storage.
package vstore

import (
	"veyron2/ipc"
	"veyron2/storage"
	"veyron2/storage/vstore/primitives"
)

type VStore struct {
	mount string
}

var _ storage.Store = (*VStore)(nil)

// New returns a storage.Store for a Veyron store mounted at an Object name.
func New(mount string, opts ...ipc.BindOpt) (storage.Store, error) {
	st := &VStore{mount}
	return st, nil
}

// BindObject returns a storage.Object for a value at an Object name. This
// method always succeeds. If the Object name is not a value in a Veyron store,
// all subsequent operations on the Object will fail.
func (st *VStore) BindObject(name string) storage.Object {
	return primitives.BindObject(st.mount, name)
}

// BindTransaction returns a storage.Transaction for a value at a Transaction
// name. This method always succeeds. If the Transaction name is not a value in
// a Veyron store, all subsequent operations on the Transaction will fail.
func (st *VStore) BindTransaction(name string) storage.Transaction {
	return primitives.BindTransaction(st.mount, name)
}

// BindTransactionRoot returns a storage.TransactionRoot for a value at a
// TransactionRoot name. This method always succeeds. If the TransactionRoot
// name is not a value in a Veyron store, all subsequent operations on the
// TransactionRoot will fail.
func (st *VStore) BindTransactionRoot(name string) storage.TransactionRoot {
	return primitives.BindTransactionRoot(st.mount, name)
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
