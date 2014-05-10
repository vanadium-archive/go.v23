package vom2

import (
	"io"
)

// Encoder manages the transmission and marshaling of typed values to the other
// side of a connection.
type Encoder struct {
	writer io.Writer
}

// NewEncoderBinary returns a new Encoder that writes to the given writer in the
// binary format.  The binary format is compact and fast.
func NewBinaryEncoder(w io.Writer) *Encoder {
	return nil
}

// NewEncoderJSON returns a new Encoder that writes to the given writer in the
// JSON format.  The JSON format is simpler but slower.
func NewJSONEncoder(w io.Writer) *Encoder {
	return nil
}

// Encode transmits the value v.  Values of type T are encodable as long as the
// type of T is representable as val.Type, or T is special-cased below;
// otherwise an error is returned.
//
// Types that are special-cased, recursively throughout v:
//   reflect.Value - Encode the semantic value represented by v.
//   val.Value     - Encode the semantic value represented by v.
//   RawValue      - Transcode v into the appropriate output format.
//
// Encode(nil) is a special case that encodes the zero value of the any type.
// See the discussion of zero values in the Value documentation.
func (e *Encoder) Encode(v interface{}) error {
	return nil
}
