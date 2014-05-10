// Package r provides access to a global instance of the runtime.
// If all you need is a reference to the already initialized runtime instance,
// and don't want to pull in other dependencies like the implementation of
// naming.MountTable, depend on this package rather than veyron2/rt.
package r

import (
	"fmt"
	"os"

	"veyron/runtimes/google/rt"

	"veyron2"
)

// R returns the initialized global instance of the runtime.
var R func() veyron2.Runtime

func init() {
	runtime := os.Getenv("VEYRON_RUNTIME")
	switch runtime {
	case "google", "":
		R = rt.R
	default:
		panic(fmt.Sprintf("unsupported runtime: %q\n", runtime))
	}
}
