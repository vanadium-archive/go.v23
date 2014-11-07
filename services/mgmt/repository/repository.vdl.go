// This file was auto-generated by the veyron vdl tool.
// Source: repository.vdl

// Package repository can be used for storing and serving various
// veyron management objects.
package repository

import (
	"veyron.io/veyron/veyron2/security"

	"veyron.io/veyron/veyron2/services/mgmt/application"

	"veyron.io/veyron/veyron2/services/mgmt/binary"

	// The non-user imports are prefixed with "__" to prevent collisions.
	__io "io"
	__veyron2 "veyron.io/veyron/veyron2"
	__context "veyron.io/veyron/veyron2/context"
	__ipc "veyron.io/veyron/veyron2/ipc"
	__vdlutil "veyron.io/veyron/veyron2/vdl/vdlutil"
	__wiretype "veyron.io/veyron/veyron2/wiretype"
)

// TODO(toddw): Remove this line once the new signature support is done.
// It corrects a bug where __wiretype is unused in VDL pacakges where only
// bootstrap types are used on interfaces.
const _ = __wiretype.TypeIDInvalid

// ApplicationClientMethods is the client interface
// containing Application methods.
//
// Application provides access to application envelopes. An
// application envelope is identified by an application name and an
// application version, which are specified through the object name,
// and a profile name, which is specified using a method argument.
//
// Example:
// /apps/search/v1.Match([]string{"base", "media"})
//   returns an application envelope that can be used for downloading
//   and executing the "search" application, version "v1", runnable
//   on either the "base" or "media" profile.
type ApplicationClientMethods interface {
	// Match checks if any of the given profiles contains an application
	// envelope for the given application version (specified through the
	// object name suffix) and if so, returns this envelope. If multiple
	// profile matches are possible, the method returns the first
	// matching profile, respecting the order of the input argument.
	Match(ctx __context.T, Profiles []string, opts ...__ipc.CallOpt) (application.Envelope, error)
}

// ApplicationClientStub adds universal methods to ApplicationClientMethods.
type ApplicationClientStub interface {
	ApplicationClientMethods
	__ipc.UniversalServiceMethods
}

// ApplicationClient returns a client stub for Application.
func ApplicationClient(name string, opts ...__ipc.BindOpt) ApplicationClientStub {
	var client __ipc.Client
	for _, opt := range opts {
		if clientOpt, ok := opt.(__ipc.Client); ok {
			client = clientOpt
		}
	}
	return implApplicationClientStub{name, client}
}

type implApplicationClientStub struct {
	name   string
	client __ipc.Client
}

func (c implApplicationClientStub) c(ctx __context.T) __ipc.Client {
	if c.client != nil {
		return c.client
	}
	return __veyron2.RuntimeFromContext(ctx).Client()
}

func (c implApplicationClientStub) Match(ctx __context.T, i0 []string, opts ...__ipc.CallOpt) (o0 application.Envelope, err error) {
	var call __ipc.Call
	if call, err = c.c(ctx).StartCall(ctx, c.name, "Match", []interface{}{i0}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&o0, &err); ierr != nil {
		err = ierr
	}
	return
}

func (c implApplicationClientStub) Signature(ctx __context.T, opts ...__ipc.CallOpt) (o0 __ipc.ServiceSignature, err error) {
	var call __ipc.Call
	if call, err = c.c(ctx).StartCall(ctx, c.name, "Signature", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&o0, &err); ierr != nil {
		err = ierr
	}
	return
}

func (c implApplicationClientStub) GetMethodTags(ctx __context.T, method string, opts ...__ipc.CallOpt) (o0 []interface{}, err error) {
	var call __ipc.Call
	if call, err = c.c(ctx).StartCall(ctx, c.name, "GetMethodTags", []interface{}{method}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&o0, &err); ierr != nil {
		err = ierr
	}
	return
}

// ApplicationServerMethods is the interface a server writer
// implements for Application.
//
// Application provides access to application envelopes. An
// application envelope is identified by an application name and an
// application version, which are specified through the object name,
// and a profile name, which is specified using a method argument.
//
// Example:
// /apps/search/v1.Match([]string{"base", "media"})
//   returns an application envelope that can be used for downloading
//   and executing the "search" application, version "v1", runnable
//   on either the "base" or "media" profile.
type ApplicationServerMethods interface {
	// Match checks if any of the given profiles contains an application
	// envelope for the given application version (specified through the
	// object name suffix) and if so, returns this envelope. If multiple
	// profile matches are possible, the method returns the first
	// matching profile, respecting the order of the input argument.
	Match(ctx __ipc.ServerContext, Profiles []string) (application.Envelope, error)
}

