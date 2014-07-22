// This file was auto-generated by the veyron vdl tool.
// Source: arith.vdl

// Package arith is an example of an IDL definition in veyron.  The syntax for
// IDL files is similar to, but not identical to, Go.  Here are the main
// concepts:
//   * PACKAGES - Just like in Go you must define the package at the beginning
//     of an IDL file, and everything defined in the file is part of this
//     package.  By convention all files in the same dir should be in the same
//     package.
//   * IMPORTS - Just like in Go you can import other idl packages, and you may
//     assign a local package name, or if unspecified the basename of the import
//     path is used as the import package name.
//   * DATA TYPES - Just like in Go you can define data types.  You get most of
//     the primitives (int32, float64, string, etc), the "error" built-in, and a
//     special "any" built-in described below.  In addition you can create
//     composite types like arrays, structs, etc.
//   * CONSTS - Just like in Go you can define constants, and numerics are
//     "infinite precision" within expressions.  Unlike Go numerics must be
//     typed to be used as const definitions or tags.
//   * INTERFACES - Just like in Go you can define interface types, which are
//     just a set of methods.  Interfaces can embed other interfaces.  Unlike
//     Go, you cannot use an interface as a data type; interfaces are purely
//     method sets.
//   * ERRORS - Errors may be defined in IDL files, and unlike Go they work
//     across separate address spaces.
package arith

import (
	"veyron2/vdl/test_base"

	// The non-user imports are prefixed with "_gen_" to prevent collisions.
	_gen_veyron2 "veyron2"
	_gen_context "veyron2/context"
	_gen_ipc "veyron2/ipc"
	_gen_naming "veyron2/naming"
	_gen_rt "veyron2/rt"
	_gen_vdlutil "veyron2/vdl/vdlutil"
	_gen_wiretype "veyron2/wiretype"
)

const (
	// Yes shows that bools may be untyped.
	Yes = true // yes trailing doc

	// No shows explicit boolean typing.
	No = false

	Hello = "hello"

	// Int32Const shows explicit integer typing.
	Int32Const = int32(123)

	// Int64Const shows explicit integer conversion from another type, and referencing
	// a constant from another package.
	Int64Const = int64(128)

	// FloatConst shows arithmetic expressions may be used.
	FloatConst = float64(2)

	// Mask shows bitwise operations.
	Mask = uint64(256)
)

