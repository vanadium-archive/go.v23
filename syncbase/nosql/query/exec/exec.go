// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package exec defines an Exec function that evaluates a given query against
// the interfaces defined in the query package.
package exec

import (
	"strconv"
	"strings"
	"v.io/v23/syncbase/nosql/query"
	"v.io/v23/syncbase/nosql/query/internal"
)

// Exec executes a syncQL query and returns the results. Headers (i.e., column
// names) are returned separately from results (i.e., values).
func Exec(db query.Database, q string) ([]string, query.ResultStream, error) {
	return internal.Exec(db, q)
}

// SplitError splits an error message into an offset and the remaining (i.e.,
// rhs of offset) message.
// The query convention is "<module><optional-rpc>[offset]<remaining-message>".
// TODO(jkline): find a better place for client utilities (which, in this case,
// is also used by internal tests).
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
