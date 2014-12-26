package access

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"v.io/core/veyron2/security"
)

func TestInclude(t *testing.T) {
	acl := ACL{
		In:    []security.BlessingPattern{"alice", "alice/friend/...", "bob/family/..."},
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
		{V{"bob"}, true},
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

func TestOpenACL(t *testing.T) {
	acl := ACL{In: []security.BlessingPattern{security.AllPrincipals}}
	if !acl.Includes() {
		t.Errorf("OpenACL should allow principals that present no blessings")
	}
	if !acl.Includes("frank") {
		t.Errorf("OpenACL should allow principals that present any blessings")
	}
}

func TestTaggedACLMapSerialization(t *testing.T) {
	obj := TaggedACLMap{
		"R": ACL{
			In:    []security.BlessingPattern{"foo/...", "bar/..."},
			NotIn: []string{"bar/baz"},
		},
		"W": ACL{
			In:    []security.BlessingPattern{"foo/...", "bar"},
			NotIn: []string{"foo/bar", "foo/baz/boz"},
		},
	}
	txt := `
{
	"R": {
		"In":["foo/...","bar/..."],
		"NotIn":["bar/baz"]
	},
	"W": {
		"In":["foo/...","bar"],
		"NotIn":["foo/bar","foo/baz/boz"]
	}
}
`
	if got, err := ReadTaggedACLMap(bytes.NewBufferString(txt)); err != nil || !reflect.DeepEqual(got, obj) {
		t.Errorf("Got error %v, TaggedACLMap: %v, want %v", err, got, obj)
	}
	// And round-trip (don't compare with 'txt' because indentation/spacing might differ).
	var buf bytes.Buffer
	if err := obj.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadTaggedACLMap(&buf); err != nil || !reflect.DeepEqual(got, obj) {
		t.Errorf("Got error %v, TaggedACLMap: %v, want %v", err, got, obj)
	}
}

func TestTaggedACLMapConversionFromOldFormat(t *testing.T) {
	allLabels := security.LabelSet(security.AdminLabel | security.ReadLabel | security.WriteLabel | security.DebugLabel | security.MonitoringLabel | security.ResolveLabel)
	oldformat := new(bytes.Buffer)
	if err := json.NewEncoder(oldformat).Encode(security.DeprecatedACL{
		In: map[security.BlessingPattern]security.LabelSet{
			"user/...": allLabels,
			"reader":   security.LabelSet(security.ReadLabel),
		},
		NotIn: map[string]security.LabelSet{
			"user/bad":      allLabels,
			"user/kindabad": security.LabelSet(security.AdminLabel),
		},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadTaggedACLMap(oldformat)
	if err != nil {
		t.Fatal(err)
	}
	want := TaggedACLMap{
		"Admin": ACL{
			In:    []security.BlessingPattern{"user/..."},
			NotIn: []string{"user/bad", "user/kindabad"},
		},
		"Debug": {
			In:    []security.BlessingPattern{"user/..."},
			NotIn: []string{"user/bad"},
		},
		"Read": {
			In:    []security.BlessingPattern{"reader", "user/..."},
			NotIn: []string{"user/bad"},
		},
		"Resolve": {
			In:    []security.BlessingPattern{"user/..."},
			NotIn: []string{"user/bad"},
		},
		"Write": {
			In:    []security.BlessingPattern{"user/..."},
			NotIn: []string{"user/bad"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Got  %#v\nWant %#v", got, want)
	}
}
