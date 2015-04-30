// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Package query_db defines the interfaces a consumer of the query package would need to
// implement.
//
// The Database interface is used to get Table interfaces (by name).
// The Table interface is used to get a KeyValueStream (by key prefixes).
// The KeyValueStream is used to consume key value pairs that match the prefixes.
//
package query_db

type Database interface {
	GetTable(name string) (Table, error)
}

type Table interface {
	// Return a KeyValueStream where all keys start with one
	// of the prefixes arguments.
	// Note: an empty string prefix (""), matches all keys.
	// The prefixes argument will be sorted (low to high).
	Scan(prefixes []string) (KeyValueStream, error)
}

// If Advance() returns true, KeyValue() will return the first/next
// key value pair.  If not advanced through the whole stream (i.e.,
// until Advance() returns false), Cancel() should be called.
// Err should be checked after every call to Advance().
type KeyValueStream interface {
	Advance() bool
	KeyValue() (string, interface{})
	Err() error
	Cancel()
}
