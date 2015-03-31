// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query_parser_test

import (
	"reflect"
	"strings"
	"testing"
	"v.io/syncbase/v23/syncbase/query_parser"
)

type parseSelectTest struct {
	query      string
	statements []query_parser.SelectStatement
	err        *query_parser.SyntaxError
}

type parseSelectErrorTest struct {
	query string
	err   *query_parser.SyntaxError
}

func binaryOpPtr(o query_parser.BinaryOperator) *query_parser.BinaryOperator {
	return &o
}

func TestQueryParser(t *testing.T) {
	basic := []parseSelectTest{
		{
			"",
			[]query_parser.SelectStatement{},
			nil,
		},
		{
			"   ",
			[]query_parser.SelectStatement{},
			nil,
		},
		{
			";",
			[]query_parser.SelectStatement{},
			nil,
		},
		{
			";;",
			[]query_parser.SelectStatement{},
			nil,
		},
		{
			";;;",
			[]query_parser.SelectStatement{},
			nil,
		},
		{
			"select * from Customer",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"*"}},
					},
					Tables: []query_parser.Table{
						{Name: "Customer"},
					},
				},
			},
			nil,
		},
		{
			"select * from Customer limit 100 offset 200",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"*"}},
					},
					Tables: []query_parser.Table{
						{Name: "Customer"},
					},
					Limit:  100,
					Offset: 200,
				},
			},
			nil,
		},
		{
			"select * from Customer offset 400 limit 10",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"*"}},
					},
					Tables: []query_parser.Table{
						{Name: "Customer"},
					},
					Limit:  10,
					Offset: 400,
				},
			},
			nil,
		},
		{
			"select * from Customer limit 100 offset 200 limit 1 offset 2",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"*"}},
					},
					Tables: []query_parser.Table{
						{Name: "Customer"},
					},
					// last limit and offset wins
					Limit:  1,
					Offset: 2,
				},
			},
			nil,
		},
		{
			"select foo.*, bar.* from Customer",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"foo", "*"}},
						{[]string{"bar", "*"}},
					},
					Tables: []query_parser.Table{
						{Name: "Customer"},
					},
				},
			},
			nil,
		},
		{
			"select * from Customer; select * from Invoice",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"*"}},
					},
					Tables: []query_parser.Table{
						{Name: "Customer"},
					},
				},
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"*"}},
					},
					Tables: []query_parser.Table{
						{Name: "Invoice"},
					},
				},
			},
			nil,
		},
		{
			"select * from Customer; select * from Invoice;",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"*"}},
					},
					Tables: []query_parser.Table{
						{Name: "Customer"},
					},
				},
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"*"}},
					},
					Tables: []query_parser.Table{
						{Name: "Invoice"},
					},
				},
			},
			nil,
		},
		{
			";select * from Customer; select * from Invoice;",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"*"}},
					},
					Tables: []query_parser.Table{
						{Name: "Customer"},
					},
				},
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"*"}},
					},
					Tables: []query_parser.Table{
						{Name: "Invoice"},
					},
				},
			},
			nil,
		},
		{
			"select select from from where where equal 42",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"select"}},
					},
					Tables: []query_parser.Table{
						{Name: "from"},
					},
					Where: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type:   query_parser.OpField,
							Column: &query_parser.Field{[]string{"where"}},
						},
						Operator: binaryOpPtr(query_parser.Equal),
						Operand2: &query_parser.Operand{
							Type: query_parser.OpInt,
							Int:  42,
						},
					},
				},
			},
			nil,
		},
		{
			"select x from y where b = 'c'",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"x"}},
					},
					Tables: []query_parser.Table{
						{Name: "y"},
					},
					Where: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type:   query_parser.OpField,
							Column: &query_parser.Field{[]string{"b"}},
						},
						Operator: binaryOpPtr(query_parser.Equal),
						Operand2: &query_parser.Operand{
							Type: query_parser.OpChar,
							Char: 'c',
						},
					},
				},
			},
			nil,
		},
		{
			"select x from y where b = 'c' limit 10 offset 20",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"x"}},
					},
					Tables: []query_parser.Table{
						{Name: "y"},
					},
					Where: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type:   query_parser.OpField,
							Column: &query_parser.Field{[]string{"b"}},
						},
						Operator: binaryOpPtr(query_parser.Equal),
						Operand2: &query_parser.Operand{
							Type: query_parser.OpChar,
							Char: 'c',
						},
					},
					Limit:  10,
					Offset: 20,
				},
			},
			nil,
		},
		{
			"select x from y where b = 'c' limit 10",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"x"}},
					},
					Tables: []query_parser.Table{
						{Name: "y"},
					},
					Where: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type:   query_parser.OpField,
							Column: &query_parser.Field{[]string{"b"}},
						},
						Operator: binaryOpPtr(query_parser.Equal),
						Operand2: &query_parser.Operand{
							Type: query_parser.OpChar,
							Char: 'c',
						},
					},
					Limit: 10,
				},
			},
			nil,
		},
		{
			"select x from y where b = 'c' offset 10",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"x"}},
					},
					Tables: []query_parser.Table{
						{Name: "y"},
					},
					Where: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type:   query_parser.OpField,
							Column: &query_parser.Field{[]string{"b"}},
						},
						Operator: binaryOpPtr(query_parser.Equal),
						Operand2: &query_parser.Operand{
							Type: query_parser.OpChar,
							Char: 'c',
						},
					},
					Offset: 10,
				},
			},
			nil,
		},
		{
			"select foo.bar, tom.dick.harry from Customer, Invoice as Inv where a.b.c = \"baz\" and d.e.f like \"%foobarbaz\"",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"foo", "bar"}},
						{[]string{"tom", "dick", "harry"}},
					},
					Tables: []query_parser.Table{
						{Name: "Customer"},
						{Name: "Invoice", As: "Inv"},
					},
					Where: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.OpExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type:   query_parser.OpField,
									Column: &query_parser.Field{[]string{"a", "b", "c"}},
								},
								Operator: binaryOpPtr(query_parser.Equal),
								Operand2: &query_parser.Operand{
									Type:    query_parser.OpLiteral,
									Literal: "baz",
								},
							},
						},
						Operator: binaryOpPtr(query_parser.And),
						Operand2: &query_parser.Operand{
							Type: query_parser.OpExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type:   query_parser.OpField,
									Column: &query_parser.Field{[]string{"d", "e", "f"}},
								},
								Operator: binaryOpPtr(query_parser.Like),
								Operand2: &query_parser.Operand{
									Type:    query_parser.OpLiteral,
									Literal: "%foobarbaz",
								},
							},
						},
					},
				},
			},
			nil,
		},
		{
			"select foo, bar from Customer where CustRecord.CustID=123 or CustRecord.Name like \"f%\"",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"foo"}},
						{[]string{"bar"}},
					},
					Tables: []query_parser.Table{
						{Name: "Customer"},
					},
					Where: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.OpExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type:   query_parser.OpField,
									Column: &query_parser.Field{[]string{"CustRecord", "CustID"}},
								},
								Operator: binaryOpPtr(query_parser.Equal),
								Operand2: &query_parser.Operand{
									Type: query_parser.OpInt,
									Int:  123,
								},
							},
						},
						Operator: binaryOpPtr(query_parser.Or),
						Operand2: &query_parser.Operand{
							Type: query_parser.OpExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type:   query_parser.OpField,
									Column: &query_parser.Field{[]string{"CustRecord", "Name"}},
								},
								Operator: binaryOpPtr(query_parser.Like),
								Operand2: &query_parser.Operand{
									Type:    query_parser.OpLiteral,
									Literal: "f%",
								},
							},
						},
					},
				},
			},
			nil,
		},
		{
			"select foo from Customer where A=123 or B=456 and C=789",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"foo"}},
					},
					Tables: []query_parser.Table{
						{Name: "Customer"},
					},
					Where: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.OpExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.OpExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type:   query_parser.OpField,
											Column: &query_parser.Field{[]string{"A"}},
										},
										Operator: binaryOpPtr(query_parser.Equal),
										Operand2: &query_parser.Operand{
											Type: query_parser.OpInt,
											Int:  123,
										},
									},
								},
								Operator: binaryOpPtr(query_parser.Or),
								Operand2: &query_parser.Operand{
									Type: query_parser.OpExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type:   query_parser.OpField,
											Column: &query_parser.Field{[]string{"B"}},
										},
										Operator: binaryOpPtr(query_parser.Equal),
										Operand2: &query_parser.Operand{
											Type: query_parser.OpInt,
											Int:  456,
										},
									},
								},
							},
						},
						Operator: binaryOpPtr(query_parser.And),
						Operand2: &query_parser.Operand{
							Type: query_parser.OpExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type:   query_parser.OpField,
									Column: &query_parser.Field{[]string{"C"}},
								},
								Operator: binaryOpPtr(query_parser.Equal),
								Operand2: &query_parser.Operand{
									Type: query_parser.OpInt,
									Int:  789,
								},
							},
						},
					},
				},
			},
			nil,
		},
		{
			"select foo from Customer where A<>123 or B not equal 456 and C not like \"abc%\"",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"foo"}},
					},
					Tables: []query_parser.Table{
						{Name: "Customer"},
					},
					Where: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.OpExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.OpExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type:   query_parser.OpField,
											Column: &query_parser.Field{[]string{"A"}},
										},
										Operator: binaryOpPtr(query_parser.NotEqual),
										Operand2: &query_parser.Operand{
											Type: query_parser.OpInt,
											Int:  123,
										},
									},
								},
								Operator: binaryOpPtr(query_parser.Or),
								Operand2: &query_parser.Operand{
									Type: query_parser.OpExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type:   query_parser.OpField,
											Column: &query_parser.Field{[]string{"B"}},
										},
										Operator: binaryOpPtr(query_parser.NotEqual),
										Operand2: &query_parser.Operand{
											Type: query_parser.OpInt,
											Int:  456,
										},
									},
								},
							},
						},
						Operator: binaryOpPtr(query_parser.And),
						Operand2: &query_parser.Operand{
							Type: query_parser.OpExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type:   query_parser.OpField,
									Column: &query_parser.Field{[]string{"C"}},
								},
								Operator: binaryOpPtr(query_parser.NotLike),
								Operand2: &query_parser.Operand{
									Type:    query_parser.OpLiteral,
									Literal: "abc%",
								},
							},
						},
					},
				},
			},
			nil,
		},
		{
			"select foo from Customer where (A=123 or B=456) and C=789",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"foo"}},
					},
					Tables: []query_parser.Table{
						{Name: "Customer"},
					},
					Where: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.OpExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.OpExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type:   query_parser.OpField,
											Column: &query_parser.Field{[]string{"A"}},
										},
										Operator: binaryOpPtr(query_parser.Equal),
										Operand2: &query_parser.Operand{
											Type: query_parser.OpInt,
											Int:  123,
										},
									},
								},
								Operator: binaryOpPtr(query_parser.Or),
								Operand2: &query_parser.Operand{
									Type: query_parser.OpExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type:   query_parser.OpField,
											Column: &query_parser.Field{[]string{"B"}},
										},
										Operator: binaryOpPtr(query_parser.Equal),
										Operand2: &query_parser.Operand{
											Type: query_parser.OpInt,
											Int:  456,
										},
									},
								},
							},
						},
						Operator: binaryOpPtr(query_parser.And),
						Operand2: &query_parser.Operand{
							Type: query_parser.OpExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type:   query_parser.OpField,
									Column: &query_parser.Field{[]string{"C"}},
								},
								Operator: binaryOpPtr(query_parser.Equal),
								Operand2: &query_parser.Operand{
									Type: query_parser.OpInt,
									Int:  789,
								},
							},
						},
					},
				},
			},
			nil,
		},
		{
			"select foo from Customer where A=123 or (B=456 and C=789)",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"foo"}},
					},
					Tables: []query_parser.Table{
						{Name: "Customer"},
					},
					Where: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.OpExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type:   query_parser.OpField,
									Column: &query_parser.Field{[]string{"A"}},
								},
								Operator: binaryOpPtr(query_parser.Equal),
								Operand2: &query_parser.Operand{
									Type: query_parser.OpInt,
									Int:  123,
								},
							},
						},
						Operator: binaryOpPtr(query_parser.Or),
						Operand2: &query_parser.Operand{
							Type: query_parser.OpExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.OpExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type:   query_parser.OpField,
											Column: &query_parser.Field{[]string{"B"}},
										},
										Operator: binaryOpPtr(query_parser.Equal),
										Operand2: &query_parser.Operand{
											Type: query_parser.OpInt,
											Int:  456,
										},
									},
								},
								Operator: binaryOpPtr(query_parser.And),
								Operand2: &query_parser.Operand{
									Type: query_parser.OpExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type:   query_parser.OpField,
											Column: &query_parser.Field{[]string{"C"}},
										},
										Operator: binaryOpPtr(query_parser.Equal),
										Operand2: &query_parser.Operand{
											Type: query_parser.OpInt,
											Int:  789,
										},
									},
								},
							},
						},
					},
				},
			},
			nil,
		},
		{
			"select foo from Customer where (A=123) or ((B=456) and (C=789))",
			[]query_parser.SelectStatement{
				query_parser.SelectStatement{
					Columns: []query_parser.Field{
						{[]string{"foo"}},
					},
					Tables: []query_parser.Table{
						{Name: "Customer"},
					},
					Where: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.OpExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type:   query_parser.OpField,
									Column: &query_parser.Field{[]string{"A"}},
								},
								Operator: binaryOpPtr(query_parser.Equal),
								Operand2: &query_parser.Operand{
									Type: query_parser.OpInt,
									Int:  123,
								},
							},
						},
						Operator: binaryOpPtr(query_parser.Or),
						Operand2: &query_parser.Operand{
							Type: query_parser.OpExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.OpExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type:   query_parser.OpField,
											Column: &query_parser.Field{[]string{"B"}},
										},
										Operator: binaryOpPtr(query_parser.Equal),
										Operand2: &query_parser.Operand{
											Type: query_parser.OpInt,
											Int:  456,
										},
									},
								},
								Operator: binaryOpPtr(query_parser.And),
								Operand2: &query_parser.Operand{
									Type: query_parser.OpExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type:   query_parser.OpField,
											Column: &query_parser.Field{[]string{"C"}},
										},
										Operator: binaryOpPtr(query_parser.Equal),
										Operand2: &query_parser.Operand{
											Type: query_parser.OpInt,
											Int:  789,
										},
									},
								},
							},
						},
					},
				},
			},
			nil,
		},
	}

	for _, test := range basic {
		statements, err := query_parser.Parse(strings.NewReader(test.query))
		if err != nil {
			t.Errorf("query: %s; unexpected error: got %v, want nil", test.query, err)
		}
		if err == nil && len(test.statements) != len(statements) {
			t.Errorf("query: %s; got %v, want: %v", test.query, statements, test.statements)
		}
		for i := range statements {
			switch s := (*statements[i]).(type) {
			case query_parser.SelectStatement:
				if !reflect.DeepEqual(test.statements[i], s) {
					t.Errorf("query: %s;\nGOT  %s\nWANT %s", test.query, *statements[i], test.statements[i])
				}
			}
		}
	}
}

