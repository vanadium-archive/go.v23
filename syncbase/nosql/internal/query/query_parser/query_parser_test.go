// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package query_parser_test

import (
	"reflect"
	"testing"

	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_parser"
)

type parseSelectTest struct {
	query     string
	statement query_parser.SelectStatement
	err       *query_parser.SyntaxError
}

type parseSelectErrorTest struct {
	query string
	err   *query_parser.SyntaxError
}

func TestQueryParser(t *testing.T) {
	basic := []parseSelectTest{
		{
			"select v from Customer",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "v",
									Node:  query_parser.Node{Off: 7},
								},
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
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"   select v from Customer",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "v",
									Node:  query_parser.Node{Off: 10},
								},
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
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "v",
									Node:  query_parser.Node{Off: 7},
								},
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
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "v",
									Node:  query_parser.Node{Off: 7},
								},
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
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "v",
									Node:  query_parser.Node{Off: 7},
								},
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
					Columns: []query_parser.Field{
						query_parser.Field{
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
						query_parser.Field{
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
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "select",
									Node:  query_parser.Node{Off: 7},
								},
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
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "v",
									Node:  query_parser.Node{Off: 7},
								},
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
			"select v from Customer where v.Value = false",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "v",
									Node:  query_parser.Node{Off: 7},
								},
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
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "v",
									Node:  query_parser.Node{Off: 7},
								},
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
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "v",
									Node:  query_parser.Node{Off: 7},
								},
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
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "x",
									Node:  query_parser.Node{Off: 7},
								},
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
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "x",
									Node:  query_parser.Node{Off: 7},
								},
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
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "x",
									Node:  query_parser.Node{Off: 7},
								},
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
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "x",
									Node:  query_parser.Node{Off: 7},
								},
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
					Columns: []query_parser.Field{
						query_parser.Field{
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
						query_parser.Field{
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
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "foo",
									Node:  query_parser.Node{Off: 7},
								},
							},
							Node: query_parser.Node{Off: 7},
						},
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "bar",
									Node:  query_parser.Node{Off: 12},
								},
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
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "foo",
									Node:  query_parser.Node{Off: 7},
								},
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
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "foo",
									Node:  query_parser.Node{Off: 7},
								},
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
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "foo",
									Node:  query_parser.Node{Off: 7},
								},
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
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "foo",
									Node:  query_parser.Node{Off: 7},
								},
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
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "foo",
									Node:  query_parser.Node{Off: 7},
								},
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
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "foo",
									Node:  query_parser.Node{Off: 7},
								},
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
					Columns: []query_parser.Field{
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "k",
									Node:  query_parser.Node{Off: 7},
								},
							},
							Node: query_parser.Node{Off: 7},
						},
						query_parser.Field{
							Segments: []query_parser.Segment{
								query_parser.Segment{
									Value: "v",
									Node:  query_parser.Node{Off: 10},
								},
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
	}

	for _, test := range basic {
		st, err := query_parser.Parse(test.query)
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
		{"", query_parser.Error(0, "No statement found.")},
		{";", query_parser.Error(0, "Expected identifier, found ';'.")},
		{"foo", query_parser.Error(0, "Unknown identifier: foo")},
		{"(foo)", query_parser.Error(0, "Expected identifier, found '('.")},
		{"select foo.", query_parser.Error(11, "Expected identifier, found ''.")},
		{"select foo. from a", query_parser.Error(17, "Expected 'from', found 'a'")},
		{"select (foo)", query_parser.Error(7, "Expected identifier, found '('.")},
		{"select from where", query_parser.Error(12, "Expected 'from', found 'where'")},
		{"create table Customer (CustRecord cust_pkg.Cust, primary key(CustRecord.CustID))", query_parser.Error(0, "Unknown identifier: create")},
		{"select foo from Customer where (A=123 or B=456) and C=789)", query_parser.Error(57, "Unexpected: ')'.")},
		{"select foo from Customer where ((A=123 or B=456) and C=789", query_parser.Error(58, "Unexpected end of statement.")},
		{"select foo from Customer where (((((A=123 or B=456 and C=789))))", query_parser.Error(64, "Unexpected end of statement.")},
		{"select foo from Customer where (A=123 or B=456) and C=789)))))", query_parser.Error(57, "Unexpected: ')'.")},
		{"select foo from Customer where", query_parser.Error(30, "Unexpected end of statement.")},
		{"select foo from Customer where ", query_parser.Error(31, "Unexpected end of statement.")},
		{"select foo from Customer where )", query_parser.Error(31, "Expected operand, found ')'.")},
		{"select foo from Customer where )A=123 or B=456) and C=789", query_parser.Error(31, "Expected operand, found ')'.")},
		{"select foo from Customer where ()A=123 or B=456) and C=789", query_parser.Error(32, "Expected operand, found ')'.")},
		{"select foo from Customer where (A=123 or B=456) and C=789)", query_parser.Error(57, "Unexpected: ')'.")},
		{"select foo bar from Customer", query_parser.Error(11, "Expected 'from', found 'bar'")},
		{"select foo from Customer Invoice", query_parser.Error(25, "Unexpected: 'Invoice'.")},
		{"select (foo) from (Customer)", query_parser.Error(7, "Expected identifier, found '('.")},
		{"select foo, bar from Customer where a = (b)", query_parser.Error(40, "Expected operand, found '('.")},
		{"select foo, bar from Customer where a = b and (c) = d", query_parser.Error(48, "Expected operator ('like', 'not like', '=', '<>', 'equal' or 'not equal', found ')'.")},
		{"select foo, bar from Customer where a = b and c =", query_parser.Error(49, "Unexpected end of statement, expected operand.")},
		{"select foo, bar from Customer where a = ", query_parser.Error(40, "Unexpected end of statement, expected operand.")},
		{"select foo, bar from Customer where a", query_parser.Error(37, "Unexpected end of statement, expected operator.")},
		{"select", query_parser.Error(6, "Unexpected end of statement.")},
		{"select a from", query_parser.Error(13, "Unexpected end of statement.")},
		{"select a from b where c = d and e =", query_parser.Error(35, "Unexpected end of statement, expected operand.")},
		{"select a from b where c = d and f", query_parser.Error(33, "Unexpected end of statement, expected operator.")},
		{"select a from b where c = d and f *", query_parser.Error(34, "Expected operator ('like', 'not like', '=', '<>', 'equal' or 'not equal', found '*'.")},
		{"select a from b where c <", query_parser.Error(25, "Unexpected end of statement, expected operand.")},
		{"select a from b where c not", query_parser.Error(27, "Expected 'equal' or 'like'")},
		{"select a from b where c not 8", query_parser.Error(28, "Expected 'equal' or 'like'")},
		{"select x from y where a and b = c", query_parser.Error(24, "Expected operator ('like', 'not like', '=', '<>', '<', '<=', '>', '>=', 'equal' or 'not equal', found 'and'.")},
		{"select v from Customer limit 100 offset a", query_parser.Error(40, "Expected positive integer literal., found 'a'.")},
		{"select v from Customer limit -100 offset 5", query_parser.Error(29, "Expected positive integer literal., found '-'.")},
		{"select v from Customer limit 100 offset -5", query_parser.Error(40, "Expected positive integer literal., found '-'.")},
		{"select v from Customer limit a offset 200", query_parser.Error(29, "Expected positive integer literal., found 'a'.")},
		{"select v from Customer limit", query_parser.Error(28, "Unexpected end of statement, expected integer literal.")},
		{"select v from Customer, Invoice", query_parser.Error(22, "Unexpected: ','.")},
		{"select v from Customer As Cust where foo = bar", query_parser.Error(23, "Unexpected: 'As'.")},
		{"select 1abc from Customer where foo = bar", query_parser.Error(7, "Expected identifier, found '1'.")},
	}

	for _, test := range basic {
		_, err := query_parser.Parse(test.query)
		if !reflect.DeepEqual(err, test.err) {
			t.Errorf("query: %s; got %v, want %v", test.query, err, test.err)
		}
	}
}
