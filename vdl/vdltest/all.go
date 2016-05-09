// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vdltest provides a variety of VDL types and values for testing.
package vdltest

import (
	"flag"
	"reflect"
	"strings"
)

// The following causes data files to be generated when "go generate" is run.
//go:generate ./gen.sh

var flagContains string

func init() {
	flag.StringVar(&flagContains, "vdltest", "", "Filter vdltest.All to only return entries that contain the given substring.")
}

// The following vars are defined in generated files:
//   var vAllPass, vAllFail, xAllPass, xAllFail []vdlEntry

// AllPass returns all entries where the source value, when converted to the
// type of the target value, results in exactly the target value.
//
// The -vdltest flag may be used to filter the returned entries.
func AllPass() []Entry {
	var result []Entry
	for _, e := range fromVDLEntries(vAllPass, xAllPass) {
		if strings.Contains(e.Name(), flagContains) {
			result = append(result, e)
		}
	}
	return result
}

// AllPassFunc returns the entries in AllPass where fn(e) returns true for each
// returned entry.
func AllPassFunc(fn func(e Entry) bool) []Entry {
	var result []Entry
	for _, e := range AllPass() {
		if fn(e) {
			result = append(result, e)
		}
	}
	return result
}

// AllFail returns all entries where the source value, when converted to the
// type of the target value, results in a conversion error.
//
// E.g. the types of the source and target may be incompatible; trying to
// convert a source bool to a target struct returns an error.  Or the values may
// be inconvertible; trying to convert a source int32(-1) to a target uint32
// returns an error.
//
// The -vdltest flag may be used to filter the returned entries.
func AllFail() []Entry {
	var result []Entry
	for _, e := range fromVDLEntries(vAllFail, xAllFail) {
		if strings.Contains(e.Name(), flagContains) {
			result = append(result, e)
		}
	}
	return result
}

// AllFailFunc returns the entries in AllFail where fn(e) returns true for each
// returned entry.
func AllFailFunc(fn func(e Entry) bool) []Entry {
	var result []Entry
	for _, e := range AllFail() {
		if fn(e) {
			result = append(result, e)
		}
	}
	return result
}

func fromVDLEntries(groups ...[]vdlEntry) []Entry {
	var result []Entry
	for _, entries := range groups {
		for _, e := range entries {
			entry := fromVDLEntry(e)
			result = append(result, entry)
			// TODO(toddw): add additional entries and pointer tests.
			/*
				switch ttTarget := vdl.TypeOf(e.Target); {
				case ttTarget.Kind() == vdl.Union:
				case ttTarget == vdl.ErrorType:
				}
			*/
		}
	}
	return result
}

func fromVDLEntry(e vdlEntry) Entry {
	return Entry{
		IsCanonical: e.IsCanonical,
		Label:       e.Label,
		TargetLabel: e.TargetLabel,
		Target:      reflect.ValueOf(e.Target),
		SourceLabel: e.SourceLabel,
		Source:      reflect.ValueOf(e.Source),
	}
}
