// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

// InvalidStream is a nosql.Stream for which all methods return errors.
type InvalidStream struct {
	Error error // returned by all methods
}

var (
	_ Stream = (*InvalidStream)(nil)
)

////////////////////////////////////////////////////////////
// InvalidStream

// Advance implements Stream.Advance.
func (s *InvalidStream) Advance() bool {
	return false
}

// Key implements Stream.Key.
func (s *InvalidStream) Key() string {
	panic(s.Error)
}

// Value implements Stream.Value.
func (s *InvalidStream) Value(value interface{}) error {
	panic(s.Error)
}

// Err implements Stream.Err.
func (s *InvalidStream) Err() error {
	return s.Error
}

// Cancel implements Stream.Cancel.
func (s *InvalidStream) Cancel() {
}
