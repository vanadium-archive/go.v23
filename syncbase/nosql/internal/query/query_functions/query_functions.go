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

type queryFunc func(query_db.Database, int64, []*query_parser.Operand) (*query_parser.Operand, error)
type checkArgsFunc func(query_db.Database, int64, []*query_parser.Operand) error

type function struct {
	argTypes      []query_parser.OperandType
	returnType    query_parser.OperandType
	funcAddr      queryFunc
	checkArgsAddr checkArgsFunc
}

var functions map[string]function

func init() {
	functions = make(map[string]function)

	functions["date"] = function{[]query_parser.OperandType{query_parser.TypStr}, query_parser.TypTime, date, singleTimeArgCheck}
	functions["datetime"] = function{[]query_parser.OperandType{query_parser.TypStr}, query_parser.TypTime, dateTime, singleTimeArgCheck}
	functions["y"] = function{[]query_parser.OperandType{query_parser.TypStr, query_parser.TypStr}, query_parser.TypTime, y, timeAndStringArgsCheck}
	functions["ym"] = function{[]query_parser.OperandType{query_parser.TypStr, query_parser.TypStr}, query_parser.TypTime, ym, timeAndStringArgsCheck}
	functions["ymd"] = function{[]query_parser.OperandType{query_parser.TypStr, query_parser.TypStr}, query_parser.TypTime, ymd, timeAndStringArgsCheck}
	functions["ymdh"] = function{[]query_parser.OperandType{query_parser.TypStr, query_parser.TypStr}, query_parser.TypTime, ymdh, timeAndStringArgsCheck}
	functions["ymdhm"] = function{[]query_parser.OperandType{query_parser.TypStr, query_parser.TypStr}, query_parser.TypTime, ymdhm, timeAndStringArgsCheck}
	functions["ymdhms"] = function{[]query_parser.OperandType{query_parser.TypStr, query_parser.TypStr}, query_parser.TypTime, ymdhms, timeAndStringArgsCheck}
	functions["now"] = function{[]query_parser.OperandType{}, query_parser.TypTime, now, nil}

	functions["lowercase"] = function{[]query_parser.OperandType{query_parser.TypStr}, query_parser.TypStr, lowerCase, singleStringArgCheck}
	functions["split"] = function{[]query_parser.OperandType{query_parser.TypStr, query_parser.TypStr}, query_parser.TypObject, split, twoStringArgsCheck}
	functions["type"] = function{[]query_parser.OperandType{query_parser.TypObject}, query_parser.TypStr, typeFunc, singleFieldArgCheck}
	functions["uppercase"] = function{[]query_parser.OperandType{query_parser.TypStr}, query_parser.TypStr, upperCase, singleStringArgCheck}

	functions["complex"] = function{[]query_parser.OperandType{query_parser.TypFloat, query_parser.TypFloat}, query_parser.TypComplex, complexFunc, twoFloatsArgsCheck}

	functions["len"] = function{[]query_parser.OperandType{query_parser.TypObject}, query_parser.TypInt, lenFunc, nil}
}

// Check that function exists and that the number of args passed matches the spec.
// Call query_functions.CheckFunction.  This will check for correct number of args
// and, to the extent possible, correct types.
// Furthermore, it may execute the function if the function takes no args or
// takes only literal args (or an arg that is a function that is also executed
// early).  CheckFunction will fill in arg types, return types and may fill in
// Computed and RetValue.
func CheckFunction(db query_db.Database, f *query_parser.Function) error {
	if entry, ok := functions[strings.ToLower(f.Name)]; !ok {
		return syncql.NewErrFunctionNotFound(db.GetContext(), f.Off, f.Name)
	} else {
		f.ArgTypes = entry.argTypes
		f.RetType = entry.returnType
		if len(f.ArgTypes) != len(f.Args) {
			return syncql.NewErrFunctionArgCount(db.GetContext(), f.Off, f.Name, int64(len(f.ArgTypes)), int64(len(f.Args)))
		}
		// Check if the function can be executed now.
		// If any arg is not a literal and not a function that has been already executed,
		// then okToExecuteNow will be set to false.
		okToExecuteNow := true
		for _, arg := range f.Args {
			switch arg.Type {
			case query_parser.TypBigInt, query_parser.TypBigRat, query_parser.TypBool, query_parser.TypComplex, query_parser.TypFloat, query_parser.TypInt, query_parser.TypStr, query_parser.TypTime, query_parser.TypUint:
				// do nothing
			case query_parser.TypFunction:
				if !arg.Function.Computed {
					okToExecuteNow = false
					break
				}
			default:
				okToExecuteNow = false
				break
			}
		}
		// If all of the functions args are literals or already computed functions,
		// execute this function now and save the result.
		if okToExecuteNow {
			op, err := ExecFunction(db, f, f.Args)
			if err != nil {
				return err
			}
			f.Computed = true
			f.RetValue = op
			return nil
		} else {
			// We can't execute now, but give the function a chance to complain
			// about the arguments that it can check now.
			return FuncCheck(db, f, f.Args)
		}
	}
}

