// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package query_test

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"v.io/syncbase/v23/syncbase/nosql/internal/query"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_checker"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_parser"
)

type MockStore struct {
	tables *[]string
}

func (db MockStore) CheckTable(table string) error {
	for _, db_table := range *db.tables {
		if table == db_table {
			return nil
		}
	}
	return errors.New(fmt.Sprintf("No such table: %s", table))
}

func (db MockStore) GetKeys(prefix string) ([]string, error) {
	var keys []string
	keys = append(keys, prefix)
	for i := 1; i < 10; i++ {
		keys = append(keys, prefix+string(i))
	}
	return keys, nil
}

type Nest2 struct {
	Foo string
	Bar bool
	Baz int64
}

type Nest1 struct {
	FooBarBaz Nest2
}

type Customer struct {
	Name              string
	ID                int64
	Active            bool
	Rating            rune
	Street            string
	City              string
	State             string
	Zip               string
	GratuituousBigInt *big.Int
	GratuituousBigRat *big.Rat
	GratuituousByte   byte
	GratuituousUint16 uint16
	GratuituousUint32 uint32
	GratuituousUint64 uint64
	GratuituousInt16  int16
	GratuituousInt32  int32
	Foo               Nest1
}

func (db MockStore) GetValue(k string) (interface{}, error) {
	if k == "123456" {
		return sampleRow, nil
	} else if k == "123" {
		return sampleRow123, nil
	} else {
		return nil, nil
	}
}

var store MockStore
var sampleRow Customer
var sampleRow123 Customer

func TestCreate(t *testing.T) {
	store.tables = &[]string{"Customer", "Invoice"}
	sampleRow = Customer{"John Smith", 123456, true, 'A', "1 Main St.", "Palo Alto", "CA", "94303", big.NewInt(1234567890), big.NewRat(123, 1), byte(12), uint16(1234), uint32(5678), uint64(999888777666), int16(9876), int32(876543), Nest1{Nest2{"foo", true, 42}}}
	sampleRow123 = Customer{"John Smith", 123, true, 123, "1 Main St.", "Palo Alto", "CA", "94303", big.NewInt(123), big.NewRat(123, 1), byte(123), uint16(123), uint32(123), uint64(123), int16(123), int32(123), Nest1{Nest2{"foo", true, 123}}}
}

type keyPrefixesTest struct {
	query       string
	keyPrefixes []string
	err         *query.QueryError
}

type evalWhereUsingOnlyKeyTest struct {
	query  string
	key    string
	result bool
	err    error
}

type evalTest struct {
	query  string
	k      string
	v      interface{}
	result bool
}

type execSelectTest struct {
	query string
	r     *query.ResultStream
	err   *query.QueryError
}

type parseSelectErrorTest struct {
	query string
	err   *query.QueryError
}

// TODO(jkline): Flesh out this test.
func TestQueryExec(t *testing.T) {
	basic := []execSelectTest{
		{
			"select k, v from Customer",
			nil,
			nil,
		},
		{
			"select k, v.name from Customer",
			nil,
			nil,
		},
		{
			"select k, v.name from Customer limit 200",
			nil,
			nil,
		},
		{
			"select k, v.name from Customer offset 100",
			nil,
			nil,
		},
		{
			"select k, v.name from Customer where k = \"foo\"",
			nil,
			nil,
		},
		{
			"select v from Customer where t = \"Foo.Bar\"",
			nil,
			nil,
		},
		{
			"select k, v from Customer where t = \"Foo.Bar\" and k like \"abc%\" limit 100 offset 200",
			nil,
			nil,
		},
	}

	for _, test := range basic {
		_, err := query.Exec(store, test.query)
		if err != nil {
			t.Errorf("query: %s; got %v, want nil", test.query, err)
		}
	}
}

