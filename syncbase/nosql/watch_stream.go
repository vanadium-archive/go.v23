// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	"fmt"
	"reflect"
	"sync"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase/util"
	"v.io/v23/vdl"
	"v.io/v23/verror"
)

type watchStream struct {
	mu sync.Mutex
	// cancel cancels the RPC stream.
	cancel context.CancelFunc
	// call is the RPC stream object.
	call watch.GlobWatcherWatchGlobClientCall
	// curr is the currently staged element, or nil if nothing is staged.
	curr *WatchChange
	// err is the first error encountered during streaming. It may also be
	// populated by a call to Cancel.
	err error
	// finished records whether we have called call.Finish().
	finished bool
}

var _ WatchStream = (*watchStream)(nil)

func newWatchStream(cancel context.CancelFunc, call watch.GlobWatcherWatchGlobClientCall) *watchStream {
	return &watchStream{
		cancel: cancel,
		call:   call,
	}
}

// Advance implements Stream interface.
func (s *watchStream) Advance() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil || s.finished {
		return false
	}
	if !s.call.RecvStream().Advance() {
		if s.err = s.call.RecvStream().Err(); s.err == nil {
			s.err = s.call.Finish()
			s.cancel()
			s.finished = true
		}
		return false
	}
	watchChange := ToWatchChange(s.call.RecvStream().Value())
	s.curr = &watchChange

	return true
}

// Change implements WatchStream interface.
func (s *watchStream) Change() WatchChange {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.curr == nil {
		panic("nothing staged")
	}
	return *s.curr
}

// Err implements Stream interface.
func (s *watchStream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		return nil
	}
	return verror.Convert(verror.IDAction{}, nil, s.err)
}

// Cancel implements Stream interface.
// TODO(rogulenko): Make Cancel non-blocking.
func (s *watchStream) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancel()
	s.err = verror.New(verror.ErrCanceled, nil)
}

// ToWatchChange converts a generic Change struct as defined in v.io/v23/services/watch
// to a Syncbase-specific WatchChange struct as defined in v.io/v23/syncbase/nosql.
func ToWatchChange(c watch.Change) WatchChange {
	// Parse the table and the row.
	table, row, err := util.ParseTableRowPair(nil, c.Name)
	if err != nil {
		panic(err)
	}
	if row == "" {
		panic("empty row name")
	}
	// Parse the store change.
	var storeChange wire.StoreChange
	rtarget, err := vdl.ReflectTarget(reflect.ValueOf(&storeChange))
	if err != nil {
		panic(err)
	}
	if err := c.Value.FillVDLTarget(rtarget, c.Value.Type); err != nil {
		panic(err)
	}
	// Parse the state.
	var changeType ChangeType
	switch c.State {
	case watch.Exists:
		changeType = PutChange
	case watch.DoesNotExist:
		changeType = DeleteChange
	default:
		panic(fmt.Sprintf("unsupported watch change state: %v", c.State))
	}
	return WatchChange{
		Table:        table,
		Row:          row,
		ChangeType:   changeType,
		ValueBytes:   storeChange.Value,
		ResumeMarker: c.ResumeMarker,
		FromSync:     storeChange.FromSync,
		Continued:    c.Continued,
	}
}
