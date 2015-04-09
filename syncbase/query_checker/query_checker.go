// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package query_checker performs a semantic check on an AST produced
// by the query_parser package.
//
// For the foreseeasble future, only SelectStatements are supported.
// The following clauses are checked sequentially,
// SelectClause
// FromClause
// WhereClause (optional)
// LimitClause (optional)
// ResultsOffsetClause (optional)

package query_checker

import (
	"fmt"
	"strings"
	"v.io/syncbase/v23/syncbase/query_parser"
)

type Store interface {
	CheckTable(table string) error
}

type SemanticError struct {
	Msg string
	Off int64
}

func (e *SemanticError) Error() string {
	return fmt.Sprintf("[Off:%d] %s", e.Off, e.Msg)
}

func Error(offset int64, msg string) *SemanticError {
	return &SemanticError{msg, offset}
}

func Check(db Store, s *query_parser.Statement) *SemanticError {
	switch sel := (*s).(type) {
	case query_parser.SelectStatement:
		return CheckSelectStatement(db, &sel)
	default:
		return Error((*s).Offset(), "Cannot semantically check statement, unkown type.")
	}
}

func CheckSelectStatement(db Store, s *query_parser.SelectStatement) *SemanticError {
	if err := CheckSelectClause(s.Select); err != nil {
		return err
	}
	if err := CheckFromClause(db, s.From); err != nil {
		return err
	}
	if err := CheckWhereClause(s.Where); err != nil {
		return err
	}
	if err := CheckLimitClause(s.Limit); err != nil {
		return err
	}
	if err := CheckResultsOffsetClause(s.ResultsOffset); err != nil {
		return err
	}
	return nil
}

// Check select clause.  Fields can be 'k' and v[{.<ident>}...][.*]
func CheckSelectClause(s *query_parser.SelectClause) *SemanticError {
	for _, c := range s.Columns {
		switch c.Segments[0].Value {
		case "k":
			if len(c.Segments) > 1 {
				return Error(c.Segments[1].Off, "Dot notation may not be used on a key (string) field.")
			}
		case "v":
			// "*" may only be ultimate segment.
			foundAsterisk := false
			for i := 1; i < len(c.Segments); i++ {
				if c.Segments[i].Value == "*" {
					foundAsterisk = true
				} else if foundAsterisk {
					return Error(c.Segments[i].Off, "'*' is only valid as the ultimate segment of a field.")
				}
			}
		default:
			return Error(c.Segments[0].Off, "Select field must be 'k' or 'v[{.<ident>}...]'.")
		}
	}
	return nil
}

// Check from clause.  Table must exist in the database.
func CheckFromClause(db Store, f *query_parser.FromClause) *SemanticError {
	if err := db.CheckTable(f.Table.Name); err != nil {
		return Error(f.Table.Off, err.Error())

	}
	return nil
}

// Check where clause.
func CheckWhereClause(w *query_parser.WhereClause) *SemanticError {
	if w == nil {
		return nil
	}
	return CheckExpression(w.Expr)
}

func CheckExpression(e *query_parser.Expression) *SemanticError {
	if err := CheckOperand(e.Operand1); err != nil {
		return err
	}
	if err := CheckOperand(e.Operand2); err != nil {
		return err
	}

	// Like expressions require operand2 to be a string literal
	if e.Operator.Type == query_parser.Like && e.Operand2.Type != query_parser.OpLiteral {
		return Error(e.Off, "Like expressions require right operand of type <string-literal>.")
	}

	// type as an operand must be the first operand, the operator must be = and the 2nd operand must be string literal.
	if (e.Operand1.Type == query_parser.OpField && strings.ToLower(e.Operand1.Column.Segments[0].Value) == "type" && (e.Operator.Type != query_parser.Equal || e.Operand2.Type != query_parser.OpLiteral)) || (e.Operand2.Type == query_parser.OpField && strings.ToLower(e.Operand2.Column.Segments[0].Value) == "type") {
		return Error(e.Off, "Type expressions must be 'type = <string-literal>'.")
	}

	// k as an operand must be the first operand and the 2nd operand must be string literal.
	if (e.Operand1.Type == query_parser.OpField && strings.ToLower(e.Operand1.Column.Segments[0].Value) == "k" && e.Operand2.Type != query_parser.OpLiteral) || (e.Operand2.Type == query_parser.OpField && strings.ToLower(e.Operand2.Column.Segments[0].Value) == "k") {
		return Error(e.Off, "Key (i.e., 'k') expressions must be of form 'k <op> <string-literal>'.")
	}

	return nil
}

func CheckOperand(o *query_parser.Operand) *SemanticError {
	switch o.Type {
	case query_parser.OpExpr:
		return CheckExpression(o.Expr)
	case query_parser.OpField:
		switch o.Column.Segments[0].Value {
		case "k":
			if len(o.Column.Segments) > 1 {
				return Error(o.Column.Segments[1].Off, "Dot notation may not be used on a key (string) field.")
			}
		case "v":
		case "type":
			if len(o.Column.Segments) > 1 {
				return Error(o.Column.Segments[1].Off, "Dot notation may not be used with type.")
			}
		default:
			return Error(o.Column.Segments[0].Off, "Where field must be 'k', 'v[{.<ident>}...]' or 'type'.")
		}
		return nil
	default:
		return nil
	}
}

// Check limit clause.  Limit must be >= 1.
// Note: The parser will not allow negative numbers here.
func CheckLimitClause(l *query_parser.LimitClause) *SemanticError {
	if l == nil {
		return nil
	}
	if l.Limit.Value < 1 {
		return Error(l.Limit.Off, "Limit must be > 0.")
	}
	return nil
}

// Check results offset clause.  Offset must be >= 0.
// Note: The parser will not allow negative numbers here, so this check is presently superfluous.
func CheckResultsOffsetClause(o *query_parser.ResultsOffsetClause) *SemanticError {
	if o == nil {
		return nil
	}
	if o.ResultsOffset.Value < 0 {
		return Error(o.ResultsOffset.Off, "Offset must be >= 0.")
	}
	return nil
}
