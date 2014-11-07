package ipc

import (
	"veyron.io/veyron/veyron2/context"
)

// UniversalServiceMethods defines the set of methods that are implemented on
// all services.
type UniversalServiceMethods interface {
	// TODO(bprosnitz) Remove GetMethodTags and fetch the method tags from
	// signature instead.
	// GetMethodTags returns the tags associated with the given method.
	GetMethodTags(ctx context.T, method string, opts ...CallOpt) ([]interface{}, error)
	// Signature returns a description of the service.
	Signature(ctx context.T, opts ...CallOpt) (ServiceSignature, error)
}
