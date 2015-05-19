// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query_parser_test

import (
	"reflect"
	"testing"

	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_db"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_parser"
	"v.io/syncbase/v23/syncbase/nosql/syncql"
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

type parseSelectTest struct {
	query     string
	statement query_parser.SelectStatement
	err       error
}

type parseSelectErrorTest struct {
	query string
	err   error
}

type mockDB struct {
	ctx *context.T
}

func (db *mockDB) GetContext() *context.T {
	return db.ctx
}

func (db *mockDB) GetTable(name string) (query_db.Table, error) {
	return nil, nil
}

var db mockDB

func init() {
	var shutdown v23.Shutdown
	db.ctx, shutdown = test.InitForTest()
	defer shutdown()
}

func TestQueryParser(t *testing.T) {
	basic := []parseSelectTest{
		{
			"select v from Customer",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select k as Key, v as Value from Customer",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "k",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							As: &query_parser.AsClause{
								AltName: query_parser.Name{
									Value: "Key",
									Node:  query_parser.Node{Off: 12},
								},
								Node: query_parser.Node{Off: 9},
							},
							Node: query_parser.Node{Off: 7},
						},
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 17},
									},
								},
								Node: query_parser.Node{Off: 17},
							},
							As: &query_parser.AsClause{
								AltName: query_parser.Name{
									Value: "Value",
									Node:  query_parser.Node{Off: 22},
								},
								Node: query_parser.Node{Off: 19},
							},
							Node: query_parser.Node{Off: 17},
						},
					},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 33},
					},
					Node: query_parser.Node{Off: 28},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"   select v from Customer",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 10},
									},
								},
								Node: query_parser.Node{Off: 10},
							},
							Node: query_parser.Node{Off: 10},
						},
					},
					Node: query_parser.Node{Off: 3},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 17},
					},
					Node: query_parser.Node{Off: 12},
				},
				Node: query_parser.Node{Off: 3},
			},
			nil,
		},
		{
			"select v from Customer limit 100 offset 200",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Limit: &query_parser.LimitClause{
					Limit: &query_parser.Int64Value{
						Value: 100,
						Node:  query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				ResultsOffset: &query_parser.ResultsOffsetClause{
					ResultsOffset: &query_parser.Int64Value{
						Value: 200,
						Node:  query_parser.Node{Off: 40},
					},
					Node: query_parser.Node{Off: 33},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer offset 400 limit 10",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Limit: &query_parser.LimitClause{
					Limit: &query_parser.Int64Value{
						Value: 10,
						Node:  query_parser.Node{Off: 40},
					},
					Node: query_parser.Node{Off: 34},
				},
				ResultsOffset: &query_parser.ResultsOffsetClause{
					ResultsOffset: &query_parser.Int64Value{
						Value: 400,
						Node:  query_parser.Node{Off: 30},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer limit 100 offset 200 limit 1 offset 2",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Limit: &query_parser.LimitClause{
					Limit: &query_parser.Int64Value{
						Value: 1,
						Node:  query_parser.Node{Off: 50},
					},
					Node: query_parser.Node{Off: 44},
				},
				ResultsOffset: &query_parser.ResultsOffsetClause{
					ResultsOffset: &query_parser.Int64Value{
						Value: 2,
						Node:  query_parser.Node{Off: 59},
					},
					Node: query_parser.Node{Off: 52},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo.x, bar.y from Customer",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "foo",
										Node:  query_parser.Node{Off: 7},
									},
									query_parser.Segment{
										Value: "x",
										Node:  query_parser.Node{Off: 11},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "bar",
										Node:  query_parser.Node{Off: 14},
									},
									query_parser.Segment{
										Value: "y",
										Node:  query_parser.Node{Off: 18},
									},
								},
								Node: query_parser.Node{Off: 14},
							},
							Node: query_parser.Node{Off: 14},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 25},
					},
					Node: query_parser.Node{Off: 20},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select select from from where where equal 42",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "select",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "from",
						Node: query_parser.Node{Off: 19},
					},
					Node: query_parser.Node{Off: 14},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "where",
										Node:  query_parser.Node{Off: 30},
									},
								},
								Node: query_parser.Node{Off: 30},
							},
							Node: query_parser.Node{Off: 30},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 36},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypInt,
							Int:  42,
							Node: query_parser.Node{Off: 42},
						},
						Node: query_parser.Node{Off: 30},
					},
					Node: query_parser.Node{Off: 24},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.Value equal true",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 29},
									},
									query_parser.Segment{
										Value: "Value",
										Node:  query_parser.Node{Off: 31},
									},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 37},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypBool,
							Bool: true,
							Node: query_parser.Node{Off: 43},
						},
						Node: query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.ZipCode is nil",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 29},
									},
									query_parser.Segment{
										Value: "ZipCode",
										Node:  query_parser.Node{Off: 31},
									},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Is,
							Node: query_parser.Node{Off: 39},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypNil,
							Node: query_parser.Node{Off: 42},
						},
						Node: query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.ZipCode is not nil",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 29},
									},
									query_parser.Segment{
										Value: "ZipCode",
										Node:  query_parser.Node{Off: 31},
									},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.IsNot,
							Node: query_parser.Node{Off: 39},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypNil,
							Node: query_parser.Node{Off: 46},
						},
						Node: query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.Value = false",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 29},
									},
									query_parser.Segment{
										Value: "Value",
										Node:  query_parser.Node{Off: 31},
									},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 37},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypBool,
							Bool: false,
							Node: query_parser.Node{Off: 39},
						},
						Node: query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.Value equal -42",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 29},
									},
									query_parser.Segment{
										Value: "Value",
										Node:  query_parser.Node{Off: 31},
									},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 37},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypInt,
							Int:  -42,
							Node: query_parser.Node{Off: 43},
						},
						Node: query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.Value equal -18.888",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 29},
									},
									query_parser.Segment{
										Value: "Value",
										Node:  query_parser.Node{Off: 31},
									},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 37},
						},
						Operand2: &query_parser.Operand{
							Type:  query_parser.TypFloat,
							Float: -18.888,
							Node:  query_parser.Node{Off: 43},
						},
						Node: query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select x from y where b = 'c'",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "x",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "y",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "b",
										Node:  query_parser.Node{Off: 22},
									},
								},
								Node: query_parser.Node{Off: 22},
							},
							Node: query_parser.Node{Off: 22},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 24},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypInt,
							Int:  'c',
							Node: query_parser.Node{Off: 26},
						},
						Node: query_parser.Node{Off: 22},
					},
					Node: query_parser.Node{Off: 16},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select x from y where b = 'c' limit 10 offset 20",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "x",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "y",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "b",
										Node:  query_parser.Node{Off: 22},
									},
								},
								Node: query_parser.Node{Off: 22},
							},
							Node: query_parser.Node{Off: 22},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 24},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypInt,
							Int:  'c',
							Node: query_parser.Node{Off: 26},
						},
						Node: query_parser.Node{Off: 22},
					},
					Node: query_parser.Node{Off: 16},
				},
				Limit: &query_parser.LimitClause{
					Limit: &query_parser.Int64Value{
						Value: 10,
						Node:  query_parser.Node{Off: 36},
					},
					Node: query_parser.Node{Off: 30},
				},
				ResultsOffset: &query_parser.ResultsOffsetClause{
					ResultsOffset: &query_parser.Int64Value{
						Value: 20,
						Node:  query_parser.Node{Off: 46},
					},
					Node: query_parser.Node{Off: 39},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select x from y where b = 'c' limit 10",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "x",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "y",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "b",
										Node:  query_parser.Node{Off: 22},
									},
								},
								Node: query_parser.Node{Off: 22},
							},
							Node: query_parser.Node{Off: 22},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 24},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypInt,
							Int:  'c',
							Node: query_parser.Node{Off: 26},
						},
						Node: query_parser.Node{Off: 22},
					},
					Node: query_parser.Node{Off: 16},
				},
				Limit: &query_parser.LimitClause{
					Limit: &query_parser.Int64Value{
						Value: 10,
						Node:  query_parser.Node{Off: 36},
					},
					Node: query_parser.Node{Off: 30},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select x from y where b = 'c' offset 10",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "x",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "y",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "b",
										Node:  query_parser.Node{Off: 22},
									},
								},
								Node: query_parser.Node{Off: 22},
							},
							Node: query_parser.Node{Off: 22},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 24},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypInt,
							Int:  'c',
							Node: query_parser.Node{Off: 26},
						},
						Node: query_parser.Node{Off: 22},
					},
					Node: query_parser.Node{Off: 16},
				},
				ResultsOffset: &query_parser.ResultsOffsetClause{
					ResultsOffset: &query_parser.Int64Value{
						Value: 10,
						Node:  query_parser.Node{Off: 37},
					},
					Node: query_parser.Node{Off: 30},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo.bar, tom.dick.harry from Customer where a.b.c = \"baz\" and d.e.f like \"%foobarbaz\"",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "foo",
										Node:  query_parser.Node{Off: 7},
									},
									query_parser.Segment{
										Value: "bar",
										Node:  query_parser.Node{Off: 11},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "tom",
										Node:  query_parser.Node{Off: 16},
									},
									query_parser.Segment{
										Value: "dick",
										Node:  query_parser.Node{Off: 20},
									},
									query_parser.Segment{
										Value: "harry",
										Node:  query_parser.Node{Off: 25},
									},
								},
								Node: query_parser.Node{Off: 16},
							},
							Node: query_parser.Node{Off: 16},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 36},
					},
					Node: query_parser.Node{Off: 31},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "a",
												Node:  query_parser.Node{Off: 51},
											},
											query_parser.Segment{
												Value: "b",
												Node:  query_parser.Node{Off: 53},
											},
											query_parser.Segment{
												Value: "c",
												Node:  query_parser.Node{Off: 55},
											},
										},
										Node: query_parser.Node{Off: 51},
									},
									Node: query_parser.Node{Off: 51},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 57},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypStr,
									Str:  "baz",
									Node: query_parser.Node{Off: 59},
								},
								Node: query_parser.Node{Off: 51},
							},
							Node: query_parser.Node{Off: 51},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 65},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "d",
												Node:  query_parser.Node{Off: 69},
											},
											query_parser.Segment{
												Value: "e",
												Node:  query_parser.Node{Off: 71},
											},
											query_parser.Segment{
												Value: "f",
												Node:  query_parser.Node{Off: 73},
											},
										},
										Node: query_parser.Node{Off: 69},
									},
									Node: query_parser.Node{Off: 69},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Like,
									Node: query_parser.Node{Off: 75},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypStr,
									Str:  "%foobarbaz",
									Node: query_parser.Node{Off: 80},
								},
								Node: query_parser.Node{Off: 69},
							},
							Node: query_parser.Node{Off: 69},
						},
						Node: query_parser.Node{Off: 51},
					},
					Node: query_parser.Node{Off: 45},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo, bar from Customer where CustRecord.CustID=123 or CustRecord.Name like \"f%\"",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "foo",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "bar",
										Node:  query_parser.Node{Off: 12},
									},
								},
								Node: query_parser.Node{Off: 12},
							},
							Node: query_parser.Node{Off: 12},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 21},
					},
					Node: query_parser.Node{Off: 16},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "CustRecord",
												Node:  query_parser.Node{Off: 36},
											},
											query_parser.Segment{
												Value: "CustID",
												Node:  query_parser.Node{Off: 47},
											},
										},
										Node: query_parser.Node{Off: 36},
									},
									Node: query_parser.Node{Off: 36},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 53},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypInt,
									Int:  123,
									Node: query_parser.Node{Off: 54},
								},
								Node: query_parser.Node{Off: 36},
							},
							Node: query_parser.Node{Off: 36},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Or,
							Node: query_parser.Node{Off: 58},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "CustRecord",
												Node:  query_parser.Node{Off: 61},
											},
											query_parser.Segment{
												Value: "Name",
												Node:  query_parser.Node{Off: 72},
											},
										},
										Node: query_parser.Node{Off: 61},
									},
									Node: query_parser.Node{Off: 61},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Like,
									Node: query_parser.Node{Off: 77},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypStr,
									Str:  "f%",
									Node: query_parser.Node{Off: 82},
								},
								Node: query_parser.Node{Off: 61},
							},
							Node: query_parser.Node{Off: 61},
						},
						Node: query_parser.Node{Off: 36},
					},
					Node: query_parser.Node{Off: 30},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo from Customer where A=123 or B=456 and C=789",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "foo",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 16},
					},
					Node: query_parser.Node{Off: 11},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "A",
														Node:  query_parser.Node{Off: 31},
													},
												},
												Node: query_parser.Node{Off: 31},
											},
											Node: query_parser.Node{Off: 31},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 32},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  123,
											Node: query_parser.Node{Off: 33},
										},
										Node: query_parser.Node{Off: 31},
									},
									Node: query_parser.Node{Off: 31},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Or,
									Node: query_parser.Node{Off: 37},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "B",
														Node:  query_parser.Node{Off: 40},
													},
												},
												Node: query_parser.Node{Off: 40},
											},
											Node: query_parser.Node{Off: 40},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 41},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  456,
											Node: query_parser.Node{Off: 42},
										},
										Node: query_parser.Node{Off: 40},
									},
									Node: query_parser.Node{Off: 40},
								},
								Node: query_parser.Node{Off: 31},
							},
							Node: query_parser.Node{Off: 31},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 46},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "C",
												Node:  query_parser.Node{Off: 50},
											},
										},
										Node: query_parser.Node{Off: 50},
									},
									Node: query_parser.Node{Off: 50},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 51},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypInt,
									Int:  789,
									Node: query_parser.Node{Off: 52},
								},
								Node: query_parser.Node{Off: 50},
							},
							Node: query_parser.Node{Off: 50},
						},
						Node: query_parser.Node{Off: 31},
					},
					Node: query_parser.Node{Off: 25},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo from Customer where (A=123 or B=456) and C=789",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "foo",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 16},
					},
					Node: query_parser.Node{Off: 11},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "A",
														Node:  query_parser.Node{Off: 32},
													},
												},
												Node: query_parser.Node{Off: 32},
											},
											Node: query_parser.Node{Off: 32},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 33},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  123,
											Node: query_parser.Node{Off: 34},
										},
										Node: query_parser.Node{Off: 32},
									},
									Node: query_parser.Node{Off: 32},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Or,
									Node: query_parser.Node{Off: 38},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "B",
														Node:  query_parser.Node{Off: 41},
													},
												},
												Node: query_parser.Node{Off: 41},
											},
											Node: query_parser.Node{Off: 41},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 42},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  456,
											Node: query_parser.Node{Off: 43},
										},
										Node: query_parser.Node{Off: 41},
									},
									Node: query_parser.Node{Off: 41},
								},
								Node: query_parser.Node{Off: 32},
							},
							Node: query_parser.Node{Off: 32},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 48},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "C",
												Node:  query_parser.Node{Off: 52},
											},
										},
										Node: query_parser.Node{Off: 52},
									},
									Node: query_parser.Node{Off: 52},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 53},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypInt,
									Int:  789,
									Node: query_parser.Node{Off: 54},
								},
								Node: query_parser.Node{Off: 52},
							},
							Node: query_parser.Node{Off: 52},
						},
						Node: query_parser.Node{Off: 32},
					},
					Node: query_parser.Node{Off: 25},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo from Customer where (A<=123 or B>456) and C>=789",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "foo",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 16},
					},
					Node: query_parser.Node{Off: 11},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "A",
														Node:  query_parser.Node{Off: 32},
													},
												},
												Node: query_parser.Node{Off: 32},
											},
											Node: query_parser.Node{Off: 32},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.LessThanOrEqual,
											Node: query_parser.Node{Off: 33},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  123,
											Node: query_parser.Node{Off: 35},
										},
										Node: query_parser.Node{Off: 32},
									},
									Node: query_parser.Node{Off: 32},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Or,
									Node: query_parser.Node{Off: 39},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "B",
														Node:  query_parser.Node{Off: 42},
													},
												},
												Node: query_parser.Node{Off: 42},
											},
											Node: query_parser.Node{Off: 42},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.GreaterThan,
											Node: query_parser.Node{Off: 43},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  456,
											Node: query_parser.Node{Off: 44},
										},
										Node: query_parser.Node{Off: 42},
									},
									Node: query_parser.Node{Off: 42},
								},
								Node: query_parser.Node{Off: 32},
							},
							Node: query_parser.Node{Off: 32},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 49},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "C",
												Node:  query_parser.Node{Off: 53},
											},
										},
										Node: query_parser.Node{Off: 53},
									},
									Node: query_parser.Node{Off: 53},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.GreaterThanOrEqual,
									Node: query_parser.Node{Off: 54},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypInt,
									Int:  789,
									Node: query_parser.Node{Off: 56},
								},
								Node: query_parser.Node{Off: 53},
							},
							Node: query_parser.Node{Off: 53},
						},
						Node: query_parser.Node{Off: 32},
					},
					Node: query_parser.Node{Off: 25},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo from Customer where A=123 or (B=456 and C=789)",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "foo",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 16},
					},
					Node: query_parser.Node{Off: 11},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "A",
												Node:  query_parser.Node{Off: 31},
											},
										},
										Node: query_parser.Node{Off: 31},
									},
									Node: query_parser.Node{Off: 31},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 32},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypInt,
									Int:  123,
									Node: query_parser.Node{Off: 33},
								},
								Node: query_parser.Node{Off: 31},
							},
							Node: query_parser.Node{Off: 31},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Or,
							Node: query_parser.Node{Off: 37},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "B",
														Node:  query_parser.Node{Off: 41},
													},
												},
												Node: query_parser.Node{Off: 41},
											},
											Node: query_parser.Node{Off: 41},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 42},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  456,
											Node: query_parser.Node{Off: 43},
										},
										Node: query_parser.Node{Off: 41},
									},
									Node: query_parser.Node{Off: 41},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.And,
									Node: query_parser.Node{Off: 47},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "C",
														Node:  query_parser.Node{Off: 51},
													},
												},
												Node: query_parser.Node{Off: 51},
											},
											Node: query_parser.Node{Off: 51},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 52},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  789,
											Node: query_parser.Node{Off: 53},
										},
										Node: query_parser.Node{Off: 51},
									},
									Node: query_parser.Node{Off: 51},
								},
								Node: query_parser.Node{Off: 41},
							},
							Node: query_parser.Node{Off: 41},
						},
						Node: query_parser.Node{Off: 31},
					},
					Node: query_parser.Node{Off: 25},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo from Customer where (A=123) or ((B=456) and (C=789))",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "foo",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 16},
					},
					Node: query_parser.Node{Off: 11},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "A",
												Node:  query_parser.Node{Off: 32},
											},
										},
										Node: query_parser.Node{Off: 32},
									},
									Node: query_parser.Node{Off: 32},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 33},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypInt,
									Int:  123,
									Node: query_parser.Node{Off: 34},
								},
								Node: query_parser.Node{Off: 32},
							},
							Node: query_parser.Node{Off: 32},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Or,
							Node: query_parser.Node{Off: 39},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "B",
														Node:  query_parser.Node{Off: 44},
													},
												},
												Node: query_parser.Node{Off: 44},
											},
											Node: query_parser.Node{Off: 44},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 45},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  456,
											Node: query_parser.Node{Off: 46},
										},
										Node: query_parser.Node{Off: 44},
									},
									Node: query_parser.Node{Off: 44},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.And,
									Node: query_parser.Node{Off: 51},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "C",
														Node:  query_parser.Node{Off: 56},
													},
												},
												Node: query_parser.Node{Off: 56},
											},
											Node: query_parser.Node{Off: 56},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 57},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  789,
											Node: query_parser.Node{Off: 58},
										},
										Node: query_parser.Node{Off: 56},
									},
									Node: query_parser.Node{Off: 56},
								},
								Node: query_parser.Node{Off: 44},
							},
							Node: query_parser.Node{Off: 44},
						},
						Node: query_parser.Node{Off: 32},
					},
					Node: query_parser.Node{Off: 25},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo from Customer where A<>123 or B not equal 456 and C not like \"abc%\"",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "foo",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 16},
					},
					Node: query_parser.Node{Off: 11},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "A",
														Node:  query_parser.Node{Off: 31},
													},
												},
												Node: query_parser.Node{Off: 31},
											},
											Node: query_parser.Node{Off: 31},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.NotEqual,
											Node: query_parser.Node{Off: 32},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  123,
											Node: query_parser.Node{Off: 34},
										},
										Node: query_parser.Node{Off: 31},
									},
									Node: query_parser.Node{Off: 31},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Or,
									Node: query_parser.Node{Off: 38},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "B",
														Node:  query_parser.Node{Off: 41},
													},
												},
												Node: query_parser.Node{Off: 41},
											},
											Node: query_parser.Node{Off: 41},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.NotEqual,
											Node: query_parser.Node{Off: 43},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  456,
											Node: query_parser.Node{Off: 53},
										},
										Node: query_parser.Node{Off: 41},
									},
									Node: query_parser.Node{Off: 41},
								},
								Node: query_parser.Node{Off: 31},
							},
							Node: query_parser.Node{Off: 31},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 57},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "C",
												Node:  query_parser.Node{Off: 61},
											},
										},
										Node: query_parser.Node{Off: 61},
									},
									Node: query_parser.Node{Off: 61},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.NotLike,
									Node: query_parser.Node{Off: 63},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypStr,
									Str:  "abc%",
									Node: query_parser.Node{Off: 72},
								},
								Node: query_parser.Node{Off: 61},
							},
							Node: query_parser.Node{Off: 61},
						},
						Node: query_parser.Node{Off: 31},
					},
					Node: query_parser.Node{Off: 25},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select k, v from Customer where k = \"\\\"",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "k",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 10},
									},
								},
								Node: query_parser.Node{Off: 10},
							},
							Node: query_parser.Node{Off: 10},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 17},
					},
					Node: query_parser.Node{Off: 12},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "k",
										Node:  query_parser.Node{Off: 32},
									},
								},
								Node: query_parser.Node{Off: 32},
							},
							Node: query_parser.Node{Off: 32},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 34},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypStr,
							Str:  "\\",
							Node: query_parser.Node{Off: 36},
						},
						Node: query_parser.Node{Off: 32},
					},
					Node: query_parser.Node{Off: 26},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where Now() < Time(\"2015/07/22\") and Foo(10,20.1,v.Bar) = true",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.ColumnEntry{
						query_parser.ColumnEntry{
							Column: query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Now",
										Args: nil,
										Node: query_parser.Node{Off: 29},
									},
									Node: query_parser.Node{Off: 29},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.LessThan,
									Node: query_parser.Node{Off: 35},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Time",
										Args: []*query_parser.Operand{
											&query_parser.Operand{
												Type: query_parser.TypStr,
												Str:  "2015/07/22",
												Node: query_parser.Node{Off: 42},
											},
										},
										Node: query_parser.Node{Off: 37},
									},
									Node: query_parser.Node{Off: 37},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Node: query_parser.Node{Off: 29},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 56},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Foo",
										Args: []*query_parser.Operand{
											&query_parser.Operand{
												Type: query_parser.TypInt,
												Int:  10,
												Node: query_parser.Node{Off: 64},
											},
											&query_parser.Operand{
												Type:  query_parser.TypFloat,
												Float: 20.1,
												Node:  query_parser.Node{Off: 67},
											},
											&query_parser.Operand{
												Type: query_parser.TypField,
												Column: &query_parser.Field{
													Segments: []query_parser.Segment{
														query_parser.Segment{
															Value: "v",
															Node:  query_parser.Node{Off: 72},
														},
														query_parser.Segment{
															Value: "Bar",
															Node:  query_parser.Node{Off: 74},
														},
													},
													Node: query_parser.Node{Off: 72},
												},
												Node: query_parser.Node{Off: 72},
											},
										},
										Node: query_parser.Node{Off: 60},
									},
									Node: query_parser.Node{Off: 60},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 79},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypBool,
									Bool: true,
									Node: query_parser.Node{Off: 81},
								},
								Node: query_parser.Node{Off: 60},
							},
							Node: query_parser.Node{Off: 60},
						},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
	}

	for _, test := range basic {
		st, err := query_parser.Parse(&db, test.query)
		if err != nil {
			t.Errorf("query: %s; unexpected error: got %v, want nil", test.query, err)
		}
		switch (*st).(type) {
		case query_parser.SelectStatement:
			if !reflect.DeepEqual(test.statement, *st) {
				t.Errorf("query: %s;\nGOT  %s\nWANT %s", test.query, *st, test.statement)
			}
		}
	}
}

