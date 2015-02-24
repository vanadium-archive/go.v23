package access

import "v.io/v23/vdl"

// TypicalTagType returns the type of the pre-defined tags in this access
// package.
//
// Typical use of this is to setup an ACL authorizer that uses these pre-defined
// tags:
//   authorizer := TaggedACLAuthorizer(myacl, TypicalTagType())
func TypicalTagType() *vdl.Type {
	return vdl.TypeOf(Tag(""))
}

// AllTypicalTags returns all access.Tag values defined in this package.
func AllTypicalTags() []Tag {
	return []Tag{Admin, Read, Write, Debug, Resolve}
}
