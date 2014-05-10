package ipc

import (
	"veyron2/verror"
	"veyron2/vom"
)

func init() {
	// The verror.Standard type is used by the ipc package to marshal errors.
	vom.Register(verror.Standard{})
}
