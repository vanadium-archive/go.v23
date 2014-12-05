// Package rt provides initialization of a specific instantiation of the
// runtime.
package rt

import (
	"fmt"
	"runtime"
	"sync"

	"veyron.io/veyron/veyron2"
	"veyron.io/veyron/veyron2/options"
	"veyron.io/veyron/veyron2/verror2"
)

type Factory func(opts ...veyron2.ROpt) (veyron2.Runtime, error)

func printStack(stack []uintptr) string {
	s := ""
	for _, pc := range stack {
		fnc := runtime.FuncForPC(pc)
		file, line := fnc.FileLine(pc)
		s += fmt.Sprintf("%s:%d: %s\n", file, line, fnc.Name())
	}
	return s
}

var (
	runtimeConfig struct {
		sync.Mutex
		profile      veyron2.Profile
		profileStack []uintptr
		factory      Factory
	}

	runtimes struct {
		sync.Mutex
		registered map[string]Factory
	}

	once       sync.Once
	verrorOnce sync.Once
)

func init() {
	runtimes.registered = make(map[string]Factory)
}

// RegisterRuntime registers a runtime name and associated Factory.
// The google runtime is preregistered with the names "google" and "".
// Additional runtimes may be registered and selected via an appropriate
// profile.
func RegisterRuntime(name string, factory Factory) {
	runtimes.Lock()
	runtimes.registered[name] = factory
	runtimes.Unlock()
}

// New creates and initializes a new instance of the runtime. It should
// be used in unit tests and any situation where a single global runtime
// instance is inappropriate.
func New(opts ...veyron2.ROpt) (veyron2.Runtime, error) {
	profile, profileOpts, factory, err := configure(opts...)
	if err != nil {
		return nil, err
	}
	opts = append(profileOpts, opts...)
	r, err := factory(prependProfile(profile, opts...)...)
	if err == nil {
		verrorOnce.Do(func() {
			verror2.SetDefaultContext(r.NewContext())
		})
	}
	return r, err
}

// RegisterProfile registers the specified Profile.
// It must be called before the Init or New functions in this package
// are called; typically it will be called by an init function. It will panic
// if called more than once.
func RegisterProfile(profile veyron2.Profile) {
	runtimeConfig.Lock()
	defer runtimeConfig.Unlock()
	if runtimeConfig.profile != nil {
		str := fmt.Sprintf("A profile, %q, has already been registered.\n", runtimeConfig.profile.Name())
		str += `This is most likely because a library package is importing a profile.
Look for imports of the form 'v.io/profiles/...' and remove them.
Profiles should only be imported in your main package.
Previous registration was from:
`
		str += printStack(runtimeConfig.profileStack)
		str += "Current registration is from:\n"
		stack := make([]uintptr, 1)
		runtime.Callers(2, stack)
		str += printStack(stack)
		panic(str)
	}
	runtimeConfig.profile = profile
	runtimeConfig.profileStack = make([]uintptr, 1)
	runtime.Callers(2, runtimeConfig.profileStack)
}

func prependProfile(profile veyron2.Profile, opts ...veyron2.ROpt) []veyron2.ROpt {
	return append([]veyron2.ROpt{options.Profile{profile}}, opts...)
}

func configure(opts ...veyron2.ROpt) (veyron2.Profile, []veyron2.ROpt, Factory, error) {
	runtimeConfig.Lock()
	defer runtimeConfig.Unlock()
	for _, o := range opts {
		switch v := o.(type) {
		case options.Profile:
			// Can override a registered profile.
			runtimeConfig.profile = v.Profile
		}
	}
	runtimes.Lock()
	defer runtimes.Unlock()
	// Let the profile specify the runtime, use a default otherwise.
	ropts := []veyron2.ROpt{}
	name := ""
	if runtimeConfig.profile != nil {
		name, ropts = runtimeConfig.profile.Runtime()
	} else {
		name = veyron2.GoogleRuntimeName
	}
	runtimeConfig.factory = runtimes.registered[name]

	// We must have a factory, but not necessarily a profile.
	if runtimeConfig.factory == nil {
		str := fmt.Sprintf("No runtime factory has been found for %q", name)
		str += `This is most likely because your main package has not imported
		a profile, or that profile does not import a runtime implementation`
		return nil, nil, nil, fmt.Errorf(str)
	}
	if runtimeConfig.profile == nil {
		str := `No profile has been registered nor specified. This is most likely because your main package has not imported a profile`
		return nil, nil, nil, fmt.Errorf(str)

	}
	return runtimeConfig.profile, ropts, runtimeConfig.factory, nil
}
