// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdltest

import "v.io/v23/vdl"

// EntryValue is like Entry, but represents the target and source values as
// *vdl.Value, rather than interface{}.
type EntryValue struct {
	Label  string
	Target *vdl.Value
	Source *vdl.Value
}

// Name returns the name of the EntryValue.
func (e EntryValue) Name() string {
	return e.Label + " Target(" + e.Target.String() + ") Source(" + e.Source.String() + ")"
}

// ToEntryValue converts the Entry e into an EntryValue.
func ToEntryValue(e Entry) EntryValue {
	return EntryValue{
		Label:  e.Label,
		Target: vdl.ValueOf(e.Target),
		Source: vdl.ValueOf(e.Source),
	}
}

// ToEntryValues converts each Entry in entries into a corresponding EntryValue.
func ToEntryValues(entries []Entry) []EntryValue {
	var result []EntryValue
	for _, e := range entries {
		result = append(result, ToEntryValue(e))
	}
	return result
}
