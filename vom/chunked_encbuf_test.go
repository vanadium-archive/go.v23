// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

func TestChunkedBufStartMessageParams(t *testing.T) {
	inBytes := []byte{0x01, 0x02, 0x03}
	typeIdsToInject := []byte{0x12, 0x31}
	msgTypeId := byte(0x20)
	tests := []struct {
		hasAny        bool
		hasLen        bool
		expectedBytes []byte
	}{
		{
			false,
			false,
			combineBytes(msgTypeId*2, inBytes),
		},
		{
			false,
			true,
			combineBytes(msgTypeId*2, len(inBytes), inBytes),
		},
		{
			true,
			true,
			combineBytes(msgTypeId*2, len(typeIdsToInject), typeIdsToInject, len(inBytes), inBytes),
		},
	}
	for _, test := range tests {
		var buf bytes.Buffer
		cb := newChunkedEncbuf(&buf)
		if err := cb.StartMessage(test.hasAny, test.hasLen, int(msgTypeId)); err != nil {
			t.Errorf("error in start message: %v", err)
		}
		if err := cb.Write(inBytes); err != nil {
			t.Errorf("error in write: %v", err)
		}
		for _, tid := range typeIdsToInject {
			cb.ReferenceTypeID(typeId(tid))
		}
		if err := cb.FinishMessage(); err != nil {
			t.Errorf("error in finish message: %v", err)
		}
		if got, want := buf.Bytes(), test.expectedBytes; !bytes.Equal(got, want) {
			t.Errorf("received %x but expected %x", got, want)
		}
	}
}

func TestChunkedBufStartMessageParamsWithChunking(t *testing.T) {
	inBytes1 := []byte{0x01, 0x02, 0x03}
	inBytes2 := []byte{0x04, 0x05, 0x06, 0x07}
	typeIdsToInject1 := []byte{0x12, 0x31}
	typeIdsToInject2 := []byte{0x48}
	tests := []struct {
		tid           int
		hasAny        bool
		hasLen        bool
		expectedBytes []byte
	}{
		{
			0x20,
			false,
			false,
			combineBytes(0x20*2, inBytes1, WireCtrlValueCont, inBytes2),
		},
		{
			0x20,
			false,
			true,
			combineBytes(0x20*2, len(inBytes1), inBytes1, WireCtrlValueCont, len(inBytes2), inBytes2),
		},
		{
			0x20,
			true,
			true,
			combineBytes(0x20*2, len(typeIdsToInject1), typeIdsToInject1, len(inBytes1), inBytes1, WireCtrlValueCont, len(typeIdsToInject2), typeIdsToInject2, len(inBytes2), inBytes2),
		},
		{
			-0x20,
			false,
			false,
			combineBytes(intToUint(-0x20), inBytes1, WireCtrlTypeCont, inBytes2),
		},
		{
			-0x20,
			false,
			true,
			combineBytes(intToUint(-0x20), len(inBytes1), inBytes1, WireCtrlTypeCont, len(inBytes2), inBytes2),
		},
		{
			-0x20,
			true,
			true,
			combineBytes(intToUint(-0x20), len(typeIdsToInject1), typeIdsToInject1, len(inBytes1), inBytes1, WireCtrlTypeCont, len(typeIdsToInject2), typeIdsToInject2, len(inBytes2), inBytes2),
		},
	}
	for _, test := range tests {
		var buf bytes.Buffer
		cb := newChunkedEncbuf(&buf)
		if err := cb.StartMessage(test.hasAny, test.hasLen, int(test.tid)); err != nil {
			t.Errorf("error in start message: %v", err)
		}
		if err := cb.Write(inBytes1); err != nil {
			t.Errorf("error in write: %v", err)
		}
		for _, tid := range typeIdsToInject1 {
			cb.ReferenceTypeID(typeId(tid))
		}
		if err := cb.finishChunk(); err != nil {
			t.Errorf("error while finishing chunk: %v", err)
		}
		if err := cb.Write(inBytes2); err != nil {
			t.Errorf("error in write: %v", err)
		}
		for _, tid := range typeIdsToInject2 {
			cb.ReferenceTypeID(typeId(tid))
		}
		if err := cb.FinishMessage(); err != nil {
			t.Errorf("error in finish message: %v", err)
		}
		if got, want := buf.Bytes(), test.expectedBytes; !bytes.Equal(got, want) {
			t.Errorf("received %x but expected %x", got, want)
		}
	}
}

