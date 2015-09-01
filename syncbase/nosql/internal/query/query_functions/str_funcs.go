// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query_functions

import (
	"strings"
	"unicode"

	"v.io/v23/syncbase/nosql/internal/query/conversions"
	"v.io/v23/syncbase/nosql/internal/query/query_parser"
	"v.io/v23/syncbase/nosql/query_db"
	"v.io/v23/syncbase/nosql/syncql"
	"v.io/v23/vdl"
)

func lowerCase(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	strOp, err := conversions.ConvertValueToString(args[0])
	if err != nil {
		return nil, err
	}
	return makeStrOp(off, strings.ToLower(strOp.Str)), nil
}

func upperCase(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	strOp, err := conversions.ConvertValueToString(args[0])
	if err != nil {
		return nil, err
	}
	return makeStrOp(off, strings.ToUpper(strOp.Str)), nil
}

func typeFunc(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	// If operand is not an object, we can't get a type
	if args[0].Type != query_parser.TypObject {
		return nil, syncql.NewErrFunctionTypeInvalidArg(db.GetContext(), args[0].Off)
	}
	return makeStrOp(off, args[0].Object.Type().Name()), nil
}

func typeFuncFieldCheck(db query_db.Database, off int64, args []*query_parser.Operand) error {
	// At this point, it is known that there is one arg. Make sure it is of type field
	// and is a value field (i.e., it must begin with a v segment).
	if args[0].Type != query_parser.TypField || len(args[0].Column.Segments) < 1 || args[0].Column.Segments[0].Value != "v" {
		return syncql.NewErrArgMustBeField(db.GetContext(), args[0].Off)
	}
	return nil
}

// Split splits str (arg[0]) into substrings separated by sep (arg[1]) and returns an
// array of substrings between those separators. If sep is empty, Split splits after each
// UTF-8 sequence.
// e.g., Split("abc.def.ghi", ".") returns a list of "abc", "def", "ghi"
func split(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	strArg, err := conversions.ConvertValueToString(args[0])
	if err != nil {
		return nil, err
	}
	sepArg, err := conversions.ConvertValueToString(args[1])
	if err != nil {
		return nil, err
	}

	var o query_parser.Operand
	o.Off = args[0].Off
	o.Type = query_parser.TypObject
	o.Object = vdl.ValueOf(strings.Split(strArg.Str, sepArg.Str))
	return &o, nil
}

// StrCat(str1, str2,... string) string
// StrCat returns the concatenation of all the string args.
// e.g., StrCat("abc", ",", "def") returns "abc,def"
func strCat(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	val := ""
	for _, arg := range args {
		str, err := conversions.ConvertValueToString(arg)
		if err != nil {
			return nil, err
		}
		val += str.Str
	}
	return makeStrOp(off, val), nil
}

// StrIndex(s, sep string) int
// StrIndex returns the index of sep in s, or -1 is sep is not present in s.
// e.g., StrIndex("abc", "bc") returns 1.
func strIndex(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	s, err := conversions.ConvertValueToString(args[0])
	if err != nil {
		return nil, err
	}
	sep, err := conversions.ConvertValueToString(args[1])
	if err != nil {
		return nil, err
	}
	return makeIntOp(off, int64(strings.Index(s.Str, sep.Str))), nil
}

// StrLastIndex(s, sep string) int
// StrLastIndex returns the index of the last instance of sep in s, or -1 is sep is not present in s.
// e.g., StrLastIndex("abcbc", "bc") returns 3.
func strLastIndex(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	s, err := conversions.ConvertValueToString(args[0])
	if err != nil {
		return nil, err
	}
	sep, err := conversions.ConvertValueToString(args[1])
	if err != nil {
		return nil, err
	}
	return makeIntOp(off, int64(strings.LastIndex(s.Str, sep.Str))), nil
}

// StrRepeat(s string, count int) int
// StrRepeat returns a new string consisting of count copies of the string s.
// e.g., StrRepeat("abc", 3) returns "abcabcabc".
func strRepeat(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	s, err := conversions.ConvertValueToString(args[0])
	if err != nil {
		return nil, err
	}
	count, err := conversions.ConvertValueToInt(args[1])
	if err != nil {
		return nil, err
	}
	if count.Int >= 0 {
		return makeStrOp(off, strings.Repeat(s.Str, int(count.Int))), nil
	} else {
		// golang strings.Repeat doesn't like count < 0
		return makeStrOp(off, ""), nil
	}
}

// StrReplace(s, old, new string) string
// StrReplace returns a copy of s with the first instance of old replaced by new.
// e.g., StrReplace("abcdef", "bc", "zzzzz") returns "azzzzzdef".
func strReplace(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	s, err := conversions.ConvertValueToString(args[0])
	if err != nil {
		return nil, err
	}
	old, err := conversions.ConvertValueToString(args[1])
	if err != nil {
		return nil, err
	}
	new, err := conversions.ConvertValueToString(args[2])
	if err != nil {
		return nil, err
	}
	return makeStrOp(off, strings.Replace(s.Str, old.Str, new.Str, 1)), nil
}

// Trim(s string) string
// Trim returns a copy of s with all leading and trailing white space removed, as defined by Unicode.
// e.g., Trim(" abc ") returns "abc".
func trim(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	s, err := conversions.ConvertValueToString(args[0])
	if err != nil {
		return nil, err
	}
	return makeStrOp(off, strings.TrimSpace(s.Str)), nil
}

// TrimLeft(s string) string
// TrimLeft returns a copy of s with all leading white space removed, as defined by Unicode.
// e.g., TrimLeft(" abc ") returns "abc ".
func trimLeft(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	s, err := conversions.ConvertValueToString(args[0])
	if err != nil {
		return nil, err
	}
	return makeStrOp(off, strings.TrimLeftFunc(s.Str, unicode.IsSpace)), nil
}

// TrimRight(s string) string
// TrimRight returns a copy of s with all leading white space removed, as defined by Unicode.
// e.g., TrimRight(" abc ") returns "abc ".
func trimRight(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	s, err := conversions.ConvertValueToString(args[0])
	if err != nil {
		return nil, err
	}
	return makeStrOp(off, strings.TrimRightFunc(s.Str, unicode.IsSpace)), nil
}