package primitives

import (
	"veyron2/context"
	"veyron2/naming"
	"veyron2/services/store"
	"veyron2/storage"
	"veyron2/vdl/vdlutil"
)

type transactionRoot struct {
	serv store.TransactionRoot
}

type errorTransactionRoot struct {
	err error
}

var (
	_ storage.TransactionRoot = (*transactionRoot)(nil)
	_ storage.TransactionRoot = (*errorTransactionRoot)(nil)
)

// BindTransactionRoot returns a storage.TransactionRoot for a value at a
// TransactionRoot name. This method always succeeds. If the TransactionRoot
// name is not a value in a Veyron store, all subsequent operations on the
// TransactionRoot will fail.
// TODO(sadovsky): Maybe just take a single name argument, so that the client
// need not know the store mount name?
func BindTransactionRoot(mount, name string) storage.TransactionRoot {
	serv, err := store.BindTransactionRoot(naming.Join(mount, name))
	if err != nil {
		return &errorTransactionRoot{err: err}
	}
	return &transactionRoot{serv: serv}
}

// transactionOptsToAnyData converts the array to []vdlutil.Any.
func transactionOptsToAnyData(opts []storage.TransactionOpt) []vdlutil.Any {
	vopts := make([]vdlutil.Any, len(opts))
	for i, x := range opts {
		vopts[i] = vdlutil.Any(x)
	}
	return vopts
}

func (t *transactionRoot) CreateTransaction(ctx context.T, opts ...storage.TransactionOpt) (string, error) {
	// Q(sadovsky): Why use []vdl.Any? I.e. why don't we put []TransactionOpt in
	// the vdl method signature?
	return t.serv.CreateTransaction(ctx, transactionOptsToAnyData(opts))
}

// errorTransactionRoot responds with an error to all operations.
func (t *errorTransactionRoot) CreateTransaction(ctx context.T, opts ...storage.TransactionOpt) (string, error) {
	return "", t.err
}
