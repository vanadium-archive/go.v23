package ipc

import (
	"veyron.io/veyron/veyron2/verror"
	"veyron.io/veyron/veyron2/vom"
)

func init() {
	// The verror.Standard type is used by the ipc package to marshal errors.
	vom.Register(verror.Standard{})
}
