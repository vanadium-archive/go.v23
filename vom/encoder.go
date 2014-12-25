package vom

import (
	"errors"
	"fmt"
	"io"
)

// Encoder manages the transmission and marshaling of type and value data to the
// other side of a connection.
type Encoder struct {
	writer io.Writer

	// We hold separate encoding states for types and values to ensure all type
	// messages are encoded before the value message that uses the types.  This is
	// necessary since we don't discover the types of interface values until we've
	// potentially already encoded parts of the value.  We need buffering in order
	// to prepend message lengths anyways, so this isn't too expensive.
	stateT encState // Encoding state for types.
	stateV encState // Encoding state for values.

	// Underlying encoder for different formats.
	formatEnc formatEncoder
	encBinary *encoderBinary
	encJSON   *encoderJSON
}

// NewEncoder returns a new Encoder that writes to the given writer in the
// binary format.
func NewEncoder(w io.Writer) *Encoder {
	e := &Encoder{writer: w}
	e.stateT.init()
	e.stateV.init()
	e.formatEnc = e.newFormatEncoder(FormatBinary)
	return e
}

type formatEncoder interface {
	Format() Format
	EncodeReflectValue(rv Value) error
}

func (e *Encoder) newFormatEncoder(f Format) formatEncoder {
	switch f {
	case FormatBinary:
		if e.encBinary == nil {
			e.encBinary = newEncoderBinary(e.writer, &e.stateT, &e.stateV)
		}
		return e.encBinary
	case FormatJSON:
		if e.encJSON == nil {
			e.encJSON = newEncoderJSON(e.writer, &e.stateT, &e.stateV)
		}
		return e.encJSON
	default:
		panic(fmt.Errorf("vom: unknown format %d", f))
	}
}

// SetFormat sets the encoding format to use for subsequent calls to Encode or
// EncodeValue.  The same Encoder may be used to write values using different
// encoding formats.  Returns the encoder as a convenience for chained calls.
func (e *Encoder) SetFormat(f Format) *Encoder {
	e.formatEnc = e.newFormatEncoder(f)
	return e
}

// Encode transmits the value v; shorthand for EncodeValue(reflect.ValueOf(v)).
func (e *Encoder) Encode(v interface{}) error {
	return e.EncodeValue(ValueOf(v))
}

// EncodeValue transmits the value represented by rv, with the guarantee that
// all necessary type definitions have been transmitted first.  Type definitions
// include the type name, e.g. "v.io/veyron/veyron/lib/vom.StructType".  The type name is
// used when decoding into pointers to nil interfaces, and is otherwise unused.
// The type name is determined by the registered types, and if not registered,
// the default name for the type; types need not be registered before encoding
// unless you want to use a name different than the default.
func (e *Encoder) EncodeValue(rv Value) error {
	if !rv.IsValid() {
		return errors.New("vom: can't encode untyped nil value")
	}
	return e.formatEnc.EncodeReflectValue(rv)
}

// encState represents the encoding state passed to format encoders.
type encState struct {
	buf *encbuf

	// For ptr/refID management.
	ptrToRefID map[uintptr]int64
	nextRefID  int64
}

func (s *encState) init() {
	s.buf = newEncbuf()
}

func (s *encState) reset() {
	s.ptrToRefID = nil
}

// lookupOrAssignRefID returns the refID associated with ptr, where lookup=true
// means the entry already existed, and lookup=false means we assigned a new
// refID and entry.
func (s *encState) lookupOrAssignRefID(ptr uintptr) (refID int64, lookup bool) {
	if s.ptrToRefID == nil {
		s.ptrToRefID = make(map[uintptr]int64)
		s.nextRefID = 1
	}
	if refID, lookup = s.ptrToRefID[ptr]; lookup {
		return
	}
	refID = s.nextRefID
	s.ptrToRefID[ptr] = refID
	s.nextRefID++
	return
}
