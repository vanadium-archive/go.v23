// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"v.io/v23/context"
	"v.io/v23/naming"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase/util"
	"v.io/v23/vom"
)

func newRow(parentFullName, key string, schemaVersion int32) Row {
	// Note, we immediately unescape row keys on the server side. See comment in
	// server/dispatcher.go for explanation.
	fullName := naming.Join(parentFullName, util.Escape(key))
	return &row{
		c:               wire.RowClient(fullName),
		fullName:        fullName,
		key:             key,
		dbSchemaVersion: schemaVersion,
	}
}

type row struct {
	c               wire.RowClientMethods
	fullName        string
	key             string
	dbSchemaVersion int32
}

var _ Row = (*row)(nil)

// Key implements Row.Key.
func (r *row) Key() string {
	return r.key
}

// FullName implements Row.FullName.
func (r *row) FullName() string {
	return r.fullName
}

// Exists implements Row.Exists.
func (r *row) Exists(ctx *context.T) (bool, error) {
	return r.c.Exists(ctx, r.dbSchemaVersion)
}

// Get implements Row.Get.
func (r *row) Get(ctx *context.T, value interface{}) error {
	bytes, err := r.c.Get(ctx, r.dbSchemaVersion)
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
	return r.c.Put(ctx, r.dbSchemaVersion, bytes)
}

// Delete implements Row.Delete.
func (r *row) Delete(ctx *context.T) error {
	return r.c.Delete(ctx, r.dbSchemaVersion)
}
