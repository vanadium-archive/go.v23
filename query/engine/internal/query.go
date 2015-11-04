// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"fmt"
	"reflect"
	"strconv"
	"sync"

	ds "v.io/v23/query/engine/datasource"
	"v.io/v23/query/engine/internal/query_checker"
	"v.io/v23/query/engine/internal/query_functions"
	"v.io/v23/query/engine/internal/query_parser"
	"v.io/v23/query/syncql"
	"v.io/v23/vdl"
)

type queryEngineImpl struct {
	db                      ds.Database
	mutexNextID             sync.Mutex
	nextID                  int64
	mutexPreparedStatements sync.Mutex
	preparedStatements      map[int64]*query_parser.Statement
}

type preparedStatementImpl struct {
	qe *queryEngineImpl
	id int64 // key to AST stored in queryEngineImpl.
}

type paramInfo struct {
	paramValues []*vdl.Value
	cursor      int64
}

func Create(db ds.Database) ds.QueryEngine {
	return &queryEngineImpl{db: db, nextID: 0, preparedStatements: map[int64]*query_parser.Statement{}}
}

func (qe *queryEngineImpl) Exec(q string) ([]string, syncql.ResultStream, error) {
	return Exec(qe.db, q)
}

func (qe *queryEngineImpl) GetPreparedStatement(v *vdl.Value) (ds.PreparedStatement, error) {
	if v == nil || v.Kind() != vdl.Int64 {
		return nil, syncql.NewErrPreparedStatementNotFound(qe.db.GetContext())
	}
	qe.mutexPreparedStatements.Lock()
	_, ok := qe.preparedStatements[v.Int()]
	qe.mutexPreparedStatements.Unlock()
	if ok {
		return &preparedStatementImpl{qe, v.Int()}, nil
	} else {
		return nil, syncql.NewErrPreparedStatementNotFound(qe.db.GetContext())
	}
}

func (qe *queryEngineImpl) PrepareStatement(q string) (ds.PreparedStatement, error) {
	s, err := query_parser.Parse(qe.db, q)
	if err != nil {
		return nil, err
	}
	qe.mutexNextID.Lock()
	id := qe.nextID
	qe.nextID++
	qe.mutexNextID.Unlock()
	qe.mutexPreparedStatements.Lock()
	qe.preparedStatements[id] = s
	qe.mutexPreparedStatements.Unlock()
	return &preparedStatementImpl{qe, id}, nil
}

func (p *preparedStatementImpl) Exec(paramValues ...*vdl.Value) ([]string, syncql.ResultStream, error) {
	// Find the AST
	p.qe.mutexPreparedStatements.Lock()
	s := p.qe.preparedStatements[p.id]
	p.qe.mutexPreparedStatements.Unlock()

	// Copy the AST and substitute any parameters with actual values.
	// Note: Not all of the AST is copied as most parts are immutable.
	sCopy, err := (*s).CopyAndSubstitute(p.qe.db, paramValues)
	if err != nil {
		return nil, nil, err
	}

	// Sematnically check the copied AST and then execute it.
	return checkAndExec(p.qe.db, &sCopy)
}

func (p *preparedStatementImpl) ToVdlValue() *vdl.Value {
	return vdl.Int64Value(p.id)
}

func (p *preparedStatementImpl) Close() {
	p.qe.mutexPreparedStatements.Lock()
	delete(p.qe.preparedStatements, p.id)
	p.qe.mutexPreparedStatements.Unlock()
}

func Exec(db ds.Database, q string) ([]string, syncql.ResultStream, error) {
	s, err := query_parser.Parse(db, q)
	if err != nil {
		return nil, nil, err
	}
	return checkAndExec(db, s)
}

func checkAndExec(db ds.Database, s *query_parser.Statement) ([]string, syncql.ResultStream, error) {
	if err := query_checker.Check(db, s); err != nil {
		return nil, nil, err
	}
	switch sel := (*s).(type) {
	case query_parser.SelectStatement:
		return execSelect(db, &sel)
	default:
		return nil, nil, syncql.NewErrExecOfUnknownStatementType(db.GetContext(), (*s).Offset(), reflect.TypeOf(*s).Name())
	}
}

// Given a key, a value and a SelectClause, return the projection.
// This function is only called if Eval returned true on the WhereClause expression.
func ComposeProjection(db ds.Database, k string, v *vdl.Value, s *query_parser.SelectClause) []*vdl.Value {
	var projection []*vdl.Value
	for _, selector := range s.Selectors {
		switch selector.Type {
		case query_parser.TypSelField:
			// If field not found, nil is returned (as per specification).
			f := ResolveField(db, k, v, selector.Field)
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
func ExecSelectSingleRow(db ds.Database, k string, v *vdl.Value, s *query_parser.SelectStatement) []*vdl.Value {
	if !Eval(db, k, v, s.Where.Expr) {
		rs := []*vdl.Value{}
		return rs
	}
	return ComposeProjection(db, k, v, s.Select)
}

type resultStreamImpl struct {
	db              ds.Database
	selectStatement *query_parser.SelectStatement
	resultCount     int64 // results served so far (needed for limit clause)
	skippedCount    int64 // skipped so far (needed for offset clause)
	keyValueStream  ds.KeyValueStream
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
					for _, key := range segment.Keys {
						columnName += getSegmentKeyAsHeading(key)
					}
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

// TODO(jkline): Should we really include key/index of a map/set/array/list in the header?
// The column names can get quite long.  Perhaps just "[]" at the end of the segment
// would be better.  The author of the query can always use the As clause to specify a
// better heading.  Note: for functions, just the function name is included in the header.
// When a decision is made, it's best to be consistent for functions and key/indexes.
func getSegmentKeyAsHeading(segKey *query_parser.Operand) string {
	val := "["
	switch segKey.Type {
	case query_parser.TypBigInt:
		val += segKey.BigInt.String()
	case query_parser.TypBigRat:
		val += segKey.BigRat.String()
	case query_parser.TypComplex:
		val += fmt.Sprintf("%g", segKey.Complex)
	case query_parser.TypField:
		sep := ""
		for _, segment := range segKey.Column.Segments {
			val += sep + segment.Value
			for _, key := range segment.Keys {
				val += getSegmentKeyAsHeading(key)
			}
			sep = "."
		}
	case query_parser.TypBool:
		val += strconv.FormatBool(segKey.Bool)
	case query_parser.TypInt:
		val += strconv.FormatInt(segKey.Int, 10)
	case query_parser.TypFloat:
		val += strconv.FormatFloat(segKey.Float, 'f', -1, 64)
	case query_parser.TypFunction:
		val += segKey.Function.Name
	case query_parser.TypStr:
		val += segKey.Str
	case query_parser.TypTime:
		val += segKey.Time.Format("Mon Jan 2 15:04:05 -0700 MST 2006")
	case query_parser.TypNil:
		val += "<nil>"
	case query_parser.TypObject:
		val += "<object>"
	default:
		val += "<?>"
	}
	val += "]"
	return val
}

func execSelect(db ds.Database, s *query_parser.SelectStatement) ([]string, syncql.ResultStream, error) {
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
