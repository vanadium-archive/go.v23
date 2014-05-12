package primitives

import (
	"errors"
	"time"

	"veyron2/idl"
	"veyron2/services/store"
	"veyron2/storage"
)

type object struct {
	sServ store.Store
	oServ store.Object
}

type errorObject struct {
	err error
}

var (
	ErrBadAttr   = errors.New("bad attribute")
	ErrTypeError = errors.New("type error")

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
			return ErrBadAttr
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
func BindObject(sServ store.Store, mount, name string) storage.Object {
	if len(name) > 0 && name[0] == '/' {
		name = name[1:]
	}
	oServ, err := store.BindObject(mount + "/" + name)
	if err != nil {
		return &errorObject{err: err}
	}
	return &object{sServ: sServ, oServ: oServ}
}

// Exists returns true iff the Entry has a value.
func (o *object) Exists(t storage.Transaction) (bool, error) {
	id, err := UpdateTransaction(t, o.sServ)
	if err != nil {
		return false, err
	}
	return o.oServ.Exists(id)
}

// Get returns the value for the Object.  The value returned is from the
// most recent mutation of the entry in the storage.Transaction, or from the
// storage.Transaction's snapshot if there is no mutation.
func (o *object) Get(t storage.Transaction) (storage.Entry, error) {
	id, err := UpdateTransaction(t, o.sServ)
	if err != nil {
		return nullEntry, err
	}
	entry, err := o.oServ.Get(id)
	if err != nil {
		return nullEntry, err
	}
	return makeEntry(&entry)
}

// Put adds or modifies the Object.
func (o *object) Put(t storage.Transaction, v interface{}) (storage.Stat, error) {
	id, err := UpdateTransaction(t, o.sServ)
	if err != nil {
		return nullStat, err
	}
	serviceStat, err := o.oServ.Put(id, v)
	if err != nil {
		return nullStat, err
	}
	return makeStat(&serviceStat)
}

// Remove removes the Object.
func (o *object) Remove(t storage.Transaction) error {
	id, err := UpdateTransaction(t, o.sServ)
	if err != nil {
		return err
	}
	return o.oServ.Remove(id)
}

// SetAttr changes the attributes of the entry, such as permissions and
// replication groups.  Attributes are associated with the value, not the
// path.
func (o *object) SetAttr(t storage.Transaction, attrs ...storage.Attr) error {
	id, err := UpdateTransaction(t, o.sServ)
	if err != nil {
		return err
	}
	serviceAttrs := make([]idl.AnyData, len(attrs))
	for i, x := range attrs {
		serviceAttrs[i] = idl.AnyData(x)
	}
	return o.oServ.SetAttr(id, serviceAttrs)
}

// Stat returns entry info.
func (o *object) Stat(t storage.Transaction) (storage.Stat, error) {
	id, err := UpdateTransaction(t, o.sServ)
	if err != nil {
		return nullStat, err
	}
	serviceStat, err := o.oServ.Stat(id)
	if err != nil {
		return nullStat, err
	}
	return makeStat(&serviceStat)
}

// Glob returns a sequence of names that match the given pattern.
func (o *object) GlobT(t storage.Transaction, pattern string) (storage.GlobStream, error) {
	id, err := UpdateTransaction(t, o.sServ)
	if err != nil {
		return nil, err
	}
	return o.oServ.GlobT(id, pattern)
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
