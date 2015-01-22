package vom

import "io"

const minBufFree = 1024 // buffers always have at least 1K free after growth

// encbuf manages the write buffer for encoders.  The approach is similar to
// bytes.Buffer, but the implementation is simplified to only deal with many
// writes followed by a read of the whole buffer.
type encbuf struct {
	buf []byte // INVARIANT: len(buf) == cap(buf)
	end int    // [0, end) is data that's already written

	// It's faster to hold end than to use both the len and cap properties of buf,
	// since end is cheaper to update than buf.
}

func newEncbuf() *encbuf {
	return &encbuf{
		buf: make([]byte, minBufFree),
	}
}

// Bytes returns a slice of the bytes written so far.
func (b *encbuf) Bytes() []byte { return b.buf[:b.end] }

// Len returns the number of bytes written so far.
func (b *encbuf) Len() int { return b.end }

// Reset the length to 0 to start a new round of writes.
func (b *encbuf) Reset() { b.end = 0 }

// Truncate the length to n.
//
// REQUIRES: n in the range [0, Len]
func (b *encbuf) Truncate(n int) {
	b.end = n
}

// reserve at least min free bytes in the buffer.
func (b *encbuf) reserve(min int) {
	if len(b.buf)-b.end < min {
		newlen := len(b.buf) * 2
		if newlen-b.end < min {
			newlen = b.end + min + minBufFree
		}
		newbuf := make([]byte, newlen)
		copy(newbuf, b.buf[:b.end])
		b.buf = newbuf
	}
}

// Grow the buffer by n bytes, and returns those bytes.
//
// Different from bytes.Buffer.Grow, which doesn't return the bytes.  Although
// this makes encbuf slightly easier to misuse, it helps to improve performance
// by avoiding unnecessary copying.
func (b *encbuf) Grow(n int) []byte {
	b.reserve(n)
	oldend := b.end
	b.end += n
	return b.buf[oldend:b.end]
}

// WriteByte writes byte c into the buffer.
func (b *encbuf) WriteByte(c byte) {
	b.reserve(1)
	b.buf[b.end] = c
	b.end++
}

// Write writes slice p into the buffer.
func (b *encbuf) Write(p []byte) {
	b.reserve(len(p))
	b.end += copy(b.buf[b.end:], p)
}

// WriteString writes string s into the buffer.
func (b *encbuf) WriteString(s string) {
	b.reserve(len(s))
	b.end += copy(b.buf[b.end:], s)
}

// decbuf manages the read buffer for decoders.  The approach is similar to
// bufio.Reader, but the API is better suited for fast decoding.
type decbuf struct {
	buf      []byte // INVARIANT: len(buf) == cap(buf)
	beg, end int    // [beg, end) is data read from reader but unread by the user
	lim      int    // number of bytes left in limit, or -1 for no limit
	reader   io.Reader

	// It's faster to hold end than to use the len and cap properties of buf,
	// since end is cheaper to update than buf.
}

func newDecbuf(r io.Reader) *decbuf {
	return &decbuf{
		buf:    make([]byte, minBufFree),
		lim:    -1,
		reader: r,
	}
}

// Reset resets the buffer so it has no data.
func (b *decbuf) Reset() {
	b.beg = 0
	b.end = 0
	b.lim = -1
}

// SetLimit sets a limit to the bytes that are returned by decbuf; after a limit
// is set, subsequent reads cannot read past the limit, even if more bytes are
// available.  Attempts to read past the limit return io.EOF.  Call RemoveLimit
// to remove the limit.
//
// REQUIRES: limit >=0,
func (b *decbuf) SetLimit(limit int) {
	b.lim = limit
}

// RemoveLimit removes the limit, and returns the number of leftover bytes.
// Returns -1 if no limit was set.
func (b *decbuf) RemoveLimit() int {
	leftover := b.lim
	b.lim = -1
	return leftover
}

