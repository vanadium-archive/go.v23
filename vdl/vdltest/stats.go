// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdltest

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"v.io/v23/vdl"
)

// statsTable is a table of statistics, represented as a sequence of rows.  Each
// row has a map of columns for easy updating.
type statsTable struct {
	Rows []statRow
}

// statsTableInfo holds info used for nice formatting.
type statsTableInfo struct {
	Headers     []string    // Sorted column headers
	RowNameSize int         // Size of row.Name column
	ColumnSizes statColumns // Size of each data column
}

// ColumnSize returns the size of the column with the given header.
func (info statsTableInfo) ColumnSize(header string) int {
	total := 0
	for index, size := range info.ColumnSizes.Map[header] {
		if index > 0 {
			total += 1 // account for comma between each value
		}
		total += size
	}
	return total
}

func (t *statsTable) computeInfo() statsTableInfo {
	var info statsTableInfo
	// Gather max size information over all values in the table.
	for _, row := range t.Rows {
		if size := len(row.Name); size > info.RowNameSize {
			info.RowNameSize = size
		}
		for header, slice := range row.Columns.Map {
			for index, value := range slice {
				// Update ColumnSizes with the size of the largest printed value.
				old := info.ColumnSizes.Get(header, index)
				if size := len(strconv.Itoa(value)); size > old {
					info.ColumnSizes.Set(header, index, size)
				}
			}
		}
	}
	// Collect Headers for sorting, and adjust ColumnSizes if necessary.
	for header, sizes := range info.ColumnSizes.Map {
		info.Headers = append(info.Headers, header)
		// If the header size is bigger than the computed column size, add padding
		// to ColumnSizes so that the table aligns correctly.  Spread the padding
		// evenly across each size, with leftover added to the first size.
		//
		// There's no need to adjust anything if the header size is smaller than
		// what's computed; Print takes care of that.
		if pad := len(header) - info.ColumnSize(header); pad > 0 {
			total, padPerSize := 0, pad/len(sizes)
			for index := range sizes {
				sizes[index] += padPerSize
				total += padPerSize
			}
			sizes[0] += pad - total
		}
	}
	sort.Strings(info.Headers)
	return info
}

func centerString(s string, size int) string {
	if len(s) >= size {
		return s
	}
	center := strings.Repeat(" ", (size-len(s))/2)
	center += s
	center += strings.Repeat(" ", size-len(center))
	return center
}

func fp(w io.Writer, format string, args ...interface{}) error {
	_, err := fmt.Fprintf(w, format, args...)
	return err
}

// Print prints t into a nicely formatted table.
func (t *statsTable) Print(w io.Writer) error {
	info := t.computeInfo()
	// Print column headers.
	if err := fp(w, strings.Repeat(" ", info.RowNameSize)); err != nil {
		return err
	}
	for _, header := range info.Headers {
		centered := centerString(header, info.ColumnSize(header))
		if err := fp(w, "|"+centered); err != nil {
			return err
		}
	}
	if err := fp(w, "|\n"); err != nil {
		return err
	}
	// Print each row.
	if err := t.printRow(w, info, statRowBreak); err != nil {
		return err
	}
	for _, row := range t.Rows {
		if err := t.printRow(w, info, row); err != nil {
			return err
		}
	}
	return t.printRow(w, info, statRowBreak)
}

func (t *statsTable) printRow(w io.Writer, info statsTableInfo, row statRow) error {
	if row.IsBreak() {
		// Print row break.
		breakName := strings.Repeat("-", info.RowNameSize)
		if err := fp(w, breakName); err != nil {
			return err
		}
		for _, header := range info.Headers {
			breakHeader := strings.Repeat("-", info.ColumnSize(header))
			if err := fp(w, "+"+breakHeader); err != nil {
				return err
			}
		}
		return fp(w, "|\n")
	}
	// Print each column.
	if err := fp(w, "%-[1]*[2]s", info.RowNameSize, row.Name); err != nil {
		return err
	}
	for _, header := range info.Headers {
		if err := fp(w, "|"); err != nil {
			return err
		}
		for index, size := range info.ColumnSizes.Map[header] {
			if index > 0 {
				if err := fp(w, ","); err != nil {
					return err
				}
			}
			if err := fp(w, "%[1]*[2]d", size, row.Columns.Get(header, index)); err != nil {
				return err
			}
		}
	}
	if err := fp(w, "|"); err != nil {
		return err
	}
	// Print each property.
	for _, prop := range row.Props {
		if err := prop.Print(w); err != nil {
			return err
		}
	}
	return fp(w, "\n")
}

// statRow represents a single row in a statsTable.  Each row contains columns
// of data, as well as a list of properties.
type statRow struct {
	Name    string
	Columns statColumns
	Props   []statProp
}

var statRowBreak = statRow{Name: "-"}

