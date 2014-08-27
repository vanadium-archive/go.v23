package security

import (
	"strings"
)

// MatchedBy returns true iff one of the presented blessings matches
// p as per the rules described in documentation for the BlessingPattern type.
func (p BlessingPattern) MatchedBy(blessings ...string) bool {
	if len(p) == 0 {
		return false
	}
	if p == AllPrincipals {
		return true
	}
	parts := strings.Split(string(p), ChainSeparator)
	var glob bool
	if parts[len(parts)-1] == string(AllPrincipals) {
		glob = true
		parts = parts[:len(parts)-1]
	}
	for _, b := range blessings {
		if p.matchedByBlessing(parts, glob, b) {
			return true
		}
	}
	return false
}

func (BlessingPattern) matchedByBlessing(patternchain []string, glob bool, b string) bool {
	// links of the delegation chain in a blessing
	blessingchain := strings.Split(b, ChainSeparator)
	if glob && len(blessingchain) > len(patternchain) {
		// ignore parts of the blessing chain that will match the "/*" component of the pattern.
		blessingchain = blessingchain[:len(patternchain)]
	}
	if len(blessingchain) > len(patternchain) {
		return false
	}
	// At this point, len(blessingchain) <= len(patternchain)
	for i, part := range blessingchain {
		if patternchain[i] != part {
			return false
		}
	}
	return true
}
