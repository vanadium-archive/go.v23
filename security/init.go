package security

import "veyron2/vom"

func init() {
	vom.Register(Signature{})
	vom.Register(Caveat{})
	vom.Register(unixTimeExpiryCaveat(0))
	vom.Register(methodCaveat{})
	vom.Register(peerBlessingsCaveat{})
	vom.Register(Expiry{})
}
