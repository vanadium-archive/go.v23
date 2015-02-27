package syncbase

// Note, some of the code below is copied from
// v.io/syncbase/veyron/services/syncbase/server/key_util.go.

import (
	"regexp"

	"v.io/v23/vdl"
)

var nameRegexp *regexp.Regexp = regexp.MustCompile("^[a-zA-Z0-9_.-]+$")

func validName(s string) bool {
	return nameRegexp.MatchString(s)
}

// TODO(sadovsky): For the initial prototype we assume that primary keys are
// always strings. We'll need to revisit this assumption.

func encodeKey(key *vdl.Value) string {
	return key.RawString()
}

func decodeKey(s string) *vdl.Value {
	return vdl.StringValue(s)
}
