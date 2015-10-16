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

func TestInterleavedMessageReader(t *testing.T) {
	type chunkHeader struct {
		msgID int64
		cr    byte
	}

	tests := []struct {
		hex            string
		typeMap        map[typeId]*vdl.Type
		expectedChunks []chunkHeader
	}{
		// Type message split into chunks.
		{
			hex: "81" +
				"5103060029" +
				"e206762e696f2f76" +
				"e22e32332f766f6d2f74657374646174612f7465737474797065732e537472756374416e7901010003416e79010fe1e1" +
				"52012a08000000010000e1e1",
			typeMap: map[typeId]*vdl.Type{
				41: vdl.TypeOf(types.StructAny{}),
			},
			expectedChunks: []chunkHeader{
				{
					msgID: -41,
				},
				{
					cr: WireCtrlTypeCont,
				},
				{
					cr: WireCtrlTypeCont,
				},
				{
					msgID: 41,
				},
			},
		},
		// Value message with types interleaved.
		{
			hex: "81" +
				"5137060029762e696f2f7632332f766f6d2f74657374646174612f7465737474797065732e537472756374416e7901010003416e79010fe1e1" +
				"520000" +
				"5337060029762e696f2f7632332f766f6d2f74657374646174612f7465737474797065732e537472756374416e7901010003416e79010fe1e1" +
				"e3012a0400000001" +
				"5503060029" +
				"e206762e696f2f76" +
				"e22e32332f766f6d2f74657374646174612f7465737474797065732e537472756374416e7901010003416e79010fe1e1" +
				"e300040000e1e1",
			typeMap: map[typeId]*vdl.Type{
				41: vdl.TypeOf(types.StructAny{}),
			},
			expectedChunks: []chunkHeader{
				{
					msgID: -41,
				},
				{
					msgID: 41,
				},
				{
					msgID: -42,
				},
				{
					cr: WireCtrlValueCont,
				},
				{
					msgID: -43,
				},
				{
					cr: WireCtrlTypeCont,
				},
				{
					cr: WireCtrlTypeCont,
				},
				{
					cr: WireCtrlValueCont,
				},
			},
		},
	}

	for _, test := range tests {
		// Helpers to iterate over expected chunks.
		var expectedChunkIndex int
		chunkRead := func() bool {
			expectedChunkIndex++
			return expectedChunkIndex < len(test.expectedChunks)
		}
		getChunk := func() *chunkHeader {
			if expectedChunkIndex >= len(test.expectedChunks) {
				t.Fatalf("chunk index %d out of bounds of slice of length %d", expectedChunkIndex, test.expectedChunks)
			}
			return &test.expectedChunks[expectedChunkIndex]
		}

		bin := hex2Bin(t, test.hex)
		mr := newMessageReader(newDecbufFromBytes(bin))

		// Fake type decoding by looking up types from a predefined map.
		lookupType := func(tid typeId) (*vdl.Type, error) {
			t := test.typeMap[tid]
			if t == nil {
				return nil, fmt.Errorf("type not found")
			}
			return t, nil
		}

		// Callback to read a type in the middle of a value read.
		readSingleType := func() error {
			if getChunk().msgID >= 0 {
				t.Errorf("expected new type, but got %#v", getChunk())
				return nil
			}
			tid, err := mr.StartTypeMessage()
			if err != nil {
				t.Fatalf("error in StartTypeMessage: %v", err)
			}
			cr := byte(0)
			for {
				if getChunk().cr != cr || getChunk().msgID != -int64(tid) {
					t.Errorf("got chunk %#v but expected %#v", chunkHeader{-int64(tid), cr}, *getChunk())
					break
				}
				if err := mr.Skip(mr.buf.lim); err != nil {
					t.Fatalf("error in Skip(): %v", err)
				}
				chunkRead()
				// The type chunk is considered to have ended when new type or non-type continuation chunk
				// is encountered.
				if getChunk().cr != WireCtrlTypeCont {
					break
				}
				consumed, err := mr.startChunk()
				if err != nil {
					t.Fatalf("error in startChunk: %v", err)
				}
				if consumed {
					t.Fatalf("chunk unexpectedly consumed while reading types")
				}
				if mr.curMsgKind != typeMessage {
					t.Fatalf("got value message when expected type message at index %d", expectedChunkIndex)
				}
				cr = WireCtrlTypeCont
				tid = 0
			}
			if err := mr.EndMessage(); err != nil {
				t.Fatalf("error in end message: %v", err)
			}
			return nil
		}

		mr.SetCallbacks(lookupType, readSingleType)

		// Read the value messages until the end of the expected chunk list is reached.
		for expectedChunkIndex < len(test.expectedChunks) {
			tid, err := mr.StartValueMessage()
			if err != nil {
				t.Fatalf("error in StartTypeMessage: %v", err)
			}
			mid := int64(tid)
			cr := byte(0)
			for {
				if getChunk().cr != cr || getChunk().msgID != mid {
					t.Errorf("got chunk %#v but expected %#v", *getChunk(), chunkHeader{mid, cr})
					break
				}
				if err := mr.Skip(mr.buf.lim); err != nil {
					t.Fatalf("error in Skip(): %v", err)
				}
				if !chunkRead() {
					break
				}
				err := mr.nextChunk()
				if err != nil {
					t.Fatalf("error in nextChunk: %v", err)
				}
				if mr.curMsgKind != valueMessage {
					// Type message is complete
					break
				}
				cr = WireCtrlValueCont
				mid = 0
			}
			if err := mr.EndMessage(); err != nil {
				t.Fatalf("error in end message: %v", err)
			}
		}

		if mr.buf.beg != len(bin) {
			// NOTE: This check is invalid if the length of the test bytes exceeds the buffer length and will
			// need to be updated if this is ever changed.
			t.Errorf("input hex was not fully read. read length was: %d, but full length was %d", mr.buf.beg, len(bin))
		}
	}
}

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
		"5120000025762e696f2f7632332f766f6d2f74657374646174612f74657374747970"+
		"e20b65732e4e426f6f6c0101e1")))
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
		"520400010103"+
		"e30761626302fff6e1")))
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
