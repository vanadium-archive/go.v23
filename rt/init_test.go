package rt_test

import (
	"net"

	"veyron2"
	"veyron2/rt"
	"veyron2/security"
)

func ExampleInit() {
	r := rt.Init()
	// Go ahead and use the runtime.
	log := r.Logger()
	log.Infof("hello world")
}

type myproduct struct{}

func (p *myproduct) Description() (vendor, model, name string) {
	return "me", "mine", "home"
}

func (p *myproduct) ID() security.PublicID {
	return nil
}

func (p *myproduct) Addresses() []net.Addr {
	return []net.Addr{}
}

func ExampleInitWithOptions() {
	r := rt.Init(veyron2.ProductOpt{&myproduct{}})
	// Go ahead and use the runtime.
	log := r.Logger()
	log.Infof("hello world for my product")
}
