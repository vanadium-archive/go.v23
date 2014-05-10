package idl

// TagGetter is an interface that enables getting tags specified in the IDL.
type TagGetter interface {
	// GetMethodTags returns the tags associated with the given method.
	GetMethodTags(method string) []interface{}
}
