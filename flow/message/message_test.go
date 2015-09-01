// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package message_test

import (
	"reflect"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow/message"
	"v.io/v23/rpc/version"
	_ "v.io/x/ref/runtime/factories/fake"
	"v.io/x/ref/test"
)

func init() {
	test.Init()
}

func testMessages(t *testing.T, ctx *context.T, cases []message.Message) {
	for _, want := range cases {
		encoded, err := message.Append(ctx, want, nil)
		if err != nil {
			t.Errorf("unexpected error for %#v: %v", want, err)
		}
		got, err := message.Read(ctx, encoded)
		if err != nil {
			t.Errorf("unexpected error reading %#v: %v", want, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got: %#v, want %#v", got, want)
		}
	}
}

func TestSetup(t *testing.T) {
	ctx, shutdown := v23.Init()
	defer shutdown()
	ep1, err := v23.NewEndpoint(
		"@5@tcp@foo.com:1234@00112233445566778899aabbccddeeff@m@v.io/foo")
	if err != nil {
		t.Fatal(err)
	}
	ep2, err := v23.NewEndpoint(
		"@5@tcp@bar.com:1234@00112233445566778899aabbccddeeff@m@v.io/bar")
	if err != nil {
		t.Fatal(err)
	}
	testMessages(t, ctx, []message.Message{
		&message.Setup{Versions: version.RPCVersionRange{Min: 3, Max: 5}},
		&message.Setup{
			Versions: version.RPCVersionRange{Min: 3, Max: 5},
			PeerNaClPublicKey: &[32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13,
				14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31},
			PeerRemoteEndpoint: ep1,
			PeerLocalEndpoint:  ep2,
		},
		&message.Setup{},
	})
}

func TestTearDown(t *testing.T) {
	ctx, shutdown := v23.Init()
	defer shutdown()
	testMessages(t, ctx, []message.Message{
		&message.TearDown{Message: "foobar"},
		&message.TearDown{},
	})
}

func TestAuth(t *testing.T) {
	ctx, shutdown := v23.Init()
	defer shutdown()
	p := v23.GetPrincipal(ctx)
	sig, err := p.Sign([]byte("message"))
	if err != nil {
		t.Fatal(err)
	}
	testMessages(t, ctx, []message.Message{
		&message.Auth{BlessingsKey: 1, DischargeKey: 5, ChannelBinding: sig, PublicKey: p.PublicKey()},
		&message.Auth{BlessingsKey: 1, DischargeKey: 5, ChannelBinding: sig},
		&message.Auth{ChannelBinding: sig, PublicKey: p.PublicKey()},
		&message.Auth{},
	})
}

func TestOpenFlow(t *testing.T) {
	ctx, shutdown := v23.Init()
	defer shutdown()
	testMessages(t, ctx, []message.Message{
		&message.OpenFlow{ID: 23, InitialCounters: 1 << 20, BlessingsKey: 42, DischargeKey: 55},
		&message.OpenFlow{},
	})
}

func TestAddReceiveBuffers(t *testing.T) {
	ctx, shutdown := v23.Init()
	defer shutdown()
	testMessages(t, ctx, []message.Message{
		&message.Release{},
		&message.Release{Counters: map[uint64]uint64{
			4: 233,
			9: 423242,
		}},
	})
}

func TestData(t *testing.T) {
	ctx, shutdown := v23.Init()
	defer shutdown()
	testMessages(t, ctx, []message.Message{
		&message.Data{ID: 1123, Flags: 232, Payload: [][]byte{[]byte("fake payload")}},
		&message.Data{},
	})
}