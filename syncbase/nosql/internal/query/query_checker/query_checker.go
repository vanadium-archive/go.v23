// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query_checker

import (
	"bytes"
	"regexp"
	"sort"
	"strings"

	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_db"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_parser"
	"v.io/syncbase/v23/syncbase/nosql/syncql"
)

func Check(db query_db.Database, s *query_parser.Statement) error {
	switch sel := (*s).(type) {
	case query_parser.SelectStatement:
		return checkSelectStatement(db, &sel)
	default:
		return syncql.NewErrCheckOfUnknownStatementType(db.GetContext(), (*s).Offset())
	}
}

func checkSelectStatement(db query_db.Database, s *query_parser.SelectStatement) error {
	if err := checkSelectClause(db, s.Select); err != nil {
		return err
	}
	if err := checkFromClause(db, s.From); err != nil {
		return err
	}
	if err := checkWhereClause(db, s.Where); err != nil {
		return err
	}
	if err := checkLimitClause(db, s.Limit); err != nil {
		return err
	}
	if err := checkResultsOffsetClause(db, s.ResultsOffset); err != nil {
		return err
	}
	return nil
}

// Check select clause.  Fields can be 'k' and v[{.<ident>}...]
func checkSelectClause(db query_db.Database, s *query_parser.SelectClause) error {
	for _, c := range s.Columns {
		switch c.Column.Segments[0].Value {
		case "k":
			if len(c.Column.Segments) > 1 {
				return syncql.NewErrDotNotationDisallowedForKey(db.GetContext(), c.Column.Segments[1].Off)
			}
		case "v":
			// Nothing to check.
		default:
			return syncql.NewErrInvalidSelectField(db.GetContext(), c.Column.Segments[0].Off)
		}
	}
	return nil
}

// Check from clause.  Table must exist in the database.
func checkFromClause(db query_db.Database, f *query_parser.FromClause) error {
	var err error
	f.Table.DBTable, err = db.GetTable(f.Table.Name)
	if err != nil {
		return syncql.NewErrTableCantAccess(db.GetContext(), f.Table.Off, f.Table.Name, err)
	}
	return nil
}

// Check where clause.
func checkWhereClause(db query_db.Database, w *query_parser.WhereClause) error {
	if w == nil {
		return nil
	}
	return checkExpression(db, w.Expr)
}

func checkExpression(db query_db.Database, e *query_parser.Expression) error {
	if err := checkOperand(db, e.Operand1); err != nil {
		return err
	}
	if err := checkOperand(db, e.Operand2); err != nil {
		return err
	}

	// Like expressions require operand2 to be a string literal that must be validated.
	if e.Operator.Type == query_parser.Like {
		if e.Operand2.Type != query_parser.TypStr {
			return syncql.NewErrLikeExpressionsRequireRhsString(db.GetContext(), e.Off)
		}
		prefix, err := computePrefix(db, e.Operand2.Off, e.Operand2.Str)
		if err != nil {
			return err
		}
		e.Operand2.Prefix = prefix
		// Compute the regular expression now to to check for errors.
		// Save the regex (testing) and the compiled regex (for later use in evaluation).
		regex, compRegex, err := computeRegex(db, e.Operand2.Off, e.Operand2.Str)
		if err != nil {
			return err
		}
		e.Operand2.Regex = regex
		e.Operand2.CompRegex = compRegex
	}

	// Is/IsNot expressions require operand1 to be a value and operand2 to be nil.
	if e.Operator.Type == query_parser.Is || e.Operator.Type == query_parser.IsNot {
		if !IsField(e.Operand1) {
			return syncql.NewErrIsIsNotRequireLhsValue(db.GetContext(), e.Operand1.Off)
		}
		if e.Operand2.Type != query_parser.TypNil {
			return syncql.NewErrIsIsNotRequireRhsNil(db.GetContext(), e.Operand2.Off)
		}
	}

	// type as an operand must be the first operand, the operator must be = and the 2nd operand must be string literal.
	if (IsType(e.Operand1) && (e.Operator.Type != query_parser.Equal || e.Operand2.Type != query_parser.TypStr)) || IsType(e.Operand2) {
		return syncql.NewErrTypeExpressionForm(db.GetContext(), e.Off)
	}

	// k as an operand must be the first operand, the operator must be like or = and the 2nd operand must be a string literal.
	if (IsKey(e.Operand1) && ((e.Operator.Type != query_parser.Equal && e.Operator.Type != query_parser.Like) || e.Operand2.Type != query_parser.TypStr)) || IsKey(e.Operand2) {
		return syncql.NewErrKeyExpressionForm(db.GetContext(), e.Off)
	}

	// If either operand is a bool, only = and <> operators are allowed.
	if (e.Operand1.Type == query_parser.TypBool || e.Operand2.Type == query_parser.TypBool) && e.Operator.Type != query_parser.Equal && e.Operator.Type != query_parser.NotEqual {
		return syncql.NewErrBoolInvalidExpression(db.GetContext(), e.Operator.Off)
	}

	return nil
}

