package security

import (
	"fmt"
	"time"
)

func init() {
	RegisterCaveatValidator(ConstCaveat, func(call Call, isValid bool) error {
		if isValid {
			return nil
		}
		return NewErrConstCaveatValidation(call.Context())
	})

	RegisterCaveatValidator(ExpiryCaveatX, func(call Call, expiry time.Time) error {
		now := call.Timestamp()
		if now.After(expiry) {
			return NewErrExpiryCaveatValidation(call.Context(), now, expiry)
		}
		return nil
	})

	RegisterCaveatValidator(MethodCaveatX, func(call Call, methods []string) error {
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

	RegisterCaveatValidator(PublicKeyThirdPartyCaveatX, func(call Call, params publicKeyThirdPartyCaveat) error {
		discharge, ok := call.RemoteDischarges()[params.ID()]
		if !ok {
			return fmt.Errorf("missing discharge for third party caveat(id=%v)", params.ID())
		}
		// Must be of the valid type.
		var d *publicKeyDischarge
		switch v := discharge.wire.(type) {
		case WireDischargePublicKey:
			d = &v.Value
		default:
			return fmt.Errorf("invalid discharge(%T) for caveat(%T)", v, params)
		}
		// Must be signed by the principal designated by c.DischargerKey
		key, err := params.discharger()
		if err != nil {
			return err
		}
		if err := d.verify(key); err != nil {
			return err
		}
		// And all caveats on the discharge must be met.
		for _, cav := range d.Caveats {
			if err := cav.Validate(call); err != nil {
				return fmt.Errorf("a caveat(%v) on the discharge failed to validate: %v", cav, err)
			}
		}
		return nil
	})
}
