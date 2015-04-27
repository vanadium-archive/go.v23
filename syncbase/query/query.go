// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package query performs SQL-like select queries on the Syncbase NoSQL database.
//
// Note: Presently, the query package is deliberately not depending on other parts of syncbase.
// This will change at a future date.

package query

import (
	"errors"
	"fmt"
	"reflect"
	"v.io/syncbase/v23/syncbase/query/query_checker"
	"v.io/syncbase/v23/syncbase/query/query_parser"
)

// TODO(jkline): Flesh out this interface.
type Store interface {
	CheckTable(table string) error
	// TODO(jkline): GetKeys will need to be streaming in real life.
	GetKeys(prefix string) ([]string, error)
	GetValue(k string) (interface{}, error)
}

// TODO(jkline): Flesh out this interface.
type ResultStream interface {
	Advance() bool
	Result() *KV
	Err() *QueryError
	Cancel()
}

// TODO(jkline): Flesh out this struct.
type KV struct {
	K string
	V interface{}
}

// TODO(jkline): Flesh out this struct.
type ResultStreamImpl struct {
}

// TODO(jkline): Flesh out this function.
func (r ResultStreamImpl) Advance() bool {
	return true
}

// TODO(jkline): Flesh out this function.
func (r ResultStreamImpl) Result() *KV {
	return nil
}

// TODO(jkline): Flesh out this function.
func (r ResultStreamImpl) Err() *QueryError {
	return nil
}

// TODO(jkline): Flesh out this function.
func (r ResultStreamImpl) Cancel() {
}

type QueryError struct {
	SynErr *query_parser.SyntaxError
	SemErr *query_checker.SemanticError
	Msg    string
	Off    int64
}

func (e *QueryError) Error() string {
	if e.SynErr != nil {
		return e.SynErr.Error()
	} else if e.SemErr != nil {
		return e.SemErr.Error()
	} else {
		return fmt.Sprintf("[Off:%d] %s", e.Off, e.Msg)
	}
}

func Error(offset int64, msg string) *QueryError {
	return &QueryError{Msg: msg, Off: offset}
}

func ErrorFromSyntax(synerr *query_parser.SyntaxError) *QueryError {
	return &QueryError{SynErr: synerr}
}

func ErrorFromSemantic(semerr *query_checker.SemanticError) *QueryError {
	return &QueryError{SemErr: semerr}
}

func Exec(db Store, q string) (ResultStream, *QueryError) {
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

func ExecSelect(db Store, s *query_parser.SelectStatement) (ResultStream, *QueryError) {
	_ = CompileKeyPrefixes(s.Where)
	return ResultStreamImpl{}, nil
}