func TestQueryParserErrors(t *testing.T) {
	basic := []parseSelectErrorTest{
		{"", syncql.NewErrNoStatementFound(db.GetContext(), 0)},
		{";", syncql.NewErrExpectedIdentifier(db.GetContext(), 0, ";")},
		{"foo", syncql.NewErrUnknownIdentifier(db.GetContext(), 0, "foo")},
		{"(foo)", syncql.NewErrExpectedIdentifier(db.GetContext(), 0, "(")},
		{"select foo.", syncql.NewErrExpectedIdentifier(db.GetContext(), 11, "")},
		{"select foo. from a", syncql.NewErrExpectedFrom(db.GetContext(), 17, "a")},
		{"select (foo)", syncql.NewErrExpectedIdentifier(db.GetContext(), 7, "(")},
		{"select from where", syncql.NewErrExpectedFrom(db.GetContext(), 12, "where")},
		{"create table Customer (CustRecord cust_pkg.Cust, primary key(CustRecord.CustID))", syncql.NewErrUnknownIdentifier(db.GetContext(), 0, "create")},
		{"select foo from Customer where (A=123 or B=456) and C=789)", syncql.NewErrUnexpected(db.GetContext(), 57, ")")},
		{"select foo from Customer where ((A=123 or B=456) and C=789", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 58)},
		{"select foo from Customer where (((((A=123 or B=456 and C=789))))", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 64)},
		{"select foo from Customer where (A=123 or B=456) and C=789)))))", syncql.NewErrUnexpected(db.GetContext(), 57, ")")},
		{"select foo from Customer where", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 30)},
		{"select foo from Customer where ", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 31)},
		{"select foo from Customer where )", syncql.NewErrExpectedOperand(db.GetContext(), 31, ")")},
		{"select foo from Customer where )A=123 or B=456) and C=789", syncql.NewErrExpectedOperand(db.GetContext(), 31, ")")},
		{"select foo from Customer where ()A=123 or B=456) and C=789", syncql.NewErrExpectedOperand(db.GetContext(), 32, ")")},
		{"select foo from Customer where (A=123 or B=456) and C=789)", syncql.NewErrUnexpected(db.GetContext(), 57, ")")},
		{"select foo bar from Customer", syncql.NewErrExpectedFrom(db.GetContext(), 11, "bar")},
		{"select foo from Customer Invoice", syncql.NewErrUnexpected(db.GetContext(), 25, "Invoice")},
		{"select (foo) from (Customer)", syncql.NewErrExpectedIdentifier(db.GetContext(), 7, "(")},
		{"select foo, bar from Customer where a = (b)", syncql.NewErrExpectedOperand(db.GetContext(), 40, "(")},
		{"select foo, bar from Customer where a = b and (c) = d", syncql.NewErrExpectedOperator(db.GetContext(), 48, ")")},
		{"select foo, bar from Customer where a = b and c =", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 49)},
		{"select foo, bar from Customer where a = ", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 40)},
		{"select foo, bar from Customer where a", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 37)},
		{"select", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 6)},
		{"select a from", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 13)},
		{"select a from b where c = d and e =", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 35)},
		{"select a from b where c = d and f", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 33)},
		{"select a from b where c = d and f *", syncql.NewErrExpectedOperator(db.GetContext(), 34, "*")},
		{"select a from b where c <", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 25)},
		{"select a from b where c not", syncql.NewErrExpected(db.GetContext(), 27, "'equal' or 'like'")},
		{"select a from b where c not 8", syncql.NewErrExpected(db.GetContext(), 28, "'equal' or 'like'")},
		{"select x from y where a and b = c", syncql.NewErrExpectedOperator(db.GetContext(), 24, "and")},
		{"select v from Customer limit 100 offset a", syncql.NewErrExpected(db.GetContext(), 40, "positive integer literal")},
		{"select v from Customer limit -100 offset 5", syncql.NewErrExpected(db.GetContext(), 29, "positive integer literal")},
		{"select v from Customer limit 100 offset -5", syncql.NewErrExpected(db.GetContext(), 40, "positive integer literal")},
		{"select v from Customer limit a offset 200", syncql.NewErrExpected(db.GetContext(), 29, "positive integer literal")},
		{"select v from Customer limit", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 28)},
		{"select v from Customer, Invoice", syncql.NewErrUnexpected(db.GetContext(), 22, ",")},
		{"select v from Customer As Cust where foo = bar", syncql.NewErrUnexpected(db.GetContext(), 23, "As")},
		{"select 1abc from Customer where foo = bar", syncql.NewErrExpectedIdentifier(db.GetContext(), 7, "1")},
		{"select v from Customer where Foo(1,) = true", syncql.NewErrExpectedOperand(db.GetContext(), 35, ")")},
		{"select v from Customer where Foo(,1) = true", syncql.NewErrExpectedOperand(db.GetContext(), 33, ",")},
		{"select v from Customer where Foo(1, 2.0 = true", syncql.NewErrUnexpected(db.GetContext(), 40, "=")},
		{"select v from Customer where Foo(1, 2.0 limit 100", syncql.NewErrUnexpected(db.GetContext(), 40, "limit")},
		{"select v from Customer where v is", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 33)},
		{"select v from Customer where v = 1.0 is k = \"abc\"", syncql.NewErrUnexpected(db.GetContext(), 37, "is")},
		{"select v as from Customer", syncql.NewErrExpectedFrom(db.GetContext(), 17, "Customer")},
	}

	for _, test := range basic {
		_, err := query_parser.Parse(&db, test.query)
		// Test both that the IDs compare and the text compares (since the offset needs to match).
		if verror.ErrorID(err) != verror.ErrorID(test.err) || err.Error() != test.err.Error() {
			t.Errorf("query: %s; got %v, want %v", test.query, err, test.err)
		}
	}
}
