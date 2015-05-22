// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package writer_test

import (
	"bytes"
	"testing"

	"v.io/syncbase/v23/syncbase/nosql/internal/query"
	"v.io/syncbase/v23/syncbase/nosql/internal/query/demo/writer"
	"v.io/v23/vdl"
)

type fakeResultStream struct {
	rows [][]*vdl.Value
	curr int
}

func newResultStream(iRows [][]interface{}) query.ResultStream {
	vRows := make([][]*vdl.Value, len(iRows))
	for i, iRow := range iRows {
		vRow := make([]*vdl.Value, len(iRow))
		for j, iCol := range iRow {
			vRow[j] = vdl.ValueOf(iCol)
		}
		vRows[i] = vRow
	}
	return &fakeResultStream{
		rows: vRows,
		curr: -1,
	}
}

func (f *fakeResultStream) Advance() bool {
	f.curr++
	return f.curr < len(f.rows)
}

func (f *fakeResultStream) Result() []*vdl.Value {
	if f.curr == -1 {
		panic("call advance first")
	}
	return f.rows[f.curr]
}

func (f *fakeResultStream) Err() error {
	return nil
}

func (f *fakeResultStream) Cancel() {
	// Nothing to do.
}

func TestWriteTable(t *testing.T) {
	type testCase struct {
		columns []string
		rows    [][]interface{}
		// To make the test cases easier to read, output should have a leading
		// newline.
		output string
	}
	tests := []testCase{
		{
			[]string{"c1", "c2"},
			[][]interface{}{
				{5, "foo"},
				{6, "bar"},
			},
			`
+----+-----+
| c1 |  c2 |
+----+-----+
| 5  | foo |
| 6  | bar |
+----+-----+
`,
		},
		{
			[]string{"c1", "c2"},
			[][]interface{}{
				{500, "foo"},
				{6, "barbaz"},
			},
			`
+-----+--------+
|  c1 |     c2 |
+-----+--------+
| 500 | foo    |
| 6   | barbaz |
+-----+--------+
`,
		},
		{
			[]string{"c1", "reallylongcolumnheader"},
			[][]interface{}{
				{5, "foo"},
				{6, "bar"},
			},
			`
+----+------------------------+
| c1 | reallylongcolumnheader |
+----+------------------------+
| 5  | foo                    |
| 6  | bar                    |
+----+------------------------+
`,
		},
		{ // Numbers.
			[]string{"byte", "uint16", "uint32", "uint64", "int16", "int32", "int64",
				"float32", "float64", "complex64", "complex128"},
			[][]interface{}{
				{
					byte(12), uint16(1234), uint32(5678), uint64(999888777666), int16(9876), int32(876543), int64(128),
					float32(3.14159), float64(2.71828182846), complex64(123.0 + 7.0i), complex128(456.789 + 10.1112i),
				},
				{
					byte(9), uint16(99), uint32(999), uint64(9999999), int16(9), int32(99), int64(88),
					float32(1.41421356237), float64(1.73205080757), complex64(9.87 + 7.65i), complex128(4.32 + 1.0i),
				},
			},
			`
+------+--------+--------+--------------+-------+--------+-------+--------------------+---------------+--------------------------------------+------------------+
| byte | uint16 | uint32 |       uint64 | int16 |  int32 | int64 |            float32 |       float64 |                            complex64 |       complex128 |
+------+--------+--------+--------------+-------+--------+-------+--------------------+---------------+--------------------------------------+------------------+
| 12   | 1234   | 5678   | 999888777666 | 9876  | 876543 | 128   | 3.141590118408203  | 2.71828182846 | 123+7i                               | 456.789+10.1112i |
| 9    | 99     | 999    | 9999999      | 9     | 99     | 88    | 1.4142135381698608 | 1.73205080757 | 9.869999885559082+7.650000095367432i | 4.32+1i          |
+------+--------+--------+--------------+-------+--------+-------+--------------------+---------------+--------------------------------------+------------------+
`,
		},
		{ // Strings with whitespace should be printed literally.
			[]string{"c1", "c2"},
			[][]interface{}{
				{"foo\tbar", "foo\nbar"},
			},
			`
+---------+---------+
|      c1 |      c2 |
+---------+---------+
| foo	bar | foo
bar |
+---------+---------+
`,
		},
	}
	for _, test := range tests {
		var b bytes.Buffer
		if err := writer.WriteTable(&b, test.columns, newResultStream(test.rows)); err != nil {
			t.Errorf("Unexpected error: %v", err)
			continue
		}
		// Add a leading newline to the output to match the leading newline
		// in our test cases.
		if got, want := "\n"+b.String(), test.output; got != want {
			t.Errorf("Wrong output:\nGOT:%s\nWANT:%s", got, want)
		}
	}
}

