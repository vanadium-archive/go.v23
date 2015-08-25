// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query_functions

import (
	"time"

	"v.io/syncbase/v23/syncbase/nosql/internal/query/conversions"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_parser"
	"v.io/syncbase/v23/syncbase/nosql/query_db"
	"v.io/syncbase/v23/syncbase/nosql/syncql"
)

// If possible, check if arg is convertable to a location.  Fields and not yet computed
// functions cannot be checked and will just return nil.
func checkIfPossibleThatArgIsConvertableToLocation(db query_db.Database, arg *query_parser.Operand) error {
	var locStr *query_parser.Operand
	var err error
	switch arg.Type {
	case query_parser.TypBigInt, query_parser.TypBigRat, query_parser.TypBool, query_parser.TypComplex, query_parser.TypFloat, query_parser.TypInt, query_parser.TypStr, query_parser.TypTime, query_parser.TypUint:
		if locStr, err = conversions.ConvertValueToString(arg); err != nil {
			if err != nil {
				return syncql.NewErrLocationConversionError(db.GetContext(), arg.Off, err)
			} else {
				return nil
			}
		}
	case query_parser.TypFunction:
		if arg.Function.Computed {
			if locStr, err = conversions.ConvertValueToString(arg.Function.RetValue); err != nil {
				if err != nil {
					return syncql.NewErrLocationConversionError(db.GetContext(), arg.Off, err)
				} else {
					return nil
				}
			}
		} else {
			// Arg is uncomputed function, can't make determination about arg.
			return nil
		}
	default:
		// Arg is not a literal or function, can't make determination about arg.
		return nil
	}
	_, err = time.LoadLocation(locStr.Str)
	if err != nil {
		return syncql.NewErrLocationConversionError(db.GetContext(), arg.Off, err)
	} else {
		return nil
	}
}

// Input: "YYYY-MM-DD TZ"
// "2015-03-17 PDT"
func date(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	dateStrOp, err := conversions.ConvertValueToString(args[0])
	if err != nil {
		return nil, err
	}
	// Mon Jan 2 15:04:05 -0700 MST 2006
	tim, err := time.Parse("2006-01-02 MST", dateStrOp.Str)
	if err != nil {
		return nil, err
	}
	return makeTimeOp(off, tim), nil
}

// "YYYY-MM-DD HH:MI:SS TZ"
// "2015-03-17 13:22:17 PDT"
func dateTime(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	dateStrOp, err := conversions.ConvertValueToString(args[0])
	if err != nil {
		return nil, err
	}
	// Mon Jan 2 15:04:05 -0700 MST 2006
	tim, err := time.Parse("2006-01-02 15:04:05 MST", dateStrOp.Str)
	if err != nil {
		return nil, err
	}
	return makeTimeOp(off, tim), nil
}

// y(v.InvoiceDate, "America/Los_Angeles")
func y(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	var timeOp *query_parser.Operand
	var locOp *query_parser.Operand
	var err error
	if timeOp, err = conversions.ConvertValueToTime(args[0]); err != nil {
		return nil, err
	}
	if locOp, err = conversions.ConvertValueToString(args[1]); err != nil {
		return nil, err
	}
	var loc *time.Location
	if loc, err = time.LoadLocation(locOp.Str); err != nil {
		return nil, err
	}
	dateStr := timeOp.Time.In(loc).Format("2006 MST")
	var tim time.Time
	if tim, err = time.Parse("2006 MST", dateStr); err != nil {
		return nil, err
	}
	return makeTimeOp(off, tim), nil
}

// ym(v.InvoiceDate, "America/Los_Angeles")
func ym(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	var timeOp *query_parser.Operand
	var locOp *query_parser.Operand
	var err error
	if timeOp, err = conversions.ConvertValueToTime(args[0]); err != nil {
		return nil, err
	}
	if locOp, err = conversions.ConvertValueToString(args[1]); err != nil {
		return nil, err
	}
	var loc *time.Location
	if loc, err = time.LoadLocation(locOp.Str); err != nil {
		return nil, err
	}
	dateStr := timeOp.Time.In(loc).Format("200601 MST")
	var tim time.Time
	if tim, err = time.Parse("200601 MST", dateStr); err != nil {
		return nil, err
	}
	return makeTimeOp(off, tim), nil
}

