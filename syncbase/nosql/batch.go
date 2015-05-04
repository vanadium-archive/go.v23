// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	"v.io/v23/context"
)

// TODO(sadovsky): Add retry loop.
func RunInBatch(ctx *context.T, d Database, opts BatchOptions, fn func(b BatchDatabase) error) error {
	b, err := d.BeginBatch(ctx, opts)
	if err != nil {
		return err
	}
	if err := fn(b); err != nil {
		b.Abort(ctx)
		return err
	}
	if err := b.Commit(ctx); err != nil {
		// TODO(sadovsky): Commit() can fail for a number of reasons, e.g. RPC
		// failure or ErrConcurrentTransaction. Depending on the cause of failure,
		// it may be desirable to retry the Commit() and/or to call Abort(). For
		// now, we always abort on a failed commit.
		b.Abort(ctx)
		return err
	}
	return nil
}
