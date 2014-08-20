package vstore

import (
	"veyron2/context"
	"veyron2/naming"
	"veyron2/services/store"
	"veyron2/storage"
)

var (
	_ storage.Transaction = (*transaction)(nil)
	_ storage.Transaction = (*errorTransaction)(nil)
)

// transaction implements the storage.Transaction interface.
type transaction struct {
	// tname is the name of the transaction object.  All objects involved in the
	// transaction are relative to this.
	tname string
	// serv is the stub supporting Commit and Abort.
	serv store.Transaction
}

// Bind implements the storage.Transaction method.
func (t *transaction) Bind(relativeName string) storage.Object {
	return newObject(naming.Join(t.tname, relativeName))
}

// Commit implements the storage.Transaction method.
func (t *transaction) Commit(ctx context.T) error {
	return t.serv.Commit(ctx)
}

// Abort implements the storage.Transaction method.
func (t *transaction) Abort(ctx context.T) error {
	return t.serv.Abort(ctx)
}

// errorTransaction responds with an error to all operations.  It implements
// the storage.Transaction interface.
type errorTransaction struct {
	err error
}

func newErrorTransaction(err error) storage.Transaction {
	return &errorTransaction{err}
}

// Bind implements the storage.Transaction method.
func (t *errorTransaction) Bind(relativeName string) storage.Object {
	return newErrorObject(t.err)
}

// Commit implements the storage.Transaction method.
func (t *errorTransaction) Commit(ctx context.T) error {
	return t.err
}

// Abort implements the storage.Transaction method.
func (t *errorTransaction) Abort(ctx context.T) error {
	return t.err
}
