// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package message

import (
	"testing"

	"v.io/v23/context"
)

func TestVarInt(t *testing.T) {
	cases := []uint64{
		0x00, 0x01,
		0x7f, 0x80,
		0xff, 0x100,
		0xffff, 0x10000,
		0xffffff, 0x1000000,
		0xffffffff, 0x100000000,
		0xffffffffff, 0x10000000000,
		0xffffffffffff, 0x1000000000000,
		0xffffffffffffff, 0x100000000000000,
		0xffffffffffffffff,
	}
	ctx, cancel := context.RootContext()
	defer cancel()
	for _, want := range cases {
		got, b, valid := readVarUint64(ctx, writeVarUint64(want, []byte{}))
		if !valid {
			t.Fatalf("error reading %x", want)
		}
		if len(b) != 0 {
			t.Errorf("unexpected buffer remaining for %x: %v", want, b)
		}
		if got != want {
			t.Errorf("got: %d want: %d", got, want)
		}
	}
}
