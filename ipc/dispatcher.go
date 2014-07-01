package ipc

import (
	"veyron2/security"
	"veyron2/verror"
)

// SoloDispatcher returns a dispatcher for a single object obj, using
// ReflectInvoker to invoke methods.  Lookup only succeeds on the empty suffix.
// The provided auth is returned for successful lookups.
func SoloDispatcher(obj interface{}, auth security.Authorizer) Dispatcher {
	return &soloDispatcher{ReflectInvoker(obj), auth}
}

type soloDispatcher struct {
	invoker Invoker
	auth    security.Authorizer
}

func (d soloDispatcher) Lookup(suffix string) (Invoker, security.Authorizer, error) {
	if suffix != "" {
		return nil, nil, verror.NotFoundf("ipc: SoloDispatcher lookup on non-empty suffix: " + suffix)
	}
	return d.invoker, d.auth, nil
}
