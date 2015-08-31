// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"regexp"
	"strings"

	"v.io/v23/context"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
)

// TODO(sadovsky): Expand the allowed charset. We should probably switch to a
// blacklist, reserving just a few special chars like '\0', '*', and '/' but
// otherwise allowing all valid UTF-8 strings.
var nameRegexp *regexp.Regexp = regexp.MustCompile("^[a-zA-Z0-9_.-]+$")

// ValidName returns true iff the given Syncbase component name is valid.
func ValidName(s string) bool {
	return nameRegexp.MatchString(s)
}

// ParseTableRowPair splits the given pattern of the form 'table/row' into
// the table part and the row part. The row part might be empty.
func ParseTableRowPair(ctx *context.T, pattern string) (string, string, error) {
	parts := strings.Split(pattern, "/")
	if len(parts) != 2 {
		return "", "", verror.New(verror.ErrBadArg, ctx, pattern)
	}
	table, prefix := parts[0], parts[1]
	if !ValidName(table) {
		return "", "", verror.New(wire.ErrInvalidName, ctx, table)
	}
	if prefix != "" && !ValidName(prefix) {
		return "", "", verror.New(wire.ErrInvalidName, ctx, prefix)
	}
	return table, prefix, nil
}

// PrefixRangeStart returns the start of the row range for the given prefix.
func PrefixRangeStart(p string) string {
	return p
}

// PrefixRangeLimit returns the limit of the row range for the given prefix.
func PrefixRangeLimit(p string) string {
	// A string is a []byte, i.e. can be thought of as a base-256 number. The code
	// below effectively adds 1 to this number, then chops off any trailing \x00
	// bytes. If the input string consists entirely of \xff bytes, we return an
	// empty string.
	x := []byte(p)
	for len(x) > 0 {
		if x[len(x)-1] == 255 {
			x = x[:len(x)-1] // chop off trailing \x00
		} else {
			x[len(x)-1] += 1 // add 1
			break            // no carry
		}
	}
	return string(x)
}

// IsPrefix returns true if start and limit strings together represent a prefix
// range. If true, start represents the prefix.
func IsPrefix(start string, limit string) bool {
	return PrefixRangeLimit(start) == limit
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
