// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"io"
	"math"

	"v.io/v23/verror"
)

const (
	typeIDListInitialSize = 16
	anyLenListInitialSize = 16
)

// paddingLen must be large enough to hold the header in writeMsg.
const paddingLen = maxEncodedUintBytes * 2

var (
	errUnusedTypeIds = verror.Register(pkgPath+".errUnusedTypeIds", verror.NoRetry, "{1:}{2:} vom: some type ids unused during encode {:_}")
	errUnusedAnys    = verror.Register(pkgPath+".errUnusedAnys", verror.NoRetry, "{1:}{2:} vom: some anys unused during encode {:_}")
)

func newMessageWriter(w io.Writer, version Version) *messageWriter {
	switch version {
	case Version80, Version81:
		return &messageWriter{
			version:   version,
			expanding: newEncbuf(int(math.MaxInt32)),
			w:         w,
		}
	default:
		panic("unsupported version")
	}
}

// messageWriter wraps the encoding buffer with extra state to assist in
// encoding.
type messageWriter struct {
	version Version

	expanding                     *encbuf
	hasLen, hasAny, hasTypeObject bool
	typeIncomplete                bool
	mid                           int64 // message id
	w                             io.Writer

	// TypeIDs and Any Lens for version 81
	tids    *typeIDList
	anyLens *anyLenList
}

func (b *messageWriter) StartMessage(hasAny, hasTypeObject, hasLen, typeIncomplete bool, mid int64) error {
	b.expanding.Reset()
	b.expanding.Grow(paddingLen)
	b.hasLen = hasLen
	b.hasAny = hasAny
	b.hasTypeObject = hasTypeObject
	b.typeIncomplete = typeIncomplete
	b.mid = mid
	if b.version >= Version81 && (b.hasAny || b.hasTypeObject) {
		b.tids = newTypeIDList()
	} else {
		b.tids = nil
	}
	if b.version >= Version81 && b.hasAny {
		b.anyLens = newAnyLenList()
	} else {
		b.anyLens = nil
	}
	return nil
}

func (b *messageWriter) FinishMessage() error {
	if b.version >= Version81 {
		if b.typeIncomplete {
			if _, err := b.w.Write([]byte{WireCtrlTypeIncomplete}); err != nil {
				return err
			}
		}
		if b.hasAny || b.hasTypeObject {
			ids := b.tids.NewIDs()
			var anys []uint64
			if b.hasAny {
				anys = b.anyLens.NewAnyLens()
			}
			spaceNeeded := (4 + len(ids) + len(anys)) * maxEncodedUintBytes
			headerBuf := newEncbuf(spaceNeeded)
			binaryEncodeIntEncBuf(headerBuf, b.mid)
			binaryEncodeUintEncBuf(headerBuf, uint64(len(ids)))
			for _, id := range ids {
				binaryEncodeUintEncBuf(headerBuf, uint64(id))
			}
			if b.hasAny {
				binaryEncodeUintEncBuf(headerBuf, uint64(len(anys)))
				for _, anyLen := range anys {
					binaryEncodeUintEncBuf(headerBuf, uint64(anyLen))
				}
			}
			msg := b.expanding.Bytes()
			if b.hasLen {
				binaryEncodeUintEncBuf(headerBuf, uint64(len(msg)-paddingLen))
			}
			if _, err := b.w.Write(headerBuf.Bytes()); err != nil {
				return err
			}
			_, err := b.w.Write(msg[paddingLen:])
			return err
		}
	}
	msg := b.expanding.Bytes()
	header := msg[:paddingLen]
	if b.hasLen {
		start := binaryEncodeUintEnd(header, uint64(len(msg)-paddingLen))
		header = header[:start]
	}
	start := binaryEncodeIntEnd(header, b.mid)
	_, err := b.w.Write(msg[start:])
	return err
}

// Grow the buffer by n bytes, and returns those bytes.
//
// Different from bytes.Buffer.Grow, which doesn't return the bytes.  Although
// this makes expandingEncbuf slightly easier to misuse, it helps to improve performance
// by avoiding unnecessary copying.
func (b *messageWriter) Grow(n int) ([]byte, error) {
	return b.expanding.Grow(n), nil
}

// WriteByte writes byte c into the buffer.
func (b *messageWriter) WriteOneByte(c byte) error {
	b.expanding.WriteOneByte(c)
	return nil
}

// Write writes slice p into the buffer.
func (b *messageWriter) Write(p []byte) error {
	b.expanding.WriteMaximumPossible(p)
	return nil
}

// WriteString writes string s into the buffer.
func (b *messageWriter) WriteString(s string) error {
	b.expanding.WriteMaximumPossible([]byte(s))
	return nil
}

func (b *messageWriter) ReferenceTypeID(typeID typeId) uint64 {
	if b.version == Version80 {
		panic("ReferenceTypeID() does not apply to versions before 0x81")
	}
	return b.tids.ReferenceTypeID(typeID)
}

func (b *messageWriter) marker() uint64 {
	return uint64(b.expanding.Len())
}

func (b *messageWriter) StartAny() *anyStartRef {
	if b.version == Version80 {
		return nil
	}
	return b.anyLens.StartAny(b.marker())
}

func (b *messageWriter) FinishAny(ref *anyStartRef) {
	if b.version == Version80 {
		return
	}
	b.anyLens.FinishAny(ref, b.marker())
}

func newTypeIDList() *typeIDList {
	return &typeIDList{
		tids: make([]typeId, 0, typeIDListInitialSize),
	}
}

type typeIDList struct {
	tids      []typeId
	totalSent int
}

func (l *typeIDList) ReferenceTypeID(tid typeId) uint64 {
	for index, existingTid := range l.tids {
		if existingTid == tid {
			return uint64(index)
		}
	}

	l.tids = append(l.tids, tid)
	return uint64(len(l.tids) - 1)
}

func (l *typeIDList) Reset() error {
	if l.totalSent != len(l.tids) {
		return verror.New(errUnusedTypeIds, nil)
	}
	l.tids = l.tids[:0]
	l.totalSent = 0
	return nil
}

func (l *typeIDList) NewIDs() []typeId {
	var newIDs []typeId
	if l.totalSent < len(l.tids) {
		newIDs = l.tids[l.totalSent:]
	}
	l.totalSent = len(l.tids)
	return newIDs
}

func newAnyLenList() *anyLenList {
	return &anyLenList{
		lens: make([]uint64, 0, anyLenListInitialSize),
	}
}

type anyStartRef struct {
	index  uint64 // index into the anyLen list
	marker uint64 // position marker for the start of the any
}

type anyLenList struct {
	lens      []uint64
	totalSent int
}

func (l *anyLenList) StartAny(startMarker uint64) *anyStartRef {
	l.lens = append(l.lens, 0)
	return &anyStartRef{
		index:  uint64(len(l.lens) - 1),
		marker: startMarker,
	}
}

func (l *anyLenList) FinishAny(start *anyStartRef, endMarker uint64) {
	lenIncLenBytes := endMarker - start.marker
	len := lenIncLenBytes - lenUint(lenIncLenBytes)
	l.lens[start.index] = len
}

func (l *anyLenList) Reset() error {
	if l.totalSent != len(l.lens) {
		return verror.New(errUnusedAnys, nil)
	}
	l.lens = l.lens[:0]
	l.totalSent = 0
	return nil
}

func (l *anyLenList) NewAnyLens() []uint64 {
	var newAnyLens []uint64
	if l.totalSent < len(l.lens) {
		newAnyLens = l.lens[l.totalSent:]
	}
	l.totalSent = len(l.lens)
	return newAnyLens
}
