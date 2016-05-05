// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vomtest provides protocol conformance tests for the Vanadium Object
// Marshaller (VOM).
package vomtest

// The following causes data files to be generated when "go generate" is run.
//go:generate ./gen.sh

// Data81 returns test cases for vom version 81.
func Data81() []TestCase {
	return data81
}
