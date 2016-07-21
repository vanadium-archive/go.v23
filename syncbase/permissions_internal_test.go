// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

func NewNonexistentBatchForTest(db Database) BatchDatabase {
	d := db.(*database)
	return newBatch(d.parentFullName, d.id, "sdeadbeefdeadbeef") // likely doesn't exist
}
