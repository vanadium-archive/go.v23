// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util_test

import (
	"strings"
	"testing"

	"v.io/v23/syncbase/util"
	tu "v.io/x/ref/services/syncbase/testutil"
)

func TestEscapeUnescape(t *testing.T) {
	strs := []string{}
	copy(strs, tu.OkAppRowNames)
	strs = append(strs, tu.NotOkAppRowNames...)
	for _, s := range strs {
		esc := util.Escape(s)
		unesc, ok := util.Unescape(esc)
		if !ok {
			t.Fatalf("unescape failed: %q, %q", s, esc)
		}
		if strings.ContainsAny(esc, "/") {
			t.Errorf("%q was escaped to %q, which contains bad chars", s, esc)
		}
		if !strings.ContainsAny(s, "%/") && s != esc {
			t.Errorf("%q should equal %q", s, esc)
		}
		if s != unesc {
			t.Errorf("%q should equal %q", s, unesc)
		}
	}
}

func TestValidNameFuncs(t *testing.T) {
	for _, s := range tu.OkAppRowNames {
		if !util.ValidAppName(s) {
			t.Errorf("%q should be valid", s)
		}
		if !util.ValidRowKey(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range tu.NotOkAppRowNames {
		if util.ValidAppName(s) {
			t.Errorf("%q should be invalid", s)
		}
		if util.ValidRowKey(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
	for _, s := range tu.OkDbTableNames {
		if !util.ValidDatabaseName(s) {
			t.Errorf("%q should be valid", s)
		}
		if !util.ValidTableName(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range tu.NotOkDbTableNames {
		if util.ValidDatabaseName(s) {
			t.Errorf("%q should be invalid", s)
		}
		if util.ValidTableName(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestPrefixRange(t *testing.T) {
	tests := []struct {
		prefix string
		start  string
		limit  string
	}{
		{"", "", ""},
		{"a", "a", "b"},
		{"aa", "aa", "ab"},
		{"\xfe", "\xfe", "\xff"},
		{"a\xfe", "a\xfe", "a\xff"},
		{"aa\xfe", "aa\xfe", "aa\xff"},
		{"a\xff", "a\xff", "b"},
		{"aa\xff", "aa\xff", "ab"},
		{"a\xff\xff", "a\xff\xff", "b"},
		{"aa\xff\xff", "aa\xff\xff", "ab"},
		{"\xff", "\xff", ""},
		{"\xff\xff", "\xff\xff", ""},
	}
	for _, test := range tests {
		start, limit := util.PrefixRangeStart(test.prefix), util.PrefixRangeLimit(test.prefix)
		if start != test.start || limit != test.limit {
			t.Errorf("%q: got {%q, %q}, want {%q, %q}", test.prefix, start, limit, test.start, test.limit)
		}
	}
}

func TestIsPrefix(t *testing.T) {
	tests := []struct {
		isPrefix bool
		start    string
		limit    string
	}{
		{true, "", ""},
		{true, "a", "b"},
		{true, "aa", "ab"},
		{true, "\xfe", "\xff"},
		{true, "a\xfe", "a\xff"},
		{true, "aa\xfe", "aa\xff"},
		{true, "a\xff", "b"},
		{true, "aa\xff", "ab"},
		{true, "a\xff\xff", "b"},
		{true, "aa\xff\xff", "ab"},
		{true, "\xff", ""},
		{true, "\xff\xff", ""},

		{false, "", "\x00"},
		{false, "a", "aa"},
		{false, "aa", "aa"},
		{false, "\xfe", "\x00"},
		{false, "a\xfe", "b\xfe"},
		{false, "aa\xfe", "aa\x00"},
		{false, "a\xff", "b\x00"},
		{false, "aa\xff", "ab\x00"},
		{false, "a\xff\xff", "a\xff\xff\xff"},
		{false, "aa\xff\xff", "a"},
		{false, "\xff", "\x00"},
	}
	for _, test := range tests {
		result := util.IsPrefix(test.start, test.limit)
		if result != test.isPrefix {
			t.Errorf("%q, %q: got %v, want %v", test.start, test.limit, result, test.isPrefix)
		}
	}
}
