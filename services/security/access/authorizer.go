package access

import (
	"fmt"
	"os"
	"reflect"

	"veyron.io/veyron/veyron2/security"
	"veyron.io/veyron/veyron2/vlog"
)

// TaggedACLAuthorizer implements an authorization policy where access is
// granted if the remote end presents blessings included in the Access Control
// Lists (ACLs) associated with the set of relevant tags.
//
// The set of relevant tags is the subset of tags associated with the
// method (security.Context.MethodTags) that have the same type as tagType.
// Currently, tagType.Kind must be reflect.String, i.e., only tags that are
// named string types are supported.
//
// If multiple tags of tagType are associated with the method, then access is
// granted if the peer presents blessings that match the ACLs of each one of
// those tags. If no tags of tagType are associated with the method, then
// access is denied.
//
// If the TaggedACLMap provided is nil, then a nil authorizer is returned.
//
// Sample usage:
//
// (1) Attach tags to methods in the VDL (eg. myservice.vdl)
//   package myservice
//
//   type MyTag string
//   const (
//     ReadAccess  = MyTag("R")
//     WriteAccess = MyTag("W")
//   )
//
//   type MyService interface {
//     Get() ([]string, error)       {ReadAccess}
//     GetIndex(int) (string, error) {ReadAccess}
//
//     Set([]string) error           {WriteAccess}
//     SetIndex(int, string) error   {WriteAccess}
//
//     GetAndSet([]string) ([]string, error) {ReadAccess, WriteAccess}
//   }
//
// (2) Setup the ipc.Dispatcher to use the TaggedACLAuthorizer
//   import (
//     "reflect"
//     "veyron.io/veyron/veyron/security/acl"
//
//     "veyron.io/veyron/veyron2/ipc"
//     "veyron.io/veyron/veyron2/security"
//   )
//
//   type dispatcher struct{}
//   func (d dispatcher) Lookup(suffix, method) (ipc.Invoker, security.Authorizer, error) {
//      acl := acl.TaggedACLMap{
//        "R": acl.ACL{In: []security.BlessingPattern{"alice/friends/...", "alice/family/..."} },
//        "W": acl.ACL{In: []security.BlessingPattern{"alice/family/...", "alice/colleagues/..." } },
//      }
//      typ := reflect.TypeOf(ReadAccess)  // equivalently, reflect.TypeOf(WriteAccess)
//      return newInvoker(), acl.TaggedACLAuthorizer(acl, typ), nil
//   }
//
// With the above dispatcher, the server will grant access to a peer with the blessing
// "alice/friend/bob" access only to the "Get" and "GetIndex" methods. A peer presenting
// the blessing "alice/colleague/carol" will get access only to the "Set" and "SetIndex"
// methods. A peer presenting "alice/family/mom" will get access to all methods, even
// GetAndSet - which requires that the blessing appear in the ACLs for both the
// ReadAccess and WriteAccess tags.
func TaggedACLAuthorizer(acls TaggedACLMap, tagType reflect.Type) (security.Authorizer, error) {
	if tagType.Kind() != reflect.String {
		return nil, fmt.Errorf("tag type(%v) must be backed by a string not %v", tagType, tagType.Kind())
	}
	return &authorizer{acls, tagType}, nil
}

// TaggedACLAuthorizerFromFile applies the same authorization policy as
// TaggedACLAuthorizer, with the TaggedACLMap to be used sourced from a file named
// filename.
//
// Changes to the file are monitored and affect subsequent calls to Authorize.
// Currently, this is achieved by re-reading the file on every call to
// Authorize.
// TODO(ashankar,ataly): Use inotify or a similar mechanism to watch for
// changes.
func TaggedACLAuthorizerFromFile(filename string, tagType reflect.Type) (security.Authorizer, error) {
	if tagType.Kind() != reflect.String {
		return nil, fmt.Errorf("tag type(%v) must be backed by a string not %v", tagType, tagType.Kind())
	}
	// TODO(ashankar): Remove this.
	// If the file was written in the old ACL format, convert to new.
	if f, err := os.Open(filename); err == nil {
		m, oldformat, err := readTaggedACLMapSupportingOldFormat(f)
		if oldformat && err != nil {
			// Rewrite the file in the new format.
			f.Close()
			if f, err := os.OpenFile(filename, os.O_WRONLY, 0600); err != nil {
				err = m.WriteTo(f)
				vlog.Infof("ACL in %q is in the old format. Rewriting to new format: %v", filename, err)
				f.Close()
			}
		} else {
			f.Close()
		}
	}
	return &fileAuthorizer{filename, tagType}, nil
}

type authorizer struct {
	acls    TaggedACLMap
	tagType reflect.Type
}

func (a *authorizer) Authorize(ctx security.Context) error {
	// TODO(ashankar,toddw,jsimsa): Hack for the Signature RPC.
	//
	// The vdl-compiler generates a "Signature" method, but that method
	// does not have any access tags associated with it (and since the
	// space of access tags is customizable by the application developer,
	// there is no way for the vdl-compiler to know the right tag to put).
	// As a result, the Signature method ends up with no ACL and thus
	// all blessings are disallowed.
	//
	// As of December 2014, Todd was working on a different scheme for these
	// generated methods, making them "reserved" methods. Once that mechanism
	// is in place, remove this hack and all reserved methods will use an
	// access policy appropriate for them and distinct from the access policy
	// for application methods.
	//
	// In the mean time, we risk leaking the interface description to all
	// principals in the world.
	if ctx.Method() == "Signature" {
		return nil
	}
	// "Self-RPCs" are always authorized.
	if l, r := ctx.LocalBlessings(), ctx.RemoteBlessings(); l != nil && r != nil && reflect.DeepEqual(l.PublicKey(), r.PublicKey()) {
		return nil
	}

	var blessingsForContext []string
	blessings := ctx.RemoteBlessings()
	if blessings != nil {
		blessingsForContext = blessings.ForContext(ctx)
	}
	grant := false
	for _, tag := range ctx.MethodTags() {
		if v := reflect.ValueOf(tag); v.Type() == a.tagType {
			if acl, exists := a.acls[v.String()]; !exists || !acl.Includes(blessingsForContext...) {
				return errACLMatch(blessings, blessingsForContext)
			}
			grant = true
		}
	}
	if grant {
		return nil
	}
	return errACLMatch(blessings, blessingsForContext)
}

type fileAuthorizer struct {
	filename string
	tagType  reflect.Type
}

func (a *fileAuthorizer) Authorize(ctx security.Context) error {
	acl, err := loadTaggedACLMapFromFile(a.filename)
	if err != nil {
		// TODO(ashankar): Information leak?
		return fmt.Errorf("failed to read ACL from file: %v", err)
	}
	return (&authorizer{acl, a.tagType}).Authorize(ctx)
}

func loadTaggedACLMapFromFile(filename string) (TaggedACLMap, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return ReadTaggedACLMap(file)
}

func errACLMatch(blessings security.Blessings, blessingsForContext []string) error {
	return fmt.Errorf("all valid blessings for this request: %v (out of %v) are disallowed by the ACL", blessingsForContext, blessings)
}
