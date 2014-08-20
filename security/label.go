package security

import (
	"bytes"
	"encoding/json"
	"fmt"
)

var (
	// ValidLabels is the set of all valid Labels for IPC methods.
	ValidLabels = []Label{ResolveLabel, ReadLabel, WriteLabel, AdminLabel, DebugLabel, MonitoringLabel}

	// AllLabels is a LabelSet containing all ValidLabels.
	AllLabels = LabelSet(ResolveLabel | ReadLabel | WriteLabel | AdminLabel | DebugLabel | MonitoringLabel)
)

// IsValidLabel tests whether a label is a member of the defined valid set.
func IsValidLabel(l Label) bool {
	for _, s := range ValidLabels {
		if s == l {
			return true
		}
	}
	return false
}

// String representation of a Label.
func (l Label) String() string {
	switch l {
	case ResolveLabel:
		return "X"
	case ReadLabel:
		return "R"
	case WriteLabel:
		return "W"
	case AdminLabel:
		return "A"
	case DebugLabel:
		return "D"
	case MonitoringLabel:
		return "M"
	}
	return ""
}

// HasLabel tests whether a LabelSet contains a Label.
func (ls LabelSet) HasLabel(l Label) bool {
	return (ls & LabelSet(l)) != 0
}

// String representation of a LabelSet.
func (ls LabelSet) String() string {
	b := bytes.NewBufferString("")
	for _, l := range ValidLabels {
		if ls.HasLabel(l) {
			b.WriteString(l.String())
		}
	}
	return b.String()
}

func (ls *LabelSet) fromString(s string) error {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case 'X', 'x':
			*ls |= LabelSet(ResolveLabel)
		case 'R', 'r':
			*ls |= LabelSet(ReadLabel)
		case 'W', 'w':
			*ls |= LabelSet(WriteLabel)
		case 'A', 'a':
			*ls |= LabelSet(AdminLabel)
		case 'D', 'd':
			*ls |= LabelSet(DebugLabel)
		case 'M', 'm':
			*ls |= LabelSet(MonitoringLabel)
		default:
			return fmt.Errorf("invalid label: %q", s[i])
		}
	}
	return nil
}

// MarshalJSON implements the JSON.Marshaler interface. The returned JSON encoding
// is essentially a string over the set of characters {R, W, A, D, M}. The string
// contains character R iff ls has ReadLabel, W iff ls has WriteLabel, A iff ls
// has Adminlabel, D iff ls has DebugLabel, and M iff ls has MonitoringLabel.
func (ls LabelSet) MarshalJSON() ([]byte, error) {
	return json.Marshal(ls.String())
}

// UnmarshalJSON implements the JSON.Marshaler interface. The function is case-insensitive
// on the provided string encoding.
func (ls *LabelSet) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	return ls.fromString(s)
}
