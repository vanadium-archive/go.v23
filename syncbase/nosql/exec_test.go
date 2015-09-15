// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/nosql"
	"v.io/v23/syncbase/nosql/query"
	// TODO(sadovsky): For legacy reasons, we import all testdata identifiers into
	// the top-level namespace.
	. "v.io/v23/syncbase/nosql/testdata"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/generic"
	tu "v.io/x/ref/services/syncbase/testutil"
)

var ctx *context.T
var db nosql.Database
var cleanup func()

// In addition to populating store, populate these arrays to make
// specifying the wanted values in the tests easier.
type kv struct {
	key   string
	value *vdl.Value
}

var customerEntries []kv
var numbersEntries []kv
var fooEntries []kv
var keyIndexDataEntries []kv

var t2015 time.Time

var t2015_04 time.Time
var t2015_04_12 time.Time
var t2015_04_12_22 time.Time
var t2015_04_12_22_16 time.Time
var t2015_04_12_22_16_06 time.Time

var t2015_07 time.Time
var t2015_07_01 time.Time
var t2015_07_01_01_23_45 time.Time

func setup(t *testing.T) {
	var sName string
	ctx, sName, cleanup = tu.SetupOrDie(nil)
	a := tu.CreateApp(t, ctx, syncbase.NewService(sName), "a")
	db = tu.CreateNoSQLDatabase(t, ctx, a, "db")
	customerTable := tu.CreateTable(t, ctx, db, "Customer")
	numbersTable := tu.CreateTable(t, ctx, db, "Numbers")
	fooTable := tu.CreateTable(t, ctx, db, "Foo")
	keyIndexDataTable := tu.CreateTable(t, ctx, db, "KeyIndexData")
	bigTable := tu.CreateTable(t, ctx, db, "BigTable")

	t20150122131101, _ := time.Parse("Jan 2 2006 15:04:05 -0700 MST", "Jan 22 2015 13:11:01 -0800 PST")
	t20150210161202, _ := time.Parse("Jan 2 2006 15:04:05 -0700 MST", "Feb 10 2015 16:12:02 -0800 PST")
	t20150311101303, _ := time.Parse("Jan 2 2006 15:04:05 -0700 MST", "Mar 11 2015 10:13:03 -0700 PDT")
	t20150317111404, _ := time.Parse("Jan 2 2006 15:04:05 -0700 MST", "Mar 17 2015 11:14:04 -0700 PDT")
	t20150317131505, _ := time.Parse("Jan 2 2006 15:04:05 -0700 MST", "Mar 17 2015 13:15:05 -0700 PDT")
	t20150412221606, _ := time.Parse("Jan 2 2006 15:04:05 -0700 MST", "Apr 12 2015 22:16:06 -0700 PDT")
	t20150413141707, _ := time.Parse("Jan 2 2006 15:04:05 -0700 MST", "Apr 13 2015 14:17:07 -0700 PDT")

	t2015, _ = time.Parse("2006 MST", "2015 PST")

	t2015_04, _ = time.Parse("Jan 2006 MST", "Apr 2015 PDT")
	t2015_07, _ = time.Parse("Jan 2006 MST", "Jul 2015 PDT")

	t2015_04_12, _ = time.Parse("Jan 2 2006 MST", "Apr 12 2015 PDT")
	t2015_07_01, _ = time.Parse("Jan 2 2006 MST", "Jul 01 2015 PDT")

	t2015_04_12_22, _ = time.Parse("Jan 2 2006 15 MST", "Apr 12 2015 22 PDT")
	t2015_04_12_22_16, _ = time.Parse("Jan 2 2006 15:04 MST", "Apr 12 2015 22:16 PDT")
	t2015_04_12_22_16_06, _ = time.Parse("Jan 2 2006 15:04:05 MST", "Apr 12 2015 22:16:06 PDT")
	t2015_07_01_01_23_45, _ = time.Parse("Jan 2 2006 15:04:05 MST", "Jul 01 2015 01:23:45 PDT")

	k := "001"
	c := Customer{"John Smith", 1, true, AddressInfo{"1 Main St.", "Palo Alto", "CA", "94303"}, CreditReport{Agency: CreditAgencyEquifax, Report: AgencyReportEquifaxReport{EquifaxCreditReport{'A'}}}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(c)})
	if err := customerTable.Put(ctx, k, c); err != nil {
		t.Fatalf("customerTable.Put() failed: %v", err)
	}
	k = "001001"
	i := Invoice{1, 1000, t20150122131101, 42, AddressInfo{"1 Main St.", "Palo Alto", "CA", "94303"}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(i)})
	if err := customerTable.Put(ctx, k, i); err != nil {
		t.Fatalf("customerTable.Put() failed: %v", err)
	}

	k = "001002"
	i = Invoice{1, 1003, t20150210161202, 7, AddressInfo{"2 Main St.", "Palo Alto", "CA", "94303"}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(i)})
	if err := customerTable.Put(ctx, k, i); err != nil {
		t.Fatalf("customerTable.Put() failed: %v", err)
	}

	k = "001003"
	i = Invoice{1, 1005, t20150311101303, 88, AddressInfo{"3 Main St.", "Palo Alto", "CA", "94303"}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(i)})
	if err := customerTable.Put(ctx, k, i); err != nil {
		t.Fatalf("customerTable.Put() failed: %v", err)
	}

	k = "002"
	c = Customer{"Bat Masterson", 2, true, AddressInfo{"777 Any St.", "Collins", "IA", "50055"}, CreditReport{Agency: CreditAgencyTransUnion, Report: AgencyReportTransUnionReport{TransUnionCreditReport{80}}}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(c)})
	if err := customerTable.Put(ctx, k, c); err != nil {
		t.Fatalf("customerTable.Put() failed: %v", err)
	}

	k = "002001"
	i = Invoice{2, 1001, t20150317111404, 166, AddressInfo{"777 Any St.", "collins", "IA", "50055"}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(i)})
	if err := customerTable.Put(ctx, k, i); err != nil {
		t.Fatalf("customerTable.Put() failed: %v", err)
	}

	k = "002002"
	i = Invoice{2, 1002, t20150317131505, 243, AddressInfo{"888 Any St.", "collins", "IA", "50055"}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(i)})
	if err := customerTable.Put(ctx, k, i); err != nil {
		t.Fatalf("customerTable.Put() failed: %v", err)
	}

	k = "002003"
	i = Invoice{2, 1004, t20150412221606, 787, AddressInfo{"999 Any St.", "collins", "IA", "50055"}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(i)})
	if err := customerTable.Put(ctx, k, i); err != nil {
		t.Fatalf("customerTable.Put() failed: %v", err)
	}

	k = "002004"
	i = Invoice{2, 1006, t20150413141707, 88, AddressInfo{"101010 Any St.", "collins", "IA", "50055"}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(i)})
	if err := customerTable.Put(ctx, k, i); err != nil {
		t.Fatalf("customerTable.Put() failed: %v", err)
	}

	k = "003"
	c = Customer{"John Steed", 3, true, AddressInfo{"100 Queen St.", "New London", "CT", "06320"}, CreditReport{Agency: CreditAgencyExperian, Report: AgencyReportExperianReport{ExperianCreditReport{ExperianRatingGood}}}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(c)})
	if err := customerTable.Put(ctx, k, c); err != nil {
		t.Fatalf("customerTable.Put() failed: %v", err)
	}

	k = "001"
	n := Numbers{byte(12), uint16(1234), uint32(5678), uint64(999888777666), int16(9876), int32(876543), int64(128), float32(3.14159), float64(2.71828182846), complex64(123.0 + 7.0i), complex128(456.789 + 10.1112i)}
	numbersEntries = append(numbersEntries, kv{k, vdl.ValueOf(n)})
	if err := numbersTable.Put(ctx, k, n); err != nil {
		t.Fatalf("numbersTable.Put() failed: %v", err)
	}

	k = "002"
	n = Numbers{byte(9), uint16(99), uint32(999), uint64(9999999), int16(9), int32(99), int64(88), float32(1.41421356237), float64(1.73205080757), complex64(9.87 + 7.65i), complex128(4.32 + 1.0i)}
	numbersEntries = append(numbersEntries, kv{k, vdl.ValueOf(n)})
	if err := numbersTable.Put(ctx, k, n); err != nil {
		t.Fatalf("numbersTable.Put() failed: %v", err)
	}

	k = "003"
	n = Numbers{byte(210), uint16(210), uint32(210), uint64(210), int16(210), int32(210), int64(210), float32(210.0), float64(210.0), complex64(210.0 + 0.0i), complex128(210.0 + 0.0i)}
	numbersEntries = append(numbersEntries, kv{k, vdl.ValueOf(n)})
	if err := numbersTable.Put(ctx, k, n); err != nil {
		t.Fatalf("numbersTable.Put() failed: %v", err)
	}

	k = "001"
	f := FooType{BarType{BazType{"FooBarBaz", TitleOrValueTypeTitle{"Vice President"}}}}
	fooEntries = append(fooEntries, kv{k, vdl.ValueOf(f)})
	if err := fooTable.Put(ctx, k, f); err != nil {
		t.Fatalf("fooTable.Put() failed: %v", err)
	}

	k = "002"
	f = FooType{BarType{BazType{"BazBarFoo", TitleOrValueTypeValue{42}}}}
	fooEntries = append(fooEntries, kv{k, vdl.ValueOf(f)})
	if err := fooTable.Put(ctx, k, f); err != nil {
		t.Fatalf("fooTable.Put() failed: %v", err)
	}

	k = "aaa"
	kid := KeyIndexData{
		[4]string{"Fee", "Fi", "Fo", "Fum"},
		[]string{"I", "smell", "the", "blood", "of", "an", "Englishman"},
		map[complex128]string{complex(1.1, 2.2): "Be he living, or be he dead"},
		map[string]struct{}{"I’ll grind his bones to mix my bread": {}},
	}
	keyIndexDataEntries = append(keyIndexDataEntries, kv{k, vdl.ValueOf(kid)})
	if err := keyIndexDataTable.Put(ctx, k, kid); err != nil {
		t.Fatalf("keyIndexDataTable.Put() failed: %v", err)
	}

	for i := 100; i < 301; i++ {
		k = fmt.Sprintf("%d", i)
		b := BigData{k}
		if err := bigTable.Put(ctx, k, b); err != nil {
			t.Fatalf("bigTable.Put() failed: %v", err)
		}
	}
}

