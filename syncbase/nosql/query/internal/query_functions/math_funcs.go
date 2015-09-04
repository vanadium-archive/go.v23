// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query_functions

import (
	"math"

	"v.io/v23/syncbase/nosql/query"
	"v.io/v23/syncbase/nosql/query/internal/conversions"
	"v.io/v23/syncbase/nosql/query/internal/query_parser"
)

func ceilingFunc(db query.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	f, err := conversions.ConvertValueToFloat(args[0])
	if err != nil {
		return nil, err
	}
	return makeFloatOp(off, math.Ceil(f.Float)), nil
}

func floorFunc(db query.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	f, err := conversions.ConvertValueToFloat(args[0])
	if err != nil {
		return nil, err
	}
	return makeFloatOp(off, math.Floor(f.Float)), nil
}

func isNanFunc(db query.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	f, err := conversions.ConvertValueToFloat(args[0])
	if err != nil {
		return nil, err
	}
	return makeBoolOp(off, math.IsNaN(f.Float)), nil
}

func isInfFunc(db query.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	f, err := conversions.ConvertValueToFloat(args[0])
	if err != nil {
		return nil, err
	}
	i, err := conversions.ConvertValueToInt(args[1])
	if err != nil {
		return nil, err
	}
	var sign int
	if i.Int < 0 {
		sign = -1
	} else if i.Int == 0 {
		sign = 0
	} else {
		sign = 1
	}

	return makeBoolOp(off, math.IsInf(f.Float, sign)), nil
}

func logFunc(db query.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	f, err := conversions.ConvertValueToFloat(args[0])
	if err != nil {
		return nil, err
	}
	return makeFloatOp(off, math.Log(f.Float)), nil
}

func log10Func(db query.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	f, err := conversions.ConvertValueToFloat(args[0])
	if err != nil {
		return nil, err
	}
	return makeFloatOp(off, math.Log10(f.Float)), nil
}

func powFunc(db query.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	x, err := conversions.ConvertValueToFloat(args[0])
	if err != nil {
		return nil, err
	}
	y, err := conversions.ConvertValueToFloat(args[1])
	if err != nil {
		return nil, err
	}
	return makeFloatOp(off, math.Pow(x.Float, y.Float)), nil
}

func pow10Func(db query.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	x, err := conversions.ConvertValueToInt(args[0])
	if err != nil {
		return nil, err
	}
	return makeFloatOp(off, math.Pow10(int(x.Int))), nil
}

func modFunc(db query.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	x, err := conversions.ConvertValueToFloat(args[0])
	if err != nil {
		return nil, err
	}

	y, err := conversions.ConvertValueToFloat(args[1])
	if err != nil {
		return nil, err
	}

	return makeFloatOp(off, math.Mod(x.Float, y.Float)), nil
}

func truncateFunc(db query.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	f, err := conversions.ConvertValueToFloat(args[0])
	if err != nil {
		return nil, err
	}
	return makeFloatOp(off, math.Trunc(f.Float)), nil
}

func remainderFunc(db query.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	x, err := conversions.ConvertValueToFloat(args[0])
	if err != nil {
		return nil, err
	}

	y, err := conversions.ConvertValueToFloat(args[1])
	if err != nil {
		return nil, err
	}

	return makeFloatOp(off, math.Remainder(x.Float, y.Float)), nil
}

func complexFunc(db query.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
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

func realFunc(db query.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	c, err := conversions.ConvertValueToComplex(args[0])
	if err != nil {
		return nil, err
	}

	return makeFloatOp(off, real(c.Complex)), nil
}
