// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import "strings"

type b739stage int

const (
	b739Stage1 b739stage = 1 // Use colon in strings but slashes in certificates. All binaries with this stage will respect colons.
	b739Stage2 b739stage = 2 // Use colons in all newly created certificates
	b739Stage3 b739stage = 3 // Final stage: Stop recognizing slashes

	b739 = b739Stage2
)

// Bug739* is used to enable rolling out the change that switches the
// ChainSeparator used in blessings from a slash to a colon.  More details in
// https://github.com/vanadium/issues/issues/739.
//
// The rollout strategy is to rollout all binaries at one stage, flip
// to the next, rollout and repeat. Once all binaries have been rolled
// out with the final stage then remove these functions.
func Bug739Slash2Colon(in string) string {
	switch b739 {
	case b739Stage1:
		fallthrough
	case b739Stage2:
		return strings.Replace(in, "/", ChainSeparator, -1)
	default:
		return in
	}
}

// See documentation for Bug739Slash2Colon.
func Bug739Colon2Slash(in string) string {
	switch b739 {
	case b739Stage1:
		return strings.Replace(in, ChainSeparator, "/", -1)
	case b739Stage2:
		fallthrough
	default:
		return in
	}
}
