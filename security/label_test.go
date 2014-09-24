package security

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

const emptyLabelSet LabelSet = 0

type lSet []Label

func (s lSet) has(labels []Label) bool {
	for _, x := range s {
		for _, label := range labels {
			if x == label {
				return true
			}
		}
	}
	return false
}

func combinations(labels []Label) [][]Label {
	l := uint(len(labels))
	n := uint(0x1 << l)
	combinations := make([][]Label, n-1)
	for b := uint(1); b < n; b++ {
		combination := make([]Label, 0)
		for i := uint(0); i < l; i++ {
			if (b>>i)&1 == 1 {
				combination = append(combination, labels[l-1-i])
			}
		}
		combinations[b-1] = combination
	}
	return combinations
}

func TestHasLabel(t *testing.T) {
	// writeAdminLabel is a new label created for testing purposes.
	writeAdminLabel := Label(12)
	// combinations contains all sets of labels on which HasLabel is tested.
	combinations := combinations(append(ValidLabels, writeAdminLabel))
	testData := []struct {
		labelSet LabelSet
		want     []Label
	}{
		{emptyLabelSet, nil},
		{LabelSet(ResolveLabel), []Label{ResolveLabel}},
		{LabelSet(ReadLabel), []Label{ReadLabel}},
		{LabelSet(ReadLabel | WriteLabel), []Label{ReadLabel, WriteLabel}},
		{LabelSet(writeAdminLabel), []Label{WriteLabel, AdminLabel, writeAdminLabel}},
		{LabelSet(DebugLabel | MonitoringLabel), []Label{DebugLabel, MonitoringLabel}},
		{LabelSet(AdminLabel | DebugLabel | MonitoringLabel), []Label{AdminLabel, DebugLabel, MonitoringLabel}},
		{LabelSet(ReadLabel | WriteLabel | AdminLabel | DebugLabel), []Label{ReadLabel, WriteLabel, AdminLabel, DebugLabel, writeAdminLabel}},
		{LabelSet(ReadLabel | WriteLabel | AdminLabel | DebugLabel | MonitoringLabel), []Label{ReadLabel, WriteLabel, AdminLabel, DebugLabel, MonitoringLabel, writeAdminLabel}},
		{LabelSet(ResolveLabel | ReadLabel | WriteLabel | AdminLabel | DebugLabel | MonitoringLabel), []Label{ResolveLabel, ReadLabel, WriteLabel, AdminLabel, DebugLabel, MonitoringLabel, writeAdminLabel}},
	}
	for _, d := range testData {
		for _, combination := range combinations {
			got := d.labelSet.HasLabel(combination[0], combination[1:]...)
			want := lSet(d.want).has(combination)
			if got != want {
				t.Errorf("0x%x.HasLabel(%v): got: %t, want: %t", uint(d.labelSet), combination, got, want)
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
