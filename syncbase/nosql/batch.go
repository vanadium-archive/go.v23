// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	"v.io/v23/context"
)

// TODO(sadovsky): Add retry loop.
func RunInBatch(ctx *context.T, db Database, opts BatchOptions, fn func(db BatchDatabase) error) error {
	b, err := db.BeginBatch(ctx, opts)
	if err != nil {
		return err
	}
	if err := fn(b); err != nil {
		b.Abort(ctx)
		return err
	}
	if err := b.Commit(ctx); err != nil {
		b.Abort(ctx)
		return err
	}
	return nil
}
