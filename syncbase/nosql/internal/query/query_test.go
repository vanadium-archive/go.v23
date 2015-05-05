// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package query_test

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"v.io/syncbase/v23/syncbase/nosql/internal/query"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_checker"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_db"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_parser"
)

type mockDB struct {
}

type customerTable struct {
}

type keyValueStreamImpl struct {
	cursor       int
	prefixes     []string
	prefixCursor int
	customerRows []customerKV
}

func (kvs *keyValueStreamImpl) Advance() bool {
	for true {
		kvs.cursor++ // initialized to -1
		if kvs.cursor >= len(kvs.customerRows) {
			return false
		}
		for kvs.prefixCursor < len(kvs.prefixes) {
			// does it match any prefix
			if kvs.prefixes[kvs.prefixCursor] == "" || strings.HasPrefix(kvs.customerRows[kvs.cursor].key, kvs.prefixes[kvs.prefixCursor]) {
				return true
			}
			// Keys and prefixes are both sorted low to high, so we can increment
			// prefixCursor if the prefix is < the key.
			if kvs.prefixes[kvs.prefixCursor] < kvs.customerRows[kvs.cursor].key {
				kvs.prefixCursor++
				if kvs.prefixCursor >= len(kvs.prefixes) {
					return false
				}
			} else {
				break
			}
		}
	}
	return false
}

func (kvs *keyValueStreamImpl) KeyValue() (string, interface{}) {
	return kvs.customerRows[kvs.cursor].key, kvs.customerRows[kvs.cursor].value
}

func (kvs *keyValueStreamImpl) Err() error {
	return nil
}

func (kvs *keyValueStreamImpl) Cancel() {
}

func (customerTable customerTable) Scan(prefixes []string) (query_db.KeyValueStream, error) {
	var keyValueStreamImpl keyValueStreamImpl
	keyValueStreamImpl.cursor = -1
	keyValueStreamImpl.prefixes = prefixes
	keyValueStreamImpl.customerRows = customerRows
	return &keyValueStreamImpl, nil
}

