package naming

import (
	"strings"

	"veyron.io/veyron/veyron2/vlog"
)

// TODO(cnicolaou): consider renaming {Split,Join}AddressName as
// {Split,Join}EndpointName. For Join, it's not clear that host:port is
// really an Endpoint. Something to consider.

// SplitAddressName takes an object name and returns the server address and
// the name relative to the server.
// The name parameter may be a rooted name or a relative name; an empty string
// address is returned for the latter case.
// The returned address may be in endpoint format or host:port format.
func SplitAddressName(name string) (string, string) {
	if !Rooted(name) {
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

// JoinAddressName takes an address and a relative name and returns a rooted
// or relative name. If a valid address is supplied then the returned name
// will always be a rooted name (i.e. starting with /), otherwise it may
// be relative. Address should not start with a / and if it does,
// that prefix will be stripped.
func JoinAddressName(address, name string) string {
	address = strings.TrimLeft(address, "/")
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

// Terminal is obsolete.
// TODO(p): Get rid of after all //s have disappeared.
func Terminal(name string) bool {
	vlog.Infof("naming.Terminal is obsolete dammit!")
	_, n := SplitAddressName(name)
	return len(n) == 0 || strings.HasPrefix(n, "//")
}

// MakeTerminal is obsolete.
// TODO(p): Leave as a NOOP till we're sure noone is using it.
func MakeTerminal(name string) string {
	vlog.Infof("naming.MakeTerminal is obsolete dammit!")
	return name
}

func ToStringSlice(e *MountEntry) []string {
	var names []string
	for _, s := range e.Servers {
		names = append(names, JoinAddressName(s.Server, e.Name))
	}
	return names
}

// Rooted returns true for any name that is considered to be rooted.
// A rooted name is one that starts with a single / followed by
// a non /. / on its own is considered rooted.
// TODO(p): Lose the // checks when I know all //'s are gone.
func Rooted(name string) bool {
	if strings.HasPrefix(name, "//") {
		return false
	}
	return strings.HasPrefix(name, "/")
}

// MakeResolvable returns a version of its argument that is resolvable,
// TODO(p): Take this out once people stop tacking //'s everywhere.
func MakeResolvable(name string) string {
	vlog.Infof("naming.MakeResolvable is obsolete dammit!")
	return name
}

// Join takes a variable number of name fragments and concatenates them
// together using '/'. It takes care to ensure that it does not create a '//'
// pair where one didn't exist before. Runs of two or more '/'s are truncated
// to '//' when they are in the middle of the joined string.
// TODO(p): Lose the // checks when I know all //'s are gone.
func Join(elems ...string) string {
	if len(elems) == 0 {
		return ""
	}
	var n int
	for _, e := range elems {
		n += len(e)
	}
	n += len(elems) - 1
	b := make([]byte, n)
	var bp int
	var delimiter string
	for i, e := range elems {
		if i > 0 {
			if strings.HasPrefix(e, "//") {
				delimiter = "//"
			}
			e = strings.TrimLeft(e, "/")
		}
		if len(e) > 0 {
			bp += copy(b[bp:], delimiter)
			delimiter = "/"
		}
		if i < len(elems)-1 {
			if strings.HasSuffix(e, "//") {
				delimiter = "//"
			}
			e = strings.TrimRight(e, "/")
		}
		bp += copy(b[bp:], e)
	}
	if delimiter == "//" {
		bp += copy(b[bp:], delimiter)
	}
	// We made b too big because we didn't know how many '/' would be stripped.
	// As a result, we have to reslice to just the portion we used.
	return string(b[:bp])
}

// TrimSuffix removes the suffix (and any connecting '/') from
// the name.  Examples are:
//  a/b/c - b/c = a
//  a//b - b = a//
//  a//b - /b = a//b (not a suffix)
//  /a - a = a (not a suffix)
//  /a//b - //b = /a
//  /a// - // = /a
// TODO(p): Lose the // checks when I know all //'s are gone.
func TrimSuffix(name, suffix string) string {
	// Easy cases first.
	if name == suffix {
		return ""
	}
	if len(suffix) >= len(name) {
		return name
	}
	// Double slash is easy, can only be a strict suffix match.
	if strings.HasPrefix(suffix, "//") {
		return strings.TrimSuffix(name, suffix)
	}
	// A suffix starting with a slash cannot be a partial match.
	if strings.HasPrefix(suffix, "/") {
		return name
	}
	// At this point suffix is guaranteed not to start with a '/' and
	// suffix is shorter than name.
	if strings.HasSuffix(name, suffix) {
		prefix := strings.TrimSuffix(name, suffix)
		if strings.HasSuffix(prefix, "//") {
			return prefix
		}
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
