// Package rt provides initialization of a specific instantiation of the
// runtime.
package rt

import (
	"fmt"
	"sync"

	// TODO(cnicolaou): consider removing this dependency and relying
	// on importing a profile to bring it in.
	google_rt "veyron.io/veyron/veyron/runtimes/google/rt"

	"veyron.io/veyron/veyron2"
	"veyron.io/veyron/veyron2/options"
	"veyron.io/veyron/veyron2/verror2"
)

type Factory func(opts ...veyron2.ROpt) (veyron2.Runtime, error)

var (
	config struct {
		sync.Mutex
		profile veyron2.Profile
		factory Factory
	}

	runtimes struct {
		sync.Mutex
		registered map[string]Factory
	}

	once    sync.Once
	globalR veyron2.Runtime
)

func init() {
	runtimes.registered = make(map[string]Factory)
	// We preregister the google runtime factory as a convenience
	// since otherwise the developer would have to import the google
	// runtime package in order for it to register itself.
	RegisterRuntime(string(options.GoogleRuntime), google_rt.New)
	RegisterRuntime("", google_rt.New)
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
	profile, factory, err := configure(opts...)
	if err != nil {
		return nil, err
	}
	return factory(prependProfile(profile, opts...)...)
}

// R returns the global Runtime instance. It can only be called after
// Init has been called.
func R() veyron2.Runtime {
	return globalR
}

// Init returns the initialized global instance of the runtime.
// Calling it multiple times will always return the result of the
// first call to Init (ignoring subsequently provided options).
// All Veyron apps should call Init as the first call in their main
// function, it will call flag.Parse internally. It will panic on
// encountering an error.
func Init(opts ...veyron2.ROpt) veyron2.Runtime {
	// TODO(cnicolaou): check that subsequent calls to Init use the same
	// or compatible options as the first one.
	once.Do(func() {
		var err error
		globalR, err = New(opts...)
		if err != nil {
			panic(fmt.Sprintf("failed to initialize global runtime: %s", err))
		}
		verror2.SetDefaultContext(globalR.NewContext())
	})
	return globalR
}

// RegisterProfile registers the specified Profile.
// It must be called before the Init or New functions in this package
// are called; typically it will be called by an init function. If called
// multiple times, the last call 'wins'.
func RegisterProfile(profile veyron2.Profile) {
	config.Lock()
	defer config.Unlock()
	config.profile = profile
}

func prependProfile(profile veyron2.Profile, opts ...veyron2.ROpt) []veyron2.ROpt {
	return append([]veyron2.ROpt{options.Profile{profile}}, opts...)
}

func configure(opts ...veyron2.ROpt) (veyron2.Profile, Factory, error) {
	config.Lock()
	defer config.Unlock()
	name := ""
	for _, o := range opts {
		switch v := o.(type) {
		case options.Profile:
			config.profile = v.Profile
		case options.RuntimeName:
			name = string(v)
		}
	}
	runtimes.Lock()
	config.factory = runtimes.registered[name]
	runtimes.Unlock()

	// We must have a factory, but not necessarily a profile.
	if config.factory == nil {
		return nil, nil, fmt.Errorf("no runtime factory has been found for %q", name)
	}
	if config.profile != nil {
		// We have a profile, so it must be compatible with the runtime.
		if config.profile.Runtime() != name {
			return nil, nil, fmt.Errorf("profile %q needs runtime %q, not %q", config.profile.Name(), config.profile.Runtime(), name)
		}
	}
	return config.profile, config.factory, nil
}
