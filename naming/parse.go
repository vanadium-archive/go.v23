package naming

import (
	"strings"
)

// SplitAddressName takes a veyron name and returns the server address and
// the name relative to the server. "//"s are retained.
// The returned address may be in endpoint format or host:port format.
// NewEndpoint should be used to test for the former and if that fails then
// internet host:port form should be used.
func SplitAddressName(name string) (string, string) {
	if !strings.HasPrefix(name, "/") {
		return "", name
	}
	if strings.HasPrefix(name, "//") {
		return "", name
	}
	elems := strings.SplitN(name[1:], "/", 2)
	if len(elems) == 1 {
		return elems[0], ""
	}
	if strings.HasPrefix(elems[1], "/") {
		return elems[0], "/" + elems[1]
	}
	return elems[0], elems[1]
}

// JoinAddressNames takes an address and name and returns a veyron name.
// If a valid address is supplied then the returned name will always be a global
// name (i.e. starting with /), otherwise it may be relative.
// Address should not start with a / or // and if it does, that prefix will
// be stripped.
func JoinAddressName(address, name string) string {
	switch {
	case strings.HasPrefix(address, "//"):
		address = address[2:]
	case strings.HasPrefix(address, "/"):
		address = address[1:]
	}
	if len(address) == 0 {
		return name
	}
	if len(name) == 0 {
		return "/" + address
	}
	if !strings.HasPrefix(name, "/") {
		return "/" + address + "/" + name
	}
	return "/" + address + name
}

// JoinAddressNamesFixed takes an address and name and returns a fixed veyron
// name.  A fixed veyron name is of the form /endpoint//suffix, and is not
// subject to further resolution.
func JoinAddressNameFixed(address, suffix string) string {
	suffix = strings.TrimLeft(suffix, "/")
	if len(suffix) > 0 {
		suffix = "//" + suffix
	}
	return JoinAddressName(address, suffix)
}

// Join takes a veyron name and appends the given suffix to it.
func Join(name, suffix string) string {
	strings.TrimRight(name, "/")
	strings.TrimRight(suffix, "/")
	if len(suffix) == 0 {
		return name
	}
	if len(name) == 0 {
		return suffix
	}
	if strings.HasPrefix(suffix, "/") {
		return name + suffix
	}
	return name + "/" + suffix
}

// Depth returns the directory depth of a given name.
func Depth(name string) int {
	name = strings.Trim(name, "/")
	if name == "" {
		return 0
	}
	return strings.Count(name, "/") - strings.Count(name, "//") + 1
}

// Fixed checks if the given veyron name is a fixed name.  A fixed veyron name
// is of the form /endpoint//suffix, and is not subject to further resolution.
func Fixed(name string) bool {
	if _, suffix := SplitAddressName(name); len(suffix) == 0 || strings.HasPrefix(suffix, "//") {
		return true
	}
	return false
}

// MakeFixed converts the given veyron name to a fixed one.  A fixed veyron name
// is of the form /endpoint//suffix, and is not subject to further resolution.
func MakeFixed(name string) string {
	return JoinAddressNameFixed(SplitAddressName(name))
}

// MakeNotFixed converts the given name to one that's not fixed.
func MakeNotFixed(name string) string {
	address, suffix := SplitAddressName(name)
	return JoinAddressName(address, strings.TrimLeft(suffix, "/"))
}

// StripSuffix removes the given suffix from the given name.  The suffix must
// appear in the name verbatim (including same number of repeated slashes in
// between name components).
func StripSuffix(name, suffix string) string {
	switch {
	case name == suffix:
		return ""
	case strings.HasPrefix(suffix, "/"):
		fallthrough
	case strings.HasSuffix(name, "/"+suffix):
		return strings.TrimRight(strings.TrimSuffix(name, suffix), "/")
	}
	return name
}
