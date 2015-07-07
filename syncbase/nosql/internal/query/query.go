// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query

import (
	"reflect"

	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_checker"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_functions"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_parser"
	"v.io/syncbase/v23/syncbase/nosql/query_db"
	"v.io/syncbase/v23/syncbase/nosql/syncql"
	"v.io/v23/vdl"
)

type ResultStream interface {
	Advance() bool
	Result() []*vdl.Value
	Err() error
	Cancel()
}

func Exec(db query_db.Database, q string) ([]string, ResultStream, error) {
	s, err := query_parser.Parse(db, q)
	if err != nil {
		return nil, nil, err
	}
	if err := query_checker.Check(db, s); err != nil {
		return nil, nil, err
	}
	switch sel := (*s).(type) {
	case query_parser.SelectStatement:
		return execSelect(db, &sel)
	default:
		return nil, nil, syncql.NewErrExecOfUnkonwnStatementType(db.GetContext(), (*s).Offset(), reflect.TypeOf(*s).Name())
	}
}

// Given a key, a value and a SelectClause, return the projection.
// This function is only called if Eval returned true on the WhereClause expression.
func ComposeProjection(db query_db.Database, k string, v *vdl.Value, s *query_parser.SelectClause) []*vdl.Value {
	var projection []*vdl.Value
	for _, selector := range s.Selectors {
		switch selector.Type {
		case query_parser.TypSelField:
			// If field not found, nil is returned (as per specification).
			f, _, _ := ResolveField(k, v, selector.Field)
			projection = append(projection, f)
		case query_parser.TypSelFunc:
			if selector.Function.Computed {
				projection = append(projection, query_functions.ConvertFunctionRetValueToVdlValue(selector.Function.RetValue))
			} else {
				// need to exec function
				// If error executing function, return nil (as per specification).
				retValue, err := resolveArgsAndExecFunction(db, k, v, selector.Function)
				if err != nil {
					retValue = nil
				}
				projection = append(projection, query_functions.ConvertFunctionRetValueToVdlValue(retValue))
			}
		}
	}
	return projection
}

// For testing purposes, given a SelectStatement, k and v;
// return nil if row not selected, else return the projection (type []*vdl.Value).
// Note: limit and offset clauses are ignored for this function as they make no sense
// for a single row.
func ExecSelectSingleRow(db query_db.Database, k string, v *vdl.Value, s *query_parser.SelectStatement) []*vdl.Value {
	if !Eval(db, k, v, s.Where.Expr) {
		rs := []*vdl.Value{}
		return rs
	}
	return ComposeProjection(db, k, v, s.Select)
}

type resultStreamImpl struct {
	db              query_db.Database
	selectStatement *query_parser.SelectStatement
	resultCount     int64 // results served so far (needed for limit clause)
	skippedCount    int64 // skipped so far (needed for offset clause)
	keyValueStream  query_db.KeyValueStream
	k               string
	v               *vdl.Value
	err             error
}

func (rs *resultStreamImpl) Advance() bool {
	if rs.selectStatement.Limit != nil && rs.resultCount >= rs.selectStatement.Limit.Limit.Value {
		return false
	}
	for rs.keyValueStream.Advance() {
		k, v := rs.keyValueStream.KeyValue()
		// EvalWhereUsingOnlyKey
		// INCLUDE: the row should be included in the results
		// EXCLUDE: the row should NOT be included
		// FETCH_VALUE: the value and/or type of the value are required to make determination.
		rv := EvalWhereUsingOnlyKey(rs.db, rs.selectStatement, k)
		var match bool
		switch rv {
		case INCLUDE:
			match = true
		case EXCLUDE:
			match = false
		case FETCH_VALUE:
			match = Eval(rs.db, k, v, rs.selectStatement.Where.Expr)
		}
		if match {
			if rs.selectStatement.ResultsOffset == nil || rs.selectStatement.ResultsOffset.ResultsOffset.Value <= rs.skippedCount {
				rs.k = k
				rs.v = v
				rs.resultCount++
				return true
			} else {
				rs.skippedCount++
			}
		}
	}
	if err := rs.keyValueStream.Err(); err != nil {
		rs.err = syncql.NewErrKeyValueStreamError(rs.db.GetContext(), rs.selectStatement.Off, err)
	}
	return false
}

func (rs *resultStreamImpl) Result() []*vdl.Value {
	return ComposeProjection(rs.db, rs.k, rs.v, rs.selectStatement.Select)
}

func (rs *resultStreamImpl) Err() error {
	return rs.err
}

func (rs *resultStreamImpl) Cancel() {
	rs.keyValueStream.Cancel()
}

func getColumnHeadings(s *query_parser.SelectStatement) []string {
	columnHeaders := []string{}
	for _, selector := range s.Select.Selectors {
		columnName := ""
		if selector.As != nil {
			columnName = selector.As.AltName.Value
		} else {
			switch selector.Type {
			case query_parser.TypSelField:
				sep := ""
				for _, segment := range selector.Field.Segments {
					columnName = columnName + sep + segment.Value
					sep = "."
				}
			case query_parser.TypSelFunc:
				columnName = selector.Function.Name
			}
		}
		columnHeaders = append(columnHeaders, columnName)
	}
	return columnHeaders
}

func execSelect(db query_db.Database, s *query_parser.SelectStatement) ([]string, ResultStream, error) {
	keyValueStream, err := s.From.Table.DBTable.Scan(*query_checker.CompileKeyRanges(s.Where))
	if err != nil {
		return nil, nil, syncql.NewErrScanError(db.GetContext(), s.Off, err)
	}
	var resultStream resultStreamImpl
	resultStream.db = db
	resultStream.selectStatement = s
	resultStream.keyValueStream = keyValueStream
	return getColumnHeadings(s), &resultStream, nil
}
