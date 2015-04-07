// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

type RowRange interface {
	Start() string
	Limit() string
}

type PrefixRange interface {
	RowRange
	Prefix() string
}

type rowRange struct {
	start string
	limit string
}

func (r *rowRange) Start() string {
	return r.start
}

func (r *rowRange) Limit() string {
	return r.limit
}

func SingleRow(row string) RowRange {
	return &rowRange{start: row, limit: row + "\x00"}
}

func Range(start, limit string) RowRange {
	return &rowRange{start: start, limit: limit}
}

// prefixRange implements the RowRange interface.  We do not represent
// a prefix as a rowRange because we want to be able to distinguish
// prefixes from ranges (e.g. syncgroups work with prefixes, not ranges).
type prefixRange struct {
	prefix string
}

func (r *prefixRange) Start() string {
	return r.prefix
}

func (r *prefixRange) Limit() string {
	// TODO(kash): implement me.
	return ""
}

func (r *prefixRange) Prefix() string {
	return r.prefix
}

func Prefix(prefix string) PrefixRange {
	return &prefixRange{prefix: prefix}
}
