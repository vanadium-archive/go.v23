// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query_checker

import (
	"bytes"
	"regexp"
	"sort"
	"strings"

	ds "v.io/v23/query/engine/datasource"
	"v.io/v23/query/engine/internal/query_functions"
	"v.io/v23/query/engine/internal/query_parser"
	"v.io/v23/query/syncql"
)

const (
	MaxRangeLimit = ""
)

var (
	KeyRangeAll = ds.KeyRange{Start: "", Limit: MaxRangeLimit}
)

func Check(db ds.Database, s *query_parser.Statement) error {
	switch sel := (*s).(type) {
	case query_parser.SelectStatement:
		return checkSelectStatement(db, &sel)
	default:
		return syncql.NewErrCheckOfUnknownStatementType(db.GetContext(), (*s).Offset())
	}
}

func checkSelectStatement(db ds.Database, s *query_parser.SelectStatement) error {
	if err := checkSelectClause(db, s.Select); err != nil {
		return err
	}
	if err := checkFromClause(db, s.From); err != nil {
		return err
	}
	if err := checkEscapeClause(db, s.Escape); err != nil {
		return err
	}
	if err := checkWhereClause(db, s.Where, s.Escape); err != nil {
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
func checkSelectClause(db ds.Database, s *query_parser.SelectClause) error {
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
			case "K":
				// Be nice and warn of mistakenly capped 'K'.
				return syncql.NewErrDidYouMeanLowercaseK(db.GetContext(), selector.Field.Segments[0].Off)
			case "V":
				// Be nice and warn of mistakenly capped 'V'.
				return syncql.NewErrDidYouMeanLowercaseV(db.GetContext(), selector.Field.Segments[0].Off)
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
func checkFromClause(db ds.Database, f *query_parser.FromClause) error {
	var err error
	f.Table.DBTable, err = db.GetTable(f.Table.Name)
	if err != nil {
		return syncql.NewErrTableCantAccess(db.GetContext(), f.Table.Off, f.Table.Name, err)
	}
	return nil
}

// Check where clause.
func checkWhereClause(db ds.Database, w *query_parser.WhereClause, ec *query_parser.EscapeClause) error {
	if w == nil {
		return nil
	}
	return checkExpression(db, w.Expr, ec)
}

func checkExpression(db ds.Database, e *query_parser.Expression, ec *query_parser.EscapeClause) error {
	if err := checkOperand(db, e.Operand1, ec); err != nil {
		return err
	}
	if err := checkOperand(db, e.Operand2, ec); err != nil {
		return err
	}

	// Like expressions require operand2 to be a string literal that must be validated.
	if e.Operator.Type == query_parser.Like || e.Operator.Type == query_parser.NotLike {
		if e.Operand2.Type != query_parser.TypStr {
			return syncql.NewErrLikeExpressionsRequireRhsString(db.GetContext(), e.Operand2.Off)
		}
		prefix, err := computePrefix(db, e.Operand2.Off, e.Operand2.Str, ec)
		if err != nil {
			return err
		}
		e.Operand2.Prefix = prefix
		// Compute the regular expression now to to check for errors.
		// Save the regex (testing) and the compiled regex (for later use in evaluation).
		regex, compRegex, foundWildcard, err := computeRegex(db, e.Operand2.Off, e.Operand2.Str, ec)
		if err != nil {
			return err
		}
		// Optimization: If like/not like argument contains no wildcards, convert the expression to equals/not equals.
		if !foundWildcard {
			if e.Operator.Type == query_parser.Like {
				e.Operator.Type = query_parser.Equal
			} else { // not like
				e.Operator.Type = query_parser.NotEqual
			}
			// Since this is no longer a like expression, we need to unescape
			// any escaped chars.
			e.Operand2.Str = unescapeLikeExpression(e.Operand2.Str, ec)
		}
		e.Operand2.Regex = regex
		e.Operand2.CompRegex = compRegex
	}

	// Is/IsNot expressions require operand1 to be a (value or function) and operand2 to be nil.
	if e.Operator.Type == query_parser.Is || e.Operator.Type == query_parser.IsNot {
		if !IsField(e.Operand1) && !IsFunction(e.Operand1) {
			return syncql.NewErrIsIsNotRequireLhsValue(db.GetContext(), e.Operand1.Off)
		}
		if e.Operand2.Type != query_parser.TypNil {
			return syncql.NewErrIsIsNotRequireRhsNil(db.GetContext(), e.Operand2.Off)
		}
	}

	// if an operand is k and the other operand is a literal, that literal must be a string
	// literal.
	if ContainsKeyOperand(e) && ((isLiteral(e.Operand1) && !isStringLiteral(e.Operand1)) ||
		(isLiteral(e.Operand2) && !isStringLiteral(e.Operand2))) {
		off := e.Operand1.Off
		if isLiteral(e.Operand2) {
			off = e.Operand2.Off
		}
		return syncql.NewErrKeyExpressionLiteral(db.GetContext(), off)
	}

	// If either operand is a bool, only = and <> operators are allowed.
	if (e.Operand1.Type == query_parser.TypBool || e.Operand2.Type == query_parser.TypBool) && e.Operator.Type != query_parser.Equal && e.Operator.Type != query_parser.NotEqual {
		return syncql.NewErrBoolInvalidExpression(db.GetContext(), e.Operator.Off)
	}

	return nil
}

func checkOperand(db ds.Database, o *query_parser.Operand, ec *query_parser.EscapeClause) error {
	switch o.Type {
	case query_parser.TypExpr:
		return checkExpression(db, o.Expr, ec)
	case query_parser.TypField:
		switch o.Column.Segments[0].Value {
		case "k":
			if len(o.Column.Segments) > 1 {
				return syncql.NewErrDotNotationDisallowedForKey(db.GetContext(), o.Column.Segments[1].Off)
			}
		case "v":
			// Nothing to do.
		case "K":
			// Be nice and warn of mistakenly capped 'K'.
			return syncql.NewErrDidYouMeanLowercaseK(db.GetContext(), o.Column.Segments[0].Off)
		case "V":
			// Be nice and warn of mistakenly capped 'V'.
			return syncql.NewErrDidYouMeanLowercaseV(db.GetContext(), o.Column.Segments[0].Off)
		default:
			return syncql.NewErrBadFieldInWhere(db.GetContext(), o.Column.Segments[0].Off)
		}
		return nil
	case query_parser.TypFunction:
		// Each of the functions args needs to be checked first.
		for _, arg := range o.Function.Args {
			if err := checkOperand(db, arg, ec); err != nil {
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
func computePrefix(db ds.Database, off int64, s string, ec *query_parser.EscapeClause) (string, error) {
	if strings.Index(s, "%") == -1 && strings.Index(s, "_") == -1 && (ec == nil || strings.IndexRune(s, ec.EscapeChar.Value) == -1) {
		return s, nil
	}
	var s2 string
	escapedChar := false
	for _, c := range s {
		if escapedChar {
			switch c {
			case '%':
				s2 += string(c)
			case '_':
				s2 += string(c)
			default:
				return "", syncql.NewErrInvalidEscapeSequence(db.GetContext(), off)
			}
			escapedChar = false
		} else {
			// Hit an unescaped wildcard, we are done
			if c == '%' || c == '_' {
				return s2, nil
			} else if ec != nil && c == ec.EscapeChar.Value {
				escapedChar = true
			} else {
				s2 += string(c)
			}
		}
	}
	if escapedChar {
		return "", syncql.NewErrInvalidEscapeSequence(db.GetContext(), off)
	}
	return s2, nil
}

// Convert Like expression to a regex.  That is, convert:
// % to .*?
// _ to .
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
func computeRegex(db ds.Database, off int64, s string, ec *query_parser.EscapeClause) (string, *regexp.Regexp, bool, error) {
	var buf bytes.Buffer            // buffer for return regex
	var toBeEscapedBuf bytes.Buffer // buffer to hold characters waiting to be escaped

	buf.WriteString("^") // '^<regex_str>$'
	escapedMode := false
	foundWildcard := false

	escChar := ' ' // blank will be ignored as an escape char
	if ec != nil {
		escChar = ec.EscapeChar.Value
	}

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
		case escChar:
			if escChar != ' ' {
				if escapedMode {
					toBeEscapedBuf.WriteString(string(c))
				}
				escapedMode = !escapedMode
			} else {
				// not an escape char, treat same as default
				toBeEscapedBuf.WriteString(string(c))
			}
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

// Unescape any escaped % and _ chars as this is being converted to an equals expression.
func unescapeLikeExpression(s string, ec *query_parser.EscapeClause) string {
	var buf bytes.Buffer // buffer for returned unescaped string

	if ec == nil {
		// there is nothing to unescape
		return s
	}

	escapedMode := false

	for _, c := range s {
		switch c {
		case ec.EscapeChar.Value:
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

func IsFunction(o *query_parser.Operand) bool {
	return o.Type == query_parser.TypFunction
}

func ContainsKeyOperand(expr *query_parser.Expression) bool {
	return IsKey(expr.Operand1) || IsKey(expr.Operand2)
}

func ContainsFunctionOperand(expr *query_parser.Expression) bool {
	return IsFunction(expr.Operand1) || IsFunction(expr.Operand2)
}

func ContainsValueFieldOperand(expr *query_parser.Expression) bool {
	return (expr.Operand1.Type == query_parser.TypField && IsValueField(expr.Operand1.Column)) ||
		(expr.Operand2.Type == query_parser.TypField && IsValueField(expr.Operand2.Column))

}

func isStringLiteral(o *query_parser.Operand) bool {
	return o.Type == query_parser.TypStr
}

func isLiteral(o *query_parser.Operand) bool {
	return o.Type == query_parser.TypBigInt ||
		o.Type == query_parser.TypBigRat || // currently, no way to specify as literal
		o.Type == query_parser.TypBool ||
		o.Type == query_parser.TypComplex || // currently, no way to specify as literal ||
		o.Type == query_parser.TypFloat ||
		o.Type == query_parser.TypInt ||
		o.Type == query_parser.TypStr ||
		o.Type == query_parser.TypTime || // currently, no way to specify as literal
		o.Type == query_parser.TypUint
}

func IsKey(o *query_parser.Operand) bool {
	return IsField(o) && IsKeyField(o.Column)
}

func IsKeyField(f *query_parser.Field) bool {
	return f.Segments[0].Value == "k"
}

func IsValueField(f *query_parser.Field) bool {
	return f.Segments[0].Value == "v"
}

func IsExpr(o *query_parser.Operand) bool {
	return o.Type == query_parser.TypExpr
}

func afterPrefix(prefix string) string {
	// Copied from syncbase.
	limit := []byte(prefix)
	for len(limit) > 0 {
		if limit[len(limit)-1] == 255 {
			limit = limit[:len(limit)-1] // chop off trailing \x00
		} else {
			limit[len(limit)-1] += 1 // add 1
			break                    // no carry
		}
	}
	return string(limit)
}

func computeKeyRangeForLike(prefix string) ds.KeyRange {
	if prefix == "" {
		return KeyRangeAll
	}
	return ds.KeyRange{Start: prefix, Limit: afterPrefix(prefix)}
}

func computeKeyRangesForNotLike(prefix string) *ds.KeyRanges {
	if prefix == "" {
		return &ds.KeyRanges{KeyRangeAll}
	}
	return &ds.KeyRanges{
		ds.KeyRange{Start: "", Limit: prefix},
		ds.KeyRange{Start: afterPrefix(prefix), Limit: ""},
	}
}

// The limit for a single value range is simply a zero byte appended.
// In this way, only the single 'start' value will be returned (or nothing if that single
// value is not present).
func computeKeyRangeForSingleValue(start string) ds.KeyRange {
	limit := []byte(start)
	limit = append(limit, 0)
	return ds.KeyRange{Start: start, Limit: string(limit)}
}

// Compute a list of key ranges to be used by query's Table.Scan implementation.
func CompileKeyRanges(where *query_parser.WhereClause) *ds.KeyRanges {
	if where == nil {
		return &ds.KeyRanges{KeyRangeAll}
	}
	return collectKeyRanges(where.Expr)
}

func computeIntersection(lhs, rhs ds.KeyRange) *ds.KeyRange {
	// Detect if lhs.Start is contained within rhs or rhs.Start is contained within lhs.
	if (lhs.Start >= rhs.Start && compareStartToLimit(lhs.Start, rhs.Limit) < 0) ||
		(rhs.Start >= lhs.Start && compareStartToLimit(rhs.Start, lhs.Limit) < 0) {
		var start, limit string
		if lhs.Start < rhs.Start {
			start = rhs.Start
		} else {
			start = lhs.Start
		}
		if compareLimits(lhs.Limit, rhs.Limit) < 0 {
			limit = lhs.Limit
		} else {
			limit = rhs.Limit
		}
		return &ds.KeyRange{Start: start, Limit: limit}
	}
	return nil
}

func keyRangeIntersection(lhs, rhs *ds.KeyRanges) *ds.KeyRanges {
	keyRanges := &ds.KeyRanges{}
	lCur, rCur := 0, 0
	for lCur < len(*lhs) && rCur < len(*rhs) {
		// Any intersection at current cursors?
		if intersection := computeIntersection((*lhs)[lCur], (*rhs)[rCur]); intersection != nil {
			// Add the intersection
			addKeyRange(*intersection, keyRanges)
		}
		// increment the range with the lesser limit
		c := compareLimits((*lhs)[lCur].Limit, (*rhs)[rCur].Limit)
		switch c {
		case -1:
			lCur++
		case 1:
			rCur++
		default:
			lCur++
			rCur++
		}
	}
	return keyRanges
}

func collectKeyRanges(expr *query_parser.Expression) *ds.KeyRanges {
	if IsExpr(expr.Operand1) { // then both operands must be expressions
		lhsKeyRanges := collectKeyRanges(expr.Operand1.Expr)
		rhsKeyRanges := collectKeyRanges(expr.Operand2.Expr)
		if expr.Operator.Type == query_parser.And {
			// intersection of lhsKeyRanges and rhsKeyRanges
			return keyRangeIntersection(lhsKeyRanges, rhsKeyRanges)
		} else { // or
			// union of lhsKeyRanges and rhsKeyRanges
			for _, rhsKeyRange := range *rhsKeyRanges {
				addKeyRange(rhsKeyRange, lhsKeyRanges)
			}
			return lhsKeyRanges
		}
	} else if ContainsKeyOperand(expr) { // true if either operand is 'k'
		if IsKey(expr.Operand1) && IsKey(expr.Operand2) {
			//k <op> k
			switch expr.Operator.Type {
			case query_parser.Equal, query_parser.GreaterThanOrEqual, query_parser.LessThanOrEqual:
				// True for all keys
				return &ds.KeyRanges{KeyRangeAll}
			default: // query_parser.NotEqual, query_parser.GreaterThan, query_parser.LessThan:
				// False for all keys
				return &ds.KeyRanges{}
			}
		} else if expr.Operator.Type == query_parser.Is {
			// k is nil
			// False for all keys
			return &ds.KeyRanges{}
		} else if expr.Operator.Type == query_parser.IsNot {
			// k is not nil
			// True for all keys
			return &ds.KeyRanges{KeyRangeAll}
		} else if isStringLiteral(expr.Operand2) {
			// k <op> <string-literal>
			switch expr.Operator.Type {
			case query_parser.Equal:
				return &ds.KeyRanges{computeKeyRangeForSingleValue(expr.Operand2.Str)}
			case query_parser.GreaterThan:
				return &ds.KeyRanges{ds.KeyRange{Start: string(append([]byte(expr.Operand2.Str), 0)), Limit: MaxRangeLimit}}
			case query_parser.GreaterThanOrEqual:
				return &ds.KeyRanges{ds.KeyRange{Start: expr.Operand2.Str, Limit: MaxRangeLimit}}
			case query_parser.Like:
				return &ds.KeyRanges{computeKeyRangeForLike(expr.Operand2.Prefix)}
			case query_parser.NotLike:
				return computeKeyRangesForNotLike(expr.Operand2.Prefix)
			case query_parser.LessThan:
				return &ds.KeyRanges{ds.KeyRange{Start: "", Limit: expr.Operand2.Str}}
			case query_parser.LessThanOrEqual:
				return &ds.KeyRanges{ds.KeyRange{Start: "", Limit: string(append([]byte(expr.Operand2.Str), 0))}}
			default: // case query_parser.NotEqual:
				return &ds.KeyRanges{
					ds.KeyRange{Start: "", Limit: expr.Operand2.Str},
					ds.KeyRange{Start: string(append([]byte(expr.Operand2.Str), 0)), Limit: MaxRangeLimit},
				}
			}
		} else if isStringLiteral(expr.Operand1) {
			//<string-literal> <op> k
			switch expr.Operator.Type {
			case query_parser.Equal:
				return &ds.KeyRanges{computeKeyRangeForSingleValue(expr.Operand1.Str)}
			case query_parser.GreaterThan:
				return &ds.KeyRanges{ds.KeyRange{Start: "", Limit: expr.Operand1.Str}}
			case query_parser.GreaterThanOrEqual:
				return &ds.KeyRanges{ds.KeyRange{Start: "", Limit: string(append([]byte(expr.Operand1.Str), 0))}}
			case query_parser.LessThan:
				return &ds.KeyRanges{ds.KeyRange{Start: string(append([]byte(expr.Operand1.Str), 0)), Limit: MaxRangeLimit}}
			case query_parser.LessThanOrEqual:
				return &ds.KeyRanges{ds.KeyRange{Start: expr.Operand1.Str, Limit: MaxRangeLimit}}
			default: // case query_parser.NotEqual:
				return &ds.KeyRanges{
					ds.KeyRange{Start: "", Limit: expr.Operand1.Str},
					ds.KeyRange{Start: string(append([]byte(expr.Operand1.Str), 0)), Limit: MaxRangeLimit},
				}
			}
		} else {
			// A function or a field s being compared to the key.
			return &ds.KeyRanges{KeyRangeAll}
		}
	} else { // not a key compare, so it applies to the entire key range
		return &ds.KeyRanges{KeyRangeAll}
	}
}

// Helper function to compare start and limit byte arrays  taking into account that
// MaxRangeLimit is actually []byte{}.
func compareLimits(limitA, limitB string) int {
	if limitA == limitB {
		return 0
	} else if limitA == MaxRangeLimit {
		return 1
	} else if limitB == MaxRangeLimit {
		return -1
	} else if limitA < limitB {
		return -1
	} else {
		return 1
	}
}

func compareStartToLimit(startA, limitB string) int {
	if limitB == MaxRangeLimit {
		return -1
	} else if startA == limitB {
		return 0
	} else if startA < limitB {
		return -1
	} else {
		return 1
	}
}

func compareLimitToStart(limitA, startB string) int {
	if limitA == MaxRangeLimit {
		return -1
	} else if limitA == startB {
		return 0
	} else if limitA < startB {
		return -1
	} else {
		return 1
	}
}

func addKeyRange(keyRange ds.KeyRange, keyRanges *ds.KeyRanges) {
	handled := false
	// Is there an overlap with an existing range?
	for i, r := range *keyRanges {
		// In the following if,
		// the first paren expr is true if the start of the range to be added is contained in r
		// the second paren expr is true if the limit of the range to be added is contained in r
		// the third paren expr is true if the range to be added entirely contains r
		if (keyRange.Start >= r.Start && compareStartToLimit(keyRange.Start, r.Limit) <= 0) ||
			(compareLimitToStart(keyRange.Limit, r.Start) >= 0 && compareLimits(keyRange.Limit, r.Limit) <= 0) ||
			(keyRange.Start <= r.Start && compareLimits(keyRange.Limit, r.Limit) >= 0) {

			// keyRange overlaps with existing range at keyRanges[i]
			// set newKeyRange to a range that ecompasses both
			var newKeyRange ds.KeyRange
			if keyRange.Start < r.Start {
				newKeyRange.Start = keyRange.Start
			} else {
				newKeyRange.Start = r.Start
			}
			if compareLimits(keyRange.Limit, r.Limit) > 0 {
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

// Check escape clause.  Escape char cannot be '\'.
// Return bool (true if escape char defined), escape char, error.
func checkEscapeClause(db ds.Database, e *query_parser.EscapeClause) error {
	if e == nil {
		return nil
	}
	switch e.EscapeChar.Value {
	case ' ', '\\':
		return syncql.NewErrInvalidEscapeChar(db.GetContext(), e.EscapeChar.Off)
	default:
		return nil
	}
}

// Check limit clause.  Limit must be >= 1.
// Note: The parser will not allow negative numbers here.
func checkLimitClause(db ds.Database, l *query_parser.LimitClause) error {
	if l == nil {
		return nil
	}
	if l.Limit.Value < 1 {
		return syncql.NewErrLimitMustBeGt0(db.GetContext(), l.Limit.Off)
	}
	return nil
}

// Check results offset clause.  Offset must be >= 0.
// Note: The parser will not allow negative numbers here, so this check is presently superfluous.
func checkResultsOffsetClause(db ds.Database, o *query_parser.ResultsOffsetClause) error {
	if o == nil {
		return nil
	}
	if o.ResultsOffset.Value < 0 {
		return syncql.NewErrOffsetMustBeGe0(db.GetContext(), o.ResultsOffset.Off)
	}
	return nil
}
