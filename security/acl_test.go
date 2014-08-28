package security

import (
	"encoding/json"
	"testing"
)

func TestCanAccess(t *testing.T) {
	const (
		acl1 = `{
		"In": {
			"ann": "RW",
			"bob": "RW",
			"che": "R"
		},
		"NotIn": {
			"bob": "W",
			"dan": "R"
		}
	}`
		acl2 = `{
		"In": {
			"...": "RW"
		},
		"NotIn": {
			"ann/friend": "W"
		}
	}`
		acl3 = `{
		"In": {
			"...": "RW"
		},
		"NotIn": {
			"ann": "W"
		}
	}`
		acl4 = `{
		"In": {
			"ann/...": "RW"
		},
		"NotIn": {
			"ann/friend": "W"
		}
	}`
		acl5 = `{
		"In": {
			"...": "RW"
		}
	}`
	)

	tests := map[string][]struct {
		Name, Access string
	}{
		acl1: {
			{"ann", "RW"},
			{"ann/friend", ""},
			{"bob", "R"},
			{"che", "R"},
			{"dan", ""},
		},
		acl2: {
			{"", "RW"},
			{"ann", "RW"},
			{"ann/friend", "R"},
			{"ann/friend/spouse", "R"},
			{"bob", "RW"},
		},
		acl3: {
			{"", "RW"},
			{"ann", "R"},
			{"ann/friend", "R"},
			{"bob", "RW"},
		},
		acl4: {
			{"ann", "RW"},
			{"ann/friend", "R"},
			{"ann/enemy", "RW"},
			{"ann/friend/spouse", "R"},
			{"bob", ""},
		},
		acl5: {
			{"", "RW"},
			{"ann", "RW"},
			{"bob", "RW"},
		},
	}

	for aclstring, entries := range tests {
		var acl ACL
		if err := json.Unmarshal([]byte(aclstring), &acl); err != nil {
			t.Errorf("json.Unmarshal(%q,%T) failed: %v", aclstring, acl, err)
			continue
		}
		for _, e := range entries {
			var access LabelSet
			if err := json.Unmarshal([]byte("\""+e.Access+"\""), &access); err != nil {
				t.Errorf("json.Unmarshal(%q, %T) failed: %v", e.Access, access, err)
				continue
			}
			for _, label := range ValidLabels {
				if got, want := acl.CanAccess(e.Name, label), access.HasLabel(label); got != want {
					t.Errorf("Got %v, want %v for CanAccess(%q, %v) on ACL %v", got, want, e.Name, label, acl)
				}
			}
		}
	}
}
