package security

import "veyron2/vom"

func init() {
	vom.Register(Signature{})
	vom.Register(ServiceCaveat{})
}
