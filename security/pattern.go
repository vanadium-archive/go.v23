package security

import (
	"strings"
)

// isDelegateOf checks if some name that exactly matches p is equal to or
// blessed by n.
//
// If P is of the form p_0/.../p_k; P.isDelegateOf(N) is true iff N is of the
// form n_0/.../n_m such that m <= k and for all i from 0 to m, p_i = n_i.
//
// If P is of the form p_0/.../p_k/*; P.isDelegateOf(N) is true iff N is of the
// form n_0/.../n_m such that for all i from 0 to min(m, k), p_i = n_i.
func (p PrincipalPattern) isDelegateOf(n string) bool {
	if p == AllPrincipals {
		return true
	}

	pattern := string(p)
	patternParts := strings.Split(pattern, ChainSeparator)
	patternLen := len(patternParts)
	nameParts := strings.Split(n, ChainSeparator)
	nameLen := len(nameParts)

	if patternParts[patternLen-1] != AllPrincipals && nameLen > patternLen {
		return false
	}

	min := nameLen
	if patternParts[patternLen-1] == AllPrincipals && nameLen > patternLen-1 {
		min = patternLen - 1
	}

	for i := 0; i < min; i++ {
		if patternParts[i] != nameParts[i] {
			return false
		}
	}
	return true
}

// isBlesserOf checks if some name that exactly matches p is equal to or a
// blesser of n.
//
// If P is of the form p_0/.../p_k; P.isBlesserOf(N) is true iff N is of the
// form n_0/.../n_m such that m >= k and for all i from 0 to k, p_i = n_i.
//
// If P is of the form p_0/.../p_k/*; P.isBlesserOf(N) is true iff N is of the
// form n_0/.../n_m such that m >= k and for all i from 0 to k, p_i = n_i.
func (p PrincipalPattern) isBlesserOf(n string) bool {
	if p == AllPrincipals {
		return true
	}

	pattern := string(p)
	patternParts := strings.Split(pattern, ChainSeparator)
	patternLen := len(patternParts)
	nameParts := strings.Split(n, ChainSeparator)
	nameLen := len(nameParts)

	min := patternLen
	if patternParts[patternLen-1] == AllPrincipals {
		min = patternLen - 1
	}
	if nameLen < min {
		return false
	}

	for i := 0; i < min; i++ {
		if patternParts[i] != nameParts[i] {
			return false
		}
	}
	return true

}

// MatchedBy checks if some name that exactly matches p is equal to or blessed
// by some name of pid.
// In other words, the principal has a name matching the pattern, or can obtain
// a name matching the pattern by manipulating its PublicID using PrivateID
// operations (e.g., <PrivateID>.Bless(<PublicID>, ..., ...)).
func (p PrincipalPattern) MatchedBy(pid PublicID) bool {
	if p == AllPrincipals {
		return true
	}
	for _, name := range pid.Names() {
		if p.isDelegateOf(name) {
			return true
		}
	}
	return false
}