type execSelectTest struct {
	query   string
	headers []string
	r       [][]*vdl.Value
}

type preExecFunctionTest struct {
	query   string
	headers []string
}

type execSelectErrorTest struct {
	query string
	err   error
}

func TestQueryExec(t *testing.T) {
	setup(t)
	defer cleanup()
	basic := []execSelectTest{
		{
			// Select values for all customer records.
			"select v from Customer where Type(v) like \"%.Customer\"",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[0].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[9].value},
			},
		},
		{
			// Select values where v.InvoiceNum is nil
			// Since InvoiceNum does not exist for Invoice,
			// this will return just customers.
			"select v from Customer where v.InvoiceNum is nil",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[0].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[9].value},
			},
		},
		{
			// Select values where v.InvoiceNum is nil
			// or v.Name is nil This will select all customers
			// with the former and all invoices with the latter.
			// Hence, all records are returned.
			"select v from Customer where v.InvoiceNum is nil or v.Name is nil",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[0].value},
				[]*vdl.Value{customerEntries[1].value},
				[]*vdl.Value{customerEntries[2].value},
				[]*vdl.Value{customerEntries[3].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[5].value},
				[]*vdl.Value{customerEntries[6].value},
				[]*vdl.Value{customerEntries[7].value},
				[]*vdl.Value{customerEntries[8].value},
				[]*vdl.Value{customerEntries[9].value},
			},
		},
		{
			// Select values where v.InvoiceNum is nil
			// and v.Name is nil.  Expect nothing returned.
			"select v from Customer where v.InvoiceNum is nil and v.Name is nil",
			[]string{"v"},
			[][]*vdl.Value{},
		},
		{
			// Select values where v.InvoiceNum is not nil
			// This will return just invoices.
			"select v from Customer where v.InvoiceNum is not nil",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[1].value},
				[]*vdl.Value{customerEntries[2].value},
				[]*vdl.Value{customerEntries[3].value},
				[]*vdl.Value{customerEntries[5].value},
				[]*vdl.Value{customerEntries[6].value},
				[]*vdl.Value{customerEntries[7].value},
				[]*vdl.Value{customerEntries[8].value},
			},
		},
		{
			// Select values where v.InvoiceNum is not nil
			// or v.Name is not nil. All records are returned.
			"select v from Customer where v.InvoiceNum is not nil or v.Name is not nil",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[0].value},
				[]*vdl.Value{customerEntries[1].value},
				[]*vdl.Value{customerEntries[2].value},
				[]*vdl.Value{customerEntries[3].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[5].value},
				[]*vdl.Value{customerEntries[6].value},
				[]*vdl.Value{customerEntries[7].value},
				[]*vdl.Value{customerEntries[8].value},
				[]*vdl.Value{customerEntries[9].value},
			},
		},
		{
			// Select values where v.InvoiceNum is nil and v.Name is not nil.
			// All customers are returned.
			"select v from Customer where v.InvoiceNum is nil and v.Name is not nil",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[0].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[9].value},
			},
		},
		{
			// Select values where v.InvoiceNum is not nil
			// and v.Name is not nil.  Expect nothing returned.
			"select v from Customer where v.InvoiceNum is not nil and v.Name is not nil",
			[]string{"v"},
			[][]*vdl.Value{},
		},
		{
			// Select keys & values for all customer records.
			"select k, v from Customer where Type(v) like \"%.Customer\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), customerEntries[0].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[4].key), customerEntries[4].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[9].key), customerEntries[9].value},
			},
		},
		{
			// Select keys & names for all customer records.
			"select k, v.Name from Customer where Type(v) like \"%.Customer\"",
			[]string{"k", "v.Name"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), vdl.ValueOf("John Smith")},
				[]*vdl.Value{vdl.ValueOf(customerEntries[4].key), vdl.ValueOf("Bat Masterson")},
				[]*vdl.Value{vdl.ValueOf(customerEntries[9].key), vdl.ValueOf("John Steed")},
			},
		},
		{
			// Select both customer and invoice records.
			// Customer records have Id.
			// Invoice records have CustId.
			"select v.Id, v.CustId from Customer",
			[]string{"v.Id", "v.CustId"},
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
				[]*vdl.Value{vdl.ValueOf(int64(3)), vdl.ValueOf(nil)},
			},
		},
		{
			// Select keys & values fo all invoice records.
			"select k, v from Customer where Type(v) like \"%.Invoice\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[1].key), customerEntries[1].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[2].key), customerEntries[2].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key), customerEntries[3].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key), customerEntries[5].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key), customerEntries[6].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key), customerEntries[7].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key), customerEntries[8].value},
			},
		},
		{
			// Select key, cust id, invoice number and amount for $88 invoices.
			"select k, v.CustId as ID, v.InvoiceNum as InvoiceNumber, v.Amount as Amt from Customer where Type(v) like \"%.Invoice\" and v.Amount = 88",
			[]string{"k", "ID", "InvoiceNumber", "Amt"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key), vdl.ValueOf(int64(1)), vdl.ValueOf(int64(1005)), vdl.ValueOf(int64(88))},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key), vdl.ValueOf(int64(2)), vdl.ValueOf(int64(1006)), vdl.ValueOf(int64(88))},
			},
		},
		{
			// Select keys & values for all records with a key prefix of "001".
			"select k, v from Customer where k like \"001%\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), customerEntries[0].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[1].key), customerEntries[1].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[2].key), customerEntries[2].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key), customerEntries[3].value},
			},
		},
		{
			// Select keys & values for all records with a key prefix of "002".
			"select k, v from Customer where k like \"002%\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[4].key), customerEntries[4].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key), customerEntries[5].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key), customerEntries[6].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key), customerEntries[7].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key), customerEntries[8].value},
			},
		},
		{
			// Select keys & values for all records with NOT key prefix "002%".
			"select k, v from Customer where k not like \"002%\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), customerEntries[0].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[1].key), customerEntries[1].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[2].key), customerEntries[2].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key), customerEntries[3].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[9].key), customerEntries[9].value},
			},
		},
		{
			// Select keys & values for all records with NOT key prefix "002".
			// Will be optimized to k <> "002"
			"select k, v from Customer where k not like \"002\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), customerEntries[0].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[1].key), customerEntries[1].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[2].key), customerEntries[2].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key), customerEntries[3].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key), customerEntries[5].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key), customerEntries[6].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key), customerEntries[7].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key), customerEntries[8].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[9].key), customerEntries[9].value},
			},
		},
		{
			// Select keys & values for all records with a key prefix of "001".
			// or a key prefix of "002".
			"select k, v from Customer where k like \"001%\" or k like \"002%\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), customerEntries[0].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[1].key), customerEntries[1].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[2].key), customerEntries[2].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key), customerEntries[3].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[4].key), customerEntries[4].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key), customerEntries[5].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key), customerEntries[6].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key), customerEntries[7].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key), customerEntries[8].value},
			},
		},
		{
			// Select keys & values for all records with a key prefix of "001".
			// or a key prefix of "002".
			"select k, v from Customer where k like \"002%\" or k like \"001%\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), customerEntries[0].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[1].key), customerEntries[1].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[2].key), customerEntries[2].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key), customerEntries[3].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[4].key), customerEntries[4].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key), customerEntries[5].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key), customerEntries[6].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key), customerEntries[7].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key), customerEntries[8].value},
			},
		},
		{
			// Let's play with whitespace and mixed case.
			"   sElEcT  k,  v from \n  Customer WhErE k lIkE \"002%\" oR k LiKe \"001%\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), customerEntries[0].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[1].key), customerEntries[1].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[2].key), customerEntries[2].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key), customerEntries[3].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[4].key), customerEntries[4].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key), customerEntries[5].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key), customerEntries[6].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key), customerEntries[7].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key), customerEntries[8].value},
			},
		},
		{
			// Add in a like clause that accepts all strings.
			"   sElEcT  k,  v from \n  Customer WhErE k lIkE \"002%\" oR k LiKe \"001%\" or k lIkE \"%\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), customerEntries[0].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[1].key), customerEntries[1].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[2].key), customerEntries[2].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key), customerEntries[3].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[4].key), customerEntries[4].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key), customerEntries[5].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key), customerEntries[6].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key), customerEntries[7].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key), customerEntries[8].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[9].key), customerEntries[9].value},
			},
		},
		{
			// Select id, name for customers whose last name is Masterson.
			"select v.Id as ID, v.Name as Name from Customer where Type(v) like \"%.Customer\" and v.Name like \"%Masterson\"",
			[]string{"ID", "Name"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int64(2)), vdl.ValueOf("Bat Masterson")},
			},
		},
		{
			// Select records where v.Address.City is "Collins" or type is Invoice.
			"select v from Customer where v.Address.City = \"Collins\" or Type(v) like \"%.Invoice\"",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[1].value},
				[]*vdl.Value{customerEntries[2].value},
				[]*vdl.Value{customerEntries[3].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[5].value},
				[]*vdl.Value{customerEntries[6].value},
				[]*vdl.Value{customerEntries[7].value},
				[]*vdl.Value{customerEntries[8].value},
			},
		},
		{
			// Select records where v.Address.City is "Collins" and v.InvoiceNum is not nil.
			"select v from Customer where v.Address.City = \"Collins\" and v.InvoiceNum is not nil",
			[]string{"v"},
			[][]*vdl.Value{},
		},
		{
			// Select records where v.Address.City is "Collins" and v.InvoiceNum is nil.
			"select v from Customer where v.Address.City = \"Collins\" and v.InvoiceNum is nil",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[4].value},
			},
		},
		{
			// Select customer name for customer Id (i.e., key) "001".
			"select v.Name as Name from Customer where Type(v) like \"%.Customer\" and k = \"001\"",
			[]string{"Name"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("John Smith")},
			},
		},
		{
			// Select v where v.Credit.Report.EquifaxReport.Rating = 'A'
			"select v from Customer where v.Credit.Report.EquifaxReport.Rating = 'A'",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[0].value},
			},
		},
		{
			// Select v where v.AgencyRating = "Bad"
			"select v from Customer where v.Credit.Report.EquifaxReport.Rating < 'A' or v.Credit.Report.ExperianReport.Rating = \"Bad\" or v.Credit.Report.TransUnionReport.Rating < 90",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[4].value},
			},
		},
		{
			// Select records where v.Bar.Baz.Name = "FooBarBaz"
			"select v from Foo where v.Bar.Baz.Name = \"FooBarBaz\"",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{fooEntries[0].value},
			},
		},
		{
			// Select records where v.Bar.Baz.TitleOrValue.Value = 42
			"select v from Foo where v.Bar.Baz.TitleOrValue.Value = 42",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{fooEntries[1].value},
			},
		},
		{
			// Select records where v.Bar.Baz.TitleOrValue.Title = "Vice President"
			"select v from Foo where v.Bar.Baz.TitleOrValue.Title = \"Vice President\"",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{fooEntries[0].value},
			},
		},
		{
			// Select records where v.Address.City = "Collins" or type is Invoice.
			// Limit 3
			"select v from Customer where v.Address.City = \"Collins\" or Type(v) like \"%.Invoice\" limit 3",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[1].value},
				[]*vdl.Value{customerEntries[2].value},
				[]*vdl.Value{customerEntries[3].value},
			},
		},
		{
			// Select records where v.Address.City = "Collins" or type is Invoice.
			// Offset 5
			"select v from Customer where v.Address.City = \"Collins\" or Type(v) like \"%.Invoice\" offset 5",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[6].value},
				[]*vdl.Value{customerEntries[7].value},
				[]*vdl.Value{customerEntries[8].value},
			},
		},
		{
			// Select records where v.Address.City = "Collins" is "Mountain View".
			"select v from Customer where v.Address.City = \"Mountain View\"",
			[]string{"v"},
			[][]*vdl.Value{},
		},
		{
			// Select records where v.Address.City = "Collins" or type is Invoice.
			// Offset 8
			"select v from Customer where v.Address.City = \"Collins\" or Type(v) like \"%.Invoice\" offset 8",
			[]string{"v"},
			[][]*vdl.Value{},
		},
		{
			// Select records where v.Address.City = "Collins" or type is Invoice.
			// Offset 23
			"select v from Customer where v.Address.City = \"Collins\" or Type(v) like \"%.Invoice\" offset 23",
			[]string{"v"},
			[][]*vdl.Value{},
		},
		{
			// Select records where v.Address.City = "Collins" is 84 or type is Invoice.
			// Limit 3 Offset 2
			"select v from Customer where v.Address.City = \"Collins\" or Type(v) like \"%.Invoice\" limit 3 offset 2",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[3].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[5].value},
			},
		},
		{
			// Select records where v.Address.City = "Collins" or (type is Invoice and v.InvoiceNum is not nil).
			// Limit 3 Offset 2
			"select v from Customer where v.Address.City = \"Collins\" or (Type(v) like \"%.Invoice\" and v.InvoiceNum is not nil) limit 3 offset 2",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[3].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[5].value},
			},
		},
		{
			// Select records where v.Address.City = "Collins" or (type is Invoice and v.InvoiceNum is nil).
			// Limit 3 Offset 2
			"select v from Customer where v.Address.City = \"Collins\" or (Type(v) like \"%.Invoice\" and v.InvoiceNum is nil) limit 3 offset 2",
			[]string{"v"},
			[][]*vdl.Value{},
		},
		// Test functions.
		{
			// Select invoice records where date is 2015-03-17
			"select v from Customer where Type(v) like \"%.Invoice\" and Year(v.InvoiceDate, \"America/Los_Angeles\") = 2015 and Month(v.InvoiceDate, \"America/Los_Angeles\") = 3 and Day(v.InvoiceDate, \"America/Los_Angeles\") = 17",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[5].value},
				[]*vdl.Value{customerEntries[6].value},
			},
		},
		{
			// Now will always be > 2012, so all customer records will be returned.
			"select v from Customer where Now() > Time(\"2006-01-02 MST\", \"2012-03-17 PDT\")",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[0].value},
				[]*vdl.Value{customerEntries[1].value},
				[]*vdl.Value{customerEntries[2].value},
				[]*vdl.Value{customerEntries[3].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[5].value},
				[]*vdl.Value{customerEntries[6].value},
				[]*vdl.Value{customerEntries[7].value},
				[]*vdl.Value{customerEntries[8].value},
				[]*vdl.Value{customerEntries[9].value},
			},
		},
		{
			// Select April 2015 PT invoices.
			// Note: this wouldn't work for March as daylight saving occurs March 8
			// and causes comparisons for those days to be off 1 hour.
			// It would work to use UTC -- see next test.
			"select k from Customer where Year(v.InvoiceDate, \"America/Los_Angeles\") = 2015 and Month(v.InvoiceDate, \"America/Los_Angeles\") = 4",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key)},
			},
		},
		{
			// Select March 2015 UTC invoices.
			"select k from Customer where Year(v.InvoiceDate, \"UTC\") = 2015 and Month(v.InvoiceDate, \"UTC\") = 3",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key)},
			},
		},
		{
			// Select 2015 UTC invoices.
			"select k from Customer where Year(v.InvoiceDate, \"UTC\") = 2015",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[1].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[2].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key)},
			},
		},
		{
			// Select the Mar 17 2015 11:14:04 America/Los_Angeles invoice.
			"select k from Customer where v.InvoiceDate = Time(\"2006-01-02 15:04:05 MST\", \"2015-03-17 11:14:04 PDT\")",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
			},
		},
		{
			// Select invoices in the minute Mar 17 2015 11:14 America/Los_Angeles invoice.
			"select k from Customer where Year(v.InvoiceDate, \"America/Los_Angeles\") = 2015 and Month(v.InvoiceDate, \"America/Los_Angeles\") = 3 and Day(v.InvoiceDate, \"America/Los_Angeles\") = 17 and Hour(v.InvoiceDate, \"America/Los_Angeles\") = 11 and Minute(v.InvoiceDate, \"America/Los_Angeles\") = 14",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
			},
		},
		{
			// Select invoices in the hour Mar 17 2015 11 hundred America/Los_Angeles invoice.
			"select k from Customer where Year(v.InvoiceDate, \"America/Los_Angeles\") = 2015 and Month(v.InvoiceDate, \"America/Los_Angeles\") = 3 and Day(v.InvoiceDate, \"America/Los_Angeles\") = 17 and Hour(v.InvoiceDate, \"America/Los_Angeles\") = 11",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
			},
		},
		{
			// Select invoices on the day Mar 17 2015 America/Los_Angeles invoice.
			"select k from Customer where Year(v.InvoiceDate, \"America/Los_Angeles\") = 2015 and Month(v.InvoiceDate, \"America/Los_Angeles\") = 3 and Day(v.InvoiceDate, \"America/Los_Angeles\") = 17",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key)},
			},
		},
		// Test string functions in where clause.
		{
			// Select invoices shipped to Any street -- using Lowercase.
			"select k from Customer where Type(v) like \"%.Invoice\" and Lowercase(v.ShipTo.Street) like \"%any%\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key)},
			},
		},
		{
			// Select invoices shipped to Any street -- using Uppercase.
			"select k from Customer where Type(v) like \"%.Invoice\" and Uppercase(v.ShipTo.Street) like \"%ANY%\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key)},
			},
		},
		// Select clause functions.
		// Time function
		{
			"select Time(\"2006-01-02 MST\", \"2015-07-01 PDT\") from Customer",
			[]string{"Time"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
			},
		},
		// Time function
		{
			"select Time(\"2006-01-02 15:04:05 MST\", \"2015-07-01 01:23:45 PDT\") from Customer",
			[]string{"Time"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
			},
		},
		// Lowercase function
		{
			"select Lowercase(v.Name) as name from Customer where Type(v) like \"%.Customer\"",
			[]string{"name"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("john smith")},
				[]*vdl.Value{vdl.ValueOf("bat masterson")},
				[]*vdl.Value{vdl.ValueOf("john steed")},
			},
		},
		// Uppercase function
		{
			"select Uppercase(v.Name) as NAME from Customer where Type(v) like \"%.Customer\"",
			[]string{"NAME"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("JOHN SMITH")},
				[]*vdl.Value{vdl.ValueOf("BAT MASTERSON")},
				[]*vdl.Value{vdl.ValueOf("JOHN STEED")},
			},
		},
		// Second function
		{
			"select k, Second(v.InvoiceDate, \"America/Los_Angeles\") from Customer where Type(v) like \"%.Invoice\" and k = \"002003\"",
			[]string{
				"k",
				"Second",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002003"), vdl.ValueOf(int64(6))},
			},
		},
		// Minute function
		{
			"select k, Minute(v.InvoiceDate, \"America/Los_Angeles\") from Customer where Type(v) like \"%.Invoice\" and k = \"002003\"",
			[]string{
				"k",
				"Minute",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002003"), vdl.ValueOf(int64(16))},
			},
		},
		// Hour function
		{
			"select k, Hour(v.InvoiceDate, \"America/Los_Angeles\") from Customer where Type(v) like \"%.Invoice\" and k = \"002003\"",
			[]string{
				"k",
				"Hour",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002003"), vdl.ValueOf(int64(22))},
			},
		},
		// Day function
		{
			"select k, Day(v.InvoiceDate, \"America/Los_Angeles\") from Customer where Type(v) like \"%.Invoice\" and k = \"002003\"",
			[]string{
				"k",
				"Day",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002003"), vdl.ValueOf(int64(12))},
			},
		},
		// Month function
		{
			"select k, Month(v.InvoiceDate, \"America/Los_Angeles\") from Customer where Type(v) like \"%.Invoice\" and k = \"002003\"",
			[]string{
				"k",
				"Month",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002003"), vdl.ValueOf(int64(4))},
			},
		},
		// Year function
		{
			"select k, Year(v.InvoiceDate, \"America/Los_Angeles\") from Customer where Type(v) like \"%.Invoice\" and k = \"001001\"",
			[]string{
				"k",
				"Year",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001001"), vdl.ValueOf(int64(2015))},
			},
		},
		// Nested functions
		{
			"select Year(Time(\"2006-01-02 15:04:05 MST\", \"2015-07-01 01:23:45 PDT\"), \"America/Los_Angeles\")  from Customer where Type(v) like \"%.Invoice\" and k = \"001001\"",
			[]string{"Year"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int64(2015))},
			},
		},
		// Bad arg to function.  Expression is false.
		{
			"select v from Customer where Type(v) like \"%.Invoice\" and Day(v.InvoiceDate, v.Foo) = v.InvoiceDate",
			[]string{"v"},
			[][]*vdl.Value{},
		},
		{
			// Test that all numeric types can compare to an uint64
			"select v from Numbers where v.Ui64 = v.B and v.Ui64 = v.Ui16 and v.Ui64 = v.Ui32 and v.Ui64 = v.F64 and v.Ui64 = v.I16 and v.Ui64 = v.I32 and v.Ui64 = v.I64 and v.Ui64 = v.F32 and v.Ui64 = v.C64 and v.Ui64 = v.C128",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{numbersEntries[2].value},
			},
		},
		{
			// array, list, map, set
			"select v.A[2], v.L[6], v.M[Complex(1.1, 2.2)], v.S[\"I’ll grind his bones to mix my bread\"] from KeyIndexData",
			[]string{"v.A[2]", "v.L[6]", "v.M[Complex]", "v.S[I’ll grind his bones to mix my bread]"},
			[][]*vdl.Value{[]*vdl.Value{
				vdl.ValueOf("Fo"),
				vdl.ValueOf("Englishman"),
				vdl.ValueOf("Be he living, or be he dead"),
				vdl.ValueOf(true),
			}},
		},
		{
			"select k, v.Key from BigTable where k < \"101\" or k = \"200\" or k like \"300%\"",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{svPair("100"), svPair("200"), svPair("300")},
		},
		{
			"select k, v.Key from BigTable where k like \"10_\" or k like \"20_\"",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{
				svPair("100"),
				svPair("101"),
				svPair("102"),
				svPair("103"),
				svPair("104"),
				svPair("105"),
				svPair("106"),
				svPair("107"),
				svPair("108"),
				svPair("109"),
				svPair("200"),
				svPair("201"),
				svPair("202"),
				svPair("203"),
				svPair("204"),
				svPair("205"),
				svPair("206"),
				svPair("207"),
				svPair("208"),
				svPair("209"),
			},
		},
		{
			"select k, v.Key from BigTable where k like \"_%9\"",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{
				svPair("109"),
				svPair("119"),
				svPair("129"),
				svPair("139"),
				svPair("149"),
				svPair("159"),
				svPair("169"),
				svPair("179"),
				svPair("189"),
				svPair("199"),
				svPair("209"),
				svPair("219"),
				svPair("229"),
				svPair("239"),
				svPair("249"),
				svPair("259"),
				svPair("269"),
				svPair("279"),
				svPair("289"),
				svPair("299"),
			},
		},
		{
			"select k, v.Key from BigTable where k like \"__0\"",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{
				svPair("100"),
				svPair("110"),
				svPair("120"),
				svPair("130"),
				svPair("140"),
				svPair("150"),
				svPair("160"),
				svPair("170"),
				svPair("180"),
				svPair("190"),
				svPair("200"),
				svPair("210"),
				svPair("220"),
				svPair("230"),
				svPair("240"),
				svPair("250"),
				svPair("260"),
				svPair("270"),
				svPair("280"),
				svPair("290"),
				svPair("300"),
			},
		},
		{
			"select k, v.Key from BigTable where k like \"10%\" or  k like \"20%\" or  k like \"30%\"",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{
				svPair("100"),
				svPair("101"),
				svPair("102"),
				svPair("103"),
				svPair("104"),
				svPair("105"),
				svPair("106"),
				svPair("107"),
				svPair("108"),
				svPair("109"),
				svPair("200"),
				svPair("201"),
				svPair("202"),
				svPair("203"),
				svPair("204"),
				svPair("205"),
				svPair("206"),
				svPair("207"),
				svPair("208"),
				svPair("209"),
				svPair("300"),
			},
		},
		{
			"select k, v.Key from BigTable where k like \"1__\" and  k like \"_2_\" and  k like \"__3\"",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{svPair("123")},
		},
		{
			"select k, v.Key from BigTable where (k >  \"100\" and k < \"103\") or (k > \"205\" and k < \"208\")",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{
				svPair("101"),
				svPair("102"),
				svPair("206"),
				svPair("207"),
			},
		},
		{
			"select k, v.Key from BigTable where k <=  \"100\" or k = \"101\" or k >= \"300\" or (k <> \"299\" and k not like \"300\" and k >= \"298\")",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{
				svPair("100"),
				svPair("101"),
				svPair("298"),
				svPair("300"),
			},
		},
		{
			"select k, v.Key from BigTable where k like  \"1%\" and k like \"%9\"",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{
				svPair("109"),
				svPair("119"),
				svPair("129"),
				svPair("139"),
				svPair("149"),
				svPair("159"),
				svPair("169"),
				svPair("179"),
				svPair("189"),
				svPair("199"),
			},
		},
		{
			"select k, v.Key from BigTable where k like  \"3%\" and k like \"30%\" and k like \"300%\"",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{svPair("300")},
		},
	}

	for _, test := range basic {
		headers, rs, err := db.Exec(ctx, test.query)
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
			if !reflect.DeepEqual(test.headers, headers) {
				t.Errorf("query: %s; got %v, want %v", test.query, headers, test.headers)
			}
		}
	}
}

