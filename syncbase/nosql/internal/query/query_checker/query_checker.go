// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
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
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_parser"
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
		return Error((*s).Offset(), "Cannot semantically check statement, unknown type.")
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

// Check select clause.  Fields can be 'k' and v[{.<ident>}...]
func CheckSelectClause(s *query_parser.SelectClause) *SemanticError {
	for _, c := range s.Columns {
		switch c.Segments[0].Value {
		case "k":
			if len(c.Segments) > 1 {
				return Error(c.Segments[1].Off, "Dot notation may not be used on a key (string) field.")
			}
		case "v":
			// Nothing to check.
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

	// Like expressions require operand2 to be a string literal that must be validated.
	if e.Operator.Type == query_parser.Like {
		if e.Operand2.Type != query_parser.TypLiteral {
			return Error(e.Off, "Like expressions require right operand of type <string-literal>.")
		}
		prefix, err := ComputePrefix(e.Operand2.Off, e.Operand2.Literal)
		if err != nil {
			return err
		}
		e.Operand2.Prefix = prefix
		// Compute the regular expression now to to check for errors.
		// Save the regex (testing) and the compiled regex (for later use in evaluation).
		regex, compRegex, err := ComputeRegex(e.Operand2.Off, e.Operand2.Literal)
		if err != nil {
			return err
		}
		e.Operand2.Regex = regex
		e.Operand2.CompRegex = compRegex
	}

	// type as an operand must be the first operand, the operator must be = and the 2nd operand must be string literal.
	if (IsType(e.Operand1) && (e.Operator.Type != query_parser.Equal || e.Operand2.Type != query_parser.TypLiteral)) || IsType(e.Operand2) {
		return Error(e.Off, "Type expressions must be 't = <string-literal>'.")
	}

	// k as an operand must be the first operand, the operator must be like or = and the 2nd operand must be a string literal.
	if (IsKey(e.Operand1) && ((e.Operator.Type != query_parser.Equal && e.Operator.Type != query_parser.Like) || e.Operand2.Type != query_parser.TypLiteral)) || IsKey(e.Operand2) {
		return Error(e.Off, "Key (i.e., 'k') expressions must be of form 'k like|= <string-literal>'.")
	}

	// If either operand is a bool, only = and <> operators are allowed.
	if (e.Operand1.Type == query_parser.TypBool || e.Operand2.Type == query_parser.TypBool) && e.Operator.Type != query_parser.Equal && e.Operator.Type != query_parser.NotEqual {
		return Error(e.Operator.Off, "Boolean operands may only be used in equals and not equals expressions.")
	}

	return nil
}

func CheckOperand(o *query_parser.Operand) *SemanticError {
	switch o.Type {
	case query_parser.TypExpr:
		return CheckExpression(o.Expr)
	case query_parser.TypField:
		switch o.Column.Segments[0].Value {
		case "k":
			if len(o.Column.Segments) > 1 {
				return Error(o.Column.Segments[1].Off, "Dot notation may not be used on a key (string) field.")
			}
		case "v":
		case "t":
			if len(o.Column.Segments) > 1 {
				return Error(o.Column.Segments[1].Off, "Dot notation may not be used with type.")
			}
		default:
			return Error(o.Column.Segments[0].Off, "Where field must be 'k', 'v[{.<ident>}...]' or 't'.")
		}
		return nil
	default:
		return nil
	}
}

// Only include up to (but not including) a wildcard character ('%', '_').
func ComputePrefix(off int64, s string) (string, *SemanticError) {
	if strings.Index(s, "%") == -1 && strings.Index(s, "_") == -1 && strings.Index(s, "\\") == -1 {
		return s, nil
	}
	var s2 string
	escapedChar := false
	for _, c := range s {
		if escapedChar {
			switch c {
			case '\\':
				s2 += string(c)
			case '%':
				s2 += string(c)
			case '_':
				s2 += string(c)
			default:
				return "", Error(off, "Expected '\\', '%' or '_' after '\\'.")
			}
			escapedChar = false
		} else {
			if c == '%' || c == '_' {
				return s2, nil
			} else if c == '\\' {
				escapedChar = true
			} else {
				s2 += string(c)
			}
		}
	}
	if escapedChar {
		return "", Error(off, "Expected '\\', '%' or '_' after '\\'")
	}
	return s2, nil
}

// Convert Like expression to a regex.  That is, convert:
// % to .*?
// _ to .
// Escape everything that would be incorrectly interpreted as a regex.
// Note: \% and \_ are used to escape % and _, respectively.
func ComputeRegex(off int64, s string) (string, *regexp.Regexp, *SemanticError) {
	// Escape everything, this will escape too much as like wildcards can
	// also be escaped by a backslash (\%, \_, \\).
	escaped := regexp.QuoteMeta(s)
	// Change all unescaped '%' chars to ".*?" and all unescaped '_' chars to '.'.
	var buf bytes.Buffer
	buf.WriteString("^")
	backslash_level := 0
	for _, c := range escaped {
		switch backslash_level {
		case 0:
			switch c {
			case '%':
				buf.WriteString(".*?")
			case '_':
				buf.WriteString(".")
			case '\\':
				backslash_level++
			default:
				buf.WriteString(string(c))
			}
		case 1:
			switch c {
			case '\\':
				// backslashes become double backslashes because of the
				// QuoteMeta above.  Let's see what's next.
				backslash_level++
			default:
				// In this case, QuoteMeta is escaping a regex character.  We
				// need to honor the escape (e.g., \*, \[, \]).
				buf.WriteString("\\")
				buf.WriteString(string(c))
				backslash_level = 0
			}
		case 2:
			switch c {
			case '\\':
				// We've hit a third backslash.
				// Write out the first \ (escaped as \\).
				// Set backslash_level to 1 since we've encountered
				// another backslash and need to see what follows.
				buf.WriteString("\\\\")
				backslash_level = 1
			default:
				// The user wrote \% or \_.
				// It was escaped by QuoteMeta to \\% or \\_.
				// Since the % or _ was escaped, just write it so regex can
				// treat it like any character.
				buf.WriteString(string(c))
				backslash_level = 0
			}
		}
	}
	buf.WriteString("$")
	regex := buf.String()
	compRegex, err := regexp.Compile(regex)
	if err != nil {
		return "", nil, Error(off, err.Error())
	}
	return regex, compRegex, nil
}

func IsLogicalOperator(o *query_parser.BinaryOperator) bool {
	return o.Type == query_parser.And || o.Type == query_parser.Or
}

func IsField(o *query_parser.Operand) bool {
	return o.Type == query_parser.TypField
}

func IsKey(o *query_parser.Operand) bool {
	return IsField(o) && strings.ToLower(o.Column.Segments[0].Value) == "k"
}

func IsType(o *query_parser.Operand) bool {
	return IsField(o) && strings.ToLower(o.Column.Segments[0].Value) == "t"
}

func IsExpr(o *query_parser.Operand) bool {
	return o.Type == query_parser.TypExpr
}

// Compile a list of key prefixes to fetch with scan.  Prefixes are returned in sorted order
// and do not overlap (e.g., prefixes of "ab" and "abc" would be combined as "ab").
// A single empty string (array len of 1) is returned if all keys are to be fetched.
// Used by query package.  In query_checker package so prefixes can be tested.
func CompileKeyPrefixes(where *query_parser.WhereClause) []string {
	if where == nil {
		return []string{""}
	} else {
		// Collect all key string literal operands.
		p := CollectKeyPrefixes(where.Expr)
		// Sort
		sort.Strings(p)
		// Elminate overlaps
		var p2 []string
		for i, s := range p {
			if i != 0 && strings.HasPrefix(s, p[i-1]) {
				continue
			}
			p2 = append(p2, s)
		}
		return p2
	}
}

// Collect all operand2 string literals where operand1 of the expression is ident "k".
func CollectKeyPrefixes(expr *query_parser.Expression) []string {
	var prefixes []string
	if IsKey(expr.Operand1) {
		if expr.Operator.Type == query_parser.Like {
			prefixes = append(prefixes, expr.Operand2.Prefix)
		} else { // OpEqual
			prefixes = append(prefixes, expr.Operand2.Literal)
		}
		return prefixes
	}
	if IsExpr(expr.Operand1) {
		for _, p := range CollectKeyPrefixes(expr.Operand1.Expr) {
			prefixes = append(prefixes, p)
		}
	}
	if IsExpr(expr.Operand2) {
		for _, p := range CollectKeyPrefixes(expr.Operand2.Expr) {
			prefixes = append(prefixes, p)
		}
	}
	return prefixes
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
