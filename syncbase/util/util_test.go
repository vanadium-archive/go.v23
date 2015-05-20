// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util_test

import (
	"testing"

	"v.io/syncbase/v23/syncbase/util"
)

func TestValidName(t *testing.T) {
	tests := []struct {
		name string
		res  bool
	}{
		{"", false},
		{"*", false},
		{"a*", false},
		{"*a", false},
		{"a*b", false},
		{"/", false},
		{"a/", false},
		{"/a", false},
		{"a/b", false},
		{"a", true},
		{"aa", true},
	}
	for _, test := range tests {
		res := util.ValidName(test.name)
		if res != test.res {
			t.Errorf("%q: got %v, want %v", test.name, res, test.res)
		}
	}
}

func TestPrefixRange(t *testing.T) {
	tests := []struct {
		prefix string
		start  string
		end    string
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
		start, end := util.PrefixRangeStart(test.prefix), util.PrefixRangeEnd(test.prefix)
		if start != test.start || end != test.end {
			t.Errorf("%q: got {%q, %q}, want {%q, %q}", test.prefix, start, end, test.start, test.end)
		}
	}
}