// ymd(v.InvoiceDate, "America/Los_Angeles")
func ymd(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	var timeOp *query_parser.Operand
	var locOp *query_parser.Operand
	var err error
	if timeOp, err = conversions.ConvertValueToTime(args[0]); err != nil {
		return nil, err
	}
	if locOp, err = conversions.ConvertValueToString(args[1]); err != nil {
		return nil, err
	}
	var loc *time.Location
	if loc, err = time.LoadLocation(locOp.Str); err != nil {
		return nil, err
	}
	dateStr := timeOp.Time.In(loc).Format("20060102 MST")
	var tim time.Time
	if tim, err = time.Parse("20060102 MST", dateStr); err != nil {
		return nil, err
	}
	return makeTimeOp(off, tim), nil
}

// ymdh(v.InvoiceDate, "America/Los_Angeles")
func ymdh(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	var timeOp *query_parser.Operand
	var locOp *query_parser.Operand
	var err error
	if timeOp, err = conversions.ConvertValueToTime(args[0]); err != nil {
		return nil, err
	}
	if locOp, err = conversions.ConvertValueToString(args[1]); err != nil {
		return nil, err
	}
	var loc *time.Location
	if loc, err = time.LoadLocation(locOp.Str); err != nil {
		return nil, err
	}
	dateStr := timeOp.Time.In(loc).Format("20060102 15 MST")
	var tim time.Time
	if tim, err = time.Parse("20060102 15 MST", dateStr); err != nil {
		return nil, err
	}
	return makeTimeOp(off, tim), nil
}

// ymdhm(v.InvoiceDate, "America/Los_Angeles")
func ymdhm(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	var timeOp *query_parser.Operand
	var locOp *query_parser.Operand
	var err error
	if timeOp, err = conversions.ConvertValueToTime(args[0]); err != nil {
		return nil, err
	}
	if locOp, err = conversions.ConvertValueToString(args[1]); err != nil {
		return nil, err
	}
	var loc *time.Location
	if loc, err = time.LoadLocation(locOp.Str); err != nil {
		return nil, err
	}
	dateStr := timeOp.Time.In(loc).Format("20060102 15:04 MST")
	var tim time.Time
	if tim, err = time.Parse("20060102 15:04 MST", dateStr); err != nil {
		return nil, err
	}
	return makeTimeOp(off, tim), nil
}

// ymdhms(v.InvoiceDate, "America/Los_Angeles")
func ymdhms(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	var timeOp *query_parser.Operand
	var locOp *query_parser.Operand
	var err error
	if timeOp, err = conversions.ConvertValueToTime(args[0]); err != nil {
		return nil, err
	}
	if locOp, err = conversions.ConvertValueToString(args[1]); err != nil {
		return nil, err
	}
	var loc *time.Location
	if loc, err = time.LoadLocation(locOp.Str); err != nil {
		return nil, err
	}
	dateStr := timeOp.Time.In(loc).Format("20060102 15:04:05 MST")
	var tim time.Time
	if tim, err = time.Parse("20060102 15:04:05 MST", dateStr); err != nil {
		return nil, err
	}
	return makeTimeOp(off, tim), nil
}

// now()
func now(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	return makeTimeOp(off, time.Now()), nil
}

func makeTimeOp(off int64, tim time.Time) *query_parser.Operand {
	var o query_parser.Operand
	o.Off = off
	o.Type = query_parser.TypTime
	o.Time = tim
	return &o
}

func secondArgLocationCheck(db query_db.Database, off int64, args []*query_parser.Operand) error {
	// At this point, for the args that can be evaluated before execution, it is known that
	// there are two args, a time followed by a string.
	// Just need to check that the 2nd arg is convertable to a location.
	if err := checkIfPossibleThatArgIsConvertableToLocation(db, args[1]); err != nil {
		return err
	}
	return nil
}
