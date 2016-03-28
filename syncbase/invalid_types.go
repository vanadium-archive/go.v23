// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

// InvalidScanStream is a ScanStream for which all methods return errors.
// TODO(sadovsky): Make InvalidScanStream private.
type InvalidScanStream struct {
	Error error // returned by all methods
}

var (
	_ ScanStream = (*InvalidScanStream)(nil)
)

////////////////////////////////////////////////////////////
// InvalidScanStream

// Advance implements the Stream interface.
func (s *InvalidScanStream) Advance() bool {
	return false
}

// Err implements the Stream interface.
func (s *InvalidScanStream) Err() error {
	return s.Error
}

// Cancel implements the Stream interface.
func (s *InvalidScanStream) Cancel() {
}

// Key implements the ScanStream interface.
func (s *InvalidScanStream) Key() string {
	panic(s.Error)
}

// Value implements the ScanStream interface.
func (s *InvalidScanStream) Value(value interface{}) error {
	panic(s.Error)
}
