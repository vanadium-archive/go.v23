// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec_test

import (
	"errors"
	"fmt"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/syncbase/nosql/query"
	"v.io/v23/syncbase/nosql/query/exec"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

type mockDB struct {
	ctx *context.T
}

func (db *mockDB) GetContext() *context.T {
	return db.ctx
}

func init() {
	var shutdown v23.Shutdown
	db.ctx, shutdown = test.V23Init()
	defer shutdown()
}

func (db *mockDB) GetTable(table string) (query.Table, error) {
	return nil, errors.New(fmt.Sprintf("No such table: %s", table))
}

var db mockDB

type splitErrorTest struct {
	err    error
	offset int64
	errStr string
}

func TestSplitError(t *testing.T) {
	basic := []splitErrorTest{
		{
			query.NewErrInvalidSelectField(db.GetContext(), 7),
			7,
			"Select field must be 'k' or 'v[{.<ident>}...]'.",
		},
		{
			query.NewErrTableCantAccess(db.GetContext(), 14, "Bob", errors.New("No such table: Bob")),
			14,
			"Table Bob does not exist (or cannot be accessed): No such table: Bob.",
		},
	}

	for _, test := range basic {
		offset, errStr := exec.SplitError(test.err)
		if offset != test.offset || errStr != test.errStr {
			t.Errorf("err: %v; got %d:%s, want %d:%s", test.err, offset, errStr, test.offset, test.errStr)
		}
	}
}
