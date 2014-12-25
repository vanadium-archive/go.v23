package access

import (
	verror "v.io/veyron/veyron2/verror2"
)

// pkgPath is the prefix of errors in this package.
const pkgPath = "v.io/veyron/veyron2/services/security/access"

// The etag passed to SetACL is invalid.  Likely, another client set the ACL
// already and invalidated the etag.  Use GetACL to fetch a fresh etag.
var BadEtag = verror.Register(
	pkgPath+".badEtag",
	verror.RetryRefetch,
	"{1} {2} invalid etag passed to SetACL {_}",
)

// The ACL is too big.  Use groups to represent large sets of principals.
var TooBig = verror.Register(
	pkgPath+".tooBig",
	verror.NoRetry,
	"{1} {2} ACL is too big {_}",
)
