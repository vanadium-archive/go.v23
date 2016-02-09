// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"bytes"
	"reflect"

	"v.io/v23/vdl"
)

type RawBytes struct {
	Version    Version
	Type       *vdl.Type
	RefTypes   []*vdl.Type
	AnyLengths []uint64
	Data       []byte
}

func RawBytesOf(value interface{}) *RawBytes {
	rb, err := RawBytesFromValue(value)
	if err != nil {
		panic(err)
	}
	return rb
}

func RawBytesFromValue(value interface{}) (*RawBytes, error) {
	// TODO(bprosnitz) This implementation is temporary - we should make it faster
	dat, err := Encode(value)
	if err != nil {
		return nil, err
	}
	var rb RawBytes
	err = Decode(dat, &rb)
	return &rb, err
}

func (rb *RawBytes) ToValue(value interface{}) error {
	// TODO(bprosnitz) This implementation is temporary - we should make it faster
	dat, err := Encode(rb)
	if err != nil {
		return err
	}
	return Decode(dat, value)
}

func (rb *RawBytes) ToTarget(target vdl.Target) error {
	var buf bytes.Buffer
	enc := NewVersionedEncoder(rb.Version, &buf)
	if err := enc.enc.encodeRaw(rb); err != nil {
		return err
	}
	dec := NewDecoder(bytes.NewReader(buf.Bytes()))
	return dec.decodeToTarget(target)
}

type rvHackInterface interface {
	HackGetRv() reflect.Value
}
