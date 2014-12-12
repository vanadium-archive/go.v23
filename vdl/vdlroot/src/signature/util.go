package signature

// SortableMethods implements sort.Interface, ordering by method name.
type SortableMethods []Method

func (s SortableMethods) Len() int           { return len(s) }
func (s SortableMethods) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s SortableMethods) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// TODO(bposnitz) The binary search version of this below should work, but the methods are currently not always sorted.
// Look into what to do about this.
func (s *Interface) FindMethod(methodName string) (Method, bool) {
	for _, method := range s.Methods {
		if method.Name == methodName {
			return method, true
		}
	}
	return Method{}, false
}

/*
// FindMethod returns the signature of the given method and true iff the method
// exists, otherwise returns an empty signature and false.
func (s *Interface) FindMethod(method string) (Method, bool) {
	// We're guaranteed the methods are ordered by name, so do binary search.
	f := func(i int) bool { return s.Methods[i].Name >= method }
	ix := sort.Search(len(s.Methods), f)
	if ix < len(s.Methods) && s.Methods[ix].Name == method {
		return s.Methods[ix], true
	}
	return Method{}, false
}
*/

// FirstMethod returns the signature of the given method and true iff the method
// exists, otherwise returns an empty signature and false.  If the method exists
// in more than one interface, we return the signature from the the first
// interface with the given method.
func FirstMethod(sig []Interface, method string) (Method, bool) {
	for _, s := range sig {
		if msig, ok := s.FindMethod(method); ok {
			return msig, true
		}
	}
	return Method{}, false
}
