package vom2

// TODO(toddw): This file contains hacky stuff for the vom->vom2 transition.  It
// should be removed when the transition is complete.

import "os"

var enabled = true

func init() {
	// The env var must start with VEYRON_, in order for the device manager
	// to pass it through, for
	// v.io/core/veyron/services/mgmt/device/impl test.
	if os.Getenv("VEYRON_VOM2") != "" {
		enabled = true
	}
}

// IsEnabled returns true iff the vom2 transition is enabled.  We check the
// VEYRON_VOM2 environment variable in an init function, and if it's set to
// anything other than the empty string, the transition is enabled.
func IsEnabled() bool {
	return enabled
}

// SetEnabled explicitly enables/disables the vom2 transition.  The passed-in
// value overrides the value set in VEYRON_VOM2 environment variable.
func SetEnabled(val bool) {
	enabled = val
}
