package ipc_test

import (
	"veyron2/ipc"
	"veyron2/naming"
	"veyron2/security"
)

type TestRuntime interface {
	ipc.Runtime
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

func (*rtImpl) Client() ipc.Client {
	return nil
}

func (*rtImpl) NewServer(opts ...ipc.ServerOpt) (ipc.Server, error) {
	return &server{}, nil
}

func (*rtImpl) RegisterType(interface{}) {
}

type stream struct{}

func (*stream) Send(item interface{}) error {
	return nil
}

func (*stream) Recv(itemptr interface{}) error {
	return nil
}

type call struct {
	stream
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

func (*client) StartCall(name, method string, args []interface{}, opts ...ipc.ClientCallOpt) (ipc.ClientCall, error) {
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
