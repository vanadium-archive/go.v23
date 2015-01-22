package vom

import (
	"v.io/core/veyron2/vdl"
)

// TODO(toddw): Flesh out the RawValue strategy.

type RawValue struct {
	recvTypes *decoderTypes
	valType   *vdl.Type
	data      []byte
}