// Arith is an example of an interface definition for an arithmetic service.
// Things to note:
//   * There must be at least 1 out-arg, and the last out-arg must be error.
// Arith is the interface the client binds and uses.
// Arith_ExcludingUniversal is the interface without internal framework-added methods
// to enable embedding without method collisions.  Not to be used directly by clients.
type Arith_ExcludingUniversal interface {
	// Add is a typical method with multiple input and output arguments.
	Add(ctx _gen_context.T, a int32, b int32, opts ..._gen_ipc.CallOpt) (reply int32, err error)
	// DivMod shows that runs of args with the same type can use the short form,
	// just like Go.
	DivMod(ctx _gen_context.T, a int32, b int32, opts ..._gen_ipc.CallOpt) (quot int32, rem int32, err error)
	// Sub shows that you can use data types defined in other packages.
	Sub(ctx _gen_context.T, args test_base.Args, opts ..._gen_ipc.CallOpt) (reply int32, err error)
	// Mul tries another data type defined in another package.
	Mul(ctx _gen_context.T, nested test_base.NestedArgs, opts ..._gen_ipc.CallOpt) (reply int32, err error)
	// GenError shows that it's fine to have no in args, and no out args other
	// than "error".  In addition GenError shows the usage of tags.  Tags are a
	// sequence of constants.  There's no requirement on uniqueness of types or
	// values, and regular const expressions may also be used.
	GenError(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (err error)
	// Count shows using only an int32 out-stream type, with no in-stream type.
	Count(ctx _gen_context.T, Start int32, opts ..._gen_ipc.CallOpt) (reply ArithCountStream, err error)
	// StreamingAdd shows a bidirectional stream.
	StreamingAdd(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply ArithStreamingAddStream, err error)
	// QuoteAny shows the any built-in type, representing a value of any type.
	QuoteAny(ctx _gen_context.T, a _gen_vdlutil.Any, opts ..._gen_ipc.CallOpt) (reply _gen_vdlutil.Any, err error)
}
type Arith interface {
	_gen_ipc.UniversalServiceMethods
	Arith_ExcludingUniversal
}

// ArithService is the interface the server implements.
type ArithService interface {

	// Add is a typical method with multiple input and output arguments.
	Add(context _gen_ipc.ServerContext, a int32, b int32) (reply int32, err error)
	// DivMod shows that runs of args with the same type can use the short form,
	// just like Go.
	DivMod(context _gen_ipc.ServerContext, a int32, b int32) (quot int32, rem int32, err error)
	// Sub shows that you can use data types defined in other packages.
	Sub(context _gen_ipc.ServerContext, args test_base.Args) (reply int32, err error)
	// Mul tries another data type defined in another package.
	Mul(context _gen_ipc.ServerContext, nested test_base.NestedArgs) (reply int32, err error)
	// GenError shows that it's fine to have no in args, and no out args other
	// than "error".  In addition GenError shows the usage of tags.  Tags are a
	// sequence of constants.  There's no requirement on uniqueness of types or
	// values, and regular const expressions may also be used.
	GenError(context _gen_ipc.ServerContext) (err error)
	// Count shows using only an int32 out-stream type, with no in-stream type.
	Count(context _gen_ipc.ServerContext, Start int32, stream ArithServiceCountStream) (err error)
	// StreamingAdd shows a bidirectional stream.
	StreamingAdd(context _gen_ipc.ServerContext, stream ArithServiceStreamingAddStream) (reply int32, err error)
	// QuoteAny shows the any built-in type, representing a value of any type.
	QuoteAny(context _gen_ipc.ServerContext, a _gen_vdlutil.Any) (reply _gen_vdlutil.Any, err error)
}

// ArithCountStream is the interface for streaming responses of the method
// Count in the service interface Arith.
type ArithCountStream interface {

	// Recv returns the next item in the input stream, blocking until
	// an item is available.  Returns io.EOF to indicate graceful end of input.
	Recv() (item int32, err error)

	// Finish closes the stream and returns the positional return values for
	// call.
	Finish() (err error)

	// Cancel cancels the RPC, notifying the server to stop processing.
	Cancel()
}

// Implementation of the ArithCountStream interface that is not exported.
type implArithCountStream struct {
	clientCall _gen_ipc.Call
}

func (c *implArithCountStream) Recv() (item int32, err error) {
	err = c.clientCall.Recv(&item)
	return
}

func (c *implArithCountStream) Finish() (err error) {
	if ierr := c.clientCall.Finish(&err); ierr != nil {
		err = ierr
	}
	return
}

func (c *implArithCountStream) Cancel() {
	c.clientCall.Cancel()
}

// ArithServiceCountStream is the interface for streaming responses of the method
// Count in the service interface Arith.
type ArithServiceCountStream interface {
	// Send places the item onto the output stream, blocking if there is no buffer
	// space available.
	Send(item int32) error
}

// Implementation of the ArithServiceCountStream interface that is not exported.
type implArithServiceCountStream struct {
	serverCall _gen_ipc.ServerCall
}

func (s *implArithServiceCountStream) Send(item int32) error {
	return s.serverCall.Send(item)
}

// ArithStreamingAddStream is the interface for streaming responses of the method
// StreamingAdd in the service interface Arith.
type ArithStreamingAddStream interface {

	// Send places the item onto the output stream, blocking if there is no buffer
	// space available.
	Send(item int32) error

	// CloseSend indicates to the server that no more items will be sent; server
	// Recv calls will receive io.EOF after all sent items.  Subsequent calls to
	// Send on the client will fail.  This is an optional call - it's used by
	// streaming clients that need the server to receive the io.EOF terminator.
	CloseSend() error

	// Recv returns the next item in the input stream, blocking until
	// an item is available.  Returns io.EOF to indicate graceful end of input.
	Recv() (item int32, err error)

	// Finish closes the stream and returns the positional return values for
	// call.
	Finish() (reply int32, err error)

	// Cancel cancels the RPC, notifying the server to stop processing.
	Cancel()
}

// Implementation of the ArithStreamingAddStream interface that is not exported.
type implArithStreamingAddStream struct {
	clientCall _gen_ipc.Call
}

func (c *implArithStreamingAddStream) Send(item int32) error {
	return c.clientCall.Send(item)
}

func (c *implArithStreamingAddStream) CloseSend() error {
	return c.clientCall.CloseSend()
}

func (c *implArithStreamingAddStream) Recv() (item int32, err error) {
	err = c.clientCall.Recv(&item)
	return
}

func (c *implArithStreamingAddStream) Finish() (reply int32, err error) {
	if ierr := c.clientCall.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (c *implArithStreamingAddStream) Cancel() {
	c.clientCall.Cancel()
}

// ArithServiceStreamingAddStream is the interface for streaming responses of the method
// StreamingAdd in the service interface Arith.
type ArithServiceStreamingAddStream interface {
	// Send places the item onto the output stream, blocking if there is no buffer
	// space available.
	Send(item int32) error

	// Recv fills itemptr with the next item in the input stream, blocking until
	// an item is available.  Returns io.EOF to indicate graceful end of input.
	Recv() (item int32, err error)
}

// Implementation of the ArithServiceStreamingAddStream interface that is not exported.
type implArithServiceStreamingAddStream struct {
	serverCall _gen_ipc.ServerCall
}

func (s *implArithServiceStreamingAddStream) Send(item int32) error {
	return s.serverCall.Send(item)
}

func (s *implArithServiceStreamingAddStream) Recv() (item int32, err error) {
	err = s.serverCall.Recv(&item)
	return
}

// BindArith returns the client stub implementing the Arith
// interface.
//
// If no _gen_ipc.Client is specified, the default _gen_ipc.Client in the
// global Runtime is used.
func BindArith(name string, opts ..._gen_ipc.BindOpt) (Arith, error) {
	var client _gen_ipc.Client
	switch len(opts) {
	case 0:
		client = _gen_rt.R().Client()
	case 1:
		switch o := opts[0].(type) {
		case _gen_veyron2.Runtime:
			client = o.Client()
		case _gen_ipc.Client:
			client = o
		default:
			return nil, _gen_vdlutil.ErrUnrecognizedOption
		}
	default:
		return nil, _gen_vdlutil.ErrTooManyOptionsToBind
	}
	stub := &clientStubArith{client: client, name: name}

	return stub, nil
}

// NewServerArith creates a new server stub.
//
// It takes a regular server implementing the ArithService
// interface, and returns a new server stub.
func NewServerArith(server ArithService) interface{} {
	return &ServerStubArith{
		service: server,
	}
}

// clientStubArith implements Arith.
type clientStubArith struct {
	client _gen_ipc.Client
	name   string
}

func (__gen_c *clientStubArith) Add(ctx _gen_context.T, a int32, b int32, opts ..._gen_ipc.CallOpt) (reply int32, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "Add", []interface{}{a, b}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubArith) DivMod(ctx _gen_context.T, a int32, b int32, opts ..._gen_ipc.CallOpt) (quot int32, rem int32, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "DivMod", []interface{}{a, b}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&quot, &rem, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubArith) Sub(ctx _gen_context.T, args test_base.Args, opts ..._gen_ipc.CallOpt) (reply int32, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "Sub", []interface{}{args}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubArith) Mul(ctx _gen_context.T, nested test_base.NestedArgs, opts ..._gen_ipc.CallOpt) (reply int32, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "Mul", []interface{}{nested}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubArith) GenError(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "GenError", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubArith) Count(ctx _gen_context.T, Start int32, opts ..._gen_ipc.CallOpt) (reply ArithCountStream, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "Count", []interface{}{Start}, opts...); err != nil {
		return
	}
	reply = &implArithCountStream{clientCall: call}
	return
}

func (__gen_c *clientStubArith) StreamingAdd(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply ArithStreamingAddStream, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "StreamingAdd", nil, opts...); err != nil {
		return
	}
	reply = &implArithStreamingAddStream{clientCall: call}
	return
}

