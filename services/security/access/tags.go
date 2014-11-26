package access

import "reflect"

// TypicalTagType returns the type of the pre-defined tags in this access
// package.
//
// Typical use of this is to setup an ACL authorizer that uses these pre-defined
// tags:
//   authorizer := TaggedACLAuthorizer(myacl, TypicalTagType())
func TypicalTagType() reflect.Type {
	var t Tag
	return reflect.TypeOf(t)
}

// AllTypicalTags returns all access.Tag values defined in this package.
func AllTypicalTags() []Tag {
	return []Tag{Admin, Read, Write, Debug, Resolve}
}