func TestQueryParserErrors(t *testing.T) {
	basic := []parseSelectErrorTest{
		{"foo", query_parser.Error(0, "Unknown identifier: foo")},
		{"(foo)", query_parser.Error(0, "Expected identifier, found '('")},
		{"select *.a from b", query_parser.Error(8, "No segments may follow an asterisk in a field.")},
		{"select a.*.c.", query_parser.Error(10, "No segments may follow an asterisk in a field.")},
		{"select foo.", query_parser.Error(11, "Expected identifier or '*', found ''")},
		{"select foo. from a", query_parser.Error(17, "Expected 'from', found 'a'")},
		{"select (foo)", query_parser.Error(7, "Expected identifier or '*', found '('")},
		{"select from where", query_parser.Error(12, "Expected 'from', found 'where'")},
		{"create table Customer (CustRecord cust_pkg.Cust, primary key(CustRecord.CustID))", query_parser.Error(0, "Unknown identifier: create")},
		{"select foo from Customer where (A=123 or B=456) and C=789)", query_parser.Error(57, "Unexpected: ')'")},
		{"select foo from Customer where ((A=123 or B=456) and C=789", query_parser.Error(58, "Unexpected end of statement.")},
		{"select foo from Customer where (((((A=123 or B=456 and C=789))))", query_parser.Error(64, "Unexpected end of statement.")},
		{"select foo from Customer where (A=123 or B=456) and C=789)))))", query_parser.Error(57, "Unexpected: ')'")},
		{"select foo from Customer where", query_parser.Error(30, "Unexpected end of statement.")},
		{"select foo from Customer where ", query_parser.Error(31, "Unexpected end of statement.")},
		{"select foo from Customer where )", query_parser.Error(31, "Expected operand, found ')'.")},
		{"select foo from Customer where )A=123 or B=456) and C=789", query_parser.Error(31, "Expected operand, found ')'.")},
		{"select foo from Customer where ()A=123 or B=456) and C=789", query_parser.Error(32, "Expected operand, found ')'.")},
		{"select foo from Customer where (A=123 or B=456) and C=789)", query_parser.Error(57, "Unexpected: ')'")},
		{"select foo bar from Customer", query_parser.Error(11, "Expected 'from', found 'bar'")},
		{"select foo from Customer Invoice", query_parser.Error(25, "Unexpected: 'Invoice'")},
		{"select (foo) from (Customer)", query_parser.Error(7, "Expected identifier or '*', found '('")},
		{"select foo, bar from Customer where a = (b)", query_parser.Error(40, "Expected operand, found '('.")},
		{"select foo, bar from Customer where a = b and (c) = d", query_parser.Error(48, "Expected operator ('like', 'not like', '=', '<>', 'equal' or 'not equal', found ')'.")},
		{"select foo, bar from Customer where a = b and c =", query_parser.Error(49, "Unexpected end of statement, expected operator.")},
		{"select foo, bar from Customer where a = ", query_parser.Error(40, "Unexpected end of statement, expected operator.")},
		{"select foo, bar from Customer where a", query_parser.Error(37, "Unexpected end of statement, expected operator.")},
		{"select", query_parser.Error(6, "Unexpected end of statement.")},
		{"select a from", query_parser.Error(13, "Unexpected end of statement.")},
		{"select a from b where c = d and e =", query_parser.Error(35, "Unexpected end of statement, expected operator.")},
		{"select a from b where c = d and f", query_parser.Error(33, "Unexpected end of statement, expected operator.")},
		{"select a from b where c = d and f *", query_parser.Error(34, "Expected operator ('like', 'not like', '=', '<>', 'equal' or 'not equal', found '*'.")},
		{"select a from b where c < 8", query_parser.Error(26, "Expected '>'")},
		{"select a from b where c <", query_parser.Error(25, "Expected '>'")},
		{"select a from b where c not", query_parser.Error(27, "Expected 'equal' or 'like'")},
		{"select a from b where c not 8", query_parser.Error(28, "Expected 'equal' or 'like'")},
		{"select x from y where a and b = c", query_parser.Error(24, "Expected operator ('like', 'not like', '=', '<>', 'equal' or 'not equal', found 'and'.")},
		{"select * from Customer limit 100 offset a", query_parser.Error(40, "Expected integer literal., found 'a'.")},
		{"select * from Customer limit a offset 200", query_parser.Error(29, "Expected integer literal., found 'a'.")},
		{"select * from Customer limit", query_parser.Error(28, "Unexpected end of statement, expected integer literal.")},
	}

	for _, test := range basic {
		_, err := query_parser.Parse(strings.NewReader(test.query))
		if !reflect.DeepEqual(err, test.err) {
			t.Errorf("query: %s; got %v, want %v", test.query, err, test.err)
		}
	}
}
