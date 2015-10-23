// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"regexp"
	"strings"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
)

// Escape escapes a component name for use in a Syncbase object name. In
// particular, it replaces bytes "%" and "/" with the "%" character followed by
// the byte's two-digit hex code. Clients using the client library need not
// escape names themselves; the client library does so on their behalf.
func Escape(s string) string {
	return naming.EncodeAsNameElement(s)
}

// Unescape applies the inverse of Escape. It returns false if the given string
// is not a valid escaped string.
func Unescape(s string) (string, bool) {
	return naming.DecodeFromNameElement(s)
}

// Currently we use \xff for perms index storage and \xfe as a component
// separator in storage engine keys. \xfc and \xfd are not used. In the future
// we might use \xfc followed by "c", "d", "e", or "f" as an order-preserving
// 2-byte encoding of our reserved bytes. Note that all valid UTF-8 byte
// sequences are allowed.
var reservedBytes = []string{"\xfc", "\xfd", "\xfe", "\xff"}

func containsReservedByte(s string) bool {
	for _, v := range reservedBytes {
		if strings.Contains(s, v) {
			return true
		}
	}
	return false
}

var identifierRegexp *regexp.Regexp = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_]*$")

func validIdentifier(s string) bool {
	return identifierRegexp.MatchString(s)
}

// ValidAppName returns true iff the given string is a valid app name.
func ValidAppName(s string) bool {
	return s != "" && !containsReservedByte(s)
}

// ValidDatabaseName returns true iff the given string is a valid database name.
func ValidDatabaseName(s string) bool {
	return validIdentifier(s)
}

// ValidTableName returns true iff the given string is a valid table name.
func ValidTableName(s string) bool {
	return validIdentifier(s)
}

// ValidRowKey returns true iff the given string is a valid row key.
func ValidRowKey(s string) bool {
	return s != "" && !containsReservedByte(s)
}

// ParseTableRowPair splits the "<table>/<row>" part of a Syncbase object name
// into the table name and the row key or prefix.
func ParseTableRowPair(ctx *context.T, pattern string) (string, string, error) {
	parts := strings.SplitN(pattern, "/", 2)
	if len(parts) != 2 { // require both table and row parts
		return "", "", verror.New(verror.ErrBadArg, ctx, pattern)
	}
	table, row := parts[0], parts[1]
	if !ValidTableName(table) {
		return "", "", verror.New(wire.ErrInvalidName, ctx, table)
	}
	if row != "" && !ValidRowKey(row) {
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