func (r *statRow) IsBreak() bool {
	return r.Name == statRowBreak.Name && r.Columns.IsEmpty() && len(r.Props) == 0
}

func (r *statRow) UpdateProp(name string, value int, accumulate bool) {
	for ix, prop := range r.Props {
		if name != prop.Name {
			continue
		}
		// Found an existing property, update it.
		switch {
		case accumulate:
			// Min and max are always equal, so they act as a single value.
			r.Props[ix].Min += value
			r.Props[ix].Max += value
		default:
			if value < prop.Min {
				r.Props[ix].Min = value
			}
			if value > prop.Max {
				r.Props[ix].Max = value
			}
		}
		return
	}
	// Add a new property, init both min and max.
	r.Props = append(r.Props, statProp{name, value, value})
}

// statColumns holds the columns for a single statRow, represented as a map from
// column headers to int values.  Each column may hold multiple values.
type statColumns struct {
	Map map[string][]int
}

func (x *statColumns) IsEmpty() bool {
	return len(x.Map) == 0
}

func (x *statColumns) Get(header string, index int) int {
	if len(x.Map[header]) <= index {
		// The map entry doesn't exist, or the slice isn't big enough.  Either way
		// there isn't an existing value.
		return 0
	}
	return x.Map[header][index]
}

func (x *statColumns) Set(header string, index, value int) {
	if x.Map == nil {
		x.Map = make(map[string][]int)
	}
	slice := x.Map[header]
	for len(slice) <= index {
		slice = append(slice, 0)
	}
	slice[index] = value
	x.Map[header] = slice
}

func (x *statColumns) Delta(header string, index, delta int) {
	if len(x.Map[header]) <= index {
		x.Set(header, index, delta)
		return
	}
	slice := x.Map[header]
	slice[index] += delta
	x.Map[header] = slice
}

// statProp is a single int property held in statRow.  We maintain the min/max
// value of the property as it is collected.
type statProp struct {
	Name string // Name of the property
	Min  int    // Min value of the property
	Max  int    // Max value of the property
}

func (p statProp) Print(w io.Writer) error {
	if p.Min == p.Max {
		return fp(w, " [%s=%d]", p.Name, p.Min)
	}
	return fp(w, " [%s min=%d max=%d]", p.Name, p.Min, p.Max)
}

// typePropFn gathers the value of a single type property, used to update the
// statProp in a statRow.
type typePropFn struct {
	Name       string              // Name of the property
	Fn         func(*vdl.Type) int // Fn that extracts the property from a type
	Accumulate bool                // Accumulate results rather than min/max
}

// typeStatsCollector collects a statsTable over all given Types.
type typeStatsCollector struct {
	Types []*vdl.Type
	Table statsTable
}

// Collect collects stats across all types for the named stat row, given the
// predicate fn.  We keep counts of the number of types that match the
// predicate, as well as the number of types containing a type that matches the
// predicate.
//
// The propFns are only run against types that match the predicate, where
// properties are either accumulated or compared for min/max.
func (c *typeStatsCollector) Collect(name string, fn func(*vdl.Type) bool, propFns ...typePropFn) {
	row := statRow{Name: name}
	for _, tt := range c.Types {
		if fn(tt) {
			// The predicate matches.  Update the total count and the properties.
			row.Columns.Delta(".Total", 0, 1)
			for _, propFn := range propFns {
				row.UpdateProp(propFn.Name, propFn.Fn(tt), propFn.Accumulate)
			}
		}
		// Update the contains count by walking through tt until the predicate
		// matches.  We invert the predicate since tt.Walk will early-exit when we
		// return false; thus false means that the predicate matched.
		contains := !tt.Walk(vdl.WalkAll, func(visit *vdl.Type) bool {
			return !fn(visit)
		})
		if contains {
			row.Columns.Delta("Contains", 0, 1)
		}
	}
	c.Table.Rows = append(c.Table.Rows, row)
}

