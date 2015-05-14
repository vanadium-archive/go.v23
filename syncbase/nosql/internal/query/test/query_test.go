// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"v.io/syncbase/v23/syncbase/nosql/internal/query"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_checker"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_db"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/query_parser"
	"v.io/v23/vdl"
)

type mockDB struct {
	tables []table
}

type table struct {
	name string
	rows []kv
}

type keyValueStreamImpl struct {
	table        table
	cursor       int
	prefixes     []string
	prefixCursor int
}

func (kvs *keyValueStreamImpl) Advance() bool {
	for true {
		kvs.cursor++ // initialized to -1
		if kvs.cursor >= len(kvs.table.rows) {
			return false
		}
		for kvs.prefixCursor < len(kvs.prefixes) {
			// does it match any prefix
			if kvs.prefixes[kvs.prefixCursor] == "" || strings.HasPrefix(kvs.table.rows[kvs.cursor].key, kvs.prefixes[kvs.prefixCursor]) {
				return true
			}
			// Keys and prefixes are both sorted low to high, so we can increment
			// prefixCursor if the prefix is < the key.
			if kvs.prefixes[kvs.prefixCursor] < kvs.table.rows[kvs.cursor].key {
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

func (kvs *keyValueStreamImpl) KeyValue() (string, *vdl.Value) {
	return kvs.table.rows[kvs.cursor].key, kvs.table.rows[kvs.cursor].value
}

func (kvs *keyValueStreamImpl) Err() error {
	return nil
}

func (kvs *keyValueStreamImpl) Cancel() {
}

func (t table) Scan(prefixes []string) (query_db.KeyValueStream, error) {
	var keyValueStreamImpl keyValueStreamImpl
	keyValueStreamImpl.table = t
	keyValueStreamImpl.cursor = -1
	keyValueStreamImpl.prefixes = prefixes
	return &keyValueStreamImpl, nil
}

func (db mockDB) GetTable(table string) (query_db.Table, error) {
	for _, t := range db.tables {
		if t.name == table {
			return t, nil
		}
	}
	return nil, errors.New(fmt.Sprintf("No such table: %s.", table))

}

var db mockDB
var custTable table
var numTable table
var fooTable table

type kv struct {
	key   string
	value *vdl.Value
}

func init() {
	custTable.name = "Customer"
	custTable.rows = []kv{
		kv{
			"001",
			vdl.ValueOf(Customer{"John Smith", 1, true, AddressInfo{"1 Main St.", "Palo Alto", "CA", "94303"}, CreditReport{Agency: CreditAgencyEquifax, Report: AgencyReportEquifaxReport{EquifaxCreditReport{'A'}}}}),
		},
		kv{
			"001001",
			vdl.ValueOf(Invoice{1, 1000, 42, AddressInfo{"1 Main St.", "Palo Alto", "CA", "94303"}}),
		},
		kv{
			"001002",
			vdl.ValueOf(Invoice{1, 1003, 7, AddressInfo{"2 Main St.", "Palo Alto", "CA", "94303"}}),
		},
		kv{
			"001003",
			vdl.ValueOf(Invoice{1, 1005, 88, AddressInfo{"3 Main St.", "Palo Alto", "CA", "94303"}}),
		},
		kv{
			"002",
			vdl.ValueOf(Customer{"Bat Masterson", 2, true, AddressInfo{"777 Any St.", "Collins", "IA", "50055"}, CreditReport{Agency: CreditAgencyTransUnion, Report: AgencyReportTransUnionReport{TransUnionCreditReport{80}}}}),
		},
		kv{
			"002001",
			vdl.ValueOf(Invoice{2, 1001, 166, AddressInfo{"777 Any St.", "collins", "IA", "50055"}}),
		},
		kv{
			"002002",
			vdl.ValueOf(Invoice{2, 1002, 243, AddressInfo{"888 Any St.", "collins", "IA", "50055"}}),
		},
		kv{
			"002003",
			vdl.ValueOf(Invoice{2, 1004, 787, AddressInfo{"999 Any St.", "collins", "IA", "50055"}}),
		},
		kv{
			"002004",
			vdl.ValueOf(Invoice{2, 1006, 88, AddressInfo{"101010 Any St.", "collins", "IA", "50055"}}),
		},
	}
	db.tables = append(db.tables, custTable)

	numTable.name = "Numbers"
	numTable.rows = []kv{
		kv{
			"001",
			vdl.ValueOf(Numbers{byte(12), uint16(1234), uint32(5678), uint64(999888777666), int16(9876), int32(876543), int64(128), float32(3.14159), float64(2.71828182846), complex64(123.0 + 7.0i), complex128(456.789 + 10.1112i)}),
		},
		kv{
			"002",
			vdl.ValueOf(Numbers{byte(9), uint16(99), uint32(999), uint64(9999999), int16(9), int32(99), int64(88), float32(1.41421356237), float64(1.73205080757), complex64(9.87 + 7.65i), complex128(4.32 + 1.0i)}),
		},
		kv{
			"003",
			vdl.ValueOf(Numbers{byte(210), uint16(210), uint32(210), uint64(210), int16(210), int32(210), int64(210), float32(210.0), float64(210.0), complex64(210.0 + 0.0i), complex128(210.0 + 0.0i)}),
		},
	}
	db.tables = append(db.tables, numTable)

	fooTable.name = "Foo"
	fooTable.rows = []kv{
		kv{
			"001",
			vdl.ValueOf(FooType{BarType{BazType{"FooBarBaz", TitleOrValueTypeTitle{"Vice President"}}}}),
		},
		kv{
			"002",
			vdl.ValueOf(FooType{BarType{BazType{"BazBarFoo", TitleOrValueTypeValue{42}}}}),
		},
	}
	db.tables = append(db.tables, fooTable)
}

type keyPrefixesTest struct {
	query       string
	keyPrefixes []string
	err         *query.QueryError
}

type evalWhereUsingOnlyKeyTest struct {
	query  string
	key    string
	result query.EvalWithKeyResult
}

type evalTest struct {
	query  string
	k      string
	v      *vdl.Value
	result bool
}

type projectionTest struct {
	query  string
	k      string
	v      *vdl.Value
	result []*vdl.Value
}

type execSelectTest struct {
	query string
	r     [][]*vdl.Value
}

type execSelectSingleRowTest struct {
	query  string
	k      string
	v      *vdl.Value
	result []*vdl.Value
}

type execSelectErrorTest struct {
	query string
	err   *query.QueryError
}

type execResolveFieldTest struct {
	k string
	v *vdl.Value
	f query_parser.Field
	r *vdl.Value
}

func TestQueryExec(t *testing.T) {
	basic := []execSelectTest{
		{
			// Select values for all customer records.
			"select v from Customer where t = \"Customer\"",
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[0].value},
				[]*vdl.Value{custTable.rows[4].value},
			},
		},
		{
			// Select values where v.InvoiceNum is nil
			// Since InvoiceNum does not exist for Invoice,
			// this will return just customers.
			"select v from Customer where v.InvoiceNum is nil",
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[0].value},
				[]*vdl.Value{custTable.rows[4].value},
			},
		},
		{
			// Select values where v.InvoiceNum is nil
			// or v.Name is nil This will select all customers
			// with the former and all invoices with the latter.
			// Hence, all records are returned.
			"select v from Customer where v.InvoiceNum is nil or v.Name is nil",
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[0].value},
				[]*vdl.Value{custTable.rows[1].value},
				[]*vdl.Value{custTable.rows[2].value},
				[]*vdl.Value{custTable.rows[3].value},
				[]*vdl.Value{custTable.rows[4].value},
				[]*vdl.Value{custTable.rows[5].value},
				[]*vdl.Value{custTable.rows[6].value},
				[]*vdl.Value{custTable.rows[7].value},
				[]*vdl.Value{custTable.rows[8].value},
			},
		},
		{
			// Select values where v.InvoiceNum is nil
			// and v.Name is nil.  Expect nothing returned.
			"select v from Customer where v.InvoiceNum is nil and v.Name is nil",
			[][]*vdl.Value{},
		},
		{
			// Select values where v.InvoiceNum is not nil
			// This will return just invoices.
			"select v from Customer where v.InvoiceNum is not nil",
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[1].value},
				[]*vdl.Value{custTable.rows[2].value},
				[]*vdl.Value{custTable.rows[3].value},
				[]*vdl.Value{custTable.rows[5].value},
				[]*vdl.Value{custTable.rows[6].value},
				[]*vdl.Value{custTable.rows[7].value},
				[]*vdl.Value{custTable.rows[8].value},
			},
		},
		{
			// Select values where v.InvoiceNum is not nil
			// or v.Name is not nil. All records are returned.
			"select v from Customer where v.InvoiceNum is not nil or v.Name is not nil",
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[0].value},
				[]*vdl.Value{custTable.rows[1].value},
				[]*vdl.Value{custTable.rows[2].value},
				[]*vdl.Value{custTable.rows[3].value},
				[]*vdl.Value{custTable.rows[4].value},
				[]*vdl.Value{custTable.rows[5].value},
				[]*vdl.Value{custTable.rows[6].value},
				[]*vdl.Value{custTable.rows[7].value},
				[]*vdl.Value{custTable.rows[8].value},
			},
		},
		{
			// Select values where v.InvoiceNum is nil and v.Name is not nil.
			// All customers are returned.
			"select v from Customer where v.InvoiceNum is nil and v.Name is not nil",
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[0].value},
				[]*vdl.Value{custTable.rows[4].value},
			},
		},
		{
			// Select values where v.InvoiceNum is not nil
			// and v.Name is not nil.  Expect nothing returned.
			"select v from Customer where v.InvoiceNum is not nil and v.Name is not nil",
			[][]*vdl.Value{},
		},
		{
			// Select keys & values for all customer records.
			"select k, v from Customer where t = \"Customer\"",
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[0].key), custTable.rows[0].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[4].key), custTable.rows[4].value},
			},
		},
		{
			// Select keys & names for all customer records.
			"select k, v.Name from Customer where t = \"Customer\"",
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[0].key), vdl.ValueOf("John Smith")},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[4].key), vdl.ValueOf("Bat Masterson")},
			},
		},
		{
			// Select both customer and invoice records.
			// Customer records have Id.
			// Invoice records have CustId.
			"select v.Id, v.CustId from Customer",
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int64(1)), vdl.ValueOf(nil)},
				[]*vdl.Value{vdl.ValueOf(nil), vdl.ValueOf(int64(1))},
				[]*vdl.Value{vdl.ValueOf(nil), vdl.ValueOf(int64(1))},
				[]*vdl.Value{vdl.ValueOf(nil), vdl.ValueOf(int64(1))},
				[]*vdl.Value{vdl.ValueOf(int64(2)), vdl.ValueOf(nil)},
				[]*vdl.Value{vdl.ValueOf(nil), vdl.ValueOf(int64(2))},
				[]*vdl.Value{vdl.ValueOf(nil), vdl.ValueOf(int64(2))},
				[]*vdl.Value{vdl.ValueOf(nil), vdl.ValueOf(int64(2))},
				[]*vdl.Value{vdl.ValueOf(nil), vdl.ValueOf(int64(2))},
			},
		},
		{
			// Select keys & values fo all invoice records.
			"select k, v from Customer where t = \"Invoice\"",
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[1].key), custTable.rows[1].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[2].key), custTable.rows[2].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[3].key), custTable.rows[3].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[5].key), custTable.rows[5].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[6].key), custTable.rows[6].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[7].key), custTable.rows[7].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[8].key), custTable.rows[8].value},
			},
		},
		{
			// Select key, cust id, invoice number and amount for $88 invoices.
			"select k, v.CustId, v.InvoiceNum, v.Amount from Customer where t = \"Invoice\" and v.Amount = 88",
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[3].key), vdl.ValueOf(int64(1)), vdl.ValueOf(int64(1005)), vdl.ValueOf(int64(88))},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[8].key), vdl.ValueOf(int64(2)), vdl.ValueOf(int64(1006)), vdl.ValueOf(int64(88))},
			},
		},
		{
			// Select keys & values for all records with a key prefix of "001".
			"select k, v from Customer where k like \"001%\"",
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[0].key), custTable.rows[0].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[1].key), custTable.rows[1].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[2].key), custTable.rows[2].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[3].key), custTable.rows[3].value},
			},
		},
		{
			// Select keys & values for all records with a key prefix of "001".
			"select k, v from Customer where k like \"002%\"",
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[4].key), custTable.rows[4].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[5].key), custTable.rows[5].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[6].key), custTable.rows[6].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[7].key), custTable.rows[7].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[8].key), custTable.rows[8].value},
			},
		},
		{
			// Select keys & values for all records with a key prefix of "001".
			// or a key prefix of "002".
			"select k, v from Customer where k like \"001%\" or k like \"002%\"",
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[0].key), custTable.rows[0].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[1].key), custTable.rows[1].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[2].key), custTable.rows[2].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[3].key), custTable.rows[3].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[4].key), custTable.rows[4].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[5].key), custTable.rows[5].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[6].key), custTable.rows[6].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[7].key), custTable.rows[7].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[8].key), custTable.rows[8].value},
			},
		},
		{
			// Select keys & values for all records with a key prefix of "001".
			// or a key prefix of "002".
			"select k, v from Customer where k like \"002%\" or k like \"001%\"",
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[0].key), custTable.rows[0].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[1].key), custTable.rows[1].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[2].key), custTable.rows[2].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[3].key), custTable.rows[3].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[4].key), custTable.rows[4].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[5].key), custTable.rows[5].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[6].key), custTable.rows[6].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[7].key), custTable.rows[7].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[8].key), custTable.rows[8].value},
			},
		},
		{
			// Let's play with whitespace and mixed case.
			"   sElEcT  k,  v from \n  Customer WhErE k lIkE \"002%\" oR k LiKe \"001%\"",
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[0].key), custTable.rows[0].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[1].key), custTable.rows[1].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[2].key), custTable.rows[2].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[3].key), custTable.rows[3].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[4].key), custTable.rows[4].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[5].key), custTable.rows[5].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[6].key), custTable.rows[6].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[7].key), custTable.rows[7].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[8].key), custTable.rows[8].value},
			},
		},
		{
			// Add in a like clause that accepts all strings.
			"   sElEcT  k,  v from \n  Customer WhErE k lIkE \"002%\" oR k LiKe \"001%\" or k lIkE \"%\"",
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[0].key), custTable.rows[0].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[1].key), custTable.rows[1].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[2].key), custTable.rows[2].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[3].key), custTable.rows[3].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[4].key), custTable.rows[4].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[5].key), custTable.rows[5].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[6].key), custTable.rows[6].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[7].key), custTable.rows[7].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[8].key), custTable.rows[8].value},
			},
		},
		{
			// Select id, name for customers whose last name is Masterson.
			"select v.Id, v.Name from Customer where t = \"Customer\" and v.Name like \"%Masterson\"",
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int64(2)), vdl.ValueOf("Bat Masterson")},
			},
		},
		{
			// Select records where v.Address.City is "Collins" or type is Invoice.
			"select v from Customer where v.Address.City = \"Collins\" or t = \"Invoice\"",
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[1].value},
				[]*vdl.Value{custTable.rows[2].value},
				[]*vdl.Value{custTable.rows[3].value},
				[]*vdl.Value{custTable.rows[4].value},
				[]*vdl.Value{custTable.rows[5].value},
				[]*vdl.Value{custTable.rows[6].value},
				[]*vdl.Value{custTable.rows[7].value},
				[]*vdl.Value{custTable.rows[8].value},
			},
		},
		{
			// Select records where v.Address.City is "Collins" and v.InvoiceNum is not nil.
			"select v from Customer where v.Address.City = \"Collins\" and v.InvoiceNum is not nil",
			[][]*vdl.Value{},
		},
		{
			// Select records where v.Address.City is "Collins" and v.InvoiceNum is nil.
			"select v from Customer where v.Address.City = \"Collins\" and v.InvoiceNum is nil",
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[4].value},
			},
		},
		{
			// Select customer name for customer Id (i.e., key) "001".
			"select v.Name from Customer where t = \"Customer\" and k = \"001\"",
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("John Smith")},
			},
		},
		{
			// Select v where v.Credit.Report.EquifaxReport.Rating = 'A'
			"select v from Customer where v.Credit.Report.EquifaxReport.Rating = 'A'",
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[0].value},
			},
		},
		{
			// Select v where v.AgencyRating = "Bad"
			"select v from Customer where v.Credit.Report.EquifaxReport.Rating < 'A' or v.Credit.Report.ExperianReport.Rating = \"Bad\" or v.Credit.Report.TransUnionReport.Rating < 90",
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[4].value},
			},
		},
		{
			// Select records where v.Bar.Baz.Name = "FooBarBaz"
			"select v from Foo where v.Bar.Baz.Name = \"FooBarBaz\"",
			[][]*vdl.Value{
				[]*vdl.Value{fooTable.rows[0].value},
			},
		},
		{
			// Select records where v.Bar.Baz.TitleOrValue.Value = 42
			"select v from Foo where v.Bar.Baz.TitleOrValue.Value = 42",
			[][]*vdl.Value{
				[]*vdl.Value{fooTable.rows[1].value},
			},
		},
		{
			// Select records where v.Bar.Baz.TitleOrValue.Title = "Vice President"
			"select v from Foo where v.Bar.Baz.TitleOrValue.Title = \"Vice President\"",
			[][]*vdl.Value{
				[]*vdl.Value{fooTable.rows[0].value},
			},
		},
		{
			// Select records where v.Address.City = "Collins" or type is Invoice.
			// Limit 3
			"select v from Customer where v.Address.City = \"Collins\" or t = \"Invoice\" limit 3",
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[1].value},
				[]*vdl.Value{custTable.rows[2].value},
				[]*vdl.Value{custTable.rows[3].value},
			},
		},
		{
			// Select records where v.Address.City = "Collins" or type is Invoice.
			// Offset 5
			"select v from Customer where v.Address.City = \"Collins\" or t = \"Invoice\" offset 5",
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[6].value},
				[]*vdl.Value{custTable.rows[7].value},
				[]*vdl.Value{custTable.rows[8].value},
			},
		},
		{
			// Select records where v.Address.City = "Collins" is "Mountain View".
			"select v from Customer where v.Address.City = \"Mountain View\"",
			[][]*vdl.Value{},
		},
		{
			// Select records where v.Address.City = "Collins" or type is Invoice.
			// Offset 8
			"select v from Customer where v.Address.City = \"Collins\" or t = \"Invoice\" offset 8",
			[][]*vdl.Value{},
		},
		{
			// Select records where v.Address.City = "Collins" or type is Invoice.
			// Offset 23
			"select v from Customer where v.Address.City = \"Collins\" or t = \"Invoice\" offset 23",
			[][]*vdl.Value{},
		},
		{
			// Select records where v.Address.City = "Collins" is 84 or type is Invoice.
			// Limit 3 Offset 2
			"select v from Customer where v.Address.City = \"Collins\" or t = \"Invoice\" limit 3 offset 2",
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[3].value},
				[]*vdl.Value{custTable.rows[4].value},
				[]*vdl.Value{custTable.rows[5].value},
			},
		},
		{
			// Select records where v.Address.City = "Collins" or (type is Invoice and v.InvoiceNum is not nil).
			// Limit 3 Offset 2
			"select v from Customer where v.Address.City = \"Collins\" or (t = \"Invoice\" and v.InvoiceNum is not nil) limit 3 offset 2",
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[3].value},
				[]*vdl.Value{custTable.rows[4].value},
				[]*vdl.Value{custTable.rows[5].value},
			},
		},
		{
			// Select records where v.Address.City = "Collins" or (type is Invoice and v.InvoiceNum is nil).
			// Limit 3 Offset 2
			"select v from Customer where v.Address.City = \"Collins\" or (t = \"Invoice\" and v.InvoiceNum is nil) limit 3 offset 2",
			[][]*vdl.Value{},
		},
	}

	for _, test := range basic {
		rs, err := query.Exec(db, test.query)
		if err != nil {
			t.Errorf("query: %s; got %v, want nil", test.query, err)
		} else {
			// Collect results.
			r := [][]*vdl.Value{}
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
			query.INCLUDE,
		},
		{
			// Row will be rejected using only the key.
			"select k, v from Customer where k like \"abc\"",
			"abcd",
			query.EXCLUDE,
		},
		{
			// Need value to determine if row should be selected.
			"select k, v from Customer where k = \"abc\" or v.zip = \"94303\"",
			"abcd",
			query.FETCH_VALUE,
		},
		{
			// Need value (i.e., its type) to determine if row should be selected.
			"select k, v from Customer where k = \"xyz\" or t = \"foo.Bar\"",
			"wxyz",
			query.FETCH_VALUE,
		},
		{
			// Although value is in where clause, it is not needed to reject row.
			"select k, v from Customer where k = \"abcd\" and v.zip = \"94303\"",
			"abcde",
			query.EXCLUDE,
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
					result := query.EvalWhereUsingOnlyKey(&sel, test.key)
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

func TestEval(t *testing.T) {
	basic := []evalTest{
		{
			"select k, v from Customer where t = \"v.io/syncbase/v23/syncbase/nosql/internal/query/test.Customer\"",
			custTable.rows[0].key, custTable.rows[0].value, true,
		},
		{
			"select k, v from Customer where t = \"Customer\"",
			numTable.rows[0].key, custTable.rows[0].value, true,
		},
		{
			"select k, v from Customer where v.Name = \"John Smith\"",
			numTable.rows[0].key, custTable.rows[0].value, true,
		},
		{
			"select k, v from Customer where v.Name = v.Name",
			numTable.rows[0].key, custTable.rows[0].value, true,
		},
		{
			"select k, v from Customer where v = v",
			numTable.rows[0].key, custTable.rows[0].value, true,
		},
		{
			"select k, v from Customer where v > v",
			numTable.rows[0].key, custTable.rows[0].value, false,
		},
		{
			"select k, v from Customer where v < v",
			numTable.rows[0].key, custTable.rows[0].value, false,
		},
		{
			"select k, v from Customer where v >= v",
			numTable.rows[0].key, custTable.rows[0].value, false,
		},
		{
			"select k, v from Customer where v <= v",
			numTable.rows[0].key, custTable.rows[0].value, false,
		},
		{
			"select k, v from Customer where v.Credit.Report.EquifaxReport.Rating = 'A'",
			numTable.rows[0].key, custTable.rows[0].value, true,
		},
		{
			"select k, v from Customer where v.Credit.Report.EquifaxReport.Rating <> 'A'",
			numTable.rows[0].key, custTable.rows[0].value, false,
		},
		{
			"select k, v from Customer where v.Credit.Report.EquifaxReport.Rating >= 'B'",
			numTable.rows[0].key, custTable.rows[0].value, false,
		},
		{
			"select k, v from Customer where v.Credit.Report.EquifaxReport.Rating <= 'B'",
			numTable.rows[0].key, custTable.rows[0].value, true,
		},
		{
			"select k, v from Customer where v.Credit.Report.TransUnionReport.Rating = 80",
			numTable.rows[0].key, custTable.rows[4].value, true,
		},
		{
			"select k, v from Customer where v.Credit.Report.TransUnionReport.Rating <> 80",
			numTable.rows[0].key, custTable.rows[4].value, false,
		},
		{
			"select k, v from Customer where v.Cr4dit.Report.TransUnionReport.Rating <= 70",
			numTable.rows[0].key, custTable.rows[0].value, false,
		},
		{
			"select k, v from Customer where v.Credit.Report.TransUnionReport.Rating >= 70",
			numTable.rows[0].key, custTable.rows[4].value, true,
		},
		{
			"select k, v from Customer where v.Active = true",
			numTable.rows[0].key, custTable.rows[0].value, true,
		},
		{
			"select k, v from Customer where v.Active = false",
			numTable.rows[0].key, custTable.rows[0].value, false,
		},
		{
			"select k, v from Numbers where 12 = v.B",
			numTable.rows[0].key, numTable.rows[0].value, true,
		},
		{
			"select k, v from Numbers where 11 < v.B",
			numTable.rows[0].key, numTable.rows[0].value, true,
		},
		{
			"select k, v from Numbers where v.B > 10",
			numTable.rows[0].key, numTable.rows[0].value, true,
		},
		{
			"select k, v from Numbers where v.B >= 14",
			numTable.rows[0].key, numTable.rows[0].value, false,
		},
		{
			"select k, v from Numbers where v.B >= 11.0",
			numTable.rows[0].key, numTable.rows[0].value, true,
		},
		{
			"select k, v from Numbers where v.Ui64 = 999888777666",
			numTable.rows[0].key, numTable.rows[0].value, true,
		},
		{
			"select k, v from Numbers where v.Ui64 < 999888777666",
			numTable.rows[0].key, numTable.rows[0].value, false,
		},
		{
			"select k, v from Numbers where v.B < v.Ui64",
			numTable.rows[0].key, numTable.rows[0].value, true,
		},
		{
			"select k, v from Numbers where v.C64 = \"(123+7i)\"",
			numTable.rows[0].key, numTable.rows[0].value, true,
		},
		{
			"select k, v from Numbers where v.C128 = \"(456.789+10.1112i)\"",
			numTable.rows[0].key, numTable.rows[0].value, true,
		},
		{
			"select k, v from Numbers where v.Ui16 = 1234",
			numTable.rows[0].key, numTable.rows[0].value, true,
		},
		{
			"select k, v from Numbers where v.Ui32 = 5678",
			numTable.rows[0].key, numTable.rows[0].value, true,
		},
		{
			"select k, v from Numbers where v.I16 = 9876",
			numTable.rows[0].key, numTable.rows[0].value, true,
		},
		{
			"select k, v from Numbers where v.I32 = 876543",
			numTable.rows[0].key, numTable.rows[0].value, true,
		},
		{
			// Deeply nested.
			"select v from Foo where v.Bar.Baz.Name = \"FooBarBaz\"",
			numTable.rows[0].key, fooTable.rows[0].value, true,
		},
		{
			// Convert int64 to string
			"select v from Customer where v.CustId = \"1\"",
			numTable.rows[0].key, custTable.rows[1].value, true,
		},
		{
			// Convert bool to string
			"select v from Customer where v.Active = \"true\"",
			numTable.rows[0].key, custTable.rows[0].value, true,
		},
		{
			// Bool can't convert to other types.
			"select v from Customer where v.Active = 1",
			numTable.rows[0].key, custTable.rows[0].value, false,
		},
		{
			// Test that numeric types can compare to a float64
			"select v from Numbers where v.F64 = v.B and v.F64 = v.Ui16 and v.F64 = v.Ui32 and v.F64 = v.Ui64 and v.F64 = v.I16 and v.F64 = v.I32 and v.F64 = v.I64 and v.F64 = v.F32 and v.F64 = v.C64 and v.F64 = v.C128",
			numTable.rows[2].key, numTable.rows[2].value, true,
		},
		{
			// Test that all numeric types can compare to an int32
			"select v from Numbers where v.I32 = v.B and v.I32 = v.Ui16 and v.I32 = v.Ui32 and v.I32 = v.Ui64 and v.I32 = v.I16 and v.I32 = v.F64 and v.I32 = v.I64 and v.I32 = v.F32 and v.I32 = v.C64 and v.I32 = v.C128",
			numTable.rows[2].key, numTable.rows[2].value, true,
		},
		{
			// Test that all numeric types can compare to an int16
			"select v from Numbers where v.I16 = v.B and v.I16 = v.Ui16 and v.I16 = v.Ui32 and v.I16 = v.Ui64 and v.I16 = v.F64 and v.I16 = v.I32 and v.I16 = v.I64 and v.I16 = v.F32 and v.I16 = v.C64 and v.I16 = v.C128",
			numTable.rows[2].key, numTable.rows[2].value, true,
		},
		{
			// Test that all numeric types can compare to an uint64
			"select v from Numbers where v.Ui64 = v.B and v.Ui64 = v.Ui16 and v.Ui64 = v.Ui32 and v.Ui64 = v.F64 and v.Ui64 = v.I16 and v.Ui64 = v.I32 and v.Ui64 = v.I64 and v.Ui64 = v.F32 and v.Ui64 = v.C64 and v.Ui64 = v.C128",
			numTable.rows[2].key, numTable.rows[2].value, true,
		},
		{
			// Test that all numeric types can compare to an uint32
			"select v from Numbers where v.Ui32 = v.B and v.Ui32 = v.Ui16 and v.Ui32 = v.F64 and v.Ui32 = v.Ui64 and v.Ui32 = v.I16 and v.Ui32 = v.I32 and v.Ui32 = v.I64 and v.Ui32 = v.F32 and v.Ui32 = v.C64 and v.Ui32 = v.C128",
			numTable.rows[2].key, numTable.rows[2].value, true,
		},
		{
			// Test that all numeric types can compare to an uint16
			"select v from Numbers where v.Ui16 = v.B and v.Ui16 = v.F64 and v.Ui16 = v.Ui32 and v.Ui16 = v.Ui64 and v.Ui16 = v.I16 and v.Ui16 = v.I32 and v.Ui16 = v.I64 and v.Ui16 = v.F32 and v.Ui16 = v.C64 and v.Ui16 = v.C128",
			numTable.rows[2].key, numTable.rows[2].value, true,
		},
		{
			// Test C64 = C128
			"select v from Numbers where v.C64 = v.C128",
			numTable.rows[2].key, numTable.rows[2].value, true,
		},
		{
			// Test C64 <> C128
			"select v from Numbers where v.C64 <> v.C128",
			numTable.rows[0].key, numTable.rows[0].value, true,
		},
		{
			// Complex integers can only compare to themselves and other numerics.
			// Compare to integer
			"select v from Numbers where v.C128 <> false",
			numTable.rows[0].key, numTable.rows[0].value, false,
		},
		{
			"select v from Numbers where v.C128 = \"(456.789+10.1112i)\"",
			numTable.rows[0].key, numTable.rows[0].value, true,
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
			"123456", custTable.rows[0].value,
			[]*vdl.Value{
				vdl.ValueOf("123456"),
				custTable.rows[0].value,
			},
		},
		{
			"select k, v, v.Name, v.Id, v.Active, v.Credit.Agency, v.Credit.Report.EquifaxReport.Rating, v.Address.Street, v.Address.City, v.Address.State, v.Address.Zip from Customer where t = \"Customer\"",
			custTable.rows[0].key, custTable.rows[0].value,
			[]*vdl.Value{
				vdl.ValueOf(custTable.rows[0].key),
				custTable.rows[0].value,
				vdl.ValueOf("John Smith"),
				vdl.ValueOf(int64(1)),
				vdl.ValueOf(true),
				vdl.ValueOf(CreditAgencyEquifax),
				vdl.ValueOf(byte('A')),
				vdl.ValueOf("1 Main St."),
				vdl.ValueOf("Palo Alto"),
				vdl.ValueOf("CA"),
				vdl.ValueOf("94303"),
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
			"123456", custTable.rows[0].value,
			[]*vdl.Value{
				vdl.ValueOf("123456"),
				custTable.rows[0].value,
			},
		},
		{
			"select k, v from Customer where t = \"Customer\" and k like \"123%\"",
			"123456", custTable.rows[0].value,
			[]*vdl.Value{
				vdl.ValueOf("123456"),
				custTable.rows[0].value,
			},
		},
		{
			"select k, v from Customer where t = \"Invoice\" and k like \"123%\"",
			"123456", custTable.rows[0].value,
			[]*vdl.Value{},
		},
		{
			"select k, v from Customer where t = \"Customer\" and k like \"456%\"",
			"123456", custTable.rows[0].value,
			[]*vdl.Value{},
		},
		{
			"select v from Customer where v.Name = \"John Smith\"",
			"123456", custTable.rows[0].value,
			[]*vdl.Value{
				custTable.rows[0].value,
			},
		},
		{
			"select v from Customer where v.Name = \"John Doe\"",
			"123456", custTable.rows[0].value,
			[]*vdl.Value{},
		},
		{
			"select k, v, v.Name, v.Id, v.Active, v.Credit.Report.EquifaxReport.Rating, v.Credit.Report.ExperianReport.Rating, v.Credit.Report.TransUnionReport.Rating, v.Address.Street, v.Address.City, v.Address.State, v.Address.Zip from Customer where t = \"Customer\"",
			custTable.rows[0].key, custTable.rows[0].value,
			[]*vdl.Value{
				vdl.ValueOf(custTable.rows[0].key),
				custTable.rows[0].value,
				vdl.ValueOf("John Smith"),
				vdl.ValueOf(int64(1)),
				vdl.ValueOf(true),
				vdl.ValueOf(byte('A')),
				vdl.ValueOf(nil),
				vdl.ValueOf(nil),
				vdl.ValueOf("1 Main St."),
				vdl.ValueOf("Palo Alto"),
				vdl.ValueOf("CA"),
				vdl.ValueOf("94303"),
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

func TestResolveField(t *testing.T) {
	basic := []execResolveFieldTest{
		{
			custTable.rows[0].key,
			custTable.rows[0].value,
			query_parser.Field{
				Segments: []query_parser.Segment{
					query_parser.Segment{
						Value: "t",
						Node:  query_parser.Node{Off: 7},
					},
				},
				Node: query_parser.Node{Off: 7},
			},
			vdl.ValueOf("v.io/syncbase/v23/syncbase/nosql/internal/query/test.Customer"),
		},
		{
			custTable.rows[0].key,
			custTable.rows[0].value,
			query_parser.Field{
				Segments: []query_parser.Segment{
					query_parser.Segment{
						Value: "k",
						Node:  query_parser.Node{Off: 7},
					},
				},
				Node: query_parser.Node{Off: 7},
			},
			vdl.ValueOf("001"),
		},
		{
			custTable.rows[0].key,
			custTable.rows[0].value,
			query_parser.Field{
				Segments: []query_parser.Segment{
					query_parser.Segment{
						Value: "v",
						Node:  query_parser.Node{Off: 7},
					},
					query_parser.Segment{
						Value: "Address",
						Node:  query_parser.Node{Off: 7},
					},
					query_parser.Segment{
						Value: "City",
						Node:  query_parser.Node{Off: 7},
					},
				},
				Node: query_parser.Node{Off: 7},
			},
			vdl.ValueOf("Palo Alto"),
		},
	}

	for _, test := range basic {
		r, _, _ := query.ResolveField(test.k, test.v, &test.f)
		if !reflect.DeepEqual(r, test.r) {
			t.Errorf("got %v(%s), want %v(%s)", r, r.Type(), test.r, test.r.Type())
		}
	}
}
