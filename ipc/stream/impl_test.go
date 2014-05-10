package stream_test

import (
	"fmt"
	"net"
	"time"

	"veyron2/ipc/stream"
	"veyron2/naming"
	"veyron2/security"
)

type TestRuntime interface {
	stream.Runtime
	naming.Runtime
}

var RT TestRuntime

func init() {
	RT = &rt{}
}

type rt struct{}

func (*rt) NewStreamManager(opts ...stream.ManagerOpt) (stream.Manager, error) { return &manager{}, nil }
func (*rt) StreamManager() stream.Manager                                      { return nil }

func (*rt) NewEndpoint(str string) (naming.Endpoint, error) { return &endpoint{}, nil }
func (*rt) NewEndpointFromNetworkAddress(n, a string) (naming.Endpoint, error) {
	return &endpoint{}, nil
}
func (*rt) NewMountTable() (naming.MountTable, error) { return nil, nil }
func (*rt) MountTable() naming.MountTable             { return nil }

// Implement Manager
type manager struct{}

func (*manager) Listen(protocol, address string, opts ...stream.ListenerOpt) (stream.Listener, naming.Endpoint, error) {
	return &listener{}, &endpoint{}, nil
}

func (*manager) Dial(remote naming.Endpoint, opts ...stream.VCOpt) (stream.VC, error) {
	return &vc{}, nil
}

func (*manager) ShutdownEndpoint(ep naming.Endpoint) {}

func (*manager) Shutdown() {}

// Implement VC
type vc struct{}

func (*vc) Connect(opts ...stream.FlowOpt) (stream.Flow, error) { return &flow{}, nil }
func (*vc) Listen() (stream.Listener, error)                    { return &listener{}, nil }

// Implement Listener
type listener struct{}

func (*listener) Accept() (stream.Flow, error) { return &flow{}, nil }
func (*listener) Close() error                 { return nil }

// Implement Flow
type flow struct{}

func (*flow) Read(p []byte) (n int, err error)   { return 0, nil }
func (*flow) Write(p []byte) (n int, err error)  { return 0, nil }
func (*flow) Close() error                       { return nil }
func (*flow) IsClosed() bool                     { return false }
func (*flow) Closed() <-chan struct{}            { return nil }
func (*flow) Cancel()                            {}
func (*flow) LocalAddr() net.Addr                { return &endpoint{} }
func (*flow) RemoteAddr() net.Addr               { return &endpoint{} }
func (*flow) SetDeadline(t time.Time) error      { return nil }
func (*flow) SetReadDeadline(t time.Time) error  { return nil }
func (*flow) SetWriteDeadline(t time.Time) error { return nil }
func (*flow) LocalID() security.PublicID         { return security.FakePublicID("test") }
func (*flow) RemoteID() security.PublicID        { return security.FakePublicID("test") }

// endpoint implements Endpoint
type endpoint struct {
	rid  naming.RoutingID
	addr string
}

func (ep *endpoint) RoutingID() naming.RoutingID { return ep.rid }
func (ep *endpoint) String() string              { return ep.addr + fmt.Sprintf(",%d", ep.rid) }
func (*endpoint) Network() string                { return "tcp" }
func (ep *endpoint) Addr() net.Addr              { return ep }
