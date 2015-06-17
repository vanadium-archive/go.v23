// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exectest

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"v.io/syncbase/v23/syncbase"
	"v.io/syncbase/v23/syncbase/nosql"
	"v.io/syncbase/v23/syncbase/nosql/syncql"
	tu "v.io/syncbase/v23/syncbase/testutil"
	"v.io/v23/context"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/generic"
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
			"select v from Customer where t = \"Customer\"",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[0].value},
				[]*vdl.Value{customerEntries[4].value},
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
			"select k, v from Customer where t = \"Customer\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), customerEntries[0].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[4].key), customerEntries[4].value},
			},
		},
		{
			// Select keys & names for all customer records.
			"select k, v.Name from Customer where t = \"Customer\"",
			[]string{"k", "v.Name"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), vdl.ValueOf("John Smith")},
				[]*vdl.Value{vdl.ValueOf(customerEntries[4].key), vdl.ValueOf("Bat Masterson")},
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
			},
		},
		{
			// Select keys & values fo all invoice records.
			"select k, v from Customer where t = \"Invoice\"",
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
			"select k, v.CustId as ID, v.InvoiceNum as InvoiceNumber, v.Amount as Amt from Customer where t = \"Invoice\" and v.Amount = 88",
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
			// Select keys & values for all records with a key prefix of "001".
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
			},
		},
		{
			// Select id, name for customers whose last name is Masterson.
			"select v.Id as ID, v.Name as Name from Customer where t = \"Customer\" and v.Name like \"%Masterson\"",
			[]string{"ID", "Name"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int64(2)), vdl.ValueOf("Bat Masterson")},
			},
		},
		{
			// Select records where v.Address.City is "Collins" or type is Invoice.
			"select v from Customer where v.Address.City = \"Collins\" or t = \"Invoice\"",
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
			"select v.Name as Name from Customer where t = \"Customer\" and k = \"001\"",
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
			"select v from Customer where v.Address.City = \"Collins\" or t = \"Invoice\" limit 3",
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
			"select v from Customer where v.Address.City = \"Collins\" or t = \"Invoice\" offset 5",
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
			"select v from Customer where v.Address.City = \"Collins\" or t = \"Invoice\" offset 8",
			[]string{"v"},
			[][]*vdl.Value{},
		},
		{
			// Select records where v.Address.City = "Collins" or type is Invoice.
			// Offset 23
			"select v from Customer where v.Address.City = \"Collins\" or t = \"Invoice\" offset 23",
			[]string{"v"},
			[][]*vdl.Value{},
		},
		{
			// Select records where v.Address.City = "Collins" is 84 or type is Invoice.
			// Limit 3 Offset 2
			"select v from Customer where v.Address.City = \"Collins\" or t = \"Invoice\" limit 3 offset 2",
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
			"select v from Customer where v.Address.City = \"Collins\" or (t = \"Invoice\" and v.InvoiceNum is not nil) limit 3 offset 2",
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
			"select v from Customer where v.Address.City = \"Collins\" or (t = \"Invoice\" and v.InvoiceNum is nil) limit 3 offset 2",
			[]string{"v"},
			[][]*vdl.Value{},
		},
		// Test functions.
		{
			// Select invoice records where date is 2015-03-17
			"select v from Customer where t = \"Invoice\" and YMD(v.InvoiceDate, \"America/Los_Angeles\") = Date(\"2015-03-17 PDT\")",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[5].value},
				[]*vdl.Value{customerEntries[6].value},
			},
		},
		{
			// Now will always be > 2012, so all customer records will be returned.
			"select v from Customer where Now() > Date(\"2012-03-17 PDT\")",
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
			},
		},
		{
			// Select April 2015 PT invoices.
			// Note: this wouldn't work for March as daylight saving occurs March 8
			// and causes comparisons for those days to be off 1 hour.
			// It would work to use UTC -- see next test.
			"select k from Customer where YM(v.InvoiceDate, \"America/Los_Angeles\") = YM(Date(\"2015-04-01 PDT\"), \"America/Los_Angeles\")",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key)},
			},
		},
		{
			// Select March 2015 UTC invoices.
			"select k from Customer where YM(v.InvoiceDate, \"UTC\") = YM(Date(\"2015-03-01 UTC\"), \"UTC\")",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key)},
			},
		},
		{
			// Select 2015 UTC invoices.
			"select k from Customer where Y(v.InvoiceDate, \"UTC\") = Y(Date(\"2015-01-01 UTC\"), \"UTC\")",
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
			"select k from Customer where v.InvoiceDate = DateTime(\"2015-03-17 11:14:04 PDT\")",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
			},
		},
		{
			// Select invoices in the minute Mar 17 2015 11:14 America/Los_Angeles invoice.
			"select k from Customer where YMDHM(v.InvoiceDate, \"America/Los_Angeles\") = YMDHM(DateTime(\"2015-03-17 11:14:00 PDT\"), \"America/Los_Angeles\")",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
			},
		},
		{
			// Select invoices in the hour Mar 17 2015 11 hundred America/Los_Angeles invoice.
			"select k from Customer where YMDH(v.InvoiceDate, \"America/Los_Angeles\") = YMDH(DateTime(\"2015-03-17 11:00:00 PDT\"), \"America/Los_Angeles\")",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
			},
		},
		{
			// Select invoices on the day Mar 17 2015 America/Los_Angeles invoice.
			"select k from Customer where YMD(v.InvoiceDate, \"America/Los_Angeles\") = YMD(Date(\"2015-03-17 PDT\"), \"America/Los_Angeles\")",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key)},
			},
		},
		// Test string functions in where clause.
		{
			// Select invoices shipped to Any street -- using LowerCase.
			"select k from Customer where t = \"Invoice\" and LowerCase(v.ShipTo.Street) like \"%any%\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key)},
			},
		},
		{
			// Select invoices shipped to Any street -- using UpperCase.
			"select k from Customer where t = \"Invoice\" and UpperCase(v.ShipTo.Street) like \"%ANY%\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key)},
			},
		},
		// Select clause functions.
		// Date function
		{
			"select Date(\"2015-07-01 PDT\") from Customer",
			[]string{"Date"},
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
			},
		},
		// DateTime function
		{
			"select DateTime(\"2015-07-01 01:23:45 PDT\") from Customer",
			[]string{"DateTime"},
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
			},
		},
		// LowerCase function
		{
			"select LowerCase(v.Name) as name from Customer where t = \"Customer\"",
			[]string{"name"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("john smith")},
				[]*vdl.Value{vdl.ValueOf("bat masterson")},
			},
		},
		// UpperCase function
		{
			"select UpperCase(v.Name) as NAME from Customer where t = \"Customer\"",
			[]string{"NAME"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("JOHN SMITH")},
				[]*vdl.Value{vdl.ValueOf("BAT MASTERSON")},
			},
		},
		// YMDHMS function
		{
			"select k, YMDHMS(v.InvoiceDate, \"America/Los_Angeles\") from Customer where t = \"Invoice\" and k = \"002003\"",
			[]string{
				"k",
				"YMDHMS",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002003"), vdl.ValueOf(t2015_04_12_22_16_06)},
			},
		},
		// YMDHM function
		{
			"select k, YMDHM(v.InvoiceDate, \"America/Los_Angeles\") from Customer where t = \"Invoice\" and k = \"002003\"",
			[]string{
				"k",
				"YMDHM",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002003"), vdl.ValueOf(t2015_04_12_22_16)},
			},
		},
		// YMDH function
		{
			"select k, YMDH(v.InvoiceDate, \"America/Los_Angeles\") from Customer where t = \"Invoice\" and k = \"002003\"",
			[]string{
				"k",
				"YMDH",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002003"), vdl.ValueOf(t2015_04_12_22)},
			},
		},
		// YMD function
		{
			"select k, YMD(v.InvoiceDate, \"America/Los_Angeles\") from Customer where t = \"Invoice\" and k = \"002003\"",
			[]string{
				"k",
				"YMD",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002003"), vdl.ValueOf(t2015_04_12)},
			},
		},
		// YM function
		{
			"select k, YM(v.InvoiceDate, \"America/Los_Angeles\") from Customer where t = \"Invoice\" and k = \"002003\"",
			[]string{
				"k",
				"YM",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002003"), vdl.ValueOf(t2015_04)},
			},
		},
		// Y function
		{
			"select k, Y(v.InvoiceDate, \"America/Los_Angeles\") from Customer where t = \"Invoice\" and k = \"001001\"",
			[]string{
				"k",
				"Y",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001001"), vdl.ValueOf(t2015)},
			},
		},
		// Nested functions
		{
			"select Y(YM(YMD(YMDH(YMDHM(YMDHMS(v.InvoiceDate, \"America/Los_Angeles\"), \"America/Los_Angeles\"), \"America/Los_Angeles\"), \"America/Los_Angeles\"), \"America/Los_Angeles\"), \"America/Los_Angeles\")  from Customer where t = \"Invoice\" and k = \"001001\"",
			[]string{"Y"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(t2015)},
			},
		},
		// Bad arg to function.  Expression is false.
		{
			"select v from Customer where t = \"Invoice\" and YMD(v.InvoiceDate, v.Foo) = v.InvoiceDate",
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
			syncql.NewErrInvalidSelectField(ctx, 7),
		},
		{
			"select v from Unknown",
			// The following error text is dependent on the implementation of the query_db.Database interface.
			syncql.NewErrTableCantAccess(ctx, 14, "Unknown", errors.New("exec_test.test:\"a/db\".Exec: Does not exist: Unknown")),
		},
		{
			"select v from Customer offset -1",
			// The following error text is dependent on implementation of Database.
			syncql.NewErrExpected(ctx, 30, "positive integer literal"),
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
