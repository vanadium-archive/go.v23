package signature

import "sort"

// SortableMethods implements sort.Interface, ordering by method name.
type SortableMethods []Method

func (s SortableMethods) Len() int           { return len(s) }
func (s SortableMethods) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s SortableMethods) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

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
