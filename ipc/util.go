package ipc

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
	return ReflectInvoker(&obj{children})
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
