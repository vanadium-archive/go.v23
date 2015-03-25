// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"fmt"
	"net"
	"sync"
	"time"

	"v.io/v23/naming"
)

// DialerFunc is the function used to create net.Conn objects given a
// protocol-specific string representation of an address.
type DialerFunc func(protocol, address string, timeout time.Duration) (net.Conn, error)

// ListenerFunc is the function used to create net.Listener objects given a
// protocol-specific string representation of the address a server will listen on.
type ListenerFunc func(protocol, address string) (net.Listener, error)

// RegisterProtocol makes available a Dialer and a Listener to RegisteredNetwork.
// If the protocol represents other actual protocols, you need to specify all the
// actual protocols. E.g, "wsh" represents "tcp4", "tcp6", "ws4", and "ws6".
//
// Implementations of the Manager interface are expected to use this registry
// in order to expand the reach of the types of network protocols they can
// handle.
//
// Successive calls to RegisterProtocol replace the contents of a previous
// call to it and returns trues if a previous value was replaced, false otherwise.
func RegisterProtocol(protocol string, dialer DialerFunc, listener ListenerFunc, p ...string) bool {
	// This is for handling the common case where protocol is a "singleton", to
	// make it easier to specify.
	if len(p) == 0 {
		p = []string{protocol}
	}
	registryLock.Lock()
	defer registryLock.Unlock()
	_, present := registry[protocol]
	registry[protocol] = registryEntry{dialer, listener, p}
	return present
}

// RegisterUnknownProtocol registers a Dialer and a Listener for endpoints with
// no specified protocol.
//
// The desired protocol provided in the first argument will be passed to the
// Dialer and Listener as the actual protocol to use when dialing or listening.
//
// The protocol itself must have already been registered before RegisterUnknownProtocol
// is called, otherwise we'll panic.
func RegisterUnknownProtocol(protocol string, dialer DialerFunc, listener ListenerFunc) bool {
	var p []string
	registryLock.RLock()
	r, present := registry[protocol]
	if !present {
		panic(fmt.Sprintf("%s not registered", protocol))
	}
	p = r.p
	registryLock.RUnlock()
	wrappedDialer := func(_, address string, timeout time.Duration) (net.Conn, error) {
		return dialer(protocol, address, timeout)
	}
	wrappedListener := func(_, address string) (net.Listener, error) {
		return listener(protocol, address)
	}
	return RegisterProtocol(naming.UnknownProtocol, wrappedDialer, wrappedListener, p...)
}

// RegisteredProtocol returns the Dialer and Listener registered with a
// previous call to RegisterProtocol.
func RegisteredProtocol(protocol string) (DialerFunc, ListenerFunc, []string) {
	registryLock.RLock()
	e := registry[protocol]
	registryLock.RUnlock()
	return e.d, e.l, e.p
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
	p []string
}

var (
	registryLock sync.RWMutex
	registry     = make(map[string]registryEntry)
)
