package security

// CanAccess tests whether the provided name has the desired access.
func (acl ACL) CanAccess(name string, label Label) bool {
	// TODO(m3b,tilaks): consult group ACLs.
	in := false
	for pattern, labels := range acl.In.Principals {
		if labels.HasLabel(label) && pattern.isDelegateOf(name) {
			in = true
			break
		}
	}
	if !in {
		return false
	}
	notIn := false
	for pattern, labels := range acl.NotIn.Principals {
		if labels.HasLabel(label) && pattern.isBlesserOf(name) {
			notIn = true
			break
		}
	}
	return in && !notIn
}
