// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package types

import (
	"v.io/v23/verror"
)

// pkgPath is the prefix of errors in this package.
const pkgPath = "v.io/v23/services/watch/types"

// The ResumeMarker provided is too far behind in the watch stream.
var UnknownResumeMarker = verror.Register(
	pkgPath+".unknownResumeMarker",
	verror.NoRetry,
	"{1} {2} unknown resume marker {_}",
)
