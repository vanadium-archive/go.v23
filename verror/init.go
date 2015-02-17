package verror

import "v.io/core/veyron2/vdl"

func init() {
	// We must register the error conversion functions between vdl.WireError and
	// the standard error interface with the vdl package.  This allows the vdl
	// package to have minimal dependencies.
	vdl.RegisterErrorConv(vdl.ErrorConvInfo{errorFromWire, errorToWire})
}

// errorFromWire converts from vdl.WireError to verror.Standard, which
// implements the standard go error interface.
func errorFromWire(wire vdl.WireError) (error, error) {
	native := Standard{
		IDAction: IDAction{
			ID:     ID(wire.IDAction.ID),
			Action: ActionCode(wire.IDAction.Action),
		},
		Msg: wire.Msg,
	}
	for _, p := range wire.ParamList {
		// TODO(toddw): Check to make sure each param has been converted to a native
		// Go type, rather than *vdl.Value?
		native.ParamList = append(native.ParamList, p)
	}
	return native, nil
}

// errorToWire converts from the standard go error interface to verror.Standard,
// and then to vdl.WireError.
func errorToWire(native error) (vdl.WireError, error) {
	e := ExplicitConvert(ErrUnknown, "", "", "", native)
	wire := vdl.WireError{
		IDAction: vdl.IDAction{
			ID:     string(ErrorID(e)),
			Action: uint32(Action(e)),
		},
		Msg: e.Error(),
	}
	for _, p := range Params(e) {
		// TODO(toddw): Check to make sure each param is convertible to *vdl.Value?
		wire.ParamList = append(wire.ParamList, p)
	}
	return wire, nil
}
