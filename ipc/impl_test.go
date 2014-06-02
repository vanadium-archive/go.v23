package ipc_test

import (
	"veyron2"
	"veyron2/ipc"
	"veyron2/ipc/stream"
	"veyron2/naming"
	"veyron2/product"
	"veyron2/security"
	"veyron2/vlog"
)

type TestRuntime interface {
	veyron2.Runtime
}

var (
	RT TestRuntime
)

func init() {
	RT = &rtImpl{}
}

type rtImpl struct{}

func (*rtImpl) NewClient(opts ...ipc.ClientOpt) (ipc.Client, error) {
	return &client{}, nil
}

func (*rtImpl) Client() ipc.Client { return nil }

func (*rtImpl) Product() product.T { return nil }
func (*rtImpl) NewIdentity(name string) (security.PrivateID, error) {
	return nil, nil
}

func (*rtImpl) Identity() security.PrivateID { return nil }

func (*rtImpl) NewServer(opts ...ipc.ServerOpt) (ipc.Server, error) {
	return &server{}, nil
}

func (*rtImpl) NewStreamManager(opts ...stream.ManagerOpt) (stream.Manager, error) {
	return nil, nil
}

func (*rtImpl) StreamManager() stream.Manager { return nil }

func (*rtImpl) NewEndpoint(ep string) (naming.Endpoint, error) {
	return nil, nil
}

func (*rtImpl) MountTable() naming.MountTable { return nil }

func (*rtImpl) Logger() vlog.Logger { return nil }

func (*rtImpl) NewLogger(name string, opts ...vlog.LoggingOpts) (vlog.Logger, error) {
	return nil, nil
}

func (*rtImpl) Stop() {}

func (*rtImpl) ForceStop() {}

func (*rtImpl) WaitForStop(chan<- string) {}

func (*rtImpl) Shutdown() {}

func (*rtImpl) RegisterType(interface{}) {
}

func (*rtImpl) NewContext() ipc.Context {
	return nil
}

func (*rtImpl) TODOContext() ipc.Context {
	return nil
}

type istream struct{}

func (*istream) Send(item interface{}) error {
	return nil
}

func (*istream) Recv(itemptr interface{}) error {
	return nil
}

type call struct {
	istream
}

func (*call) Cancel() {
}

func (*call) CloseSend() error {
	return nil
}

func (*call) Finish(results ...interface{}) error {
	return nil
}

type client struct{}

// IPCBindOpt makes client implement BindOpt.
func (c *client) IPCBindOpt() {}

func (*client) StartCall(ctx ipc.Context, name, method string, args []interface{}, opts ...ipc.CallOpt) (ipc.Call, error) {
	return &call{}, nil
}

func (*client) Close() {
}

type server struct{}

func (*server) Listen(protocol, address string) (naming.Endpoint, error) {
	return nil, nil
}

func (*server) Register(prefix string, disp ipc.Dispatcher) error {
	return nil
}

func (*server) Publish(mountpoint string) error {
	return nil
}

func (*server) Published() ([]string, error) {
	return nil, nil
}

func (*server) Stop() error {
	return nil
}

type context struct{}

func (*context) Send(item interface{}) error {
	return nil
}

func (*context) Recv(itemptr interface{}) error {
	return nil
}

func (*context) Cancelled() bool {
	return false
}

func (*context) RemoteID() security.PublicID {
	return security.FakePublicID("remote")
}
