package rt_test

import (
	"testing"

	"veyron.io/veyron/veyron2"
	"veyron.io/veyron/veyron2/config"
	"veyron.io/veyron/veyron2/options"
	"veyron.io/veyron/veyron2/rt"
)

func ExampleInit() {
	r := rt.Init()
	// Go ahead and use the runtime.
	log := r.Logger()
	log.Infof("hello world")
}

type myprofile struct{}

func (mp *myprofile) Name() string {
	return "test"
}

func (mp *myprofile) Runtime() string {
	return ""
}

func (mp *myprofile) Platform() *veyron2.Platform {
	return &veyron2.Platform{"google", nil, "v1", "any", "rel1", ".2", "who knows", "this host"}
}

func (mp *myprofile) String() string {
	return "myprofile on " + mp.Platform().String()
}

func (mp *myprofile) Init(veyron2.Runtime, *config.Publisher) error {
	return nil
}

func ExampleInitWithProfile() {
	r := rt.Init(options.Profile{&myprofile{}})
	// Go ahead and use the runtime.
	log := r.Logger()
	log.Infof("hello world from my product: %s", r.Profile())
}

// TODO(cnicolaou): add tests to:
//  - catch mismatched profile and runtimes - e.g. profile asks for "foo"
// runtime, but only bar is available.
//  - tests to catch multiple calls to init with different options

func TestErrorOnNew(t *testing.T) {
	_, err := rt.New(options.RuntimeName("foo"))
	if err == nil {
		t.Errorf("expected an error!")
	}
}
