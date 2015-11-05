// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package datasource defines the interfaces a system must implement to support
// querying.
//
// The Database interface is used to get Table interfaces (by name).
// The Table interface is used to get a KeyValueStream (by key prefixes).
// The KeyValueStream interface is used to iterate over key-value pairs from a
// table.
package datasource

import (
	"v.io/v23/context"
	"v.io/v23/vdl"
)

type Database interface {
	GetContext() *context.T
	GetTable(name string) (Table, error)
}

type Table interface {
	// Return a KeyValueStream where all keys start with one
	// of the prefixes arguments.
	// Note: an empty string prefix (""), matches all keys.
	// The prefixes argument will be sorted (low to high).
	Scan(keyRanges KeyRanges) (KeyValueStream, error)
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

type KeyRange struct {
	Start string
	Limit string
}

type KeyRanges []KeyRange

// Implement sort interface for KeyRanges.
func (keyRanges KeyRanges) Len() int {
	return len(keyRanges)
}

func (keyRanges KeyRanges) Less(i, j int) bool {
	return keyRanges[i].Start < keyRanges[j].Start
}

func (keyRanges KeyRanges) Swap(i, j int) {
	saveStart := keyRanges[i].Start
	saveLimit := keyRanges[i].Limit
	keyRanges[i].Start = keyRanges[j].Start
	keyRanges[i].Limit = keyRanges[j].Limit
	keyRanges[j].Start = saveStart
	keyRanges[j].Limit = saveLimit
}
