// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query_functions

import (
	"strings"

	"v.io/syncbase/v23/syncbase/nosql/internal/query/conversions"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_parser"
	"v.io/syncbase/v23/syncbase/nosql/query_db"
	"v.io/syncbase/v23/syncbase/nosql/syncql"
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

// Split splits str (arg[0]) into substrings separated by sep (arg[1]) and returns an
// array of substrings between those separators. If sep is empty, Split splits after each
// UTF-8 sequence.
// e.g., Split("abc.def.ghi", ".") an list of "abc", "def", "ghi"
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
