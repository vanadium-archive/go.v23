// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package engine defines an Exec function while performs a syncQL query
// and returns a syncql.ResultStream.
package engine

import (
	ds "v.io/v23/query/engine/datasource"
	"v.io/v23/query/engine/internal"
)

func Create(db ds.Database) ds.QueryEngine {
	return internal.Create(db)
}
