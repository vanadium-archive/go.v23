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
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_db"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_parser"
	"v.io/syncbase/v23/syncbase/nosql/syncql"
	"v.io/v23"
	"v.io/v23/context"
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

type customerTable struct {
}

type invoiceTable struct {
}

func init() {
	var shutdown v23.Shutdown
	db.ctx, shutdown = test.InitForTest()
	defer shutdown()
}

func (t invoiceTable) Scan(prefixes []string) (query_db.KeyValueStream, error) {
	return nil, errors.New("unimplemented")
}

func (t customerTable) Scan(prefixes []string) (query_db.KeyValueStream, error) {
	return nil, errors.New("unimplemented")
}

func (db *mockDB) GetTable(table string) (query_db.Table, error) {
	if table == "Customer" {
		var t customerTable
		return t, nil
	} else if table == "Invoice" {
		var t invoiceTable
		return t, nil
	}
	return nil, errors.New(fmt.Sprintf("No such table: %s", table))
}

var db mockDB

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
	err   error
}

func TestQueryChecker(t *testing.T) {
	basic := []checkSelectTest{
		{"select k, v from Customer"},
		{"select k, v.name from Customer"},
		{"select k, v.name from Customer limit 200"},
		{"select k, v.name from Customer offset 100"},
		{"select k, v.name from Customer where k = \"foo\""},
		{"select v from Customer where t = \"Foo.Bar\""},
		{"select k, v from Customer where t = \"Foo.Bar\" and k like \"abc%\" limit 100 offset 200"},
		{"select v from Customer where v.A = true"},
		{"select v from Customer where v.A <> true"},
		{"select v from Customer where false = v.A"},
		{"select v from Customer where false = false"},
		{"select v from Customer where true = true"},
		{"select v from Customer where false = true"},
		{"select v from Customer where true = false"},
		{"select v from Customer where false <> true"},
		{"select v from Customer where v.ZipCode is nil"},
		{"select v from Customer where v.ZipCode Is Nil"},
		{"select v from Customer where v.ZipCode is not nil"},
		{"select v from Customer where v.ZipCode IS NOT NIL"},
	}

	for _, test := range basic {
		s, syntaxErr := query_parser.Parse(&db, test.query)
		if syntaxErr != nil {
			t.Errorf("query: %s; unexpected error: got %v, want nil", test.query, syntaxErr)
		}
		err := query_checker.Check(&db, s)
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
			"select k, v from Customer where t = \"Foo.Bar\" and k like \"abc%\" limit 100 offset 200",
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
		s, syntaxErr := query_parser.Parse(&db, test.query)
		if syntaxErr != nil {
			t.Errorf("query: %s; unexpected error: got %v, want nil", test.query, syntaxErr)
		}
		err := query_checker.Check(&db, s)
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
		s, syntaxErr := query_parser.Parse(&db, test.query)
		if syntaxErr != nil {
			t.Errorf("query: %s; unexpected error: got %v, want nil", test.query, syntaxErr)
		}
		err := query_checker.Check(&db, s)
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
		{"select a from Customer", syncql.NewErrInvalidSelectField(db.GetContext(), 7)},
		{"select v from Bob", syncql.NewErrTableCantAccess(db.GetContext(), 14, "Bob", errors.New("No such table: Bob"))},
		{"select k.a from Customer", syncql.NewErrDotNotationDisallowedForKey(db.GetContext(), 9)},
		{"select k from Customer where t.a = \"Foo.Bar\"", syncql.NewErrDotNotationDisallowedForType(db.GetContext(), 31)},
		{"select v from Customer where a=1", syncql.NewErrBadFieldInWhere(db.GetContext(), 29)},
		{"select v from Customer limit 0", syncql.NewErrLimitMustBeGe0(db.GetContext(), 29)},
		{"select v.z from Customer where t = 2", syncql.NewErrTypeExpressionForm(db.GetContext(), 31)},
		{"select v.z from Customer where t <> \"foo\"", syncql.NewErrTypeExpressionForm(db.GetContext(), 31)},
		{"select v.z from Customer where t < \"foo\"", syncql.NewErrTypeExpressionForm(db.GetContext(), 31)},
		{"select v.z from Customer where t <= \"foo\"", syncql.NewErrTypeExpressionForm(db.GetContext(), 31)},
		{"select v.z from Customer where t > \"foo\"", syncql.NewErrTypeExpressionForm(db.GetContext(), 31)},
		{"select v.z from Customer where t >= \"foo\"", syncql.NewErrTypeExpressionForm(db.GetContext(), 31)},
		{"select v.z from Customer where \"foo\" = t", syncql.NewErrTypeExpressionForm(db.GetContext(), 31)},
		{"select v.z from Customer where v.x like v.y", syncql.NewErrLikeExpressionsRequireRhsString(db.GetContext(), 31)},
		{"select v.z from Customer where k = v.y", syncql.NewErrKeyExpressionForm(db.GetContext(), 31)},
		{"select v.z from Customer where k <> v.y", syncql.NewErrKeyExpressionForm(db.GetContext(), 31)},
		{"select v.z from Customer where k < v.y", syncql.NewErrKeyExpressionForm(db.GetContext(), 31)},
		{"select v.z from Customer where k <= v.y", syncql.NewErrKeyExpressionForm(db.GetContext(), 31)},
		{"select v.z from Customer where k > v.y", syncql.NewErrKeyExpressionForm(db.GetContext(), 31)},
		{"select v.z from Customer where k >= v.y", syncql.NewErrKeyExpressionForm(db.GetContext(), 31)},
		{"select v.z from Customer where \"abc%\" = k", syncql.NewErrKeyExpressionForm(db.GetContext(), 31)},
		{"select v from Customer where t = \"Foo.Bar\" and k >= \"100\" and k < \"200\" and v.foo > 50 and v.bar <= 1000 and v.baz <> -20.7", syncql.NewErrKeyExpressionForm(db.GetContext(), 47)},
		{"select v.z from Customer where k like \"a\\bc%\"", syncql.NewErrInvalidEscapedChar(db.GetContext(), 38)},
		{"select v from Customer where v.A > false", syncql.NewErrBoolInvalidExpression(db.GetContext(), 33)},
		{"select v from Customer where true <= v.A", syncql.NewErrBoolInvalidExpression(db.GetContext(), 34)},
		{"select v from Customer where Now() < Date(\"2015/07/22\")", syncql.NewErrFunctionsNotYetSupported(db.GetContext(), 29)},
		{"select v from Customer where Foo(\"2015/07/22\", true, 3.14157) = true", syncql.NewErrFunctionsNotYetSupported(db.GetContext(), 29)},
		{"select v from Customer where t is nil", syncql.NewErrTypeExpressionForm(db.GetContext(), 29)},
		{"select v from Customer where k is nil", syncql.NewErrKeyExpressionForm(db.GetContext(), 29)},
		{"select v from Customer where nil is v.ZipCode", syncql.NewErrIsIsNotRequireLhsValue(db.GetContext(), 29)},
		{"select v from Customer where v.ZipCode is \"94303\"", syncql.NewErrIsIsNotRequireRhsNil(db.GetContext(), 42)},
		{"select v from Customer where v.ZipCode is 94303", syncql.NewErrIsIsNotRequireRhsNil(db.GetContext(), 42)},
		{"select v from Customer where v.ZipCode is true", syncql.NewErrIsIsNotRequireRhsNil(db.GetContext(), 42)},
		{"select v from Customer where v.ZipCode is 943.03", syncql.NewErrIsIsNotRequireRhsNil(db.GetContext(), 42)},
		{"select v from Customer where t is not nil", syncql.NewErrTypeExpressionForm(db.GetContext(), 29)},
		{"select v from Customer where k is not nil", syncql.NewErrKeyExpressionForm(db.GetContext(), 29)},
		{"select v from Customer where nil is not v.ZipCode", syncql.NewErrIsIsNotRequireLhsValue(db.GetContext(), 29)},
		{"select v from Customer where v.ZipCode is not \"94303\"", syncql.NewErrIsIsNotRequireRhsNil(db.GetContext(), 46)},
		{"select v from Customer where v.ZipCode is not 94303", syncql.NewErrIsIsNotRequireRhsNil(db.GetContext(), 46)},
		{"select v from Customer where v.ZipCode is not true", syncql.NewErrIsIsNotRequireRhsNil(db.GetContext(), 46)},
		{"select v from Customer where v.ZipCode is not 943.03", syncql.NewErrIsIsNotRequireRhsNil(db.GetContext(), 46)},
	}

	for _, test := range basic {
		s, syntaxErr := query_parser.Parse(&db, test.query)
		if syntaxErr != nil {
			t.Errorf("query: %s; unexpected error: got %v, want nil", test.query, syntaxErr)
		} else {
			err := query_checker.Check(&db, s)
			// Test both that the IDs compare and the text compares (since the offset needs to match).
			if verror.ErrorID(err) != verror.ErrorID(test.err) || err.Error() != test.err.Error() {
				t.Errorf("query: %s; got %v, want %v", test.query, err, test.err)
			}
		}
	}
}