// ApplicationServerStubMethods is the server interface containing
// Application methods, as expected by ipc.Server.  The difference between
// this interface and ApplicationServerMethods is that the first context
// argument for each method is always ipc.ServerCall here, while it is either
// ipc.ServerContext or a typed streaming context there.
type ApplicationServerStubMethods interface {
	// Match checks if any of the given profiles contains an application
	// envelope for the given application version (specified through the
	// object name suffix) and if so, returns this envelope. If multiple
	// profile matches are possible, the method returns the first
	// matching profile, respecting the order of the input argument.
	Match(call __ipc.ServerCall, Profiles []string) (application.Envelope, error)
}

// ApplicationServerStub adds universal methods to ApplicationServerStubMethods.
type ApplicationServerStub interface {
	ApplicationServerStubMethods
	// GetMethodTags will be replaced with DescribeInterfaces.
	GetMethodTags(call __ipc.ServerCall, method string) ([]interface{}, error)
	// Signature will be replaced with DescribeInterfaces.
	Signature(call __ipc.ServerCall) (__ipc.ServiceSignature, error)
}

// ApplicationServer returns a server stub for Application.
// It converts an implementation of ApplicationServerMethods into
// an object that may be used by ipc.Server.
func ApplicationServer(impl ApplicationServerMethods) ApplicationServerStub {
	stub := implApplicationServerStub{
		impl: impl,
	}
	// Initialize GlobState; always check the stub itself first, to handle the
	// case where the user has the Glob method defined in their VDL source.
	if gs := __ipc.NewGlobState(stub); gs != nil {
		stub.gs = gs
	} else if gs := __ipc.NewGlobState(impl); gs != nil {
		stub.gs = gs
	}
	return stub
}

type implApplicationServerStub struct {
	impl ApplicationServerMethods
	gs   *__ipc.GlobState
}

func (s implApplicationServerStub) Match(call __ipc.ServerCall, i0 []string) (application.Envelope, error) {
	return s.impl.Match(call, i0)
}

func (s implApplicationServerStub) VGlob() *__ipc.GlobState {
	return s.gs
}

func (s implApplicationServerStub) GetMethodTags(call __ipc.ServerCall, method string) ([]interface{}, error) {
	// TODO(toddw): Replace with new DescribeInterfaces implementation.
	switch method {
	case "Match":
		return []interface{}{security.Label(2)}, nil
	default:
		return nil, nil
	}
}

func (s implApplicationServerStub) Signature(call __ipc.ServerCall) (__ipc.ServiceSignature, error) {
	// TODO(toddw) Replace with new DescribeInterfaces implementation.
	result := __ipc.ServiceSignature{Methods: make(map[string]__ipc.MethodSignature)}
	result.Methods["Match"] = __ipc.MethodSignature{
		InArgs: []__ipc.MethodArgument{
			{Name: "Profiles", Type: 61},
		},
		OutArgs: []__ipc.MethodArgument{
			{Name: "", Type: 65},
			{Name: "", Type: 66},
		},
	}

	result.TypeDefs = []__vdlutil.Any{
		__wiretype.StructType{
			[]__wiretype.FieldType{
				__wiretype.FieldType{Type: 0x3, Name: "Title"},
				__wiretype.FieldType{Type: 0x3d, Name: "Args"},
				__wiretype.FieldType{Type: 0x3, Name: "Binary"},
				__wiretype.FieldType{Type: 0x3d, Name: "Env"},
			},
			"veyron.io/veyron/veyron2/services/mgmt/application.Envelope", []string(nil)},
		__wiretype.NamedPrimitiveType{Type: 0x1, Name: "error", Tags: []string(nil)}}

	return result, nil
}

// BinaryClientMethods is the client interface
// containing Binary methods.
//
// Binary can be used to store and retrieve veyron application
// binaries.
//
// To create a binary, clients first invoke the Create() method that
// specifies the number of parts the binary consists of. Clients then
// uploads the individual parts through the Upload() method, which
// identifies the part being uploaded. To resume an upload after a
// failure, clients invoke the UploadStatus() method, which returns a
// slice that identifies which parts are missing.
//
// To download a binary, clients first invoke Stat(), which returns
// information describing the binary, including the number of parts
// the binary consists of. Clients then download the individual parts
// through the Download() method, which identifies the part being
// downloaded. Alternatively, clients can download the binary through
// HTTP using a transient URL available through the DownloadURL()
// method.
//
// To delete the binary, clients invoke the Delete() method.
type BinaryClientMethods interface {
	// Create expresses the intent to create a binary identified by the
	// object name suffix consisting of the given number of parts. If
	// the suffix identifies a binary that has already been created, the
	// method returns an error.
	Create(ctx __context.T, nparts int32, opts ...__ipc.CallOpt) error
	// Delete deletes the binary identified by the object name
	// suffix. If the binary that has not been created, the method
	// returns an error.
	Delete(__context.T, ...__ipc.CallOpt) error
	// Download opens a stream that can used for downloading the given
	// part of the binary identified by the object name suffix. If the
	// binary part has not been uploaded, the method returns an
	// error. If the Delete() method is invoked when the Download()
	// method is in progress, the outcome the Download() method is
	// undefined.
	Download(ctx __context.T, part int32, opts ...__ipc.CallOpt) (BinaryDownloadCall, error)
	// DownloadURL returns a transient URL from which the binary
	// identified by the object name suffix can be downloaded using the
	// HTTP protocol. If not all parts of the binary have been uploaded,
	// the method returns an error.
	DownloadURL(__context.T, ...__ipc.CallOpt) (URL string, TTL int64, err error)
	// Stat returns information describing the parts of the binary
	// identified by the object name suffix. If the binary has not been
	// created, the method returns an error.
	Stat(__context.T, ...__ipc.CallOpt) ([]binary.PartInfo, error)
	// Upload opens a stream that can be used for uploading the given
	// part of the binary identified by the object name suffix. If the
	// binary has not been created, the method returns an error. If the
	// binary part has been uploaded, the method returns an error. If
	// the same binary part is being uploaded by another caller, the
	// method returns an error.
	Upload(ctx __context.T, part int32, opts ...__ipc.CallOpt) (BinaryUploadCall, error)
}

