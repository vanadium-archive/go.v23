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
	if version == Version80 {
		return &switchedEncbuf{
			version:   version,
			expanding: newEncbuf(int(math.MaxInt32)),
			w:         w,
		}
	} else {
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

	expanding *encbuf
	hasLen    bool
	mid       int64
	w         io.Writer

	chunked *chunkedEncbuf
}

func (b *switchedEncbuf) StartMessage(hasAnyOrTypeObject, hasLen, typeIncomplete bool, mid int64) error {
	if b.version == Version80 {
		b.expanding.Reset()
		b.expanding.Grow(paddingLen)
		b.hasLen = hasLen
		b.mid = mid
		return nil
	} else {
		return b.chunked.StartMessage(hasAnyOrTypeObject, hasLen, typeIncomplete, int(mid))
	}
}

func (b *switchedEncbuf) FinishMessage() error {
	if b.version == Version80 {
		msg := b.expanding.Bytes()
		header := msg[:paddingLen]
		if b.hasLen {
			start := binaryEncodeUintEnd(header, uint64(len(msg)-paddingLen))
			header = header[:start]
		}
		// Note that we encode the negative id for type definitions.
		start := binaryEncodeIntEnd(header, b.mid)
		_, err := b.w.Write(msg[start:])
		return err
	} else {
		return b.chunked.FinishMessage()
	}
}

// Grow the buffer by n bytes, and returns those bytes.
//
// Different from bytes.Buffer.Grow, which doesn't return the bytes.  Although
// this makes expandingEncbuf slightly easier to misuse, it helps to improve performance
// by avoiding unnecessary copying.
func (b *switchedEncbuf) Grow(n int) ([]byte, error) {
	if b.version == Version80 {
		return b.expanding.Grow(n), nil
	} else {
		return b.chunked.Grow(n)
	}
}

// WriteByte writes byte c into the buffer.
func (b *switchedEncbuf) WriteOneByte(c byte) error {
	if b.version == Version80 {
		b.expanding.WriteOneByte(c)
		return nil
	} else {
		return b.chunked.WriteOneByte(c)
	}
}

// Write writes slice p into the buffer.
func (b *switchedEncbuf) Write(p []byte) error {
	if b.version == Version80 {
		// Should write the entire byte array.
		b.expanding.WriteMaximumPossible(p)
		return nil
	} else {
		return b.chunked.Write(p)
	}
}

// WriteString writes string s into the buffer.
func (b *switchedEncbuf) WriteString(s string) error {
	if b.version == Version80 {
		// Should write the entire string.
		b.expanding.WriteMaximumPossible([]byte(s))
		return nil
	} else {
		return b.chunked.WriteString(s)
	}
}

func (b *switchedEncbuf) ReferenceTypeID(typeID typeId) uint64 {
	if b.version == Version80 {
		panic("AddTypeID() does not apply to versions before 0x81")
	}
	return b.chunked.ReferenceTypeID(typeID)
}
