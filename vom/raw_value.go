// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"v.io/v23/vdl"
)

// TODO(toddw): Flesh out the RawValue strategy.

type RawValue struct {
	typeDec *TypeDecoder
	valType *vdl.Type
	data    []byte
}
