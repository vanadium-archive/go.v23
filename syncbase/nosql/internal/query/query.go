// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Package query performs SQL-like select queries on the Syncbase NoSQL database.
//
// Note: Presently, the query package is deliberately not depending on other parts of syncbase.
// This will change at a future date.

package query

import (
	"errors"
	"fmt"
	"reflect"

	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_checker"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_db"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_parser"
)

type QueryError struct {
	Msg string
	Off int64
}

func (e *QueryError) Error() string {
	return fmt.Sprintf("[Off:%d] %s", e.Off, e.Msg)
}

func Error(offset int64, msg string) *QueryError {
	return &QueryError{msg, offset}
}

func ErrorFromSyntax(synerr *query_parser.SyntaxError) *QueryError {
	return &QueryError{synerr.Msg, synerr.Off}
}

func ErrorFromSemantic(semerr *query_checker.SemanticError) *QueryError {
	return &QueryError{semerr.Msg, semerr.Off}
}

func Exec(db query_db.Database, q string) (ResultStream, *QueryError) {
	s, err := query_parser.Parse(q)
	if err != nil {
		return nil, ErrorFromSyntax(err)
	}
	if err := query_checker.Check(db, s); err != nil {
		return nil, ErrorFromSemantic(err)
	}
	switch sel := (*s).(type) {
	case query_parser.SelectStatement:
		return ExecSelect(db, &sel)
	default:
		return nil, Error((*s).Offset(), fmt.Sprintf("Cannot exec statement type %v", reflect.TypeOf(*s)))
	}
}

// Given a key, a value and a SelectClause, return the projection.
// This function is only called if Eval returned true on the WhereClause expression.
func ComposeProjection(k string, v interface{}, s *query_parser.SelectClause) []interface{} {
	var projection []interface{}
	for _, f := range s.Columns {
		if query_checker.IsKeyField(&f) {
			projection = append(projection, k)
		} else {
			// If field not found, nil is returned (as per specification).
			projection = append(projection, ResolveField(v, &f))
		}
	}
	return projection
}

// Given a query (i.e.,, select statement), return the key prefixes needed to satisfy the query.
// A return of a single empty string ([]string{ "" }) means fetch all keys.
func CompileKeyPrefixes(w *query_parser.WhereClause) []string {
	// First determine if every key needs to be fetched.  To do this, evaluate the
	// where clause substituting false for every key expression and true for every
	// other (type for value) expression.  If the where clause evaluates to true,
	// it is possible for a row to be selected without any dependence on the contents
	// of the key.  In that case, all keys must be fetched.
	if w == nil || AllKeysMustBeFetched(w.Expr) {
		return []string{""}
	} else {
		return query_checker.CompileKeyPrefixes(w)
	}
}

// Evaluate the where clause, substituting false for all expressions involving the key and
// true for all other expressions.  If the answer is true, it is possible to satisfy the
// expression for any key.  As such, all keys must be fetched.
func AllKeysMustBeFetched(e *query_parser.Expression) bool {
	switch e.Operator.Type {
	case query_parser.And:
		return AllKeysMustBeFetched(e.Operand1.Expr) && AllKeysMustBeFetched(e.Operand2.Expr)
	case query_parser.Or:
		return AllKeysMustBeFetched(e.Operand1.Expr) || AllKeysMustBeFetched(e.Operand2.Expr)
	default: // =, > >=, <, <=, Like, <>, NotLike
		if query_checker.IsKey(e.Operand1) {
			return false
		} else {
			return true
		}
	}
}

// Evaluate the where clause to determine if the row should be selected, but do so using only
// the key.  Possible returns are:
// true: the row should included in the results
// false: the row should NOT be included
// error: the value and/or type of the value are required to determine if row should be included.
// The above decision is accomplished by evaluating all expressions which reference the key and
// substituing false for all other expressions.  If the result is true, true is returned.
// If the result is false, but no other experssions (i.e., expressions which refer to the type of
// of the value or the value itself) were encountered, false is returned; else, an error is
// returned indicating the value must be fetched in order to determine if the row should be included
// in the results.
func EvalWhereUsingOnlyKey(s *query_parser.SelectStatement, k string) (bool, error) {
	if s.Where == nil { // all rows will be in result
		return true, nil
	}
	return EvalExprUsingOnlyKey(s.Where.Expr, k)
}

