// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"sync"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
	"v.io/v23/vom"
)

type scanStream struct {
	mu sync.Mutex
	// cancel cancels the RPC stream.
	cancel context.CancelFunc
	// call is the RPC stream object.
	call wire.CollectionScanClientCall
	// curr is the currently staged key-value pair, or nil if nothing is staged.
	curr *wire.KeyValue
	// err is the first error encountered during streaming. It may also be
	// populated by a call to Cancel.
	err error
	// finished records whether we have called call.Finish().
	finished bool
}

var _ ScanStream = (*scanStream)(nil)

func newScanStream(cancel context.CancelFunc, call wire.CollectionScanClientCall) *scanStream {
	return &scanStream{
		cancel: cancel,
		call:   call,
	}
}

// Advance implements the Stream interface.
func (s *scanStream) Advance() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil || s.finished {
		return false
	}
	if !s.call.RecvStream().Advance() {
		if s.call.RecvStream().Err() != nil {
			s.err = s.call.RecvStream().Err()
		} else {
			s.err = s.call.Finish()
			s.cancel()
			s.finished = true
		}
		return false
	}
	curr := s.call.RecvStream().Value()
	s.curr = &curr
	return true
}

// Key implements the ScanStream interface.
func (s *scanStream) Key() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.curr == nil {
		panic("nothing staged")
	}
	return s.curr.Key
}

// Value implements the ScanStream interface.
func (s *scanStream) Value(value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.curr == nil {
		panic("nothing staged")
	}
	return vom.Decode(s.curr.Value, value)
}

// Err implements the Stream interface.
func (s *scanStream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		return nil
	}
	return verror.Convert(verror.IDAction{}, nil, s.err)
}

// Cancel implements the Stream interface.
// TODO(sadovsky): Make Cancel non-blocking.
func (s *scanStream) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancel()
	s.err = verror.New(verror.ErrCanceled, nil)
}
