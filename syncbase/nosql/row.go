// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	wire "v.io/syncbase/v23/services/syncbase/nosql"
	"v.io/v23/context"
	"v.io/v23/verror"
)

type row struct {
	c    wire.RowClientMethods
	name string
	key  string
}

var _ Row = (*row)(nil)

// TODO(sadovsky): Validate names before sending RPCs.

// Key implements Row.Key.
func (r *row) Key() string {
	return r.key
}

// Get implements Row.Get.
func (r *row) Get(ctx *context.T, value interface{}) error {
	return verror.NewErrNotImplemented(ctx)
}

// Put implements Row.Put.
func (r *row) Put(ctx *context.T, value interface{}) error {
	return verror.NewErrNotImplemented(ctx)
}

// Delete implements Row.Delete.
func (r *row) Delete(ctx *context.T) error {
	return r.c.Delete(ctx)
}
