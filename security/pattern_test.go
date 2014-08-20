package security

import (
	"testing"
	"time"
)

func bless(blessee PublicID, blesser PrivateID, name string) PublicID {
	blessed, err := blesser.Bless(blessee, name, 5*time.Minute, nil)
	if err != nil {
		panic(err)
	}
	return blessed
}

func TestMatchedBy(t *testing.T) {
	annPrivateID := FakePrivateID("ann")
	bobPrivateID := FakePrivateID("bob")
	ann := annPrivateID.PublicID()
	bob := bobPrivateID.PublicID()
	annFriend := bless(bob, annPrivateID, "friend")
	// tests if a nameless publicID can access *
	nameless := FakePublicID("")

	tests := map[PrincipalPattern][]PublicID{
		"*":               {ann, bob, annFriend, nameless},
		"fake/*":          {ann, bob, annFriend},
		"fake/ann":        {ann},
		"fake/bob":        {bob},
		"fake/ann/friend": {ann, annFriend},
		"fake/ann/*":      {ann, annFriend},
	}
	for p, test := range tests {
		pidToMatch := map[PublicID]bool{
			ann: false, bob: false, annFriend: false, nameless: false,
		}
		for _, pid := range test {
			pidToMatch[pid] = true
		}
		for pid, match := range pidToMatch {
			if p.MatchedBy(pid) != match {
				t.Errorf("%q.MatchedBy(%v) was not %v", p, pid, match)
			}
		}
	}
}
