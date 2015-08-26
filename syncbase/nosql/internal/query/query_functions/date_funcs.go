// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query_functions

// TODO(jkline): Probably rename this file to time_functions.go

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

// Time(layout, value string)
// e.g., Time("Mon Jan 2 15:04:05 -0700 MST 2006", "Tue Aug 25 10:01:00 -0700 PDT 2015")
// e.g., Time("Jan 2 15:04 MST 2006", "Aug 25 10:01 PDT 2015")
func timeFunc(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	layoutOp, err := conversions.ConvertValueToString(args[0])
	if err != nil {
		return nil, err
	}
	valueOp, err := conversions.ConvertValueToString(args[1])
	if err != nil {
		return nil, err
	}
	// Mon Jan 2 15:04:05 -0700 MST 2006
	tim, err := time.Parse(layoutOp.Str, valueOp.Str)
	if err != nil {
		return nil, err
	}
	return makeTimeOp(off, tim), nil
}

// now()
func now(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	return makeTimeOp(off, time.Now()), nil
}
func timeInLocation(db query_db.Database, off int64, args []*query_parser.Operand) (time.Time, error) {
	var timeOp *query_parser.Operand
	var locOp *query_parser.Operand
	var err error
	if timeOp, err = conversions.ConvertValueToTime(args[0]); err != nil {
		return time.Time{}, err
	}
	if locOp, err = conversions.ConvertValueToString(args[1]); err != nil {
		return time.Time{}, err
	}
	var loc *time.Location
	if loc, err = time.LoadLocation(locOp.Str); err != nil {
		return time.Time{}, err
	}
	return timeOp.Time.In(loc), nil
}

// Year(v.InvoiceDate, "America/Los_Angeles")
func year(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	if tim, err := timeInLocation(db, off, args); err != nil {
		return nil, err
	} else {
		return makeIntOp(off, int64(tim.Year())), nil
	}
}

// Month(v.InvoiceDate, "America/Los_Angeles")
func month(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	if tim, err := timeInLocation(db, off, args); err != nil {
		return nil, err
	} else {
		return makeIntOp(off, int64(tim.Month())), nil
	}
}

// Day(v.InvoiceDate, "America/Los_Angeles")
func day(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	if tim, err := timeInLocation(db, off, args); err != nil {
		return nil, err
	} else {
		return makeIntOp(off, int64(tim.Day())), nil
	}
}

// Hour(v.InvoiceDate, "America/Los_Angeles")
func hour(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	if tim, err := timeInLocation(db, off, args); err != nil {
		return nil, err
	} else {
		return makeIntOp(off, int64(tim.Hour())), nil
	}
}

// Minute(v.InvoiceDate, "America/Los_Angeles")
func minute(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	if tim, err := timeInLocation(db, off, args); err != nil {
		return nil, err
	} else {
		return makeIntOp(off, int64(tim.Minute())), nil
	}
}

// Second(v.InvoiceDate, "America/Los_Angeles")
func second(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	if tim, err := timeInLocation(db, off, args); err != nil {
		return nil, err
	} else {
		return makeIntOp(off, int64(tim.Second())), nil
	}
}

// Nanosecond(v.InvoiceDate, "America/Los_Angeles")
func nanosecond(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	if tim, err := timeInLocation(db, off, args); err != nil {
		return nil, err
	} else {
		return makeIntOp(off, int64(tim.Nanosecond())), nil
	}
}

// Weekday(v.InvoiceDate, "America/Los_Angeles")
func weekday(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	if tim, err := timeInLocation(db, off, args); err != nil {
		return nil, err
	} else {
		return makeIntOp(off, int64(tim.Weekday())), nil
	}
}

// YearDay(v.InvoiceDate, "America/Los_Angeles")
func yearDay(db query_db.Database, off int64, args []*query_parser.Operand) (*query_parser.Operand, error) {
	if tim, err := timeInLocation(db, off, args); err != nil {
		return nil, err
	} else {
		return makeIntOp(off, int64(tim.YearDay())), nil
	}
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
