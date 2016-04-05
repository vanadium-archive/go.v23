// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util_test

import (
	"strings"
	"testing"

	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase/util"
	tu "v.io/x/ref/services/syncbase/testutil"
)

func TestEncodeDecode(t *testing.T) {
	strs := []string{}
	copy(strs, tu.OkRowKeys)
	strs = append(strs, tu.NotOkRowKeys...)
	for _, s := range strs {
		enc := util.Encode(s)
		dec, err := util.Decode(enc)
		if err != nil {
			t.Fatalf("decode failed: %q, %v", s, err)
		}
		if strings.ContainsAny(enc, "/") {
			t.Errorf("%q was escaped to %q, which contains bad chars", s, enc)
		}
		if !strings.ContainsAny(s, "%/") && s != enc {
			t.Errorf("%q should equal %q", s, enc)
		}
		if s != dec {
			t.Errorf("%q should equal %q", s, dec)
		}
	}
}

func TestEncodeIdDecodeId(t *testing.T) {
	for _, blessing := range tu.OkAppBlessings {
		for _, name := range tu.OkDbNames {
			id := wire.Id{blessing, name}
			enc := util.EncodeId(id)
			dec, err := util.DecodeId(enc)
			if err != nil {
				t.Fatalf("decode failed: %v, %v", id, err)
			}
			if id != dec {
				t.Errorf("%v should equal %v", id, dec)
			}
		}
	}
}

func TestValidNameFuncs(t *testing.T) {
	for _, a := range tu.OkAppBlessings {
		for _, d := range tu.OkDbNames {
			id := wire.Id{Blessing: a, Name: d}
			if !util.ValidDatabaseId(id) {
				t.Errorf("%v should be valid", id)
			}
		}
	}
	for _, a := range tu.NotOkAppBlessings {
		for _, d := range append(tu.OkDbNames, tu.NotOkDbNames...) {
			id := wire.Id{Blessing: a, Name: d}
			if util.ValidDatabaseId(id) {
				t.Errorf("%v should be invalid", id)
			}
		}
	}
	for _, d := range tu.NotOkDbNames {
		for _, a := range append(tu.OkAppBlessings, tu.NotOkAppBlessings...) {
			id := wire.Id{Blessing: a, Name: d}
			if util.ValidDatabaseId(id) {
				t.Errorf("%v should be invalid", id)
			}
		}
	}
	for _, s := range tu.OkCollectionNames {
		if !util.ValidCollectionName(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range tu.NotOkCollectionNames {
		if util.ValidCollectionName(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
	for _, s := range tu.OkRowKeys {
		if !util.ValidRowKey(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range tu.NotOkRowKeys {
		if util.ValidRowKey(s) {
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
