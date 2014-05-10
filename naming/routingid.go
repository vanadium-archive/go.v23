package naming

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
)

var (
	ErrInvalidString = errors.New("string is of the wrong format and/or size")
)

// RoutingIDs have one essential property, namely that they are, to a very
// high probability globally unique. Global uniqueness is required in order
// to support comparing Endpoints for equality; this is required for sharing
// connections, for proxying (though global uniqueness is not strictly
// required) and determining if different names resolve to the same endpoint.
type RoutingID struct {
	value [routingIDLength]byte
}

const (
	routingIDLength          = 16
	firstUnreservedRoutingID = 1024
)

var (
	// NullRoutingID is a special value representing the nil route.
	NullRoutingID = FixedRoutingID(0)

	errNotARoutingID = errors.New("Not a RoutingID")
)

// FixedRoutingID returns a routing ID from a constant.
func FixedRoutingID(i uint64) RoutingID {
	var rid RoutingID
	binary.BigEndian.PutUint64(rid.value[8:16], i)
	return rid
}

// IsReserved() returns true iff the RoutingID is in the reserved range.
func (rid RoutingID) IsReserved() bool {
	return isZero(rid.value[0:14]) && isLessThan(rid.value[15:16], firstUnreservedRoutingID)
}

func isZero(buf []byte) bool {
	for _, b := range buf {
		if b != 0 {
			return false
		}
	}
	return true
}

func isLessThan(buf []byte, j uint16) bool {
	return binary.BigEndian.Uint16(buf) < j
}

// String returns a print representation of the RoutingID.
func (rid RoutingID) String() string {
	return hex.EncodeToString(rid.value[:])
}

// FromString reads an RoutingID from a hex encoded string. If the argument
// string is of zero length the RoutingID will be set to NullRoutingID
func (rid *RoutingID) FromString(s string) error {
	if len(s) == 0 {
		*rid = NullRoutingID
		return nil
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	if len(b) != routingIDLength {
		return ErrInvalidString
	}
	copy(rid.value[:], b)
	return nil
}

// Read a RoutingID from an io.Reader.
func ReadRoutingID(reader io.Reader) (RoutingID, error) {
	var rid RoutingID
	_, err := io.ReadFull(reader, rid.value[:])
	return rid, err
}

// Write a RoutingID to an io.Writer.
func (rid RoutingID) Write(writer io.Writer) error {
	_, err := writer.Write(rid.value[:])
	return err
}

// Since the type is opaque, we provide a custom Gob encoder.
func (rid *RoutingID) GobEncode() ([]byte, error) {
	return rid.value[:], nil
}

func (rid *RoutingID) GobDecode(buf []byte) error {
	if len(buf) != routingIDLength {
		return errNotARoutingID
	}
	copy(rid.value[:], buf)
	return nil
}

func (rid *RoutingID) VomEncode() ([routingIDLength]byte, error) {
	return rid.value, nil
}

func (rid *RoutingID) VomDecode(value [routingIDLength]byte) error {
	rid.value = value
	return nil
}

func NewRoutingID() (RoutingID, error) {
	var rid RoutingID
	for {
		_, err := io.ReadFull(rand.Reader, rid.value[:])
		if err != nil {
			return NullRoutingID, err
		}
		if !rid.IsReserved() {
			return rid, nil
		}
	}
	return rid, nil
}

func Compare(a, b RoutingID) bool {
	return bytes.Compare(a.value[:], b.value[:]) == 0
}

// Implement EndpointOpt so that RoutingID can be passed as an optional
// argument to FormatEndpoint
func (RoutingID) EndpointOpt() {}
