package vom

import (
	"io"
	"reflect"

	"v.io/core/veyron2/vdl"
)

// Encoder manages the transmission and marshaling of typed values to the other
// side of a connection.
type Encoder struct {
	enc encoder
}

type encoder interface {
	vdl.Target
	StartEncode() error
	FinishEncode() error
}

// NewBinaryEncoder returns a new Encoder that writes to the given writer in the
// binary format.  The binary format is compact and fast.
//
// TODO(toddw): Rename to NewEncoder.
func NewBinaryEncoder(w io.Writer) (*Encoder, error) {
	// The binary format always starts with a magic byte.
	_, err := w.Write([]byte{binaryMagicByte})
	if err != nil {
		return nil, err
	}
	return &Encoder{newBinaryEncoder(w, newEncoderTypes())}, nil
}

// Encode transmits the value v.  Values of type T are encodable as long as the
// type of T is representable as val.Type, or T is special-cased below;
// otherwise an error is returned.
//
//   Types that are special-cased, only for v:
//     *RawValue     - Transcode v into the appropriate output format.
//
//   Types that are special-cased, recursively throughout v:
//     *val.Value    - Encode the semantic value represented by v.
//     reflect.Value - Encode the semantic value represented by v.
//
// Encode(nil) is a special case that encodes the zero value of the any type.
// See the discussion of zero values in the Value documentation.
func (e *Encoder) Encode(v interface{}) error {
	if raw, ok := v.(*RawValue); ok && raw != nil {
		// TODO(toddw): Decode from RawValue, encoding into e.enc.
		_ = raw
		panic("Encode(RawValue) NOT IMPLEMENTED")
	}
	if err := e.enc.StartEncode(); err != nil {
		return err
	}
	if err := vdl.FromReflect(e.enc, reflect.ValueOf(v)); err != nil {
		return err
	}
	return e.enc.FinishEncode()
}
