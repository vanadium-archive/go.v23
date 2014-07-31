package primitives

import (
	"veyron2/context"
	"veyron2/naming"
	"veyron2/services/store"
	"veyron2/storage"
)

type transaction struct {
	serv store.Transaction
}

type errorTransaction struct {
	err error
}

var (
	_ storage.Transaction = (*transaction)(nil)
	_ storage.Transaction = (*errorTransaction)(nil)
)

// BindTransaction returns a storage.Transaction for a value at a Transaction
// name. This method always succeeds. If the Transaction name is not a value in
// a Veyron store, all subsequent operations on the Transaction will fail.
// TODO(sadovsky): Maybe just take a single name argument, so that the client
// need not know the store mount name?
func BindTransaction(mount, name string) storage.Transaction {
	serv, err := store.BindTransaction(naming.Join(mount, name))
	if err != nil {
		return &errorTransaction{err: err}
	}
	return &transaction{serv: serv}
}

// Commit commits the transaction. Returns an error if the operation aborted.
func (t *transaction) Commit(ctx context.T) error {
	if t.serv == nil {
		return nil
	}
	return t.serv.Commit(ctx)
}

// Abort aborts the transaction. Returns an error if the operation could not be
// aborted.
func (t *transaction) Abort(ctx context.T) error {
	if t.serv == nil {
		return nil
	}
	return t.serv.Abort(ctx)
}

// errorTransaction responds with an error to all operations.
func (t *errorTransaction) Commit(ctx context.T) error {
	return t.err
}

func (t *errorTransaction) Abort(ctx context.T) error {
	return t.err
}
