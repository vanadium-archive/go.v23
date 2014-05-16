package security

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"veyron2/vom"
)

var (
	// ValidLabels is the set of all valid Labels for IPC methods.
	ValidLabels = []Label{ReadLabel, WriteLabel, AdminLabel, DebugLabel, MonitoringLabel}

	// AllLabels is a LabelSet containing all ValidLabels.
	AllLabels = LabelSet(ReadLabel | WriteLabel | AdminLabel | DebugLabel | MonitoringLabel)
)

// String representation of a Label.
func (l Label) String() string {
	switch l {
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

// MarshalJSON implements the JSON.Marshaler interface. The returned JSON encoding
// is essentially a string over the set of characters {R, W, A, D, M}. The string
// contains character R iff ls has ReadLabel, W iff ls has WriteLabel, A iff ls
// has Adminlabel, D iff ls has DebugLabel, and M iff ls has MonitoringLabel.
func (ls LabelSet) MarshalJSON() ([]byte, error) {
	return json.Marshal(ls.String())
}

func (ls *LabelSet) fromString(s string) error {
	for i := 0; i < len(s); i++ {
		switch s[i] {
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

// UnmarshalJSON implements the JSON.Marshaler interface. The function is case-insensitive
// on the provided string encoding.
func (ls *LabelSet) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	return ls.fromString(s)
}

// IsValidLabel tests whether a label is a member of the defined valid set.
func IsValidLabel(l Label) bool {
	for _, s := range ValidLabels {
		if s == l {
			return true
		}
	}
	return false
}

// UniversalCaveat takes a Caveat and returns a ServiceCaveat bound to all principals.
func UniversalCaveat(cav Caveat) ServiceCaveat {
	return ServiceCaveat{Service: AllPrincipals, Caveat: cav}
}

// LoadIdentity reads a PrivateID from the provided Reader containing a Base64VOM encoded PrivateID.
func LoadIdentity(r io.Reader) (PrivateID, error) {
	var id PrivateID
	if err := vom.NewDecoder(base64.NewDecoder(base64.URLEncoding, r)).Decode(&id); err != nil {
		return nil, err
	}
	return id, nil
}

// SaveIdentity encodes a PrivateID in Base64VOM format and writes it to the provided Writer.
func SaveIdentity(w io.Writer, id PrivateID) error {
	closer := base64.NewEncoder(base64.URLEncoding, w)
	if err := vom.NewEncoder(closer).Encode(id); err != nil {
		return err
	}
	// Must close the base64 encoder to flush out any partially written
	// blocks.
	if err := closer.Close(); err != nil {
		return err
	}
	return nil
}

// LoadACL reads an ACL from the provided Reader containing a JSON encoded ACL.
func LoadACL(r io.Reader) (ACL, error) {
	var acl ACL
	if err := json.NewDecoder(r).Decode(&acl); err != nil {
		return nil, err
	}
	return acl, nil
}

// SaveACL encodes an ACL in JSON format and writes it to the provided Writer.
func SaveACL(w io.Writer, acl ACL) error {
	return json.NewEncoder(w).Encode(acl)
}
