// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query_functions

import (
	"v.io/syncbase/v23/syncbase/nosql/internal/query/conversions"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_parser"
	"v.io/syncbase/v23/syncbase/nosql/query_db"
	"v.io/syncbase/v23/syncbase/nosql/syncql"
)

func complexFunc(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	r, err := conversions.ConvertValueToFloat(args[0])
	if err != nil {
		return nil, err
	}

	i, err := conversions.ConvertValueToFloat(args[1])
	if err != nil {
		return nil, err
	}

	return makeComplexOp(off, complex(r.Float, i.Float)), nil
}

func twoFloatsArgsCheck(db query_db.Database, off int64, args []*query_parser.Operand) error {
	// The two args must be convertable to floats.
	for i := 0; i < 2; i++ {
		if err := checkIfPossibleThatArgIsConvertableToFloat(db, args[i]); err != nil {
			return syncql.NewErrFloatConversionError(db.GetContext(), args[i].Off, err)
		}
	}
	return nil
}
