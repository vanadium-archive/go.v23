// Package vdlroot contains import dependencies on all its sub-packages.  This
// is meant as a convenient mechanism to pull in standard vdl packages; import
// vdlroot to ensure the types for all standard vdl packages are registered.
package vdlroot

import (
	_ "v.io/v23/vdl/vdlroot/src/signature"
	_ "v.io/v23/vdl/vdlroot/src/time"
	_ "v.io/v23/vdl/vdlroot/src/vdltool"
)
