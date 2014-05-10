package vom

import (
	"io"
)

// decbuf manages the read buffer for decoders.  The approach is similar to
// bufio.Reader, but the API is better suited for fast decoding.
type decbuf struct {
	buf    []byte
	nr     int
	nw     int
	lim    int
	reader io.Reader
}

// newDecbuf returns a new decbuf with the given reader and size.  The size must
// be >= the largest sized ReadBuf() that will be called.
func newDecbuf(r io.Reader, size int) *decbuf {
	return &decbuf{
		buf:    make([]byte, size),
		lim:    -1,
		reader: r,
	}
}

func (b *decbuf) Size() int { return len(b.buf) }

// HasLimit returns true iff a limit has been set via SetLimit.
func (b *decbuf) HasLimit() bool {
	return b.lim > -1
}

// SetLimit sets a limit to the bytes that are returned by decbuf; after a limit
// is set, subsequent reads cannot read past the limit, even if more bytes are
// actually available.  Attempts to read past the limit return io.EOF.  Call
// SkipToLimit to disable the limit.
//
// REQUIRES: limit >=0,
// REQUIRES: SkipToLimit must be called before SetLimit is called again.
func (b *decbuf) SetLimit(limit int) {
	// Enabling these checks costs > 5% performance in microbenchmarks.
	// switch {
	// case limit < 0:
	// 	panic(fmt.Errorf("vom: decbuf set negative limit %d", limit))
	// case b.lim > -1:
	// 	panic(fmt.Errorf("vom: decbuf multiple SetLimit calls old %d, new %d", b.lim, limit))
	// }
	b.lim = limit
}

// SkipToLimit skips remaining bytes up to the previously set limit, and returns
// the leftover bytes that were skipped.  If no bytes were skipped or no limit
// was set returns 0 leftover bytes.
func (b *decbuf) SkipToLimit() (int, error) {
	leftover := b.lim
	b.lim = -1
	if leftover <= 0 {
		// Either there's no leftover or no limit was set.
		return 0, nil
	}
	if err := b.Skip(leftover); err != nil {
		return 0, err
	}
	return leftover, nil
}

// fillAtLeast fills the buffer with at least min bytes of data.  Returns an
// error iff fewer than min bytes could be filled.
//
// REQUIRES: min >= 0 && min <= Size()
func (b *decbuf) fillAtLeast(min int) error {
	if b.nw-b.nr >= min {
		return nil
	}
	if len(b.buf)-b.nr < min {
		// Move existing data forward, only if there isn't enough contiguous space
		// left for min.
		copy(b.buf, b.buf[b.nr:b.nw])
		b.nw -= b.nr
		b.nr = 0
	}
	// Need to loop; Read may return success with less bytes than requested.
	for buf := b.buf[b.nw:]; b.nw-b.nr < min; {
		n, err := b.reader.Read(buf)
		if n == 0 && err != nil {
			return err
		}
		b.nw += n
		buf = buf[n:]
	}
	return nil
}

// PeekAtLeast returns a buffer with the unread bytes in the decbuf.  The
// current read position isn't incremented.  Returns an error if fewer than min
// bytes are available, or if min isn't in the range [0, Size].
//
// The returned slice points directly at our internal buffer, and is only valid
// until the next decbuf call.
//
// REQUIRES: min >= 0 && min <= Size()
func (b *decbuf) PeekAtLeast(min int) ([]byte, error) {
	// Enabling these checks costs > 5% performance in microbenchmarks.
	// switch {
	// case min < 0:
	// 	panic(fmt.Errorf("vom: decbuf negative PeekAtLeast: %d", min))
	// case min > len(b.buf):
	// 	panic(fmt.Errorf("vom: decbuf oversized PeekAtLeast: %d", min))
	// }
	if b.lim > -1 && b.lim < min {
		return nil, io.EOF
	}
	if err := b.fillAtLeast(min); err != nil {
		return nil, err
	}
	return b.buf[b.nr:b.nw], nil
}

// SkipPeek skips the next n bytes, where n must be within the range of the
// previous peek.
func (b *decbuf) SkipPeek(n int) {
	b.nr += n
}

// ReadBuf returns a buffer with the next n unread bytes and increments the read
// position past those bytes.  Returns an error if fewer than n bytes are
// available, or if n isn't in the range [0, Size].
//
// The returned slice points directly at our internal buffer, and is only valid
// until the next decbuf call.
//
// REQUIRES: n >= 0 && n <= Size()
func (b *decbuf) ReadBuf(n int) ([]byte, error) {
	// Enabling these checks costs > 5% performance in microbenchmarks.
	// switch {
	// case n < 0:
	// 	panic(fmt.Errorf("vom: decbuf negative ReadBuf: %d", n))
	// case n > len(b.buf):
	// 	panic(fmt.Errorf("vom: decbuf oversized ReadBuf: %d", n))
	// }
	if err := b.fillAtLeast(n); err != nil {
		return nil, err
	}
	var err error
	if b.lim > -1 {
		if b.lim < n {
			n = b.lim
			err = io.EOF
		}
		b.lim -= n
	}
	buf := b.buf[b.nr : b.nr+n]
	b.nr += n
	return buf, err
}

// Skip skips the next n bytes.
func (b *decbuf) Skip(n int) error {
	for n > len(b.buf) {
		if _, err := b.ReadBuf(len(b.buf)); err != nil {
			return err
		}
		n -= len(b.buf)
	}
	_, err := b.ReadBuf(n)
	return err
}

// ReadByte reads and returns the next byte.
func (b *decbuf) ReadByte() (byte, error) {
	if b.lim == 0 {
		return 0, io.EOF
	}
	if err := b.fillAtLeast(1); err != nil {
		return 0, err
	}
	ret := b.buf[b.nr]
	b.nr++
	if b.lim > 0 {
		b.lim--
	}
	return ret, nil
}

// PeekByte returns the next byte, without changing the current read position.
func (b *decbuf) PeekByte() (byte, error) {
	if b.lim == 0 {
		return 0, io.EOF
	}
	if err := b.fillAtLeast(1); err != nil {
		return 0, err
	}
	return b.buf[b.nr], nil
}

// Read implements the io.Reader interface.
func (b *decbuf) Read(p []byte) (int, error) {
	total := len(p)
	for {
		nr := len(p)
		// TODO(toddw): Add optimization to read directly into p.
		if nr > len(b.buf) {
			nr = len(b.buf)
		}
		buf, err := b.ReadBuf(nr)
		nc := copy(p, buf)
		p = p[nc:]
		if len(p) == 0 || err != nil {
			return total - len(p), err
		}
	}
}
