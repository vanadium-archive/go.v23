package primitives

import (
	"errors"
	"log"
	"math/rand"

	"veyron2/idl"
	"veyron2/services/store"
	"veyron2/storage"
)

var (
	ErrBadTransaction = errors.New("bad transaction")
)

type transaction struct {
	id   store.TransactionID
	opts []storage.TransactionOpt
	serv store.Store
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

// updateTransaction casts the transaction and sets the store object in it.
func UpdateTransaction(t storage.Transaction, serv store.Store) (store.TransactionID, error) {
	if t == nil {
		return nullTransactionID, nil
	}
	tr, ok := t.(*transaction)
	if !ok || !tr.setServ(serv) {
		return nullTransactionID, ErrBadTransaction
	}
	return tr.id, nil
}

// transactionOptsToAnyData converts the array to []idl.AnyData.
func transactionOptsToAnyData(opts []storage.TransactionOpt) []idl.AnyData {
	vopts := make([]idl.AnyData, len(opts))
	for i, x := range opts {
		vopts[i] = idl.AnyData(x)
	}
	return vopts
}

// setServ binds the transaction to a store if it hasn't already been bound.
// Returns true iff the binding succeeded, or the transaction is already bound
// to the specified store.
func (tr *transaction) setServ(serv store.Store) bool {
	if tr.serv == nil {
		vopts := transactionOptsToAnyData(tr.opts)
		if err := serv.CreateTransaction(tr.id, vopts); err != nil {
			log.Printf("CreateTransaction error: %s", err)
			return false
		}
		tr.serv = serv
		return true
	}
	return tr.serv == serv
}

// Commit commits the transaction.  Returns an error if the operation aborted.
func (tr *transaction) Commit() error {
	if tr.serv == nil {
		return nil
	}
	return tr.serv.Commit(tr.id)
}

// Abort aborts the transaction.  Returns an error if the operation
// could not be aborted.
func (tr *transaction) Abort() error {
	if tr.serv == nil {
		return nil
	}
	return tr.serv.Abort(tr.id)
}
