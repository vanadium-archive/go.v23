// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query_checker

import (
	"bytes"
	"regexp"
	"sort"
	"strings"

	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_functions"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_parser"
	"v.io/syncbase/v23/syncbase/nosql/query_db"
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
	for _, selector := range s.Selectors {
		switch selector.Type {
		case query_parser.TypSelField:
			switch selector.Field.Segments[0].Value {
			case "k":
				if len(selector.Field.Segments) > 1 {
					return syncql.NewErrDotNotationDisallowedForKey(db.GetContext(), selector.Field.Segments[1].Off)
				}
			case "v":
				// Nothing to check.
			default:
				return syncql.NewErrInvalidSelectField(db.GetContext(), selector.Field.Segments[0].Off)
			}
		case query_parser.TypSelFunc:
			err := query_functions.CheckFunction(db, selector.Function)
			if err != nil {
				return err
			}
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
		regex, compRegex, foundWildcard, err := computeRegex(db, e.Operand2.Off, e.Operand2.Str)
		if err != nil {
			return err
		}
		// Optimization: If like argument contains no wildcards, convert the expression to equals.
		if !foundWildcard {
			e.Operator.Type = query_parser.Equal
			// Since this is no longer a like expression, we need to unescape
			// any escaped chars (i.e., "\\", "\_" and "\%" become
			// "\", "_" and "%", respectively).
			e.Operand2.Str = unescapeLikeExpression(e.Operand2.Str)
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
		// Each of the functions args needs to be checked first.
		for _, arg := range o.Function.Args {
			if err := checkOperand(db, arg); err != nil {
				return err
			}
		}
		// Call query_functions.CheckFunction.  This will check for correct number of args
		// and, to the extent possible, correct types.
		// Furthermore, it may execute the function if the function takes no args or
		// takes only literal args (or an arg that is a function that is also executed
		// early).  CheckFunction will fill in arg types, return types and may fill in
		// Computed and RetValue.
		err := query_functions.CheckFunction(db, o.Function)
		if err != nil {
			return err
		}
		// If function executed early, computed will be true and RetValue set.
		// Convert the operand to the RetValue
		if o.Function.Computed {
			*o = *o.Function.RetValue
		}
	}
	return nil
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
// Unescape '\\', '\%' and '\_' to '\', '%' and '_', respectively.
// Escape everything that would be incorrectly interpreted as a regex.
//
// The approach this function takes is to collect characters to be escaped
// into toBeEscapedBuf.  When a wildcard is encountered, first toBeEscapedBuf
// is escaped and written to the regex buffer, next the wildcard is translated
// to regex (either ".*?" or ".") and written to the regex buffer.
// At the end, any remaining chars in toBeEscapedBuf are written.
//
// Return values are:
// 1. string: uncompiled regular expression
// 2. *Regexp: compiled regular expression
// 3. bool: true if wildcards were found (if false, like is converted to equal)
// 4. error: non-nil if error encountered
func computeRegex(db query_db.Database, off int64, s string) (string, *regexp.Regexp, bool, error) {
	var buf bytes.Buffer            // buffer for return regex
	var toBeEscapedBuf bytes.Buffer // buffer to hold characters waiting to be escaped

	buf.WriteString("^") // '^<regex_str>$'
	escapedMode := false
	foundWildcard := false

	for _, c := range s {
		switch c {
		case '%', '_':
			if escapedMode {
				toBeEscapedBuf.WriteString(string(c))
			} else {
				// Write out any chars waiting to be escaped, then
				// write ".*?' or '.'.
				buf.WriteString(regexp.QuoteMeta(toBeEscapedBuf.String()))
				toBeEscapedBuf.Reset()
				if c == '%' {
					buf.WriteString(".*?")
				} else {
					buf.WriteString(".")
				}
				foundWildcard = true
			}
			escapedMode = false
		case '\\':
			if escapedMode {
				toBeEscapedBuf.WriteString(string(c))
			}
			escapedMode = !escapedMode
		default:
			toBeEscapedBuf.WriteString(string(c))
		}
	}
	// Write any remaining chars in toBeEscapedBuf.
	buf.WriteString(regexp.QuoteMeta(toBeEscapedBuf.String()))
	buf.WriteString("$") // '^<regex_str>$'
	regex := buf.String()
	compRegex, err := regexp.Compile(regex)
	if err != nil {
		return "", nil, false, syncql.NewErrErrorCompilingRegularExpression(db.GetContext(), off, regex, err)
	}
	return regex, compRegex, foundWildcard, nil
}

// Unescape '\\', '\%' and '\_' to '\', '%' and '_', respectively.
func unescapeLikeExpression(s string) string {
	var buf bytes.Buffer // buffer for returned unescaped string

	escapedMode := false

	for _, c := range s {
		switch c {
		case '\\':
			if escapedMode {
				buf.WriteString(string(c))
			}
			escapedMode = !escapedMode
		default:
			buf.WriteString(string(c))
			escapedMode = false
		}
	}
	return buf.String()
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

// Function copied from syncbase.
func computeKeyRangeForPrefix(prefix string) query_db.KeyRange {
	if prefix == "" {
		return query_db.KeyRange{string([]byte{0}), string([]byte{255})}
	}
	limit := []byte(prefix)
	for len(limit) > 0 {
		if limit[len(limit)-1] == 255 {
			limit = limit[:len(limit)-1] // chop off trailing \x00
		} else {
			limit[len(limit)-1] += 1 // add 1
			break                    // no carry
		}
	}
	return query_db.KeyRange{prefix, string(limit)}
}

// The limit for a single value range is simply a zero byte appended.
// In this way, only the single 'start' value will be returned (or nothing if that single
// value is not present).
func computeKeyRangeForSingleValue(start string) query_db.KeyRange {
	limit := []byte(start)
	limit = append(limit, 0)
	return query_db.KeyRange{start, string(limit)}
}

// Compute a list of key ranges to be used by query_db's Table.Scan implementation.
func CompileKeyRanges(where *query_parser.WhereClause) query_db.KeyRanges {
	if where == nil {
		return query_db.KeyRanges{computeKeyRangeForPrefix("")}
	}
	var keyRanges query_db.KeyRanges
	collectKeyRanges(where.Expr, &keyRanges)
	return keyRanges
}

func collectKeyRanges(expr *query_parser.Expression, keyRanges *query_db.KeyRanges) {
	if IsKey(expr.Operand1) {
		if expr.Operator.Type == query_parser.Like {
			addKeyRange(computeKeyRangeForPrefix(expr.Operand2.Prefix), keyRanges)
		} else { // OpEqual
			addKeyRange(computeKeyRangeForSingleValue(expr.Operand2.Str), keyRanges)
		}
	}
	if IsExpr(expr.Operand1) {
		collectKeyRanges(expr.Operand1.Expr, keyRanges)
	}
	if IsExpr(expr.Operand2) {
		collectKeyRanges(expr.Operand2.Expr, keyRanges)
	}
}

func addKeyRange(keyRange query_db.KeyRange, keyRanges *query_db.KeyRanges) {
	handled := false
	// Is there an overlap with an existing range?
	for i, r := range *keyRanges {
		// In the following if,
		// the first paren expr is true if the start of the range to be added is contained in r
		// the second paren expr is true if the limit of the range to be added is contained in r
		// the third paren expr is true if the range to be added entirely contains r
		if (keyRange.Start >= r.Start && keyRange.Start <= r.Limit) ||
			(keyRange.Limit >= r.Start && keyRange.Limit <= r.Limit) ||
			(keyRange.Start <= r.Start && keyRange.Limit >= r.Limit) {
			// keyRange overlaps with existing range at keyRanges[i]
			// set newKeyRange to a range that ecompasses both
			var newKeyRange query_db.KeyRange
			if keyRange.Start < r.Start {
				newKeyRange.Start = keyRange.Start
			} else {
				newKeyRange.Start = r.Start
			}
			if keyRange.Limit > r.Limit {
				newKeyRange.Limit = keyRange.Limit
			} else {
				newKeyRange.Limit = r.Limit
			}
			// The new range may overlap with other ranges in keyRanges
			// delete the current range and call addKeyRange again
			// This recursion will continue until no ranges overlap.
			*keyRanges = append((*keyRanges)[:i], (*keyRanges)[i+1:]...)
			addKeyRange(newKeyRange, keyRanges)
			handled = true // we don't want to add keyRange below
			break
		}
	}
	// no overlap, just add it
	if !handled {
		*keyRanges = append(*keyRanges, keyRange)
	}
	// sort before returning
	sort.Sort(*keyRanges)
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
