// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"io"
	"math"
)

// paddingLen must be large enough to hold the header in writeMsg.
const paddingLen = maxEncodedUintBytes * 2

func newSwitchedEncbuf(w io.Writer, version Version) *switchedEncbuf {
	switch version {
	case Version80, Version81:
		return &switchedEncbuf{
			version:   version,
			expanding: newEncbuf(int(math.MaxInt32)),
			w:         w,
		}
	default:
		return &switchedEncbuf{
			version: version,
			chunked: newChunkedEncbuf(w),
		}
	}
}

// switchedEncbuf switches between using chunkedEncbuf or encbuf on its own, depending on
// the version of VOM.
type switchedEncbuf struct {
	version Version

	expanding                  *encbuf
	hasLen, hasAnyOrTypeObject bool
	typeIncomplete             bool
	mid                        int64
	w                          io.Writer

	// TypeIDs for version 81
	// TODO(bprosnitz) This is confusing because version 82's type IDs exist elsewhere
	tidsV81 *typeIDList

	chunked *chunkedEncbuf
}

func (b *switchedEncbuf) StartMessage(hasAnyOrTypeObject, hasLen, typeIncomplete bool, mid int64) error {
	switch b.version {
	case Version80, Version81:
		b.expanding.Reset()
		b.expanding.Grow(paddingLen)
		b.hasLen = hasLen
		b.hasAnyOrTypeObject = hasAnyOrTypeObject
		b.typeIncomplete = typeIncomplete
		b.mid = mid
		if b.version == Version81 && b.hasAnyOrTypeObject {
			b.tidsV81 = newTypeIDList()
		} else {
			b.tidsV81 = nil
		}
		return nil
	default:
		return b.chunked.StartMessage(hasAnyOrTypeObject, hasLen, typeIncomplete, int(mid))
	}
}

func (b *switchedEncbuf) FinishMessage() error {
	switch b.version {
	case Version81:
		if b.typeIncomplete {
			if _, err := b.w.Write([]byte{WireCtrlTypeIncomplete}); err != nil {
				return err
			}
		}
		if b.hasAnyOrTypeObject {
			ids := b.tidsV81.NewIDs()
			spaceNeeded := (3 + len(ids)) * maxEncodedUintBytes
			headerBuf := newEncbuf(spaceNeeded)
			binaryEncodeIntEncBuf(headerBuf, b.mid)
			binaryEncodeUintEncBuf(headerBuf, uint64(len(ids)))
			for _, id := range ids {
				binaryEncodeUintEncBuf(headerBuf, uint64(id))
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
		fallthrough
	case Version80:
		msg := b.expanding.Bytes()
		header := msg[:paddingLen]
		if b.hasLen {
			start := binaryEncodeUintEnd(header, uint64(len(msg)-paddingLen))
			header = header[:start]
		}
		start := binaryEncodeIntEnd(header, b.mid)
		_, err := b.w.Write(msg[start:])
		return err
	default:
		return b.chunked.FinishMessage()
	}
}

// Grow the buffer by n bytes, and returns those bytes.
//
// Different from bytes.Buffer.Grow, which doesn't return the bytes.  Although
// this makes expandingEncbuf slightly easier to misuse, it helps to improve performance
// by avoiding unnecessary copying.
func (b *switchedEncbuf) Grow(n int) ([]byte, error) {
	switch b.version {
	case Version80, Version81:
		return b.expanding.Grow(n), nil
	default:
		return b.chunked.Grow(n)
	}
}

// WriteByte writes byte c into the buffer.
func (b *switchedEncbuf) WriteOneByte(c byte) error {
	switch b.version {
	case Version80, Version81:
		b.expanding.WriteOneByte(c)
		return nil
	default:
		return b.chunked.WriteOneByte(c)
	}
}

// Write writes slice p into the buffer.
func (b *switchedEncbuf) Write(p []byte) error {
	switch b.version {
	case Version80, Version81:
		// Should write the entire byte array.
		b.expanding.WriteMaximumPossible(p)
		return nil
	default:
		return b.chunked.Write(p)
	}
}

// WriteString writes string s into the buffer.
func (b *switchedEncbuf) WriteString(s string) error {
	switch b.version {
	case Version80, Version81:
		// Should write the entire string.
		b.expanding.WriteMaximumPossible([]byte(s))
		return nil
	default:
		return b.chunked.WriteString(s)
	}
}

func (b *switchedEncbuf) ReferenceTypeID(typeID typeId) uint64 {
	switch b.version {
	case Version80:
		panic("ReferenceTypeID() does not apply to versions before 0x81")
	case Version81:
		return b.tidsV81.ReferenceTypeID(typeID)
	default:
		return b.chunked.ReferenceTypeID(typeID)
	}
}
