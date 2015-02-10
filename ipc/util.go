package ipc

import (
	"v.io/core/veyron2/context"
	"v.io/core/veyron2/verror2"
)

// NewGlobState returns the GlobState corresponding to obj.  Returns nil if obj
// doesn't implement AllGlobber or ChildrenGlobber.
func NewGlobState(obj interface{}) *GlobState {
	a, ok1 := obj.(AllGlobber)
	c, ok2 := obj.(ChildrenGlobber)
	if ok1 || ok2 {
		return &GlobState{AllGlobber: a, ChildrenGlobber: c}
	}
	return nil
}

// ChildrenGlobberInvoker returns an Invoker for an object that implements the
// ChildrenGlobber interface, and nothing else.
func ChildrenGlobberInvoker(children ...string) Invoker {
	return ReflectInvokerOrDie(&obj{children})
}

type obj struct {
	children []string
}

func (o obj) GlobChildren__(ServerContext) (<-chan string, error) {
	ch := make(chan string, len(o.children))
	for _, v := range o.children {
		ch <- v
	}
	close(ch)
	return ch, nil
}

var (
	unknownMethod      = verror2.Register("v.io/core/veyron2/ipc.unknownMethod", verror2.NoRetry, "{1:}{2:} unknown method {3}")
	unknownSuffix      = verror2.Register("v.io/core/veyron2/ipc.unknownSuffix", verror2.NoRetry, "{1:}{2:} unknown object with suffix: {3}")
	globNotImplemented = verror2.Register("v.io/core/veyron2/ipc.globNotImplemented", verror2.NoRetry, "{1:}{2:} Glob not implemented by suffix: {3}")
)

// MakeUnknownMethod returns an unknown method error.
func MakeUnknownMethod(ctx *context.T, method string) error {
	return verror2.Make(verror2.NoExist, ctx, verror2.Make(unknownMethod, ctx, method))
}

// MakeUnknownSuffix returns an unknown suffix error.
func MakeUnknownSuffix(ctx *context.T, suffix string) error {
	return verror2.Make(verror2.NoExist, ctx, verror2.Make(unknownSuffix, ctx, suffix))
}

// MakeGlobNotImplemented returns a glob not implemented error.
func MakeGlobNotImplemented(ctx *context.T, suffix string) error {
	return verror2.Make(verror2.NoExist, ctx, verror2.Make(globNotImplemented, ctx, suffix))
}