// BinaryClientStub adds universal methods to BinaryClientMethods.
type BinaryClientStub interface {
	BinaryClientMethods
	__ipc.UniversalServiceMethods
}

// BinaryClient returns a client stub for Binary.
func BinaryClient(name string, opts ...__ipc.BindOpt) BinaryClientStub {
	var client __ipc.Client
	for _, opt := range opts {
		if clientOpt, ok := opt.(__ipc.Client); ok {
			client = clientOpt
		}
	}
	return implBinaryClientStub{name, client}
}

type implBinaryClientStub struct {
	name   string
	client __ipc.Client
}

func (c implBinaryClientStub) c(ctx __context.T) __ipc.Client {
	if c.client != nil {
		return c.client
	}
	return __veyron2.RuntimeFromContext(ctx).Client()
}

func (c implBinaryClientStub) Create(ctx __context.T, i0 int32, opts ...__ipc.CallOpt) (err error) {
	var call __ipc.Call
	if call, err = c.c(ctx).StartCall(ctx, c.name, "Create", []interface{}{i0}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&err); ierr != nil {
		err = ierr
	}
	return
}

func (c implBinaryClientStub) Delete(ctx __context.T, opts ...__ipc.CallOpt) (err error) {
	var call __ipc.Call
	if call, err = c.c(ctx).StartCall(ctx, c.name, "Delete", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&err); ierr != nil {
		err = ierr
	}
	return
}

func (c implBinaryClientStub) Download(ctx __context.T, i0 int32, opts ...__ipc.CallOpt) (ocall BinaryDownloadCall, err error) {
	var call __ipc.Call
	if call, err = c.c(ctx).StartCall(ctx, c.name, "Download", []interface{}{i0}, opts...); err != nil {
		return
	}
	ocall = &implBinaryDownloadCall{call, implBinaryDownloadClientRecv{call: call}}
	return
}

func (c implBinaryClientStub) DownloadURL(ctx __context.T, opts ...__ipc.CallOpt) (o0 string, o1 int64, err error) {
	var call __ipc.Call
	if call, err = c.c(ctx).StartCall(ctx, c.name, "DownloadURL", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&o0, &o1, &err); ierr != nil {
		err = ierr
	}
	return
}

func (c implBinaryClientStub) Stat(ctx __context.T, opts ...__ipc.CallOpt) (o0 []binary.PartInfo, err error) {
	var call __ipc.Call
	if call, err = c.c(ctx).StartCall(ctx, c.name, "Stat", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&o0, &err); ierr != nil {
		err = ierr
	}
	return
}

func (c implBinaryClientStub) Upload(ctx __context.T, i0 int32, opts ...__ipc.CallOpt) (ocall BinaryUploadCall, err error) {
	var call __ipc.Call
	if call, err = c.c(ctx).StartCall(ctx, c.name, "Upload", []interface{}{i0}, opts...); err != nil {
		return
	}
	ocall = &implBinaryUploadCall{call, implBinaryUploadClientSend{call}}
	return
}

func (c implBinaryClientStub) Signature(ctx __context.T, opts ...__ipc.CallOpt) (o0 __ipc.ServiceSignature, err error) {
	var call __ipc.Call
	if call, err = c.c(ctx).StartCall(ctx, c.name, "Signature", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&o0, &err); ierr != nil {
		err = ierr
	}
	return
}

func (c implBinaryClientStub) GetMethodTags(ctx __context.T, method string, opts ...__ipc.CallOpt) (o0 []interface{}, err error) {
	var call __ipc.Call
	if call, err = c.c(ctx).StartCall(ctx, c.name, "GetMethodTags", []interface{}{method}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&o0, &err); ierr != nil {
		err = ierr
	}
	return
}