func (__gen_c *clientStubArith) QuoteAny(ctx _gen_context.T, a _gen_vdlutil.Any, opts ..._gen_ipc.CallOpt) (reply _gen_vdlutil.Any, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "QuoteAny", []interface{}{a}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubArith) UnresolveStep(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply []string, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "UnresolveStep", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubArith) Signature(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply _gen_ipc.ServiceSignature, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "Signature", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubArith) GetMethodTags(ctx _gen_context.T, method string, opts ..._gen_ipc.CallOpt) (reply []interface{}, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "GetMethodTags", []interface{}{method}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

// ServerStubArith wraps a server that implements
// ArithService and provides an object that satisfies
// the requirements of veyron2/ipc.ReflectInvoker.
type ServerStubArith struct {
	service ArithService
}

func (__gen_s *ServerStubArith) GetMethodTags(call _gen_ipc.ServerCall, method string) ([]interface{}, error) {
	// TODO(bprosnitz) GetMethodTags() will be replaces with Signature().
	// Note: This exhibits some weird behavior like returning a nil error if the method isn't found.
	// This will change when it is replaced with Signature().
	switch method {
	case "Add":
		return []interface{}{}, nil
	case "DivMod":
		return []interface{}{}, nil
	case "Sub":
		return []interface{}{}, nil
	case "Mul":
		return []interface{}{}, nil
	case "GenError":
		return []interface{}{"foo", "barz", "hello", int32(129), uint64(36)}, nil
	case "Count":
		return []interface{}{}, nil
	case "StreamingAdd":
		return []interface{}{}, nil
	case "QuoteAny":
		return []interface{}{}, nil
	default:
		return nil, nil
	}
}

func (__gen_s *ServerStubArith) Signature(call _gen_ipc.ServerCall) (_gen_ipc.ServiceSignature, error) {
	result := _gen_ipc.ServiceSignature{Methods: make(map[string]_gen_ipc.MethodSignature)}
	result.Methods["Add"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "a", Type: 36},
			{Name: "b", Type: 36},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "", Type: 36},
			{Name: "", Type: 65},
		},
	}
	result.Methods["Count"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "Start", Type: 36},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "", Type: 65},
		},

		OutStream: 36,
	}
	result.Methods["DivMod"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "a", Type: 36},
			{Name: "b", Type: 36},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "quot", Type: 36},
			{Name: "rem", Type: 36},
			{Name: "err", Type: 65},
		},
	}
	result.Methods["GenError"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "", Type: 65},
		},
	}
	result.Methods["Mul"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "nested", Type: 67},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "", Type: 36},
			{Name: "", Type: 65},
		},
	}
	result.Methods["QuoteAny"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "a", Type: 68},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "", Type: 68},
			{Name: "", Type: 65},
		},
	}
	result.Methods["StreamingAdd"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "total", Type: 36},
			{Name: "err", Type: 65},
		},
		InStream:  36,
		OutStream: 36,
	}
	result.Methods["Sub"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{
			{Name: "args", Type: 66},
		},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "", Type: 36},
			{Name: "", Type: 65},
		},
	}

	result.TypeDefs = []_gen_vdlutil.Any{
		_gen_wiretype.NamedPrimitiveType{Type: 0x1, Name: "error", Tags: []string(nil)}, _gen_wiretype.StructType{
			[]_gen_wiretype.FieldType{
				_gen_wiretype.FieldType{Type: 0x24, Name: "A"},
				_gen_wiretype.FieldType{Type: 0x24, Name: "B"},
			},
			"veyron2/vdl/test_base.Args", []string(nil)},
		_gen_wiretype.StructType{
			[]_gen_wiretype.FieldType{
				_gen_wiretype.FieldType{Type: 0x42, Name: "Args"},
			},
			"veyron2/vdl/test_base.NestedArgs", []string(nil)},
		_gen_wiretype.NamedPrimitiveType{Type: 0x1, Name: "anydata", Tags: []string(nil)}}

	return result, nil
}

