// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vomtest provides protocol conformance tests for the Vanadium Object
// Marshaller (VOM).
package vomtest

import (
	"flag"
	"strings"
)

// The following causes data files to be generated when "go generate" is run.
//go:generate ./gen.sh

var flagContains string

func init() {
	flag.StringVar(&flagContains, "vomtest", "", "Filter vomtest.Data to only return entries that contain the given substring.")
}

// Data returns all vom test cases.
func Data() []TestCase {
	var result []TestCase
	for _, t := range data81 {
		if strings.Contains(t.Name, flagContains) {
			result = append(result, t)
		}
	}
	return result
}

// DataFunc returns the entries in Data where fn(e) returns true for each
// returned entry.
func DataFunc(fn func(testCase TestCase) bool) []TestCase {
	var result []TestCase
	for _, e := range Data() {
		if fn(e) {
			result = append(result, e)
		}
	}
	return result
}

func versionedData(version byte) []TestCase {
	return DataFunc(func(testCase TestCase) bool {
		return testCase.Version == version
	})
}

// Data81 returns test cases for vom version 81.
func Data81() []TestCase {
	return versionedData(0x81)
}