// BinaryDownloadClientStream is the client stream for Binary.Download.
type BinaryDownloadClientStream interface {
	// RecvStream returns the receiver side of the client stream.
	RecvStream() interface {
		// Advance stages an item so that it may be retrieved via Value.  Returns
		// true iff there is an item to retrieve.  Advance must be called before
		// Value is called.  May block if an item is not available.
		Advance() bool
		// Value returns the item that was staged by Advance.  May panic if Advance
		// returned false or was not called.  Never blocks.
		Value() []byte
		// Err returns any error encountered by Advance.  Never blocks.
		Err() error
	}
}

// BinaryDownloadCall represents the call returned from Binary.Download.
type BinaryDownloadCall interface {
	BinaryDownloadClientStream
	// Finish blocks until the server is done, and returns the positional return
	// values for call.
	//
	// Finish returns immediately if Cancel has been called; depending on the
	// timing the output could either be an error signaling cancelation, or the
	// valid positional return values from the server.
	//
	// Calling Finish is mandatory for releasing stream resources, unless Cancel
	// has been called or any of the other methods return an error.  Finish should
	// be called at most once.
	Finish() error
	// Cancel cancels the RPC, notifying the server to stop processing.  It is
	// safe to call Cancel concurrently with any of the other stream methods.
	// Calling Cancel after Finish has returned is a no-op.
	Cancel()
}

type implBinaryDownloadClientRecv struct {
	call __ipc.Call
	val  []byte
	err  error
}

func (c *implBinaryDownloadClientRecv) Advance() bool {
	c.err = c.call.Recv(&c.val)
	return c.err == nil
}
func (c *implBinaryDownloadClientRecv) Value() []byte {
	return c.val
}
func (c *implBinaryDownloadClientRecv) Err() error {
	if c.err == __io.EOF {
		return nil
	}
	return c.err
}

type implBinaryDownloadCall struct {
	call __ipc.Call
	recv implBinaryDownloadClientRecv
}

func (c *implBinaryDownloadCall) RecvStream() interface {
	Advance() bool
	Value() []byte
	Err() error
} {
	return &c.recv
}
func (c *implBinaryDownloadCall) Finish() (err error) {
	if ierr := c.call.Finish(&err); ierr != nil {
		err = ierr
	}
	return
}
func (c *implBinaryDownloadCall) Cancel() {
	c.call.Cancel()
}

// BinaryUploadClientStream is the client stream for Binary.Upload.
type BinaryUploadClientStream interface {
	// SendStream returns the send side of the client stream.
	SendStream() interface {
		// Send places the item onto the output stream.  Returns errors encountered
		// while sending, or if Send is called after Close or Cancel.  Blocks if
		// there is no buffer space; will unblock when buffer space is available or
		// after Cancel.
		Send(item []byte) error
		// Close indicates to the server that no more items will be sent; server
		// Recv calls will receive io.EOF after all sent items.  This is an optional
		// call - e.g. a client might call Close if it needs to continue receiving
		// items from the server after it's done sending.  Returns errors
		// encountered while closing, or if Close is called after Cancel.  Like
		// Send, blocks if there is no buffer space available.
		Close() error
	}
}

// BinaryUploadCall represents the call returned from Binary.Upload.
type BinaryUploadCall interface {
	BinaryUploadClientStream
	// Finish performs the equivalent of SendStream().Close, then blocks until
	// the server is done, and returns the positional return values for the call.
	//
	// Finish returns immediately if Cancel has been called; depending on the
	// timing the output could either be an error signaling cancelation, or the
	// valid positional return values from the server.
	//
	// Calling Finish is mandatory for releasing stream resources, unless Cancel
	// has been called or any of the other methods return an error.  Finish should
	// be called at most once.
	Finish() error
	// Cancel cancels the RPC, notifying the server to stop processing.  It is
	// safe to call Cancel concurrently with any of the other stream methods.
	// Calling Cancel after Finish has returned is a no-op.
	Cancel()
}

type implBinaryUploadClientSend struct {
	call __ipc.Call
}

func (c *implBinaryUploadClientSend) Send(item []byte) error {
	return c.call.Send(item)
}
func (c *implBinaryUploadClientSend) Close() error {
	return c.call.CloseSend()
}

type implBinaryUploadCall struct {
	call __ipc.Call
	send implBinaryUploadClientSend
}

func (c *implBinaryUploadCall) SendStream() interface {
	Send(item []byte) error
	Close() error
} {
	return &c.send
}
func (c *implBinaryUploadCall) Finish() (err error) {
	if ierr := c.call.Finish(&err); ierr != nil {
		err = ierr
	}
	return
}
func (c *implBinaryUploadCall) Cancel() {
	c.call.Cancel()
}

