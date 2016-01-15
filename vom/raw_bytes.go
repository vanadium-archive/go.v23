// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"v.io/v23/vdl"
)

type RawBytes struct {
	Type     *vdl.Type
	RefTypes []*vdl.Type
	Data     []byte
}

func RawBytesFromValue(value interface{}) (*RawBytes, error) {
	// TODO(bprosnitz) This implementation is temporary - we should make it faster
	dat, err := VersionedEncode(Version81, value)
	if err != nil {
		return nil, err
	}
	var rb RawBytes
	err = Decode(dat, &rb)
	return &rb, err
}

func (rb *RawBytes) ToValue(value interface{}) error {
	// TODO(bprosnitz) This implementation is temporary - we should make it faster
	dat, err := VersionedEncode(Version81, rb)
	if err != nil {
		return err
	}
	return Decode(dat, value)
}