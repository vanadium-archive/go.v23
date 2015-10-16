// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"io"

	"v.io/v23/verror"
)

const typeIDListInitialSize = 16

const encodeBufSize = 2 << 10
const encodeHeaderBufSize = 2 << 10

var (
	errUnusedTypeIds = verror.Register(pkgPath+".errEncodeZeroTypeId", verror.NoRetry, "{1:}{2:} vom: some type ids unused during encode {:_}")
)

func newChunkedEncbuf(writer io.Writer) *chunkedEncbuf {
	return &chunkedEncbuf{
		writer:    writer,
		encBuf:    newEncbuf(encodeBufSize),
		headerBuf: newEncbuf(encodeHeaderBufSize), // TODO(bprosnitz) This has a fixed size buffer. This may be fine, but the list of type IDs can be large and we may want to have it be expandable.
		typeIDs:   newTypeIDList(),
	}
}

// chunkedEncbuf is an encoding buffer that performs chunking automatically.
type chunkedEncbuf struct {
	writer                 io.Writer
	encBuf                 *encbuf
	typeIDs                *typeIDList
	typeID                 int
	hasLen                 bool
	hasAnyOrTypeObject     bool
	messageStartHeaderSent bool
	headerBuf              *encbuf
}

func (b *chunkedEncbuf) StartMessage(hasAnyOrTypeObject, hasLen bool, typeID int) error {
	if b.encBuf.Len() != 0 {
		panic("unsent outstanding message")
	}
	b.hasAnyOrTypeObject = hasAnyOrTypeObject
	if b.hasAnyOrTypeObject {
		// TODO(bprosnitz) This is confusing, but we only use type ids for the value stream
		// and an easy way to prevent resets on the type stream is to reset when there is
		// an any/type id (only on the value stream).
		b.typeIDs.Reset()
	}
	b.hasLen = hasLen
	b.typeID = typeID
	b.messageStartHeaderSent = false
	return nil
}

func (b *chunkedEncbuf) FinishMessage() error {
	return b.finishChunk()
}

// Grow allocates the specified amount of space in the current buffer.
// This allocation will not be broken across chunks.
// The bytes returned are considered invalid after the next write, as
// they may be have been sent.
func (b *chunkedEncbuf) Grow(n int) ([]byte, error) {
	if b.encBuf.SpaceAvailable() < n {
		if err := b.finishChunk(); err != nil {
			return nil, err
		}
	}
	return b.encBuf.Grow(n), nil
}

// WriteOneByte writes byte c into the buffer.
func (b *chunkedEncbuf) WriteOneByte(c byte) error {
	if b.encBuf.SpaceAvailable() < 1 {
		if err := b.finishChunk(); err != nil {
			return err
		}
	}
	b.encBuf.WriteOneByte(c)
	return nil
}

// Write writes slice p into the buffer.
// Written data may be broken into chunks.
func (b *chunkedEncbuf) Write(p []byte) error {
	for len(p) > 0 {
		amt := b.encBuf.WriteMaximumPossible(p)
		p = p[amt:]
		if b.encBuf.SpaceAvailable() == 0 {
			if err := b.finishChunk(); err != nil {
				return err
			}
		}
	}
	return nil
}

// WriteString writes string s into the buffer.
func (b *chunkedEncbuf) WriteString(s string) error {
	return b.Write([]byte(s))
}

// Add a reference to the type id to the next chunk
func (b *chunkedEncbuf) ReferenceTypeID(typeID typeId) uint64 {
	return b.typeIDs.ReferenceTypeID(typeID)
}

func (b *chunkedEncbuf) writeMessageHeaderContents() {
	if b.typeID == 0 {
		panic("zero type id")
	}

}

func (b *chunkedEncbuf) writeChunkHeaderContents() {
	if !b.messageStartHeaderSent {
		binaryEncodeIntEncBuf(b.headerBuf, int64(b.typeID))
		b.messageStartHeaderSent = true
	} else {
		if b.typeID > 0 {
			b.headerBuf.WriteOneByte(WireCtrlValueCont)
		} else {
			b.headerBuf.WriteOneByte(WireCtrlTypeCont)
		}
	}
	if b.hasAnyOrTypeObject {
		newTidsInChunk := b.typeIDs.NewIDs()
		binaryEncodeUintEncBuf(b.headerBuf, uint64(len(newTidsInChunk)))
		for _, id := range newTidsInChunk {
			binaryEncodeUintEncBuf(b.headerBuf, uint64(id))
		}
	}
	if b.hasLen {
		binaryEncodeUintEncBuf(b.headerBuf, uint64(b.encBuf.Len()))
	}
}

func (b *chunkedEncbuf) finishChunk() error {
	if b.encBuf.Len() == 0 && b.messageStartHeaderSent {
		// Don't send empty messages that aren't the initial chunk.
		b.encBuf.Reset()
		return nil
	}
	b.headerBuf.Reset()
	b.writeChunkHeaderContents()
	if _, err := b.writer.Write(b.headerBuf.Bytes()); err != nil {
		b.encBuf.Reset()
		return err
	}
	_, err := b.writer.Write(b.encBuf.Bytes())
	b.encBuf.Reset()
	return err
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
