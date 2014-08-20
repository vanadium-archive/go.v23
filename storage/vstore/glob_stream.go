package vstore

// TODO(sadovsky): Write detailed unit tests. For now, globStream is tested
// indirectly via vstore_test.go.

import (
	"errors"
	"sync"

	"veyron2/services/mounttable"
	"veyron2/storage"
)

// newGlobStream constructs a storage.GlobStream from the given
// mounttable.GlobbableGlobStream.
func newGlobStream(call mounttable.GlobbableGlobCall) storage.GlobCall {
	return &globStream{call: call}
}

type globStream struct {
	call mounttable.GlobbableGlobCall
	// mu protects all fields below.
	mu sync.Mutex
	// curr is the result that should be returned by Value().
	curr *mounttable.MountEntry
	// err is the first error encountered from stream.  It may also be populated
	// by a call to Cancel.
	err error
}

// Advance stages an element so the client can retrieve it with Value.
// Advance returns true iff there is an element to retrieve.  The client must
// call Advance before calling Value.  The client must call Cancel if it does
// not iterate through all elements (i.e. until Advance returns false).
// Advance may block if an element is not immediately available.
func (s *globStream) Advance() bool {
	// Don't hold the lock while waiting for the next result.
	rStream := s.call.RecvStream()
	hasValue := rStream.Advance()
	s.mu.Lock()
	defer s.mu.Unlock()
	// The client might have called Cancel() while we were blocked on Advance().
	if s.err != nil {
		return false
	}
	if !hasValue {
		if rStream.Err() != nil {
			s.err = rStream.Err()
		} else {
			s.err = s.call.Finish()
		}
		return false
	}
	curr := rStream.Value()
	s.curr = &curr
	return true
}

func (s *globStream) RecvStream() storage.GlobStream {
	return s
}

// Value returns the element that was staged by Advance.  Value may panic if
// Advance returned false or was not called at all.  Value does not block.
func (s *globStream) Value() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.curr == nil {
		panic("need to call advance first")
	}
	return s.curr.Name
}

// Err returns a non-nil error iff the stream encountered any errors.  Err
// does not block.
func (s *globStream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Cancel notifies the stream provider that it can stop producing elements.
// The client must call Cancel if it does not iterate through all elements
// (i.e. until Advance returns false).  Cancel is idempotent and can be called
// concurrently with a goroutine that is iterating via Advance/Value.  Cancel
// causes Advance to subsequently return false.  Cancel does not block.
func (s *globStream) Cancel() {
	s.mu.Lock()
	s.err = errors.New("cancelled by client")
	s.mu.Unlock()
	s.call.Cancel()
}
