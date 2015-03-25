// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"fmt"
	"regexp"
	"strings"
	"v.io/v23/naming"
)

// Syntax is /{endpoint}/__({pattern})/{name}
var namePatternRegexp = regexp.MustCompile(`^__\(([^)]*)\)($|/)(.*)`)

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

// MakeNonExtendable returns a pattern that is matched exactly
// by the blessing specified by the given pattern string.
//
// For example:
//   onlyAlice := BlessingPattern("google/alice").MakeNonExtendable()
//   onlyAlice.MatchedBy("google/alice")  // Returns true
//   onlyAlice.MatchedBy("google")  // Returns false
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

// PrefixPatterns returns a set of BlessingPatterns that are matched by
// blessings that either directly match the provided pattern or can be
// extended to match the provided pattern.
//
// For example:
// BlessingPattern("google/alice/friend").PrefixPatterns() returns
//   ["google/$", "google/alice/$", "google/alice/friend"]
// BlessingPattern("google/alice/friend/$").PrefixPatterns() returns
//   ["google/$", "google/alice/$", "google/alice/friend/$"]
//
// The returned set of BlessingPatterns are ordered by the number of
// "/"-separated components in the pattern.
func (p BlessingPattern) PrefixPatterns() []BlessingPattern {
	if p == NoExtension {
		return []BlessingPattern{p}
	}
	parts := strings.Split(string(p), ChainSeparator)
	if parts[len(parts)-1] == string(NoExtension) {
		parts = parts[:len(parts)-2]
	} else {
		parts = parts[:len(parts)-1]
	}
	var ret []BlessingPattern
	for i := 0; i < len(parts); i++ {
		ret = append(ret, BlessingPattern(strings.Join(parts[:i+1], ChainSeparator)).MakeNonExtendable())
	}
	return append(ret, p)
}

func matchedByBlessing(patternchain []string, glob bool, b string) bool {
	// links of the delegation chain in a blessing
	blessingchain := strings.Split(b, ChainSeparator)
	if len(blessingchain) < len(patternchain) {
		return false
	}
	if glob && len(blessingchain) > len(patternchain) {
		// Ignore parts of the blessing chain that will match extensions of the pattern.
		blessingchain = blessingchain[:len(patternchain)]
	}
	if len(blessingchain) > len(patternchain) {
		return false
	}
	// At this point, len(blessingchain) == len(patternchain)
	for i, part := range blessingchain {
		if patternchain[i] != part {
			return false
		}
	}
	return true
}

// SplitPatternName takes an object name and parses out the server blessing pattern.
// It returns the pattern specified, and the name with the pattern removed.
func SplitPatternName(origName string) (BlessingPattern, string) {
	rooted := naming.Rooted(origName)
	ep, name := naming.SplitAddressName(origName)
	match := namePatternRegexp.FindStringSubmatch(name)
	if len(match) == 0 {
		return BlessingPattern(""), origName
	}

	pattern := BlessingPattern(match[1])
	name = naming.Clean(match[3])
	if rooted {
		name = naming.JoinAddressName(ep, name)
	}
	return pattern, name
}

// JoinPatternName embeds the specified pattern into a name.
func JoinPatternName(pattern BlessingPattern, name string) string {
	if len(pattern) == 0 {
		return name
	}
	ep, rel := naming.SplitAddressName(name)
	return naming.JoinAddressName(ep, fmt.Sprintf("__(%s)/%s", pattern, rel))
}
