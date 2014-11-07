package ipc

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
