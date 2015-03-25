// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vdlroot contains the standard VDL packages; the VDLROOT environment
// variable should point at this directory.
//
// This package contains import dependencies on all its sub-packages.  This is
// meant as a convenient mechanism to pull in all standard vdl packages; import
// vdlroot to ensure the types for all standard vdl packages are registered.
package vdlroot

import (
	_ "v.io/v23/vdlroot/signature"
	_ "v.io/v23/vdlroot/time"
	_ "v.io/v23/vdlroot/vdltool"
)
