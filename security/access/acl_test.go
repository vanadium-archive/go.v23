// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package access

import (
	"bytes"
	"reflect"
	"testing"

	"v.io/v23/security"
)

func TestInclude(t *testing.T) {
	acl := AccessList{
		In:    []security.BlessingPattern{"alice/$", "alice/friend", "bob/family"},
		NotIn: []string{"alice/friend/carol", "bob/family/mallory"},
	}
	type V []string // shorthand
	tests := []struct {
		Blessings []string
		Want      bool
	}{
		{nil, false}, // No blessings presented, cannot access
		{V{}, false},
		{V{"alice"}, true},
		{V{"bob"}, false},
		{V{"carol"}, false},
		{V{"alice/colleague"}, false},
		{V{"alice", "carol/friend"}, true}, // Presenting one blessing that grants access is sufficient
		{V{"alice/friend/bob"}, true},
		{V{"alice/friend/carol"}, false},        // alice/friend/carol is blacklisted
		{V{"alice/friend/carol/family"}, false}, // alice/friend/carol is blacklisted, thus her delegates must be too.
		{V{"alice/friend/bob", "alice/friend/carol"}, true},
		{V{"bob/family/eve", "bob/family/mallory"}, true},
		{V{"bob/family/mallory", "alice/friend/carol"}, false},
	}
	for _, test := range tests {
		if got, want := acl.Includes(test.Blessings...), test.Want; got != want {
			t.Errorf("Includes(%v): Got %v, want %v", test.Blessings, got, want)
		}
	}
}

func TestOpenAccessList(t *testing.T) {
	acl := AccessList{In: []security.BlessingPattern{security.AllPrincipals}}
	if !acl.Includes() {
		t.Errorf("OpenAccessList should allow principals that present no blessings")
	}
	if !acl.Includes("frank") {
		t.Errorf("OpenAccessList should allow principals that present any blessings")
	}
}

func TestPermissionsSerialization(t *testing.T) {
	obj := Permissions{
		"R": AccessList{
			In:    []security.BlessingPattern{"foo", "bar"},
			NotIn: []string{"bar/baz"},
		},
		"W": AccessList{
			In:    []security.BlessingPattern{"foo", "bar/$"},
			NotIn: []string{"foo/bar", "foo/baz/boz"},
		},
	}
	txt := `
{
	"R": {
		"In":["foo","bar"],
		"NotIn":["bar/baz"]
	},
	"W": {
		"In":["foo","bar/$"],
		"NotIn":["foo/bar","foo/baz/boz"]
	}
}
`
	if got, err := ReadPermissions(bytes.NewBufferString(txt)); err != nil || !reflect.DeepEqual(got, obj) {
		t.Errorf("Got error %v, Permissions: %v, want %v", err, got, obj)
	}
	// And round-trip (don't compare with 'txt' because indentation/spacing might differ).
	var buf bytes.Buffer
	if err := obj.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadPermissions(&buf); err != nil || !reflect.DeepEqual(got, obj) {
		t.Errorf("Got error %v, Permissions: %v, want %v", err, got, obj)
	}
}
