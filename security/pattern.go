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

// IsValid returns true iff the BlessingPattern is well formed,
// i.e., does not contain any character sequences that will cause
// the BlessingPattern to never match any valid blessings.
func (p BlessingPattern) IsValid() bool {
	parts := strings.Split(string(p), ChainSeparator)
	for i, e := range parts {
		if e == "" {
			return false
		}
		isGlob := e == string(AllPrincipals)
		if isGlob && (i < len(parts)-1) {
			return false
		}
		if !isGlob && validateExtension(e) != nil {
			return false
		}
	}
	return true
}

// MakeGlob returns a pattern that matches all extensions of the blessings that
// are matched by 'p'.
//
// For example:
//   delegates := BlessingPattern("alice").Glob()
//   delegates.MatchedBy("alice")  // Returns true
//   delegates.MatchedBy("alice/friend/bob") // Returns true
func (p BlessingPattern) MakeGlob() BlessingPattern {
	if len(p) == 0 || p == AllPrincipals {
		return AllPrincipals
	}
	if strings.HasSuffix(string(p), ChainSeparator+string(AllPrincipals)) {
		return p
	}
	return BlessingPattern(string(p) + ChainSeparator + string(AllPrincipals))
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
