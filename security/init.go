package security

import "veyron.io/veyron/veyron2/vom"

func init() {
	vom.Register(Signature{})
	vom.Register(Caveat{})
	vom.Register(unixTimeExpiryCaveat(0))
	vom.Register(methodCaveat{})
	vom.Register(peerBlessingsCaveat{})
	vom.Register(blessingsImpl{})
	vom.Register(Certificate{})
	vom.Register(publicKeyThirdPartyCaveat{})
	vom.Register(publicKeyDischarge{})
}