func (__gen_s *ServerStubArith) UnresolveStep(call _gen_ipc.ServerCall) (reply []string, err error) {
	if unresolver, ok := __gen_s.service.(_gen_ipc.Unresolver); ok {
		return unresolver.UnresolveStep(call)
	}
	if call.Server() == nil {
		return
	}
	var published []string
	if published, err = call.Server().Published(); err != nil || published == nil {
		return
	}
	reply = make([]string, len(published))
	for i, p := range published {
		reply[i] = _gen_naming.Join(p, call.Name())
	}
	return
}

func (__gen_s *ServerStubArith) Add(call _gen_ipc.ServerCall, a int32, b int32) (reply int32, err error) {
	reply, err = __gen_s.service.Add(call, a, b)
	return
}

func (__gen_s *ServerStubArith) DivMod(call _gen_ipc.ServerCall, a int32, b int32) (quot int32, rem int32, err error) {
	quot, rem, err = __gen_s.service.DivMod(call, a, b)
	return
}

func (__gen_s *ServerStubArith) Sub(call _gen_ipc.ServerCall, args test_base.Args) (reply int32, err error) {
	reply, err = __gen_s.service.Sub(call, args)
	return
}

func (__gen_s *ServerStubArith) Mul(call _gen_ipc.ServerCall, nested test_base.NestedArgs) (reply int32, err error) {
	reply, err = __gen_s.service.Mul(call, nested)
	return
}

