// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query_functions

import (
	"strings"

	"v.io/syncbase/v23/syncbase/nosql/internal/query/conversions"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_parser"
)

func lowerCase(off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	strOp, err := conversions.ConvertValueToString(args[0])
	if err != nil {
		return nil, err
	}
	return makeStrOp(off, strings.ToLower(strOp.Str)), nil
}

func upperCase(off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	strOp, err := conversions.ConvertValueToString(args[0])
	if err != nil {
		return nil, err
	}
	return makeStrOp(off, strings.ToUpper(strOp.Str)), nil
}
