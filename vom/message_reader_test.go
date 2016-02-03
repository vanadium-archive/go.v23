// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"fmt"
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vom/testdata/types"
)

func TestTypeMessageReader(t *testing.T) {
	lookupType := func(tid typeId) (*vdl.Type, error) {
		switch tid {
		case 1:
			return vdl.BoolType, nil
		case 41:
			return vdl.TypeOf(types.NBool(true)), nil
		default:
			return nil, fmt.Errorf("invalid type id: %d", tid)
		}
	}

	successMr := newMessageReader(newDecbufFromBytes(hex2Bin(t, "81"+
		crHex(WireCtrlTypeFirstChunk)+"5120000025762e696f2f7632332f766f6d2f74657374646174612f74657374747970"+
		crHex(WireCtrlTypeLastChunk)+"0b65732e4e426f6f6c0101e1")))
	successMr.SetCallbacks(lookupType, nil)
	mid, err := successMr.StartTypeMessage()
	if err != nil {
		t.Fatalf("error starting type message: %v", err)
	}
	if mid != 41 {
		t.Errorf("got invalid type id: %v, expected %v", mid, 41)
	}
	if err := successMr.Skip(43); err != nil {
		t.Fatalf("error skipping bytes: %v", err)
	}
	if err := successMr.EndMessage(); err != nil {
		t.Fatalf("error ending message: %v", err)
	}

	failingMr := newMessageReader(newDecbufFromBytes(hex2Bin(t, "810201")))
	failingMr.SetCallbacks(lookupType, nil)
	if _, err := failingMr.StartTypeMessage(); err == nil {
		t.Fatalf("expected error when reading value message on type stream")
	}
}

func TestValueMessageReader(t *testing.T) {
	lookupType := func(tid typeId) (*vdl.Type, error) {
		switch tid {
		case 1:
			return vdl.BoolType, nil
		case 41:
			return vdl.TypeOf(types.NStruct{}), nil
		default:
			return nil, fmt.Errorf("invalid type id: %d", tid)
		}
	}

	successMr := newMessageReader(newDecbufFromBytes(hex2Bin(t, "81"+
		crHex(WireCtrlValueFirstChunk)+"520400010103"+
		crHex(WireCtrlValueLastChunk)+"0761626302fff6e1")))
	successMr.SetCallbacks(lookupType, nil)
	mid, err := successMr.StartValueMessage()
	if err != nil {
		t.Fatalf("error starting value message: %v", err)
	}
	if mid != 41 {
		t.Errorf("got invalid message id: %v, expected %v", mid, 41)
	}
	if err := successMr.Skip(11); err != nil {
		t.Fatalf("error skipping bytes: %v", err)
	}
	if err := successMr.EndMessage(); err != nil {
		t.Fatalf("error ending message: %v", err)
	}

	failingMr := newMessageReader(newDecbufFromBytes(hex2Bin(t, "81513f060027762e696f2f7632332f766f6d2f74657374646174612f7465737474797065732e4e53747275637401030001410101e10001420103e10001430109e1e1")))
	failingMr.SetCallbacks(lookupType, nil)
	if _, err := failingMr.StartValueMessage(); err == nil {
		t.Fatalf("expected error when reading type message on value stream")
	}
}

func crHex(b byte) string {
	return fmt.Sprintf("%x", b)
}
