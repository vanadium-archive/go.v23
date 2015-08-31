// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query_exec

import (
	"strconv"
	"strings"
	"v.io/v23/syncbase/nosql"
	"v.io/v23/syncbase/nosql/internal/query"
	"v.io/v23/syncbase/nosql/query_db"
)

// Exec executes a syncQL query and returns all results as specified by
// in the query's select clause.  Headers (i.e., column names) are returned
// separately from the result stream.
func Exec(db query_db.Database, q string) ([]string, nosql.ResultStream, error) {
	return query.Exec(db, q)
}

// Split an error message into an offset and the remaining (i.e., rhs of offset) message.
// The convention for syncql is "<module><optional-rpc>[offset]<remaining-message>".
// TODO(jkline): find a better place for client utilities (which, in this case, is also
// used by internal tests).
func SplitError(err error) (int64, string) {
	errMsg := err.Error()
	idx1 := strings.Index(errMsg, "[")
	idx2 := strings.Index(errMsg, "]")
	if idx1 == -1 || idx2 == -1 {
		return 0, errMsg
	}
	offsetString := errMsg[idx1+1 : idx2]
	offset, err := strconv.ParseInt(offsetString, 10, 64)
	if err != nil {
		return 0, errMsg
	}
	return offset, errMsg[idx2+1:]
}
