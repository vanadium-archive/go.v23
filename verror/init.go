// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package verror

import (
	"fmt"

	"v.io/v23/vdl"
)

func init() {
	// We must register the error conversion functions between vdl.WireError and
	// the standard error interface with the vdl package.  This allows the vdl
	// package to have minimal dependencies.
	vdl.RegisterNative(WireToNative, WireFromNative)
}

// WireToNative converts from vdl.WireError to verror.E, which
// implements the standard go error interface.
func WireToNative(wire vdl.WireError, native *E) error {
	*native = E{
		ID:     ID(wire.Id),
		Action: retryToAction(wire.RetryCode),
		Msg:    wire.Msg,
	}
	for _, p := range wire.ParamList {
		var pNative interface{}
		if err := vdl.Convert(&pNative, p); err != nil {
			// It's questionable what to do if the conversion fails, or similarly if
			// the conversion ends up with a *vdl.Value, rather than a native Go
			// value.
			//
			// At the moment, for both cases we plug the *vdl.Value or conversion
			// error into the native params.  The idea is that this will still be more
			// useful to the user, since they'll still have the error Id and Action.
			//
			// TODO(toddw): Consider whether there is a better strategy.
			pNative = err
		}
		native.ParamList = append(native.ParamList, pNative)
	}
	return nil
}

// WireFromNative converts from the standard go error interface to
// verror.E, and then to vdl.WireError.
func WireFromNative(wire *vdl.WireError, native error) error {
	e := ExplicitConvert(ErrUnknown, "", "", "", native)
	*wire = vdl.WireError{
		Id:        string(ErrorID(e)),
		RetryCode: retryFromAction(Action(e)),
		Msg:       e.Error(),
	}
	for _, p := range params(e) {
		var pWire *vdl.Value
		if err := vdl.Convert(&pWire, p); err != nil {
			// It's questionable what to do here if the conversion fails, similarly to
			// the conversion failure above in errorFromWire.
			//
			// TODO(toddw): Consider whether there is a better strategy.
			pWire = vdl.StringValue(err.Error())
		}
		wire.ParamList = append(wire.ParamList, pWire)
	}
	return nil
}

func retryToAction(retry vdl.WireRetryCode) ActionCode {
	switch retry {
	case vdl.WireRetryCodeNoRetry:
		return NoRetry
	case vdl.WireRetryCodeRetryConnection:
		return RetryConnection
	case vdl.WireRetryCodeRetryRefetch:
		return RetryRefetch
	case vdl.WireRetryCodeRetryBackoff:
		return RetryBackoff
	}
	// Backoff to no retry by default.
	return NoRetry
}

func retryFromAction(action ActionCode) vdl.WireRetryCode {
	switch action.RetryAction() {
	case NoRetry:
		return vdl.WireRetryCodeNoRetry
	case RetryConnection:
		return vdl.WireRetryCodeRetryConnection
	case RetryRefetch:
		return vdl.WireRetryCodeRetryRefetch
	case RetryBackoff:
		return vdl.WireRetryCodeRetryBackoff
	}
	// Backoff to no retry by default.
	return vdl.WireRetryCodeNoRetry
}

// VDLRead implements the logic to read error from dec.
func VDLRead(dec vdl.Decoder, x *error) error {
	if err := dec.StartValue(); err != nil {
		return err
	}
	if dec.IsNil() {
		if (dec.StackDepth() == 1 || dec.IsAny()) && !vdl.Compatible(vdl.ErrorType, dec.Type()) {
			return fmt.Errorf("incompatible error, from %v", dec.Type())
		}
		*x = nil
		return dec.FinishValue()
	}
	dec.IgnoreNextStartValue()
	var wire vdl.WireError
	if err := wire.VDLRead(dec); err != nil {
		return err
	}
	nativePtr := new(E)
	if err := WireToNative(wire, nativePtr); err != nil {
		return err
	}
	*x = nativePtr
	return nil
}

// VDLWrite implements the logic to write error to enc.
func VDLWrite(enc vdl.Encoder, x error) error {
	var wire vdl.WireError
	if err := WireFromNative(&wire, x); err != nil {
		return err
	}
	return wire.VDLWrite(enc)
}
