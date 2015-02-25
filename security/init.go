package security

import (
	"fmt"
	"time"
)

func init() {
	RegisterCaveatValidator(ConstCaveat, func(ctx Context, isValid bool) error {
		if isValid {
			return nil
		}
		return fmt.Errorf("failing validation in false const caveat")
	})

	RegisterCaveatValidator(UnixTimeExpiryCaveatX, func(ctx Context, unixTime int64) error {
		now := ctx.Timestamp()
		expiry := time.Unix(int64(unixTime), 0)
		if now.After(expiry) {
			return fmt.Errorf("now(%v) is after expiry(%v)", now, expiry)
		}
		return nil
	})

	RegisterCaveatValidator(ExpiryCaveatX, func(ctx Context, expiry time.Time) error {
		now := ctx.Timestamp()
		if now.After(expiry) {
			return fmt.Errorf("now(%v) is after expiry(%v)", now, expiry)
		}
		return nil
	})

	RegisterCaveatValidator(MethodCaveatX, func(ctx Context, methods []string) error {
		if ctx.Method() == "" && len(methods) == 0 {
			return nil
		}
		for _, m := range methods {
			if ctx.Method() == m {
				return nil
			}
		}
		return fmt.Errorf("method %q is not in list %v", ctx.Method(), methods)
	})

	RegisterCaveatValidator(PublicKeyThirdPartyCaveatX, func(ctx Context, params publicKeyThirdPartyCaveat) error {
		discharge, ok := ctx.RemoteDischarges()[params.ID()]
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
			if err := cav.Validate(ctx); err != nil {
				return fmt.Errorf("a caveat(%v) on the discharge failed to validate: %v", cav, err)
			}
		}
		return nil
	})
}
