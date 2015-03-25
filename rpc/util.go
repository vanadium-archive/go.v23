// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"v.io/v23/context"
	"v.io/v23/verror"
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
	return ReflectInvokerOrDie(&obj{children})
}

type obj struct {
	children []string
}

func (o obj) GlobChildren__(ServerCall) (<-chan string, error) {
	ch := make(chan string, len(o.children))
	for _, v := range o.children {
		ch <- v
	}
	close(ch)
	return ch, nil
}

var (
	unknownMethod = verror.Register("v.io/v23/rpc.unknownMethod", verror.NoRetry, "{1:}{2:} unknown method {3}")
	unknownSuffix = verror.Register("v.io/v23/rpc.unknownSuffix", verror.NoRetry, "{1:}{2:} unknown object with suffix: {3}")
)

// NewErrUnknownMethod returns an unknown method error.
func NewErrUnknownMethod(ctx *context.T, method string) error {
	return verror.New(verror.ErrNoExist, ctx, verror.New(unknownMethod, ctx, method))
}

// NewErrUnknownSuffix returns an unknown suffix error.
func NewErrUnknownSuffix(ctx *context.T, suffix string) error {
	return verror.New(verror.ErrNoExist, ctx, verror.New(unknownSuffix, ctx, suffix))
}