func TestKeyPrefixes(t *testing.T) {
	basic := []keyPrefixesTest{
		{
			// Need all keys (single prefix of "").
			"select k, v from Customer",
			[]string{""},
			nil,
		},
		{
			// All selected rows will have key prefix of "abc".
			"select k, v from Customer where t = \"Foo.Bar\" and k like \"abc%\"",
			[]string{"abc"},
			nil,
		},
		{
			// Need all keys (single prefix of "").
			"select k, v from Customer where t = \"Foo.Bar\" or k like \"abc%\"",
			[]string{""},
			nil,
		},
		{
			// Need all keys (single prefix of "").
			"select k, v from Customer where k like \"abc%\" or v.zip = \"94303\"",
			[]string{""},
			nil,
		},
		{
			// All selected rows will have key prefix of "foo".
			"select k, v from Customer where t = \"Foo.Bar\" and k like \"foo_bar\"",
			[]string{"foo"},
			nil,
		},
		{
			// All selected rows will have key prefix of "baz" or "foo".
			"select k, v from Customer where k like \"foo_bar\" or k = \"baz\"",
			[]string{"baz", "foo"},
			nil,
		},
		{
			// All selected rows will have key prefix of "fo".
			"select k, v from Customer where k like \"foo_bar\" or k = \"fo\"",
			[]string{"fo"},
			nil,
		},
		{
			// All selected rows will have key prefix of "foo".
			"select k, v from Customer where k like \"foo%bar\"",
			[]string{"foo"},
			nil,
		},
		{
			// All selected rows will have key prefix of "foo\bar".
			"select k, v from Customer where k like \"foo\\\\bar\"",
			[]string{"foo\\bar"},
			nil,
		},
		{
			// All selected rows will have key prefix of "foo%bar".
			"select k, v from Customer where k like \"foo\\%bar\"",
			[]string{"foo%bar"},
			nil,
		},
		{
			// All selected rows will have key prefix of "foo\%bar".
			"select k, v from Customer where k like \"foo\\\\\\%bar\"",
			[]string{"foo\\%bar"},
			nil,
		},
		{
			// Need all keys (single prefix of "").
			"select k, v from Customer where k like \"%foo\"",
			[]string{""},
			nil,
		},
		{
			// Need all keys (single prefix of "").
			"select k, v from Customer where k like \"_foo\"",
			[]string{""},
			nil,
		},
		{
			// All selected rows will have key prefix of "foo_bar".
			"select k, v from Customer where k like \"foo\\_bar\"",
			[]string{"foo_bar"},
			nil,
		},
		{
			// All selected rows will have key prefix of "foobar%".
			"select k, v from Customer where k like \"foobar\\%\"",
			[]string{"foobar%"},
			nil,
		},
		{
			// All selected rows will have key prefix of "foobar_".
			"select k, v from Customer where k like \"foobar\\_\"",
			[]string{"foobar_"},
			nil,
		},
		{
			// All selected rows will have key prefix of "\%_".
			"select k, v from Customer where k like \"\\\\\\%\\_\"",
			[]string{"\\%_"},
			nil,
		},
		{
			// All selected rows will have key prefix of "%_abc\".
			"select k, v from Customer where k = \"%_abc\\\"",
			[]string{"%_abc\\"},
			nil,
		},
	}

	for _, test := range basic {
		s, synErr := query_parser.Parse(test.query)
		if synErr != nil {
			t.Errorf("query: %s; got %v, want nil", test.query, synErr)
		}
		if synErr == nil {
			semErr := query_checker.Check(store, s)
			if semErr != nil {
				t.Errorf("query: %s; got %v, want nil", test.query, semErr)
			}
			if semErr == nil {
				switch sel := (*s).(type) {
				case query_parser.SelectStatement:
					keyPrefixes := query.CompileKeyPrefixes(sel.Where)
					if !reflect.DeepEqual(test.keyPrefixes, keyPrefixes) {
						t.Errorf("query: %s;\nGOT  %v\nWANT %v", test.query, keyPrefixes, test.keyPrefixes)
					}
				default:
					t.Errorf("query: %s; got %v, want query_parser.SelectStatement", reflect.TypeOf(*s))
				}
			}
		}
	}
}

