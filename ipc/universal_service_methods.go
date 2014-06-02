package ipc

// UniversalServiceMethods defines the set of methods that are implemented on
// all services.
type UniversalServiceMethods interface {
	// TODO(bprosnitz) Remove GetMethodTags and fetch the method tags from
	// signature instead.
	// GetMethodTags returns the tags associated with the given method.
	GetMethodTags(ctx Context, method string, opts ...CallOpt) ([]interface{}, error)
	// Signature returns a description of the service.
	Signature(ctx Context, opts ...CallOpt) (ServiceSignature, error)
	// UnresolveStep returns the names for the remote service, rooted at the
	// service's immediate namespace ancestor.
	UnresolveStep(ctx Context, opts ...CallOpt) ([]string, error)
}
