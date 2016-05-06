// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"fmt"
	"regexp"
	"strings"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
)

// Encode escapes a component name for use in a Syncbase object name. In
// particular, it replaces bytes "%" and "/" with the "%" character followed by
// the byte's two-digit hex code. Clients using the client library need not
// escape names themselves; the client library does so on their behalf.
func Encode(s string) string {
	return naming.EncodeAsNameElement(s)
}

// Decode is the inverse of Encode.
func Decode(s string) (string, error) {
	res, ok := naming.DecodeFromNameElement(s)
	if !ok {
		return "", fmt.Errorf("failed to decode component name: %s", s)
	}
	return res, nil
}

// EncodeId encodes the given Id for use in a Syncbase object name.
func EncodeId(id wire.Id) string {
	// Note that "," is not allowed to appear in blessing patterns. We also
	// could've used "/" as a separator, but then we would've had to be more
	// careful with decoding and splitting name components elsewhere.
	// TODO(sadovsky): Maybe define "," constant in v23/services/syncbase.
	return Encode(id.Blessing + "," + id.Name)
}

// DecodeId is the inverse of EncodeId.
func DecodeId(s string) (wire.Id, error) {
	dec, err := Decode(s)
	if err != nil {
		return wire.Id{}, err
	}
	parts := strings.SplitN(dec, ",", 2)
	if len(parts) != 2 {
		return wire.Id{}, fmt.Errorf("failed to decode id: %s", s)
	}
	return wire.Id{Blessing: parts[0], Name: parts[1]}, nil
}

// Currently we use \xff for perms index storage and \xfe as a component
// separator in storage engine keys. \xfc and \xfd are not used. In the future
// we might use \xfc followed by "c", "d", "e", or "f" as an order-preserving
// 2-byte encoding of our reserved bytes. Note that all valid UTF-8 byte
// sequences are allowed.
var reservedBytes = []string{"\xfc", "\xfd", "\xfe", "\xff"}

func containsAnyOf(s string, needles []string) bool {
	for _, v := range needles {
		if strings.Contains(s, v) {
			return true
		}
	}
	return false
}

// TODO(ivanpi): Consider relaxing this.
var idNameRegexp *regexp.Regexp = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_]*$")

const (
	// maxBlessingLen is the max allowed number of bytes in an Id blessing.
	// TODO(ivanpi): Blessings are theoretically unbounded in length.
	maxBlessingLen = 1024
	// maxNameLen is the max allowed number of bytes in an Id name.
	maxNameLen = 64
)

// ValidateId returns nil iff the given Id is a valid database, collection, or
// syncgroup Id.
func ValidateId(id wire.Id) error {
	if x := len([]byte(id.Blessing)); x == 0 {
		return fmt.Errorf("Id blessing cannot be empty")
	} else if x > maxBlessingLen {
		return fmt.Errorf("Id blessing %q exceeds %d bytes", id.Blessing, maxBlessingLen)
	}
	if x := len([]byte(id.Name)); x == 0 {
		return fmt.Errorf("Id name cannot be empty")
	} else if x > maxNameLen {
		return fmt.Errorf("Id name %q exceeds %d bytes", id.Name, maxNameLen)
	}
	if !security.BlessingPattern(id.Blessing).IsValid() {
		return fmt.Errorf("Id blessing %q is not a valid blessing pattern", id.Blessing)
	}
	if containsAnyOf(id.Blessing, reservedBytes) {
		return fmt.Errorf("Id blessing %q contains a reserved byte (one of %q)", id.Blessing, reservedBytes)
	}
	if !idNameRegexp.MatchString(id.Name) {
		return fmt.Errorf("Id name %q does not satisfy regex %q", id.Name, idNameRegexp.String())
	}
	return nil
}

// ValidateRowKey returns nil iff the given string is a valid row key.
func ValidateRowKey(s string) error {
	if s == "" {
		return fmt.Errorf("row key cannot be empty")
	}
	if containsAnyOf(s, reservedBytes) {
		return fmt.Errorf("row key %q contains a reserved byte (one of %q)", s, reservedBytes)
	}
	return nil
}

// ParseCollectionRowPair splits the "<collectionId>/<row>" part of a Syncbase
// object name into the collection id and the row key or prefix.
func ParseCollectionRowPair(ctx *context.T, pattern string) (wire.Id, string, error) {
	parts := strings.SplitN(pattern, "/", 2)
	if len(parts) != 2 { // require both collection and row parts
		return wire.Id{}, "", verror.New(verror.ErrBadArg, ctx, pattern)
	}
	collectionEnc, row := parts[0], parts[1]
	collection, err := DecodeId(parts[0])
	if err == nil {
		err = ValidateId(collection)
	}
	if err != nil {
		return wire.Id{}, "", verror.New(wire.ErrInvalidName, ctx, collectionEnc, err)
	}
	if row != "" {
		if err := ValidateRowKey(row); err != nil {
			return wire.Id{}, "", verror.New(wire.ErrInvalidName, ctx, row, err)
		}
	}
	return collection, row, nil
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

// AppBlessingFromContext returns an app blessing pattern from the given
// context.
// TODO(sadovsky,ashankar): Implement.
func AppBlessingFromContext(ctx *context.T) (string, error) {
	// NOTE(sadovsky): For now, we use a blessing string that will be easy to
	// find-replace when we actually implement this method.
	return "v.io:a:xyz", nil
}

// UserBlessingFromContext returns a user blessing pattern from the given
// context.
// TODO(sadovsky,ashankar): Implement.
func UserBlessingFromContext(ctx *context.T) (string, error) {
	// NOTE(sadovsky): For now, we use a blessing string that will be easy to
	// find-replace when we actually implement this method.
	return "v.io:u:sam", nil
}
