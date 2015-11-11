// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !mojo

package v23

import "v.io/v23/context"

// Init should be called once for each vanadium executable, providing
// the setup of the vanadium initial context.T and a Shutdown function
// that can be used to clean up the runtime.  We allow calling Init
// multiple times (useful in tests), but only as long as you call the
// Shutdown returned previously before calling Init the second time.
func Init() (*context.T, Shutdown) {
	return internalInit()
}
