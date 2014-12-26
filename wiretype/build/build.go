package build

import (
	"reflect"

	"v.io/core/veyron2/vdl/vdlutil"
	"v.io/core/veyron2/wiretype"
)

// TypeDefs is a slice of type definitions.
type TypeDefs []vdlutil.Any

// Put looks up the TypeID for the wire type or adds a new one.
func (td TypeDefs) Put(wt vdlutil.Any) (TypeDefs, wiretype.TypeID) {
	// Check if it is a bootstrap type.
	for _, pair := range wiretype.BootstrapTypes {
		if reflect.DeepEqual(pair.WT, wt) {
			return td, pair.TID
		}
	}

	// Look for a matching type.
	for i, ewt := range td {
		// Note: DeepEqual is used because the slices in the structs are not comparable.
		if reflect.DeepEqual(wt, ewt) {
			return td, wiretype.TypeID(i) + wiretype.TypeIDFirst
		}
	}

	// Create a new type.
	tid := wiretype.TypeID(len(td)) + wiretype.TypeIDFirst
	td = append(td, wt)
	return td, tid
}
