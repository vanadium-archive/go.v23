package reserved

import (
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/ipc"
	"v.io/v23/vdlroot/signature"
)

// Signature invokes the reserved signature RPC on the given name, and returns
// the results.  The client will be used to invoke the RPC - if it is nil, the
// default client from the runtime is used.
func Signature(ctx *context.T, name string, opts ...ipc.CallOpt) ([]signature.Interface, error) {
	call, err := v23.GetClient(ctx).StartCall(ctx, name, ipc.ReservedSignature, nil, opts...)
	if err != nil {
		return nil, err
	}
	var sig []signature.Interface
	if err := call.Finish(&sig); err != nil {
		return nil, err
	}
	return sig, nil
}

// MethodSignature invokes the reserved method signature RPC on the given name,
// and returns the results.  The client will be used to invoke the RPC - if it
// is nil, the default client from the runtime is used.
func MethodSignature(ctx *context.T, name, method string, opts ...ipc.CallOpt) (signature.Method, error) {
	args := []interface{}{method}
	call, err := v23.GetClient(ctx).StartCall(ctx, name, ipc.ReservedMethodSignature, args, opts...)
	if err != nil {
		return signature.Method{}, err
	}
	var sig signature.Method
	if err := call.Finish(&sig); err != nil {
		return signature.Method{}, err
	}
	return sig, nil
}