func TestEvalWhereUsingOnlyKey(t *testing.T) {
	basic := []evalWhereUsingOnlyKeyTest{
		{
			// Row will be selected using only the key.
			"select k, v from Customer where k like \"abc%\"",
			"abcdef",
			true,
			nil,
		},
		{
			// Row will be rejected using only the key.
			"select k, v from Customer where k like \"abc\"",
			"abcd",
			false,
			nil,
		},
		{
			// Need value to determine if row should be selected.
			"select k, v from Customer where k = \"abc\" or v.zip = \"94303\"",
			"abcd",
			false,
			errors.New("Value required for answer."),
		},
		{
			// Need value (i.e., its type) to determine if row should be selected.
			"select k, v from Customer where k = \"xyz\" or t = \"foo.Bar\"",
			"wxyz",
			false,
			errors.New("Value required for answer."),
		},
		{
			// Although value is in where clause, it is not needed to reject row.
			"select k, v from Customer where k = \"abcd\" and v.zip = \"94303\"",
			"abcde",
			false,
			nil,
		},
	}

	for _, test := range basic {
		s, synErr := query_parser.Parse(test.query)
		if synErr != nil {
			t.Errorf("query: %s; got %v, want nil", test.query, synErr)
		}
		if synErr == nil {
			semErr := query_checker.Check(store, s)
			if semErr != nil {
				t.Errorf("query: %s; got %v, want nil", test.query, semErr)
			}
			if semErr == nil {
				switch sel := (*s).(type) {
				case query_parser.SelectStatement:
					result, err := query.EvalWhereUsingOnlyKey(&sel, test.key)
					if result != test.result {
						t.Errorf("query: %s; got %v, want %v", test.query, result, test.result)
					}
					if (err == nil && test.err != nil) || (err != nil && test.err == nil) {
						t.Errorf("query: %s; got %v, want %v", test.query, err, test.err)
					}
				default:
					t.Errorf("query: %s; got %v, want query_parser.SelectStatement", reflect.TypeOf(*s))
				}
			}
		}
	}
}