func TestBytesAutoChunking(t *testing.T) {
	input := make([]byte, encodeBufSize*2+50)
	for i := 0; i < len(input); i++ {
		input[i] = byte(i % 10)
	}

	var buf bytes.Buffer
	cb := newChunkedEncbuf(&buf)

	if err := cb.StartMessage(false, true, 0x11); err != nil {
		t.Fatalf("error in start message: %v", err)

	}

	if err := cb.Write(input); err != nil {
		t.Errorf("error writting bytes: %v", err)
	}

	cb.FinishMessage()

	expectedOutput := combineBytes(0x22, uintAsBytes(encodeBufSize), input[:encodeBufSize], WireCtrlValueCont, uintAsBytes(encodeBufSize), input[encodeBufSize:2*encodeBufSize], WireCtrlValueCont, uintAsBytes(50), input[2*encodeBufSize:])
	if got, want := buf.Bytes(), expectedOutput; !bytes.Equal(got, want) {
		t.Errorf("bytes don't match:\n got      %v\n expected %v", got, want)
	}
}

func TestTypeIDList(t *testing.T) {
	tids := newTypeIDList()

	// Ensure NewIDs doesn't fail when empty.
	if got, want := tids.NewIDs(), []typeId(nil); !reflect.DeepEqual(got, want) {
		t.Errorf("expected ids: %v but got %v", want, got)
	}

	// First chunk
	if index := tids.ReferenceTypeID(100); index != 0 {
		t.Errorf("expected index 0, but got %v", index)
	}

	if index := tids.ReferenceTypeID(101); index != 1 {
		t.Errorf("expected index 1, but got %v", index)
	}

	if index := tids.ReferenceTypeID(102); index != 2 {
		t.Errorf("expected index 2, but got %v", index)
	}

	if index := tids.ReferenceTypeID(101); index != 1 {
		t.Errorf("expected index 1, but got %v", index)
	}

	if index := tids.ReferenceTypeID(100); index != 0 {
		t.Errorf("expected index 0, but got %v", index)
	}

	if got, want := tids.NewIDs(), []typeId{100, 101, 102}; !reflect.DeepEqual(got, want) {
		t.Errorf("expected ids: %v but got %v", want, got)
	}

	// Second chunk
	if index := tids.ReferenceTypeID(102); index != 2 {
		t.Errorf("expected index 2, but got %v", index)
	}

	if index := tids.ReferenceTypeID(103); index != 3 {
		t.Errorf("expected index 3, but got %v", index)
	}

	if index := tids.ReferenceTypeID(100); index != 0 {
		t.Errorf("expected index 0, but got %v", index)
	}

	if index := tids.ReferenceTypeID(104); index != 4 {
		t.Errorf("expected index 4, but got %v", index)
	}

	if got, want := tids.NewIDs(), []typeId{103, 104}; !reflect.DeepEqual(got, want) {
		t.Error("expected ids: %v but got %v", want, got)
	}

	// Reset before send
	if index := tids.ReferenceTypeID(105); index != 5 {
		t.Errorf("expected index 5, but got %v", index)
	}

	if err := tids.Reset(); err == nil {
		t.Errorf("expected error resetting when resetting before all types are sent")
	}

	tids.NewIDs()

	// Reset
	if err := tids.Reset(); err != nil {
		t.Errorf("unexpected error in reset: %v", err)
	}

	// Should resend the type ids that were sent before the reset.
	if index := tids.ReferenceTypeID(100); index != 0 {
		t.Errorf("expected index 0, but got %v", index)
	}

	if got, want := tids.NewIDs(), []typeId{100}; !reflect.DeepEqual(got, want) {
		t.Errorf("expected ids: %v but got %v", want, got)
	}
}

func uintAsBytes(u uint64) []byte {
	b := newEncbuf(9)
	binaryEncodeUintEncBuf(b, u)
	return b.Bytes()
}

func combineBytes(parts ...interface{}) []byte {
	bytes := []byte{}
	for _, part := range parts {
		switch v := part.(type) {
		case byte:
			bytes = append(bytes, v)
		case []byte:
			bytes = append(bytes, v...)
		case int:
			bytes = append(bytes, byte(v))
		case uint64:
			bytes = append(bytes, byte(v))
		default:
			panic(fmt.Sprintf("not implemented %T", part))
		}
	}
	return bytes
}
