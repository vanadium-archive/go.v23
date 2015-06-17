// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query_functions_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_functions"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_parser"
	"v.io/syncbase/v23/syncbase/nosql/query_db"
	"v.io/v23"
	"v.io/v23/context"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

type mockDB struct {
	ctx *context.T
}

func (db *mockDB) GetContext() *context.T {
	return db.ctx
}

func init() {
	var shutdown v23.Shutdown
	db.ctx, shutdown = test.V23Init()
	defer shutdown()
}

func (db *mockDB) GetTable(table string) (query_db.Table, error) {
	return nil, errors.New("unimplemented")
}

var db mockDB

//type queryFunc func(int64, []*query_parser.Operand) (*query_parser.Operand, error)
//type checkArgsFunc func(int64, []*query_parser.Operand) (*query_parser.Operand, error)

type functionsTest struct {
	f      *query_parser.Function
	args   []*query_parser.Operand
	result *query_parser.Operand
}

var t_2015 time.Time
var t_2015_06 time.Time
var t_2015_06_21 time.Time
var t_2015_06_21_01 time.Time
var t_2015_06_21_01_23 time.Time
var t_2015_06_21_01_23_45 time.Time

func init() {
	// Mon Jan 2 15:04:05 -0700 MST 2006
	t_2015, _ = time.Parse("2006 MST", "2015 PDT")
	t_2015_06, _ = time.Parse("2006/01 MST", "2015/06 PDT")
	t_2015_06_21, _ = time.Parse("2006/01/02 MST", "2015/06/21 PDT")
	t_2015_06_21_01, _ = time.Parse("2006/01/02 15 MST", "2015/06/21 01 PDT")
	t_2015_06_21_01_23, _ = time.Parse("2006/01/02 15:04 MST", "2015/06/21 01:23 PDT")
	t_2015_06_21_01_23_45, _ = time.Parse("2006/01/02 15:04:05 MST", "2015/06/21 01:23:45 PDT")
}

func TestFunctions(t *testing.T) {
	tests := []functionsTest{
		// Date
		functionsTest{
			&query_parser.Function{
				Name: "Date",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "2015-06-21 PDT",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
				},
				RetType:  query_parser.TypTime,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "2015-06-21 PDT",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypTime,
				Time: t_2015_06_21,
			},
		},
		// DateTime
		functionsTest{
			&query_parser.Function{
				Name: "DateTime",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "2015-06-21 01:23:45 PDT",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
				},
				RetType:  query_parser.TypTime,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "2015-06-21 01:23:45 PDT",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypTime,
				Time: t_2015_06_21_01_23_45,
			},
		},
		// Y
		functionsTest{
			&query_parser.Function{
				Name: "Y",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypTime,
						Time: t_2015_06_21_01_23_45,
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "America/Los_Angeles",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
					query_parser.TypStr,
				},
				RetType:  query_parser.TypTime,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypTime,
					Time: t_2015_06_21_01_23_45,
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "America/Los_Angeles",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypTime,
				Time: t_2015,
			},
		},
		// YM
		functionsTest{
			&query_parser.Function{
				Name: "YM",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypTime,
						Time: t_2015_06_21_01_23_45,
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "America/Los_Angeles",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
					query_parser.TypStr,
				},
				RetType:  query_parser.TypTime,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypTime,
					Time: t_2015_06_21_01_23_45,
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "America/Los_Angeles",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypTime,
				Time: t_2015_06,
			},
		},
		// YMD
		functionsTest{
			&query_parser.Function{
				Name: "YMD",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypTime,
						Time: t_2015_06_21_01_23_45,
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "America/Los_Angeles",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
					query_parser.TypStr,
				},
				RetType:  query_parser.TypTime,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypTime,
					Time: t_2015_06_21_01_23_45,
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "America/Los_Angeles",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypTime,
				Time: t_2015_06_21,
			},
		},
		// YMDH
		functionsTest{
			&query_parser.Function{
				Name: "YMDH",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypTime,
						Time: t_2015_06_21_01_23_45,
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "America/Los_Angeles",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
					query_parser.TypStr,
				},
				RetType:  query_parser.TypTime,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypTime,
					Time: t_2015_06_21_01_23_45,
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "America/Los_Angeles",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypTime,
				Time: t_2015_06_21_01,
			},
		},
		// YMDHM
		functionsTest{
			&query_parser.Function{
				Name: "YMDHM",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypTime,
						Time: t_2015_06_21_01_23_45,
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "America/Los_Angeles",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
					query_parser.TypStr,
				},
				RetType:  query_parser.TypTime,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypTime,
					Time: t_2015_06_21_01_23_45,
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "America/Los_Angeles",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypTime,
				Time: t_2015_06_21_01_23,
			},
		},
		// YMDHMS
		functionsTest{
			&query_parser.Function{
				Name: "YMDHMS",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypTime,
						Time: t_2015_06_21_01_23_45,
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "America/Los_Angeles",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
					query_parser.TypStr,
				},
				RetType:  query_parser.TypTime,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypTime,
					Time: t_2015_06_21_01_23_45,
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "America/Los_Angeles",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypTime,
				Time: t_2015_06_21_01_23_45,
			},
		},
		// LowerCase
		functionsTest{
			&query_parser.Function{
				Name: "LowerCase",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "FooBar",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
				},
				RetType:  query_parser.TypTime,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "FooBar",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypStr,
				Str:  "foobar",
			},
		},
		// UpperCase
		functionsTest{
			&query_parser.Function{
				Name: "UpperCase",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "FooBar",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
				},
				RetType:  query_parser.TypTime,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "FooBar",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypStr,
				Str:  "FOOBAR",
			},
		},
	}

	for _, test := range tests {
		r, err := query_functions.ExecFunction(&db, test.f, test.args)
		if err != nil {
			t.Errorf("function: %v; unexpected error: got %v, want nil", test.f, err)
		}
		if !reflect.DeepEqual(test.result, r) {
			t.Errorf("function: %v; got %v, want %v", test.f, r, test.result)
		}
	}
}
