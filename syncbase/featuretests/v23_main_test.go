// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"os"
	"testing"

	"v.io/x/ref/lib/v23test"
	_ "v.io/x/ref/runtime/factories/generic"
)

func TestMain(m *testing.M) {
	os.Exit(v23test.Run(m.Run))
}