func svPair(s string) []*vdl.Value {
	v := vdl.ValueOf(s)
	return []*vdl.Value{v, v}
}

// Use Now to verify it is "pre" executed such that all the rows
// have the same time.
func TestPreExecFunctions(t *testing.T) {
	setup(t)
	defer cleanup()
	basic := []preExecFunctionTest{
		{
			"select Now() from Customer",
			[]string{
				"Now",
			},
		},
	}

	for _, test := range basic {
		headers, rs, err := db.Exec(ctx, test.query)
		if err != nil {
			t.Errorf("query: %s; got %v, want nil", test.query, err)
		} else {
			// Check that all results are identical.
			// Collect results.
			var last []*vdl.Value
			for rs.Advance() {
				result := rs.Result()
				if last != nil && !reflect.DeepEqual(last, result) {
					t.Errorf("query: %s; got %v, want %v", test.query, result, last)
				}
				last = result
			}
			if !reflect.DeepEqual(test.headers, headers) {
				t.Errorf("query: %s; got %v, want %v", test.query, headers, test.headers)
			}
		}
	}
}

// TODO(jkline): More negative tests here (even though they are tested elsewhere)?
func TestExecErrors(t *testing.T) {
	setup(t)
	defer cleanup()
	basic := []execSelectErrorTest{
		{
			"select a from Customer",
			query.NewErrInvalidSelectField(ctx, 7),
		},
		{
			"select v from Unknown",
			// The following error text is dependent on the implementation of the query.Database interface.
			query.NewErrTableCantAccess(ctx, 14, "Unknown", errors.New("nosql.test:\"a/$/db\".Exec: Does not exist: $table:Unknown")),
		},
		{
			"select v from Customer offset -1",
			// The following error text is dependent on implementation of Database.
			query.NewErrExpected(ctx, 30, "positive integer literal"),
		},
	}

	for _, test := range basic {
		_, rs, err := db.Exec(ctx, test.query)
		if err == nil {
			err = rs.Err()
		}
		// Test both that the IDs compare and the text compares (since the offset needs to match).
		// Note: This is a little tricky because the actual error message will contain the calling
		//       module.
		wantPrefix := test.err.Error()[:strings.Index(test.err.Error(), ":")]
		wantSuffix := test.err.Error()[len(wantPrefix)+1:]
		if verror.ErrorID(err) != verror.ErrorID(test.err) || !strings.HasPrefix(err.Error(), wantPrefix) || !strings.HasSuffix(err.Error(), wantSuffix) {
			t.Errorf("query: %s; got %v, want %v", test.query, err, test.err)
		}
	}
}
