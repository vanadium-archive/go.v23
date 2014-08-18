package naming

import (
	"strings"
)

// TODO(cnicolaou): consider renaming {Split,Join}AddressName as
// {Split,Join}EndpointName. For Join, it's not clear that host:port is
// really an Endpoint. Something to consider.

// SplitAddressName takes an object name and returns the server address and
// the name relative to the server. Terminal names (i.e. those starting
// with "//"s) are maintained.
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
// be relative. Address should not start with a / or // and if it does,
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

// Terminal returns true if its argument is considered to be a terminal name.
// Terminal names have three forms:
// 1. A rooted name with a relative component starting with //.
// 2. A relative name starting with //.
// 3. A rooted name with an empty relative name.
// A name containing // in other location is not considered terminal.
func Terminal(name string) bool {
	_, n := SplitAddressName(name)
	return len(n) == 0 || strings.HasPrefix(n, "//")
}

// MakeTerminal returns a version of its parameter that's guaranteed to
// return true when passed as an argument to the Terminal function above.
func MakeTerminal(name string) string {
	return MakeTerminalAtIndex(name, 0)
}

// TODO(cnicolaou): if this function doesn't end up being used then make it
// private.

// MakeTerminalAtIndex returns a version of its argument that ensures that
// the portion of the name, starting at the / specified by index, would
// be considered terminal if it were the only portion of the relative name.
//
// For rooted names, the index starts at the relative name component
// following the address. For relative names the index is is from the start
// of the relative name. Thus an index of 0 can be used to ensure that a
// name is terminal as per the Terminal function above.
//
// A negative index starts counting from the end of the name. If an index
// runs off of the end of the name (in either direction) it is treated
// as referring to the end of that name.
//
// Consider the relative name a/b/c:
// MakeTerminalIndexAt("a/b/c",1) -> "a//b/c"
// MakeTerminalIndexAt("a/b/c",-1) -> "a/b//c"
//
// Trailing /'s are counted as if they were separating /, so that:
// MakeTerminalIndexAt("a/b/c/",3) -> "a/b/c//"
//
// Runs of two or more / are truncated to // for the matching index,
// but are otherwise unaffected:
// MakeTerminalIndexAt("a////b/c///",3) -> "a////b/c//"
// MakeTerminalIndexAt("a/b///c",1) -> "a//b///c"
//
func MakeTerminalAtIndex(name string, index int) string {
	if name == "" {
		return ""
	}
	addr, rel := SplitAddressName(name)
	// "" is considered terminal, so we just return and the index
	// doesn't matter.
	if rel == "" {
		return JoinAddressName(addr, "")
	}

	// Split rel on runs of one or more /, count the number of slashes
	// in the string, assuming that the prefix to the string is counted
	// as a slash (i.e. index 0 means prepend)
	fields := strings.FieldsFunc(rel, func(r rune) bool { return r == '/' })
	nSlashes := len(fields)
	if len(fields) > 0 && strings.HasSuffix(rel, "/") {
		nSlashes++
	}
	pos := index
	switch {
	case index < 0:
		pos = nSlashes + index
	case index >= nSlashes:
		return JoinAddressName(addr, strings.TrimRight(rel, "/")+"//")
	}
	if pos <= 0 {
		return JoinAddressName(addr, "//"+strings.TrimLeft(rel, "/"))
	}
	// replace the pos'th run of /'s' with //, leaving all else
	// alone.
	inarun := false
	intherun := false
	runindex := 1
	if strings.HasPrefix(rel, "//") {
		runindex = 0
	}
	result := []rune{}
	for _, c := range rel {
		if c == '/' {
			if !inarun {
				inarun = true
				if runindex == pos {
					intherun = true
					result = append(result, '/', '/')
				}
				runindex++
			}
		} else {
			inarun, intherun = false, false
		}
		if !intherun {
			result = append(result, c)
		}
	}
	return JoinAddressName(addr, string(result))
}

// Rooted returns true for any name that is considered to be rooted.
// A rooted name is one that starts with a single / followed by
// a non /. / on its own is considered rooted.
func Rooted(name string) bool {
	if !strings.HasPrefix(name, "/") {
		return false
	}
	if strings.HasPrefix(name, "//") {
		return false
	}
	return true
}

// MakeResolvable returns a version of its argument that is resolvable,
// that is, will cause Terminal to return false, with the following
// exceptions:
// - the name passed in is "", "/" since it returns the same value back
// - the name passed in is "//", which is returned as "" which is terminal.
//
// Rooted names have the first run of one or more / after the address
// reduced to a single /
// Unrooted, relative names have either all leading / removed if present,
// or, not present, the first run of one or more / reduced to a single /
func MakeResolvable(name string) string {
	switch name {
	case "", "/":
		// There's not much we can do with "" and /
		return name
	}
	if Rooted(name) {
		a, n := SplitAddressName(name)
		return JoinAddressName(a, squashSlashRun(n))
	}
	// Consume all leading /
	n := strings.TrimLeft(name, "/")
	if len(n) < len(name) {
		// We removed all leading / for a non-rooted name, so it must
		// now be resolvable
		return n
	}
	return squashSlashRun(n)
}

// Squash the first run of one or more / in s to a single /
func squashSlashRun(s string) string {
	r := []rune{}
	seen := false
	for i, c := range s {
		switch {
		case c != '/':
			if seen {
				r = append(r, []rune(s[i:])...)
				return string(r)
			}
			seen = false
			r = append(r, c)
		default:
			if !seen {
				r = append(r, '/')
			}
			seen = true
		}
	}
	return string(r)
}

// Join takes a variable number of name fragments and concatenates them
// together using '/'. It takes care to ensure that it does not create a '//'
// pair where one didn't exist before. Runs of two or more '/'s are truncated
// to '//' when they are in the middle of the joined string.
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