func checkOperand(db query_db.Database, o *query_parser.Operand) error {
	switch o.Type {
	case query_parser.TypExpr:
		return checkExpression(db, o.Expr)
	case query_parser.TypField:
		switch o.Column.Segments[0].Value {
		case "k":
			if len(o.Column.Segments) > 1 {
				return syncql.NewErrDotNotationDisallowedForKey(db.GetContext(), o.Column.Segments[1].Off)
			}
		case "v":
		case "t":
			if len(o.Column.Segments) > 1 {
				return syncql.NewErrDotNotationDisallowedForType(db.GetContext(), o.Column.Segments[1].Off)
			}
		default:
			return syncql.NewErrBadFieldInWhere(db.GetContext(), o.Column.Segments[0].Off)
		}
		return nil
	case query_parser.TypFunction:
		return syncql.NewErrFunctionsNotYetSupported(db.GetContext(), o.Function.Off)
	default:
		return nil
	}
}

// Only include up to (but not including) a wildcard character ('%', '_').
func computePrefix(db query_db.Database, off int64, s string) (string, error) {
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
				return "", syncql.NewErrInvalidEscapedChar(db.GetContext(), off)
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
		return "", syncql.NewErrInvalidEscapedChar(db.GetContext(), off)
	}
	return s2, nil
}

// Convert Like expression to a regex.  That is, convert:
// % to .*?
// _ to .
// Escape everything that would be incorrectly interpreted as a regex.
// Note: \% and \_ are used to escape % and _, respectively.
func computeRegex(db query_db.Database, off int64, s string) (string, *regexp.Regexp, error) {
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
		return "", nil, syncql.NewErrErrorCompilingRegularExpression(db.GetContext(), off, regex, err)
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
	return IsField(o) && IsKeyField(o.Column)
}

func IsType(o *query_parser.Operand) bool {
	return IsField(o) && IsTypeField(o.Column)
}

func IsKeyField(f *query_parser.Field) bool {
	return strings.ToLower(f.Segments[0].Value) == "k"
}

func IsValueField(f *query_parser.Field) bool {
	return strings.ToLower(f.Segments[0].Value) == "v"
}

func IsTypeField(f *query_parser.Field) bool {
	return strings.ToLower(f.Segments[0].Value) == "t"
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
		p := collectKeyPrefixes(where.Expr)
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
func collectKeyPrefixes(expr *query_parser.Expression) []string {
	var prefixes []string
	if IsKey(expr.Operand1) {
		if expr.Operator.Type == query_parser.Like {
			prefixes = append(prefixes, expr.Operand2.Prefix)
		} else { // OpEqual
			prefixes = append(prefixes, expr.Operand2.Str)
		}
		return prefixes
	}
	if IsExpr(expr.Operand1) {
		for _, p := range collectKeyPrefixes(expr.Operand1.Expr) {
			prefixes = append(prefixes, p)
		}
	}
	if IsExpr(expr.Operand2) {
		for _, p := range collectKeyPrefixes(expr.Operand2.Expr) {
			prefixes = append(prefixes, p)
		}
	}
	return prefixes
}

// Check limit clause.  Limit must be >= 1.
// Note: The parser will not allow negative numbers here.
func checkLimitClause(db query_db.Database, l *query_parser.LimitClause) error {
	if l == nil {
		return nil
	}
	if l.Limit.Value < 1 {
		return syncql.NewErrLimitMustBeGe0(db.GetContext(), l.Limit.Off)
	}
	return nil
}

// Check results offset clause.  Offset must be >= 0.
// Note: The parser will not allow negative numbers here, so this check is presently superfluous.
func checkResultsOffsetClause(db query_db.Database, o *query_parser.ResultsOffsetClause) error {
	if o == nil {
		return nil
	}
	if o.ResultsOffset.Value < 0 {
		return syncql.NewErrOffsetMustBeGe0(db.GetContext(), o.ResultsOffset.Off)
	}
	return nil
}
