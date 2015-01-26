package access

import (
	"v.io/core/veyron2/verror"
	"v.io/core/veyron2/verror2"
)

// TODO(toddw): Remove this file and specify errors in service.vdl with new error syntax.

const (
	// pkgPath is the prefix of errors in this package.
	pkgPath = "v.io/core/veyron2/services/security/access"

	ErrBadEtag = verror.ID("v.io/core/veyron2/services/security/access.ErrBadEtag")
	ErrTooBig  = verror.ID("v.io/core/veyron2/services/security/access.ErrTooBig")
)

// The etag passed to SetACL is invalid.  Likely, another client set the ACL
// already and invalidated the etag.  Use GetACL to fetch a fresh etag.
var BadEtag = verror2.Register(
	pkgPath+".badEtag",
	verror2.RetryRefetch,
	"{1} {2} invalid etag passed to SetACL {_}",
)

// The ACL is too big.  Use groups to represent large sets of principals.
var TooBig = verror2.Register(
	pkgPath+".tooBig",
	verror2.NoRetry,
	"{1} {2} ACL is too big {_}",
)
