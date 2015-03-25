// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package naming

import (
	"strings"
)

// SplitAddressName takes an object name and returns the server address and
// the name relative to the server.
// The name parameter may be a rooted name or a relative name; an empty string
// address is returned for the latter case.
// The returned address may be in endpoint format or host:port format.
func SplitAddressName(name string) (string, string) {
	name = Clean(name)
	if !Rooted(name) {
		return "", name
	}
	name = name[1:] // trim the beginning "/"
	if len(name) == 0 {
		return "", ""
	}
	// Could have used regular expressions, but that makes this function
	// 10x slower as per the benchmark.
	if strings.HasPrefix(name, "@") { // <endpoint>/<suffix>
		addr, suffix := splitIntoTwo(name, "@@/")
		if len(suffix) > 0 { // The trailing "@@" was stripped, restore it
			addr = addr + "@@"
		}
		return addr, suffix
	}
	if strings.HasPrefix(name, "(") { // (blessing)@host:[port]/suffix
		_, tmp := splitIntoTwo(name, ")@")
		_, suffix := splitIntoTwo(tmp, "/")
		return strings.TrimSuffix(name, "/"+suffix), suffix
	}
	// host:[port]/suffix
	return splitIntoTwo(name, "/")
}

// JoinAddressName takes an address and a relative name and returns a rooted
// or relative name. If a valid address is supplied then the returned name
// will always be a rooted name (i.e. starting with /), otherwise it may
// be relative. Address should not start with a / and if it does,
// that prefix will be stripped.
func JoinAddressName(address, name string) string {
	address = strings.TrimLeft(address, "/")
	if len(address) == 0 {
		return Clean(name)
	}
	if len(name) == 0 {
		return Clean("/" + address)
	}
	return Clean("/" + address + "/" + name)
}

// Rooted returns true for any name that is considered to be rooted.
// A rooted name is one that starts with a single / followed by
// a non /. / on its own is considered rooted.
func Rooted(name string) bool {
	return strings.HasPrefix(name, "/")
}

// Join takes a variable number of name fragments and concatenates them
// together using '/'.  The returned name is cleaned of multiple adjacent
// '/'s.
func Join(elems ...string) string {
	for len(elems) > 0 && elems[0] == "" {
		elems = elems[1:]
	}
	return Clean(strings.Join(elems, "/"))
}

// TrimSuffix removes the suffix (and any connecting '/') from
// the name.
func TrimSuffix(name, suffix string) string {
	name = Clean(name)
	suffix = Clean(suffix)

	// Easy cases first.
	if name == suffix {
		return ""
	}
	if len(suffix) >= len(name) {
		return name
	}
	// A suffix starting with a slash cannot be a partial match.
	if strings.HasPrefix(suffix, "/") {
		return name
	}
	// At this point suffix is guaranteed not to start with a '/' and
	// suffix is shorter than name.
	if strings.HasSuffix(name, suffix) {
		prefix := strings.TrimSuffix(name, suffix)
		if strings.HasSuffix(prefix, "/") {
			if len(prefix) == 1 {
				return name
			}
			return strings.TrimSuffix(prefix, "/")
		}
	}
	return name
}

// IsReserved returns true if name is a reserved name.
func IsReserved(name string) bool {
	return strings.HasPrefix(name, ReservedNamePrefix)
}

// StripReserved returns the name stripped of the reserved prefix.
func StripReserved(name string) string {
	if IsReserved(name) {
		return name[len(ReservedNamePrefix):]
	}
	return name
}

// Clean reduces multiple adjacent slashes to a single slash.
// It also removes any trailing slash.
func Clean(name string) string {
	// Eradicate duplicate slashes and trailing slashes.  We
	// could use path.Clean but it has other side effects.
	for strings.Contains(name, "//") {
		name = strings.Replace(name, "//", "/", -1)
	}
	if name == "/" {
		return name
	}
	return strings.TrimSuffix(name, "/")
}

func splitIntoTwo(str, separator string) (string, string) {
	elems := strings.SplitN(str, separator, 2)
	if len(elems) == 1 {
		return elems[0], ""
	}
	return elems[0], elems[1]
}
