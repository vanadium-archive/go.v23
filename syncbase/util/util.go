// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"strings"
	"unicode/utf8"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
)

// NameSepWithSlashes is the component name separator used in Syncbase object
// names. For example, rows in Syncbase have object names of the form:
//     <syncbase>/<app>/$/<database>/$/<table>/$/<row>
//
// Note that component names may include "$" characters, e.g. an app may be
// named "foo$bar".
//
// TODO(sadovsky): Consider adding /$/ between <syncbase> and <app>. This would
// have the side effect of making non-app/db/tb/row suffixes less ugly, e.g.
// eliminating the need for "@@" prefix in "@@sync". (But note, we should also
// make it so clients using the client library never have to specify absolute
// names beyond <syncbase>.)
const NameSep = "$"
const NameSepWithSlashes = "/" + NameSep + "/"

// ValidName returns true iff the given Syncbase component name (i.e. app,
// database, table, or row name) is valid. Component names:
//   must be valid UTF-8;
//   must not contain "\x00" or "@@";
//   must not have any slash-separated parts equal to "" or "$"; and
//   must not have any slash-separated parts that start with "__".
func ValidName(s string) bool {
	// TODO(sadovsky): Temporary hack to avoid updating lots of internal tests as
	// part of the change that switches us over to the /$/ hierarchy separator
	// scheme. This check should go away by Sept 21, 2015.
	if strings.Contains(s, ":") {
		return false
	}
	if strings.Contains(s, "\x00") || strings.Contains(s, "@@") || !utf8.ValidString(s) {
		return false
	}
	parts := strings.Split(s, "/")
	for _, v := range parts {
		if v == "" || v == NameSep || strings.HasPrefix(v, naming.ReservedNamePrefix /* __ */) {
			return false
		}
	}
	return true
}

// ParseTableRowPair splits the "<table>/$/<row>" part of a Syncbase object name
// into the table name and the row key. The row key may be empty.
func ParseTableRowPair(ctx *context.T, pattern string) (string, string, error) {
	parts := strings.Split(pattern, NameSepWithSlashes)
	if len(parts) != 2 {
		return "", "", verror.New(verror.ErrBadArg, ctx, pattern)
	}
	table, row := parts[0], parts[1]
	if !ValidName(table) {
		return "", "", verror.New(wire.ErrInvalidName, ctx, table)
	}
	if row != "" && !ValidName(row) {
		return "", "", verror.New(wire.ErrInvalidName, ctx, row)
	}
	return table, row, nil
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

// IsPrefix returns true if start and limit together represent a prefix range.
// If true, start is the represented prefix.
func IsPrefix(start, limit string) bool {
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
