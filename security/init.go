package security

import "v.io/core/veyron2/vdl"

func init() {
	vdl.Register(unixTimeExpiryCaveat(0))
	vdl.Register(methodCaveat{})
	vdl.Register(blessingsImpl{})
	vdl.Register(publicKeyThirdPartyCaveat{})
	vdl.Register(publicKeyDischarge{})
}