func (db mockDB) GetTable(table string) (query_db.Table, error) {
	if table == "Customer" {
		var customerTable customerTable
		return customerTable, nil
	}
	return nil, errors.New(fmt.Sprintf("No such table: %s.", table))

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

type Invoice struct {
	CustID     int64
	InvoiceNum int64
	Amount     int64
}

var db mockDB
var sampleRow Customer
var sampleRow123 Customer

type customerKV struct {
	key   string
	value interface{}
}

var customerRows []customerKV

func TestCreate(t *testing.T) {
	sampleRow = Customer{"John Smith", 123456, true, 'A', "1 Main St.", "Palo Alto", "CA", "94303", big.NewInt(1234567890), big.NewRat(123, 1), byte(12), uint16(1234), uint32(5678), uint64(999888777666), int16(9876), int32(876543), Nest1{Nest2{"foo", true, 42}}}
	sampleRow123 = Customer{"John Smith", 123, true, 123, "1 Main St.", "Palo Alto", "CA", "94303", big.NewInt(123), big.NewRat(123, 1), byte(123), uint16(123), uint32(123), uint64(123), int16(123), int32(123), Nest1{Nest2{"foo", true, 123}}}
	sampleRow = Customer{"John Smith", 123456, true, 'A', "1 Main St.", "Palo Alto", "CA", "94303", big.NewInt(1234567890), big.NewRat(123, 1), byte(12), uint16(1234), uint32(5678), uint64(999888777666), int16(9876), int32(876543), Nest1{Nest2{"foo", true, 42}}}

	customerRows = []customerKV{
		customerKV{
			"001",
			Customer{"John Smith", 1, true, 'A', "1 Main St.", "Palo Alto", "CA", "94303", big.NewInt(1234567890), big.NewRat(123, 1), byte(12), uint16(1234), uint32(5678), uint64(999888777666), int16(9876), int32(876543), Nest1{Nest2{"foo", true, 42}}},
		},
		customerKV{
			"001001",
			Invoice{1, 1000, 42},
		},
		customerKV{
			"001002",
			Invoice{1, 1003, 7},
		},
		customerKV{
			"001003",
			Invoice{1, 1005, 88},
		},
		customerKV{
			"002",
			Customer{"Bat Masterson", 2, true, 'B', "777 Any St.", "collins", "IA", "50055", big.NewInt(9999), big.NewRat(999999, 1), byte(9), uint16(99), uint32(999), uint64(9999999), int16(9), int32(99), Nest1{Nest2{"bar", false, 84}}},
		},
		customerKV{
			"002001",
			Invoice{2, 1001, 166},
		},
		customerKV{
			"002002",
			Invoice{2, 1002, 243},
		},
		customerKV{
			"002003",
			Invoice{2, 1004, 787},
		},
		customerKV{
			"002004",
			Invoice{2, 1006, 88},
		},
	}
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

type projectionTest struct {
	query  string
	k      string
	v      interface{}
	result []interface{}
}

type execSelectTest struct {
	query string
	r     []interface{}
}

type execSelectSingleRowTest struct {
	query  string
	k      string
	v      interface{}
	result interface{}
}

type execSelectErrorTest struct {
	query string
	err   *query.QueryError
}

func TestQueryExec(t *testing.T) {
	basic := []execSelectTest{
		{
			// Select values for all customer records.
			"select v from Customer where t = \"Customer\"",
			[]interface{}{
				[]interface{}{customerRows[0].value},
				[]interface{}{customerRows[4].value},
			},
		},
		{
			// Select keys & values for all customer records.
			"select k, v from Customer where t = \"Customer\"",
			[]interface{}{
				[]interface{}{customerRows[0].key, customerRows[0].value},
				[]interface{}{customerRows[4].key, customerRows[4].value},
			},
		},
		{
			// Select keys & names for all customer records.
			"select k, v.Name from Customer where t = \"Customer\"",
			[]interface{}{
				[]interface{}{customerRows[0].key, "John Smith"},
				[]interface{}{customerRows[4].key, "Bat Masterson"},
			},
		},
		{
			// Select both customer and invoice records.
			// Customer records have ID.
			// Invoice records have CustID.
			"select v.ID, v.CustID from Customer",
			[]interface{}{
				[]interface{}{int64(1), nil},
				[]interface{}{nil, int64(1)},
				[]interface{}{nil, int64(1)},
				[]interface{}{nil, int64(1)},
				[]interface{}{int64(2), nil},
				[]interface{}{nil, int64(2)},
				[]interface{}{nil, int64(2)},
				[]interface{}{nil, int64(2)},
				[]interface{}{nil, int64(2)},
			},
		},
		{
			// Select keys & values fo all invoice records.
			"select k, v from Customer where t = \"Invoice\"",
			[]interface{}{
				[]interface{}{customerRows[1].key, customerRows[1].value},
				[]interface{}{customerRows[2].key, customerRows[2].value},
				[]interface{}{customerRows[3].key, customerRows[3].value},
				[]interface{}{customerRows[5].key, customerRows[5].value},
				[]interface{}{customerRows[6].key, customerRows[6].value},
				[]interface{}{customerRows[7].key, customerRows[7].value},
				[]interface{}{customerRows[8].key, customerRows[8].value},
			},
		},
		{
			// Select key, cust id, invoice number and amount for $88 invoices.
			"select k, v.CustID, v.InvoiceNum, v.Amount from Customer where t = \"Invoice\" and v.Amount = 88",
			[]interface{}{
				[]interface{}{customerRows[3].key, int64(1), int64(1005), int64(88)},
				[]interface{}{customerRows[8].key, int64(2), int64(1006), int64(88)},
			},
		},
		{
			// Select keys & values for all records with a key prefix of "001".
			"select k, v from Customer where k like \"001%\"",
			[]interface{}{
				[]interface{}{customerRows[0].key, customerRows[0].value},
				[]interface{}{customerRows[1].key, customerRows[1].value},
				[]interface{}{customerRows[2].key, customerRows[2].value},
				[]interface{}{customerRows[3].key, customerRows[3].value},
			},
		},
		{
			// Select keys & values for all records with a key prefix of "001".
			"select k, v from Customer where k like \"002%\"",
			[]interface{}{
				[]interface{}{customerRows[4].key, customerRows[4].value},
				[]interface{}{customerRows[5].key, customerRows[5].value},
				[]interface{}{customerRows[6].key, customerRows[6].value},
				[]interface{}{customerRows[7].key, customerRows[7].value},
				[]interface{}{customerRows[8].key, customerRows[8].value},
			},
		},
		{
			// Select keys & values for all records with a key prefix of "001".
			// or a key prefix of "002".
			"select k, v from Customer where k like \"001%\" or k like \"002%\"",
			[]interface{}{
				[]interface{}{customerRows[0].key, customerRows[0].value},
				[]interface{}{customerRows[1].key, customerRows[1].value},
				[]interface{}{customerRows[2].key, customerRows[2].value},
				[]interface{}{customerRows[3].key, customerRows[3].value},
				[]interface{}{customerRows[4].key, customerRows[4].value},
				[]interface{}{customerRows[5].key, customerRows[5].value},
				[]interface{}{customerRows[6].key, customerRows[6].value},
				[]interface{}{customerRows[7].key, customerRows[7].value},
				[]interface{}{customerRows[8].key, customerRows[8].value},
			},
		},
		{
			// Select keys & values for all records with a key prefix of "001".
			// or a key prefix of "002".
			"select k, v from Customer where k like \"002%\" or k like \"001%\"",
			[]interface{}{
				[]interface{}{customerRows[0].key, customerRows[0].value},
				[]interface{}{customerRows[1].key, customerRows[1].value},
				[]interface{}{customerRows[2].key, customerRows[2].value},
				[]interface{}{customerRows[3].key, customerRows[3].value},
				[]interface{}{customerRows[4].key, customerRows[4].value},
				[]interface{}{customerRows[5].key, customerRows[5].value},
				[]interface{}{customerRows[6].key, customerRows[6].value},
				[]interface{}{customerRows[7].key, customerRows[7].value},
				[]interface{}{customerRows[8].key, customerRows[8].value},
			},
		},
		{
			// Let's play with whitespace and mixed case.
			"   sElEcT  k,  v from \n  Customer WhErE k lIkE \"002%\" oR k LiKe \"001%\"",
			[]interface{}{
				[]interface{}{customerRows[0].key, customerRows[0].value},
				[]interface{}{customerRows[1].key, customerRows[1].value},
				[]interface{}{customerRows[2].key, customerRows[2].value},
				[]interface{}{customerRows[3].key, customerRows[3].value},
				[]interface{}{customerRows[4].key, customerRows[4].value},
				[]interface{}{customerRows[5].key, customerRows[5].value},
				[]interface{}{customerRows[6].key, customerRows[6].value},
				[]interface{}{customerRows[7].key, customerRows[7].value},
				[]interface{}{customerRows[8].key, customerRows[8].value},
			},
		},
		{
			// Add in a like clause that accepts all strings.
			"   sElEcT  k,  v from \n  Customer WhErE k lIkE \"002%\" oR k LiKe \"001%\" or k lIkE \"%\"",
			[]interface{}{
				[]interface{}{customerRows[0].key, customerRows[0].value},
				[]interface{}{customerRows[1].key, customerRows[1].value},
				[]interface{}{customerRows[2].key, customerRows[2].value},
				[]interface{}{customerRows[3].key, customerRows[3].value},
				[]interface{}{customerRows[4].key, customerRows[4].value},
				[]interface{}{customerRows[5].key, customerRows[5].value},
				[]interface{}{customerRows[6].key, customerRows[6].value},
				[]interface{}{customerRows[7].key, customerRows[7].value},
				[]interface{}{customerRows[8].key, customerRows[8].value},
			},
		},
		{
			// Select id, name for customers whose last name is Masterson.
			"select v.ID, v.Name from Customer where t = \"Customer\" and v.Name like \"%Masterson\"",
			[]interface{}{
				[]interface{}{int64(2), "Bat Masterson"},
			},
		},
		{
			// Select records where v.Foo.FooBarBaz.Baz is 84 or type is Invoice.
			"select v from Customer where v.Foo.FooBarBaz.Baz = 84 or t = \"Invoice\"",
			[]interface{}{
				[]interface{}{customerRows[1].value},
				[]interface{}{customerRows[2].value},
				[]interface{}{customerRows[3].value},
				[]interface{}{customerRows[4].value},
				[]interface{}{customerRows[5].value},
				[]interface{}{customerRows[6].value},
				[]interface{}{customerRows[7].value},
				[]interface{}{customerRows[8].value},
			},
		},
		{
			// Select customer name for customer ID (i.e., key) "001".
			"select v.Name from Customer where t = \"Customer\" and k = \"001\"",
			[]interface{}{
				[]interface{}{"John Smith"},
			},
		},
		{
			// Select records where v.Foo.FooBarBaz.Bar is true.
			"select v from Customer where v.Foo.FooBarBaz.Bar = true",
			[]interface{}{
				[]interface{}{customerRows[0].value},
			},
		},
		{
			// Select records where v.Foo.FooBarBaz.Baz is 84 or type is Invoice.
			// Limit 3
			"select v from Customer where v.Foo.FooBarBaz.Baz = 84 or t = \"Invoice\" limit 3",
			[]interface{}{
				[]interface{}{customerRows[1].value},
				[]interface{}{customerRows[2].value},
				[]interface{}{customerRows[3].value},
			},
		},
		{
			// Select records where v.Foo.FooBarBaz.Baz is 84 or type is Invoice.
			// Offset 5
			"select v from Customer where v.Foo.FooBarBaz.Baz = 84 or t = \"Invoice\" offset 5",
			[]interface{}{
				[]interface{}{customerRows[6].value},
				[]interface{}{customerRows[7].value},
				[]interface{}{customerRows[8].value},
			},
		},
		{
			// Select records where v.Foo.FooBarBaz.Baz is 199.
			"select v from Customer where v.Foo.FooBarBaz.Baz = 199",
			[]interface{}{},
		},
		{
			// Select records where v.Foo.FooBarBaz.Baz is 84 or type is Invoice.
			// Offset 8
			"select v from Customer where v.Foo.FooBarBaz.Baz = 84 or t = \"Invoice\" offset 8",
			[]interface{}{},
		},
		{
			// Select records where v.Foo.FooBarBaz.Baz is 84 or type is Invoice.
			// Offset 23
			"select v from Customer where v.Foo.FooBarBaz.Baz = 84 or t = \"Invoice\" offset 23",
			[]interface{}{},
		},
		{
			// Select records where v.Foo.FooBarBaz.Baz is 84 or type is Invoice.
			// Limit 3 Offset 2
			"select v from Customer where v.Foo.FooBarBaz.Baz = 84 or t = \"Invoice\" limit 3 offset 2",
			[]interface{}{
				[]interface{}{customerRows[3].value},
				[]interface{}{customerRows[4].value},
				[]interface{}{customerRows[5].value},
			},
		},
	}

	for _, test := range basic {
		rs, err := query.Exec(db, test.query)
		if err != nil {
			t.Errorf("query: %s; got %v, want nil", test.query, err)
		} else {
			// Collect results.
			r := []interface{}{}
			for rs.Advance() {
				r = append(r, rs.Result())
			}
			if !reflect.DeepEqual(test.r, r) {
				t.Errorf("query: %s; got %v, want %v", test.query, r, test.r)
			}
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
			semErr := query_checker.Check(db, s)
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
					t.Errorf("query: %s; got %v, want query_parser.SelectStatement", test.query, reflect.TypeOf(*s))
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
			semErr := query_checker.Check(db, s)
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
					t.Errorf("query: %s; got %v, want query_parser.SelectStatement", test.query, reflect.TypeOf(*s))
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
			semErr := query_checker.Check(db, s)
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
					t.Errorf("query: %s; got %v, want query_parser.SelectStatement", test.query, reflect.TypeOf(*s))
				}
			}
		}
	}
}

