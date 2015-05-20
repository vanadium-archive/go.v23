// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	wire "v.io/syncbase/v23/services/syncbase/nosql"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/vom"
)

func newRow(parentFullName, key string) Row {
	// TODO(sadovsky): Escape delimiters in key?
	fullName := naming.Join(parentFullName, key)
	return &row{
		c:        wire.RowClient(fullName),
		fullName: fullName,
		key:      key,
	}
}

type row struct {
	c        wire.RowClientMethods
	fullName string
	key      string
}

var _ Row = (*row)(nil)

// TODO(sadovsky): Validate names before sending RPCs.

// Key implements Row.Key.
func (r *row) Key() string {
	return r.key
}

// FullName implements Row.FullName.
func (r *row) FullName() string {
	return r.fullName
}

// Get implements Row.Get.
func (r *row) Get(ctx *context.T, value interface{}) error {
	bytes, err := r.c.Get(ctx)
	if err != nil {
		return err
	}
	return vom.Decode(bytes, value)
}

// Put implements Row.Put.
func (r *row) Put(ctx *context.T, value interface{}) error {
	bytes, err := vom.Encode(value)
	if err != nil {
		return err
	}
	return r.c.Put(ctx, bytes)
}

// Delete implements Row.Delete.
func (r *row) Delete(ctx *context.T) error {
	return r.c.Delete(ctx)
}
