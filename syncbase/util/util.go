// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"regexp"

	"v.io/v23/context"
	"v.io/v23/security/access"
)

// TODO(sadovsky): Expand the allowed charset. We should probably switch to a
// blacklist, reserving just a few special chars like '\0' and '/' but otherwise
// allowing all valid UTF-8 strings.
var nameRegexp *regexp.Regexp = regexp.MustCompile("^[a-zA-Z0-9_.-]+$")

// ValidName returns true iff the given Syncbase component name is valid.
func ValidName(s string) bool {
	return nameRegexp.MatchString(s)
}

// AccessController provides access control for various syncbase objects.
type AccessController interface {
	// SetPermissions replaces the current Permissions for an object.
	// For detailed documentation, see Object.SetPermissions.
	SetPermissions(ctx *context.T, perms access.Permissions, version string) error

	// GetPermissions returns the current Permissions for an object.
	// For detailed documentation, see Object.GetPermissions.
	GetPermissions(ctx *context.T) (perms access.Permissions, version string, err error)
}
