package verror

import "v.io/core/veyron2/vdl"

func init() {
	// We must register the error conversion functions between vdl.WireError and
	// the standard error interface with the vdl package.  This allows the vdl
	// package to have minimal dependencies.
	vdl.RegisterNative(wireErrorToNative, wireErrorFromNative)
}

// wireErrorToNative converts from vdl.WireError to verror.Standard, which
// implements the standard go error interface.
//
// REQUIRES: native is an allocated zero value of error.  Note that this is
// guaranteed by vdl.RegisterNative for calls to this this function.
func wireErrorToNative(wire vdl.WireError, native *error) error {
	std := Standard{
		IDAction: IDAction{
			ID:     ID(wire.IDAction.ID),
			Action: ActionCode(wire.IDAction.Action),
		},
		Msg: wire.Msg,
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
			// useful to the user, since they'll still have the error ID and Action.
			//
			// TODO(toddw): Consider whether there is a better strategy.
			pNative = err
		}
		std.ParamList = append(std.ParamList, pNative)
	}
	*native = std
	return nil
}

// wireErrorFromNative converts from the standard go error interface to
// verror.Standard, and then to vdl.WireError.
//
// REQUIRES: wire is an allocated zero value of vdl.WireError.  Note that this
// is guaranteed by vdl.RegisterNative for calls to this this function.
func wireErrorFromNative(wire *vdl.WireError, native error) error {
	e := ExplicitConvert(ErrUnknown, "", "", "", native)
	wire.IDAction.ID = string(ErrorID(e))
	wire.IDAction.Action = uint32(Action(e))
	wire.Msg = e.Error()
	wire.ParamList = nil
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
