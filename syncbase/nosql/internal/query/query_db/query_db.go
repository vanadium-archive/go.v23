// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query_db

import (
	"v.io/v23/vdl"
)

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

type KeyValueStream interface {
	// Advance stages an element so the client can retrieve it
	// with KeyValue.  Advance returns true iff there is an
	// element to retrieve.  The client must call Advance before
	// calling KeyValue.  The client must call Cancel if it does
	// not iterate through all elements (i.e. until Advance
	// returns false).  Advance may block if an element is not
	// immediately available.
	Advance() bool

	// KeyValue returns the element that was staged by Advance.
	// KeyValue may panic if Advance returned false or was not
	// called at all.  KeyValue does not block.
	KeyValue() (string, *vdl.Value)

	// Err returns a non-nil error iff the stream encountered
	// any errors.  Err does not block.
	Err() error

	// Cancel notifies the stream provider that it can stop
	// producing elements.  The client must call Cancel if it does
	// not iterate through all elements (i.e. until Advance
	// returns false).  Cancel is idempotent and can be called
	// concurrently with a goroutine that is iterating via
	// Advance/Value.  Cancel causes Advance to subsequently
	// return  false.  Cancel does not block.
	Cancel()
}