func TestProjection(t *testing.T) {
	basic := []projectionTest{
		{
			"select k, v from Customer where t = \"Customer\"",
			"123456", sampleRow,
			[]interface{}{
				"123456",
				sampleRow,
			},
		},
		{
			"select k, v, v.Name, v.ID, v.Active, v.Rating, v.Street, v.City, v.State, v.Zip, v.GratuituousBigInt, v.GratuituousBigRat, v.GratuituousByte, v.GratuituousUint16, v.GratuituousUint32, v.GratuituousUint64, v.GratuituousInt16, v.GratuituousInt32, v.Foo, v.Foo.FooBarBaz, v.Foo.FooBarBaz.Foo, v.Foo.FooBarBaz.Bar, v.Foo.FooBarBaz.Baz from Customer where t = \"Customer\"",
			"123456", sampleRow,
			[]interface{}{
				"123456",
				sampleRow,
				"John Smith",
				int64(123456),
				true,
				'A',
				"1 Main St.",
				"Palo Alto",
				"CA",
				"94303",
				big.NewInt(1234567890),
				big.NewRat(123, 1),
				byte(12),
				uint16(1234),
				uint32(5678),
				uint64(999888777666),
				int16(9876),
				int32(876543),
				Nest1{Nest2{"foo", true, int64(42)}},
				Nest2{"foo", true, int64(42)},
				"foo",
				true,
				int64(42),
			},
		},
	}

	for _, test := range basic {
		s, synErr := query_parser.Parse(test.query)
		if synErr != nil {
			t.Errorf("query: %s; got %v, want nil", test.query, synErr)
		}
		if synErr == nil {
			semErr := query_checker.Check(db, s)
			if semErr != nil {
				t.Errorf("query: %s; got %v, want nil", test.query, semErr)
			}
			if semErr == nil {
				switch sel := (*s).(type) {
				case query_parser.SelectStatement:
					result := query.ComposeProjection(test.k, test.v, sel.Select)
					if !reflect.DeepEqual(result, test.result) {
						t.Errorf("query: %s; got %v, want %v", test.query, result, test.result)
					}
				default:
					t.Errorf("query: %s; got %v, want query_parser.SelectStatement", test.query, reflect.TypeOf(*s))
				}
			}
		}
	}
}

