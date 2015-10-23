// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package engine defines an Exec function while performs a syncQL query
// and returns a syncql.ResultStream.
package engine

import (
	"v.io/v23/query/syncql"
	ds "v.io/v23/query/engine/datasource"
	"v.io/v23/query/engine/internal"
)

// Exec executes a syncQL query and returns the results. Headers (i.e., column
// names) are returned separately from results (i.e., values).
// db: an implementation of datasource.Database
// q : the query (e.g., select v from Customers
func Exec(db ds.Database, q string) ([]string, syncql.ResultStream, error) {
	return internal.Exec(db, q)
}