// BinaryServerMethods is the interface a server writer
// implements for Binary.
//
// Binary can be used to store and retrieve veyron application
// binaries.
//
// To create a binary, clients first invoke the Create() method that
// specifies the number of parts the binary consists of. Clients then
// uploads the individual parts through the Upload() method, which
// identifies the part being uploaded. To resume an upload after a
// failure, clients invoke the UploadStatus() method, which returns a
// slice that identifies which parts are missing.
//
// To download a binary, clients first invoke Stat(), which returns
// information describing the binary, including the number of parts
// the binary consists of. Clients then download the individual parts
// through the Download() method, which identifies the part being
// downloaded. Alternatively, clients can download the binary through
// HTTP using a transient URL available through the DownloadURL()
// method.
//
// To delete the binary, clients invoke the Delete() method.
type BinaryServerMethods interface {
	// Create expresses the intent to create a binary identified by the
	// object name suffix consisting of the given number of parts. If
	// the suffix identifies a binary that has already been created, the
	// method returns an error.
	Create(ctx __ipc.ServerContext, nparts int32) error
	// Delete deletes the binary identified by the object name
	// suffix. If the binary that has not been created, the method
	// returns an error.
	Delete(__ipc.ServerContext) error
	// Download opens a stream that can used for downloading the given
	// part of the binary identified by the object name suffix. If the
	// binary part has not been uploaded, the method returns an
	// error. If the Delete() method is invoked when the Download()
	// method is in progress, the outcome the Download() method is
	// undefined.
	Download(ctx BinaryDownloadContext, part int32) error
	// DownloadURL returns a transient URL from which the binary
	// identified by the object name suffix can be downloaded using the
	// HTTP protocol. If not all parts of the binary have been uploaded,
	// the method returns an error.
	DownloadURL(__ipc.ServerContext) (URL string, TTL int64, err error)
	// Stat returns information describing the parts of the binary
	// identified by the object name suffix. If the binary has not been
	// created, the method returns an error.
	Stat(__ipc.ServerContext) ([]binary.PartInfo, error)
	// Upload opens a stream that can be used for uploading the given
	// part of the binary identified by the object name suffix. If the
	// binary has not been created, the method returns an error. If the
	// binary part has been uploaded, the method returns an error. If
	// the same binary part is being uploaded by another caller, the
	// method returns an error.
	Upload(ctx BinaryUploadContext, part int32) error
}

// BinaryServerStubMethods is the server interface containing
// Binary methods, as expected by ipc.Server.  The difference between
// this interface and BinaryServerMethods is that the first context
// argument for each method is always ipc.ServerCall here, while it is either
// ipc.ServerContext or a typed streaming context there.
type BinaryServerStubMethods interface {
	// Create expresses the intent to create a binary identified by the
	// object name suffix consisting of the given number of parts. If
	// the suffix identifies a binary that has already been created, the
	// method returns an error.
	Create(call __ipc.ServerCall, nparts int32) error
	// Delete deletes the binary identified by the object name
	// suffix. If the binary that has not been created, the method
	// returns an error.
	Delete(__ipc.ServerCall) error
	// Download opens a stream that can used for downloading the given
	// part of the binary identified by the object name suffix. If the
	// binary part has not been uploaded, the method returns an
	// error. If the Delete() method is invoked when the Download()
	// method is in progress, the outcome the Download() method is
	// undefined.
	Download(call __ipc.ServerCall, part int32) error
	// DownloadURL returns a transient URL from which the binary
	// identified by the object name suffix can be downloaded using the
	// HTTP protocol. If not all parts of the binary have been uploaded,
	// the method returns an error.
	DownloadURL(__ipc.ServerCall) (URL string, TTL int64, err error)
	// Stat returns information describing the parts of the binary
	// identified by the object name suffix. If the binary has not been
	// created, the method returns an error.
	Stat(__ipc.ServerCall) ([]binary.PartInfo, error)
	// Upload opens a stream that can be used for uploading the given
	// part of the binary identified by the object name suffix. If the
	// binary has not been created, the method returns an error. If the
	// binary part has been uploaded, the method returns an error. If
	// the same binary part is being uploaded by another caller, the
	// method returns an error.
	Upload(call __ipc.ServerCall, part int32) error
}

// BinaryServerStub adds universal methods to BinaryServerStubMethods.
type BinaryServerStub interface {
	BinaryServerStubMethods
	// GetMethodTags will be replaced with DescribeInterfaces.
	GetMethodTags(call __ipc.ServerCall, method string) ([]interface{}, error)
	// Signature will be replaced with DescribeInterfaces.
	Signature(call __ipc.ServerCall) (__ipc.ServiceSignature, error)
}

