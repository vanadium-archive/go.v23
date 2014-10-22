package types

import (
	verror "veyron.io/veyron/veyron2/verror2"
)

// pkgPath is the prefix of errors in this package.
const pkgPath = "veyron.io/veyron/veyron2/services/watch/types"

// The ResumeMarker provided is too far behind in the watch stream.
var UnknownResumeMarker = verror.Register(
	pkgPath+".unknownResumeMarker",
	verror.NoRetry,
	"{1} {2} unknown resume marker {_}",
)
