package stream

import (
	"net"
	"sync"
)

// DialerFunc is the function used to create net.Conn objects given a
// protocol-specific string representation of an address.
type DialerFunc func(address string) (net.Conn, error)

// ListenerFunc is the function used to create net.Listener objects given a
// protocol-specific string representation of the address a server will listen on.
type ListenerFunc func(address string) (net.Listener, error)

// RegisterProtocol makes available a Dialer and a Listener to
// RegisteredNetwork.
//
// Implementations of the Manager interface are expected to use this registry
// in order to expand the reach of the types of network protocols they can
// handle.
//
// Successive calls to RegisterProtocol replace the contents of a previous
// call to it and returns trues if a previous value was replaced, false otherwise.
func RegisterProtocol(protocol string, dialer DialerFunc, listener ListenerFunc) bool {
	registryLock.Lock()
	defer registryLock.Unlock()
	_, present := registry[protocol]
	registry[protocol] = registryEntry{dialer, listener}
	return present
}

// RegisteredProtocol returns the Dialer and Listener registered with a
// previous call to RegisterProtocol.
//
// If the Dialer is nil, the client is expected to use net.Dial instead and
// if Listener is nil, the client is expected to use net.Listen instead.
func RegisteredProtocol(protocol string) (DialerFunc, ListenerFunc) {
	registryLock.RLock()
	e := registry[protocol]
	registryLock.RUnlock()
	return e.d, e.l
}

type registryEntry struct {
	d DialerFunc
	l ListenerFunc
}

var (
	registryLock sync.RWMutex
	registry     = make(map[string]registryEntry)
)
