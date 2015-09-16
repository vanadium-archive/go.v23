// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/syncbase/nosql/query"
	"v.io/v23/syncbase/nosql/query/internal"
	"v.io/v23/syncbase/nosql/query/internal/query_checker"
	"v.io/v23/syncbase/nosql/query/internal/query_parser"
	// TODO(sadovsky): For legacy reasons, we import all testdata identifiers into
	// the top-level namespace.
	. "v.io/v23/syncbase/nosql/query/internal/testdata"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

type mockDB struct {
	ctx    *context.T
	tables []table
}

type table struct {
	name string
	rows []kv
}

type keyValueStreamImpl struct {
	table           table
	cursor          int
	keyRanges       query.KeyRanges
	keyRangesCursor int
}

func compareKeyToLimit(key, limit string) int {
	if limit == "" || key < limit {
		return -1
	} else if key == limit {
		return 0
	} else {
		return 1
	}
}

func (kvs *keyValueStreamImpl) Advance() bool {
	for true {
		kvs.cursor++ // initialized to -1
		if kvs.cursor >= len(kvs.table.rows) {
			return false
		}
		// does it match any keyRange
		for kvs.keyRangesCursor < len(kvs.keyRanges) {
			if kvs.table.rows[kvs.cursor].key >= kvs.keyRanges[kvs.keyRangesCursor].Start && compareKeyToLimit(kvs.table.rows[kvs.cursor].key, kvs.keyRanges[kvs.keyRangesCursor].Limit) < 0 {
				return true
			}
			// Keys and keyRanges are both sorted low to high, so we can increment
			// keyRangesCursor if the keyRange.Limit is < the key.
			if compareKeyToLimit(kvs.table.rows[kvs.cursor].key, kvs.keyRanges[kvs.keyRangesCursor].Limit) > 0 {
				kvs.keyRangesCursor++
				if kvs.keyRangesCursor >= len(kvs.keyRanges) {
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

func (t table) Scan(keyRanges query.KeyRanges) (query.KeyValueStream, error) {
	var keyValueStreamImpl keyValueStreamImpl
	keyValueStreamImpl.table = t
	keyValueStreamImpl.cursor = -1
	keyValueStreamImpl.keyRanges = keyRanges
	return &keyValueStreamImpl, nil
}

func (db mockDB) GetContext() *context.T {
	return db.ctx
}

func (db mockDB) GetTable(table string) (query.Table, error) {
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
var funWithMapsTable table
var ratingsArrayTable table
var tdhApprovalsTable table
var previousRatingsTable table
var previousAddressesTable table
var manyMapsTable table
var manySetsTable table
var bigTable table

type kv struct {
	key   string
	value *vdl.Value
}

var t2015 time.Time

var t2015_04 time.Time
var t2015_04_12 time.Time
var t2015_04_12_22 time.Time
var t2015_04_12_22_16 time.Time
var t2015_04_12_22_16_06 time.Time

var t2015_07 time.Time
var t2015_07_01 time.Time
var t2015_07_01_01_23_45 time.Time

func init() {
	var shutdown v23.Shutdown
	db.ctx, shutdown = test.V23Init()
	defer shutdown()

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

	custTable.name = "Customer"
	custTable.rows = []kv{
		kv{
			"001",
			vdl.ValueOf(Customer{"John Smith", 1, true, AddressInfo{"1 Main St.", "Palo Alto", "CA", "94303"}, []AddressInfo{AddressInfo{"10 Brown St.", "Mountain View", "CA", "94043"}}, CreditReport{Agency: CreditAgencyEquifax, Report: AgencyReportEquifaxReport{EquifaxCreditReport{'A', [4]int16{87, 81, 42, 2}}}}}),
		},
		kv{
			"001001",
			vdl.ValueOf(Invoice{1, 1000, t20150122131101, 42, AddressInfo{"1 Main St.", "Palo Alto", "CA", "94303"}}),
		},
		kv{
			"001002",
			vdl.ValueOf(Invoice{1, 1003, t20150210161202, 7, AddressInfo{"2 Main St.", "Palo Alto", "CA", "94303"}}),
		},
		kv{
			"001003",
			vdl.ValueOf(Invoice{1, 1005, t20150311101303, 88, AddressInfo{"3 Main St.", "Palo Alto", "CA", "94303"}}),
		},
		kv{
			"002",
			vdl.ValueOf(Customer{"Bat Masterson", 2, true, AddressInfo{"777 Any St.", "Collins", "IA", "50055"}, []AddressInfo{AddressInfo{"19 Green St.", "Boulder", "CO", "80301"}, AddressInfo{"558 W. Orange St.", "Lancaster", "PA", "17603"}}, CreditReport{Agency: CreditAgencyTransUnion, Report: AgencyReportTransUnionReport{TransUnionCreditReport{80, map[string]int16{"2015Q2": 40, "2015Q1": 60}}}}}),
		},
		kv{
			"002001",
			vdl.ValueOf(Invoice{2, 1001, t20150317111404, 166, AddressInfo{"777 Any St.", "collins", "IA", "50055"}}),
		},
		kv{
			"002002",
			vdl.ValueOf(Invoice{2, 1002, t20150317131505, 243, AddressInfo{"888 Any St.", "collins", "IA", "50055"}}),
		},
		kv{
			"002003",
			vdl.ValueOf(Invoice{2, 1004, t20150412221606, 787, AddressInfo{"999 Any St.", "collins", "IA", "50055"}}),
		},
		kv{
			"002004",
			vdl.ValueOf(Invoice{2, 1006, t20150413141707, 88, AddressInfo{"101010 Any St.", "collins", "IA", "50055"}}),
		},
		kv{
			"003",
			vdl.ValueOf(Customer{"John Steed", 3, true, AddressInfo{"100 Queen St.", "New London", "CT", "06320"}, []AddressInfo{}, CreditReport{Agency: CreditAgencyExperian, Report: AgencyReportExperianReport{ExperianCreditReport{ExperianRatingGood, map[Tdh]struct{}{TdhTom: {}, TdhHarry: {}}, TdhTom}}}}),
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

	funWithMapsTable.name = "FunWithMaps"
	funWithMapsTable.rows = []kv{
		kv{
			"AAA",
			vdl.ValueOf(FunWithMaps{K{'a', "bbb"}, map[K]V{K{'a', "aaa"}: V{"bbb", 23.0}, K{'a', "bbb"}: V{"ccc", 14.7}},
				map[int16][]map[string]struct{}{
					23: []map[string]struct{}{
						map[string]struct{}{"foo": {}, "bar": {}},
					},
				},
			}),
		},
		kv{
			"BBB",
			vdl.ValueOf(FunWithMaps{K{'x', "zzz"}, map[K]V{K{'x', "zzz"}: V{"yyy", 17.1}, K{'r', "sss"}: V{"qqq", 7.8}},
				map[int16][]map[string]struct{}{
					42: []map[string]struct{}{
						map[string]struct{}{"great": {}, "dane": {}},
						map[string]struct{}{"german": {}, "shepard": {}},
					},
				},
			}),
		},
	}
	db.tables = append(db.tables, funWithMapsTable)

	ratingsArrayTable.name = "RatingsArray"
	ratingsArrayTable.rows = []kv{
		kv{
			"000",
			vdl.ValueOf(RatingsArray{40, 20, 10, 0}),
		},
		kv{
			"111",
			vdl.ValueOf(RatingsArray{17, 18, 19, 20}),
		},
	}
	db.tables = append(db.tables, ratingsArrayTable)

	tdhApprovalsTable.name = "TdhApprovals"
	tdhApprovalsTable.rows = []kv{
		kv{
			"yyy",
			vdl.ValueOf(map[Tdh]struct{}{TdhTom: {}}),
		},
		kv{
			"zzz",
			vdl.ValueOf(map[Tdh]struct{}{TdhDick: {}, TdhHarry: {}}),
		},
	}
	db.tables = append(db.tables, tdhApprovalsTable)

	previousRatingsTable.name = "PreviousRatings"
	previousRatingsTable.rows = []kv{
		kv{
			"x1",
			vdl.ValueOf(map[string]int16{"1Q2015": 1, "2Q2015": 2}),
		},
		kv{
			"x2",
			vdl.ValueOf(map[string]int16{"2Q2015": 3}),
		},
	}
	db.tables = append(db.tables, previousRatingsTable)

	previousAddressesTable.name = "PreviousAddresses"
	previousAddressesTable.rows = []kv{
		kv{
			"a1",
			vdl.ValueOf([]AddressInfo{
				AddressInfo{"100 Main St.", "Anytown", "CA", "94303"},
				AddressInfo{"200 Main St.", "Othertown", "IA", "51050"},
			}),
		},
		kv{
			"a2",
			vdl.ValueOf([]AddressInfo{
				AddressInfo{"500 Orange St", "Uptown", "ID", "83209"},
				AddressInfo{"200 Fulton St", "Downtown", "MT", "59001"},
			}),
		},
	}
	db.tables = append(db.tables, previousAddressesTable)

	manyMapsTable.name = "ManyMaps"
	manyMapsTable.rows = []kv{
		kv{
			"0",
			vdl.ValueOf(ManyMaps{
				map[bool]string{true: "It was the best of times,"},
				map[byte]string{10: "it was the worst of times,"},
				map[uint16]string{16: "it was the age of wisdom,"},
				map[uint32]string{32: "it was the age of foolishness,"},
				map[uint64]string{64: "it was the epoch of belief,"},
				map[int16]string{17: "it was the epoch of incredulity,"},
				map[int32]string{33: "it was the season of Light,"},
				map[int64]string{65: "it was the season of Darkness,"},
				map[float32]string{32.1: "it was the spring of hope,"},
				map[float64]string{64.2: "it was the winter of despair,"},
				map[complex64]string{(456.789 + 10.1112i): "we had everything before us,"},
				map[complex128]string{(123.456 + 11.2223i): "we had nothing before us,"},
				map[string]string{"Dickens": "we are all going direct to Heaven,"},
				map[string]map[string]string{
					"Charles": map[string]string{"Dickens": "we are all going direct to Heaven,"},
				},
				map[time.Time]string{t2015_07_01_01_23_45: "we are all going direct the other way"},
			}),
		},
	}
	db.tables = append(db.tables, manyMapsTable)

	manySetsTable.name = "ManySets"
	manySetsTable.rows = []kv{
		kv{
			"0",
			vdl.ValueOf(ManySets{
				map[bool]struct{}{true: {}},
				map[byte]struct{}{10: {}},
				map[uint16]struct{}{16: {}},
				map[uint32]struct{}{32: {}},
				map[uint64]struct{}{64: {}},
				map[int16]struct{}{17: {}},
				map[int32]struct{}{33: {}},
				map[int64]struct{}{65: {}},
				map[float32]struct{}{32.1: {}},
				map[float64]struct{}{64.2: {}},
				map[complex64]struct{}{(456.789 + 10.1112i): {}},
				map[complex128]struct{}{(123.456 + 11.2223i): {}},
				map[string]struct{}{"Dickens": {}},
				map[time.Time]struct{}{t2015_07_01_01_23_45: {}},
			}),
		},
	}
	db.tables = append(db.tables, manySetsTable)

	bigTable.name = "BigTable"

	for i := 100; i < 301; i++ {
		k := fmt.Sprintf("%d", i)
		b := vdl.ValueOf(BigData{k})
		bigTable.rows = append(bigTable.rows, kv{k, b})
	}
	db.tables = append(db.tables, bigTable)
}

type keyRangesTest struct {
	query     string
	keyRanges *query.KeyRanges
	err       error
}

type evalWhereUsingOnlyKeyTest struct {
	query  string
	key    string
	result internal.EvalWithKeyResult
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
	query   string
	headers []string
	r       [][]*vdl.Value
}

type preExecFunctionTest struct {
	query   string
	headers []string
}

type execSelectSingleRowTest struct {
	query  string
	k      string
	v      *vdl.Value
	result []*vdl.Value
}

type execSelectErrorTest struct {
	query string
	err   error
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
			"select v from Customer where Type(v) like \"%.Customer\"",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[0].value},
				[]*vdl.Value{custTable.rows[4].value},
				[]*vdl.Value{custTable.rows[9].value},
			},
		},
		{
			// Select values for all customer records.
			"select v from Customer where Type(v) not like \"%.Customer\"",
			[]string{"v"},
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
			// Select values for all customer records.
			"select Type(v) from Customer where Type(v) not like \"%Customer\"",
			[]string{"Type"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.Invoice")},
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.Invoice")},
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.Invoice")},
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.Invoice")},
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.Invoice")},
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.Invoice")},
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.Invoice")},
			},
		},
		{
			// All customers have a v.Credit with type CreditReport.
			"select v from Customer where Type(v.Credit) like \"%.CreditReport\"",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[0].value},
				[]*vdl.Value{custTable.rows[4].value},
				[]*vdl.Value{custTable.rows[9].value},
			},
		},
		{
			// Only customer "001" has an equifax report.
			"select v from Customer where Type(v.Credit.Report.EquifaxReport) like \"%.EquifaxCreditReport\"",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[0].value},
			},
		},
		{
			// Print the types of every record
			"select Type(v) from Customer",
			[]string{"Type"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.Customer")},
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.Invoice")},
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.Invoice")},
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.Invoice")},
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.Customer")},
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.Invoice")},
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.Invoice")},
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.Invoice")},
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.Invoice")},
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.Customer")},
			},
		},
		{
			// Print the types of every credit report
			"select Type(v.Credit.Report) from Customer",
			[]string{"Type"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.AgencyReport")},
				[]*vdl.Value{vdl.ValueOf(nil)},
				[]*vdl.Value{vdl.ValueOf(nil)},
				[]*vdl.Value{vdl.ValueOf(nil)},
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.AgencyReport")},
				[]*vdl.Value{vdl.ValueOf(nil)},
				[]*vdl.Value{vdl.ValueOf(nil)},
				[]*vdl.Value{vdl.ValueOf(nil)},
				[]*vdl.Value{vdl.ValueOf(nil)},
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.AgencyReport")},
			},
		},
		{
			// Print the types of every cusomer's v.Credit.Report.EquifaxReport
			"select Type(v.Credit.Report.EquifaxReport) from Customer where Type(v) like \"%.Customer\"",
			[]string{"Type"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.EquifaxCreditReport")},
				[]*vdl.Value{vdl.ValueOf(vdl.ValueOf(nil))},
				[]*vdl.Value{vdl.ValueOf(vdl.ValueOf(nil))},
			},
		},
		{
			// Select values where v.InvoiceNum is nil
			// Since InvoiceNum does not exist for Invoice,
			// this will return just customers.
			"select v from Customer where v.InvoiceNum is nil",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[0].value},
				[]*vdl.Value{custTable.rows[4].value},
				[]*vdl.Value{custTable.rows[9].value},
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
				[]*vdl.Value{custTable.rows[0].value},
				[]*vdl.Value{custTable.rows[1].value},
				[]*vdl.Value{custTable.rows[2].value},
				[]*vdl.Value{custTable.rows[3].value},
				[]*vdl.Value{custTable.rows[4].value},
				[]*vdl.Value{custTable.rows[5].value},
				[]*vdl.Value{custTable.rows[6].value},
				[]*vdl.Value{custTable.rows[7].value},
				[]*vdl.Value{custTable.rows[8].value},
				[]*vdl.Value{custTable.rows[9].value},
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
			[]string{"v"},
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
				[]*vdl.Value{custTable.rows[9].value},
			},
		},
		{
			// Select values where v.InvoiceNum is nil and v.Name is not nil.
			// All customers are returned.
			"select v from Customer where v.InvoiceNum is nil and v.Name is not nil",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[0].value},
				[]*vdl.Value{custTable.rows[4].value},
				[]*vdl.Value{custTable.rows[9].value},
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
			"select k, v from Customer where \"v.io/v23/syncbase/nosql/query/internal/testdata.Customer\" = Type(v)",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[0].key), custTable.rows[0].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[4].key), custTable.rows[4].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[9].key), custTable.rows[9].value},
			},
		},
		{
			// Select keys & names for all customer records.
			"select k, v.Name from Customer where Type(v) like \"%.Customer\"",
			[]string{"k", "v.Name"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[0].key), vdl.ValueOf("John Smith")},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[4].key), vdl.ValueOf("Bat Masterson")},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[9].key), vdl.ValueOf("John Steed")},
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
			"select k, v.CustId as ID, v.InvoiceNum as InvoiceNumber, v.Amount as Amt from Customer where Type(v) like \"%.Invoice\" and v.Amount = 88",
			[]string{"k", "ID", "InvoiceNumber", "Amt"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[3].key), vdl.ValueOf(int64(1)), vdl.ValueOf(int64(1005)), vdl.ValueOf(int64(88))},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[8].key), vdl.ValueOf(int64(2)), vdl.ValueOf(int64(1006)), vdl.ValueOf(int64(88))},
			},
		},
		{
			// Select keys & values for all records with a key prefix of "001".
			"select k, v from Customer where k like \"001%\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[0].key), custTable.rows[0].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[1].key), custTable.rows[1].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[2].key), custTable.rows[2].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[3].key), custTable.rows[3].value},
			},
		},
		{
			// Select keys & values for all records with a key prefix of "002".
			"select k, v from Customer where k like \"002%\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[4].key), custTable.rows[4].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[5].key), custTable.rows[5].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[6].key), custTable.rows[6].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[7].key), custTable.rows[7].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[8].key), custTable.rows[8].value},
			},
		},
		{
			// Select keys & values for all records with a key prefix NOT LIKE "002%".
			"select k, v from Customer where k not like \"002%\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[0].key), custTable.rows[0].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[1].key), custTable.rows[1].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[2].key), custTable.rows[2].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[3].key), custTable.rows[3].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[9].key), custTable.rows[9].value},
			},
		},
		{
			// Select keys & values for all records with a key prefix NOT LIKE "002".
			// will be optimized to k <> "002"
			"select k, v from Customer where k not like \"002\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[0].key), custTable.rows[0].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[1].key), custTable.rows[1].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[2].key), custTable.rows[2].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[3].key), custTable.rows[3].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[5].key), custTable.rows[5].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[6].key), custTable.rows[6].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[7].key), custTable.rows[7].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[8].key), custTable.rows[8].value},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[9].key), custTable.rows[9].value},
			},
		},
		{
			// Select keys & values for all records with a key prefix of "001".
			// or a key prefix of "002".
			"select k, v from Customer where k like \"001%\" or k like \"002%\"",
			[]string{"k", "v"},
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
			[]string{"k", "v"},
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
			[]string{"k", "v"},
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
			[]string{"k", "v"},
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
				[]*vdl.Value{vdl.ValueOf(custTable.rows[9].key), custTable.rows[9].value},
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
			"select k from Customer where k >= \"002001\" and k <= \"002002\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
			},
		},
		{
			"select k from Customer where k > \"002001\" and k <= \"002002\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002002")},
			},
		},
		{
			"select k from Customer where k > \"002001\" and k < \"002002\"",
			[]string{"k"},
			[][]*vdl.Value{},
		},
		{
			"select k from Customer where k > \"002001\" or k < \"002002\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("001001")},
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("001003")},
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			"select k from Customer where k <> \"002\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("001001")},
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("001003")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			"select k from Customer where k <> \"002\" or k like \"002\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("001001")},
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("001003")},
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			"select k from Customer where k <> \"002\" and k like \"002\"",
			[]string{"k"},
			[][]*vdl.Value{},
		},
		{
			"select k from Customer where k <> \"002\" and k like \"002%\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
			},
		},
		{
			"select k from Customer where k <> \"002\" and k like \"%002\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("002002")},
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
				[]*vdl.Value{custTable.rows[4].value},
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
				[]*vdl.Value{custTable.rows[0].value},
			},
		},
		{
			// Select v where v.AgencyRating = "Bad"
			"select v from Customer where v.Credit.Report.EquifaxReport.Rating < 'A' or v.Credit.Report.ExperianReport.Rating = \"Bad\" or v.Credit.Report.TransUnionReport.Rating < 90",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[4].value},
			},
		},
		{
			// Select records where v.Bar.Baz.Name = "FooBarBaz"
			"select v from Foo where v.Bar.Baz.Name = \"FooBarBaz\"",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{fooTable.rows[0].value},
			},
		},
		{
			// Select records where v.Bar.Baz.TitleOrValue.Value = 42
			"select v from Foo where v.Bar.Baz.TitleOrValue.Value = 42",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{fooTable.rows[1].value},
			},
		},
		{
			// Select records where v.Bar.Baz.TitleOrValue.Title = "Vice President"
			"select v from Foo where v.Bar.Baz.TitleOrValue.Title = \"Vice President\"",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{fooTable.rows[0].value},
			},
		},
		{
			// Select records where v.Address.City = "Collins" or type is Invoice.
			// Limit 3
			"select v from Customer where v.Address.City = \"Collins\" or Type(v) like \"%.Invoice\" limit 3",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[1].value},
				[]*vdl.Value{custTable.rows[2].value},
				[]*vdl.Value{custTable.rows[3].value},
			},
		},
		{
			// Select records where v.Address.City = "Collins" or type is Invoice.
			// Offset 5
			"select v from Customer where v.Address.City = \"Collins\" or Type(v) like \"%.Invoice\" offset 5",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[6].value},
				[]*vdl.Value{custTable.rows[7].value},
				[]*vdl.Value{custTable.rows[8].value},
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
				[]*vdl.Value{custTable.rows[3].value},
				[]*vdl.Value{custTable.rows[4].value},
				[]*vdl.Value{custTable.rows[5].value},
			},
		},
		{
			// Select records where v.Address.City = "Collins" or (type is Invoice and v.InvoiceNum is not nil).
			// Limit 3 Offset 2
			"select v from Customer where v.Address.City = \"Collins\" or (Type(v) like \"%.Invoice\" and v.InvoiceNum is not nil) limit 3 offset 2",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{custTable.rows[3].value},
				[]*vdl.Value{custTable.rows[4].value},
				[]*vdl.Value{custTable.rows[5].value},
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
				[]*vdl.Value{custTable.rows[5].value},
				[]*vdl.Value{custTable.rows[6].value},
			},
		},
		{
			// Now will always be > 2012, so all customer records will be returned.
			"select v from Customer where Now() > Time(\"2006-01-02 MST\", \"2012-03-17 PDT\")",
			[]string{"v"},
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
				[]*vdl.Value{custTable.rows[9].value},
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
				[]*vdl.Value{vdl.ValueOf(custTable.rows[7].key)},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[8].key)},
			},
		},
		{
			// Select March 2015 UTC invoices.
			"select k from Customer where Year(v.InvoiceDate, \"UTC\") = 2015 and Month(v.InvoiceDate, \"UTC\") = 3",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[3].key)},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[5].key)},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[6].key)},
			},
		},
		{
			// Select 2015 UTC invoices.
			"select k from Customer where Year(v.InvoiceDate, \"UTC\") = 2015",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[1].key)},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[2].key)},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[3].key)},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[5].key)},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[6].key)},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[7].key)},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[8].key)},
			},
		},
		{
			// Select the Mar 17 2015 11:14:04 America/Los_Angeles invoice.
			"select k from Customer where v.InvoiceDate = Time(\"2006-01-02 15:04:05 MST\", \"2015-03-17 11:14:04 PDT\")",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[5].key)},
			},
		},
		{
			// Select invoices in the minute Mar 17 2015 11:14 America/Los_Angeles invoice.
			"select k from Customer where Year(v.InvoiceDate, \"America/Los_Angeles\") = 2015 and Month(v.InvoiceDate, \"America/Los_Angeles\") = 3 and Day(v.InvoiceDate, \"America/Los_Angeles\") = 17 and Hour(v.InvoiceDate, \"America/Los_Angeles\") = 11 and Minute(v.InvoiceDate, \"America/Los_Angeles\") = 14",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[5].key)},
			},
		},
		{
			// Select invoices in the hour Mar 17 2015 11 hundred America/Los_Angeles invoice.
			"select k from Customer where Year(v.InvoiceDate, \"America/Los_Angeles\") = 2015 and Month(v.InvoiceDate, \"America/Los_Angeles\") = 3 and Day(v.InvoiceDate, \"America/Los_Angeles\") = 17 and Hour(v.InvoiceDate, \"America/Los_Angeles\") = 11",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[5].key)},
			},
		},
		{
			// Select invoices on the day Mar 17 2015 America/Los_Angeles invoice.
			"select k from Customer where Year(v.InvoiceDate, \"America/Los_Angeles\") = 2015 and Month(v.InvoiceDate, \"America/Los_Angeles\") = 3 and Day(v.InvoiceDate, \"America/Los_Angeles\") = 17",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[5].key)},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[6].key)},
			},
		},
		{
			"select Str(v.Address) from Customer where Str(v.Id) = \"1\"",
			[]string{"Str"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("v.io/v23/syncbase/nosql/query/internal/testdata.AddressInfo struct{Street string;City string;State string;Zip string}{Street: \"1 Main St.\", City: \"Palo Alto\", State: \"CA\", Zip: \"94303\"}")},
			},
		},
		// Test string functions in where clause.
		{
			"select k from Customer where Str(v.Id) = \"1\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[0].key)},
			},
		},
		{
			// Select invoices shipped to Any street -- using Lowercase.
			"select k from Customer where Type(v) like \"%.Invoice\" and Lowercase(v.ShipTo.Street) like \"%any%\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[5].key)},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[6].key)},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[7].key)},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[8].key)},
			},
		},
		{
			"select HtmlEscape(\"<a img='foo'>Foo Image</a>\") from Customer where k = \"001\"",
			[]string{"HtmlEscape"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("&lt;a img=&#39;foo&#39;&gt;Foo Image&lt;/a&gt;")},
			},
		},
		{
			"select HtmlUnescape(\"&lt;a img=&#39;foo&#39;&gt;Foo Image&lt;/a&gt;\") from Customer where k = \"001\"",
			[]string{"HtmlUnescape"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("<a img='foo'>Foo Image</a>")},
			},
		},
		{
			// Select invoices shipped to Any street -- using Uppercase.
			"select k from Customer where Type(v) like \"%.Invoice\" and Uppercase(v.ShipTo.Street) like \"%ANY%\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(custTable.rows[5].key)},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[6].key)},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[7].key)},
				[]*vdl.Value{vdl.ValueOf(custTable.rows[8].key)},
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
		// Map in selection
		{
			"select v.Credit.Report.TransUnionReport.PreviousRatings[\"2015Q2\"] from Customer where v.Name = \"Bat Masterson\"",
			[]string{"v.Credit.Report.TransUnionReport.PreviousRatings[2015Q2]"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int16(40))},
			},
		},
		// Map in selection using function as key.
		{
			"select v.Credit.Report.TransUnionReport.PreviousRatings[Uppercase(\"2015q2\")] from Customer where v.Name = \"Bat Masterson\"",
			[]string{"v.Credit.Report.TransUnionReport.PreviousRatings[Uppercase]"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int16(40))},
			},
		},
		// Map in selection using struct as key.
		{
			"select v.Map[v.Key] from FunWithMaps",
			[]string{"v.Map[v.Key]"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(V{"ccc", 14.7})},
				[]*vdl.Value{vdl.ValueOf(V{"yyy", 17.1})},
			},
		},
		// map of int16 to array of sets of strings
		{
			"select v.Confusing[23][0][\"foo\"] from FunWithMaps",
			[]string{"v.Confusing[23][0][foo]"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(true)},
				[]*vdl.Value{vdl.ValueOf(nil)},
			},
		},
		// Function using a map lookup as arg
		{
			"select Uppercase(v.B[true]) from ManyMaps",
			[]string{"Uppercase"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("IT WAS THE BEST OF TIMES,")},
			},
		},
		// Set in selection
		{
			"select v.Credit.Report.ExperianReport.TdhApprovals[\"Tom\"], v.Credit.Report.ExperianReport.TdhApprovals[\"Dick\"], v.Credit.Report.ExperianReport.TdhApprovals[\"Harry\"] from Customer where v.Name = \"John Steed\"",
			[]string{
				"v.Credit.Report.ExperianReport.TdhApprovals[Tom]",
				"v.Credit.Report.ExperianReport.TdhApprovals[Dick]",
				"v.Credit.Report.ExperianReport.TdhApprovals[Harry]",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(true), vdl.ValueOf(false), vdl.ValueOf(true)},
			},
		},
		// List in selection
		{
			"select v.PreviousAddresses[0].Street from Customer where v.Name = \"Bat Masterson\"",
			[]string{"v.PreviousAddresses[0].Street"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("19 Green St.")},
			},
		},
		// List in selection (index out of bounds)
		{
			"select v.PreviousAddresses[2].Street from Customer where v.Name = \"Bat Masterson\"",
			[]string{"v.PreviousAddresses[2].Street"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(nil)},
			},
		},
		// Array in selection
		{
			"select v.Credit.Report.EquifaxReport.FourScoreRatings[2] from Customer where v.Name = \"John Smith\"",
			[]string{"v.Credit.Report.EquifaxReport.FourScoreRatings[2]"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int16(42))},
			},
		},
		// Array in selection (using an array as the index)
		// Note: v.Credit.Report.EquifaxReport.FourScoreRatings[3] is 2
		// and v.Credit.Report.EquifaxReport.FourScoreRatings[2] is 42
		{
			"select v.Credit.Report.EquifaxReport.FourScoreRatings[v.Credit.Report.EquifaxReport.FourScoreRatings[3]] from Customer where v.Name = \"John Smith\"",
			[]string{"v.Credit.Report.EquifaxReport.FourScoreRatings[v.Credit.Report.EquifaxReport.FourScoreRatings[3]]"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int16(42))},
			},
		},
		// Array in selection (index out of bounds)
		{
			"select v.Credit.Report.EquifaxReport.FourScoreRatings[4] from Customer where v.Name = \"John Smith\"",
			[]string{"v.Credit.Report.EquifaxReport.FourScoreRatings[4]"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(nil)},
			},
		},
		// Map in where expression
		{
			"select v.Name from Customer where v.Credit.Report.TransUnionReport.PreviousRatings[\"2015Q2\"] = 40",
			[]string{"v.Name"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("Bat Masterson")},
			},
		},
		// Set in where expression (convert string to enum to do lookup)
		{
			"select v.Name from Customer where v.Credit.Report.ExperianReport.TdhApprovals[\"Tom\"] = true",
			[]string{
				"v.Name",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("John Steed")},
			},
		},
		// Negative case: Set in where expression (convert string to enum to do lookup)
		{
			"select v.Name from Customer where v.Credit.Report.ExperianReport.TdhApprovals[\"Dick\"] = true",
			[]string{
				"v.Name",
			},
			[][]*vdl.Value{},
		},
		// Set in where expression (use another field as lookup key)
		// Find all customers where experian auditor was also an approver.
		{
			"select v.Name from Customer where v.Credit.Report.ExperianReport.TdhApprovals[v.Credit.Report.ExperianReport.Auditor] = true",
			[]string{
				"v.Name",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("John Steed")},
			},
		},
		// List in where expression
		{
			"select v.Name from Customer where v.PreviousAddresses[0].Street = \"19 Green St.\"",
			[]string{"v.Name"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("Bat Masterson")},
			},
		},
		// List in where expression (index out of bounds)
		{
			"select v.Name from Customer where v.PreviousAddresses[10].Street = \"19 Green St.\"",
			[]string{"v.Name"},
			[][]*vdl.Value{},
		},
		// Array in where expression
		{
			"select v.Name from Customer where v.Credit.Report.EquifaxReport.FourScoreRatings[2] = 42",
			[]string{"v.Name"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("John Smith")},
			},
		},
		// Array in where expression (using another field as index)
		// Note: v.Credit.Report.EquifaxReport.FourScoreRatings[3] is 2
		// and v.Credit.Report.EquifaxReport.FourScoreRatings[2] is 42
		{
			"select v.Name from Customer where v.Credit.Report.EquifaxReport.FourScoreRatings[v.Credit.Report.EquifaxReport.FourScoreRatings[3]] = 42",
			[]string{"v.Name"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("John Smith")},
			},
		},
		// Array in where expression (index out of bounds, using another field as index)
		{
			"select v.Name from Customer where v.Credit.Report.EquifaxReport.FourScoreRatings[v.Credit.Report.EquifaxReport.FourScoreRatings[2]] = 42",
			[]string{"v.Name"},
			[][]*vdl.Value{},
		},
		// Array in select and where expressions (top level value is the array)
		{
			"select v[0], v[1], v[2], v[3] from RatingsArray where v[0] = 40",
			[]string{"v[0]", "v[1]", "v[2]", "v[3]"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int16(40)), vdl.ValueOf(int16(20)), vdl.ValueOf(int16(10)), vdl.ValueOf(int16(0))},
			},
		},
		// List in select and where expressions (top level value is the list)
		{
			"select v[-1].City, v[0].City, v[1].City, v[2].City from PreviousAddresses where v[1].Street = \"200 Main St.\"",
			[]string{"v[-1].City", "v[0].City", "v[1].City", "v[2].City"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(nil), vdl.ValueOf("Anytown"), vdl.ValueOf("Othertown"), vdl.ValueOf(nil)},
			},
		},
		// Set in select and where expressions (top level value is the set)
		{
			"select v[\"Tom\"], v[\"Dick\"], v[\"Harry\"] from TdhApprovals where v[\"Dick\"] = true",
			[]string{"v[Tom]", "v[Dick]", "v[Harry]"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(false), vdl.ValueOf(true), vdl.ValueOf(true)},
			},
		},
		// Map in select and where expressions (top level value is the map)
		{
			"select v[\"1Q2015\"], v[\"2Q2015\"] from PreviousRatings where v[\"2Q2015\"] = 3",
			[]string{"v[1Q2015]", "v[2Q2015]"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(nil), vdl.ValueOf(int16(3))},
			},
		},
		// Test lots of types as map keys
		{
			"select v.B[true], v.By[10], v.U16[16], v.U32[32], v.U64[64], v.I16[17], v.I32[33], v.I64[65], v.F32[32.1], v.F64[64.2], v.C64[Complex(456.789, 10.1112)], v.C128[Complex(123.456, 11.2223)], v.S[\"Dickens\"], v.Ms[\"Charles\"][\"Dickens\"], v.T[Time(\"2006-01-02 15:04:05 MST\", \"2015-07-01 01:23:45 PDT\")] from ManyMaps",
			[]string{"v.B[true]", "v.By[10]", "v.U16[16]", "v.U32[32]", "v.U64[64]", "v.I16[17]", "v.I32[33]", "v.I64[65]", "v.F32[32.1]", "v.F64[64.2]", "v.C64[Complex]", "v.C128[Complex]", "v.S[Dickens]", "v.Ms[Charles][Dickens]", "v.T[Time]"},
			[][]*vdl.Value{
				[]*vdl.Value{
					vdl.ValueOf("It was the best of times,"),
					vdl.ValueOf("it was the worst of times,"),
					vdl.ValueOf("it was the age of wisdom,"),
					vdl.ValueOf("it was the age of foolishness,"),
					vdl.ValueOf("it was the epoch of belief,"),
					vdl.ValueOf("it was the epoch of incredulity,"),
					vdl.ValueOf("it was the season of Light,"),
					vdl.ValueOf("it was the season of Darkness,"),
					vdl.ValueOf("it was the spring of hope,"),
					vdl.ValueOf("it was the winter of despair,"),
					vdl.ValueOf("we had everything before us,"),
					vdl.ValueOf("we had nothing before us,"),
					vdl.ValueOf("we are all going direct to Heaven,"),
					vdl.ValueOf("we are all going direct to Heaven,"),
					vdl.ValueOf("we are all going direct the other way"),
				},
			},
		},
		// Test lots of types as set keys
		{
			"select v.B[true], v.By[10], v.U16[16], v.U32[32], v.U64[64], v.I16[17], v.I32[33], v.I64[65], v.F32[32.1], v.F64[64.2], v.C64[Complex(456.789, 10.1112)], v.C128[Complex(123.456, 11.2223)], v.S[\"Dickens\"], v.T[Time(\"2006-01-02 15:04:05 MST\", \"2015-07-01 01:23:45 PDT\")] from ManySets",
			[]string{"v.B[true]", "v.By[10]", "v.U16[16]", "v.U32[32]", "v.U64[64]", "v.I16[17]", "v.I32[33]", "v.I64[65]", "v.F32[32.1]", "v.F64[64.2]", "v.C64[Complex]", "v.C128[Complex]", "v.S[Dickens]", "v.T[Time]"},
			[][]*vdl.Value{
				[]*vdl.Value{
					vdl.ValueOf(true),
					vdl.ValueOf(true),
					vdl.ValueOf(true),
					vdl.ValueOf(true),
					vdl.ValueOf(true),
					vdl.ValueOf(true),
					vdl.ValueOf(true),
					vdl.ValueOf(true),
					vdl.ValueOf(true),
					vdl.ValueOf(true),
					vdl.ValueOf(true),
					vdl.ValueOf(true),
					vdl.ValueOf(true),
					vdl.ValueOf(true),
				},
			},
		},
		{
			"select k, v.Key from BigTable where k < \"101\" or k = \"200\" or k like \"300%\"",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{svPair("100"), svPair("200"), svPair("300")},
		},
		{
			"select k, v.Key from BigTable where \"101\" > k or \"200\" = k or k like \"300%\"",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{svPair("100"), svPair("200"), svPair("300")},
		},
		{
			"select k, v.Key from BigTable where k is nil",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{},
		},
		{
			"select k, v.Key from BigTable where k is not nil and \"103\" = v.Key",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{svPair("103")},
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
			"select k, v.Key from BigTable where ( \"100\" < k and \"103\" > k) or (\"205\" < k and \"208\" > k)",
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
			"select k, v.Key from BigTable where \"100\" >= k or \"101\" = k or \"300\" <= k or (\"299\" <> k and k not like \"300\" and \"298\" <= k)",
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
		{
			"select k, v.Key from BigTable where \"110\" > k",
			[]string{"k", "v.Key"},
			svPairs(100, 109),
		},
		{
			"select k, v.Key from BigTable where \"110\" < k and \"205\" > k",
			[]string{"k", "v.Key"},
			svPairs(111, 204),
		},
		{
			"select k, v.Key from BigTable where \"110\" <= k and \"205\" >= k",
			[]string{"k", "v.Key"},
			svPairs(110, 205),
		},
		{
			"select k, v.Key from BigTable where k is nil",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{},
		},
		{
			"select k, v.Key from BigTable where k is not nil",
			[]string{"k", "v.Key"},
			svPairs(100, 300),
		},
		{
			"select k, v.Key from BigTable where k <> k",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{},
		},
		{
			"select k, v.Key from BigTable where k < k",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{},
		},
		{
			"select k, v.Key from BigTable where k > k",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{},
		},
		{
			"select k, v.Key from BigTable where k = k",
			[]string{"k", "v.Key"},
			svPairs(100, 300),
		},
		{
			"select k, v.Key from BigTable where k <= k",
			[]string{"k", "v.Key"},
			svPairs(100, 300),
		},
		{
			"select k, v.Key from BigTable where k >= k",
			[]string{"k", "v.Key"},
			svPairs(100, 300),
		},
		{
			"select k, v.Key from BigTable where k = v.Key",
			[]string{"k", "v.Key"},
			svPairs(100, 300),
		},
		{
			"select k, v.Key from BigTable where v.Key = k",
			[]string{"k", "v.Key"},
			svPairs(100, 300),
		},
		{
			"select k, v.Key from BigTable where v.Key = k",
			[]string{"k", "v.Key"},
			svPairs(100, 300),
		},
		{
			"select k, v.Key from BigTable where k <> v.key",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{},
		},
		{
			"select k, v.Key from BigTable where v.key <> k",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{},
		},
		{
			"select k, v.Key from BigTable where k < v.key",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{},
		},
		{
			"select k, v.Key from BigTable where v.key < k",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{},
		},
		{
			"select k, v.Key from BigTable where k > v.key",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{},
		},
		{
			"select k, v.Key from BigTable where v.key > k",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{},
		},
		{
			"select k, v.Key from BigTable where k >= v.Key",
			[]string{"k", "v.Key"},
			svPairs(100, 300),
		},
		{
			"select k, v.Key from BigTable where v.Key >= k",
			[]string{"k", "v.Key"},
			svPairs(100, 300),
		},
		{
			"select k, v.Key from BigTable where k <= v.Key",
			[]string{"k", "v.Key"},
			svPairs(100, 300),
		},
		{
			"select k, v.Key from BigTable where v.Key <= k",
			[]string{"k", "v.Key"},
			svPairs(100, 300),
		},
		{
			// Split on .
			"select Split(Type(v), \".\") from Customer where k = \"001\"",
			[]string{"Split"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf([]string{"v", "io/v23/syncbase/nosql/query/internal/testdata", "Customer"})},
			},
		},
		{
			// Split on /
			"select Split(Type(v), \"/\") from Customer where k = \"001\"",
			[]string{"Split"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf([]string{"v.io", "v23", "syncbase", "nosql", "query", "internal", "testdata.Customer"})},
			},
		},
		{
			// Split on /, Len of array
			"select Len(Split(Type(v), \"/\")) from Customer where k = \"001\"",
			[]string{"Len"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int64(7))},
			},
		},
		{
			// Split on empty string, Len of array
			// Split with sep == empty string splits on chars.
			"select Len(Split(Type(v), \"\")) from Customer where k = \"001\"",
			[]string{"Len"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int64(len("v.io/v23/syncbase/nosql/query/internal/testdata.Customer")))},
			},
		},
		{
			// Len of string, list and struct.
			// returns len of string, elements in list and nil for struct.
			"select Len(v.Name), Len(v.PreviousAddresses), Len(v.Credit) from Customer where k = \"002\"",
			[]string{"Len", "Len", "Len"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int64(13)), vdl.ValueOf(int64(2)), vdl.ValueOf(nil)},
			},
		},
		{
			// Len of set
			"select Len(v.Credit.Report.ExperianReport.TdhApprovals) from Customer where Type(v) like \"%.Customer\" and k = \"003\"",
			[]string{"Len"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int64(2))},
			},
		},
		{
			// Len of map
			"select Len(v.Map) from FunWithMaps",
			[]string{"Len"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int64(2))},
				[]*vdl.Value{vdl.ValueOf(int64(2))},
			},
		},
		{
			// Real
			"select k, Real(v.C64) from Numbers where v.C64 = Complex(123, 7)",
			[]string{"k", "Real"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001"), vdl.ValueOf(float64(123.0))},
			},
		},
		{
			// Real
			"select k, Real(v.C128) from Numbers where v.C128 = Complex(456.789, 10.1112)",
			[]string{"k", "Real"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001"), vdl.ValueOf(float64(456.789))},
			},
		},
		{
			// Ceiling
			"select k, Ceiling(v.F64) from Numbers where k = \"001\"",
			[]string{"k", "Ceiling"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001"), vdl.ValueOf(float64(3.0))},
			},
		},
		{
			// Floor
			"select k, Floor(v.F64) from Numbers where k = \"001\"",
			[]string{"k", "Floor"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001"), vdl.ValueOf(float64(2.0))},
			},
		},
		{
			// Truncate
			"select k, Truncate(v.F64) from Numbers where k = \"001\"",
			[]string{"k", "Truncate"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001"), vdl.ValueOf(float64(2.0))},
			},
		},
		{
			// Log
			"select k, Log(v.F64) from Numbers where k = \"001\"",
			[]string{"k", "Log"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001"), vdl.ValueOf(float64(1.0000000000003513))},
			},
		},
		{
			// Log10
			"select k, Log10(v.F64) from Numbers where k = \"001\"",
			[]string{"k", "Log10"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001"), vdl.ValueOf(float64(0.43429448190340436))},
			},
		},
		{
			// Pow
			"select k, Pow(10.0, 4.0) from Numbers where k = \"001\"",
			[]string{"k", "Pow"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001"), vdl.ValueOf(float64(10000))},
			},
		},
		{
			// Pow10
			"select k, Pow10(5) from Numbers where k = \"001\"",
			[]string{"k", "Pow10"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001"), vdl.ValueOf(float64(100000))},
			},
		},
		{
			// Mod
			"select k, Mod(v.F32, v.F64) from Numbers where k = \"001\"",
			[]string{"k", "Mod"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001"), vdl.ValueOf(float64(0.42330828994820324))},
			},
		},
		{
			// Remainder
			"select k, Remainder(v.F32, v.F64) from Numbers where k = \"001\"",
			[]string{"k", "Remainder"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001"), vdl.ValueOf(float64(0.42330828994820324))},
			},
		},
		{
			// Sprintf
			"select Sprintf(\"%d, %d, %d, %d, %d, %d, %d, %g, %g, %g, %g\", v.B, v.Ui16, v.Ui32, v.Ui64, v.I16, v.I32, v.I64, v.F32, v.F64, v.C64, v.C128) from Numbers where k = \"001\"",
			[]string{"Sprintf"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("12, 1234, 5678, 999888777666, 9876, 876543, 128, 3.141590118408203, 2.71828182846, (123+7i), (456.789+10.1112i)")},
			},
		},
		{
			// Sprintf
			"select Sprintf(\"%s, %s %s\", v.Address.City, v.Address.State, v.Address.Zip) from Customer where v.Name = \"John Smith\"",
			[]string{"Sprintf"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("Palo Alto, CA 94303")},
			},
		},
		{
			// StrCat
			"select StrCat(v.Address.City, v.Address.State) from Customer where v.Name = \"John Smith\"",
			[]string{"StrCat"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("Palo AltoCA")},
			},
		},
		{
			// StrCat
			"select StrCat(v.Address.City, \", \", v.Address.State) from Customer where v.Name = \"John Smith\"",
			[]string{"StrCat"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("Palo Alto, CA")},
			},
		},
		{
			// StrCat
			"select StrCat(v.Address.City, \"42\") from Customer where v.Name = \"John Smith\"",
			[]string{"StrCat"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("Palo Alto42")},
			},
		},
		{
			// StrIndex
			"select StrIndex(v.Address.City, \"lo\") from Customer where v.Name = \"John Smith\"",
			[]string{"StrIndex"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int64(2))},
			},
		},
		{
			// StrLastIndex
			"select StrLastIndex(v.Address.City, \"l\") from Customer where v.Name = \"John Smith\"",
			[]string{"StrLastIndex"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int64(6))},
			},
		},
		{
			// StrRepeat
			"select StrRepeat(v.Address.City, 3) from Customer where v.Name = \"John Smith\"",
			[]string{"StrRepeat"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("Palo AltoPalo AltoPalo Alto")},
			},
		},
		{
			// StrReplace
			"select StrReplace(v.Address.City, \"Palo\", \"Shallow\") from Customer where v.Name = \"John Smith\"",
			[]string{"StrReplace"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("Shallow Alto")},
			},
		},
		{
			// Trim
			"select Trim(\"   Foo     \") from Customer where v.Name = \"John Smith\"",
			[]string{"Trim"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("Foo")},
			},
		},
		{
			// TrimLeft
			"select TrimLeft(\"   Foo     \") from Customer where v.Name = \"John Smith\"",
			[]string{"TrimLeft"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("Foo     ")},
			},
		},
		{
			// TrimRight
			"select TrimRight(\"   Foo     \") from Customer where v.Name = \"John Smith\"",
			[]string{"TrimRight"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("   Foo")},
			},
		},
	}

	for _, test := range basic {
		headers, rs, err := internal.Exec(db, test.query)
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

// Genearate k,v pairs for start to finish (*INCLUSIVE*)
func svPairs(start, finish int64) [][]*vdl.Value {
	retVal := [][]*vdl.Value{}
	for i := start; i <= finish; i++ {
		v := vdl.ValueOf(fmt.Sprintf("%d", i))
		retVal = append(retVal, []*vdl.Value{v, v})
	}
	return retVal
}

// Use Now to verify it is "pre" executed such that all the rows
// have the same time.
func TestPreExecFunctions(t *testing.T) {
	basic := []preExecFunctionTest{
		{
			"select Now() from Customer",
			[]string{
				"Now",
			},
		},
	}

	for _, test := range basic {
		headers, rs, err := internal.Exec(db, test.query)
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

func appendZeroByte(start string) string {
	limit := []byte(start)
	limit = append(limit, 0)
	return string(limit)

}

func plusOne(start string) string {
	limit := []byte(start)
	for len(limit) > 0 {
		if limit[len(limit)-1] == 255 {
			limit = limit[:len(limit)-1] // chop off trailing \x00
		} else {
			limit[len(limit)-1] += 1 // add 1
			break                    // no carry
		}
	}
	return string(limit)
}

func TestKeyRanges(t *testing.T) {
	basic := []keyRangesTest{
		{
			// Need all keys
			"select k, v from Customer",
			&query.KeyRanges{
				query.KeyRange{Start: "", Limit: ""},
			},
			nil,
		},
		{
			// Keys 001 or 003
			"   select  k,  v from Customer where k = \"001\" or k = \"003\"",
			&query.KeyRanges{
				query.KeyRange{Start: "001", Limit: appendZeroByte("001")},
				query.KeyRange{Start: "003", Limit: appendZeroByte("003")},
			},
			nil,
		},
		{
			// Keys 001 and 003 (resulting in no keys)
			"   select  k,  v from Customer where k = \"001\" and k = \"003\"",
			&query.KeyRanges{},
			nil,
		},
		{
			// Need all keys
			"select  k,  v from Customer where k like \"%\" or k like \"001%\" or k like \"002%\"",
			&query.KeyRanges{
				query.KeyRange{Start: "", Limit: ""},
			},
			nil,
		},
		{
			// Need all keys, likes in where clause in different order
			"select  k,  v from Customer where k like \"002%\" or k like \"001%\" or k like \"%\"",
			&query.KeyRanges{
				query.KeyRange{Start: "", Limit: ""},
			},
			nil,
		},
		{
			// All selected rows will have key prefix of "abc".
			"select k, v from Customer where Type(v) like \"%.Foo.Bar\" and k like \"abc%\"",
			&query.KeyRanges{
				query.KeyRange{Start: "abc", Limit: plusOne("abc")},
			},
			nil,
		},
		{
			// Need all keys
			"select k, v from Customer where Type(v) like \"%.Foo.Bar\" or k like \"abc%\"",
			&query.KeyRanges{
				query.KeyRange{Start: "", Limit: ""},
			},
			nil,
		},
		{
			// Need all keys
			"select k, v from Customer where k like \"abc%\" or v.zip = \"94303\"",
			&query.KeyRanges{
				query.KeyRange{Start: "", Limit: ""},
			},
			nil,
		},
		{
			// All selected rows will have key prefix of "foo".
			"select k, v from Customer where Type(v) like \"%.Foo.Bar\" and k like \"foo_bar\"",
			&query.KeyRanges{
				query.KeyRange{Start: "foo", Limit: plusOne("foo")},
			},
			nil,
		},
		{
			// All selected rows will have key == "baz" or prefix of "foo".
			"select k, v from Customer where k like \"foo_bar\" or k = \"baz\"",
			&query.KeyRanges{
				query.KeyRange{Start: "baz", Limit: appendZeroByte("baz")},
				query.KeyRange{Start: "foo", Limit: plusOne("foo")},
			},
			nil,
		},
		{
			// All selected rows will have key == "fo" or prefix of "foo".
			"select k, v from Customer where k like \"foo_bar\" or k = \"fo\"",
			&query.KeyRanges{
				query.KeyRange{Start: "fo", Limit: appendZeroByte("fo")},
				query.KeyRange{Start: "foo", Limit: plusOne("foo")},
			},
			nil,
		},
		{
			// All selected rows will have prefix of "fo".
			// k == foo is a subset of above prefix
			"select k, v from Customer where k like \"fo_bar\" or k = \"foo\"",
			&query.KeyRanges{
				query.KeyRange{Start: "fo", Limit: plusOne("fo")},
			},
			nil,
		},
		{
			// All selected rows will have key prefix of "foo".
			"select k, v from Customer where k like \"foo%bar\"",
			&query.KeyRanges{
				query.KeyRange{Start: "foo", Limit: plusOne("foo")},
			},
			nil,
		},
		{
			// Select "foo\bar" row.
			"select k, v from Customer where k like \"foo\\\\bar\"",
			&query.KeyRanges{
				query.KeyRange{Start: "foo\\bar", Limit: appendZeroByte("foo\\bar")},
			},
			nil,
		},
		{
			// Select "foo%bar" row.
			"select k, v from Customer where k like \"foo\\%bar\"",
			&query.KeyRanges{
				query.KeyRange{Start: "foo%bar", Limit: appendZeroByte("foo%bar")},
			},
			nil,
		},
		{
			// Select "foo\%bar" row.
			"select k, v from Customer where k like \"foo\\\\\\%bar\"",
			&query.KeyRanges{
				query.KeyRange{Start: "foo\\%bar", Limit: appendZeroByte("foo\\%bar")},
			},
			nil,
		},
		{
			// Need all keys
			"select k, v from Customer where k like \"%foo\"",
			&query.KeyRanges{
				query.KeyRange{Start: "", Limit: ""},
			},
			nil,
		},
		{
			// Need all keys
			"select k, v from Customer where k like \"_foo\"",
			&query.KeyRanges{
				query.KeyRange{Start: "", Limit: ""},
			},
			nil,
		},
		{
			// Select "foo_bar" row.
			"select k, v from Customer where k like \"foo\\_bar\"",
			&query.KeyRanges{
				query.KeyRange{Start: "foo_bar", Limit: appendZeroByte("foo_bar")},
			},
			nil,
		},
		{
			// Select "foobar%" row.
			"select k, v from Customer where k like \"foobar\\%\"",
			&query.KeyRanges{
				query.KeyRange{Start: "foobar%", Limit: appendZeroByte("foobar%")},
			},
			nil,
		},
		{
			// Select "foobar_" row.
			"select k, v from Customer where k like \"foobar\\_\"",
			&query.KeyRanges{
				query.KeyRange{Start: "foobar_", Limit: appendZeroByte("foobar_")},
			},
			nil,
		},
		{
			// Select "\%_" row.
			"select k, v from Customer where k like \"\\\\\\%\\_\"",
			&query.KeyRanges{
				query.KeyRange{Start: "\\%_", Limit: appendZeroByte("\\%_")},
			},
			nil,
		},
		{
			// Select "%_abc\" row.
			"select k, v from Customer where k = \"%_abc\\\"",
			&query.KeyRanges{
				query.KeyRange{Start: "%_abc\\", Limit: appendZeroByte("%_abc\\")},
			},
			nil,
		},
	}

	for _, test := range basic {
		s, synErr := query_parser.Parse(db, test.query)
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
					keyRanges := query_checker.CompileKeyRanges(sel.Where)
					if !reflect.DeepEqual(test.keyRanges, keyRanges) {
						t.Errorf("query: %s;\nGOT  %v\nWANT %v", test.query, keyRanges, test.keyRanges)
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
			internal.INCLUDE,
		},
		{
			// Row will be rejected using only the key.
			"select k, v from Customer where k like \"abc\"",
			"abcd",
			internal.EXCLUDE,
		},
		{
			// Need value to determine if row should be selected.
			"select k, v from Customer where k = \"abc\" or v.zip = \"94303\"",
			"abcd",
			internal.FETCH_VALUE,
		},
		{
			// Need value (i.e., its type) to determine if row should be selected.
			"select k, v from Customer where k = \"xyz\" or Type(v) like \"%.foo.Bar\"",
			"wxyz",
			internal.FETCH_VALUE,
		},
		{
			// Although value is in where clause, it is not needed to reject row.
			"select k, v from Customer where k = \"abcd\" and v.zip = \"94303\"",
			"abcde",
			internal.EXCLUDE,
		},
	}

	for _, test := range basic {
		s, synErr := query_parser.Parse(db, test.query)
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
					result := internal.EvalWhereUsingOnlyKey(db, &sel, test.key)
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
			"select k, v from Customer where Type(v) = \"v.io/v23/syncbase/nosql/query/internal/testdata.Customer\"",
			custTable.rows[0].key, custTable.rows[0].value, true,
		},
		{
			"select k, v from Customer where Type(v) like \"%.Customer\"",
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
			"select k, v from Numbers where Str(v.C64) = \"(123+7i)\"",
			numTable.rows[0].key, numTable.rows[0].value, true,
		},
		{
			"select k, v from Numbers where v.C64 = Complex(123, 7)",
			numTable.rows[0].key, numTable.rows[0].value, true,
		},
		{
			"select k, v from Numbers where Str(v.C128) = \"(456.789+10.1112i)\"",
			numTable.rows[0].key, numTable.rows[0].value, true,
		},
		{
			"select k, v from Numbers where v.C128 = Complex(456.789, 10.1112)",
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
			"select v from Customer where Str(v.CustId) = \"1\"",
			numTable.rows[0].key, custTable.rows[1].value, true,
		},
		{
			// Convert bool to string
			"select v from Customer where Str(v.Active) = \"true\"",
			numTable.rows[0].key, custTable.rows[0].value, true,
		},
		{
			// Compare bool
			"select v from Customer where v.Active = true",
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
			"select v from Numbers where Str(v.C128) = \"(456.789+10.1112i)\"",
			numTable.rows[0].key, numTable.rows[0].value, true,
		},
	}

	for _, test := range basic {
		s, synErr := query_parser.Parse(db, test.query)
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
					result := internal.Eval(db, test.k, test.v, sel.Where.Expr)
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
			"select k, v from Customer where Type(v) like \"%.Customer\"",
			"123456", custTable.rows[0].value,
			[]*vdl.Value{
				vdl.ValueOf("123456"),
				custTable.rows[0].value,
			},
		},
		{
			"select k, v, v.Name, v.Id, v.Active, v.Credit.Agency, v.Credit.Report.EquifaxReport.Rating, v.Address.Street, v.Address.City, v.Address.State, v.Address.Zip from Customer where Type(v) like \"%.Customer\"",
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
		s, synErr := query_parser.Parse(db, test.query)
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
					result := internal.ComposeProjection(db, test.k, test.v, sel.Select)
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
			"select k, v from Customer where Type(v) like \"%.Customer\"",
			"123456", custTable.rows[0].value,
			[]*vdl.Value{
				vdl.ValueOf("123456"),
				custTable.rows[0].value,
			},
		},
		{
			"select k, v from Customer where Type(v) like \"%.Customer\" and k like \"123%\"",
			"123456", custTable.rows[0].value,
			[]*vdl.Value{
				vdl.ValueOf("123456"),
				custTable.rows[0].value,
			},
		},
		{
			"select k, v from Customer where Type(v) like \"%.Invoice\" and k like \"123%\"",
			"123456", custTable.rows[0].value,
			[]*vdl.Value{},
		},
		{
			"select k, v from Customer where Type(v) like \"%.Customer\" and k like \"456%\"",
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
			"select k, v, v.Name, v.Id, v.Active, v.Credit.Report.EquifaxReport.Rating, v.Credit.Report.ExperianReport.Rating, v.Credit.Report.TransUnionReport.Rating, v.Address.Street, v.Address.City, v.Address.State, v.Address.Zip from Customer where Type(v) like \"%.Customer\"",
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
		s, synErr := query_parser.Parse(db, test.query)
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
					result := internal.ExecSelectSingleRow(db, test.k, test.v, &sel)
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
			query.NewErrInvalidSelectField(db.GetContext(), 7),
		},
		{
			"select v from Unknown",
			// The following error text is dependent on implementation of Database.
			query.NewErrTableCantAccess(db.GetContext(), 14, "Unknown", errors.New("No such table: Unknown.")),
		},
		{
			"select v from Customer offset -1",
			// The following error text is dependent on implementation of Database.
			query.NewErrExpected(db.GetContext(), 30, "positive integer literal"),
		},
		{
			"select k, v.Key from BigTable where 110 <= k and 205 >= k",
			query.NewErrKeyExpressionLiteral(db.GetContext(), 36),
		},
		{
			"select k, v.Key from BigTable where Type(k) = \"BigData\"",
			query.NewErrArgMustBeField(db.GetContext(), 41),
		},
		{
			"select v from Customer where Len(10) = 3",
			query.NewErrFunctionLenInvalidArg(db.GetContext(), 33),
		},
		{
			"select K from Customer where Type(v) = \"Invoice\"",
			query.NewErrDidYouMeanLowercaseK(db.GetContext(), 7),
		},
		{
			"select V from Customer where Type(v) = \"Invoice\"",
			query.NewErrDidYouMeanLowercaseV(db.GetContext(), 7),
		},
		{
			"select k from Customer where K = \"001\"",
			query.NewErrDidYouMeanLowercaseK(db.GetContext(), 29),
		},
		{
			"select v from Customer where Type(V) = \"Invoice\"",
			query.NewErrDidYouMeanLowercaseV(db.GetContext(), 34),
		},
		{
			"select K, V from Customer where Type(V) = \"Invoice\" and K = \"001\"",
			query.NewErrDidYouMeanLowercaseK(db.GetContext(), 7),
		},
		{
			"select type(v) from Customer where Type(v) = \"Invoice\"",
			query.NewErrDidYouMeanFunction(db.GetContext(), 7, "Type"),
		},
		{
			"select Type(v) from Customer where type(v) = \"Invoice\"",
			query.NewErrDidYouMeanFunction(db.GetContext(), 35, "Type"),
		},
		{
			"select type(v) from Customer where type(v) = \"Invoice\"",
			query.NewErrDidYouMeanFunction(db.GetContext(), 7, "Type"),
		},
		{
			"select time(\"foo\") from Customer",
			query.NewErrDidYouMeanFunction(db.GetContext(), 7, "Time"),
		},
		{
			"select TimE(\"foo\") from Customer",
			query.NewErrDidYouMeanFunction(db.GetContext(), 7, "Time"),
		},
		{
			"select year(\"foo\") from Customer",
			query.NewErrDidYouMeanFunction(db.GetContext(), 7, "Year"),
		},
		{
			"select month(\"foo\") from Customer",
			query.NewErrDidYouMeanFunction(db.GetContext(), 7, "Month"),
		},
		{
			"select day(\"foo\") from Customer",
			query.NewErrDidYouMeanFunction(db.GetContext(), 7, "Day"),
		},
		{
			"select hour(\"foo\") from Customer",
			query.NewErrDidYouMeanFunction(db.GetContext(), 7, "Hour"),
		},
		{
			"select minute(\"foo\") from Customer",
			query.NewErrDidYouMeanFunction(db.GetContext(), 7, "Minute"),
		},
		{
			"select second(\"foo\") from Customer",
			query.NewErrDidYouMeanFunction(db.GetContext(), 7, "Second"),
		},
		{
			"select now() from Customer",
			query.NewErrDidYouMeanFunction(db.GetContext(), 7, "Now"),
		},
		{
			"select LowerCase(\"foo\") from Customer",
			query.NewErrDidYouMeanFunction(db.GetContext(), 7, "Lowercase"),
		},
		{
			"select UPPERCASE(\"foo\") from Customer",
			query.NewErrDidYouMeanFunction(db.GetContext(), 7, "Uppercase"),
		},
		{
			"select spliT(\"foo:bar\", \":\") from Customer",
			query.NewErrDidYouMeanFunction(db.GetContext(), 7, "Split"),
		},
		{
			"select comPLex(1.0, 2.0) from Customer",
			query.NewErrDidYouMeanFunction(db.GetContext(), 7, "Complex"),
		},
		{
			"select len(\"foo\") from Customer",
			query.NewErrDidYouMeanFunction(db.GetContext(), 7, "Len"),
		},
		{
			"select StrRepeat(\"foo\", \"x\") from Customer",
			query.NewErrIntConversionError(db.GetContext(), 24, errors.New("Cannot convert operand to int64.")),
		},
		{
			"select Complex(23, \"foo\") from Customer",
			query.NewErrFloatConversionError(db.GetContext(), 19, errors.New("Cannot convert operand to float64.")),
		},
		{
			"select Complex(\"foo\", 42) from Customer",
			query.NewErrFloatConversionError(db.GetContext(), 15, errors.New("Cannot convert operand to float64.")),
		},
		{
			"select Complex(42.0) from Customer",
			query.NewErrFunctionArgCount(db.GetContext(), 7, "Complex", 2, 1),
		},
		{
			"select StrCat(v.Address.City, 42) from Customer",
			query.NewErrStringConversionError(db.GetContext(), 30, errors.New("Cannot convert operand to string.")),
		},
	}

	for _, test := range basic {
		_, _, err := internal.Exec(db, test.query)
		// Test both that the IDs compare and the text compares (since the offset needs to match).
		if verror.ErrorID(err) != verror.ErrorID(test.err) || err.Error() != test.err.Error() {
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
		r := internal.ResolveField(db, test.k, test.v, &test.f)
		if !reflect.DeepEqual(r, test.r) {
			t.Errorf("got %v(%s), want %v(%s)", r, r.Type(), test.r, test.r.Type())
		}
	}
}