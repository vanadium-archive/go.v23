// Package vstore implements a client interface to a Veyron store.
// The API is defined in veyron2/storage.
package vstore

import (
	"veyron2/context"
	"veyron2/storage"
)

type vStore struct{}

var _ storage.Store = (*vStore)(nil)

// New returns a storage.Store.
func New() storage.Store {
	return &vStore{}
}

func (v *vStore) Bind(name string) storage.Object {
	return newObject(name)
}

func (v *vStore) NewTransaction(ctx context.T, name string,
	opts ...storage.TransactionOpt) storage.Transaction {
	// root, err := store.BindTransactionRoot(name)
	// if err != nil {
	// 	return newErrorTransaction(err)
	// }
	// tid, err := root.CreateTransaction(ctx, nil)
	// if err != nil {
	// 	return newErrorTransaction(err)
	// }
	// tname := naming.Join(name, tid)
	// tx, err := store.BindTransaction(tname)
	// if err != nil {
	// 	// We would want to abort tx if there was an error, but there's no way
	// 	// to send the abort if we can't bind.
	// 	return newErrorTransaction(err)
	// }
	// return &transaction{tname, tx}
	return &transaction{}
}
