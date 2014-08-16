package security

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"
)

const emptyLabelSet LabelSet = 0

type lSet []Label

func (s lSet) has(l Label) bool {
	for _, x := range s {
		if x == l {
			return true
		}
	}
	return false
}

func bless(blessee PublicID, blesser PrivateID, name string) PublicID {
	blessed, err := blesser.Bless(blessee, name, 5*time.Minute, nil)
	if err != nil {
		panic(err)
	}
	return blessed
}

func TestHasLabel(t *testing.T) {
	// writeAdminLabel is a new label created for testing purposes.
	writeAdminLabel := Label(12)
	// labels is the set of labels on which HasLabel is tested.
	labels := append(ValidLabels, writeAdminLabel)
	testData := []struct {
		labelSet LabelSet
		want     []Label
	}{
		{emptyLabelSet, nil},
		{LabelSet(ResolveLabel), []Label{ResolveLabel}},
		{LabelSet(ReadLabel), []Label{ReadLabel}},
		{LabelSet(writeAdminLabel), []Label{WriteLabel, AdminLabel, writeAdminLabel}},
		{LabelSet(ReadLabel | WriteLabel), []Label{ReadLabel, WriteLabel, writeAdminLabel}},
		{LabelSet(DebugLabel | MonitoringLabel), []Label{DebugLabel, MonitoringLabel}},
		{LabelSet(AdminLabel | DebugLabel | MonitoringLabel), []Label{AdminLabel, DebugLabel, MonitoringLabel, writeAdminLabel}},
		{LabelSet(ReadLabel | WriteLabel | AdminLabel | DebugLabel), []Label{ReadLabel, WriteLabel, AdminLabel, DebugLabel, writeAdminLabel}},
		{LabelSet(ReadLabel | WriteLabel | AdminLabel | DebugLabel | MonitoringLabel), []Label{ReadLabel, WriteLabel, AdminLabel, DebugLabel, MonitoringLabel, writeAdminLabel}},
		{LabelSet(ResolveLabel | ReadLabel | WriteLabel | AdminLabel | DebugLabel | MonitoringLabel), []Label{ResolveLabel, ReadLabel, WriteLabel, AdminLabel, DebugLabel, MonitoringLabel, writeAdminLabel}},
	}
	for _, d := range testData {
		for _, l := range labels {
			if got, want := d.labelSet.HasLabel(l), lSet(d.want).has(l); got != want {
				t.Errorf("0x%x.HasLabel(0x%x): got: %t, want: %t", uint(d.labelSet), uint(l), got, want)
			}
		}
	}
}

func TestLabelSetRoundTripJSON(t *testing.T) {
	testData := []LabelSet{
		emptyLabelSet,
		LabelSet(ResolveLabel),
		LabelSet(ReadLabel),
		LabelSet(ResolveLabel | ReadLabel),
		LabelSet(ReadLabel | WriteLabel),
		LabelSet(AdminLabel | DebugLabel | MonitoringLabel),
		LabelSet(WriteLabel | AdminLabel | DebugLabel | MonitoringLabel),
		LabelSet(ReadLabel | WriteLabel | AdminLabel | DebugLabel | MonitoringLabel),
		LabelSet(ResolveLabel | ReadLabel | WriteLabel | AdminLabel | DebugLabel | MonitoringLabel),
	}
	for _, ls := range testData {
		marshalled, err := json.Marshal(ls)
		if err != nil {
			t.Fatalf("json.Marshal(0x%x) failed: %v", uint(ls), err)
		}
		var unmarshalled LabelSet
		if err := json.Unmarshal(marshalled, &unmarshalled); err != nil {
			t.Errorf("json.Unmarshal(%q) failed: %v", marshalled, err)
		}
		if !reflect.DeepEqual(ls, unmarshalled) {
			t.Errorf("0x%x: JSON round-trip produced 0x%x", ls, unmarshalled)
		}
	}
}

