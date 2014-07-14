package primitives

import (
	"errors"
	"log"
	"math/rand"
	"time"

	"veyron2/context"
	"veyron2/services/store"
	"veyron2/storage"
	"veyron2/vdl/vdlutil"
)

var (
	ErrBadTransaction = errors.New("bad transaction")
	rng               = rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
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
func NewTransaction(ctx context.T, opts ...storage.TransactionOpt) storage.Transaction {
	tr := &transaction{
		id:   store.TransactionID(rng.Int63()),
		opts: opts,
	}
	return tr
}

// updateTransaction casts the transaction and sets the store object in it.
func UpdateTransaction(ctx context.T, t storage.Transaction, serv store.Store) (store.TransactionID, error) {
	if t == nil {
		return nullTransactionID, nil
	}
	tr, ok := t.(*transaction)
	if !ok || !tr.setServ(ctx, serv) {
		return nullTransactionID, ErrBadTransaction
	}
	return tr.id, nil
}

// transactionOptsToAnyData converts the array to []vdlutil.Any.
func transactionOptsToAnyData(opts []storage.TransactionOpt) []vdlutil.Any {
	vopts := make([]vdlutil.Any, len(opts))
	for i, x := range opts {
		vopts[i] = vdlutil.Any(x)
	}
	return vopts
}

// setServ binds the transaction to a store if it hasn't already been bound.
// Returns true iff the binding succeeded, or the transaction is already bound
// to the specified store.
func (tr *transaction) setServ(ctx context.T, serv store.Store) bool {
	if tr.serv == nil {
		vopts := transactionOptsToAnyData(tr.opts)
		if err := serv.CreateTransaction(ctx, tr.id, vopts); err != nil {
			log.Printf("CreateTransaction error: %s", err)
			return false
		}
		tr.serv = serv
		return true
	}
	return tr.serv == serv
}

// Commit commits the transaction.  Returns an error if the operation aborted.
func (tr *transaction) Commit(ctx context.T) error {
	if tr.serv == nil {
		return nil
	}
	return tr.serv.Commit(ctx, tr.id)
}

// Abort aborts the transaction.  Returns an error if the operation
// could not be aborted.
func (tr *transaction) Abort(ctx context.T) error {
	if tr.serv == nil {
		return nil
	}
	return tr.serv.Abort(ctx, tr.id)
}
