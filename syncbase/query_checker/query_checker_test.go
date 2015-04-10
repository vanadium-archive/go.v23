// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query_checker_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
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

type checkSelectTest struct {
	query string
	err   *query_checker.SemanticError
}

type parseSelectErrorTest struct {
	query string
	err   *query_checker.SemanticError
}

func TestQueryChecker(t *testing.T) {
	basic := []checkSelectTest{
		{
			"select k, v from Customer",
			nil,
		},
		{
			"select k, v.name from Customer",
			nil,
		},
		{
			"select k, v.name from Customer limit 200",
			nil,
		},
		{
			"select k, v.name from Customer offset 100",
			nil,
		},
		{
			"select k, v.name from Customer where k = \"foo\"",
			nil,
		},
		{
			"select v from Customer where type = \"Foo.Bar\"",
			nil,
		},
		{
			"select v from Customer where type = \"Foo.Bar\" and k >= \"100\" and k < \"200\" and v.foo > 50 and v.bar <= 1000 and v.baz <> -20.7",
			nil,
		},
		{
			"select k, v from Customer where type = \"Foo.Bar\" and k like \"abc%\" limit 100 offset 200",
			nil,
		},
	}

	for _, test := range basic {
		s, syntaxErr := query_parser.Parse(test.query)
		if syntaxErr != nil {
			t.Errorf("query: %s; unexpected error: got %v, want nil", test.query, syntaxErr)
		}
		err := query_checker.Check(store, s)
		if err != nil {
			t.Errorf("query: %s; got %v, want: nil", test.query, err)
		}
	}
}

func TestQueryParserErrors(t *testing.T) {
	basic := []parseSelectErrorTest{
		{"select a from Customer", query_checker.Error(7, "Select field must be 'k' or 'v[{.<ident>}...]'.")},
		{"select v from Bob", query_checker.Error(14, "No such table: Bob")},
		{"select k.a from Customer", query_checker.Error(9, "Dot notation may not be used on a key (string) field.")},
		{"select k from Customer where type.a = \"Foo.Bar\"", query_checker.Error(34, "Dot notation may not be used with type.")},
		{"select v from Customer where a=1", query_checker.Error(29, "Where field must be 'k', 'v[{.<ident>}...]' or 'type'.")},
		{"select v.*.a from Customer", query_checker.Error(11, "'*' is only valid as the ultimate segment of a field.")},
		{"select v from Customer limit 0", query_checker.Error(29, "Limit must be > 0.")},
		{"select v.* from Customer where type = 2", query_checker.Error(31, "Type expressions must be 'type = <string-literal>'.")},
		{"select v.* from Customer where type <> \"foo\"", query_checker.Error(31, "Type expressions must be 'type = <string-literal>'.")},
		{"select v.* from Customer where type < \"foo\"", query_checker.Error(31, "Type expressions must be 'type = <string-literal>'.")},
		{"select v.* from Customer where type <= \"foo\"", query_checker.Error(31, "Type expressions must be 'type = <string-literal>'.")},
		{"select v.* from Customer where type > \"foo\"", query_checker.Error(31, "Type expressions must be 'type = <string-literal>'.")},
		{"select v.* from Customer where type >= \"foo\"", query_checker.Error(31, "Type expressions must be 'type = <string-literal>'.")},
		{"select v.* from Customer where \"foo\" = type", query_checker.Error(31, "Type expressions must be 'type = <string-literal>'.")},
		{"select v.* from Customer where v.x like v.y", query_checker.Error(31, "Like expressions require right operand of type <string-literal>.")},
		{"select v.* from Customer where k = v.y", query_checker.Error(31, "Key (i.e., 'k') expressions must be of form 'k <op> <string-literal>'.")},
		{"select v.* from Customer where k <> v.y", query_checker.Error(31, "Key (i.e., 'k') expressions must be of form 'k <op> <string-literal>'.")},
		{"select v.* from Customer where k < v.y", query_checker.Error(31, "Key (i.e., 'k') expressions must be of form 'k <op> <string-literal>'.")},
		{"select v.* from Customer where k <= v.y", query_checker.Error(31, "Key (i.e., 'k') expressions must be of form 'k <op> <string-literal>'.")},
		{"select v.* from Customer where k > v.y", query_checker.Error(31, "Key (i.e., 'k') expressions must be of form 'k <op> <string-literal>'.")},
		{"select v.* from Customer where k >= v.y", query_checker.Error(31, "Key (i.e., 'k') expressions must be of form 'k <op> <string-literal>'.")},
		{"select v.* from Customer where \"abc%\" = k", query_checker.Error(31, "Key (i.e., 'k') expressions must be of form 'k <op> <string-literal>'.")},
	}

	for _, test := range basic {
		s, syntaxErr := query_parser.Parse(test.query)
		if syntaxErr != nil {
			t.Errorf("query: %s; unexpected error: got %v, want nil", test.query, syntaxErr)
		}
		err := query_checker.Check(store, s)
		if !reflect.DeepEqual(err, test.err) {
			t.Errorf("query: %s; got %v, want %v", test.query, err, test.err)
		}
	}
}
