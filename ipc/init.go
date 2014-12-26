package ipc

import (
	"v.io/core/veyron2/verror"
	"v.io/core/veyron2/verror2"
	"v.io/core/veyron2/vom"

	// Ensure all standard vdl types are registered.
	_ "v.io/core/veyron2/vdl/vdlroot"
)

func init() {
	// The verror.Standard type is used by the ipc package to marshal errors.
	// TODO(toddw): Remove this after the vom2 transition.
	vom.Register(verror.Standard{})
	vom.Register(verror2.Standard{})
}