func TestEval(t *testing.T) {
	basic := []evalTest{
		{
			"select k, v from Customer where t = \"v.io/syncbase/v23/syncbase/nosql/internal/query_test.Customer\"",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where t = \"Customer\"",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where v.Name = \"John Smith\"",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where v.Name = v.Name",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where v = v",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where v > v",
			"123456", sampleRow, false,
		},
		{
			"select k, v from Customer where v < v",
			"123456", sampleRow, false,
		},
		{
			"select k, v from Customer where v >= v",
			"123456", sampleRow, false,
		},
		{
			"select k, v from Customer where v <= v",
			"123456", sampleRow, false,
		},
		{
			"select k, v from Customer where v.Rating = 'A'",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where v.Rating <> 'A'",
			"123456", sampleRow, false,
		},
		{
			"select k, v from Customer where v.Rating >= 'B'",
			"123456", sampleRow, false,
		},
		{
			"select k, v from Customer where v.Rating <= 'B'",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where v.Active = true",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where v.Active = false",
			"123456", sampleRow, false,
		},
		{
			"select k, v from Customer where v.GratuituousBigInt > 100",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where v.GratuituousBigInt = 1234567890",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where 9876543210 < v.GratuituousBigInt",
			"123456", sampleRow, false,
		},
		{
			"select k, v from Customer where 12 = v.GratuituousByte",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where 11 < v.GratuituousByte",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where v.GratuituousByte > 10",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where v.GratuituousByte >= 14",
			"123456", sampleRow, false,
		},
		{
			"select k, v from Customer where v.GratuituousByte >= 11.0",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where v.GratuituousUint64 = 999888777666",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where v.GratuituousUint64 < 999888777666",
			"123456", sampleRow, false,
		},
		{
			"select k, v from Customer where v.GratuituousByte < v.GratuituousUint64",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where v.GratuituousBigRat = 123",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where v.GratuituousUint16 = 1234",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where v.GratuituousUint32 = 5678",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where v.GratuituousInt16 = 9876",
			"123456", sampleRow, true,
		},
		{
			"select k, v from Customer where v.GratuituousInt32 = 876543",
			"123456", sampleRow, true,
		},
		{
			// Deeply nested string.
			"select v from Customer where v.Foo.FooBarBaz.Foo = \"foo\"",
			"123456", sampleRow, true,
		},
		{
			// Deeply nested bool.
			"select v from Customer where v.Foo.FooBarBaz.Bar = true",
			"123456", sampleRow, true,
		},
		{
			// Deeply nested int64.
			"select v from Customer where v.Foo.FooBarBaz.Baz = 42",
			"123456", sampleRow, true,
		},
		{
			// Convert int64 to string
			"select v from Customer where v.Foo.FooBarBaz.Baz = \"42\"",
			"123456", sampleRow, true,
		},
		{
			// Convert bool to string
			"select v from Customer where v.Foo.FooBarBaz.Bar = \"true\"",
			"123456", sampleRow, true,
		},
		{
			// Bool can't convert to other types.
			"select v from Customer where v.Foo.FooBarBaz.Bar = 1",
			"123456", sampleRow, false,
		},
		{
			// Test that all numeric types can compare to a big.Rat
			"select v from Customer where v.GratuituousBigRat = v.Foo.FooBarBaz.Baz and v.GratuituousBigRat = v.ID and v.GratuituousBigRat = v.Rating and v.GratuituousBigRat = v.GratuituousBigInt and v.GratuituousBigRat = v.GratuituousByte and v.GratuituousBigRat = v.GratuituousUint16 and v.GratuituousBigRat = v.GratuituousUint32 and v.GratuituousBigRat = v.GratuituousUint64 and v.GratuituousBigRat = v.GratuituousInt16 and v.GratuituousBigRat = v.GratuituousInt32",
			"123", sampleRow123, true,
		},
		{
			// Test that all numeric types can compare to a big.Int
			"select v from Customer where v.GratuituousBigInt = v.Foo.FooBarBaz.Baz and v.GratuituousBigInt = v.ID and v.GratuituousBigRat = v.Rating and v.GratuituousBigInt = v.GratuituousBigInt and v.GratuituousBigInt = v.GratuituousByte and v.GratuituousBigInt = v.GratuituousUint16 and v.GratuituousBigInt = v.GratuituousUint32 and v.GratuituousBigInt = v.GratuituousUint64 and v.GratuituousBigInt = v.GratuituousInt16 and v.GratuituousBigInt = v.GratuituousInt32",
			"123", sampleRow123, true,
		},
		{
			// Test that all numeric types can compare to an int32
			"select v from Customer where v.GratuituousInt32 = v.Foo.FooBarBaz.Baz and v.GratuituousInt32 = v.ID and v.GratuituousBigRat = v.Rating and v.GratuituousInt32 = v.GratuituousInt32 and v.GratuituousInt32 = v.GratuituousByte and v.GratuituousInt32 = v.GratuituousUint16 and v.GratuituousInt32 = v.GratuituousUint32 and v.GratuituousInt32 = v.GratuituousUint64 and v.GratuituousInt32 = v.GratuituousInt16 and v.GratuituousInt32 = v.GratuituousBigInt",
			"123", sampleRow123, true,
		},
		{
			// Test that all numeric types can compare to an int16
			"select v from Customer where v.GratuituousInt16 = v.Foo.FooBarBaz.Baz and v.GratuituousInt16 = v.ID and v.GratuituousBigRat = v.Rating and v.GratuituousInt16 = v.GratuituousInt16 and v.GratuituousInt16 = v.GratuituousByte and v.GratuituousInt16 = v.GratuituousUint16 and v.GratuituousInt16 = v.GratuituousUint32 and v.GratuituousInt16 = v.GratuituousUint64 and v.GratuituousInt16 = v.GratuituousInt32 and v.GratuituousInt16 = v.GratuituousBigInt",
			"123", sampleRow123, true,
		},
		{
			// Test that all numeric types can compare to an uint64
			"select v from Customer where v.GratuituousUint64 = v.Foo.FooBarBaz.Baz and v.GratuituousUint64 = v.ID and v.GratuituousBigRat = v.Rating and v.GratuituousUint64 = v.GratuituousUint64 and v.GratuituousUint64 = v.GratuituousByte and v.GratuituousUint64 = v.GratuituousUint16 and v.GratuituousUint64 = v.GratuituousUint32 and v.GratuituousUint64 = v.GratuituousUint16 and v.GratuituousUint64 = v.GratuituousInt32 and v.GratuituousUint64 = v.GratuituousBigInt",
			"123", sampleRow123, true,
		},
		{
			// Test that all numeric types can compare to an uint32
			"select v from Customer where v.GratuituousUint32 = v.Foo.FooBarBaz.Baz and v.GratuituousUint32 = v.ID and v.GratuituousBigRat = v.Rating and v.GratuituousUint32 = v.GratuituousUint32 and v.GratuituousUint32 = v.GratuituousByte and v.GratuituousUint32 = v.GratuituousUint16 and v.GratuituousUint32 = v.GratuituousUint64 and v.GratuituousUint32 = v.GratuituousUint16 and v.GratuituousUint32 = v.GratuituousInt32 and v.GratuituousUint32 = v.GratuituousBigInt",
			"123", sampleRow123, true,
		},
		{
			// Test that all numeric types can compare to an uint16
			"select v from Customer where v.GratuituousUint16 = v.Foo.FooBarBaz.Baz and v.GratuituousUint16 = v.ID and v.GratuituousBigRat = v.Rating and v.GratuituousUint16 = v.GratuituousUint16 and v.GratuituousUint16 = v.GratuituousByte and v.GratuituousUint32 = v.GratuituousUint16 and v.GratuituousUint16 = v.GratuituousUint64 and v.GratuituousUint16 = v.GratuituousUint16 and v.GratuituousUint16 = v.GratuituousInt32 and v.GratuituousUint16 = v.GratuituousBigInt",
			"123", sampleRow123, true,
		},
	}

	for _, test := range basic {
		s, synErr := query_parser.Parse(test.query)
		if synErr != nil {
			t.Errorf("query: %s; got %v, want nil", test.query, synErr)
		}
		if synErr == nil {
			semErr := query_checker.Check(store, s)
			if semErr != nil {
				t.Errorf("query: %s; got %v, want nil", test.query, semErr)
			}
			if semErr == nil {
				switch sel := (*s).(type) {
				case query_parser.SelectStatement:
					result := query.Eval(test.k, test.v, sel.Where.Expr)
					if result != test.result {
						t.Errorf("query: %s; got %v, want %v", test.query, result, test.result)
					}
				default:
					t.Errorf("query: %s; got %v, want query_parser.SelectStatement", reflect.TypeOf(*s))
				}
			}
		}
	}
}

// TODO(jkline): Flesh out this test.
func TestExecErrors(t *testing.T) {
	basic := []parseSelectErrorTest{
	//{"select a from Customer", query_checker.Error(7, "Select field must be 'k' or 'v[{.<ident>}...]'.")},
	}

	for _, test := range basic {
		_, err := query.Exec(store, test.query)
		if !reflect.DeepEqual(err, test.err) {
			t.Errorf("query: %s; got %v, want %v", test.query, err, test.err)
		}
	}
}
