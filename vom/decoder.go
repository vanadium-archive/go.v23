package vom

import (
	"fmt"
	"io"
	"reflect"
)

// Decoder manages the receipt and unmarshaling of type and value data from the
// other side of a connection.
type Decoder struct {
	decbuf    *decbuf
	decBinary *decoderBinary
	decJSON   *decoderJSON
}

// NewDecoder returns a new Decoder that reads from the given reader.
func NewDecoder(r io.Reader) *Decoder {
	// Use a 4KiB decode buffer.  The size must be >= the largest sized ReadBuf()
	// and PeekAtLeast() that we'll call.  At the moment this means it must be >=
	// the length of the longest JSON number we wish to decode.
	decbuf := newDecbuf(r, 4<<10)
	return &Decoder{
		decbuf:    decbuf,
		decBinary: newDecoderBinary(decbuf),
		decJSON:   newDecoderJSON(decbuf),
	}
}

// SetMaxStringLength sets the maximum decoded string length - if the decoder
// encounters a string longer than this, it returns ErrStringTooLong.  We need a
// maximum to avoid invalid JSON (e.g. mismatched quotes) from using all
// available memory.  The default is DefaultMaxStringLength.
func (d *Decoder) SetMaxStringLength(max int) {
	d.decJSON.SetMaxStringLength(max)
}

// Decode reads the next value from the reader and stores it in v; shorthand for
// DecodeValue(reflect.ValueOf(v)).  Since DecodeValue requires a settable
// value, v must be a pointer to an allocated object that will receive the
// decoded value.
func (d *Decoder) Decode(v interface{}) error {
	return d.DecodeValue(ValueOf(v))
}

// DecodeValue reads the next value from the reader and stores it in rv, where
// rv must be settable so that it can receive the decoded value.  There are
// three decoding modes depending on rv:
//
// * If rv is invalid (representing nil) the value will be read and discarded.
//
// * If rv represents a value of a concrete type, or an interface containing a
// value of a concrete type, the value will be filled in, and the type must be
// compatible with the encoded value.  Nil pointers will be replaced by pointers
// to allocated objects of the corresponding type.
//
// * If rv represents a nil interface, a value of an appropriate type is created
// by vom to receive the decoded value.  The type is determined first via lookup
// of the encoded type name in the registered types.  If that fails we attempt
// to create the type dynamically based on the underlying type of the encoded
// value.  E.g. if you registered type Foo, you can decode *Foo or []Foo without
// additional registration.  Some types cannot created dynamically: arrays are
// turned into slices, and unregistered structs return an error.  If you want to
// decode an encoded struct into a nil interface, you must register its type
// first.
//
// Note that the choice of whether to decode into a concrete or interface value
// is orthogonal to whether the encoded data is a concrete or interface value.
// Any combination is fine, as long as the resulting decode type is compatible
// with the encoded value.
func (d *Decoder) DecodeValue(rv Value) error {
	// An invalid rv means the decoded value should be ignored - e.g. Decode(nil).
	// A valid rv must be checked for settability, to ensure the user will receive
	// the decoded value.
	if rv.IsValid() {
		if rv.Kind() == reflect.Ptr && !rv.IsNil() {
			// If the user passed in a non-nil pointer to a value, they expect to
			// decode into the value, which is settable.
			rv = rv.Elem()
		}
		if !rv.CanSet() {
			return fmt.Errorf("vom: can't decode into unsettable value of type %q", rv.Type())
		}
	}
	// Keep decoding messages until we've decoded a value; an arbitrary number of
	// types (each in its own message) are allowed.
	for {
		isValue, err := d.decodeMsg(rv)
		if err != nil {
			return err
		}
		if isValue {
			break
		}
	}
	return nil
}

// decodeMsg decodes a single type or value message, filling in rv for value
// messages.  The returned bool is true iff the message is a value message.
func (d *Decoder) decodeMsg(rv Value) (bool, error) {
	// The first item in each message is a signed id.
	id, err := rawDecodeInt(d.decbuf)
	if err != nil {
		return false, err
	}
	switch id {
	case -5, +5, -6, +6, -7, +16:
		// Ignore ascii whitespace: '\t', '\n', '\v', '\f', '\r', ' '.  Recall that
		// the id is encoded as a signed integer, where we take the sign bit and
		// move it from the msb to the lsb, and complement negative numbers.  Thus
		// e.g. -5 is encoded as 0x9 and +5 is encoded as 0xa.
		return false, nil
	case jsonStartID: // '['
		return d.decJSON.DecodeMsg(rv)
	default:
		return d.decBinary.DecodeMsg(id, rv)
	}
}

// decState represents the decoding state - currently just the refID to value
// map and associated logic.
type decState struct {
	refIDToValue map[int64]Value
}

func (s *decState) reset() {
	s.refIDToValue = nil
}

// fillFromRefID fills in the rv with the value associated with refID.
func (s *decState) fillFromRefID(refID int64, rti *rtInfo, rv Value) error {
	if s.refIDToValue != nil {
		if refval, ok := s.refIDToValue[refID]; ok {
			if !refval.Type().ConvertibleTo(rti.rt) {
				return fmt.Errorf("vom: can't convert refID %d type %q to type %q", refID, refval.Type(), rti.rt)
			}
			return rv.trySet(refval.Convert(rti.rt))
		}
	}
	return fmt.Errorf("vom: unknown refID %d, seen refIDs %v", refID, s.refIDToValue)
}

// defineRefID adds the refID -> rv association to our map.
func (s *decState) defineRefID(refID int64, rti *rtInfo, rv Value) (*rtInfo, Value, error) {
	if s.refIDToValue == nil {
		s.refIDToValue = make(map[int64]Value)
	}
	if val, ok := s.refIDToValue[refID]; ok {
		return nil, nil, fmt.Errorf("vom: duplicate refID %d def, existing value: %v", refID, val)
	}
	// Fill in rv with a new value and add rv to our map.  We must do this before
	// the recursive call to decodeTypedValue, so that cyclic values will have a
	// valid pointer to copy even if the actual value hasn't been filled in yet.
	if rti.kind == typeKindPtr && rv.IsNil() {
		rv.Set(New(rti.elem.rt))
	}
	s.refIDToValue[refID] = rv
	// Walk one step of rv if it's a pointer, to match the encoded value.
	if rti.kind == typeKindPtr {
		rti = rti.elem
		rv = rv.Elem()
	}
	return rti, rv, nil
}
