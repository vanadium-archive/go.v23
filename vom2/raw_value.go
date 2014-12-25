package vom2

import (
	"v.io/veyron/veyron2/vdl"
)

// TODO(toddw): Flesh out the RawValue strategy.

type RawValue struct {
	recvTypes *decoderTypes
	valType   *vdl.Type
	data      []byte
}
