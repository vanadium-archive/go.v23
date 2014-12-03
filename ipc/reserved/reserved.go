package reserved

import (
	"veyron.io/veyron/veyron2"
	"veyron.io/veyron/veyron2/context"
	"veyron.io/veyron/veyron2/ipc"
	"veyron.io/veyron/veyron2/vdl/vdlroot/src/signature"
)

func client(ctx context.T, opts []ipc.CallOpt) ipc.Client {
	return veyron2.RuntimeFromContext(ctx).Client()
}

// Signature returns the signature for the given name.
func Signature(ctx context.T, name string, opts ...ipc.CallOpt) ([]signature.Interface, error) {
	call, err := client(ctx, opts).StartCall(ctx, name, ipc.ReservedSignature, nil, opts...)
	if err != nil {
		return nil, err
	}
	var sig []signature.Interface
	if ierr := call.Finish(&sig, &err); ierr != nil {
		err = ierr
	}
	return sig, err
}

// MethodSignature returns the method signature for the given name and method.
func MethodSignature(ctx context.T, name, method string, opts ...ipc.CallOpt) (signature.Method, error) {
	args := []interface{}{method}
	call, err := client(ctx, opts).StartCall(ctx, name, ipc.ReservedMethodSignature, args, opts...)
	if err != nil {
		return signature.Method{}, err
	}
	var sig signature.Method
	if ferr := call.Finish(&sig, &err); ferr != nil {
		err = ferr
	}
	return sig, err
}
