package security

import (
	"strings"
)

// MatchedBy returns true iff one of the presented blessings matches
// p as per the rules described in documentation for the BlessingPattern type.
func (p BlessingPattern) MatchedBy(blessings ...string) bool {
	if len(p) == 0 || !p.IsValid() {
		return false
	}
	if p == AllPrincipals {
		return true
	}
	parts := strings.Split(string(p), ChainSeparator)
	glob := true
	if parts[len(parts)-1] == string(NoExtension) {
		glob = false
		parts = parts[:len(parts)-1]
	}
	for _, b := range blessings {
		if matchedByBlessing(parts, glob, b) {
			return true
		}
	}
	return false
}

// IsValid returns true iff the BlessingPattern is well formed, as per the
// rules described in documentation for the BlessingPattern type.
func (p BlessingPattern) IsValid() bool {
	if len(p) == 0 {
		return false
	}
	if p == AllPrincipals {
		return true
	}
	parts := strings.Split(string(p), ChainSeparator)
	if parts[len(parts)-1] == string(NoExtension) {
		parts = parts[:len(parts)-1]
	}
	for _, e := range parts {
		if validateExtension(e) != nil {
			return false
		}
	}
	return true
}

// MakeNonExtendable returns a pattern that are not matched by any extension of the blessing
// represented by p.
//
// For example:
//   onlyAlice := BlessingPattern("google/alice").MakeNonExtendable()
//   onlyAlice.MatchedBy("google")  // Returns true
//   onlyAlice.MatchedBy("google/alice")  // Returns true
//   onlyAlice.MatchedBy("google/alice/bob")  // Returns false
func (p BlessingPattern) MakeNonExtendable() BlessingPattern {
	if len(p) == 0 || p == BlessingPattern(NoExtension) {
		return BlessingPattern(NoExtension)
	}
	if strings.HasSuffix(string(p), ChainSeparator+string(NoExtension)) {
		return p
	}
	return BlessingPattern(string(p) + ChainSeparator + string(NoExtension))
}

func matchedByBlessing(patternchain []string, glob bool, b string) bool {
	// links of the delegation chain in a blessing
	blessingchain := strings.Split(b, ChainSeparator)
	if glob && len(blessingchain) > len(patternchain) {
		// ignore parts of the blessing chain that will match extensions of the pattern.
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
