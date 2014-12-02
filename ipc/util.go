package ipc

import (
	"sort"
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

// OrderByMethodName implements sort.Interface, ordering by method name.
type OrderByMethodName []MethodSig

func (s OrderByMethodName) Len() int           { return len(s) }
func (s OrderByMethodName) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s OrderByMethodName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// FindMethod returns the signature of the given method and true iff the method
// exists, otherwise returns an empty signature and false.
func (s *InterfaceSig) FindMethod(method string) (MethodSig, bool) {
	// We're guaranteed the methods are ordered by name, so do binary search.
	f := func(i int) bool { return s.Methods[i].Name >= method }
	ix := sort.Search(len(s.Methods), f)
	if ix < len(s.Methods) && s.Methods[ix].Name == method {
		return s.Methods[ix], true
	}
	return MethodSig{}, false
}
