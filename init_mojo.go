// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build mojo

package v23

import (
	"os"

	"mojo/public/go/application"

	"v.io/v23/context"
)

func init() {
	// musllibc does not sent an Arg[0] when dlopening() a shared library.  We
	// must set Arg[0] manually if it does not exist.
	if len(os.Args) < 1 {
		os.Args = []string{""}
	}
}

// Init should be called once for each vanadium mojo app, providing
// the setup of the initial vanadium context and a shutdown function
// that can be used to clean up the runtime. We allow calling Init
// multiple times, but only as long as you call the Shutdown returned
// previously before calling Init the second time.
func Init(mctx application.Context) (*context.T, Shutdown) {
	ctx, shutdown, err := TryInit(mctx)
	if err != nil {
		panic(err)
	}
	return ctx, shutdown
}

func TryInit(mctx application.Context) (*context.T, Shutdown, error) {
	// mctx.Args() is a slice that contains the url of this mojo service
	// followed by all arguments passed to the mojo service via the
	// "--args-for" flag. Since the v23 runtime factories parse arguments
	// from os.Args, we must overwrite os.Args with mctx.Args().
	//
	// Note that os.Args must be set before calling internalInit().
	os.Args = mctx.Args()
	if len(os.Args) == 0 {
		// TODO(jhahn): mojo_run doesn't pass the service url when there is
		// no flags. Follow up this bug.
		os.Args = []string{mctx.URL()}
	}
	return internalInit()
}
