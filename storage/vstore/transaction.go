package vstore

import (
	"log"
	"math/rand"

	"veyron2/idl"
	"veyron2/services/store"
	"veyron2/storage"
)

type transaction struct {
	id    store.TransactionID
	opts  []storage.TransactionOpt
	store *vstore
}

var _ storage.Transaction = (*transaction)(nil)

// NewTransaction returns a fresh empty transaction.  The Transaction can not
// span multiple stores; it can be used only with one instance of a store.Store.
// The storage.instance is determined by the first Object operation to use the
// Transaction.
func NewTransaction(opts ...storage.TransactionOpt) storage.Transaction {
	tr := &transaction{
		id:   store.TransactionID(rand.Int63()),
		opts: opts,
	}
	return tr
}

// transactionOptsToAnyData converts the array to []idl.AnyData.
func transactionOptsToAnyData(opts []storage.TransactionOpt) []idl.AnyData {
	vopts := make([]idl.AnyData, len(opts))
	for i, x := range opts {
		vopts[i] = idl.AnyData(x)
	}
	return vopts
}

// setStore binds the transaction to a store if it hasn't already been bound.
// Returns true iff the binding succeeded, or the transaction is already bound
// to the specified store.
func (tr *transaction) setStore(st *vstore) bool {
	if tr.store == nil {
		vopts := transactionOptsToAnyData(tr.opts)
		if err := st.serv.CreateTransaction(tr.id, vopts); err != nil {
			log.Printf("CreateTransaction error: %s", err)
			return false
		}
		tr.store = st
		return true
	}
	return tr.store == st
}

// Commit commits the transaction.  Returns an error if the operation aborted.
func (tr *transaction) Commit() error {
	if tr.store == nil {
		return nil
	}
	return tr.store.serv.Commit(tr.id)
}

// Abort aborts the transaction.  Returns an error if the operation
// could not be aborted.
func (tr *transaction) Abort() error {
	if tr.store == nil {
		return nil
	}
	return tr.store.serv.Abort(tr.id)
}
