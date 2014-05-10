/*
Package veyron2 defines the Runtime interface of the public Veyron API and its subdirectories define the entire Veyron public API.

These public APIs will be stable over an extended period and changes to
them will be carefully managed to ensure backward compatibility. The same
policy as used for go (http://golang.org/doc/go1compat) will be used for these
APIs.
*/
package veyron2

import (
	"veyron2/ipc"
	"veyron2/ipc/stream"
	"veyron2/mgmt"
	"veyron2/naming"
	"veyron2/product"
	"veyron2/security"
	"veyron2/vlog"
)

// Runtime is the interface that concrete veyron implementations must implement;
// it is the union of the runtime interfaces for each subcomponent.
type Runtime interface {
	ipc.Runtime
	naming.Runtime
	product.Runtime
	security.Runtime
	stream.Runtime
	vlog.Runtime
	mgmt.Runtime

	// Shutdown cleanly shuts down any internal state, logging, goroutines
	// etc spawned and managed by the runtime. It is useful for cases where
	// an application or library wants to be sure that it cleans up after
	// itself, and should typically be the last thing the program does.
	// Shutdown does not wait for any inflight requests to complete on
	// existing servers and clients in the runtime -- these need to be shut
	// down cleanly in advance if desired.
	Shutdown()
}

// The runtime must provide two package level functions, R and NewR.
// R returns the initialized global instance of the Runtime. NewR will
// create and initialiaze a new instance of the Runtime; it will typically
// be used from within unit tests.
//
// Their signatures are:
// <package>.R(opts ...NewROpt{}) (Runtime, error)
// <package>.NewR(opts ...NewROpt{}) (Runtime, error)

type ROpt interface {
	ROpt()
}
