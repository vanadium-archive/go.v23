// +build ignore

// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	wire "v.io/syncbase/v23/services/syncbase"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/vdl"
)

type table struct {
	c            wire.TableClientMethods
	name         string
	relativeName string
}

var _ Table = (*table)(nil)

// Name implements Table.Name.
func (t *table) Name() string {
	return t.relativeName
}

// BindItem implements Table.BindItem.
func (t *table) BindItem(key *vdl.Value) Item {
	name := naming.Join(t.name, encodeKey(key))
	return &item{wire.ItemClient(name), name, key}
}

// Get implements Table.Get.
func (t *table) Get(ctx *context.T, key *vdl.Value) (*vdl.Value, error) {
	return t.BindItem(key).Get(ctx)
}

// Put implements Table.Put.
func (t *table) Put(ctx *context.T, value *vdl.Value) error {
	return t.BindItem(t.getPrimaryKey(value)).Put(ctx, value)
}

// Delete implements Table.Delete.
func (t *table) Delete(ctx *context.T, key *vdl.Value) error {
	return t.BindItem(key).Delete(ctx)
}

// TODO(sadovsky): Implement. Perhaps for the initial prototype we should
// require there to be a string field named "primaryKey".
func (t *table) getPrimaryKey(value *vdl.Value) *vdl.Value {
	return nil
}
