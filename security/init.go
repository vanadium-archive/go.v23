package security

import "veyron.io/veyron/veyron2/vdl/vdlutil"

func init() {
	vdlutil.Register(Signature{})
	vdlutil.Register(Caveat{})
	vdlutil.Register(unixTimeExpiryCaveat(0))
	vdlutil.Register(methodCaveat{})
	vdlutil.Register(peerBlessingsCaveat{})
	vdlutil.Register(blessingsImpl{})
	vdlutil.Register(Certificate{})
	vdlutil.Register(publicKeyThirdPartyCaveat{})
	vdlutil.Register(publicKeyDischarge{})
}
