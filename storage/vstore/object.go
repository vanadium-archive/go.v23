package vstore

import (
	"errors"
	"time"

	"veyron2/idl"
	"veyron2/services/store"
	"veyron2/storage"
)

type object struct {
	store *vstore
	serv  store.Object
}

type errorObject struct {
	err error
}

var (
	errBadTransaction = errors.New("bad transaction")
	errBadAttr        = errors.New("bad attribute")
	errTypeError      = errors.New("type error")

	_ storage.Object = (*object)(nil)

	nullEntry         storage.Entry
	nullStat          storage.Stat
	nullTransactionID store.TransactionID
)

func fillStat(stat *storage.Stat, serviceStat *store.Stat) error {
	attrs := make([]storage.Attr, len(serviceStat.Attrs))
	for i, attr := range serviceStat.Attrs {
		a, ok := attr.(storage.Attr)
		if !ok {
			return errBadAttr
		}
		attrs[i] = a
	}
	stat.ID = serviceStat.ID
	stat.MTime = time.Unix(0, serviceStat.MTimeNS)
	stat.Attrs = attrs
	return nil
}

func makeStat(serviceStat *store.Stat) (storage.Stat, error) {
	if serviceStat == nil {
		return nullStat, nil
	}
	var stat storage.Stat
	if err := fillStat(&stat, serviceStat); err != nil {
		return nullStat, err
	}
	return stat, nil
}

func makeEntry(serviceEntry *store.Entry) (storage.Entry, error) {
	entry := storage.Entry{Value: serviceEntry.Value}
	if err := fillStat(&entry.Stat, &serviceEntry.Stat); err != nil {
		return nullEntry, err
	}
	return entry, nil
}

// Bind returns a storage.Object for a value at a Veyron name.  The Bind always
// succeeds.  If the Veyron name is not a value in a Veyron storage. all
// subsequent operations on the object will fail.
func (st *vstore) Bind(name string) storage.Object {
	if len(name) > 0 && name[0] == '/' {
		name = name[1:]
	}
	serv, err := store.BindObject(st.mount + "/" + name)
	if err != nil {
		return &errorObject{err: err}
	}
	return &object{store: st, serv: serv}
}

// updateTransaction casts the transaction and sets the store object in it.
func (o *object) updateTransaction(t storage.Transaction) (store.TransactionID, bool) {
	if t == nil {
		return nullTransactionID, true
	}
	tr, ok := t.(*transaction)
	if !ok || !tr.setStore(o.store) {
		return nullTransactionID, false
	}
	return tr.id, true
}

// Exists returns true iff the Entry has a value.
func (o *object) Exists(t storage.Transaction) (bool, error) {
	id, ok := o.updateTransaction(t)
	if !ok {
		return false, errBadTransaction
	}
	return o.serv.Exists(id)
}

// Get returns the value for the Object.  The value returned is from the
// most recent mutation of the entry in the storage.Transaction, or from the
// storage.Transaction's snapshot if there is no mutation.
func (o *object) Get(t storage.Transaction) (storage.Entry, error) {
	id, ok := o.updateTransaction(t)
	if !ok {
		return nullEntry, errBadTransaction
	}
	entry, err := o.serv.Get(id)
	if err != nil {
		return nullEntry, err
	}
	return makeEntry(&entry)
}

// Put adds or modifies the Object.
func (o *object) Put(t storage.Transaction, v interface{}) (storage.Stat, error) {
	tid, ok := o.updateTransaction(t)
	if !ok {
		return nullStat, errBadTransaction
	}
	serviceStat, err := o.serv.Put(tid, v)
	if err != nil {
		return nullStat, err
	}
	return makeStat(&serviceStat)
}

// Remove removes the Object.
func (o *object) Remove(t storage.Transaction) error {
	id, ok := o.updateTransaction(t)
	if !ok {
		return errBadTransaction
	}
	return o.serv.Remove(id)
}

// SetAttr changes the attributes of the entry, such as permissions and
// replication groups.  Attributes are associated with the value, not the
// path.
func (o *object) SetAttr(t storage.Transaction, attrs ...storage.Attr) error {
	id, ok := o.updateTransaction(t)
	if !ok {
		return errBadTransaction
	}
	serviceAttrs := make([]idl.AnyData, len(attrs))
	for i, x := range attrs {
		serviceAttrs[i] = idl.AnyData(x)
	}
	return o.serv.SetAttr(id, serviceAttrs)
}

// Stat returns entry info.
func (o *object) Stat(t storage.Transaction) (storage.Stat, error) {
	id, ok := o.updateTransaction(t)
	if !ok {
		return nullStat, errBadTransaction
	}
	serviceStat, err := o.serv.Stat(id)
	if err != nil {
		return nullStat, err
	}
	return makeStat(&serviceStat)
}

// Glob returns a sequence of names that match the given pattern.
func (o *object) GlobT(t storage.Transaction, pattern string) (storage.GlobStream, error) {
	id, ok := o.updateTransaction(t)
	if !ok {
		return nil, errBadTransaction
	}
	return o.serv.GlobT(id, pattern)
}

// The errorObject responds with an error to all operations.
func (o *errorObject) Exists(t storage.Transaction) (bool, error) {
	return false, o.err
}

func (o *errorObject) Get(t storage.Transaction) (storage.Entry, error) {
	return nullEntry, o.err
}

func (o *errorObject) Put(t storage.Transaction, v interface{}) (storage.Stat, error) {
	return nullStat, o.err
}

func (o *errorObject) Remove(t storage.Transaction) error {
	return o.err
}

func (o *errorObject) SetAttr(t storage.Transaction, attrs ...storage.Attr) error {
	return o.err
}

func (o *errorObject) Stat(t storage.Transaction) (storage.Stat, error) {
	return nullStat, o.err
}

func (o *errorObject) GetAllPaths(t storage.Transaction) ([]string, error) {
	return nil, o.err
}

func (o *errorObject) Glob(pattern string) (storage.GlobStream, error) {
	return nil, o.err
}

func (o *errorObject) GlobT(t storage.Transaction, pattern string) (storage.GlobStream, error) {
	return nil, o.err
}
