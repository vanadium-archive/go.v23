// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query_exec

import (
	"v.io/syncbase/v23/syncbase/nosql"
	"v.io/syncbase/v23/syncbase/nosql/internal/query"
	"v.io/syncbase/v23/syncbase/nosql/query_db"
)

// Exec executes a syncQL query and returns all results as specified by
// in the query's select clause.  Headers (i.e., column names) are returned
// separately from the result stream.
func Exec(db query_db.Database, q string) ([]string, nosql.ResultStream, error) {
	return query.Exec(db, q)
}
