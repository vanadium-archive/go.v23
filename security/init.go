package security

import (
	"fmt"
	"time"
	"v.io/v23/verror"
)

// Prefix for error codes.
const pkgPath = "v.io/v23/security"

var (
	errMissingDischarge = verror.Register(pkgPath+".errMissingDischarge", verror.NoRetry, "{1:}{2:}missing discharge for third party caveat(id={3}){:_}")
	errInvalidDischarge = verror.Register(pkgPath+".errInvalidDischarge", verror.NoRetry, "{1:}{2:}invalid discharge({3}) for caveat({4}){:_}")
	errFailedDischarge  = verror.Register(pkgPath+".errFailedDischarge", verror.NoRetry, "{1:}{2:}a caveat({3}) on the discharge failed to validate{:_}")
)

func init() {
	RegisterCaveatValidator(ConstCaveat, func(call Call, _ CallSide, isValid bool) error {
		if isValid {
			return nil
		}
		return NewErrConstCaveatValidation(call.Context())
	})

	RegisterCaveatValidator(ExpiryCaveatX, func(call Call, _ CallSide, expiry time.Time) error {
		now := call.Timestamp()
		if now.After(expiry) {
			return NewErrExpiryCaveatValidation(call.Context(), now, expiry)
		}
		return nil
	})

	RegisterCaveatValidator(MethodCaveatX, func(call Call, _ CallSide, methods []string) error {
		if call.Method() == "" && len(methods) == 0 {
			return nil
		}
		for _, m := range methods {
			if call.Method() == m {
				return nil
			}
		}
		return NewErrMethodCaveatValidation(call.Context(), call.Method(), methods)
	})

	RegisterCaveatValidator(PublicKeyThirdPartyCaveatX, func(call Call, side CallSide, params publicKeyThirdPartyCaveat) error {
		var dmap map[string]Discharge
		switch side {
		case CallSideLocal:
			dmap = call.LocalDischarges()
		case CallSideRemote:
			dmap = call.RemoteDischarges()
		default:
			return fmt.Errorf("invalid end argument %v provided to caveat validator", side)
		}

		discharge, ok := dmap[params.ID()]
		if !ok {
			return verror.New(errMissingDischarge, call.Context(), params.ID())
		}
		// Must be of the valid type.
		var d *publicKeyDischarge
		switch v := discharge.wire.(type) {
		case WireDischargePublicKey:
			d = &v.Value
		default:
			return verror.New(errInvalidDischarge, call.Context(), fmt.Sprintf("%T", v), fmt.Sprintf("%T", params))
		}
		// Must be signed by the principal designated by c.DischargerKey
		key, err := params.discharger(call.Context())
		if err != nil {
			return err
		}
		if err := d.verify(call.Context(), key); err != nil {
			return err
		}
		// And all caveats on the discharge must be met.
		for _, cav := range d.Caveats {
			if err := cav.Validate(call, side); err != nil {
				return verror.New(errFailedDischarge, call.Context(), cav, err)
			}
		}
		return nil
	})
}