func FuncCheck(db query_db.Database, f *query_parser.Function, args []*query_parser.Operand) error {
	if entry, ok := functions[strings.ToLower(f.Name)]; !ok {
		return syncql.NewErrFunctionNotFound(db.GetContext(), f.Off, f.Name)
	} else {
		if entry.checkArgsAddr != nil {
			if err := entry.checkArgsAddr(db, f.Off, args); err != nil {
				return err
			}
		}
	}
	return nil
}

func ExecFunction(db query_db.Database, f *query_parser.Function, args []*query_parser.Operand) (*query_parser.Operand, error) {
	if entry, ok := functions[strings.ToLower(f.Name)]; !ok {
		return nil, syncql.NewErrFunctionNotFound(db.GetContext(), f.Off, f.Name)
	} else {
		retValue, err := entry.funcAddr(db, f.Off, args)
		if err != nil {
			return nil, err
		} else {
			return retValue, nil
		}
	}
}

func ConvertFunctionRetValueToVdlValue(o *query_parser.Operand) *vdl.Value {
	if o == nil {
		return vdl.ValueOf(nil)
	}
	switch o.Type {
	case query_parser.TypBool:
		return vdl.ValueOf(o.Bool)
	case query_parser.TypComplex:
		return vdl.ValueOf(o.Complex)
	case query_parser.TypFloat:
		return vdl.ValueOf(o.Float)
	case query_parser.TypInt:
		return vdl.ValueOf(o.Int)
	case query_parser.TypStr:
		return vdl.ValueOf(o.Str)
	case query_parser.TypTime:
		return vdl.ValueOf(o.Time)
	case query_parser.TypObject:
		return vdl.ValueOf(o.Object)
	case query_parser.TypUint:
		return vdl.ValueOf(o.Uint)
	default:
		// Other types can't be converted and *shouldn't* be returned
		// from a function.  This case will result in a nil for this
		// column in the row.
		return vdl.ValueOf(nil)
	}
}

func makeStrOp(off int64, s string) *query_parser.Operand {
	var o query_parser.Operand
	o.Off = off
	o.Type = query_parser.TypStr
	o.Str = s
	return &o
}

func makeComplexOp(off int64, c complex128) *query_parser.Operand {
	var o query_parser.Operand
	o.Off = off
	o.Type = query_parser.TypComplex
	o.Complex = c
	return &o
}

func makeIntOp(off int64, i int64) *query_parser.Operand {
	var o query_parser.Operand
	o.Off = off
	o.Type = query_parser.TypInt
	o.Int = i
	return &o
}

func singleStringArgCheck(db query_db.Database, off int64, args []*query_parser.Operand) error {
	return checkIfPossibleThatArgIsConvertableToString(db, args[0])
}

func twoStringArgsCheck(db query_db.Database, off int64, args []*query_parser.Operand) error {
	if err := checkIfPossibleThatArgIsConvertableToString(db, args[0]); err != nil {
		return err
	}
	return checkIfPossibleThatArgIsConvertableToString(db, args[1])
}

func singleFieldArgCheck(db query_db.Database, off int64, args []*query_parser.Operand) error {
	// single argument must be of type field
	// It must begin with a v segment.
	if args[0].Type != query_parser.TypField || len(args[0].Column.Segments) < 1 || strings.ToLower(args[0].Column.Segments[0].Value) != "v" {
		return syncql.NewErrArgMustBeField(db.GetContext(), args[0].Off)
	}
	return nil
}

// If possible, check if arg is convertable to a string.  Fields and not yet computed
// functions cannot be checked and will just return nil.
func checkIfPossibleThatArgIsConvertableToString(db query_db.Database, arg *query_parser.Operand) error {
	// If arg is a literal or an already computed function,
	// make sure it can be converted to a string.
	switch arg.Type {
	case query_parser.TypBigInt, query_parser.TypBigRat, query_parser.TypBool, query_parser.TypComplex, query_parser.TypFloat, query_parser.TypInt, query_parser.TypStr, query_parser.TypTime, query_parser.TypUint:
		_, err := conversions.ConvertValueToString(arg)
		if err != nil {
			return syncql.NewErrStringConversionError(db.GetContext(), arg.Off, err)
		} else {
			return nil
		}
	case query_parser.TypFunction:
		if arg.Function.Computed {
			_, err := conversions.ConvertValueToString(arg.Function.RetValue)
			if err != nil {
				return syncql.NewErrStringConversionError(db.GetContext(), arg.Off, err)
			} else {
				return nil
			}
		}
	}
	return nil
}
