package security

import "strings"

// CanAccess returns true iff the ACL provides blessing with access to operations with label.
func (acl ACL) CanAccess(blessing string, label Label) bool {
	// Step 1: blessing should match a pattern in acl.In
	in := false
	// TODO(m3b,tilaks): consult group ACLs.
	for pattern, labels := range acl.In.Principals {
		if labels.HasLabel(label) && pattern.MatchedBy(blessing) {
			in = true
			break
		}
	}
	if !in {
		return false
	}
	// Step 2: Check the NotIn list.
	// NotIn denies access to the delegates of all blessings explicitly
	// specified in it.
	const glob = ChainSeparator + string(AllPrincipals)
	pattern := BlessingPattern(blessing)
	for notin, labels := range acl.NotIn.Principals {
		if labels.HasLabel(label) && pattern.MatchedBy(strings.TrimSuffix(string(notin), glob)) {
			return false
		}
	}
	return true
}