// fill the buffer with at least min bytes of data.  Returns an error if fewer
// than min bytes could be filled.  Doesn't advance the read position.
func (b *decbuf) fill(min int) error {
	switch avail := b.end - b.beg; {
	case avail >= min:
		// Fastpath - enough bytes are available.
		return nil
	case len(b.buf) < min:
		// The buffer isn't big enough.  Make a new buffer that's big enough and
		// copy existing data to the front.
		newlen := len(b.buf) * 2
		if newlen < min+minBufFree {
			newlen = min + minBufFree
		}
		newbuf := make([]byte, newlen)
		b.end = copy(newbuf, b.buf[b.beg:b.end])
		b.beg = 0
		b.buf = newbuf
	default:
		// The buffer is big enough.  Move existing data to the front.
		b.moveDataToFront()
	}
	// INVARIANT: len(b.buf)-b.beg >= min
	//
	// Fill [b.end:] until min bytes are available.  We must loop since Read may
	// return success with fewer bytes than requested.
	for b.end-b.beg < min {
		switch nread, err := b.reader.Read(b.buf[b.end:]); {
		case nread > 0:
			b.end += nread
		case err != nil:
			return err
		}
	}
	return nil
}

// moveDataToFront moves existing data in buf to the front, so that b.beg is 0.
func (b *decbuf) moveDataToFront() {
	b.end = copy(b.buf, b.buf[b.beg:b.end])
	b.beg = 0
}

// ReadBuf returns a buffer with the next n bytes, and increments the read
// position past those bytes.  Returns an error if fewer than n bytes are
// available.
//
// The returned slice points directly at our internal buffer, and is only valid
// until the next decbuf call.
//
// REQUIRES: n >= 0
func (b *decbuf) ReadBuf(n int) ([]byte, error) {
	if b.lim > -1 {
		if b.lim < n {
			b.lim = 0
			return nil, io.EOF
		}
		b.lim -= n
	}
	if err := b.fill(n); err != nil {
		return nil, err
	}
	buf := b.buf[b.beg : b.beg+n]
	b.beg += n
	return buf, nil
}

// PeekAtLeast returns a buffer with at least the next n bytes, but possibly
// more.  The read position isn't incremented.  Returns an error if fewer than
// min bytes are available.
//
// The returned slice points directly at our internal buffer, and is only valid
// until the next decbuf call.
//
// REQUIRES: min >= 0
func (b *decbuf) PeekAtLeast(min int) ([]byte, error) {
	if b.lim > -1 && b.lim < min {
		return nil, io.EOF
	}
	if err := b.fill(min); err != nil {
		return nil, err
	}
	return b.buf[b.beg:b.end], nil
}

// Skip increments the read position past the next n bytes.  Returns an error if
// fewer than n bytes are available.
//
// REQUIRES: n >= 0
func (b *decbuf) Skip(n int) error {
	if b.lim > -1 {
		if b.lim < n {
			b.lim = 0
			return io.EOF
		}
		b.lim -= n
	}
	// If enough bytes are available, just update indices.
	avail := b.end - b.beg
	if avail >= n {
		b.beg += n
		return nil
	}
	n -= avail
	// Keep reading into buf until we've read enough bytes.
	for {
		switch nread, err := b.reader.Read(b.buf); {
		case nread > 0:
			if nread >= n {
				b.beg = n
				b.end = nread
				return nil
			}
			n -= nread
		case err != nil:
			return err
		}
	}
}

// ReadByte returns the next byte, and increments the read position.
func (b *decbuf) ReadByte() (byte, error) {
	if b.lim > -1 {
		if b.lim == 0 {
			return 0, io.EOF
		}
		b.lim--
	}
	if err := b.fill(1); err != nil {
		return 0, err
	}
	ret := b.buf[b.beg]
	b.beg++
	return ret, nil
}

// PeekByte returns the next byte, without changing the read position.
func (b *decbuf) PeekByte() (byte, error) {
	if b.lim == 0 {
		return 0, io.EOF
	}
	if err := b.fill(1); err != nil {
		return 0, err
	}
	return b.buf[b.beg], nil
}

// ReadFull reads the next len(p) bytes into p, and increments the read position
// past those bytes.  Returns an error if fewer than len(p) bytes are available.
func (b *decbuf) ReadFull(p []byte) error {
	if b.lim > -1 {
		if b.lim < len(p) {
			return io.EOF
		}
		b.lim -= len(p)
	}
	// Copy bytes from the buffer.
	ncopy := copy(p, b.buf[b.beg:b.end])
	b.beg += ncopy
	p = p[ncopy:]
	// Keep reading into p until we've read enough bytes.
	for len(p) > 0 {
		switch nread, err := b.reader.Read(p); {
		case nread > 0:
			p = p[nread:]
		case err != nil:
			return err
		}
	}
	return nil
}
