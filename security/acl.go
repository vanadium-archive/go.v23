package security

import "strings"

// CanAccess returns true iff the ACL grants blessing access to any operation
// with one of the given labels.
func (acl ACL) CanAccess(blessing string, label Label, additionalLabels ...Label) bool {
	permittedLabels := LabelSet(0)
	// Step 1: blessing should match a pattern in acl.In
	// TODO(ataly,ashankar,hpucha): consult group ACLs.
	for pattern, ls := range acl.In {
		if pattern.MatchedBy(blessing) {
			permittedLabels |= ls
		}
	}
	// Short-circuit.
	if !permittedLabels.HasLabel(label, additionalLabels...) {
		return false
	}
	// Step 2: Check the NotIn list.
	// NotIn denies access to the delegates of all blessings explicitly
	// specified in it.
	const glob = ChainSeparator + string(AllPrincipals)
	pattern := BlessingPattern(blessing)
	for notin, ls := range acl.NotIn {
		if pattern.MatchedBy(strings.TrimSuffix(notin, glob)) {
			permittedLabels &= ^ls
			// Short-circuit.
			if !permittedLabels.HasLabel(label, additionalLabels...) {
				return false
			}
		}
	}
	return true
}
