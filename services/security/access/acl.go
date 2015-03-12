package access

import (
	"encoding/json"
	"io"
	"sort"

	"v.io/v23/security"
)

// Includes returns true iff the AccessList grants access to a principal that presents
// blessings (i.e., if at least one of the blessings matches the AccessList).
func (acl AccessList) Includes(blessings ...string) bool {
	blessings = acl.pruneBlacklisted(blessings)
	for _, pattern := range acl.In {
		if pattern.MatchedBy(blessings...) {
			return true
		}
	}
	return false
}

func (acl AccessList) pruneBlacklisted(blessings []string) []string {
	if len(acl.NotIn) == 0 {
		return blessings
	}
	var filtered []string
	for _, b := range blessings {
		blacklisted := false
		for _, bp := range acl.NotIn {
			if security.BlessingPattern(bp).MatchedBy(b) {
				blacklisted = true
				break
			}
		}
		if !blacklisted {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

// Authorize implements security.Authorizer where the request is authorized
// only if the remote blessings are included in the AccessList.
//
// TODO(ashankar): Add tests for this
func (acl *AccessList) Authorize(call security.Call) error {
	blessings := call.RemoteBlessings()
	blessingsForCall, invalid := blessings.ForCall(call)
	if acl.Includes(blessingsForCall...) {
		return nil
	}
	return NewErrAccessListMatch(nil, blessingsForCall, invalid)
}

// WriteTo writes the JSON-encoded representation of a Permissions to w.
func (m Permissions) WriteTo(w io.Writer) error {
	return json.NewEncoder(w).Encode(m.Normalize())
}

// ReadPermissions reads the JSON-encoded representation of a Permissions from r.
func ReadPermissions(r io.Reader) (m Permissions, err error) {
	err = json.NewDecoder(r).Decode(&m)
	return
}

// Add updates m to so that blessings matching pattern will be included in the
// access control lists for the provided tag (by adding to the "In" list).
//
// TODO(ashankar): Add tests for Add, Blacklist and Clear.
func (m Permissions) Add(pattern security.BlessingPattern, tag string) {
	list := m[tag]
	list.In = append(list.In, pattern)
	sort.Sort(byPattern(list.In))
	// TODO(ashankar): Remove duplicates
	m[tag] = list
}

// Blacklist updates m so that the provided blessing will be excluded from
// the access control list for the provided tag (via m[tag].NotIn).
func (m Permissions) Blacklist(blessing string, tag string) {
	list := m[tag]
	list.NotIn = append(list.NotIn, blessing)
	sort.Strings(list.NotIn)
	// TODO(ashankar): Remove duplicates
	m[tag] = list
}

// Clear removes all references to blessingOrPattern from all the provided
// tags in the AccessList, or all tags if len(tags) = 0.
func (m Permissions) Clear(blessingOrPattern string, tags ...string) {
	if len(tags) == 0 {
		tags = make([]string, 0, len(m))
		for t, _ := range m {
			tags = append(tags, t)
		}
	}
	for _, t := range tags {
		oldList := m[t]
		var newList AccessList
		for _, p := range oldList.In {
			if string(p) != blessingOrPattern {
				newList.In = append(newList.In, p)
			}
		}
		for _, b := range oldList.NotIn {
			if b != blessingOrPattern {
				newList.NotIn = append(newList.NotIn, b)
			}
		}
		m[t] = newList
	}
}

// Copy returns a new Permissions that is a copy of m.
// TODO(ashankar): Unittest for this.
func (m Permissions) Copy() Permissions {
	ret := make(Permissions)
	for tag, list := range m {
		newlist := AccessList{
			In:    make([]security.BlessingPattern, len(list.In)),
			NotIn: make([]string, len(list.NotIn)),
		}
		for idx, item := range list.In {
			newlist.In[idx] = item
		}
		for idx, item := range list.NotIn {
			newlist.NotIn[idx] = item
		}
		ret[tag] = newlist
	}
	return ret
}

// Normalize re-organizes 'm' so that two equivalent Permissionss are
// comparable via reflection. It returns 'm'.
//
// TODO(ashankar): Add a unittest for this.
func (m Permissions) Normalize() Permissions {
	for tag, list := range m {
		sort.Sort(byPattern(list.In))
		sort.Strings(list.NotIn)
		// TODO(ashankar): Remove duplicate entries.
		if len(list.In) == 0 && list.In != nil {
			list.In = nil
			m[tag] = list
		}
		if len(list.NotIn) == 0 && list.NotIn != nil {
			list.NotIn = nil
			m[tag] = list
		}
	}
	return m
}

type byPattern []security.BlessingPattern

func (a byPattern) Len() int           { return len(a) }
func (a byPattern) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byPattern) Less(i, j int) bool { return a[i] < a[j] }
