// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package query_checker_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

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

var store MockStore

func TestCreate(t *testing.T) {
	store.tables = &[]string{"Customer", "Invoice"}
}

type checkSelectTest struct {
	query string
}

type keyPrefixesTest struct {
	query       string
	keyPrefixes []string
}

type regularExpressionsTest struct {
	query      string
	regex      string
	matches    []string
	nonMatches []string
}

type parseSelectErrorTest struct {
	query string
	err   *query_checker.SemanticError
}

func TestQueryChecker(t *testing.T) {
	basic := []checkSelectTest{
		{"select k, v from Customer"},
		{"select k, v.name from Customer"},
		{"select k, v.name from Customer limit 200"},
		{"select k, v.name from Customer offset 100"},
		{"select k, v.name from Customer where k = \"foo\""},
		{"select v from Customer where type = \"Foo.Bar\""},
		{"select k, v from Customer where type = \"Foo.Bar\" and k like \"abc%\" limit 100 offset 200"},
		{"select v from Customer where v.A = true"},
		{"select v from Customer where v.A <> true"},
		{"select v from Customer where false = v.A"},
		{"select v from Customer where false = false"},
		{"select v from Customer where true = true"},
		{"select v from Customer where false = true"},
		{"select v from Customer where true = false"},
		{"select v from Customer where false <> true"},
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

func TestKeyPrefixes(t *testing.T) {
	basic := []keyPrefixesTest{
		{
			"select k, v from Customer",
			[]string{""},
		},
		{
			"select k, v from Customer where type = \"Foo.Bar\" and k like \"abc%\" limit 100 offset 200",
			[]string{"abc"},
		},
		{
			"select k, v from Customer where k = \"Foo.Bar\" and k like \"abc%\" limit 100 offset 200",
			[]string{"Foo.Bar", "abc"},
		},
		{
			"select k, v from Customer where k = \"Foo.Bar\" or k like \"Foo\" or k like \"abc%\" limit 100 offset 200",
			[]string{"Foo", "abc"},
		},
		{
			"select k, v from Customer where k like \"Foo\\%Bar\" or k like \"abc%\" limit 100 offset 200",
			[]string{"Foo%Bar", "abc"},
		},
		{
			"select k, v from Customer where k like \"Foo\\\\%Bar\" or k like \"abc%\" limit 100 offset 200",
			[]string{"Foo\\", "abc"},
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
		switch sel := (*s).(type) {
		case query_parser.SelectStatement:
			prefixes := query_checker.CompileKeyPrefixes(sel.Where)
			if !reflect.DeepEqual(test.keyPrefixes, prefixes) {
				t.Errorf("query: %s;\nGOT  %v\nWANT %v", test.query, prefixes, test.keyPrefixes)
			}
		default:
			t.Errorf("query: %s;\nGOT  %v\nWANT query_parser.SelectStatement", test.query, *s)
		}
	}
}

func TestRegularExpressions(t *testing.T) {
	basic := []regularExpressionsTest{
		{
			"select v from Customer where v like \"abc%\"",
			"^abc.*?$",
			[]string{"abc", "abcd", "abcabc"},
			[]string{"xabcd"},
		},
		{
			"select v from Customer where v like \"abc_\"",
			"^abc.$",
			[]string{"abcd", "abc1"},
			[]string{"abc", "xabcd", "abcde"},
		},
		{
			"select v from Customer where v like \"abc_efg\"",
			"^abc.efg$",
			[]string{"abcdefg"},
			[]string{"abc", "xabcd", "abcde", "abcdefgh"},
		},
		{
			"select v from Customer where v like \"abc\\\\efg\"",
			"^abc\\\\efg$",
			[]string{"abc\\efg"},
			[]string{"abc\\", "xabc\\efg", "abc\\de", "abc\\defgh"},
		},
		{
			"select v from Customer where v like \"abc%def\"",
			"^abc.*?def$",
			[]string{"abcdefdef", "abcdef", "abcdefghidef"},
			[]string{"abcdefg", "abcdefde"},
		},
		{
			"select v from Customer where v like \"[0-9]*abc%def\"",
			"^\\[0-9\\]\\*abc.*?def$",
			[]string{"[0-9]*abcdefdef", "[0-9]*abcdef", "[0-9]*abcdefghidef"},
			[]string{"0abcdefg", "9abcdefde", "[0-9]abcdefg", "[0-9]abcdefg", "[0-9]abcdefg"},
		},
		{
			"select v from Customer where v like \"[0-9]*a\\\\b\\\\c%def\"",
			"^\\[0-9\\]\\*a\\\\b\\\\c.*?def$",
			[]string{"[0-9]*a\\b\\cdefdef", "[0-9]*a\\b\\cdef", "[0-9]*a\\b\\cdefghidef"},
			[]string{"0a\\b\\cdefg", "9a\\\b\\cdefde", "[0-9]a\\\b\\cdefg", "[0-9]a\\b\\cdefg", "[0-9]a\\b\\cdefg"},
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
		switch sel := (*s).(type) {
		case query_parser.SelectStatement:
			// We know there is exactly one like expression and operand2 contains
			// a regex and compiled regex.
			if sel.Where.Expr.Operand2.Regex != test.regex {
				t.Errorf("query: %s;\nGOT  %s\nWANT %s", test.query, sel.Where.Expr.Operand2.Regex, test.regex)
			}
			regexp := sel.Where.Expr.Operand2.CompRegex
			// Make sure all matches actually match
			for _, m := range test.matches {
				if !regexp.MatchString(m) {
					t.Errorf("query: %s;Expected match: %s; \nGOT  false\nWANT true", test.query, m)
				}
			}
			// Make sure all nonMatches actually don't match
			for _, n := range test.nonMatches {
				if regexp.MatchString(n) {
					t.Errorf("query: %s;Expected nonMatch: %s; \nGOT  true\nWANT false", test.query, n)
				}
			}
		}
	}
}

func TestQueryCheckerErrors(t *testing.T) {
	basic := []parseSelectErrorTest{
		{"select a from Customer", query_checker.Error(7, "Select field must be 'k' or 'v[{.<ident>}...]'.")},
		{"select v from Bob", query_checker.Error(14, "No such table: Bob")},
		{"select k.a from Customer", query_checker.Error(9, "Dot notation may not be used on a key (string) field.")},
		{"select k from Customer where type.a = \"Foo.Bar\"", query_checker.Error(34, "Dot notation may not be used with type.")},
		{"select v from Customer where a=1", query_checker.Error(29, "Where field must be 'k', 'v[{.<ident>}...]' or 'type'.")},
		{"select v from Customer limit 0", query_checker.Error(29, "Limit must be > 0.")},
		{"select v.z from Customer where type = 2", query_checker.Error(31, "Type expressions must be 'type = <string-literal>'.")},
		{"select v.z from Customer where type <> \"foo\"", query_checker.Error(31, "Type expressions must be 'type = <string-literal>'.")},
		{"select v.z from Customer where type < \"foo\"", query_checker.Error(31, "Type expressions must be 'type = <string-literal>'.")},
		{"select v.z from Customer where type <= \"foo\"", query_checker.Error(31, "Type expressions must be 'type = <string-literal>'.")},
		{"select v.z from Customer where type > \"foo\"", query_checker.Error(31, "Type expressions must be 'type = <string-literal>'.")},
		{"select v.z from Customer where type >= \"foo\"", query_checker.Error(31, "Type expressions must be 'type = <string-literal>'.")},
		{"select v.z from Customer where \"foo\" = type", query_checker.Error(31, "Type expressions must be 'type = <string-literal>'.")},
		{"select v.z from Customer where v.x like v.y", query_checker.Error(31, "Like expressions require right operand of type <string-literal>.")},
		{"select v.z from Customer where k = v.y", query_checker.Error(31, "Key (i.e., 'k') expressions must be of form 'k like|= <string-literal>'.")},
		{"select v.z from Customer where k <> v.y", query_checker.Error(31, "Key (i.e., 'k') expressions must be of form 'k like|= <string-literal>'.")},
		{"select v.z from Customer where k < v.y", query_checker.Error(31, "Key (i.e., 'k') expressions must be of form 'k like|= <string-literal>'.")},
		{"select v.z from Customer where k <= v.y", query_checker.Error(31, "Key (i.e., 'k') expressions must be of form 'k like|= <string-literal>'.")},
		{"select v.z from Customer where k > v.y", query_checker.Error(31, "Key (i.e., 'k') expressions must be of form 'k like|= <string-literal>'.")},
		{"select v.z from Customer where k >= v.y", query_checker.Error(31, "Key (i.e., 'k') expressions must be of form 'k like|= <string-literal>'.")},
		{"select v.z from Customer where \"abc%\" = k", query_checker.Error(31, "Key (i.e., 'k') expressions must be of form 'k like|= <string-literal>'.")},
		{"select v from Customer where type = \"Foo.Bar\" and k >= \"100\" and k < \"200\" and v.foo > 50 and v.bar <= 1000 and v.baz <> -20.7", query_checker.Error(50, "Key (i.e., 'k') expressions must be of form 'k like|= <string-literal>'.")},
		{"select v.z from Customer where k like \"a\\bc%\"", query_checker.Error(38, "Expected '\\', '%' or '_' after '\\'.")},
		{"select v from Customer where v.A > false", query_checker.Error(33, "Boolean operands may only be used in equals and not equals expressions.")},
		{"select v from Customer where true <= v.A", query_checker.Error(34, "Boolean operands may only be used in equals and not equals expressions.")},
	}

	for _, test := range basic {
		s, syntaxErr := query_parser.Parse(test.query)
		if syntaxErr != nil {
			t.Errorf("query: %s; unexpected error: got %v, want nil", test.query, syntaxErr)
		} else {
			err := query_checker.Check(store, s)
			if !reflect.DeepEqual(err, test.err) {
				t.Errorf("query: %s; got %v, want %v", test.query, err, test.err)
			}
		}
	}
}
