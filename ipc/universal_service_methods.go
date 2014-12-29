package ipc

import (
	"v.io/core/veyron2/context"
)

// UniversalServiceMethods defines the set of methods that are implemented on
// all services.
type UniversalServiceMethods interface {
	// Signature returns a description of the service.
	//
	// TODO(toddw): Replace with reserved Signature__ method.
	Signature(ctx *context.T, opts ...CallOpt) (ServiceSignature, error)
}
