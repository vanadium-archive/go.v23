// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package db

import (
	"errors"
	"fmt"
	"time"

	"v.io/syncbase/v23/syncbase/nosql/query_db"
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/vdl"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

var d *demoDB

func InitDB() v23.Shutdown {
	d = createDB()
	var shutdown v23.Shutdown
	d.ctx, shutdown = test.V23Init()
	return shutdown
}

type demoDB struct {
	ctx    *context.T
	tables []table
}

type kv struct {
	key   string
	value *vdl.Value
}

type table struct {
	name string
	rows []kv
}

type keyValueStreamImpl struct {
	table       table
	cursor      int
	keyRanges   query_db.KeyRanges
	rangeCursor int
}

func (kvs *keyValueStreamImpl) Advance() bool {
	for true {
		kvs.cursor++ // initialized to -1
		if kvs.cursor >= len(kvs.table.rows) {
			return false
		}
		for kvs.rangeCursor < len(kvs.keyRanges) {
			// does it match any keyRange (or is the keyRange the 0-255 wildcard)?
			if (kvs.keyRanges[kvs.rangeCursor].Start == string([]byte{0}) && kvs.keyRanges[kvs.rangeCursor].Limit == string([]byte{255})) ||
				(kvs.table.rows[kvs.cursor].key >= kvs.keyRanges[kvs.rangeCursor].Start && kvs.table.rows[kvs.cursor].key <= kvs.keyRanges[kvs.rangeCursor].Limit) {
				return true
			}
			// Keys and keyRanges are both sorted low to high, so we can increment
			// rangeCursor if the keyRange.Limit is < the key.
			if kvs.keyRanges[kvs.rangeCursor].Limit < kvs.table.rows[kvs.cursor].key {
				kvs.rangeCursor++
				if kvs.rangeCursor >= len(kvs.keyRanges) {
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

func (t table) Scan(keyRanges query_db.KeyRanges) (query_db.KeyValueStream, error) {
	var keyValueStreamImpl keyValueStreamImpl
	keyValueStreamImpl.table = t
	keyValueStreamImpl.cursor = -1
	keyValueStreamImpl.keyRanges = keyRanges
	return &keyValueStreamImpl, nil
}

// GetTableNames is not part of the query_db.Database interface.
// Tables are discovered outside the query package.
// TODO(jkline): Consider system tables to discover info such as this.
func GetTableNames() []string {
	tables := []string{}
	for _, table := range d.tables {
		tables = append(tables, table.name)
	}
	return tables
}

// GetDatabase in not part of the query_db.Database interface.
// Database instances are obtained outside the query package.
func GetDatabase() query_db.Database {
	return d
}

func (db demoDB) GetContext() *context.T {
	return db.ctx
}

func (d demoDB) GetTable(table string) (query_db.Table, error) {
	for _, t := range d.tables {
		if t.name == table {
			return t, nil
		}
	}
	return nil, errors.New(fmt.Sprintf("No such table: %s.", table))
}

func createDB() *demoDB {
	d = &demoDB{}
	var custTable table
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
			vdl.ValueOf(Invoice{2, 1001, 166, AddressInfo{"777 Any St.", "Collins", "IA", "50055"}}),
		},
		kv{
			"002002",
			vdl.ValueOf(Invoice{2, 1002, 243, AddressInfo{"888 Any St.", "Collins", "IA", "50055"}}),
		},
		kv{
			"002003",
			vdl.ValueOf(Invoice{2, 1004, 787, AddressInfo{"999 Any St.", "Collins", "IA", "50055"}}),
		},
		kv{
			"002004",
			vdl.ValueOf(Invoice{2, 1006, 88, AddressInfo{"101010 Any St.", "Collins", "IA", "50055"}}),
		},
	}
	d.tables = append(d.tables, custTable)

	var numTable table
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
	d.tables = append(d.tables, numTable)

	var compositeTable table
	compositeTable.name = "Composites"
	compositeTable.rows = []kv{
		kv{
			"uno",
			vdl.ValueOf(Composite{Array2String{"foo", "bar"}, []int32{1, 2}, map[int32]struct{}{1: struct{}{}, 2: struct{}{}}, map[string]int32{"foo": 1, "bar": 2}}),
		},
	}
	d.tables = append(d.tables, compositeTable)

	var recursiveTable table
	recursiveTable.name = "Recursives"
	recursiveTable.rows = []kv{
		kv{
			"alpha",
			vdl.ValueOf(Recursive{nil, &Times{time.Unix(123456789, 42244224), time.Duration(1337)}, map[Array2String]Recursive{
				Array2String{"a", "b"}: Recursive{},
				Array2String{"x", "y"}: Recursive{vdl.ValueOf(CreditReport{Agency: CreditAgencyExperian, Report: AgencyReportExperianReport{ExperianCreditReport{ExperianRatingGood}}}), nil, map[Array2String]Recursive{
					Array2String{"alpha", "beta"}: Recursive{vdl.ValueOf(FooType{Bar: BarType{Baz: BazType{Name: "hello", TitleOrValue: TitleOrValueTypeValue{Value: 42}}}}), nil, nil},
				}},
				Array2String{"u", "v"}: Recursive{vdl.ValueOf(vdl.TypeOf(Recursive{})), nil, nil},
			}}),
		},
	}
	d.tables = append(d.tables, recursiveTable)

	return d
}
