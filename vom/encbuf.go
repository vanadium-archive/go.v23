package vom

import (
	"fmt"
)

// encbuf manages the write buffer for encoders.  The approach is similar to
// bytes.Buffer, but the implementation is simplified to only deal with many
// writes followed by a read of the whole buffer.
type encbuf struct {
	buf []byte
	len int
}

func newEncbuf() *encbuf {
	b := new(encbuf)
	b.ensure(1024) // start with 1KiB buffer
	return b
}

// Bytes returns a slice of the bytes written so far.
func (b *encbuf) Bytes() []byte { return b.buf[:b.len] }

// Len returns the number of bytes written so far.
func (b *encbuf) Len() int { return b.len }

// Reset sets the length to 0 to start a new round of writes.
func (b *encbuf) Reset() { b.len = 0 }

// Truncate sets the length to n, which must be in the range [0, len].
func (b *encbuf) Truncate(n int) {
	if n < 0 || n > b.len {
		panic(fmt.Errorf("vom: encbuf invalid Truncate(%d), len=%d", n, b.len))
	}
	b.len = n
}

// ensures there are at least n free bytes in the buffer.
func (b *encbuf) ensure(n int) {
	oldlen := len(b.buf)
	if oldlen-b.len < n {
		newlen := oldlen*2 + n
		newbuf := make([]byte, newlen)
		copy(newbuf, b.buf)
		b.buf = newbuf
	}
}

// Grow grows the buffer length by n bytes, and returns a slice of those bytes.
// This is different from bytes.Buffer.Grow, which doesn't return the slice of
// bytes.  Although this makes encbuf slightly easier to misuse, it helps to
// improve performance by avoiding unnecessary copying.
func (b *encbuf) Grow(n int) []byte {
	b.ensure(n)
	b.len += n
	return b.buf[b.len-n : b.len]
}

// WriteByte writes byte c into the buffer.
func (b *encbuf) WriteByte(c byte) {
	b.ensure(1)
	b.buf[b.len] = c
	b.len++
}

// Write writes slice p into the buffer.
func (b *encbuf) Write(p []byte) {
	b.ensure(len(p))
	b.len += copy(b.buf[b.len:], p)
}

// WriteString writes string s into the buffer.
func (b *encbuf) WriteString(s string) {
	b.ensure(len(s))
	b.len += copy(b.buf[b.len:], s)
}