func TestLabelSetUnmarshalJSON(t *testing.T) {
	testData := []struct {
		marshalled string
		labelSet   LabelSet
		err        string
	}{
		{
			marshalled: `""`,
			labelSet:   emptyLabelSet,
		},
		{
			marshalled: `"X"`,
			labelSet:   LabelSet(ResolveLabel),
		},
		{
			marshalled: `"R"`,
			labelSet:   LabelSet(ReadLabel),
		},
		{
			marshalled: `"RW"`,
			labelSet:   LabelSet(ReadLabel | WriteLabel),
		},
		{
			marshalled: `"WR"`,
			labelSet:   LabelSet(ReadLabel | WriteLabel),
		},
		{
			marshalled: `"RWADM"`,
			labelSet:   LabelSet(ReadLabel | WriteLabel | AdminLabel | DebugLabel | MonitoringLabel),
		},
		{
			marshalled: `"DAWRM"`,
			labelSet:   LabelSet(ReadLabel | WriteLabel | AdminLabel | DebugLabel | MonitoringLabel),
		},
		{
			marshalled: `"AWDMR"`,
			labelSet:   LabelSet(ReadLabel | WriteLabel | AdminLabel | DebugLabel | MonitoringLabel),
		},
		{
			marshalled: `"AWYMR"`,
			err:        fmt.Sprintf("invalid label: %q", 'Y'),
		},
		{
			marshalled: `"a"`,
			labelSet:   LabelSet(AdminLabel),
		},
		{
			marshalled: `"dm"`,
			labelSet:   LabelSet(DebugLabel | MonitoringLabel),
		},
		{
			marshalled: `"mD"`,
			labelSet:   LabelSet(DebugLabel | MonitoringLabel),
		},
		{
			marshalled: `"dRm"`,
			labelSet:   LabelSet(ReadLabel | DebugLabel | MonitoringLabel),
		},
		{
			marshalled: `"WrAm"`,
			labelSet:   LabelSet(ReadLabel | WriteLabel | AdminLabel | MonitoringLabel),
		},
		{
			marshalled: `"MrdwA"`,
			labelSet:   LabelSet(ReadLabel | WriteLabel | AdminLabel | DebugLabel | MonitoringLabel),
		},
	}
	for _, d := range testData {
		var labelSet LabelSet
		err := json.Unmarshal([]byte(d.marshalled), &labelSet)
		// If err != nil, should match d.err
		if err != nil {
			if err.Error() != d.err {
				t.Errorf("json.Unmarshal([]byte(%q)): got error [%s], want [%s]", d.marshalled, err, d.err)
			}
			continue
		}
		// If d.err is specified, then err should not have been nil
		if len(d.err) > 0 {
			t.Errorf("json.Unmarshal([]byte(%q)): got error [%s], want [%s]", d.marshalled, err, d.err)
			continue
		}
		// Compare labelSets
		if got, want := labelSet, d.labelSet; got != want {
			t.Errorf("json.Unmarshal([]byte(%q)): got: %d, want: %d", d.marshalled, got, want)
		}
	}
}

func TestACLMatches(t *testing.T) {
	annPrivateID := FakePrivateID("ann")
	bobPrivateID := FakePrivateID("bob")
	chePrivateID := FakePrivateID("che")
	danPrivateID := FakePrivateID("dan")
	evaPrivateID := FakePrivateID("eva")
	ann := annPrivateID.PublicID()
	bob := bobPrivateID.PublicID()
	che := chePrivateID.PublicID()
	dan := danPrivateID.PublicID()
	eva := evaPrivateID.PublicID()
	annFriend := bless(bob, annPrivateID, "friend")

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
		Principal PublicID
		Label     Label
		Match     bool
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
		acl, err := LoadACL(bytes.NewBufferString(aclstring))
		if err != nil {
			t.Fatalf("Cannot parse ACL %s: %v", aclstring, err)
		}
		for _, test := range tests {
			if acl.Matches(test.Principal, test.Label) != test.Match {
				t.Errorf("acl.Matches(%v, %v) was not %v", test.Principal, test.Label, test.Match)
			}
		}
	}
}

func TestLoadSaveIdentity(t *testing.T) {
	id := FakePrivateID("test")

	var buf bytes.Buffer
	if err := SaveIdentity(&buf, id); err != nil {
		t.Fatalf("Failed to save PrivateID %q: %v", id, err)
	}

	loadedID, err := LoadIdentity(&buf)
	if err != nil {
		t.Fatalf("Failed to load PrivateID: %v", err)
	}
	if !reflect.DeepEqual(loadedID, id) {
		t.Fatalf("Got Identity %v, but want %v", loadedID, id)
	}
}

func TestLoadSaveACL(t *testing.T) {
	acl := ACL{}
	acl.In.Principals = map[PrincipalPattern]LabelSet{
		"veyron/*":     LabelSet(ReadLabel),
		"veyron/alice": LabelSet(ReadLabel | WriteLabel),
		"veyron/bob":   LabelSet(AdminLabel),
	}
	acl.NotIn.Principals = map[PrincipalPattern]LabelSet{
		"veyron/che": LabelSet(ReadLabel),
	}

	var buf bytes.Buffer
	if err := SaveACL(&buf, acl); err != nil {
		t.Fatalf("Failed to save ACL %q: %v", acl, err)
	}

	loadedACL, err := LoadACL(&buf)
	if err != nil {
		t.Fatalf("Failed to load ACL: %v", err)
	}
	if !reflect.DeepEqual(loadedACL, acl) {
		t.Fatalf("Got ACL %v, but want %v", loadedACL, acl)
	}
}
