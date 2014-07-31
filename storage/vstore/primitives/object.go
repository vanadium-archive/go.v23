package primitives

import (
	"errors"
	"time"

	"veyron2"
	"veyron2/context"
	"veyron2/ipc"
	"veyron2/naming"
	"veyron2/query"
	"veyron2/services/store"
	"veyron2/services/watch"
	"veyron2/storage"
	"veyron2/vdl/vdlutil"
)

type object struct {
	serv store.Object
}

type errorObject struct {
	err error
}

var (
	ErrBadAttr = errors.New("bad attribute")

	_ storage.Object      = (*object)(nil)
	_ storage.Object      = (*errorObject)(nil)
	_ storage.QueryStream = (*errorQueryStream)(nil)
	_ storage.GlobStream  = (*errorGlobStream)(nil)

	nullEntry       storage.Entry
	nullStat        storage.Stat
	nullChangeBatch watch.ChangeBatch
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

// BindObject returns a storage.Object for a value at an Object name. This
// method always succeeds. If the Object name is not a value in a Veyron store,
// all subsequent operations on the Object will fail.
// TODO(sadovsky): Maybe just take a single name argument, so that the client
// need not know the store mount name?
func BindObject(mount, name string) storage.Object {
	serv, err := store.BindObject(naming.Join(mount, name))
	if err != nil {
		return &errorObject{err: err}
	}
	return &object{serv: serv}
}

// Exists returns true iff the Entry has a value.
func (o *object) Exists(ctx context.T) (bool, error) {
	return o.serv.Exists(ctx)
}

// Get returns the value for the Object.  The value returned is from the
// most recent mutation of the entry in the storage.Transaction, or from the
// storage.Transaction's snapshot if there is no mutation.
func (o *object) Get(ctx context.T) (storage.Entry, error) {
	entry, err := o.serv.Get(ctx)
	if err != nil {
		return nullEntry, err
	}
	return makeEntry(&entry)
}

// Put adds or modifies the Object.
func (o *object) Put(ctx context.T, v interface{}) (storage.Stat, error) {
	serviceStat, err := o.serv.Put(ctx, v)
	if err != nil {
		return nullStat, err
	}
	return makeStat(&serviceStat)
}

// Remove removes the Object.
func (o *object) Remove(ctx context.T) error {
	return o.serv.Remove(ctx)
}

// SetAttr changes the attributes of the entry, such as permissions and
// replication groups.  Attributes are associated with the value, not the
// path.
func (o *object) SetAttr(ctx context.T, attrs ...storage.Attr) error {
	serviceAttrs := make([]vdlutil.Any, len(attrs))
	for i, x := range attrs {
		serviceAttrs[i] = vdlutil.Any(x)
	}
	return o.serv.SetAttr(ctx, serviceAttrs)
}

// Stat returns entry info.
func (o *object) Stat(ctx context.T) (storage.Stat, error) {
	serviceStat, err := o.serv.Stat(ctx)
	if err != nil {
		return nullStat, err
	}
	return makeStat(&serviceStat)
}

// Query returns entries matching the given query.
func (o *object) Query(ctx context.T, q query.Query) storage.QueryStream {
	stream, err := o.serv.Query(ctx, q)
	if err != nil {
		return &errorQueryStream{err}
	}
	return newQueryStream(stream)
}

// Glob returns names matching the given pattern.
func (o *object) Glob(ctx context.T, pattern string) storage.GlobStream {
	stream, err := o.serv.Glob(ctx, pattern)
	if err != nil {
		return &errorGlobStream{err}
	}
	return newGlobStream(stream)
}

// ChangeBatchStream is an interface for streaming responses of type
// ChangeBatch.
type ChangeBatchStream interface {
	// Advance stages an element so the client can retrieve it with Value.
	// Advance returns true iff there is an element to retrieve.  The client must
	// call Advance before calling Value.  The client must call Cancel if it does
	// not iterate through all elements (i.e. until Advance returns false).
	// Advance may block if an element is not immediately available.
	Advance() bool

	// Value returns the element that was staged by Advance.  Value may panic if
	// Advance returned false or was not called at all.  Value does not block.
	Value() watch.ChangeBatch

	// Err returns a non-nil error iff the stream encountered any errors.  Err
	// does not block.
	Err() error

	// Finish closes the stream and returns the positional return values for
	// call.
	Finish() error

	// Cancel notifies the stream provider that it can stop producing elements.
	Cancel()
}

// entryTransformStream implements GlobWatcherWatchGlobStream and
// QueryWatcherWatchQueryStream. It wraps a ChangeBatchStream, transforming the
// value in each change from *store.Entry to *storage.Entry.
type entryTransformStream struct {
	delegate ChangeBatchStream
	value    watch.ChangeBatch
	err      error
}

func (s *entryTransformStream) Advance() bool {
	if !s.delegate.Advance() {
		s.value = nullChangeBatch
		s.err = s.Err()
		return false
	}
	s.value = s.delegate.Value()
	// Transform the ChangeBatch in-place.
	changes := s.value.Changes
	for i := range changes {
		if changes[i].Value != nil {
			serviceEntry := changes[i].Value.(*store.Entry)
			entry, err := makeEntry(serviceEntry)
			if err != nil {
				s.value = nullChangeBatch
				s.err = err
				return false
			}
			changes[i].Value = &entry
		}
	}
	return true
}

func (s *entryTransformStream) Value() watch.ChangeBatch {
	return s.value
}

func (s *entryTransformStream) Err() error {
	return s.err
}

func (s *entryTransformStream) Finish() error {
	return s.delegate.Finish()
}

func (s *entryTransformStream) Cancel() {
	s.delegate.Cancel()
}

// WatchGlob returns a stream of changes that match a pattern.
func (o *object) WatchGlob(ctx context.T, req watch.GlobRequest) (watch.GlobWatcherWatchGlobStream, error) {
	stream, err := o.serv.WatchGlob(ctx, req, veyron2.CallTimeout(ipc.NoTimeout))
	if err != nil {
		return nil, err
	}
	return &entryTransformStream{delegate: stream}, nil
}

// WatchQuery returns a stream of changes that satisy a query.
func (o *object) WatchQuery(ctx context.T, req watch.QueryRequest) (watch.QueryWatcherWatchQueryStream, error) {
	stream, err := o.serv.WatchQuery(ctx, req, veyron2.CallTimeout(ipc.NoTimeout))
	if err != nil {
		return nil, err
	}
	return &entryTransformStream{delegate: stream}, nil
}

// errorObject responds with an error to all operations.
func (o *errorObject) Exists(ctx context.T) (bool, error) {
	return false, o.err
}

func (o *errorObject) Get(ctx context.T) (storage.Entry, error) {
	return nullEntry, o.err
}

func (o *errorObject) Put(ctx context.T, v interface{}) (storage.Stat, error) {
	return nullStat, o.err
}

func (o *errorObject) Remove(ctx context.T) error {
	return o.err
}

func (o *errorObject) SetAttr(ctx context.T, attrs ...storage.Attr) error {
	return o.err
}

func (o *errorObject) Stat(ctx context.T) (storage.Stat, error) {
	return nullStat, o.err
}

func (o *errorObject) GetAllPaths(ctx context.T) ([]string, error) {
	return nil, o.err
}

func (o *errorObject) Query(ctx context.T, q query.Query) storage.QueryStream {
	return &errorQueryStream{o.err}
}

func (o *errorObject) Glob(ctx context.T, pattern string) storage.GlobStream {
	return &errorGlobStream{o.err}
}

func (o *errorObject) Commit(ctx context.T) error {
	return o.err
}

func (o *errorObject) Abort(ctx context.T) error {
	return o.err
}

func (o *errorObject) WatchGlob(ctx context.T, req watch.GlobRequest) (watch.GlobWatcherWatchGlobStream, error) {
	return nil, o.err
}

func (o *errorObject) WatchQuery(ctx context.T, req watch.QueryRequest) (watch.QueryWatcherWatchQueryStream, error) {
	return nil, o.err
}

// errorQueryStream implements storage.QueryStream.
type errorQueryStream struct {
	err error
}

func (e *errorQueryStream) Advance() bool {
	return false
}

func (e *errorQueryStream) Value() storage.QueryResult {
	panic("No value.")
}

func (e *errorQueryStream) Err() error {
	return e.err
}

func (e *errorQueryStream) Cancel() {}

// errorGlobStream implements storage.GlobStream.
type errorGlobStream struct {
	err error
}

func (e *errorGlobStream) Advance() bool {
	return false
}

func (e *errorGlobStream) Value() string {
	panic("No value.")
}

func (e *errorGlobStream) Err() error {
	return e.err
}

func (e *errorGlobStream) Cancel() {}
