// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package nosql

import (
	"v.io/syncbase/v23/syncbase/util"
)

// RowRange represents all rows with keys in [start, end). If end is "", all
// rows with keys >= start are included.
type RowRange interface {
	Start() string
	// TODO(sadovsky): Rename to "limit" everywhere.
	End() string
}

// PrefixRange represents all rows with keys that have some prefix.
type PrefixRange interface {
	RowRange
	Prefix() string
}

// rowRange implements the RowRange interface.
type rowRange struct {
	start string
	end   string
}

var _ RowRange = (*rowRange)(nil)

func (r *rowRange) Start() string {
	return r.start
}

func (r *rowRange) End() string {
	return r.end
}

func SingleRow(row string) RowRange {
	return &rowRange{start: row, end: row + "\x00"}
}

func Range(start, end string) RowRange {
	return &rowRange{start: start, end: end}
}

// prefixRange implements the PrefixRange interface (and thus also the RowRange
// interface). We do not represent a prefix as a rowRange because we want to be
// able to distinguish prefixes from ranges (e.g. syncgroups work with prefixes,
// not ranges).
type prefixRange struct {
	prefix string
}

var _ PrefixRange = (*prefixRange)(nil)

func (r *prefixRange) Start() string {
	return util.PrefixRangeStart(r.prefix)
}

func (r *prefixRange) End() string {
	return util.PrefixRangeEnd(r.prefix)
}

func (r *prefixRange) Prefix() string {
	return r.prefix
}

func Prefix(prefix string) PrefixRange {
	return &prefixRange{prefix: prefix}
}