// BinaryServer returns a server stub for Binary.
// It converts an implementation of BinaryServerMethods into
// an object that may be used by ipc.Server.
func BinaryServer(impl BinaryServerMethods) BinaryServerStub {
	stub := implBinaryServerStub{
		impl: impl,
	}
	// Initialize GlobState; always check the stub itself first, to handle the
	// case where the user has the Glob method defined in their VDL source.
	if gs := __ipc.NewGlobState(stub); gs != nil {
		stub.gs = gs
	} else if gs := __ipc.NewGlobState(impl); gs != nil {
		stub.gs = gs
	}
	return stub
}

type implBinaryServerStub struct {
	impl BinaryServerMethods
	gs   *__ipc.GlobState
}

func (s implBinaryServerStub) Create(call __ipc.ServerCall, i0 int32) error {
	return s.impl.Create(call, i0)
}

func (s implBinaryServerStub) Delete(call __ipc.ServerCall) error {
	return s.impl.Delete(call)
}

func (s implBinaryServerStub) Download(call __ipc.ServerCall, i0 int32) error {
	ctx := &implBinaryDownloadContext{call, implBinaryDownloadServerSend{call}}
	return s.impl.Download(ctx, i0)
}

func (s implBinaryServerStub) DownloadURL(call __ipc.ServerCall) (string, int64, error) {
	return s.impl.DownloadURL(call)
}

func (s implBinaryServerStub) Stat(call __ipc.ServerCall) ([]binary.PartInfo, error) {
	return s.impl.Stat(call)
}

func (s implBinaryServerStub) Upload(call __ipc.ServerCall, i0 int32) error {
	ctx := &implBinaryUploadContext{call, implBinaryUploadServerRecv{call: call}}
	return s.impl.Upload(ctx, i0)
}

func (s implBinaryServerStub) VGlob() *__ipc.GlobState {
	return s.gs
}

func (s implBinaryServerStub) GetMethodTags(call __ipc.ServerCall, method string) ([]interface{}, error) {
	// TODO(toddw): Replace with new DescribeInterfaces implementation.
	switch method {
	case "Create":
		return []interface{}{security.Label(4)}, nil
	case "Delete":
		return []interface{}{security.Label(4)}, nil
	case "Download":
		return []interface{}{security.Label(2)}, nil
	case "DownloadURL":
		return []interface{}{security.Label(2)}, nil
	case "Stat":
		return []interface{}{security.Label(2)}, nil
	case "Upload":
		return []interface{}{security.Label(4)}, nil
	default:
		return nil, nil
	}
}

func (s implBinaryServerStub) Signature(call __ipc.ServerCall) (__ipc.ServiceSignature, error) {
	// TODO(toddw) Replace with new DescribeInterfaces implementation.
	result := __ipc.ServiceSignature{Methods: make(map[string]__ipc.MethodSignature)}
	result.Methods["Create"] = __ipc.MethodSignature{
		InArgs: []__ipc.MethodArgument{
			{Name: "nparts", Type: 36},
		},
		OutArgs: []__ipc.MethodArgument{
			{Name: "", Type: 65},
		},
	}
	result.Methods["Delete"] = __ipc.MethodSignature{
		InArgs: []__ipc.MethodArgument{},
		OutArgs: []__ipc.MethodArgument{
			{Name: "", Type: 65},
		},
	}
	result.Methods["Download"] = __ipc.MethodSignature{
		InArgs: []__ipc.MethodArgument{
			{Name: "part", Type: 36},
		},
		OutArgs: []__ipc.MethodArgument{
			{Name: "", Type: 65},
		},

		OutStream: 67,
	}
	result.Methods["DownloadURL"] = __ipc.MethodSignature{
		InArgs: []__ipc.MethodArgument{},
		OutArgs: []__ipc.MethodArgument{
			{Name: "URL", Type: 3},
			{Name: "TTL", Type: 37},
			{Name: "err", Type: 65},
		},
	}
	result.Methods["Stat"] = __ipc.MethodSignature{
		InArgs: []__ipc.MethodArgument{},
		OutArgs: []__ipc.MethodArgument{
			{Name: "", Type: 69},
			{Name: "", Type: 65},
		},
	}
	result.Methods["Upload"] = __ipc.MethodSignature{
		InArgs: []__ipc.MethodArgument{
			{Name: "part", Type: 36},
		},
		OutArgs: []__ipc.MethodArgument{
			{Name: "", Type: 65},
		},
		InStream: 67,
	}

	result.TypeDefs = []__vdlutil.Any{
		__wiretype.NamedPrimitiveType{Type: 0x1, Name: "error", Tags: []string(nil)}, __wiretype.NamedPrimitiveType{Type: 0x32, Name: "byte", Tags: []string(nil)}, __wiretype.SliceType{Elem: 0x42, Name: "", Tags: []string(nil)}, __wiretype.StructType{
			[]__wiretype.FieldType{
				__wiretype.FieldType{Type: 0x3, Name: "Checksum"},
				__wiretype.FieldType{Type: 0x25, Name: "Size"},
			},
			"veyron.io/veyron/veyron2/services/mgmt/binary.PartInfo", []string(nil)},
		__wiretype.SliceType{Elem: 0x44, Name: "", Tags: []string(nil)}}

	return result, nil
}

