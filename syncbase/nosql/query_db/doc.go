// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package query_db defines the interfaces a consumer of the query package needs to
// implement.
//
// The Database interface is used to get Table interfaces (by name).
// The Table interface is used to get a KeyValueStream (by key prefixes).
// The KeyValueStream is used to consume key value pairs that match the prefixes.
// The Exec function is a public wrapper around Exec in the internal query package.
// TODO(jkline): Choose a better name than query_db.
package query_db
