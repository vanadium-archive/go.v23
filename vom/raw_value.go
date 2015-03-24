package vom

import (
	"v.io/v23/vdl"
)

// TODO(toddw): Flesh out the RawValue strategy.

type RawValue struct {
	typeDec *TypeDecoder
	valType *vdl.Type
	data    []byte
}