// BinaryDownloadServerStream is the server stream for Binary.Download.
type BinaryDownloadServerStream interface {
	// SendStream returns the send side of the server stream.
	SendStream() interface {
		// Send places the item onto the output stream.  Returns errors encountered
		// while sending.  Blocks if there is no buffer space; will unblock when
		// buffer space is available.
		Send(item []byte) error
	}
}

// BinaryDownloadContext represents the context passed to Binary.Download.
type BinaryDownloadContext interface {
	__ipc.ServerContext
	BinaryDownloadServerStream
}

type implBinaryDownloadServerSend struct {
	call __ipc.ServerCall
}

func (s *implBinaryDownloadServerSend) Send(item []byte) error {
	return s.call.Send(item)
}

type implBinaryDownloadContext struct {
	__ipc.ServerContext
	send implBinaryDownloadServerSend
}

func (s *implBinaryDownloadContext) SendStream() interface {
	Send(item []byte) error
} {
	return &s.send
}

// BinaryUploadServerStream is the server stream for Binary.Upload.
type BinaryUploadServerStream interface {
	// RecvStream returns the receiver side of the server stream.
	RecvStream() interface {
		// Advance stages an item so that it may be retrieved via Value.  Returns
		// true iff there is an item to retrieve.  Advance must be called before
		// Value is called.  May block if an item is not available.
		Advance() bool
		// Value returns the item that was staged by Advance.  May panic if Advance
		// returned false or was not called.  Never blocks.
		Value() []byte
		// Err returns any error encountered by Advance.  Never blocks.
		Err() error
	}
}

// BinaryUploadContext represents the context passed to Binary.Upload.
type BinaryUploadContext interface {
	__ipc.ServerContext
	BinaryUploadServerStream
}

type implBinaryUploadServerRecv struct {
	call __ipc.ServerCall
	val  []byte
	err  error
}

func (s *implBinaryUploadServerRecv) Advance() bool {
	s.err = s.call.Recv(&s.val)
	return s.err == nil
}
func (s *implBinaryUploadServerRecv) Value() []byte {
	return s.val
}
func (s *implBinaryUploadServerRecv) Err() error {
	if s.err == __io.EOF {
		return nil
	}
	return s.err
}

type implBinaryUploadContext struct {
	__ipc.ServerContext
	recv implBinaryUploadServerRecv
}

func (s *implBinaryUploadContext) RecvStream() interface {
	Advance() bool
	Value() []byte
	Err() error
} {
	return &s.recv
}

// ProfileClientMethods is the client interface
// containing Profile methods.
//
// Profile abstracts a device's ability to run binaries, and hides
// specifics such as the operating system, hardware architecture, and
// the set of installed libraries. Profiles describe binaries and
// devices, and are used to match them.
type ProfileClientMethods interface {
	// Label is the human-readable profile key for the profile,
	// e.g. "linux-media". The label can be used to uniquely identify
	// the profile (for the purpose of matching application binaries and
	// nodes).
	Label(__context.T, ...__ipc.CallOpt) (string, error)
	// Description is a free-text description of the profile, meant for
	// human consumption.
	Description(__context.T, ...__ipc.CallOpt) (string, error)
}

// ProfileClientStub adds universal methods to ProfileClientMethods.
type ProfileClientStub interface {
	ProfileClientMethods
	__ipc.UniversalServiceMethods
}

// ProfileClient returns a client stub for Profile.
func ProfileClient(name string, opts ...__ipc.BindOpt) ProfileClientStub {
	var client __ipc.Client
	for _, opt := range opts {
		if clientOpt, ok := opt.(__ipc.Client); ok {
			client = clientOpt
		}
	}
	return implProfileClientStub{name, client}
}

type implProfileClientStub struct {
	name   string
	client __ipc.Client
}

func (c implProfileClientStub) c(ctx __context.T) __ipc.Client {
	if c.client != nil {
		return c.client
	}
	return __veyron2.RuntimeFromContext(ctx).Client()
}

func (c implProfileClientStub) Label(ctx __context.T, opts ...__ipc.CallOpt) (o0 string, err error) {
	var call __ipc.Call
	if call, err = c.c(ctx).StartCall(ctx, c.name, "Label", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&o0, &err); ierr != nil {
		err = ierr
	}
	return
}

func (c implProfileClientStub) Description(ctx __context.T, opts ...__ipc.CallOpt) (o0 string, err error) {
	var call __ipc.Call
	if call, err = c.c(ctx).StartCall(ctx, c.name, "Description", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&o0, &err); ierr != nil {
		err = ierr
	}
	return
}

