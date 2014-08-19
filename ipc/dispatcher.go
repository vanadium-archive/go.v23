package ipc

import (
	"veyron2/security"
	"veyron2/verror"
)

// LeafDispatcher returns a dispatcher for a single object obj, using
// ReflectInvoker to invoke methods. Lookup only succeeds on the empty suffix.
// The provided auth is returned for successful lookups.
func LeafDispatcher(obj interface{}, auth security.Authorizer) Dispatcher {
	return &leafDispatcher{ReflectInvoker(obj), auth}
}

type leafDispatcher struct {
	invoker Invoker
	auth    security.Authorizer
}

func (d leafDispatcher) Lookup(suffix, method string) (Invoker, security.Authorizer, error) {
	if suffix != "" {
		return nil, nil, verror.NotFoundf("ipc: LeafDispatcher lookup on non-empty suffix: " + suffix)
	}
	return d.invoker, d.auth, nil
}