// PrintTypeStats prints statistics gathered from types into w.
func PrintTypeStats(w io.Writer, types ...*vdl.Type) error {
	stats := typeStatsCollector{Types: types}
	// Collect stats by kind.
	for kind := vdl.Any; kind <= vdl.Union; kind++ {
		var fns []typePropFn
		switch kind {
		case vdl.Array:
			fns = append(fns, typePropFn{Name: "len", Fn: (*vdl.Type).Len})
		case vdl.List, vdl.Set, vdl.Map:
			fn := func(tt *vdl.Type) int {
				return predToProp(tt.Name() == "")
			}
			fns = append(fns, typePropFn{Name: "unnamed", Fn: fn, Accumulate: true})
		case vdl.Enum:
			fns = append(fns, typePropFn{Name: "labels", Fn: (*vdl.Type).NumEnumLabel})
		case vdl.Struct, vdl.Union:
			fns = append(fns, typePropFn{Name: "fields", Fn: (*vdl.Type).NumField})
		}
		stats.Collect(kind.String(), func(tt *vdl.Type) bool {
			return tt.Kind() == kind
		}, fns...)
	}
	stats.Table.Rows = append(stats.Table.Rows, statRowBreak)
	// Collect stats by type predicate.
	stats.Collect("IsNamed", func(tt *vdl.Type) bool {
		return tt.Name() != ""
	})
	stats.Collect("IsUnnamed", func(tt *vdl.Type) bool {
		return tt.Name() == ""
	})
	stats.Collect("IsNumber", func(tt *vdl.Type) bool {
		return tt.Kind().IsNumber()
	})
	stats.Collect("IsErrorType", func(tt *vdl.Type) bool {
		return tt == vdl.ErrorType || tt.Name() == "VError"
	})
	stats.Collect("IsBytes", (*vdl.Type).IsBytes)
	stats.Collect("IsPartOfCycle", (*vdl.Type).IsPartOfCycle)
	stats.Collect("CanBeNamed", (*vdl.Type).CanBeNamed)
	stats.Collect("CanBeKey", (*vdl.Type).CanBeKey)
	stats.Collect("CanBeNil", (*vdl.Type).CanBeNil)
	stats.Collect("CanBeOptional", (*vdl.Type).CanBeOptional)
	// Print stats table.
	if err := fp(w, "Types: %d\n", len(types)); err != nil {
		return err
	}
	return stats.Table.Print(w)
}

func predToProp(pred bool) int {
	if pred {
		return 1
	}
	return 0
}

// entryPropFn gathers the value of a single entry property, used to update the
// statProp in a statRow.
type entryPropFn struct {
	Name       string               // Name of the property
	Fn         func(EntryValue) int // Fn that extracts the property from an entry
	Accumulate bool                 // Accumulate results rather than min/max
}

// entryStatsCollector collects a statsTable over all given Entries.
type entryStatsCollector struct {
	Entries []EntryValue
	Table   statsTable
}

// Collect collects stats across all entries for the named stat row, given the
// predicate fn.
//
// The propFns are only run against entries that match the predicate, where
// properties are either accumulated or compared for min/max.
func (c *entryStatsCollector) Collect(name string, fn func(EntryValue) bool, propFns ...entryPropFn) {
	row := statRow{Name: name}
	for _, e := range c.Entries {
		if fn(e) {
			// The predicate matches.  Update the stats table.
			index := 1
			if e.IsCanonical() {
				index = 0
			}
			row.Columns.Delta(".Total", index, 1)
			row.Columns.Delta(e.Label, index, 1)
			if e.Source.IsZero() {
				row.Columns.Delta("isZero", index, 1)
			}
			// Update the properties.
			for _, propFn := range propFns {
				row.UpdateProp(propFn.Name, propFn.Fn(e), propFn.Accumulate)
			}
			// Add some error checking.
			if e.Source.Kind() == vdl.String {
				// TODO(toddw): Walk through source and check all strings.
				if !utf8.ValidString(e.Source.RawString()) {
					row.UpdateProp("ERROR: INVALID UTF8", 1, true)
				}
			}
		}
	}
	c.Table.Rows = append(c.Table.Rows, row)
}

const targetEqSourceRowName = "target=src"

// PrintEntryStats prints statistics gathered from entries into w.
func PrintEntryStats(w io.Writer, entries ...EntryValue) error {
	stats := entryStatsCollector{Entries: entries}
	// Collect stats by entry predicate.
	stats.Collect("total", func(e EntryValue) bool {
		return true
	})
	stats.Collect("isZero", func(e EntryValue) bool {
		return e.Source.IsZero()
	})
	stats.Collect(targetEqSourceRowName, func(e EntryValue) bool {
		return vdl.EqualValue(e.Target, e.Source)
	})
	stats.Table.Rows = append(stats.Table.Rows, statRowBreak)
	// Collect stats by source kind.
	for kind := vdl.Any; kind <= vdl.Union; kind++ {
		var fns []entryPropFn
		switch kind {
		case vdl.Array, vdl.List, vdl.Set, vdl.Map:
			fns = append(fns, entryPropFn{Name: "len", Fn: func(e EntryValue) int {
				return e.Source.Len()
			}})
		case vdl.Any, vdl.Optional:
			fn := func(e EntryValue) int {
				return predToProp(e.Source.IsNil())
			}
			fns = append(fns, entryPropFn{Name: "nil", Fn: fn, Accumulate: true})
		}
		stats.Collect(kind.String(), func(e EntryValue) bool {
			return e.Source.Kind() == kind
		}, fns...)
	}
	// Print stats table.
	if err := fp(w, "Entries: %d\n", len(entries)); err != nil {
		return err
	}
	return stats.Table.Print(w)
}