func (c implProfileClientStub) Signature(ctx __context.T, opts ...__ipc.CallOpt) (o0 __ipc.ServiceSignature, err error) {
	var call __ipc.Call
	if call, err = c.c(ctx).StartCall(ctx, c.name, "Signature", nil, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&o0, &err); ierr != nil {
		err = ierr
	}
	return
}

func (c implProfileClientStub) GetMethodTags(ctx __context.T, method string, opts ...__ipc.CallOpt) (o0 []interface{}, err error) {
	var call __ipc.Call
	if call, err = c.c(ctx).StartCall(ctx, c.name, "GetMethodTags", []interface{}{method}, opts...); err != nil {
		return
	}
	if ierr := call.Finish(&o0, &err); ierr != nil {
		err = ierr
	}
	return
}

// ProfileServerMethods is the interface a server writer
// implements for Profile.
//
// Profile abstracts a device's ability to run binaries, and hides
// specifics such as the operating system, hardware architecture, and
// the set of installed libraries. Profiles describe binaries and
// devices, and are used to match them.
type ProfileServerMethods interface {
	// Label is the human-readable profile key for the profile,
	// e.g. "linux-media". The label can be used to uniquely identify
	// the profile (for the purpose of matching application binaries and
	// nodes).
	Label(__ipc.ServerContext) (string, error)
	// Description is a free-text description of the profile, meant for
	// human consumption.
	Description(__ipc.ServerContext) (string, error)
}

// ProfileServerStubMethods is the server interface containing
// Profile methods, as expected by ipc.Server.  The difference between
// this interface and ProfileServerMethods is that the first context
// argument for each method is always ipc.ServerCall here, while it is either
// ipc.ServerContext or a typed streaming context there.
type ProfileServerStubMethods interface {
	// Label is the human-readable profile key for the profile,
	// e.g. "linux-media". The label can be used to uniquely identify
	// the profile (for the purpose of matching application binaries and
	// nodes).
	Label(__ipc.ServerCall) (string, error)
	// Description is a free-text description of the profile, meant for
	// human consumption.
	Description(__ipc.ServerCall) (string, error)
}

// ProfileServerStub adds universal methods to ProfileServerStubMethods.
type ProfileServerStub interface {
	ProfileServerStubMethods
	// GetMethodTags will be replaced with DescribeInterfaces.
	GetMethodTags(call __ipc.ServerCall, method string) ([]interface{}, error)
	// Signature will be replaced with DescribeInterfaces.
	Signature(call __ipc.ServerCall) (__ipc.ServiceSignature, error)
}

// ProfileServer returns a server stub for Profile.
// It converts an implementation of ProfileServerMethods into
// an object that may be used by ipc.Server.
func ProfileServer(impl ProfileServerMethods) ProfileServerStub {
	stub := implProfileServerStub{
		impl: impl,
	}
	// Initialize GlobState; always check the stub itself first, to handle the
	// case where the user has the Glob method defined in their VDL source.
	if gs := __ipc.NewGlobState(stub); gs != nil {
		stub.gs = gs
	} else if gs := __ipc.NewGlobState(impl); gs != nil {
		stub.gs = gs
	}
	return stub
}

type implProfileServerStub struct {
	impl ProfileServerMethods
	gs   *__ipc.GlobState
}

func (s implProfileServerStub) Label(call __ipc.ServerCall) (string, error) {
	return s.impl.Label(call)
}

func (s implProfileServerStub) Description(call __ipc.ServerCall) (string, error) {
	return s.impl.Description(call)
}

func (s implProfileServerStub) VGlob() *__ipc.GlobState {
	return s.gs
}

func (s implProfileServerStub) GetMethodTags(call __ipc.ServerCall, method string) ([]interface{}, error) {
	// TODO(toddw): Replace with new DescribeInterfaces implementation.
	switch method {
	case "Label":
		return []interface{}{security.Label(2)}, nil
	case "Description":
		return []interface{}{security.Label(2)}, nil
	default:
		return nil, nil
	}
}

func (s implProfileServerStub) Signature(call __ipc.ServerCall) (__ipc.ServiceSignature, error) {
	// TODO(toddw) Replace with new DescribeInterfaces implementation.
	result := __ipc.ServiceSignature{Methods: make(map[string]__ipc.MethodSignature)}
	result.Methods["Description"] = __ipc.MethodSignature{
		InArgs: []__ipc.MethodArgument{},
		OutArgs: []__ipc.MethodArgument{
			{Name: "", Type: 3},
			{Name: "", Type: 65},
		},
	}
	result.Methods["Label"] = __ipc.MethodSignature{
		InArgs: []__ipc.MethodArgument{},
		OutArgs: []__ipc.MethodArgument{
			{Name: "", Type: 3},
			{Name: "", Type: 65},
		},
	}

	result.TypeDefs = []__vdlutil.Any{
		__wiretype.NamedPrimitiveType{Type: 0x1, Name: "error", Tags: []string(nil)}}

	return result, nil
}
