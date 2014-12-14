package stream

import (
	"net"
	"sync"
	"time"
)

// DialerFunc is the function used to create net.Conn objects given a
// protocol-specific string representation of an address.
type DialerFunc func(protocol, address string, timeout time.Duration) (net.Conn, error)

// ListenerFunc is the function used to create net.Listener objects given a
// protocol-specific string representation of the address a server will listen on.
type ListenerFunc func(protocol, address string) (net.Listener, error)

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
func RegisteredProtocol(protocol string) (DialerFunc, ListenerFunc) {
	registryLock.RLock()
	e := registry[protocol]
	registryLock.RUnlock()
	return e.d, e.l
}

// RegisteredProtocols returns the list of protocols that have been previously
// registered using RegisterProtocol. The underlying implementation will
// support additional protocols such as those supported by the native RPC stack.
func RegisteredProtocols() []string {
	registryLock.RLock()
	defer registryLock.RUnlock()
	p := make([]string, 0, len(registry))
	for k, _ := range registry {
		p = append(p, k)
	}
	return p
}

type registryEntry struct {
	d DialerFunc
	l ListenerFunc
}

var (
	registryLock sync.RWMutex
	registry     = make(map[string]registryEntry)
)
