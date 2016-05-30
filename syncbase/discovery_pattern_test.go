// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"reflect"
	"testing"

	"v.io/v23/security"
)

func TestJoinSplitPatterns(t *testing.T) {
	cases := []struct {
		patterns []security.BlessingPattern
		joined   string
	}{
		{nil, ""},
		{[]security.BlessingPattern{"a", "b"}, "a,b"},
		{[]security.BlessingPattern{"a:b:c", "d:e:f"}, "a:b:c,d:e:f"},
		{[]security.BlessingPattern{"alpha:one", "alpha:two", "alpha:three"}, "alpha:one,alpha:two,alpha:three"},
	}
	for _, c := range cases {
		if got := joinPatterns(c.patterns); got != c.joined {
			t.Errorf("%#v, got %q, wanted %q", c.patterns, got, c.joined)
		}
		if got := splitPatterns(c.joined); !reflect.DeepEqual(got, c.patterns) {
			t.Errorf("%q, got %#v, wanted %#v", c.joined, got, c.patterns)
		}
	}
	// Special case, Joining an empty non-nil list results in empty string.
	if got := joinPatterns([]security.BlessingPattern{}); got != "" {
		t.Errorf("Joining empty list: got %q, want %q", got, "")
	}
}
