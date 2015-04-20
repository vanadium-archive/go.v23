// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"v.io/syncbase/v23/syncbase/query"
	"v.io/syncbase/v23/syncbase/query_checker"
	"v.io/syncbase/v23/syncbase/query_parser"
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

var store MockStore

func TestCreate(t *testing.T) {
	store.tables = &[]string{"Customer", "Invoice"}
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
			"select v from Customer where type = \"Foo.Bar\"",
			nil,
			nil,
		},
		{
			"select k, v from Customer where type = \"Foo.Bar\" and k like \"abc%\" limit 100 offset 200",
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
			"select k, v from Customer where type = \"Foo.Bar\" and k like \"abc%\"",
			[]string{"abc"},
			nil,
		},
		{
			// Need all keys (single prefix of "").
			"select k, v from Customer where type = \"Foo.Bar\" or k like \"abc%\"",
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
			"select k, v from Customer where type = \"Foo.Bar\" and k like \"foo_bar\"",
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
			"select k, v from Customer where k = \"xyz\" or type = \"foo.Bar\"",
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
