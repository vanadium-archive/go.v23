package vdl

import (
	"errors"
	"sync"
)

// ErrorConvInfo holds functions used to convert between WireError and the
// standard error interface.
type ErrorConvInfo struct {
	// FromWire converts from WireError to the standard error interface.
	FromWire func(wire WireError) (native error, _ error)
	// ToWire converts from the standard error interface to WireError.
	ToWire func(native error) (wire WireError, _ error)
}

var (
	errorConvMu sync.Mutex
	errorConv   *ErrorConvInfo

	errRegErrNil   = errors.New("vdl: RegisterErrorConv called with nil conversion function")
	errRegErrMulti = errors.New("vdl: RegisterErrorConv called multiple times")
	errRegErrNever = errors.New(`vdl: RegisterErrorConv must be called to perform native<->wire error conversions.  Import the "v.io/core/veyron2/verror" package in your program.`)
)

// RegisterErrorConv registers the conversion functions between WireError and
// the standard error interface.  Called exactly once via an init function in
// the "v.io/core/veyron2/verror" package.  Panics if called multiple times.
//
// This registration mechanism exists solely to minimize dependencies on other
// packages from the vdl package.  The vdl package is depended upon by all
// vdl-generated files and many other core packages, so minimizing dependencies
// is a good thing, and sometimes necessary to avoid cycles.
func RegisterErrorConv(conv ErrorConvInfo) {
	if conv.FromWire == nil || conv.ToWire == nil {
		panic(errRegErrNil)
	}
	errorConvMu.Lock()
	defer errorConvMu.Unlock()
	if errorConv != nil {
		panic(errRegErrMulti)
	}
	errorConv = &conv
}

// ErrorConv returns the conversion information from RegisterErrorConv, or an
// error if RegisterErrorConv hasn't been called.
func ErrorConv() (ErrorConvInfo, error) {
	errorConvMu.Lock()
	defer errorConvMu.Unlock()
	if errorConv == nil {
		return ErrorConvInfo{}, errRegErrNever
	}
	return *errorConv, nil
}
