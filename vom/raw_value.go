package vom

import (
	"v.io/v23/vdl"
)

// TODO(toddw): Flesh out the RawValue strategy.

type RawValue struct {
	recvTypes *decoderTypes
	valType   *vdl.Type
	data      []byte
}
