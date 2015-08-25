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
	"v.io/syncbase/v23/syncbase/nosql/syncql"
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/vdl"
	"v.io/v23/verror"
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

type functionsErrorTest struct {
	f    *query_parser.Function
	args []*query_parser.Operand
	err  error
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
		// Lowercase
		functionsTest{
			&query_parser.Function{
				Name: "Lowercase",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "FooBar",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
				},
				RetType:  query_parser.TypStr,
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
		// Uppercase
		functionsTest{
			&query_parser.Function{
				Name: "Uppercase",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "FooBar",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
				},
				RetType:  query_parser.TypStr,
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
		// Split
		functionsTest{
			&query_parser.Function{
				Name: "Split",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "alpha.bravo.charlie.delta.echo",
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  ".",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
					query_parser.TypStr,
				},
				RetType:  query_parser.TypObject,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "alpha.bravo.charlie.delta.echo",
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  ".",
				},
			},
			&query_parser.Operand{
				Type:   query_parser.TypObject,
				Object: vdl.ValueOf([]string{"alpha", "bravo", "charlie", "delta", "echo"}),
			},
		},
		// Len (of list)
		functionsTest{
			&query_parser.Function{
				Name: "Len",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type:   query_parser.TypObject,
						Object: vdl.ValueOf([]string{"alpha", "bravo"}),
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypObject,
				},
				RetType:  query_parser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type:   query_parser.TypObject,
					Object: vdl.ValueOf([]string{"alpha", "bravo"}),
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypInt,
				Int:  2,
			},
		},
		// Len (of nil)
		functionsTest{
			&query_parser.Function{
				Name: "Len",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypNil,
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypObject,
				},
				RetType:  query_parser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypNil,
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypInt,
				Int:  0,
			},
		},
		// Len (of string)
		functionsTest{
			&query_parser.Function{
				Name: "Len",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "foo",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypObject,
				},
				RetType:  query_parser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "foo",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypInt,
				Int:  3,
			},
		},
		// Len (of map)
		functionsTest{
			&query_parser.Function{
				Name: "Len",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type:   query_parser.TypObject,
						Object: vdl.ValueOf(map[string]string{"alpha": "ALPHA", "bravo": "BRAVO"}),
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypObject,
				},
				RetType:  query_parser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type:   query_parser.TypObject,
					Object: vdl.ValueOf(map[string]string{"alpha": "ALPHA", "bravo": "BRAVO"}),
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypInt,
				Int:  2,
			},
		},
		// StrCat (2 args)
		functionsTest{
			&query_parser.Function{
				Name: "StrCat",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "Foo",
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "Bar",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
					query_parser.TypStr,
				},
				RetType:  query_parser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "Foo",
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "Bar",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypStr,
				Str:  "FooBar",
			},
		},
		// StrCat (3 args)
		functionsTest{
			&query_parser.Function{
				Name: "StrCat",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "Foo",
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  ",",
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "Bar",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
					query_parser.TypStr,
				},
				RetType:  query_parser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "Foo",
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  ",",
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "Bar",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypStr,
				Str:  "Foo,Bar",
			},
		},
		// StrCat (5 args)
		functionsTest{
			&query_parser.Function{
				Name: "StrCat",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "[",
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "Foo",
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "]",
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "[",
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "Bar",
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "]",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
					query_parser.TypStr,
				},
				RetType:  query_parser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "[",
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "Foo",
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "]",
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "[",
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "Bar",
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "]",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypStr,
				Str:  "[Foo][Bar]",
			},
		},
		// StrIndex
		functionsTest{
			&query_parser.Function{
				Name: "StrIndex",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "FooBar",
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "Bar",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
					query_parser.TypStr,
				},
				RetType:  query_parser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "FooBar",
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "Bar",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypInt,
				Int:  3,
			},
		},
		// StrIndex
		functionsTest{
			&query_parser.Function{
				Name: "StrIndex",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "FooBar",
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "Baz",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
					query_parser.TypStr,
				},
				RetType:  query_parser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "FooBar",
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "Baz",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypInt,
				Int:  -1,
			},
		},
		// StrLastIndex
		functionsTest{
			&query_parser.Function{
				Name: "StrLastIndex",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "FooBarBar",
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "Bar",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
					query_parser.TypStr,
				},
				RetType:  query_parser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "FooBarBar",
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "Bar",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypInt,
				Int:  6,
			},
		},
		// StrLastIndex
		functionsTest{
			&query_parser.Function{
				Name: "StrLastIndex",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "FooBar",
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "Baz",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
					query_parser.TypStr,
				},
				RetType:  query_parser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "FooBar",
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "Baz",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypInt,
				Int:  -1,
			},
		},
		// StrRepeat
		functionsTest{
			&query_parser.Function{
				Name: "StrRepeat",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "FooBar",
					},
					&query_parser.Operand{
						Type: query_parser.TypInt,
						Int:  2,
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
					query_parser.TypInt,
				},
				RetType:  query_parser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "FooBar",
				},
				&query_parser.Operand{
					Type: query_parser.TypInt,
					Int:  2,
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypStr,
				Str:  "FooBarFooBar",
			},
		},
		// StrRepeat
		functionsTest{
			&query_parser.Function{
				Name: "StrRepeat",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "FooBar",
					},
					&query_parser.Operand{
						Type: query_parser.TypInt,
						Int:  0,
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
					query_parser.TypInt,
				},
				RetType:  query_parser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "FooBar",
				},
				&query_parser.Operand{
					Type: query_parser.TypInt,
					Int:  0,
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypStr,
				Str:  "",
			},
		},
		// StrRepeat
		functionsTest{
			&query_parser.Function{
				Name: "StrRepeat",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "FooBar",
					},
					&query_parser.Operand{
						Type: query_parser.TypInt,
						Int:  -1,
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
					query_parser.TypInt,
				},
				RetType:  query_parser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "FooBar",
				},
				&query_parser.Operand{
					Type: query_parser.TypInt,
					Int:  -1,
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypStr,
				Str:  "",
			},
		},
		// StrReplace
		functionsTest{
			&query_parser.Function{
				Name: "StrReplace",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "FooBar",
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "B",
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "ZZZ",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
					query_parser.TypStr,
					query_parser.TypStr,
				},
				RetType:  query_parser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "FooBar",
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "B",
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "ZZZ",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypStr,
				Str:  "FooZZZar",
			},
		},
		// StrReplace
		functionsTest{
			&query_parser.Function{
				Name: "StrReplace",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "FooBar",
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "X",
					},
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "ZZZ",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
					query_parser.TypStr,
					query_parser.TypStr,
				},
				RetType:  query_parser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "FooBar",
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "X",
				},
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "ZZZ",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypStr,
				Str:  "FooBar",
			},
		},
		// Trim
		functionsTest{
			&query_parser.Function{
				Name: "Trim",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "     Foo  ",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
				},
				RetType:  query_parser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "     Foo  ",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypStr,
				Str:  "Foo",
			},
		},
		// Trim
		functionsTest{
			&query_parser.Function{
				Name: "Trim",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "Foo",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
				},
				RetType:  query_parser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "Foo",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypStr,
				Str:  "Foo",
			},
		},
		// TrimLeft
		functionsTest{
			&query_parser.Function{
				Name: "TrimLeft",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "     Foo  ",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
				},
				RetType:  query_parser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "     Foo  ",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypStr,
				Str:  "Foo  ",
			},
		},
		// TrimLeft
		functionsTest{
			&query_parser.Function{
				Name: "TrimLeft",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "Foo",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
				},
				RetType:  query_parser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "Foo",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypStr,
				Str:  "Foo",
			},
		},
		// TrimRight
		functionsTest{
			&query_parser.Function{
				Name: "TrimRight",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "     Foo  ",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
				},
				RetType:  query_parser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "     Foo  ",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypStr,
				Str:  "     Foo",
			},
		},
		// TrimLeft
		functionsTest{
			&query_parser.Function{
				Name: "TrimLeft",
				Args: []*query_parser.Operand{
					&query_parser.Operand{
						Type: query_parser.TypStr,
						Str:  "Foo",
					},
				},
				ArgTypes: []query_parser.OperandType{
					query_parser.TypStr,
				},
				RetType:  query_parser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "Foo",
				},
			},
			&query_parser.Operand{
				Type: query_parser.TypStr,
				Str:  "Foo",
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

func TestErrorFunctions(t *testing.T) {
	tests := []functionsErrorTest{
		// date
		functionsErrorTest{
			&query_parser.Function{
				Name: "date",
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
				Node:     query_parser.Node{Off: 42},
			},
			[]*query_parser.Operand{
				&query_parser.Operand{
					Type: query_parser.TypStr,
					Str:  "2015-06-21 PDT",
				},
			},
			syncql.NewErrDidYouMeanFunction(db.GetContext(), int64(42), "Date"),
		},
	}

	for _, test := range tests {
		_, err := query_functions.ExecFunction(&db, test.f, test.args)
		if verror.ErrorID(err) != verror.ErrorID(test.err) || err.Error() != test.err.Error() {
			t.Errorf("function: %v; got %v, want %v", test.f, err, test.err)
		}
	}
}