func EvalExprUsingOnlyKey(e *query_parser.Expression, k string) (bool, error) {
	switch e.Operator.Type {
	case query_parser.And:
		op1Result, err1 := EvalExprUsingOnlyKey(e.Operand1.Expr, k)
		op2Result, err2 := EvalExprUsingOnlyKey(e.Operand2.Expr, k)
		if op1Result && op2Result {
			return true, nil
		} else if (op1Result == false && err1 == nil) || (op2Result == false && err2 == nil) {
			// One of the operands evaluated to false with no error.
			// As such, the value is not needed to reject the row.
			return false, nil
		} else {
			if err1 != nil {
				return false, err1
			} else {
				return false, err2
			}
		}
	case query_parser.Or:
		op1Result, err1 := EvalExprUsingOnlyKey(e.Operand1.Expr, k)
		op2Result, err2 := EvalExprUsingOnlyKey(e.Operand2.Expr, k)
		if op1Result || op2Result {
			return true, nil
		} else {
			if err1 != nil {
				return false, err1
			} else {
				return false, err2 // err2 may or may not be nil
			}
		}
	default: // =, > >=, <, <=, Like, <>, NotLike
		if !query_checker.IsKey(e.Operand1) {
			// Non-key expressions are evaluated as false.
			return false, errors.New("Value required for answer.") // err text not used
		} else {
			return EvalKeyExpression(e, k), nil
		}
	}
}

// For testing purposes, given a SelectStatement, k and v;
// return nil if row not selected, else return the projection (type []interface{}).
// Note: limit and offset clauses are ignored for this function as they make no sense
// for a single row.
func ExecSelectSingleRow(k string, v interface{}, s *query_parser.SelectStatement) interface{} {
	if !Eval(k, v, s.Where.Expr) {
		return nil
	}
	return ComposeProjection(k, v, s.Select)
}

type ResultStream interface {
	Advance() bool
	Result() []interface{}
	Err() *QueryError
	Cancel()
}

type ResultStreamImpl struct {
	selectStatement *query_parser.SelectStatement
	resultCount     int64 // results served so far (needed for limit clause)
	skippedCount    int64 // skipped so far (needed for offset clause)
	keyValueStream  query_db.KeyValueStream
	k               string
	v               interface{}
	err             *QueryError
}

func (rs *ResultStreamImpl) Advance() bool {
	if rs.selectStatement.Limit != nil && rs.resultCount >= rs.selectStatement.Limit.Limit.Value {
		return false
	}
	for rs.keyValueStream.Advance() {
		if err := rs.keyValueStream.Err(); err != nil {
			rs.err = Error(rs.selectStatement.Off, err.Error())
			return false
		}
		k, v := rs.keyValueStream.KeyValue()
		if err := rs.keyValueStream.Err(); err != nil {
			rs.err = Error(rs.selectStatement.Off, err.Error())
			return false
		}
		// EvalWhereUsingOnlyKey
		// true: the row should included in the results
		// false: the row should NOT be included
		// error: the value and/or type of the value are required to determine...
		match, err := EvalWhereUsingOnlyKey(rs.selectStatement, k)
		if err != nil {
			match = Eval(k, v, rs.selectStatement.Where.Expr)
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
	return false
}

func (rs *ResultStreamImpl) Result() []interface{} {
	return ComposeProjection(rs.k, rs.v, rs.selectStatement.Select)
}

func (rs *ResultStreamImpl) Err() *QueryError {
	return rs.err
}

func (rs *ResultStreamImpl) Cancel() {
	rs.keyValueStream.Cancel()
}

func ExecSelect(db query_db.Database, s *query_parser.SelectStatement) (ResultStream, *QueryError) {
	prefixes := CompileKeyPrefixes(s.Where)
	keyValueStream, err := s.From.Table.DBTable.Scan(prefixes)
	if err != nil {
		return nil, Error(s.Off, err.Error())
	}
	var resultStream ResultStreamImpl
	resultStream.selectStatement = s
	resultStream.keyValueStream = keyValueStream
	return &resultStream, nil
}
