// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	wire "v.io/syncbase/v23/services/syncbase/nosql"
	"v.io/v23/context"
	"v.io/v23/verror"
)

type batch struct {
	database
}

var _ BatchDatabase = (*batch)(nil)

// Commit implements BatchDatabase.Commit.
func (b *batch) Commit(ctx *context.T) error {
	return b.c.Commit(ctx, b.schemaVersion())
}

// Abort implements BatchDatabase.Abort.
func (b *batch) Abort(ctx *context.T) error {
	return b.c.Abort(ctx, b.schemaVersion())
}

// RunInBatch runs the given fn in a batch, managing retries and commit/abort.
func RunInBatch(ctx *context.T, d Database, opts wire.BatchOptions, fn func(b BatchDatabase) error) error {
	// TODO(sadovsky): Make the number of attempts configurable.
	var err error
	for i := 0; i < 3; i++ {
		b, err := d.BeginBatch(ctx, opts)
		if err != nil {
			return err
		}
		if err = fn(b); err != nil {
			b.Abort(ctx)
			return err
		}
		// TODO(sadovsky): Commit() can fail for a number of reasons, e.g. RPC
		// failure or ErrConcurrentTransaction. Depending on the cause of failure,
		// it may be desirable to retry the Commit() and/or to call Abort().
		if err = b.Commit(ctx); verror.ErrorID(err) != wire.ErrConcurrentBatch.ID {
			return err
		}
	}
	return err
}
