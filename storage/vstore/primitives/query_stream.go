package primitives

import (
	"errors"
	"sort"
	"sync"

	"veyron2/naming"
	"veyron2/services/store"
	"veyron2/storage"
)

// newQueryStream constructs a storage.QueryStream from the given
// store.ObjectQueryStream.
func newQueryStream(stream store.ObjectQueryStream) storage.QueryStream {
	return &queryStream{stream: &serverStream{stream: stream}}
}

// internalQueryStream exists so we can find our implementations of
// storage.QueryStream without accidentally getting objects that just
// happen to implement the same interface.  See the typecast in
// cancelSubstreams().
type internalQueryStream interface {
	storage.QueryStream
	isInternalQueryStream()
}

// serverStream is a wrapper around store.ObjectQueryStream.  One serverStream
// is shared by all of the nested streams that make up a set of query results.
//
// serverStream supports rewinding the stream by one result.  This is
// necessary because each nested storage.QueryStream has to read one result
// too many to detect that there are no more results to read.
type serverStream struct {
	// stream contains the interleaved query results as sent by the server.
	stream store.ObjectQueryStream

	// mu protects all fields below.
	mu sync.Mutex
	// curr is the result that should be returned by Value().
	curr *store.QueryResult
	// next is the result that should be moved to curr by a call to Advance.
	// Rewind moves curr to next.  If next is nil, Advance should pull a result
	// from stream.
	next *store.QueryResult
	// err is the first error encountered from stream.  It may also be populated
	// by a call to Cancel.
	err error
}

// Advance stages an element so the client can retrieve it with Value.
// Advance returns true iff there is an element to retrieve.  The client must
// call Advance before calling Value.  The client must call Cancel if it does
// not iterate through all elements (i.e. until Advance returns false).
// Advance may block if an element is not immediately available.
func (r *serverStream) Advance() bool {
	r.mu.Lock()
	if r.err != nil {
		r.mu.Unlock()
		return false
	}
	if r.next != nil {
		r.curr = r.next
		r.next = nil
		r.mu.Unlock()
		return true
	}
	r.mu.Unlock()
	// Don't hold the lock while waiting for the next result.
	hasValue := r.stream.Advance()
	r.mu.Lock()
	defer r.mu.Unlock()
	// The client might have called Cancel() while we were blocked on Advance().
	if r.err != nil {
		return false
	}
	if !hasValue {
		if r.stream.Err() != nil {
			r.err = r.stream.Err()
		} else {
			r.err = r.stream.Finish()
		}
		return false
	}
	curr := r.stream.Value()
	r.curr = &curr
	r.next = nil
	return true
}

// Rewind moves the stream back one position.  After calling Rewind, the client
// must call Advance before calling Value or Rewind.
func (r *serverStream) Rewind() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.curr == nil {
		panic("can't rewind any further")
	}
	r.next = r.curr
	r.curr = nil
}

// Value returns the element that was staged by Advance.  Value may panic if
// Advance returned false or was not called at all.  Value does not block.
func (r *serverStream) Value() *store.QueryResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.curr == nil {
		panic("need to call advance first")
	}
	return r.curr
}

// Err returns a non-nil error iff the stream encountered any errors.  Err
// does not block.
func (r *serverStream) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

// Cancel notifies the stream provider that it can stop producing elements.
// The client must call Cancel if it does not iterate through all elements
// (i.e. until Advance returns false).  Cancel is idempotent and can be called
// concurrently with a goroutine that is iterating via Advance/Value.  Cancel
// causes Advance to subsequently return false.  Cancel does not block.
func (r *serverStream) Cancel() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stream.Cancel()
	r.err = errors.New("cancelled by client")
}

// queryStream is an implementation of storage.QueryStream that
// processes one result at a time.  This allows for a very large
// number of results to be processed without keeping them all
// in memory at the same time.
type queryStream struct {
	// stream is the interleaved set of results coming from the server. This one
	// object is shared with all parent and child storage.QueryStreams.
	stream *serverStream
	// nesting is the value for all result objects to be produced by
	// this QueryStream.  If a result has a NestedResult that is not
	// the same as nesting, that result must be for another QueryStream.
	nesting store.NestedResult
	// namePrefix should be prepended to the name of all results.
	namePrefix string

	// mu protects all fields below.
	mu sync.Mutex
	// curr is the result that was most recently processed by Advance.
	curr storage.QueryResult
	// done is true iff all future calls to Advance should return false.
	done bool
}

// Advance implements the storage.QueryStream method.
func (s *queryStream) Advance() bool {
	s.mu.Lock()
	if s.done {
		s.mu.Unlock()
		return false
	}
	s.mu.Unlock()

	// Don't call any blocking functions while holding the lock.
	var translated storage.QueryResult
	advanced := s.stream.Advance()
	if advanced {
		res := s.stream.Value()
		if res.NestedResult != s.nesting {
			s.stream.Rewind()
			advanced = false
		} else {
			translated = translateResult(s.stream, res, s.namePrefix)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.curr != nil {
		cancelSubstreams(s.curr)
		s.curr = nil
	}
	if !advanced {
		s.done = true
		return false
	}
	s.curr = translated
	return true
}

// Value implements the storage.QueryStream method.
func (s *queryStream) Value() storage.QueryResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stream.Err() != nil {
		panic("can't call Value when there is an error")
	}
	return s.curr
}

// Err implements the storage.QueryStream method.
func (s *queryStream) Err() error {
	return s.stream.Err()
}

// Cancel implements the storage.QueryStream method.
func (s *queryStream) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Send the cancel to the server if this is the top-level stream.
	if s.nesting == 0 {
		s.stream.Cancel()
	}
	if s.curr != nil {
		cancelSubstreams(s.curr)
	}
	s.done = true
}

