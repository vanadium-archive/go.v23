// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package writer

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"v.io/syncbase/v23/syncbase/nosql/internal/query"
	"v.io/v23/vdl"
)

type Justification int

const (
	Unknown Justification = iota
	Left
	Right
)

// WriteTable formats the results as ASCII tables.
func WriteTable(out io.Writer, columnNames []string, rs query.ResultStream) error {
	// Buffer the results so we can compute the column widths.
	columnWidths := make([]int, len(columnNames))
	for i, cName := range columnNames {
		columnWidths[i] = len(cName)
	}
	justification := make([]Justification, len(columnNames))
	var results [][]string
	for rs.Advance() {
		row := make([]string, len(columnNames))
		for i, column := range rs.Result() {
			if i >= len(columnNames) {
				return errors.New("more columns in result than in columnNames")
			}
			if justification[i] == Unknown {
				justification[i] = getJustification(column)
			}
			columnStr := toString(column)
			row[i] = columnStr
			if len(columnStr) > columnWidths[i] {
				columnWidths[i] = len(columnStr)
			}
		}
		results = append(results, row)
	}
	if rs.Err() != nil {
		return rs.Err()
	}

	writeBorder(out, columnWidths)
	sep := "| "
	for i, cName := range columnNames {
		io.WriteString(out, fmt.Sprintf("%s%*s", sep, columnWidths[i], cName))
		sep = " | "
	}
	io.WriteString(out, " |\n")
	writeBorder(out, columnWidths)
	for _, result := range results {
		sep = "| "
		for i, column := range result {
			if justification[i] == Right {
				io.WriteString(out, fmt.Sprintf("%s%*s", sep, columnWidths[i], column))
			} else {
				io.WriteString(out, fmt.Sprintf("%s%-*s", sep, columnWidths[i], column))
			}
			sep = " | "
		}
		io.WriteString(out, " |\n")
	}
	writeBorder(out, columnWidths)
	return nil
}

func writeBorder(out io.Writer, columnWidths []int) {
	sep := "+-"
	for _, width := range columnWidths {
		io.WriteString(out, fmt.Sprintf("%s%s", sep, strings.Repeat("-", width)))
		sep = "-+-"
	}
	io.WriteString(out, "-+\n")
}

func getJustification(val *vdl.Value) Justification {
	switch val.Kind() {
	// TODO(kash): Floating point numbers should have the decimal point line up.
	case vdl.Bool, vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int16, vdl.Int32, vdl.Int64,
		vdl.Float32, vdl.Float64, vdl.Complex64, vdl.Complex128:
		return Right
	// TODO(kash): Leave nil values as unknown.
	default:
		return Left
	}
}

// WriteCSV formats the results as CSV as specified by https://tools.ietf.org/html/rfc4180.
func WriteCSV(out io.Writer, columnNames []string, rs query.ResultStream, delimiter string) error {
	delim := ""
	for _, cName := range columnNames {
		str := doubleQuoteForCSV(cName, delimiter)
		io.WriteString(out, fmt.Sprintf("%s%s", delim, str))
		delim = delimiter
	}
	io.WriteString(out, "\n")
	for rs.Advance() {
		delim := ""
		for _, column := range rs.Result() {
			str := doubleQuoteForCSV(toString(column), delimiter)
			io.WriteString(out, fmt.Sprintf("%s%s", delim, str))
			delim = delimiter
		}
		io.WriteString(out, "\n")
	}
	return rs.Err()
}

// doubleQuoteForCSV follows the escaping rules from
// https://tools.ietf.org/html/rfc4180. In particular, values containing
// newlines, double quotes, and the delimiter must be enclosed in double
// quotes.
func doubleQuoteForCSV(str, delimiter string) string {
	doubleQuote := strings.Index(str, delimiter) != -1 || strings.Index(str, "\n") != -1
	if strings.Index(str, "\"") != -1 {
		str = strings.Replace(str, "\"", "\"\"", -1)
		doubleQuote = true
	}
	if doubleQuote {
		str = "\"" + str + "\""
	}
	return str
}

func toString(val *vdl.Value) string {
	switch val.Kind() {
	case vdl.Bool:
		return fmt.Sprint(val.Bool())
	case vdl.Byte:
		return fmt.Sprint(val.Byte())
	case vdl.Uint16, vdl.Uint32, vdl.Uint64:
		return fmt.Sprint(val.Uint())
	case vdl.Int16, vdl.Int32, vdl.Int64:
		return fmt.Sprint(val.Int())
	case vdl.Float32, vdl.Float64:
		return fmt.Sprint(val.Float())
	case vdl.Complex64, vdl.Complex128:
		c := val.Complex()
		return fmt.Sprintf("%v+%vi", real(c), imag(c))
	case vdl.String:
		return val.RawString()
	case vdl.Enum:
		return val.EnumLabel()
	case vdl.Array, vdl.List:
		ret := "["
		sep := ""
		for i := 0; i < val.Len(); i++ {
			ret += sep + toString(val.Index(i))
		}
		return ret + "]"
		// TODO(kash): Add support for Nil, Time, TypeObject, Set, Map, Struct,
		// Union.  Not sure if I need to support Any and Optional.
	default:
		return fmt.Sprintf("unknown Kind %s", val.Kind())
	}
}
