// Package rt provides initialization of a specific instantiation of the
// Veyron2 runtime.
package rt

import (
	"fmt"
	"os"

	grt "veyron/runtimes/google/rt"

	"veyron2"
)

type publicFactory func(opts ...veyron2.ROpt) (veyron2.Runtime, error)
type publicInit func(opts ...veyron2.ROpt) veyron2.Runtime

var (
	// Init returns the initialized global instance of the runtime.
	// Calling it multiple times will always return the result of the
	// first call to Init (ignoring subsequently provided options).
	// All Veyron apps should call Init as the first call in their main
	// function, it will call flag.Parse internally. It will panic on
	// encountering an error.
	Init publicInit

	// New creates and initializes a new instance of the runtime. It should
	// be used in unit tests and any situation where a single global runtime
	// instance is inappropriate.
	New publicFactory

	// R returns the initialized global instance of the runtime.
	R func() veyron2.Runtime
)

func init() {
	runtime := os.Getenv("VEYRON_RUNTIME")
	switch runtime {
	case "google", "":
		Init = func(opts ...veyron2.ROpt) veyron2.Runtime {
			r, err := grt.Init(opts...)
			if err != nil {
				panic(fmt.Sprintf("Init failed: %v", err))
			}
			return r
		}
		New = func(opts ...veyron2.ROpt) (veyron2.Runtime, error) { return grt.New(opts...) }
		R = grt.R
	default:
		panic(fmt.Sprintf("unsupported runtime: %q\n", runtime))
	}
}
