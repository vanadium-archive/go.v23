package verror_test

import (
	"errors"
	"fmt"

	"v.io/veyron/veyron2/verror"
)

func Example_usage() {
	// Create two errors with id A.
	idA := verror.ID("A")
	a1 := verror.Make(idA, "msg 1")
	a2 := verror.Makef(idA, "msg %d", 2)
	// Create an error with id B.
	idB := verror.ID("B")
	b3 := verror.Make(idB, "msg 3")
	// Convert a regular error into an error with unknown id.
	e4 := verror.Convert(errors.New("msg 4"))
	// Use the Is and IsUnknown functions to check errors.
	for _, err := range []verror.E{b3, a1, e4, a2} {
		switch {
		case verror.Is(err, idA):
			fmt.Printf("A error received: %s\n", err)
		case verror.Is(err, idB):
			fmt.Printf("B error received: %s\n", err)
		case verror.IsUnknown(err):
			fmt.Printf("Unknown error received: %s\n", err)
		}
	}

	// Output:
	// B error received: msg 3
	// A error received: msg 1
	// Unknown error received: msg 4
	// A error received: msg 2
}
