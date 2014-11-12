package ipc

import (
	"veyron.io/veyron/veyron2/naming"
)

// NewGlobState returns the GlobState corresponding to obj.  Returns nil if obj
// doesn't implement VAllGlobber or VChildrenGlobber.
func NewGlobState(obj interface{}) *GlobState {
	a, ok1 := obj.(VAllGlobber)
	c, ok2 := obj.(VChildrenGlobber)
	if ok1 || ok2 {
		return &GlobState{VAllGlobber: a, VChildrenGlobber: c}
	}
	return nil
}

// VChildrenGlobberInvoker returns an Invoker for an object that implements the
// VGlobChildren interface, and nothing else.
func VChildrenGlobberInvoker(children ...string) Invoker {
	return ReflectInvoker(&obj{children})
}

type obj struct {
	children []string
}

func (o obj) VGlobChildren() ([]string, error) {
	return o.children, nil
}

// GlobServerStream is the server stream for implementations of Glob.
type GlobServerStream interface {
	// SendStream returns the send side of the Glob server stream.
	SendStream() interface {
		// Send places the item onto the output stream.  Returns errors encountered
		// while sending.  Blocks if there is no buffer space; will unblock when
		// buffer space is available.
		Send(item naming.VDLMountEntry) error
	}
}

// GlobContext represents the server context passed to implementations of Glob.
// This is the interface that users implement for mounttable.Globbable.
type GlobContext interface {
	ServerContext
	GlobServerStream
}

// GlobContextStub is a wrapper that converts ipc.ServerCall into a typesafe
// stub that implements GlobContext.  This is the type used by the server stub,
// to enable ReflectInvoker to grab the send stream type.
type GlobContextStub struct {
	ServerCall
}

// Init initializes GlobContextStub from ipc.ServerCall.
func (s *GlobContextStub) Init(call ServerCall) {
	s.ServerCall = call
}

// SendStream returns the send side of the .Glob server stream.
func (s *GlobContextStub) SendStream() interface {
	Send(item naming.VDLMountEntry) error
} {
	return implGlobContextSend{s}
}

type implGlobContextSend struct {
	s *GlobContextStub
}

func (s implGlobContextSend) Send(item naming.VDLMountEntry) error {
	return s.s.ServerCall.Send(item)
}