func (__gen_s *ServerStubArith) GenError(call _gen_ipc.ServerCall) (err error) {
	err = __gen_s.service.GenError(call)
	return
}

func (__gen_s *ServerStubArith) Count(call _gen_ipc.ServerCall, Start int32) (err error) {
	stream := &implArithServiceCountStream{serverCall: call}
	err = __gen_s.service.Count(call, Start, stream)
	return
}

func (__gen_s *ServerStubArith) StreamingAdd(call _gen_ipc.ServerCall) (reply int32, err error) {
	stream := &implArithServiceStreamingAddStream{serverCall: call}
	reply, err = __gen_s.service.StreamingAdd(call, stream)
	return
}

func (__gen_s *ServerStubArith) QuoteAny(call _gen_ipc.ServerCall, a _gen_vdlutil.Any) (reply _gen_vdlutil.Any, err error) {
	reply, err = __gen_s.service.QuoteAny(call, a)
	return
}

// Calculator is the interface the client binds and uses.
// Calculator_ExcludingUniversal is the interface without internal framework-added methods
// to enable embedding without method collisions.  Not to be used directly by clients.
type Calculator_ExcludingUniversal interface {
	// Arith is an example of an interface definition for an arithmetic service.
	// Things to note:
	//   * There must be at least 1 out-arg, and the last out-arg must be error.
	Arith_ExcludingUniversal
	// AdvancedMath is an interface for more advanced math than arith.  It embeds
	// interfaces defined both in the same file and in an external package; and in
	// turn it is embedded by arith.Calculator (which is in the same package but
	// different file) to verify that embedding works in all these scenarios.
	AdvancedMath_ExcludingUniversal
	On(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (err error)  // On turns the calculator on.
	Off(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (err error) // Off turns the calculator off.
}
type Calculator interface {
	_gen_ipc.UniversalServiceMethods
	Calculator_ExcludingUniversal
}

// CalculatorService is the interface the server implements.
type CalculatorService interface {

	// Arith is an example of an interface definition for an arithmetic service.
	// Things to note:
	//   * There must be at least 1 out-arg, and the last out-arg must be error.
	ArithService
	// AdvancedMath is an interface for more advanced math than arith.  It embeds
	// interfaces defined both in the same file and in an external package; and in
	// turn it is embedded by arith.Calculator (which is in the same package but
	// different file) to verify that embedding works in all these scenarios.
	AdvancedMathService
	On(context _gen_ipc.ServerContext) (err error)  // On turns the calculator on.
	Off(context _gen_ipc.ServerContext) (err error) // Off turns the calculator off.
}

// BindCalculator returns the client stub implementing the Calculator
// interface.
//
// If no _gen_ipc.Client is specified, the default _gen_ipc.Client in the
// global Runtime is used.
func BindCalculator(name string, opts ..._gen_ipc.BindOpt) (Calculator, error) {
	var client _gen_ipc.Client
	switch len(opts) {
	case 0:
		client = _gen_rt.R().Client()
	case 1:
		switch o := opts[0].(type) {
		case _gen_veyron2.Runtime:
			client = o.Client()
		case _gen_ipc.Client:
			client = o
		default:
			return nil, _gen_vdlutil.ErrUnrecognizedOption
		}
	default:
		return nil, _gen_vdlutil.ErrTooManyOptionsToBind
	}
	stub := &clientStubCalculator{client: client, name: name}
	stub.Arith_ExcludingUniversal, _ = BindArith(name, client)
	stub.AdvancedMath_ExcludingUniversal, _ = BindAdvancedMath(name, client)

	return stub, nil
}

// NewServerCalculator creates a new server stub.
//
// It takes a regular server implementing the CalculatorService
// interface, and returns a new server stub.
func NewServerCalculator(server CalculatorService) interface{} {
	return &ServerStubCalculator{
		ServerStubArith:        *NewServerArith(server).(*ServerStubArith),
		ServerStubAdvancedMath: *NewServerAdvancedMath(server).(*ServerStubAdvancedMath),
		service:                server,
	}
}

// clientStubCalculator implements Calculator.
type clientStubCalculator struct {
	Arith_ExcludingUniversal
	AdvancedMath_ExcludingUniversal

	client _gen_ipc.Client
	name   string
}

func (__gen_c *clientStubCalculator) On(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "On", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubCalculator) Off(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "Off", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubCalculator) UnresolveStep(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply []string, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "UnresolveStep", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubCalculator) Signature(ctx _gen_context.T, opts ..._gen_ipc.CallOpt) (reply _gen_ipc.ServiceSignature, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "Signature", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

func (__gen_c *clientStubCalculator) GetMethodTags(ctx _gen_context.T, method string, opts ..._gen_ipc.CallOpt) (reply []interface{}, err error) {
	var call _gen_ipc.Call
	if call, err = __gen_c.client.StartCall(ctx, __gen_c.name, "GetMethodTags", []interface{}{method}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&reply, &err); ierr != nil {
		err = ierr
	}
	return
}

// ServerStubCalculator wraps a server that implements
// CalculatorService and provides an object that satisfies
// the requirements of veyron2/ipc.ReflectInvoker.
type ServerStubCalculator struct {
	ServerStubArith
	ServerStubAdvancedMath

	service CalculatorService
}

func (__gen_s *ServerStubCalculator) GetMethodTags(call _gen_ipc.ServerCall, method string) ([]interface{}, error) {
	// TODO(bprosnitz) GetMethodTags() will be replaces with Signature().
	// Note: This exhibits some weird behavior like returning a nil error if the method isn't found.
	// This will change when it is replaced with Signature().
	if resp, err := __gen_s.ServerStubArith.GetMethodTags(call, method); resp != nil || err != nil {
		return resp, err
	}
	if resp, err := __gen_s.ServerStubAdvancedMath.GetMethodTags(call, method); resp != nil || err != nil {
		return resp, err
	}
	switch method {
	case "On":
		return []interface{}{}, nil
	case "Off":
		return []interface{}{"offtag"}, nil
	default:
		return nil, nil
	}
}

func (__gen_s *ServerStubCalculator) Signature(call _gen_ipc.ServerCall) (_gen_ipc.ServiceSignature, error) {
	result := _gen_ipc.ServiceSignature{Methods: make(map[string]_gen_ipc.MethodSignature)}
	result.Methods["Off"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "", Type: 65},
		},
	}
	result.Methods["On"] = _gen_ipc.MethodSignature{
		InArgs: []_gen_ipc.MethodArgument{},
		OutArgs: []_gen_ipc.MethodArgument{
			{Name: "", Type: 65},
		},
	}

	result.TypeDefs = []_gen_vdlutil.Any{
		_gen_wiretype.NamedPrimitiveType{Type: 0x1, Name: "error", Tags: []string(nil)}}
	var ss _gen_ipc.ServiceSignature
	var firstAdded int
	ss, _ = __gen_s.ServerStubArith.Signature(call)
	firstAdded = len(result.TypeDefs)
	for k, v := range ss.Methods {
		for i, _ := range v.InArgs {
			if v.InArgs[i].Type >= _gen_wiretype.TypeIDFirst {
				v.InArgs[i].Type += _gen_wiretype.TypeID(firstAdded)
			}
		}
		for i, _ := range v.OutArgs {
			if v.OutArgs[i].Type >= _gen_wiretype.TypeIDFirst {
				v.OutArgs[i].Type += _gen_wiretype.TypeID(firstAdded)
			}
		}
		if v.InStream >= _gen_wiretype.TypeIDFirst {
			v.InStream += _gen_wiretype.TypeID(firstAdded)
		}
		if v.OutStream >= _gen_wiretype.TypeIDFirst {
			v.OutStream += _gen_wiretype.TypeID(firstAdded)
		}
		result.Methods[k] = v
	}
	//TODO(bprosnitz) combine type definitions from embeded interfaces in a way that doesn't cause duplication.
	for _, d := range ss.TypeDefs {
		switch wt := d.(type) {
		case _gen_wiretype.SliceType:
			if wt.Elem >= _gen_wiretype.TypeIDFirst {
				wt.Elem += _gen_wiretype.TypeID(firstAdded)
			}
			d = wt
		case _gen_wiretype.ArrayType:
			if wt.Elem >= _gen_wiretype.TypeIDFirst {
				wt.Elem += _gen_wiretype.TypeID(firstAdded)
			}
			d = wt
		case _gen_wiretype.MapType:
			if wt.Key >= _gen_wiretype.TypeIDFirst {
				wt.Key += _gen_wiretype.TypeID(firstAdded)
			}
			if wt.Elem >= _gen_wiretype.TypeIDFirst {
				wt.Elem += _gen_wiretype.TypeID(firstAdded)
			}
			d = wt
		case _gen_wiretype.StructType:
			for i, fld := range wt.Fields {
				if fld.Type >= _gen_wiretype.TypeIDFirst {
					wt.Fields[i].Type += _gen_wiretype.TypeID(firstAdded)
				}
			}
			d = wt
			// NOTE: other types are missing, but we are upgrading anyways.
		}
		result.TypeDefs = append(result.TypeDefs, d)
	}
	ss, _ = __gen_s.ServerStubAdvancedMath.Signature(call)
	firstAdded = len(result.TypeDefs)
	for k, v := range ss.Methods {
		for i, _ := range v.InArgs {
			if v.InArgs[i].Type >= _gen_wiretype.TypeIDFirst {
				v.InArgs[i].Type += _gen_wiretype.TypeID(firstAdded)
			}
		}
		for i, _ := range v.OutArgs {
			if v.OutArgs[i].Type >= _gen_wiretype.TypeIDFirst {
				v.OutArgs[i].Type += _gen_wiretype.TypeID(firstAdded)
			}
		}
		if v.InStream >= _gen_wiretype.TypeIDFirst {
			v.InStream += _gen_wiretype.TypeID(firstAdded)
		}
		if v.OutStream >= _gen_wiretype.TypeIDFirst {
			v.OutStream += _gen_wiretype.TypeID(firstAdded)
		}
		result.Methods[k] = v
	}
	//TODO(bprosnitz) combine type definitions from embeded interfaces in a way that doesn't cause duplication.
	for _, d := range ss.TypeDefs {
		switch wt := d.(type) {
		case _gen_wiretype.SliceType:
			if wt.Elem >= _gen_wiretype.TypeIDFirst {
				wt.Elem += _gen_wiretype.TypeID(firstAdded)
			}
			d = wt
		case _gen_wiretype.ArrayType:
			if wt.Elem >= _gen_wiretype.TypeIDFirst {
				wt.Elem += _gen_wiretype.TypeID(firstAdded)
			}
			d = wt
		case _gen_wiretype.MapType:
			if wt.Key >= _gen_wiretype.TypeIDFirst {
				wt.Key += _gen_wiretype.TypeID(firstAdded)
			}
			if wt.Elem >= _gen_wiretype.TypeIDFirst {
				wt.Elem += _gen_wiretype.TypeID(firstAdded)
			}
			d = wt
		case _gen_wiretype.StructType:
			for i, fld := range wt.Fields {
				if fld.Type >= _gen_wiretype.TypeIDFirst {
					wt.Fields[i].Type += _gen_wiretype.TypeID(firstAdded)
				}
			}
			d = wt
			// NOTE: other types are missing, but we are upgrading anyways.
		}
		result.TypeDefs = append(result.TypeDefs, d)
	}

	return result, nil
}

func (__gen_s *ServerStubCalculator) UnresolveStep(call _gen_ipc.ServerCall) (reply []string, err error) {
	if unresolver, ok := __gen_s.service.(_gen_ipc.Unresolver); ok {
		return unresolver.UnresolveStep(call)
	}
	if call.Server() == nil {
		return
	}
	var published []string
	if published, err = call.Server().Published(); err != nil || published == nil {
		return
	}
	reply = make([]string, len(published))
	for i, p := range published {
		reply[i] = _gen_naming.Join(p, call.Name())
	}
	return
}

func (__gen_s *ServerStubCalculator) On(call _gen_ipc.ServerCall) (err error) {
	err = __gen_s.service.On(call)
	return
}

func (__gen_s *ServerStubCalculator) Off(call _gen_ipc.ServerCall) (err error) {
	err = __gen_s.service.Off(call)
	return
}