// isInternalQueryStream implements the internalQueryStream method.
func (s *queryStream) isInternalQueryStream() {}

// nestedStream makes it possible to sort streams by the order that they
// come from the server.
type nestedStream struct {
	nesting store.NestedResult
	field   string
}

type byNesting []nestedStream

func (n byNesting) Len() int           { return len(n) }
func (n byNesting) Swap(i, j int)      { n[i], n[j] = n[j], n[i] }
func (n byNesting) Less(i, j int) bool { return n[i].nesting < n[j].nesting }

// translateResult recursively converts a store.QueryResult into a
// storage.QueryResult.  For any nested streams, it greedily pulls results
// from stream.  namePrefix is prepended to the names of all converted
// results.
//
// TODO(kash): If the nested result sets use a lot of memory, they could cause
// the app to crash.  It would be preferable to send these nested results via
// a separate rpc so they could be pulled off the network on demand.  We have
// to do it greedily because the API allows the client app to access the
// nested results in arbitrary order, but stream sends them in a fixed order.
// Possible solutions:
// 1) For each substream, the client opens a new rpc to the server.
//    Not many changes are necessary other than a way for the server to
//    specify that a nested result should be fetched via a separate rpc.
//    The downside is that the client must wait for the new rpcs to be
//    set up (i.e. at least one roundtrip) before it can start processing
//    the sub-results.
// 2) For each substream, the server opens a new rpc to the client.  This
//    requires changes to the Veyron rpc system to reuse the existing
//    rpc channel.  The advantage of this option over (1) is that the server
//    can preemptively send sub-results to the client.
// 3) Allow the client to send commands to the server via the existing rpc
//    stream.  For example, as the client app starts processing a substream,
//    the client library would send a command like, "send me results for
//    nested stream N".  This has the downside of building an rpc system
//    on top of an rpc system.
// 4) Send small nested result sets inline like the current code, and send
//    large nesed result sets via (1), (2), or (3).
func translateResult(stream *serverStream, res *store.QueryResult, namePrefix string) storage.QueryResult {
	if res.Value != nil {
		return &queryResult{
			name:  naming.Join(namePrefix, res.Name),
			value: res.Value,
		}
	}
	trans := &queryResult{
		name:   naming.Join(namePrefix, res.Name),
		fields: make(map[string]interface{}),
	}
	var nestings []nestedStream
	for k, v := range res.Fields {
		if nesting, ok := v.(store.NestedResult); ok {
			nestings = append(nestings, nestedStream{nesting, k})
		} else {
			trans.fields[k] = interface{}(v)
		}
	}
	sort.Sort(byNesting(nestings))
	for _, n := range nestings {
		substream := newInlineQueryStream(stream, n.nesting, trans.name)
		trans.fields[n.field] = substream
	}
	return trans
}

// cancelSubstreams calls Cancel on any substreams in res.
func cancelSubstreams(res storage.QueryResult) {
	if res.Fields() != nil {
		for _, v := range res.Fields() {
			if substream, ok := v.(internalQueryStream); ok {
				substream.Cancel()
			}
		}
	}
}

// inlineQueryStream is an implementation of storage.QueryStream that stores
// all of its results in an array.
type inlineQueryStream struct {
	// stream is the source of results.  Once newInlineQueryStream has pulled
	// all of the results for this inlineQueryStream, stream is unused other
	// than to detect if there was an error.
	stream *serverStream

	// mu protects all of the fields below.
	mu sync.Mutex
	// results is the set of results for this stream.
	results []storage.QueryResult
	// curr is the current offset into results.  -1 indicates that iteration
	// has not yet started.
	curr int
}

// newInlineQueryStream greedily pulls from stream all results with the given
// nesting.  These results names are prepended with namePrefix.  If there
// are any nested results, those are pulled from stream as well.
func newInlineQueryStream(stream *serverStream, nesting store.NestedResult,
	namePrefix string) storage.QueryStream {

	var results []storage.QueryResult
	for stream.Advance() {
		res := stream.Value()
		if res.NestedResult != nesting {
			stream.Rewind()
			break
		}
		results = append(results, translateResult(stream, res, namePrefix))
	}
	return &inlineQueryStream{
		stream:  stream,
		results: results,
		curr:    -1,
	}
}

// Advance implements the storage.QueryStream method.
func (s *inlineQueryStream) Advance() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stream.Err() != nil {
		return false
	}
	if s.curr >= 0 {
		cancelSubstreams(s.results[s.curr])
		// Allow the curr element to be garbage collected.
		s.results[s.curr] = nil
	}
	s.curr++
	return s.curr < len(s.results)
}

// Value implements the storage.QueryStream method.
func (s *inlineQueryStream) Value() storage.QueryResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.curr >= len(s.results) {
		panic("no more results")
	}
	if s.curr == -1 {
		panic("need to call Advance before calling Value")
	}
	return s.results[s.curr]
}

// Err implements the storage.QueryStream method.
func (s *inlineQueryStream) Err() error {
	return s.stream.Err()
}

// Cancel implements the storage.QueryStream method.
func (s *inlineQueryStream) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results = nil
}

// isInternalQueryStream implements the internalQueryStream method.
func (s *inlineQueryStream) isInternalQueryStream() {}

// queryResult implements the storage.QueryResult interface.
type queryResult struct {
	name   string
	value  interface{}
	fields map[string]interface{}
}

// Name implements the storage.QueryResult method.
func (r *queryResult) Name() string {
	return r.name
}

// Value implements the storage.QueryResult method.
func (r *queryResult) Value() interface{} {
	return r.value
}

// Fields implements the storage.QueryResult method.
func (r *queryResult) Fields() map[string]interface{} {
	return r.fields
}
