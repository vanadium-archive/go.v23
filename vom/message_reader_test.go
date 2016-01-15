// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"bytes"
	"fmt"
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vom/testdata/types"
)

func TestInterleavedMessageReader(t *testing.T) {
	type chunkHeader struct {
		msgID      int64
		finalChunk bool
	}

	tests := []struct {
		name           string
		hex            string
		typeMap        map[typeId]*vdl.Type
		expectedChunks []chunkHeader
	}{
		// Type message split into chunks.
		{
			name: "type chunks",
			hex: "81" +
				crHex(WireCtrlTypeFirstChunk) + "5103060029" +
				crHex(WireCtrlTypeChunk) + "06762e696f2f76" +
				crHex(WireCtrlTypeLastChunk) + "2e32332f766f6d2f74657374646174612f7465737474797065732e537472756374416e7901010003416e79010fe1e1" +
				"52012a08000000010000e1e1",
			typeMap: map[typeId]*vdl.Type{
				41: vdl.TypeOf(types.StructAny{}),
			},
			expectedChunks: []chunkHeader{
				{
					msgID: -41,
				},
				{},
				{
					finalChunk: true,
				},
				{
					msgID:      41,
					finalChunk: true,
				},
			},
		},
		// Value message with types interleaved.
		{
			name: "interleaved chunks",
			hex: "81" +
				"5137060029762e696f2f7632332f766f6d2f74657374646174612f7465737474797065732e537472756374416e7901010003416e79010fe1e1" +
				crHex(WireCtrlValueFirstChunk) + "520000" +
				"5337060029762e696f2f7632332f766f6d2f74657374646174612f7465737474797065732e537472756374416e7901010003416e79010fe1e1" +
				crHex(WireCtrlValueChunk) + "012a0400000001" +
				crHex(WireCtrlTypeFirstChunk) + "5503060029" +
				crHex(WireCtrlTypeChunk) + "06762e696f2f76" +
				crHex(WireCtrlTypeLastChunk) + "2e32332f766f6d2f74657374646174612f7465737474797065732e537472756374416e7901010003416e79010fe1e1" +
				crHex(WireCtrlValueLastChunk) + "00040000e1e1",
			typeMap: map[typeId]*vdl.Type{
				41: vdl.TypeOf(types.StructAny{}),
			},
			expectedChunks: []chunkHeader{
				{
					msgID:      -41,
					finalChunk: true,
				},
				{
					msgID: 41,
				},
				{
					msgID:      -42,
					finalChunk: true,
				},
				{},
				{
					msgID: -43,
				},
				{},
				{
					finalChunk: true,
				},
				{
					finalChunk: true,
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
		expectedChunk := func() *chunkHeader {
			if expectedChunkIndex >= len(test.expectedChunks) {
				t.Fatalf("%s: chunk index %d out of bounds of slice of length %d", test.name, expectedChunkIndex, test.expectedChunks)
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
			if expectedChunk().msgID >= 0 {
				t.Errorf("%s: expected new type, but got %#v", test.name, expectedChunk())
				return nil
			}
			tid, err := mr.StartTypeMessage()
			if err != nil {
				t.Fatalf("%s: error in StartTypeMessage: %v", test.name, err)
			}
			for {
				if expectedChunk().msgID != -int64(tid) {
					t.Errorf("%s: got chunk %v but expected %v", test.name, -int64(tid), expectedChunk().msgID)
					break
				}
				if expectedChunk().finalChunk != mr.finalChunk {
					t.Errorf("%s: final chunk was %v, but expected %v", test.name, mr.finalChunk, expectedChunk().finalChunk)
				}
				if err := mr.Skip(mr.buf.lim); err != nil {
					t.Fatalf("%s: error in Skip(): %v", test.name, err)
				}
				if expectedChunk().finalChunk {
					break
				}
				if !chunkRead() {
					break
				}

				consumed, err := mr.startChunk()
				if err != nil {
					t.Fatalf("%s: error in startChunk: %v", test.name, err)
				}
				if consumed {
					t.Fatalf("%s: chunk unexpectedly consumed while reading types", test.name)
				}
				if mr.curMsgKind != typeMessage {
					t.Fatalf("%s: got value message when expected type message at index %d", test.name, expectedChunkIndex)
				}
				tid = 0
			}
			if err := mr.EndMessage(); err != nil {
				t.Fatalf("%s: error in end message: %v", test.name, err)
			}
			chunkRead() // expectedChunk() now refers to the message after the type message
			return nil
		}

		mr.SetCallbacks(lookupType, readSingleType)

		// Read the value messages until the end of the expected chunk list is reached.
		for expectedChunkIndex < len(test.expectedChunks) {
			tid, err := mr.StartValueMessage()
			if err != nil {
				t.Fatalf("%s: error in StartValueMessage: %v", test.name, err)
			}
			mid := int64(tid)
			for {
				if expectedChunk().msgID != mid {
					t.Errorf("%s: got chunk mid %v but expected %v", test.name, mid, expectedChunk().msgID)
					break
				}
				if err := mr.Skip(mr.buf.lim); err != nil {
					t.Fatalf("%s: error in Skip(): %v", test.name, err)
				}
				if expectedChunk().finalChunk {
					break
				}
				if !chunkRead() {
					break
				}
				err := mr.nextChunk()
				if err != nil {
					t.Fatalf("%s: error in nextChunk: %v", test.name, err)
				}
				mid = 0
			}
			if err := mr.EndMessage(); err != nil {
				t.Fatalf("%s: error in end message: %v", test.name, err)
			}
			chunkRead() // Point to the next message
		}

		if mr.buf.beg != len(bin) {
			// NOTE: This check is invalid if the length of the test bytes exceeds the buffer length and will
			// need to be updated if this is ever changed.
			t.Errorf("%s: input hex was not fully read. read length was: %d, but full length was %d", test.name, mr.buf.beg, len(bin))
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

func TestReadAllValueBytes(t *testing.T) {
	type testCase struct {
		Name          string
		InputTypeHex  string
		InputValueHex string
		ExpectedHex   string
		NumMessages   int
	}

	// NOTE: More types are tested in raw_bytes_test.go -- this just tests broad classes.
	tests := []testCase{
		{
			Name:          "Bool",
			InputValueHex: "0201",
			ExpectedHex:   "01",
			NumMessages:   1,
		},
		{
			Name:          "uint16(65534)",
			InputValueHex: "08fefffe",
			ExpectedHex:   "fefffe",
			NumMessages:   1,
		},
		{
			Name:          "\"abc\"",
			InputValueHex: "0603616263",
			ExpectedHex:   "03616263",
			NumMessages:   1,
		},
		{
			Name:          "[]byte(\"abc\")",
			InputValueHex: "4e03616263",
			ExpectedHex:   "03616263",
			NumMessages:   1,
		},
		{
			Name:          "typeobject(bool)",
			InputValueHex: "1c010100",
			ExpectedHex:   "00",
			NumMessages:   1,
		},
		{
			Name:          "testtypes.NStruct{A: true, B: \"abc\", C: 123}",
			InputTypeHex:  "513f060027762e696f2f7632332f766f6d2f74657374646174612f7465737474797065732e4e53747275637401030001410101e10001420103e10001430109e1e1",
			InputValueHex: "520b0001010361626302fff6e1",
			ExpectedHex:   "0001010361626302fff6e1",
			NumMessages:   1,
		},
		{
			Name:          "testtypes.StructAny{Any: false}",
			InputTypeHex:  "5137060029762e696f2f7632332f766f6d2f74657374646174612f7465737474797065732e537472756374416e7901010003416e79010fe1e1",
			InputValueHex: "52010104000000e1",
			ExpectedHex:   "000000e1",
			NumMessages:   1,
		},
		{
			Name: "chunked testtypes.NStruct{A: true, B: \"abc\", C: 123}",
			InputTypeHex: crHex(WireCtrlTypeFirstChunk) + "513b060027762e696f2f7632332f766f6d2f74657374646174612f7465737474797065732e4e53747275637401030001410101e10001420103e1000143" +
				crHex(WireCtrlTypeLastChunk) + "040109e1e1",
			InputValueHex: crHex(WireCtrlValueFirstChunk) + "52050001010361" +
				crHex(WireCtrlValueLastChunk) + "06626302fff6e1",
			ExpectedHex: "0001010361626302fff6e1",
			NumMessages: 1,
		},
		{
			Name:         "chunked testtypes.StructAny{Any: false}",
			InputTypeHex: "5137060029762e696f2f7632332f766f6d2f74657374646174612f7465737474797065732e537472756374416e7901010003416e79010fe1e1",
			InputValueHex: crHex(WireCtrlValueFirstChunk) + "5201000100" +
				crHex(WireCtrlValueLastChunk) + "0101030000e1",
			ExpectedHex: "000000e1",
			NumMessages: 1,
		},
		{
			Name:         "two messages testtypes.NStruct{A: true, B: \"abc\", C: 123}",
			InputTypeHex: "513f060027762e696f2f7632332f766f6d2f74657374646174612f7465737474797065732e4e53747275637401030001410101e10001420103e10001430109e1e1",
			InputValueHex: "520b0001010361626302fff6e1" + // first message
				"520b0001010361626302fff6e1", // second message
			ExpectedHex: "0001010361626302fff6e1",
			NumMessages: 2,
		},
	}

nextTest:
	for _, test := range tests {
		// Interleaved
		mr := newMessageReader(newDecbufFromBytes(hex2Bin(t, "81"+test.InputTypeHex+test.InputValueHex)))
		typeDec := newTypeDecoderInternal(mr)
		mr.SetCallbacks(typeDec.lookupType, typeDec.readSingleType)

		for i := 0; i < test.NumMessages; i++ {
			_, err := mr.StartValueMessage()
			if err != nil {
				t.Errorf("%s (interleaved): error in start value message: %v", test.Name, err)
				continue nextTest
			}
			b, err := mr.ReadAllValueBytes()
			if err != nil {
				t.Errorf("%s (interleaved): error in ReadAllBytes(): %v", test.Name, err)
				continue nextTest
			}
			if !bytes.Equal(b, hex2Bin(t, test.ExpectedHex)) {
				t.Errorf("%s (interleaved): expected: %s but got %x", test.Name, test.ExpectedHex, b)
				continue nextTest
			}
			if err := mr.EndMessage(); err != nil {
				t.Errorf("%s (interleaved): error ending message: %v", test.Name, err)
				continue nextTest
			}
		}

		// Split type and value streams
		typeMr := newMessageReader(newDecbufFromBytes(hex2Bin(t, "81"+test.InputTypeHex)))
		valueMr := newMessageReader(newDecbufFromBytes(hex2Bin(t, "81"+test.InputValueHex)))
		typeDec = newTypeDecoderInternal(typeMr)
		typeMr.SetCallbacks(typeDec.lookupType, nil)
		valueMr.SetCallbacks(typeDec.lookupType, nil)
		typeDec.Start()
		defer typeDec.Stop()

		for i := 0; i < test.NumMessages; i++ {
			_, err := valueMr.StartValueMessage()
			if err != nil {
				t.Errorf("%s (split): error in start value message: %v", test.Name, err)
				continue nextTest
			}
			b, err := valueMr.ReadAllValueBytes()
			if err != nil {
				t.Errorf("%s (split): error in ReadAllBytes(): %v", test.Name, err)
				continue nextTest
			}
			if !bytes.Equal(b, hex2Bin(t, test.ExpectedHex)) {
				t.Errorf("%s (split): expected: %s but got %x", test.Name, test.ExpectedHex, b)
				continue nextTest
			}
			if err := valueMr.EndMessage(); err != nil {
				t.Errorf("%s (split): error ending message: %v", test.Name, err)
				continue nextTest
			}
		}
	}
}

func crHex(b byte) string {
	return fmt.Sprintf("%x", b)
}