func TestWriteCSV(t *testing.T) {
	type testCase struct {
		columns   []string
		rows      [][]interface{}
		delimiter string
		// To make the test cases easier to read, output should have a leading
		// newline.
		output string
	}
	tests := []testCase{
		{ // Basic.
			[]string{"c1", "c2"},
			[][]interface{}{
				{5, "foo"},
				{6, "bar"},
			},
			",",
			`
c1,c2
5,foo
6,bar
`,
		},
		{ // Numbers.
			[]string{"byte", "uint16", "uint32", "uint64", "int16", "int32", "int64",
				"float32", "float64", "complex64", "complex128"},
			[][]interface{}{
				{
					byte(12), uint16(1234), uint32(5678), uint64(999888777666), int16(9876), int32(876543), int64(128),
					float32(3.14159), float64(2.71828182846), complex64(123.0 + 7.0i), complex128(456.789 + 10.1112i),
				},
				{
					byte(9), uint16(99), uint32(999), uint64(9999999), int16(9), int32(99), int64(88),
					float32(1.41421356237), float64(1.73205080757), complex64(9.87 + 7.65i), complex128(4.32 + 1.0i),
				},
			},
			",",
			`
byte,uint16,uint32,uint64,int16,int32,int64,float32,float64,complex64,complex128
12,1234,5678,999888777666,9876,876543,128,3.141590118408203,2.71828182846,123+7i,456.789+10.1112i
9,99,999,9999999,9,99,88,1.4142135381698608,1.73205080757,9.869999885559082+7.650000095367432i,4.32+1i
`,
		},
		{
			// Values containing newlines, double quotes, and the delimiter must be
			// enclosed in double quotes.
			[]string{"c1", "c2"},
			[][]interface{}{
				{"foo\tbar", "foo\nbar"},
				{"foo\"bar\"", "foo,bar"},
			},
			",",
			`
c1,c2
foo	bar,"foo
bar"
"foo""bar""","foo,bar"
`,
		},
		{ // Delimiters other than comma should be supported.
			[]string{"c1", "c2"},
			[][]interface{}{
				{"foo\tbar", "foo\nbar"},
				{"foo\"bar\"", "foo,bar"},
			},
			"\t",
			`
c1	c2
"foo	bar"	"foo
bar"
"foo""bar"""	foo,bar
`,
		},
		{ // Column names should be escaped properly.
			[]string{"foo\tbar", "foo,bar"},
			[][]interface{}{},
			",",
			`
foo	bar,"foo,bar"
`,
		},
		{ // Same as above but use a non-default delimiter.
			[]string{"foo\tbar", "foo,bar"},
			[][]interface{}{},
			"\t",
			`
"foo	bar"	foo,bar
`,
		},
	}
	for _, test := range tests {
		var b bytes.Buffer
		if err := writer.WriteCSV(&b, test.columns, newResultStream(test.rows), test.delimiter); err != nil {
			t.Errorf("Unexpected error: %v", err)
			continue
		}
		// Add a leading newline to the output to match the leading newline
		// in our test cases.
		if got, want := "\n"+b.String(), test.output; got != want {
			t.Errorf("Wrong output:\nGOT: %q\nWANT:%q", got, want)
		}
	}
}
