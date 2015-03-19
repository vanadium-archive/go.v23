package access

import (
	"os"
	"reflect"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/vdl"
	"v.io/v23/verror"
)

const pkgPath = "v.io/v23/services/security/access"

var (
	errTagNeedsString             = verror.Register(pkgPath+".errTagNeedsString", verror.NoRetry, "{1:}{2:}tag type({3}) must be backed by a string not {4}{:_}")
	errNoMethodTags               = verror.Register(pkgPath+".errNoMethodTags", verror.NoRetry, "{1:}{2:}PermissionsAuthorizer.Authorize called with an object ({3}, method {4}) that has no method tags; this is likely unintentional{:_}")
	errCantReadAccessListFromFile = verror.Register(pkgPath+".errCantReadAccessListFromFile", verror.NoRetry, "{1:}{2:}failed to read AccessList from file{:_}")
)

// PermissionsAuthorizer implements an authorization policy where access is
// granted if the remote end presents blessings included in the Access Control
// Lists (AccessLists) associated with the set of relevant tags.
//
// The set of relevant tags is the subset of tags associated with the
// method (security.Call.MethodTags) that have the same type as tagType.
// Currently, tagType.Kind must be reflect.String, i.e., only tags that are
// named string types are supported.
//
// If multiple tags of tagType are associated with the method, then access is
// granted if the peer presents blessings that match the AccessLists of each one of
// those tags. If no tags of tagType are associated with the method, then
// access is denied.
//
// If the Permissions provided is nil, then a nil authorizer is returned.
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
// (2) Setup the rpc.Dispatcher to use the PermissionsAuthorizer
//   import (
//     "reflect"
//     "v.io/x/ref/security/acl"
//
//     "v.io/v23/rpc"
//     "v.io/v23/security"
//   )
//
//   type dispatcher struct{}
//   func (d dispatcher) Lookup(suffix, method) (rpc.Invoker, security.Authorizer, error) {
//      acl := acl.Permissions{
//        "R": acl.AccessList{In: []security.BlessingPattern{"alice/friends", "alice/family"} },
//        "W": acl.AccessList{In: []security.BlessingPattern{"alice/family", "alice/colleagues" } },
//      }
//      typ := reflect.TypeOf(ReadAccess)  // equivalently, reflect.TypeOf(WriteAccess)
//      return newInvoker(), acl.PermissionsAuthorizer(acl, typ), nil
//   }
//
// With the above dispatcher, the server will grant access to a peer with the blessing
// "alice/friend/bob" access only to the "Get" and "GetIndex" methods. A peer presenting
// the blessing "alice/colleague/carol" will get access only to the "Set" and "SetIndex"
// methods. A peer presenting "alice/family/mom" will get access to all methods, even
// GetAndSet - which requires that the blessing appear in the AccessLists for both the
// ReadAccess and WriteAccess tags.
func PermissionsAuthorizer(acls Permissions, tagType *vdl.Type) (security.Authorizer, error) {
	if tagType.Kind() != vdl.String {
		return nil, errTagType(tagType)
	}
	return &authorizer{acls, tagType}, nil
}

// PermissionsAuthorizerFromFile applies the same authorization policy as
// PermissionsAuthorizer, with the Permissions to be used sourced from a file named
// filename.
//
// Changes to the file are monitored and affect subsequent calls to Authorize.
// Currently, this is achieved by re-reading the file on every call to
// Authorize.
// TODO(ashankar,ataly): Use inotify or a similar mechanism to watch for
// changes.
func PermissionsAuthorizerFromFile(filename string, tagType *vdl.Type) (security.Authorizer, error) {
	if tagType.Kind() != vdl.String {
		return nil, errTagType(tagType)
	}
	return &fileAuthorizer{filename, tagType}, nil
}

func errTagType(tt *vdl.Type) error {
	return verror.New(errTagNeedsString, nil, verror.New(errTagNeedsString, nil, tt, tt.Kind()))
}

type authorizer struct {
	acls    Permissions
	tagType *vdl.Type
}

func (a *authorizer) Authorize(ctx *context.T) error {
	call := security.GetCall(ctx)
	// "Self-RPCs" are always authorized.
	if l, r := call.LocalBlessings().PublicKey(), call.RemoteBlessings().PublicKey(); l != nil && r != nil && reflect.DeepEqual(l, r) {
		return nil
	}

	blessings, invalid := security.RemoteBlessingNames(ctx)
	grant := false
	if len(call.MethodTags()) == 0 {
		// The following error message leaks the fact that the server is likely
		// misconfigured, but that doesn't seem like a big deal.
		return verror.New(errNoMethodTags, call.Context(), call.Suffix(), call.Method())
	}
	for _, tag := range call.MethodTags() {
		if tag.Type() == a.tagType {
			if acl, exists := a.acls[tag.RawString()]; !exists || !acl.Includes(blessings...) {
				return NewErrAccessListMatch(nil, blessings, invalid)
			}
			grant = true
		}
	}
	if grant {
		return nil
	}
	return NewErrAccessListMatch(nil, blessings, invalid)
}

type fileAuthorizer struct {
	filename string
	tagType  *vdl.Type
}

func (a *fileAuthorizer) Authorize(ctx *context.T) error {
	acl, err := loadPermissionsFromFile(a.filename)
	if err != nil {
		// TODO(ashankar): Information leak?
		return verror.New(errCantReadAccessListFromFile, ctx, err)
	}
	return (&authorizer{acl, a.tagType}).Authorize(ctx)
}

func loadPermissionsFromFile(filename string) (Permissions, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return ReadPermissions(file)
}
