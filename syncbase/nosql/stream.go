// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	"sync"

	wire "v.io/syncbase/v23/services/syncbase/nosql"
	"v.io/v23/context"
	"v.io/v23/verror"
	"v.io/v23/vom"
)

type stream struct {
	mu sync.Mutex
	// cancel cancels the RPC stream.
	cancel context.CancelFunc
	// call is the RPC stream object.
	call wire.TableScanClientCall
	// curr is the currently staged key-value pair, or nil if nothing is staged.
	curr *wire.KeyValue
	// err is the first error encountered during streaming. It may also be
	// populated by a call to Cancel.
	err error
	// finished records whether we have called call.Finish().
	finished bool
}

var _ Stream = (*stream)(nil)

func newStream(cancel context.CancelFunc, call wire.TableScanClientCall) *stream {
	return &stream{
		cancel: cancel,
		call:   call,
	}
}

// Advance implements Stream.Advance.
func (s *stream) Advance() bool {
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

// Key implements Stream.Key.
func (s *stream) Key() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.curr == nil {
		panic("nothing staged")
	}
	return s.curr.Key
}

// Value implements Stream.Value.
func (s *stream) Value(value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.curr == nil {
		panic("nothing staged")
	}
	return vom.Decode(s.curr.Value, value)
}

// Err implements Stream.Err.
func (s *stream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		return nil
	}
	return verror.Convert(verror.IDAction{}, nil, s.err)
}

// Cancel implements Stream.Cancel.
// TODO(sadovsky): Make Cancel non-blocking.
func (s *stream) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancel()
	s.err = verror.New(verror.ErrCanceled, nil)
}
