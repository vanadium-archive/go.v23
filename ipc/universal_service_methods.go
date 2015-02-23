package ipc

// UniversalServiceMethods defines the set of methods that are implemented on
// all services.
//
// TODO(toddw): Remove this interface now that there aren't any universal
// methods?  Or should we add VDL-generated Signature / MethodSignature / Glob
// methods as a convenience?
type UniversalServiceMethods interface {
}
