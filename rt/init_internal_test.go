package rt

import (
	"strings"
	"testing"

	"v.io/core/veyron2"
	"v.io/core/veyron2/config"
	"v.io/core/veyron2/options"
)

type myprofile struct {
	called int
}

func (mp *myprofile) Name() string {
	return "test"
}

func (mp *myprofile) Runtime() (string, []veyron2.ROpt) {
	return "mock", nil
}

func (mp *myprofile) Platform() *veyron2.Platform {
	return &veyron2.Platform{"google", nil, "v1", "any", "rel1", ".2", "who knows", "this host"}
}

func (mp *myprofile) String() string {
	return "myprofile on " + mp.Platform().String()
}

func (mp *myprofile) Init(veyron2.Runtime, *config.Publisher) (veyron2.AppCycle, error) {
	mp.called++
	return nil, nil
}

func (mp *myprofile) Cleanup() {}

func TestErrorOnNew(t *testing.T) {
	clearProfile()
	defer clearProfile()
	profile := &myprofile{}
	RegisterProfile(profile)
	_, err := New(&options.Profile{profile})
	if err == nil {
		t.Errorf("expected an error!")
	}
}

func TestPanicOnSecondRegistration(t *testing.T) {
	clearProfile()
	defer clearProfile()
	profile := &myprofile{}
	RegisterProfile(profile)
	catcher := func() {
		r := recover()
		if r == nil {
			t.Fatalf("recover returned nil")
		}
		str := r.(string)
		if !strings.Contains(str, "test\", has already been registered") {
			t.Fatalf("unexpected error: %s", str)
		}
	}
	defer catcher()
	RegisterProfile(profile)
}

// for tests only
func clearRuntime() {
	runtimes.Lock()
	runtimes.registered = make(map[string]Factory)
	runtimes.Unlock()
}

// for tests only
func clearProfile() {
	runtimeConfig.Lock()
	runtimeConfig.profile = nil
	runtimeConfig.profileStack = nil
	runtimeConfig.factory = nil
	runtimeConfig.Unlock()
}
