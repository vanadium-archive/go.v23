package security

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCanAccess(t *testing.T) {
	ann := "fake/ann"
	bob := "fake/bob"
	che := "fake/che"
	dan := "fake/dan"
	eva := "fake/eva"
	annFriend := "fake/ann/friend"
	// tests if a nameless publicID can access *
	nameless := ""

	aclstring1 := `{
		"In": { "Principals": {
			"fake/ann": "RW",
			"fake/bob": "RW",
			"fake/che": "R"
		}},
		"NotIn": { "Principals": {
			"fake/bob": "W",
			"fake/dan": "R"
		}}
	}`
	aclstring2 := `{
		"In": { "Principals": {
			"*": "RW"
		}},
		"NotIn": { "Principals": {
			"fake/ann/friend": "W"
		}}
	}`
	aclstring3 := `{
		"In": { "Principals": {
			"*": "RW"
		}},
		"NotIn": { "Principals": {
			"fake/ann/*": "W"
		}}
	}`
	aclstring4 := `{
		"In": { "Principals": {
			"fake/ann/*": "RW"
		}},
		"NotIn": { "Principals": {
			"fake/ann/friend": "W"
		}}
	}`
	aclToTests := map[string][]struct {
		Name  string
		Label Label
		Match bool
	}{
		aclstring1: {
			{ann, ReadLabel, true},
			{ann, WriteLabel, true},
			{annFriend, ReadLabel, false},
			{annFriend, WriteLabel, false},
			{bob, ReadLabel, true},
			{bob, WriteLabel, false},
			{che, ReadLabel, true},
			{che, WriteLabel, false},
			{dan, ReadLabel, false},
			{dan, WriteLabel, false},
			{eva, ReadLabel, false},
			{eva, WriteLabel, false},
		},
		aclstring2: {
			{ann, ReadLabel, true},
			{ann, WriteLabel, true},
			{annFriend, ReadLabel, true},
			{annFriend, WriteLabel, false},
			{bob, ReadLabel, true},
			{bob, WriteLabel, true},
		},
		aclstring3: {
			{nameless, ReadLabel, true},
			{ann, ReadLabel, true},
			{ann, WriteLabel, false},
			{annFriend, ReadLabel, true},
			{annFriend, WriteLabel, false},
			{bob, ReadLabel, true},
			{bob, WriteLabel, true},
		},
		aclstring4: {
			{ann, ReadLabel, true},
			{ann, WriteLabel, true},
			{annFriend, ReadLabel, true},
			{annFriend, WriteLabel, false},
			{bob, ReadLabel, false},
			{bob, WriteLabel, false},
		},
	}
	for aclstring, tests := range aclToTests {
		var acl ACL
		if err := json.NewDecoder(bytes.NewBufferString(aclstring)).Decode(&acl); err != nil {
			t.Fatalf("Cannot parse ACL %s: %v", aclstring, err)
		}
		for _, test := range tests {
			if acl.CanAccess(test.Name, test.Label) != test.Match {
				t.Errorf("acl.CanAccess(%v, %v) was not %v", test.Name, test.Label, test.Match)
			}
		}
	}
}
