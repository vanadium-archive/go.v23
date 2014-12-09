package reserved

import (
	"veyron.io/veyron/veyron2"
	"veyron.io/veyron/veyron2/context"
	"veyron.io/veyron/veyron2/ipc"
	"veyron.io/veyron/veyron2/vdl/vdlroot/src/signature"
)

func cl(ctx context.T, client ipc.Client) ipc.Client {
	if client != nil {
		return client
	}
	return veyron2.RuntimeFromContext(ctx).Client()
}

// Signature invokes the reserved signature RPC on the given name, and returns
// the results.  The client will be used to invoke the RPC - if it is nil, the
// default client from the runtime is used.
func Signature(ctx context.T, client ipc.Client, name string, opts ...ipc.CallOpt) ([]signature.Interface, error) {
	call, err := cl(ctx, client).StartCall(ctx, name, ipc.ReservedSignature, nil, opts...)
	if err != nil {
		return nil, err
	}
	var sig []signature.Interface
	if ierr := call.Finish(&sig, &err); ierr != nil {
		err = ierr
	}
	return sig, err
}

// MethodSignature invokes the reserved method signature RPC on the given name,
// and returns the results.  The client will be used to invoke the RPC - if it
// is nil, the default client from the runtime is used.
func MethodSignature(ctx context.T, client ipc.Client, name, method string, opts ...ipc.CallOpt) (signature.Method, error) {
	args := []interface{}{method}
	call, err := cl(ctx, client).StartCall(ctx, name, ipc.ReservedMethodSignature, args, opts...)
	if err != nil {
		return signature.Method{}, err
	}
	var sig signature.Method
	if ferr := call.Finish(&sig, &err); ferr != nil {
		err = ferr
	}
	return sig, err
}