func TestExecSelectSingleRow(t *testing.T) {
	basic := []execSelectSingleRowTest{
		{
			"select k, v from Customer where t = \"Customer\"",
			"123456", sampleRow,
			[]interface{}{
				"123456",
				sampleRow,
			},
		},
		{
			"select k, v from Customer where t = \"Customer\" and k like \"123%\"",
			"123456", sampleRow,
			[]interface{}{
				"123456",
				sampleRow,
			},
		},
		{
			"select k, v from Customer where t = \"Invoice\" and k like \"123%\"",
			"123456", sampleRow,
			nil,
		},
		{
			"select k, v from Customer where t = \"Customer\" and k like \"456%\"",
			"123456", sampleRow,
			nil,
		},
		{
			"select v from Customer where v.Name = \"John Smith\"",
			"123456", sampleRow,
			[]interface{}{
				sampleRow,
			},
		},
		{
			"select v from Customer where v.Name = \"John Doe\"",
			"123456", sampleRow,
			nil,
		},
		{
			"select k, v, v.Name, v.ID, v.Active, v.Rating, v.Street, v.City, v.State, v.Zip, v.GratuituousBigInt, v.GratuituousBigRat, v.GratuituousByte, v.GratuituousUint16, v.GratuituousUint32, v.GratuituousUint64, v.GratuituousInt16, v.GratuituousInt32, v.Foo, v.Foo.FooBarBaz, v.Foo.FooBarBaz.Foo, v.Foo.FooBarBaz.Bar, v.Foo.FooBarBaz.Baz from Customer where t = \"Customer\"",
			"123456", sampleRow,
			[]interface{}{
				"123456",
				sampleRow,
				"John Smith",
				int64(123456),
				true,
				'A',
				"1 Main St.",
				"Palo Alto",
				"CA",
				"94303",
				big.NewInt(1234567890),
				big.NewRat(123, 1),
				byte(12),
				uint16(1234),
				uint32(5678),
				uint64(999888777666),
				int16(9876),
				int32(876543),
				Nest1{Nest2{"foo", true, int64(42)}},
				Nest2{"foo", true, int64(42)},
				"foo",
				true,
				int64(42),
			},
		},
	}

	for _, test := range basic {
		s, synErr := query_parser.Parse(test.query)
		if synErr != nil {
			t.Errorf("query: %s; got %v, want nil", test.query, synErr)
		}
		if synErr == nil {
			semErr := query_checker.Check(db, s)
			if semErr != nil {
				t.Errorf("query: %s; got %v, want nil", test.query, semErr)
			}
			if semErr == nil {
				switch sel := (*s).(type) {
				case query_parser.SelectStatement:
					result := query.ExecSelectSingleRow(test.k, test.v, &sel)
					if !reflect.DeepEqual(result, test.result) {
						t.Errorf("query: %s; got %v, want %v", test.query, result, test.result)
					}
				default:
					t.Errorf("query: %s; got %v, want query_parser.SelectStatement", test.query, reflect.TypeOf(*s))
				}
			}
		}
	}
}

// TODO(jkline): More negative tests here (even though they are tested elsewhere)?
func TestExecErrors(t *testing.T) {
	basic := []execSelectErrorTest{
		{
			"select a from Customer",
			query.Error(7, "Select field must be 'k' or 'v[{.<ident>}...]'."),
		},
		{
			"select v from Unknown",
			// The following error text is dependent on implementation of Database.
			query.Error(14, "No such table: Unknown."),
		},
		{
			"select v from Customer offset -1",
			// The following error text is dependent on implementation of Database.
			query.Error(30, "Expected positive integer literal., found '-'."),
		},
	}

	for _, test := range basic {
		_, err := query.Exec(db, test.query)
		if !reflect.DeepEqual(err, test.err) {
			t.Errorf("query: %s; got %v, want %v", test.query, err, test.err)
		}
	}
}
