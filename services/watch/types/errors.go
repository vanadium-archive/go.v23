package types

import (
	"v.io/core/veyron2/verror"
)

// pkgPath is the prefix of errors in this package.
const pkgPath = "v.io/core/veyron2/services/watch/types"

// The ResumeMarker provided is too far behind in the watch stream.
var UnknownResumeMarker = verror.Register(
	pkgPath+".unknownResumeMarker",
	verror.NoRetry,
	"{1} {2} unknown resume marker {_}",
)
